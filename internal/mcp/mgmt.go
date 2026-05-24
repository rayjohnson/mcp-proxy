package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// ManagementDeps holds dependencies needed by the proxy management tools.
type ManagementDeps struct {
	UpstreamStore store.UpstreamStoreI
	CatalogStore  store.CatalogStoreI
	KMSEncrypt    func(ctx context.Context, plaintext []byte) ([]byte, error)
}

func textResult(text string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}
}

func errResult(msg string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: msg}},
		IsError: true,
	}
}

func stringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func parseArgs(req *sdkmcp.CallToolRequest) (map[string]any, error) {
	if req.Params.Arguments == nil {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return args, nil
}

// registerManagementTools adds the five proxy management tools to the MCP server.
func registerManagementTools(server *sdkmcp.Server, ps *ProxySession, deps ManagementDeps) {
	server.AddTool(&sdkmcp.Tool{
		Name:        "proxy_list_catalog",
		Description: "List available upstream MCP servers that can be connected to.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return handleListCatalog(ctx, deps)
	})

	server.AddTool(&sdkmcp.Tool{
		Name:        "proxy_connect_upstream",
		Description: "Connect to an upstream MCP server using an API key or PAT.",
		InputSchema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"catalog_id", "api_key"},
			Properties: map[string]*jsonschema.Schema{
				"catalog_id": {Type: "string", Description: "Catalog entry ID from proxy_list_catalog"},
				"api_key":    {Type: "string", Description: "API key or personal access token for the service"},
			},
		},
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return handleConnectUpstream(ctx, ps, deps, req)
	})

	server.AddTool(&sdkmcp.Tool{
		Name:        "proxy_list_upstreams",
		Description: "List your currently connected upstream MCP servers.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return handleListUpstreams(ctx, ps, deps)
	})

	server.AddTool(&sdkmcp.Tool{
		Name:        "proxy_disconnect_upstream",
		Description: "Disconnect a connected upstream MCP server.",
		InputSchema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"upstream_id"},
			Properties: map[string]*jsonschema.Schema{
				"upstream_id": {Type: "string", Description: "Upstream ID from proxy_list_upstreams"},
			},
		},
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return handleDisconnectUpstream(ctx, ps, deps, req)
	})

	server.AddTool(&sdkmcp.Tool{
		Name:        "proxy_update_credentials",
		Description: "Update the API key for a connected upstream.",
		InputSchema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"upstream_id", "api_key"},
			Properties: map[string]*jsonschema.Schema{
				"upstream_id": {Type: "string", Description: "Upstream ID from proxy_list_upstreams"},
				"api_key":     {Type: "string", Description: "New API key or personal access token"},
			},
		},
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return handleUpdateCredentials(ctx, ps, deps, req)
	})
}

func handleListCatalog(ctx context.Context, deps ManagementDeps) (*sdkmcp.CallToolResult, error) {
	entries, err := deps.CatalogStore.ListActiveCatalogEntries(ctx)
	if err != nil {
		return errResult("Internal error: could not load catalog"), nil
	}

	type catalogItem struct {
		ID            string  `json:"id"`
		DisplayName   string  `json:"display_name"`
		Description   *string `json:"description,omitempty"`
		ServerType    string  `json:"server_type"`
		AuthType      string  `json:"auth_type"`
		RequiresOAuth bool    `json:"requires_oauth"`
	}

	items := make([]catalogItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, catalogItem{
			ID:            e.ID,
			DisplayName:   e.DisplayName,
			Description:   e.Description,
			ServerType:    e.ServerType,
			AuthType:      e.AuthType,
			RequiresOAuth: e.AuthType == "oauth2",
		})
	}

	out, err := json.Marshal(items)
	if err != nil {
		return errResult("Internal error: could not encode catalog"), nil
	}
	return textResult(string(out)), nil
}

