package aitools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ClaudeCodeTool detects and configures Claude Code CLI.
// claudeBin may be set in tests to override binary lookup.
type ClaudeCodeTool struct {
	claudeBin string
}

func (t *ClaudeCodeTool) ID() string { return "claude-code" }

func (t *ClaudeCodeTool) lookupClaude() (string, error) {
	if t.claudeBin != "" {
		if _, err := os.Stat(t.claudeBin); err != nil {
			return "", err
		}
		return t.claudeBin, nil
	}
	return lookupBinary("claude")
}

func (t *ClaudeCodeTool) Detect() AITool {
	tool := AITool{ID: t.ID(), DisplayName: "Claude Code CLI", InstallURL: "https://claude.ai/claude-code"}

	claudePath, err := t.lookupClaude()
	if err != nil {
		tool.Status = StatusNotInstalled
		return tool
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, claudePath, "mcp", "list").Output() //nolint:gosec // claudePath resolved via lookupBinary or test override; args are fixed literals
	if err != nil {
		tool.Status = StatusUnconfigured
		return tool
	}

	if strings.Contains(string(out), "mcp-proxy") {
		tool.Status = StatusConfigured
	} else {
		tool.Status = StatusUnconfigured
	}
	return tool
}

func (t *ClaudeCodeTool) Unconfigure() error {
	claudePath, err := t.lookupClaude()
	if err != nil {
		return fmt.Errorf("claude CLI not found: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, claudePath, "mcp", "remove", "mcp-proxy", "--scope", "user") //nolint:gosec // claudePath from lookupBinary or test override; args are fixed literals
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("claude mcp remove failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (t *ClaudeCodeTool) Configure(mcpURL string) error {
	claudePath, err := t.lookupClaude()
	if err != nil {
		return fmt.Errorf("claude CLI not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Remove any existing entry first so add is always idempotent.
	_ = exec.CommandContext(ctx, claudePath, "mcp", "remove", "mcp-proxy", "--scope", "user").Run() //nolint:gosec // claudePath from lookupBinary or test override; args are fixed literals

	cmd := exec.CommandContext(ctx, claudePath, "mcp", "add", "--transport", "http", "--scope", "user", "mcp-proxy", mcpURL) //nolint:gosec // same as above
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("claude mcp add failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
