package menuapp

import (
	webview "github.com/webview/webview_go"
)

var activePrefs webview.WebView

// OpenPreferences opens the Preferences window showing the proxy dashboard.
// If a window is already open, it is brought to the foreground.
func OpenPreferences(port string) {
	if activePrefs != nil {
		// Window already open — bring it to front by running an empty dispatch.
		activePrefs.Dispatch(func() {})
		return
	}

	wv := webview.New(false)
	wv.SetTitle("mcp-proxy — Preferences")
	wv.SetSize(960, 700, webview.HintNone)
	wv.Navigate("http://localhost:" + port + "/dashboard")
	activePrefs = wv
	wv.Run()
	wv.Destroy()
	activePrefs = nil
}