func handleConnectUpstream(ctx context.Context, ps *ProxySession, deps ManagementDeps, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
	args, err := parseArgs(req)
	if err != nil {
		return errResult(err.Error()), nil
	}

	catalogID := strings.TrimSpace(stringArg(args, "catalog_id"))
	apiKey := strings.TrimSpace(stringArg(args, "api_key"))

	if catalogID == "" {
		return errResult("catalog_id must not be empty"), nil
	}
	if apiKey == "" {
		return errResult("api_key must not be empty"), nil
	}

	entry, err := deps.CatalogStore.GetCatalogEntryByID(ctx, catalogID)
	if err != nil {
		return errResult(fmt.Sprintf("Unknown catalog entry: %s", catalogID)), nil
	}

	switch {
	case entry.AuthType == "oauth2":
		return errResult("This server requires OAuth2. Complete the connection at /dashboard"), nil
	case entry.AuthType == "none", entry.Transport == "stdio":
		return errResult("This server requires no credentials and is connected automatically"), nil
	}

	encrypted, err := deps.KMSEncrypt(ctx, []byte(apiKey))
	if err != nil {
		return errResult("Internal error: could not secure credentials"), nil
	}

	cfg, err := deps.UpstreamStore.CreateUpstreamConfig(ctx,
		ps.UserID, entry.ServerType, entry.ServerURL, entry.AuthType, encrypted)
	if err != nil {
		return errResult("Internal error: could not save upstream connection"), nil
	}

	type connectResult struct {
		ID         string `json:"id"`
		ServerType string `json:"server_type"`
		Status     string `json:"status"`
	}
	out, _ := json.Marshal(connectResult{ID: cfg.ID, ServerType: cfg.ServerType, Status: cfg.Status})
	return textResult(string(out)), nil
}

func handleListUpstreams(ctx context.Context, ps *ProxySession, deps ManagementDeps) (*sdkmcp.CallToolResult, error) {
	configs, err := deps.UpstreamStore.GetUpstreamConfigsByUserID(ctx, ps.UserID)
	if err != nil {
		return errResult("Internal error: could not load upstreams"), nil
	}

	nameMap := make(map[string]string)
	if entries, err := deps.CatalogStore.ListActiveCatalogEntries(ctx); err == nil {
		for _, e := range entries {
			nameMap[e.ServerType] = e.DisplayName
		}
	}

	type upstreamItem struct {
		ID                string  `json:"id"`
		DisplayName       string  `json:"display_name"`
		ServerType        string  `json:"server_type"`
		AuthType          string  `json:"auth_type"`
		Status            string  `json:"status"`
		DetectedTransport *string `json:"detected_transport,omitempty"`
	}

	items := make([]upstreamItem, 0, len(configs))
	for _, c := range configs {
		name := nameMap[c.ServerType]
		if name == "" {
			name = c.ServerType
		}
		items = append(items, upstreamItem{
			ID:                c.ID,
			DisplayName:       name,
			ServerType:        c.ServerType,
			AuthType:          c.AuthType,
			Status:            c.Status,
			DetectedTransport: c.DetectedTransport,
		})
	}

	out, err := json.Marshal(items)
	if err != nil {
		return errResult("Internal error: could not encode upstreams"), nil
	}
	return textResult(string(out)), nil
}

func handleDisconnectUpstream(ctx context.Context, ps *ProxySession, deps ManagementDeps, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
	args, err := parseArgs(req)
	if err != nil {
		return errResult(err.Error()), nil
	}

	upstreamID := strings.TrimSpace(stringArg(args, "upstream_id"))
	if upstreamID == "" {
		return errResult("upstream_id must not be empty"), nil
	}

	cfg, err := deps.UpstreamStore.GetUpstreamConfigByID(ctx, upstreamID)
	if err != nil || cfg.UserID != ps.UserID {
		return errResult("Upstream not found"), nil
	}

	if err := deps.UpstreamStore.DeleteUpstreamConfig(ctx, upstreamID); err != nil {
		return errResult("Internal error: could not remove upstream"), nil
	}
	return textResult("Disconnected successfully."), nil
}

func handleUpdateCredentials(ctx context.Context, ps *ProxySession, deps ManagementDeps, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
	args, err := parseArgs(req)
	if err != nil {
		return errResult(err.Error()), nil
	}

	upstreamID := strings.TrimSpace(stringArg(args, "upstream_id"))
	apiKey := strings.TrimSpace(stringArg(args, "api_key"))

	if upstreamID == "" {
		return errResult("upstream_id must not be empty"), nil
	}
	if apiKey == "" {
		return errResult("api_key must not be empty"), nil
	}

	cfg, err := deps.UpstreamStore.GetUpstreamConfigByID(ctx, upstreamID)
	if err != nil || cfg.UserID != ps.UserID {
		return errResult("Upstream not found"), nil
	}

	encrypted, err := deps.KMSEncrypt(ctx, []byte(apiKey))
	if err != nil {
		return errResult("Internal error: could not secure credentials"), nil
	}

	if err := deps.UpstreamStore.UpdateEncryptedCreds(ctx, upstreamID, encrypted); err != nil {
		return errResult("Internal error: could not update credentials"), nil
	}
	if err := deps.UpstreamStore.UpdateUpstreamStatus(ctx, upstreamID, "active"); err != nil {
		return errResult("Internal error: could not update status"), nil
	}
	return textResult("Credentials updated. Status reset to active."), nil
}
