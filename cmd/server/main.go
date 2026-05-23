package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rayjohnson/mcp-proxy/internal/catalog"
	"github.com/rayjohnson/mcp-proxy/internal/config"
	"github.com/rayjohnson/mcp-proxy/internal/handler"
	"github.com/rayjohnson/mcp-proxy/internal/kms"
	internalmcp "github.com/rayjohnson/mcp-proxy/internal/mcp"
	"github.com/rayjohnson/mcp-proxy/internal/oauth2client"
	"github.com/rayjohnson/mcp-proxy/internal/store"
	"github.com/rayjohnson/mcp-proxy/internal/upstream"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	pool, err := store.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		slog.Error("connect db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	kmsClient, err := kms.New(ctx, cfg.KMSKeyName)
	if err != nil {
		slog.Error("init kms", "err", err)
		os.Exit(1)
	}
	defer kmsClient.Close()

	// Stores
	userStore := store.NewUserStore(pool)
	upstreamStore := store.NewUpstreamStore(pool)
	catalogStore := store.NewCatalogStore(pool)
	suggestionStore := store.NewSuggestionStore(pool)
	oauth2StateStore := store.NewOAuth2StateStore(pool)

	// Services
	catalogSvc := catalog.NewService(catalogStore, suggestionStore)
	oauth2Svc := oauth2client.NewService(oauth2StateStore, upstreamStore, kmsClient, cfg.BaseURL)

	// Handlers
	authHandler := handler.NewAuthHandler(userStore, catalogStore, suggestionStore)
	adminHandler := handler.NewAdminHandler(catalogSvc, catalogStore)
	suggestionHandler := handler.NewSuggestionHandler(suggestionStore)
	upstreamHandler := handler.NewUpstreamHandler(upstreamStore, kmsClient)
	oauth2Handler := handler.NewOAuth2Handler(oauth2Svc)

	// MCP proxy
	sessionDeps := internalmcp.SessionDeps{
		UpstreamStore: upstreamStore,
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
	mux.Handle("POST /api/upstream",
		handler.AuthMiddleware(http.HandlerFunc(upstreamHandler.AddUpstream)))
	mux.Handle("DELETE /api/upstream/{id}",
		handler.AuthMiddleware(http.HandlerFunc(upstreamHandler.DeleteUpstream)))
	mux.Handle("PATCH /api/upstream/{id}/credentials",
		handler.AuthMiddleware(http.HandlerFunc(upstreamHandler.UpdateCredentials)))
	mux.Handle("GET /api/upstream/{id}/status",
		handler.AuthMiddleware(http.HandlerFunc(upstreamHandler.GetStatus)))

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

	// Admin API (admin role required)
	adminMW := func(h http.Handler) http.Handler {
		return handler.AuthMiddleware(handler.AdminMiddleware(h))
	}
	mux.Handle("GET /api/admin/catalog",
		adminMW(http.HandlerFunc(adminHandler.ListCatalog)))
	mux.Handle("POST /api/admin/catalog",
		adminMW(http.HandlerFunc(adminHandler.AddCatalogEntry)))
	mux.Handle("DELETE /api/admin/catalog/{id}",
		adminMW(http.HandlerFunc(adminHandler.DeleteCatalogEntry)))

	// UI pages
	mux.HandleFunc("GET /login", handler.LoginPage)
	mux.HandleFunc("GET /register", handler.RegisterPage)
	mux.Handle("GET /dashboard", handler.AuthMiddleware(http.HandlerFunc(handler.DashboardPage)))

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Health probes for Cloud Run and Docker HEALTHCHECK
	mux.HandleFunc("GET /health", handler.HealthHandler)
	mux.HandleFunc("GET /healthz", handler.HealthHandler)

	// Start background health probe
	handler.StartHealthProbe(ctx, upstreamStore, userStore, oauth2Svc)

	addr := ":" + cfg.Port
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, handler.LoggingMiddleware(mux)); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
