package handler

import (
	"net/http"

	"github.com/rayjohnson/mcp-proxy/internal/oauth2client"
)

type OAuth2Handler struct {
	svc *oauth2client.Service
}

func NewOAuth2Handler(svc *oauth2client.Service) *OAuth2Handler {
	return &OAuth2Handler{svc: svc}
}

func (h *OAuth2Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	serverType := r.PathValue("server_type")
	if serverType == "" {
		http.Error(w, "missing server_type", http.StatusBadRequest)
		return
	}

	authURL, err := h.svc.StartAuthFlow(r.Context(), claims.UserID, serverType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *OAuth2Handler) Callback(w http.ResponseWriter, r *http.Request) {
	serverType := r.PathValue("server_type")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	if err := h.svc.HandleCallback(r.Context(), serverType, code, state); err != nil {
		http.Error(w, "authorization failed", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}
