package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rayjohnson/mcp-proxy/internal/auth"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type AuthHandler struct {
	users        *store.UserStore
	catalogStore *store.CatalogStore
	suggestions  *store.SuggestionStore
}

func NewAuthHandler(users *store.UserStore, cs *store.CatalogStore, ss *store.SuggestionStore) *AuthHandler {
	return &AuthHandler{users: users, catalogStore: cs, suggestions: ss}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Email == "" || body.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	user, err := h.users.CreateUser(r.Context(), body.Email, hash, "developer")
	if err != nil {
		http.Error(w, "email already registered", http.StatusConflict)
		return
	}

	// Seed the new user's dashboard with suggestions for every active catalog entry.
	// CreateSuggestionForAllUsers targets all developer users (including the new one),
	// and the ON CONFLICT DO NOTHING clause prevents duplicate rows for existing users.
	entries, _ := h.catalogStore.ListActiveCatalogEntries(r.Context())
	for _, e := range entries {
		_ = h.suggestions.CreateSuggestionForAllUsers(r.Context(), e.ID)
	}

	token, err := auth.SignToken(user.ID, user.Role)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, token)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"proxy_token": user.ProxyToken}) //nolint:errcheck
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.users.GetUserByEmail(r.Context(), body.Email)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if user.PasswordHash == nil || !auth.CheckPassword(*user.PasswordHash, body.Password) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.SignToken(user.ID, user.Role)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, token)
	w.WriteHeader(http.StatusOK)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) ProxyEndpoint(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.users.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"proxy_token": user.ProxyToken}) //nolint:errcheck
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}
