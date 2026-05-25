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
	ID            string            `json:"id"`
	ServerType    string            `json:"server_type"`
	ServerURL     string            `json:"server_url"`
	DisplayName   string            `json:"display_name"`
	Description   *string           `json:"description,omitempty"`
	AuthType      string            `json:"auth_type"`
	OAuthClientID *string           `json:"oauth_client_id,omitempty"`
	Transport     string            `json:"transport"`
	Command       *string           `json:"command"`
	Args          []string          `json:"args"`
	Env           map[string]string `json:"env"`
}

func catalogEntryToResponse(e *store.CatalogEntry) catalogEntryResponse {
	args := e.Args
	if args == nil {
		args = []string{}
	}
	env := e.Env
	if env == nil {
		env = map[string]string{}
	}
	return catalogEntryResponse{
		ID:            e.ID,
		ServerType:    e.ServerType,
		ServerURL:     e.ServerURL,
		DisplayName:   e.DisplayName,
		Description:   e.Description,
		AuthType:      e.AuthType,
		OAuthClientID: e.OAuthClientID,
		Transport:     e.Transport,
		Command:       e.Command,
		Args:          args,
		Env:           env,
	}
}

type addCatalogRequest struct {
	ServerType        string            `json:"server_type"`
	ServerURL         string            `json:"server_url"`
	DisplayName       string            `json:"display_name"`
	Description       string            `json:"description"`
	AuthType          string            `json:"auth_type"`
	OAuthClientID     string            `json:"oauth_client_id"`
	OAuthClientSecret string            `json:"oauth_client_secret"`
	Transport         string            `json:"transport"`
	Command           string            `json:"command"`
	Args              []string          `json:"args"`
	Env               map[string]string `json:"env"`
}

type adminUserStore interface {
	ListAllUsers(context.Context) ([]*store.User, error)
	UpdateUserRole(ctx context.Context, id, role string) error
}

// AdminHandler serves the admin catalog and user-management pages.
type AdminHandler struct {
	catalogSvc   *catalog.Service
	catalogStore store.CatalogStoreI
	userStore    adminUserStore
	kmsClient    *kms.Client
	localMode    bool
}

func NewAdminHandler(svc *catalog.Service, cs store.CatalogStoreI, us store.UserStoreI, k *kms.Client, localMode bool) *AdminHandler {
	return &AdminHandler{catalogSvc: svc, catalogStore: cs, userStore: us, kmsClient: k, localMode: localMode}
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
		PageBase: PageBase{IsAdmin: claims != nil && claims.Role == "admin", Version: appVersion},
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
	if h.localMode && authType == "oauth2" {
		redirectErr("OAuth2 app credentials require hosted mode. Use PAT or API key in local mode.")
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
		authType, "http", nil, nil, nil,
		oauthClientID, encryptedSecret)
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
		PageBase: PageBase{IsAdmin: claims != nil && claims.Role == "admin", Version: appVersion},
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
	if req.ServerType == "" || req.DisplayName == "" || req.AuthType == "" {
		writeJSONError(w, "server_type, display_name, and auth_type are required", http.StatusBadRequest)
		return
	}

	transport := req.Transport
	if transport == "" {
		transport = "http"
	}
	if transport != "http" && transport != "stdio" {
		writeJSONError(w, "transport must be 'http' or 'stdio'", http.StatusBadRequest)
		return
	}
	if !h.localMode && transport == "stdio" {
		writeJSONError(w, "transport 'stdio' is only supported in local mode", http.StatusBadRequest)
		return
	}
	if transport == "stdio" && req.Command == "" {
		writeJSONError(w, "command is required when transport is 'stdio'", http.StatusBadRequest)
		return
	}
	if transport == "http" && req.ServerURL == "" {
		writeJSONError(w, "server_url is required when transport is 'http'", http.StatusBadRequest)
		return
	}

	validAuthTypes := map[string]bool{"api_key": true, "pat": true, "oauth2": true, "none": true}
	if !validAuthTypes[req.AuthType] {
		writeJSONError(w, "auth_type must be one of: api_key, pat, oauth2, none", http.StatusBadRequest)
		return
	}
	if h.localMode && req.AuthType == "oauth2" {
		writeJSONError(w, "auth_type 'oauth2' requires hosted mode; use 'pat' or 'api_key' in local mode", http.StatusBadRequest)
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

	var cmd *string
	if req.Command != "" {
		cmd = &req.Command
	}

	claims := ClaimsFromContext(r.Context())
	entry, err := h.catalogSvc.AddToCatalog(r.Context(),
		req.ServerType, req.ServerURL, req.DisplayName, req.Description, claims.UserID,
		req.AuthType, transport, cmd, req.Args, req.Env,
		oauthClientID, encryptedSecret)
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

// ModeHandler handles GET /api/admin/mode.
func (h *AdminHandler) ModeHandler(w http.ResponseWriter, r *http.Request) {
	mode := "hosted"
	if h.localMode {
		mode = "local"
	}
	writeJSON(w, http.StatusOK, map[string]string{"mode": mode})
}
