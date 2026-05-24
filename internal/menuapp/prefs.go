//go:build darwin

package menuapp

import (
	"log/slog"
	"os/exec"
)

// OpenPreferences opens the proxy dashboard in the default browser.
// A native WebKit window cannot be used here because systray already occupies
// the main thread with its NSRunLoop, and NSWindow requires the main thread.
func OpenPreferences(port string) {
	if err := exec.Command("open", "http://localhost:"+port+"/dashboard").Start(); err != nil { //nolint:gosec
		slog.Warn("open preferences failed", "err", err)
	}
}
