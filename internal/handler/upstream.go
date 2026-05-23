package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rayjohnson/mcp-proxy/internal/kms"
	"github.com/rayjohnson/mcp-proxy/internal/store"
	"github.com/rayjohnson/mcp-proxy/internal/upstream"
)

type UpstreamHandler struct {
	upstreamStore *store.UpstreamStore
	kmsClient     *kms.Client
}

func NewUpstreamHandler(us *store.UpstreamStore, k *kms.Client) *UpstreamHandler {
	return &UpstreamHandler{upstreamStore: us, kmsClient: k}
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

func (h *UpstreamHandler) AddUpstream(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		ServerType string `json:"server_type"`
		ServerURL  string `json:"server_url"`
		APIKey     string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.ServerType == "" || body.ServerURL == "" || body.APIKey == "" {
		http.Error(w, "server_type, server_url, api_key are required", http.StatusBadRequest)
		return
	}

	adapter, err := upstream.GetAdapter(body.ServerType)
	if err != nil {
		http.Error(w, "unsupported server_type", http.StatusBadRequest)
		return
	}
	if adapter.AuthType() != "api_key" {
		http.Error(w, "use the OAuth2 flow for this server type", http.StatusBadRequest)
		return
	}

	encrypted, err := h.kmsClient.Encrypt(r.Context(), []byte(body.APIKey))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	cfg, err := h.upstreamStore.CreateUpstreamConfig(r.Context(),
		claims.UserID, body.ServerType, body.ServerURL, "api_key", encrypted)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": cfg.ID}) //nolint:errcheck
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
