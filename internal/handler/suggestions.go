package handler

import (
	"encoding/json"
	"net/http"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type SuggestionHandler struct {
	suggestions store.SuggestionStoreI
}

func NewSuggestionHandler(ss store.SuggestionStoreI) *SuggestionHandler {
	return &SuggestionHandler{suggestions: ss}
}

func (h *SuggestionHandler) ListSuggestions(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	suggestions, err := h.suggestions.ListPendingSuggestionsForUser(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions) //nolint:errcheck
}

func (h *SuggestionHandler) DismissSuggestion(w http.ResponseWriter, r *http.Request) {
	h.resolveSuggestion(w, r, "dismissed")
}

func (h *SuggestionHandler) AcceptSuggestion(w http.ResponseWriter, r *http.Request) {
	h.resolveSuggestion(w, r, "accepted")
}

func (h *SuggestionHandler) resolveSuggestion(w http.ResponseWriter, r *http.Request, status string) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing suggestion id", http.StatusBadRequest)
		return
	}
	if err := h.suggestions.ResolveSuggestion(r.Context(), id, claims.UserID, status); err != nil {
		http.Error(w, "not found or already resolved", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
