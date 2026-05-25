package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rayjohnson/mcp-proxy/internal/aitools"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type aitoolsUserStore interface {
	GetUserByID(context.Context, string) (*store.User, error)
}

// AIToolsHandler serves GET /api/tools and POST /api/tools/{id}/configure.
// It is only active in local mode.
type AIToolsHandler struct {
	tools     []aitools.Configurer
	userStore aitoolsUserStore
	baseURL   string
	localMode bool
}

func NewAIToolsHandler(tools []aitools.Configurer, us aitoolsUserStore, baseURL string, localMode bool) *AIToolsHandler {
	return &AIToolsHandler{tools: tools, userStore: us, baseURL: baseURL, localMode: localMode}
}

// proxyURLFromRequest builds the base URL from the incoming request when
// baseURL is not set (e.g. first-run with no config).
func proxyURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// StatusAPI handles GET /api/tools.
func (h *AIToolsHandler) StatusAPI(w http.ResponseWriter, r *http.Request) {
	if !h.localMode {
		writeJSONError(w, "auto-configuration is only available in local mode", http.StatusServiceUnavailable)
		return
	}
	results := make([]aitools.AITool, len(h.tools))
	for i, t := range h.tools {
		results[i] = t.Detect()
	}
	writeJSON(w, http.StatusOK, results)
}

// ConfigureAPI handles POST /api/tools/{id}/configure.
func (h *AIToolsHandler) ConfigureAPI(w http.ResponseWriter, r *http.Request) {
	if !h.localMode {
		writeJSONError(w, "auto-configuration is only available in local mode", http.StatusServiceUnavailable)
		return
	}

	id := r.PathValue("id")
	var found aitools.Configurer
	for _, t := range h.tools {
		if t.ID() == id {
			found = t
			break
		}
	}
	if found == nil {
		writeJSONError(w, "unknown tool: "+id, http.StatusBadRequest)
		return
	}

	current := found.Detect()
	if current.Status == aitools.StatusNotInstalled {
		writeJSONError(w, id+" is not installed", http.StatusNotFound)
		return
	}

	claims := ClaimsFromContext(r.Context())
	user, err := h.userStore.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeJSONError(w, "failed to get user", http.StatusInternalServerError)
		return
	}

	base := h.baseURL
	if base == "" {
		base = proxyURLFromRequest(r)
	}
	mcpURL := base + "/mcp/" + user.ProxyToken

	if err := found.Configure(mcpURL); err != nil {
		writeJSONError(w, "failed to write config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	updated := found.Detect()
	writeJSON(w, http.StatusOK, updated)
}
