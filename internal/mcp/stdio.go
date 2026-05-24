package mcp

import (
	"context"
	"fmt"
	"os/exec"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// ConnectStdio launches the stdio MCP server described by entry and returns an
// UpstreamClient backed by the child process. The process is started fresh for
// each call; the caller is responsible for cancelling ctx to terminate it.
func ConnectStdio(ctx context.Context, entry *store.CatalogEntry) (*UpstreamClient, error) {
	if entry.Command == nil || *entry.Command == "" {
		return nil, fmt.Errorf("stdio catalog entry %q has no command", entry.ServerType)
	}

	//nolint:gosec // command comes from admin-configured catalog, not user input
	cmd := exec.CommandContext(ctx, *entry.Command, entry.Args...)

	// Merge catalog env overrides into child process environment.
	if len(entry.Env) > 0 {
		// cmd.Env == nil means inherit from parent; we append overrides on top.
		for k, v := range entry.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	transport := &sdkmcp.CommandTransport{Command: cmd}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "mcp-proxy", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect stdio server %q: %w", entry.ServerType, err)
	}
	return &UpstreamClient{Session: session, DetectedTransport: TransportStdio}, nil
}
