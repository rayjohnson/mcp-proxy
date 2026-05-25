package aitools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeDesktopTool detects and configures Claude Desktop.
// appPath and configFile may be set in tests to override the macOS defaults.
type ClaudeDesktopTool struct {
	appPath    string
	configFile string
}

func (t *ClaudeDesktopTool) ID() string { return "claude-desktop" }

func (t *ClaudeDesktopTool) getAppPath() string {
	if t.appPath != "" {
		return t.appPath
	}
	return "/Applications/Claude.app"
}

func (t *ClaudeDesktopTool) getConfigFile() string {
	if t.configFile != "" {
		return t.configFile
	}
	return claudeDesktopConfigPath()
}

func (t *ClaudeDesktopTool) Detect() AITool {
	tool := AITool{ID: t.ID(), DisplayName: "Claude Desktop", InstallURL: "https://claude.ai/download"}

	if _, err := os.Stat(t.getAppPath()); os.IsNotExist(err) {
		tool.Status = StatusNotInstalled
		return tool
	}

	data, err := os.ReadFile(t.getConfigFile())
	if os.IsNotExist(err) {
		tool.Status = StatusUnconfigured
		return tool
	}
	if err != nil {
		tool.Status = StatusError
		tool.ErrorMessage = "cannot read config: " + err.Error()
		return tool
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		tool.Status = StatusError
		tool.ErrorMessage = "config file contains invalid JSON"
		return tool
	}

	mcpServers, _ := cfg["mcpServers"].(map[string]any)
	if _, ok := mcpServers["mcp-proxy"]; ok {
		tool.Status = StatusConfigured
	} else {
		tool.Status = StatusUnconfigured
	}
	return tool
}

func (t *ClaudeDesktopTool) Unconfigure() error {
	configPath := t.getConfigFile()
	data, err := os.ReadFile(configPath) //nolint:gosec // path derived from os.UserHomeDir or test override
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("config file contains invalid JSON: %w", err)
	}
	mcpServers, _ := cfg["mcpServers"].(map[string]any)
	if mcpServers == nil {
		return nil
	}
	delete(mcpServers, "mcp-proxy")
	cfg["mcpServers"] = mcpServers
	return atomicWriteJSON(configPath, cfg)
}

func (t *ClaudeDesktopTool) Configure(mcpURL string) error {
	configPath := t.getConfigFile()

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	existing := map[string]any{}
	data, err := os.ReadFile(configPath) //nolint:gosec // path derived from os.UserHomeDir or test override
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("config file contains invalid JSON: %w", err)
		}
	}

	mcpServers, _ := existing["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = map[string]any{}
	}
	mcpServers["mcp-proxy"] = map[string]any{
		"command": "npx",
		"args":    []string{"-y", "mcp-remote", mcpURL, "--allow-http"},
	}
	existing["mcpServers"] = mcpServers

	return atomicWriteJSON(configPath, existing)
}

func claudeDesktopConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
}
