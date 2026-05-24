package handler

import (
	"context"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

var pageTemplates map[string]*template.Template

// InitTemplates builds a per-page template set from an fs.FS rooted at the
// repo root (i.e. templates live at templates/<page>.html within fsys).
func InitTemplates(fsys fs.FS) error {
	sub, err := fs.Sub(fsys, "templates")
	if err != nil {
		return err
	}

	partials, err := fs.Glob(sub, "partials/*.html")
	if err != nil {
		return err
	}

	pages := []string{
		"login.html",
		"register.html",
		"dashboard.html",
		"admin-catalog.html",
		"admin-users.html",
		"connect.html",
	}

	pageTemplates = make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		files := append([]string{"layout.html", page}, partials...)
		tmpl, err := template.ParseFS(sub, files...)
		if err != nil {
			return err
		}
		pageTemplates[page] = tmpl
	}
	return nil
}

func renderTemplate(w http.ResponseWriter, page string, data any) {
	tmpl, ok := pageTemplates[page]
	if !ok {
		slog.Error("template not found", "page", page)
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("template execute", "page", page, "err", err)
	}
}

// PageBase is embedded in every page data struct so the layout can render
// admin navigation links conditionally.
type PageBase struct {
	IsAdmin bool
}

// AuthPageData is used by login and register pages.
type AuthPageData struct {
	PageBase
	Error string
}

func LoginPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "login.html", AuthPageData{Error: r.URL.Query().Get("error")})
}

func RegisterPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "register.html", AuthPageData{Error: r.URL.Query().Get("error")})
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

type dashUserStore interface {
	GetUserByID(ctx context.Context, id string) (*store.User, error)
}

// CatalogCard is a catalog entry the user can connect to.
type CatalogCard struct {
	ID          string
	ServerType  string
	DisplayName string
	Description *string
	AuthType    string
}

// UpstreamView is a display-safe view of a connected upstream.
type UpstreamView struct {
	ID          string
	ServerType  string
	DisplayName string
	Status      string
}

// DashboardData is passed to dashboard.html.
type DashboardData struct {
	PageBase
	ProxyURL  string
	LocalMode bool
	Available []CatalogCard
	Connected []UpstreamView
	Error     string
}

// DashboardHandler serves the user dashboard with live data from the stores.
type DashboardHandler struct {
	userStore     dashUserStore
	upstreamStore store.UpstreamStoreI
	catalogStore  store.CatalogStoreI
	baseURL       string
	localMode     bool
}

func NewDashboardHandler(us dashUserStore, ups store.UpstreamStoreI, cs store.CatalogStoreI, baseURL string, localMode bool) *DashboardHandler {
	return &DashboardHandler{userStore: us, upstreamStore: ups, catalogStore: cs, baseURL: baseURL, localMode: localMode}
}

func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())

	user, err := h.userStore.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	upstreams, _ := h.upstreamStore.GetUpstreamConfigsByUserID(r.Context(), claims.UserID)
	catalogEntries, _ := h.catalogStore.ListActiveCatalogEntries(r.Context())

	connectedTypes := make(map[string]bool, len(upstreams))
	for _, u := range upstreams {
		connectedTypes[u.ServerType] = true
	}

	nameByType := make(map[string]string, len(catalogEntries))
	for _, e := range catalogEntries {
		nameByType[e.ServerType] = e.DisplayName
	}

	var available []CatalogCard
	for _, e := range catalogEntries {
		if !connectedTypes[e.ServerType] {
			available = append(available, CatalogCard{
				ID:          e.ID,
				ServerType:  e.ServerType,
				DisplayName: e.DisplayName,
				Description: e.Description,
				AuthType:    e.AuthType,
			})
		}
	}

	connected := make([]UpstreamView, 0, len(upstreams))
	for _, u := range upstreams {
		name := nameByType[u.ServerType]
		if name == "" {
			name = u.ServerType
		}
		connected = append(connected, UpstreamView{
			ID:          u.ID,
			ServerType:  u.ServerType,
			DisplayName: name,
			Status:      u.Status,
		})
	}

	renderTemplate(w, "dashboard.html", DashboardData{
		PageBase:  PageBase{IsAdmin: claims.Role == "admin"},
		ProxyURL:  h.baseURL + "/mcp/" + user.ProxyToken,
		LocalMode: h.localMode,
		Available: available,
		Connected: connected,
		Error:     r.URL.Query().Get("error"),
	})
}

// ---------------------------------------------------------------------------
// Connect page (API-key servers only)
// ---------------------------------------------------------------------------

// ConnectData is passed to connect.html.
type ConnectData struct {
	PageBase
	CatalogID   string
	ServerType  string
	DisplayName string
	Description *string
	Error       string
}

func (h *DashboardHandler) ConnectPage(w http.ResponseWriter, r *http.Request) {
	serverType := r.PathValue("server_type")
	entry, err := h.catalogStore.GetCatalogEntryByServerType(r.Context(), serverType)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if entry.Transport == "stdio" {
		// stdio servers are auto-connected; no credential page needed.
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	if entry.AuthType != "api_key" && entry.AuthType != "pat" {
		http.Redirect(w, r, "/api/oauth2/authorize/"+serverType, http.StatusFound) //nolint:gosec // serverType is validated against the catalog above
		return
	}
	claims := ClaimsFromContext(r.Context())
	renderTemplate(w, "connect.html", ConnectData{
		PageBase:    PageBase{IsAdmin: claims != nil && claims.Role == "admin"},
		CatalogID:   entry.ID,
		ServerType:  entry.ServerType,
		DisplayName: entry.DisplayName,
		Description: entry.Description,
		Error:       r.URL.Query().Get("error"),
	})
}
