//go:build darwin

package menuapp

import (
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/getlantern/systray"
)

// Run starts the systray event loop. This must be called from main().
func Run() {
	systray.Run(onReady, onExit)
}

func onReady() {
	port := ReadConfig()
	stateCh := StartPoller(port)

	systray.SetIcon(iconStopped)
	systray.SetTooltip("mcp-proxy: Checking…")

	mStatus := systray.AddMenuItem("– Checking…", "")
	mStatus.Disable()
	systray.AddSeparator()

	mToggle := systray.AddMenuItem("Start", "Start the proxy service")
	systray.AddSeparator()

	mPrefs := systray.AddMenuItem("Preferences…", "Open the Preferences window")
	mDash := systray.AddMenuItem("Open Dashboard", "Open the proxy dashboard in your browser")
	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "Quit mcp-proxy menu bar app")

	go func() {
		current := StateUnknown
		for {
			select {
			case state := <-stateCh:
				current = state
				switch state {
				case StateRunning:
					systray.SetIcon(iconRunning)
					systray.SetTooltip("mcp-proxy: Running")
					mStatus.SetTitle("● Running")
					mToggle.SetTitle("Stop")
					mToggle.Enable()
					mPrefs.Enable()
				case StateStopped:
					systray.SetIcon(iconStopped)
					systray.SetTooltip("mcp-proxy: Stopped")
					mStatus.SetTitle("○ Stopped")
					mToggle.SetTitle("Start")
					mToggle.Enable()
					mPrefs.Disable()
				case StateUnknown:
					systray.SetIcon(iconStopped)
					systray.SetTooltip("mcp-proxy: Checking…")
					mStatus.SetTitle("– Checking…")
					mToggle.Disable()
				}

			case <-mToggle.ClickedCh:
				switch current {
				case StateRunning:
					mToggle.SetTitle("Stopping…")
					mToggle.Disable()
					if err := Stop(); err != nil {
						slog.Warn("stop failed", "err", err)
					}
				case StateStopped:
					mToggle.SetTitle("Starting…")
					mToggle.Disable()
					if err := Start(); err != nil {
						slog.Warn("start failed", "err", err)
					}
					go notifyIfNotStarted(stateCh)
				}

			case <-mPrefs.ClickedCh:
				go OpenPreferences(port)

			case <-mDash.ClickedCh:
				go func() {
					if err := exec.Command("open", "http://localhost:"+port+"/dashboard").Start(); err != nil { //nolint:gosec
						slog.Warn("open dashboard failed", "err", err)
					}
				}()

			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

// notifyIfNotStarted posts a macOS notification if the service doesn't reach
// StateRunning within 10 seconds of a start attempt.
func notifyIfNotStarted(stateCh <-chan ServiceState) {
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case <-deadline.C:
			msg := fmt.Sprintf("display notification %q with title %q",
				"mcp-proxy failed to start.", "mcp-proxy")
			if err := exec.Command("osascript", "-e", msg).Run(); err != nil { //nolint:gosec
				slog.Warn("osascript notification failed", "err", err)
			}
			return
		case state := <-stateCh:
			if state == StateRunning {
				return
			}
		}
	}
}

func onExit() {}
