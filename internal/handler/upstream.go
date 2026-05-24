package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rayjohnson/mcp-proxy/internal/kms"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type UpstreamHandler struct {
	upstreamStore *store.UpstreamStore
	catalogStore  *store.CatalogStore
	kmsClient     *kms.Client
}

func NewUpstreamHandler(us *store.UpstreamStore, cs *store.CatalogStore, k *kms.Client) *UpstreamHandler {
	return &UpstreamHandler{upstreamStore: us, catalogStore: cs, kmsClient: k}
}

func (h *UpstreamHandler) ListUpstreams(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	cfgs, err := h.upstreamStore.GetUpstreamConfigsByUserID(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Strip encrypted creds from response.
	type safeConfig struct {
		ID         string  `json:"id"`
		ServerType string  `json:"server_type"`
		ServerURL  string  `json:"server_url"`
		AuthType   string  `json:"auth_type"`
		Status     string  `json:"status"`
		Transport  *string `json:"detected_transport,omitempty"`
	}
	out := make([]safeConfig, len(cfgs))
	for i, c := range cfgs {
		out[i] = safeConfig{
			ID:         c.ID,
			ServerType: c.ServerType,
			ServerURL:  c.ServerURL,
			AuthType:   c.AuthType,
			Status:     c.Status,
			Transport:  c.DetectedTransport,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out) //nolint:errcheck
}

// Connect handles POST /api/upstream/connect (catalog-driven, PRG).
// The user supplies only their API key; URL and auth type come from the catalog.
func (h *UpstreamHandler) Connect(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/dashboard?error=Invalid+form.", http.StatusSeeOther)
		return
	}
	catalogID := r.FormValue("catalog_id")
	apiKey := r.FormValue("api_key")
	if catalogID == "" || apiKey == "" {
		http.Redirect(w, r, "/dashboard?error=Missing+required+fields.", http.StatusSeeOther)
		return
	}

	entry, err := h.catalogStore.GetCatalogEntryByID(r.Context(), catalogID)
	if err != nil {
		http.Redirect(w, r, "/dashboard?error=Unknown+server.", http.StatusSeeOther)
		return
	}
	if entry.AuthType != "api_key" {
		http.Redirect(w, r, "/dashboard?error=Use+OAuth+for+this+server.", http.StatusSeeOther)
		return
	}

	encrypted, err := h.kmsClient.Encrypt(r.Context(), []byte(apiKey))
	if err != nil {
		http.Redirect(w, r, "/connect/"+entry.ServerType+"?error=Internal+error.", http.StatusSeeOther) //nolint:gosec // serverType from catalog, not user input
		return
	}

	_, err = h.upstreamStore.CreateUpstreamConfig(r.Context(),
		claims.UserID, entry.ServerType, entry.ServerURL, "api_key", encrypted)
	if err != nil {
		http.Redirect(w, r, "/connect/"+entry.ServerType+"?error=Failed+to+connect.", http.StatusSeeOther) //nolint:gosec // serverType from catalog, not user input
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *UpstreamHandler) DeleteUpstream(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	// Verify ownership.
	cfg, err := h.upstreamStore.GetUpstreamConfigByID(r.Context(), id)
	if err != nil || cfg.UserID != claims.UserID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := h.upstreamStore.DeleteUpstreamConfig(r.Context(), id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Disconnect handles POST /api/upstream/{id}/disconnect (PRG form-based delete).
func (h *UpstreamHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/dashboard?error=Missing+ID.", http.StatusSeeOther)
		return
	}
	cfg, err := h.upstreamStore.GetUpstreamConfigByID(r.Context(), id)
	if err != nil || cfg.UserID != claims.UserID {
		http.Redirect(w, r, "/dashboard?error=Not+found.", http.StatusSeeOther)
		return
	}
	if err := h.upstreamStore.DeleteUpstreamConfig(r.Context(), id); err != nil {
		http.Redirect(w, r, "/dashboard?error=Failed+to+remove.", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *UpstreamHandler) UpdateCredentials(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	var body struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cfg, err := h.upstreamStore.GetUpstreamConfigByID(r.Context(), id)
	if err != nil || cfg.UserID != claims.UserID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	encrypted, err := h.kmsClient.Encrypt(r.Context(), []byte(body.APIKey))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.upstreamStore.UpdateEncryptedCreds(r.Context(), id, encrypted); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.upstreamStore.UpdateUpstreamStatus(r.Context(), id, "active"); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UpstreamHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	cfg, err := h.upstreamStore.GetUpstreamConfigByID(r.Context(), id)
	if err != nil || cfg.UserID != claims.UserID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": cfg.ID, "status": cfg.Status}) //nolint:errcheck
}

// GetUpstreamConfigsByUserID is a helper for the session layer.
func GetUpstreamConfigsByUserID(ctx context.Context, userStore *store.UpstreamStore, userID string) ([]*store.UpstreamConfig, error) {
	return userStore.GetUpstreamConfigsByUserID(ctx, userID)
}
