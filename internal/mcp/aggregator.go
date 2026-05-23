package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// PrefixedTool combines an upstream tool with its server-type prefix.
type PrefixedTool struct {
	ServerType   string
	OriginalName string
	Tool         *sdkmcp.Tool
}

// PrefixedName returns the tool name as the AI tool sees it.
func (p *PrefixedTool) PrefixedName() string {
	return p.ServerType + "__" + p.OriginalName
}

// AggregateTools collects and prefixes tools from all live upstream sessions.
// Upstream errors are logged but do not fail the whole list.
func AggregateTools(ctx context.Context, clients map[string]*sdkmcp.ClientSession) ([]*PrefixedTool, error) {
	var all []*PrefixedTool

	for serverType, session := range clients {
		result, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
		if err != nil {
			slog.Warn("list tools failed", "server_type", serverType, "err", err)
			continue
		}
		for _, t := range result.Tools {
			tool := t // capture
			all = append(all, &PrefixedTool{
				ServerType:   serverType,
				OriginalName: tool.Name,
				Tool: &sdkmcp.Tool{
					Name:        serverType + "__" + tool.Name,
					Description: tool.Description,
					InputSchema: tool.InputSchema,
				},
			})
		}
	}

	return all, nil
}

// ParseServerType extracts the server_type from a prefixed tool name.
// e.g. "github__search_repos" → "github", "search_repos"
func ParseServerType(prefixedName string) (serverType, originalName string, err error) {
	idx := strings.Index(prefixedName, "__")
	if idx < 0 {
		return "", "", fmt.Errorf("tool name %q has no server type prefix", prefixedName)
	}
	return prefixedName[:idx], prefixedName[idx+2:], nil
}
