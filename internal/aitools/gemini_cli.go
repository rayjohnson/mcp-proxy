package aitools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GeminiCLITool detects and configures Gemini CLI.
// geminiBin may be set in tests to override exec.LookPath.
type GeminiCLITool struct {
	geminiBin string
}

func (t *GeminiCLITool) ID() string { return "gemini-cli" }

func (t *GeminiCLITool) lookupGemini() (string, error) {
	if t.geminiBin != "" {
		if _, err := os.Stat(t.geminiBin); err != nil {
			return "", err
		}
		return t.geminiBin, nil
	}
	return lookupBinary("gemini")
}

func (t *GeminiCLITool) Detect() AITool {
	tool := AITool{ID: t.ID(), DisplayName: "Gemini CLI", InstallURL: "https://github.com/google-gemini/gemini-cli#installation"}

	geminiPath, err := t.lookupGemini()
	if err != nil {
		tool.Status = StatusNotInstalled
		return tool
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, geminiPath, "mcp", "list").Output() //nolint:gosec // geminiPath resolved via LookPath or test override; args are fixed literals
	if err != nil {
		// Installed but mcp list failed — treat as unconfigured rather than error.
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

func (t *GeminiCLITool) Configure(mcpURL string) error {
	geminiPath, err := t.lookupGemini()
	if err != nil {
		return fmt.Errorf("gemini CLI not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, geminiPath, "mcp", "add", "mcp-proxy", mcpURL) //nolint:gosec // geminiPath from LookPath or test override; mcpURL is the proxy's own endpoint
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gemini mcp add failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
