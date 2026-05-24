package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/rayjohnson/mcp-proxy/internal/auth"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// Minimal interfaces so handlers can be tested without a real database.
type userStore interface {
	CountUsers(ctx context.Context) (int, error)
	CreateUser(ctx context.Context, email, passwordHash, role string) (*store.User, error)
	GetUserByEmail(ctx context.Context, email string) (*store.User, error)
	GetUserByID(ctx context.Context, id string) (*store.User, error)
}

type catalogLister interface {
	ListActiveCatalogEntries(ctx context.Context) ([]*store.CatalogEntry, error)
}

type suggestionSeeder interface {
	CreateSuggestionForAllUsers(ctx context.Context, catalogID string) error
}

type AuthHandler struct {
	users       userStore
	catalog     catalogLister
	suggestions suggestionSeeder
}

func NewAuthHandler(u *store.UserStore, c *store.CatalogStore, s *store.SuggestionStore) *AuthHandler {
	return &AuthHandler{users: u, catalog: c, suggestions: s}
}

// Register handles POST /api/auth/register.
// Uses Post-Redirect-Get: success → 303 /dashboard, error → 303 /register?error=...
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.registerError(w, r, "Invalid form data.")
		return
	}
	email := r.FormValue("email")
	password := r.FormValue("password")
	if email == "" || password == "" {
		h.registerError(w, r, "Email and password are required.")
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		h.registerError(w, r, "Internal error, please try again.")
		return
	}

	// First registered user becomes admin so there's always a way to manage the system.
	count, err := h.users.CountUsers(r.Context())
	if err != nil {
		h.registerError(w, r, "Registration failed, please try again.")
		return
	}
	role := "developer"
	if count == 0 {
		role = "admin"
	}

	user, err := h.users.CreateUser(r.Context(), email, hash, role)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateEmail) {
			h.registerError(w, r, "That email is already registered.")
		} else {
			h.registerError(w, r, "Registration failed, please try again.")
		}
		return
	}

	entries, _ := h.catalog.ListActiveCatalogEntries(r.Context())
	for _, e := range entries {
		_ = h.suggestions.CreateSuggestionForAllUsers(r.Context(), e.ID)
	}

	token, err := auth.SignToken(user.ID, user.Role)
	if err != nil {
		h.registerError(w, r, "Internal error, please try again.")
		return
	}

	setSessionCookie(w, r, token)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *AuthHandler) registerError(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/register?error="+url.QueryEscape(msg), http.StatusSeeOther)
}

// Login handles POST /api/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.loginError(w, r, "Invalid form data.")
		return
	}
	email := r.FormValue("email")
	password := r.FormValue("password")

	user, err := h.users.GetUserByEmail(r.Context(), email)
	if err != nil || user.PasswordHash == nil || !auth.CheckPassword(*user.PasswordHash, password) {
		h.loginError(w, r, "Invalid email or password.")
		return
	}

	token, err := auth.SignToken(user.ID, user.Role)
	if err != nil {
		h.loginError(w, r, "Internal error, please try again.")
		return
	}

	setSessionCookie(w, r, token)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (h *AuthHandler) loginError(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, "/login?error="+url.QueryEscape(msg), http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // clearing cookie: Secure omitted intentionally (no value to protect)
		Name:     "session",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
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

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is set dynamically based on request scheme
		Name:     "session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}
