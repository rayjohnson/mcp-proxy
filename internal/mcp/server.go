package mcp

import (
	"context"
	"log/slog"
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// ProxyServerDeps holds everything needed to look up users and open upstream sessions.
type ProxyServerDeps struct {
	UserStore   store.UserStoreI
	SessionDeps SessionDeps
}

// GetServerFunc returns a callback suitable for mcp.NewStreamableHTTPHandler.
// It is invoked once per new MCP session: it validates the proxy token, opens
// upstream connections for that user, and registers all aggregated tools.
// Returning nil causes the SDK to respond with 400 Bad Request.
func GetServerFunc(deps ProxyServerDeps) func(*http.Request) *sdkmcp.Server {
	return func(r *http.Request) *sdkmcp.Server {
		token := r.PathValue("token")
		if token == "" {
			return nil
		}

		user, err := deps.UserStore.GetUserByProxyToken(r.Context(), token)
		if err != nil {
			slog.Warn("proxy token lookup failed", "err", err)
			return nil
		}

		// Use a detached context so the upstream session outlives this request.
		ctx := context.Background()

		ps, err := OpenSession(ctx, user.ID, deps.SessionDeps)
		if err != nil {
			slog.Warn("open session failed", "user_id", user.ID, "err", err)
			return nil
		}

		return buildMCPServer(ctx, ps)
	}
}

// buildMCPServer creates an mcp.Server populated with all aggregated upstream tools.
func buildMCPServer(ctx context.Context, ps *ProxySession) *sdkmcp.Server {
	server := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "mcp-proxy", Version: "1.0"},
		nil,
	)

	tools, err := AggregateTools(ctx, ps.AllClients())
	if err != nil {
		slog.Warn("aggregate tools failed", "err", err)
		return server
	}

	for _, pt := range tools {
		server.AddTool(pt.Tool, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return RouteToolCall(ctx, ps, req)
		})
	}

	return server
}
