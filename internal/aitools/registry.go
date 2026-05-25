package aitools

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ToolStatus is the detected state of a supported AI tool.
type ToolStatus string

const (
	StatusNotInstalled ToolStatus = "not_installed"
	StatusUnconfigured ToolStatus = "unconfigured"
	StatusConfigured   ToolStatus = "configured"
	StatusError        ToolStatus = "error"
)

// AITool describes a supported AI application and its detected state.
type AITool struct {
	ID           string     `json:"id"`
	DisplayName  string     `json:"display_name"`
	Status       ToolStatus `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

// Configurer detects and configures a supported AI tool to use the proxy.
type Configurer interface {
	ID() string
	Detect() AITool
	Configure(mcpURL string) error
}

// atomicWriteJSON marshals v to JSON and writes it atomically to path via
// temp-file rename, leaving the original file untouched on any error.
func atomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mcp-proxy-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName) // no-op after successful rename
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
