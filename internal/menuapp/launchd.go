//go:build darwin

package menuapp

import (
	"fmt"
	"os"
	"os/exec"
)

const serviceLabel = "io.mcp-proxy.local"

// Start starts the proxy service via launchctl kickstart.
func Start() error {
	uid := os.Getuid()
	cmd := exec.Command("launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/%s", uid, serviceLabel)) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl kickstart: %w: %s", err, out)
	}
	return nil
}

// Stop stops the proxy service via launchctl kill.
func Stop() error {
	uid := os.Getuid()
	cmd := exec.Command("launchctl", "kill", "TERM", fmt.Sprintf("gui/%d/%s", uid, serviceLabel)) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl kill: %w: %s", err, out)
	}
	return nil
}
