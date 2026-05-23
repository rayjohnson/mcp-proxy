package handler

import (
	"context"
	"net/http"
	"net/url"

	"github.com/rayjohnson/mcp-proxy/internal/catalog"
	"github.com/rayjohnson/mcp-proxy/internal/kms"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type adminUserStore interface {
	ListAllUsers(context.Context) ([]*store.User, error)
	UpdateUserRole(ctx context.Context, id, role string) error
}

// AdminHandler serves the admin catalog and user-management pages.
type AdminHandler struct {
	catalogSvc   *catalog.Service
	catalogStore *store.CatalogStore
	userStore    adminUserStore
	kmsClient    *kms.Client
}

func NewAdminHandler(svc *catalog.Service, cs *store.CatalogStore, us *store.UserStore, k *kms.Client) *AdminHandler {
	return &AdminHandler{catalogSvc: svc, catalogStore: cs, userStore: us, kmsClient: k}
}

// --- Catalog page ---

// AdminCatalogData is passed to admin-catalog.html.
type AdminCatalogData struct {
	PageBase
	Entries []*store.CatalogEntry
	Error   string
}

func (h *AdminHandler) CatalogPage(w http.ResponseWriter, r *http.Request) {
	entries, _ := h.catalogStore.ListActiveCatalogEntries(r.Context())
	claims := ClaimsFromContext(r.Context())
	renderTemplate(w, "admin-catalog.html", AdminCatalogData{
		PageBase: PageBase{IsAdmin: claims != nil && claims.Role == "admin"},
		Entries:  entries,
		Error:    r.URL.Query().Get("error"),
	})
}

// AddCatalogEntry handles POST /admin/catalog (PRG).
func (h *AdminHandler) AddCatalogEntry(w http.ResponseWriter, r *http.Request) {
	redirectErr := func(msg string) {
		http.Redirect(w, r, "/admin/catalog?error="+url.QueryEscape(msg), http.StatusSeeOther)
	}

	if err := r.ParseForm(); err != nil {
		redirectErr("Invalid form data.")
		return
	}
	serverType := r.FormValue("server_type")
	serverURL := r.FormValue("server_url")
	displayName := r.FormValue("display_name")
	description := r.FormValue("description")
	authType := r.FormValue("auth_type")

	if serverType == "" || serverURL == "" || displayName == "" || authType == "" {
		redirectErr("Server type, URL, name, and auth type are required.")
		return
	}
	if authType != "api_key" && authType != "oauth2" {
		redirectErr("Auth type must be api_key or oauth2.")
		return
	}

	var oauthClientID *string
	var encryptedSecret []byte

	if authType == "oauth2" {
		clientID := r.FormValue("oauth_client_id")
		clientSecret := r.FormValue("oauth_client_secret")
		if clientID == "" || clientSecret == "" {
			redirectErr("OAuth2 requires a client ID and secret.")
			return
		}
		oauthClientID = &clientID
		var err error
		encryptedSecret, err = h.kmsClient.Encrypt(r.Context(), []byte(clientSecret))
		if err != nil {
			redirectErr("Failed to encrypt OAuth secret.")
			return
		}
	}

	claims := ClaimsFromContext(r.Context())
	_, err := h.catalogSvc.AddToCatalog(r.Context(),
		serverType, serverURL, displayName, description, claims.UserID,
		authType, oauthClientID, encryptedSecret)
	if err != nil {
		redirectErr("Failed to add server: " + err.Error())
		return
	}
	http.Redirect(w, r, "/admin/catalog", http.StatusSeeOther)
}

// RemoveCatalogEntry handles POST /admin/catalog/{id}/remove (PRG).
func (h *AdminHandler) RemoveCatalogEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/admin/catalog?error=Missing+catalog+ID.", http.StatusSeeOther)
		return
	}
	if err := h.catalogSvc.RemoveFromCatalog(r.Context(), id); err != nil {
		http.Redirect(w, r, "/admin/catalog?error=Failed+to+remove+entry.", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/catalog", http.StatusSeeOther)
}

// --- Users page ---

// AdminUsersData is passed to admin-users.html.
type AdminUsersData struct {
	PageBase
	Users []*store.User
	Error string
}

func (h *AdminHandler) UsersPage(w http.ResponseWriter, r *http.Request) {
	users, _ := h.userStore.ListAllUsers(r.Context())
	claims := ClaimsFromContext(r.Context())
	renderTemplate(w, "admin-users.html", AdminUsersData{
		PageBase: PageBase{IsAdmin: claims != nil && claims.Role == "admin"},
		Users:    users,
		Error:    r.URL.Query().Get("error"),
	})
}

// UpdateUserRole handles POST /admin/users/{id}/role (PRG).
func (h *AdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	redirectErr := func(msg string) {
		http.Redirect(w, r, "/admin/users?error="+url.QueryEscape(msg), http.StatusSeeOther)
	}

	if err := r.ParseForm(); err != nil {
		redirectErr("Invalid form.")
		return
	}
	id := r.PathValue("id")
	role := r.FormValue("role")
	if id == "" || (role != "admin" && role != "developer") {
		redirectErr("Invalid request.")
		return
	}
	// Prevent admins from removing their own admin role.
	claims := ClaimsFromContext(r.Context())
	if claims != nil && claims.UserID == id && role != "admin" {
		redirectErr("You cannot remove your own admin role.")
		return
	}
	if err := h.userStore.UpdateUserRole(r.Context(), id, role); err != nil {
		redirectErr("Failed to update role.")
		return
	}
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}
