package mcp

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RouteToolCall parses the prefixed tool name, finds the upstream client session,
// and forwards the call with the original (unprefixed) name.
func RouteToolCall(ctx context.Context, ps *ProxySession, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
	serverType, originalName, err := ParseServerType(req.Params.Name)
	if err != nil {
		return nil, err
	}

	session := ps.GetClient(serverType)
	if session == nil {
		return nil, fmt.Errorf("no upstream connection for server type %q", serverType)
	}

	return session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      originalName,
		Arguments: req.Params.Arguments,
	})
}
