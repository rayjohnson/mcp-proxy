package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rayjohnson/mcp-proxy/internal/aitools"
	"github.com/rayjohnson/mcp-proxy/internal/catalog"
	"github.com/rayjohnson/mcp-proxy/internal/config"
	"github.com/rayjohnson/mcp-proxy/internal/handler"
	"github.com/rayjohnson/mcp-proxy/internal/kms"
	internalmcp "github.com/rayjohnson/mcp-proxy/internal/mcp"
	"github.com/rayjohnson/mcp-proxy/internal/oauth2client"
	"github.com/rayjohnson/mcp-proxy/internal/store"
	sqstore "github.com/rayjohnson/mcp-proxy/internal/store/sqlite"
	"github.com/rayjohnson/mcp-proxy/internal/upstream"
	"github.com/rayjohnson/mcp-proxy/web"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("mcp-proxy " + version)
		os.Exit(0)
	}

	handler.SetVersion(version)

	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	if err := handler.InitTemplates(web.FS); err != nil {
		slog.Error("init templates", "err", err)
		os.Exit(1)
	}

	kmsClient, err := kms.New(ctx, cfg.KMSKeyName, cfg.LocalKMSKey)
	if err != nil {
		slog.Error("init kms", "err", err)
		os.Exit(1)
	}
	defer func() { _ = kmsClient.Close() }()

	// Store factory: choose Postgres or SQLite based on config.
	var (
		userStore        store.UserStoreI
		upstreamStore    store.UpstreamStoreI
		catalogStore     store.CatalogStoreI
		suggestionStore  store.SuggestionStoreI
		oauth2StateStore store.OAuth2StateStoreI
		toggleStore      store.ToggleStoreI
	)

	if cfg.LocalMode {
		slog.Info("local mode: using SQLite", "dsn", cfg.DBDSN)
		sqlDB, err := sqstore.Open(ctx, cfg.DBDSN)
		if err != nil {
			slog.Error("open sqlite", "err", err)
			os.Exit(1)
		}
		defer sqlDB.Close() //nolint:errcheck
		userStore = sqstore.NewUserStore(sqlDB)
		upstreamStore = sqstore.NewUpstreamStore(sqlDB)
		catalogStore = sqstore.NewCatalogStore(sqlDB)
		suggestionStore = sqstore.NewSuggestionStore(sqlDB)
		oauth2StateStore = sqstore.NewOAuth2StateStore(sqlDB)
		toggleStore = sqstore.NewToggleStore(sqlDB)
	} else {
		pool, err := store.NewPool(ctx, cfg.DBDSN)
		if err != nil {
			slog.Error("connect db", "err", err)
			os.Exit(1)
		}
		defer pool.Close()
		userStore = store.NewUserStore(pool)
		upstreamStore = store.NewUpstreamStore(pool)
		catalogStore = store.NewCatalogStore(pool)
		suggestionStore = store.NewSuggestionStore(pool)
		oauth2StateStore = store.NewOAuth2StateStore(pool)
		toggleStore = store.NewToggleStore(pool)
	}

	// Services
	catalogSvc := catalog.NewService(catalogStore, suggestionStore)
	oauth2Svc := oauth2client.NewService(oauth2StateStore, upstreamStore, catalogStore, kmsClient, cfg.BaseURL)

	// Handlers
	authHandler := handler.NewAuthHandler(userStore, catalogStore, suggestionStore)
	adminHandler := handler.NewAdminHandler(catalogSvc, catalogStore, userStore, kmsClient, cfg.LocalMode)
	dashHandler := handler.NewDashboardHandler(userStore, upstreamStore, catalogStore, toggleStore, cfg.BaseURL, cfg.LocalMode)
	toggleHandler := handler.NewToggleHandler(upstreamStore, toggleStore)
	suggestionHandler := handler.NewSuggestionHandler(suggestionStore)
	upstreamHandler := handler.NewUpstreamHandler(upstreamStore, catalogStore, kmsClient)
	oauth2Handler := handler.NewOAuth2Handler(oauth2Svc)
	aiToolsHandler := handler.NewAIToolsHandler(
		[]aitools.Configurer{
			&aitools.ClaudeDesktopTool{},
			&aitools.GeminiCLITool{},
			&aitools.ClaudeCodeTool{},
		},
		userStore,
		cfg.BaseURL,
		cfg.LocalMode,
	)

	// MCP proxy
	sessionDeps := internalmcp.SessionDeps{
		UpstreamStore: upstreamStore,
		CatalogStore:  catalogStore,
		ToggleStore:   toggleStore,
		KMSDecrypt: func(ctx context.Context, ciphertext []byte) ([]byte, error) {
			return kmsClient.Decrypt(ctx, ciphertext)
		},
		AuthHeader: func(cfg *store.UpstreamConfig, plainCreds []byte) (string, error) {
			adapter, err := upstream.GetAdapter(cfg.ServerType)
			if err != nil {
				return "", err
			}
			return adapter.AuthHeader(plainCreds)
		},
		UpdateTransport: func(ctx context.Context, id, transport string) error {
			return upstreamStore.UpdateDetectedTransport(ctx, id, transport)
		},
	}

	mcpHandler := sdkmcp.NewStreamableHTTPHandler(
		internalmcp.GetServerFunc(internalmcp.ProxyServerDeps{
			UserStore:   userStore,
			SessionDeps: sessionDeps,
			ManagementDeps: internalmcp.ManagementDeps{
				UpstreamStore: upstreamStore,
				CatalogStore:  catalogStore,
				KMSEncrypt:    kmsClient.Encrypt,
			},
		}),
		nil,
	)

	mux := http.NewServeMux()

	// MCP proxy endpoint
	mux.Handle("GET /mcp/{token}", mcpHandler)
	mux.Handle("POST /mcp/{token}", mcpHandler)
	mux.Handle("DELETE /mcp/{token}", mcpHandler)

	// Auth API
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
	mux.Handle("GET /api/proxy/endpoint",
		handler.AuthMiddleware(http.HandlerFunc(authHandler.ProxyEndpoint)))

	// Upstream management API (authenticated)
	mux.Handle("GET /api/upstream",
		handler.AuthMiddleware(http.HandlerFunc(upstreamHandler.ListUpstreams)))
	mux.Handle("POST /api/upstream/connect",
		handler.AuthMiddleware(http.HandlerFunc(upstreamHandler.Connect)))
	mux.Handle("POST /api/upstream/{id}/disconnect",
		handler.AuthMiddleware(http.HandlerFunc(upstreamHandler.Disconnect)))
	mux.Handle("DELETE /api/upstream/{id}",
		handler.AuthMiddleware(http.HandlerFunc(upstreamHandler.DeleteUpstream)))
	mux.Handle("PATCH /api/upstream/{id}/credentials",
		handler.AuthMiddleware(http.HandlerFunc(upstreamHandler.UpdateCredentials)))
	mux.Handle("GET /api/upstream/{id}/status",
		handler.AuthMiddleware(http.HandlerFunc(upstreamHandler.GetStatus)))
	mux.Handle("POST /api/upstreams/{id}/toggle",
		handler.AuthMiddleware(http.HandlerFunc(toggleHandler.ToggleUpstreamHandler)))
	mux.Handle("POST /api/catalog/{id}/toggle",
		handler.AuthMiddleware(http.HandlerFunc(toggleHandler.ToggleCatalogHandler)))

	// OAuth2 flows
	mux.Handle("GET /api/oauth2/authorize/{server_type}",
		handler.AuthMiddleware(http.HandlerFunc(oauth2Handler.Authorize)))
	mux.HandleFunc("GET /api/oauth2/callback/{server_type}", oauth2Handler.Callback)

	// Suggestions API (authenticated)
	mux.Handle("GET /api/suggestions",
		handler.AuthMiddleware(http.HandlerFunc(suggestionHandler.ListSuggestions)))
	mux.Handle("POST /api/suggestions/{id}/dismiss",
		handler.AuthMiddleware(http.HandlerFunc(suggestionHandler.DismissSuggestion)))
	mux.Handle("POST /api/suggestions/{id}/accept",
		handler.AuthMiddleware(http.HandlerFunc(suggestionHandler.AcceptSuggestion)))

	// AI tool auto-configuration (local mode only)
	mux.Handle("GET /api/tools",
		handler.AuthMiddleware(http.HandlerFunc(aiToolsHandler.StatusAPI)))
	mux.Handle("POST /api/tools/{id}/configure",
		handler.AuthMiddleware(http.HandlerFunc(aiToolsHandler.ConfigureAPI)))
	mux.Handle("DELETE /api/tools/{id}/configure",
		handler.AuthMiddleware(http.HandlerFunc(aiToolsHandler.UnconfigureAPI)))

	// Admin pages and API (admin role required)
	adminMW := func(h http.Handler) http.Handler {
		return handler.AuthMiddleware(handler.AdminMiddleware(h))
	}
	mux.Handle("GET /admin/catalog",
		adminMW(http.HandlerFunc(adminHandler.CatalogPage)))
	mux.Handle("POST /admin/catalog",
		adminMW(http.HandlerFunc(adminHandler.AddCatalogEntry)))
	mux.Handle("POST /admin/catalog/{id}/remove",
		adminMW(http.HandlerFunc(adminHandler.RemoveCatalogEntry)))
	mux.Handle("GET /admin/users",
		adminMW(http.HandlerFunc(adminHandler.UsersPage)))
	mux.Handle("POST /admin/users/{id}/role",
		adminMW(http.HandlerFunc(adminHandler.UpdateUserRole)))

	// Admin JSON API
	mux.Handle("GET /api/admin/catalog",
		adminMW(http.HandlerFunc(adminHandler.ListCatalogAPI)))
	mux.Handle("POST /api/admin/catalog",
		adminMW(http.HandlerFunc(adminHandler.AddCatalogEntryAPI)))
	mux.Handle("DELETE /api/admin/catalog/{id}",
		adminMW(http.HandlerFunc(adminHandler.RemoveCatalogEntryAPI)))
	mux.Handle("PATCH /api/admin/catalog/{id}",
		adminMW(http.HandlerFunc(adminHandler.UpdateCatalogEntryAPI)))
	mux.HandleFunc("GET /api/admin/mode", adminHandler.ModeHandler)
	mux.Handle("GET /api/version/check",
		adminMW(http.HandlerFunc(adminHandler.CheckVersionAPI)))
	mux.Handle("POST /api/admin/update",
		adminMW(http.HandlerFunc(adminHandler.UpdateAPI)))

	// UI pages
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
	mux.HandleFunc("GET /login", handler.LoginPage)
	mux.HandleFunc("GET /register", handler.RegisterPage)
	mux.Handle("GET /dashboard",
		handler.AuthMiddleware(http.HandlerFunc(dashHandler.Dashboard)))
	mux.Handle("GET /connect/{server_type}",
		handler.AuthMiddleware(http.HandlerFunc(dashHandler.ConnectPage)))

	// Static files (embedded)
	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		slog.Error("init static FS", "err", err)
		os.Exit(1)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Health probes
	mux.HandleFunc("GET /health", handler.HealthHandler)
	mux.HandleFunc("GET /healthz", handler.HealthHandler)

	// Background health probe
	handler.StartHealthProbe(ctx, upstreamStore, userStore, oauth2Svc)

	addr := ":" + cfg.Port
	slog.Info("starting server", "addr", addr, "local_mode", cfg.LocalMode)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler.LoggingMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
