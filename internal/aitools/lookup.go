package aitools

import (
	"os"
	"os/exec"
	"path/filepath"
)

// commonBinDirs are probed in order when exec.LookPath finds nothing.
// The launchd service runs with a restricted PATH that omits Homebrew and
// user-local installs, so we check known locations explicitly.
var commonBinDirs = []string{
	"/opt/homebrew/bin",  // Homebrew on Apple Silicon
	"/usr/local/bin",     // Homebrew on Intel / manual installs
	"~/.local/bin",       // user-local installs
}

// lookupBinary returns the path to name by trying exec.LookPath first, then
// probing each directory in commonBinDirs. Returns an error if not found.
func lookupBinary(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	for _, dir := range commonBinDirs {
		if dir == "~/.local/bin" {
			dir = filepath.Join(home, ".local", "bin")
		}
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", &os.PathError{Op: "lookupBinary", Path: name, Err: os.ErrNotExist}
}
