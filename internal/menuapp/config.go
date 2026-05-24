package menuapp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type installConfig struct {
	Port string `json:"port"`
}

// ReadConfig reads the installed port from config.json.
// Returns "9753" if the file is absent or malformed.
func ReadConfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "9753"
	}
	data, err := os.ReadFile(filepath.Join(home, "Library", "Application Support", "mcp-proxy", "config.json")) //nolint:gosec
	if err != nil {
		return "9753"
	}
	var cfg installConfig
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.Port == "" {
		return "9753"
	}
	return cfg.Port
}
