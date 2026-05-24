package handler

import (
	"context"
	"net/http"
	"net/url"

	"github.com/rayjohnson/mcp-proxy/internal/catalog"
	"github.com/rayjohnson/mcp-proxy/internal/kms"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// catalogEntryResponse is the JSON view of a catalog entry (no encrypted fields).
type catalogEntryResponse struct {
	ID            string  `json:"id"`
	ServerType    string  `json:"server_type"`
	ServerURL     string  `json:"server_url"`
	DisplayName   string  `json:"display_name"`
	Description   *string `json:"description,omitempty"`
	AuthType      string  `json:"auth_type"`
	OAuthClientID *string `json:"oauth_client_id,omitempty"`
}

func catalogEntryToResponse(e *store.CatalogEntry) catalogEntryResponse {
	return catalogEntryResponse{
		ID:            e.ID,
		ServerType:    e.ServerType,
		ServerURL:     e.ServerURL,
		DisplayName:   e.DisplayName,
		Description:   e.Description,
		AuthType:      e.AuthType,
		OAuthClientID: e.OAuthClientID,
	}
}

type addCatalogRequest struct {
	ServerType        string `json:"server_type"`
	ServerURL         string `json:"server_url"`
	DisplayName       string `json:"display_name"`
	Description       string `json:"description"`
	AuthType          string `json:"auth_type"`
	OAuthClientID     string `json:"oauth_client_id"`
	OAuthClientSecret string `json:"oauth_client_secret"`
}

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

// --- JSON API ---

// ListCatalogAPI handles GET /api/admin/catalog.
func (h *AdminHandler) ListCatalogAPI(w http.ResponseWriter, r *http.Request) {
	entries, err := h.catalogStore.ListActiveCatalogEntries(r.Context())
	if err != nil {
		writeJSONError(w, "failed to list catalog", http.StatusInternalServerError)
		return
	}
	resp := make([]catalogEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = catalogEntryToResponse(e)
	}
	writeJSON(w, http.StatusOK, resp)
}

// AddCatalogEntryAPI handles POST /api/admin/catalog.
func (h *AdminHandler) AddCatalogEntryAPI(w http.ResponseWriter, r *http.Request) {
	var req addCatalogRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.ServerType == "" || req.ServerURL == "" || req.DisplayName == "" || req.AuthType == "" {
		writeJSONError(w, "server_type, server_url, display_name, and auth_type are required", http.StatusBadRequest)
		return
	}
	if req.AuthType != "api_key" && req.AuthType != "oauth2" {
		writeJSONError(w, "auth_type must be api_key or oauth2", http.StatusBadRequest)
		return
	}

	var oauthClientID *string
	var encryptedSecret []byte

	if req.AuthType == "oauth2" {
		if req.OAuthClientID == "" || req.OAuthClientSecret == "" {
			writeJSONError(w, "oauth_client_id and oauth_client_secret are required for oauth2", http.StatusBadRequest)
			return
		}
		oauthClientID = &req.OAuthClientID
		var err error
		encryptedSecret, err = h.kmsClient.Encrypt(r.Context(), []byte(req.OAuthClientSecret))
		if err != nil {
			writeJSONError(w, "failed to encrypt OAuth secret", http.StatusInternalServerError)
			return
		}
	}

	claims := ClaimsFromContext(r.Context())
	entry, err := h.catalogSvc.AddToCatalog(r.Context(),
		req.ServerType, req.ServerURL, req.DisplayName, req.Description, claims.UserID,
		req.AuthType, oauthClientID, encryptedSecret)
	if err != nil {
		writeJSONError(w, "failed to add catalog entry: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, catalogEntryToResponse(entry))
}

// RemoveCatalogEntryAPI handles DELETE /api/admin/catalog/{id}.
func (h *AdminHandler) RemoveCatalogEntryAPI(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, "missing catalog entry id", http.StatusBadRequest)
		return
	}
	if err := h.catalogSvc.RemoveFromCatalog(r.Context(), id); err != nil {
		writeJSONError(w, "failed to remove catalog entry", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
