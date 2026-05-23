package handler

import (
	"encoding/json"
	"net/http"

	"github.com/rayjohnson/mcp-proxy/internal/catalog"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type AdminHandler struct {
	catalogSvc  *catalog.Service
	catalogStore *store.CatalogStore
}

func NewAdminHandler(svc *catalog.Service, cs *store.CatalogStore) *AdminHandler {
	return &AdminHandler{catalogSvc: svc, catalogStore: cs}
}

func (h *AdminHandler) ListCatalog(w http.ResponseWriter, r *http.Request) {
	entries, err := h.catalogStore.ListActiveCatalogEntries(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries) //nolint:errcheck
}

func (h *AdminHandler) AddCatalogEntry(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ServerType  string `json:"server_type"`
		ServerURL   string `json:"server_url"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.ServerType == "" || body.ServerURL == "" || body.DisplayName == "" {
		http.Error(w, "server_type, server_url, display_name are required", http.StatusBadRequest)
		return
	}

	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	entry, err := h.catalogSvc.AddToCatalog(r.Context(),
		body.ServerType, body.ServerURL, body.DisplayName, body.Description, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry) //nolint:errcheck
}

func (h *AdminHandler) DeleteCatalogEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing catalog id", http.StatusBadRequest)
		return
	}
	if err := h.catalogSvc.RemoveFromCatalog(r.Context(), id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
