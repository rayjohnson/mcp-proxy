package handler

import (
	"encoding/json"
	"net/http"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// ToggleHandler handles enable/disable toggling for upstream configs and catalog entries.
type ToggleHandler struct {
	upstreamStore store.UpstreamStoreI
	toggleStore   store.ToggleStoreI
}

func NewToggleHandler(us store.UpstreamStoreI, ts store.ToggleStoreI) *ToggleHandler {
	return &ToggleHandler{upstreamStore: us, toggleStore: ts}
}

// ToggleUpstreamHandler handles POST /api/upstreams/{id}/toggle.
func (h *ToggleHandler) ToggleUpstreamHandler(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")
	cfg, err := h.upstreamStore.GetUpstreamConfigByID(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if cfg.UserID != claims.UserID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	newEnabled, err := h.upstreamStore.ToggleUpstream(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": newEnabled}) //nolint:errcheck
}

// ToggleCatalogHandler handles POST /api/catalog/{id}/toggle.
func (h *ToggleHandler) ToggleCatalogHandler(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	catalogID := r.PathValue("id")
	newEnabled, err := h.toggleStore.ToggleCatalogEntry(r.Context(), claims.UserID, catalogID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": newEnabled}) //nolint:errcheck
}
