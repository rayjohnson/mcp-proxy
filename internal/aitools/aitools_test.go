package aitools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// atomicWriteJSON
// ---------------------------------------------------------------------------

func TestAtomicWriteJSON_WritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	if err := atomicWriteJSON(path, map[string]string{"k": "v"}); err != nil {
		t.Fatalf("atomicWriteJSON: %v", err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is from t.TempDir(), not user input
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["k"] != "v" {
		t.Errorf("got[k] = %q, want %q", got["k"], "v")
	}
}

func TestAtomicWriteJSON_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	_ = atomicWriteJSON(path, map[string]string{"v": "1"})
	_ = atomicWriteJSON(path, map[string]string{"v": "2"})

	data, _ := os.ReadFile(path) //nolint:gosec // path is from t.TempDir(), not user input
	var got map[string]string
	_ = json.Unmarshal(data, &got)
	if got["v"] != "2" {
		t.Errorf("got[v] = %q, want %q", got["v"], "2")
	}
}

func TestAtomicWriteJSON_MissingParentDir(t *testing.T) {
	err := atomicWriteJSON("/nonexistent/dir/out.json", map[string]string{})
	if err == nil {
		t.Error("expected error for missing parent directory")
	}
}

func TestAtomicWriteJSON_LeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	_ = atomicWriteJSON(path, map[string]any{"x": 1})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("got %d files in dir, want 1 (no temp file residue)", len(entries))
	}
}

// ---------------------------------------------------------------------------
// ClaudeDesktopTool helpers
// ---------------------------------------------------------------------------

// newClaudeTest builds a ClaudeDesktopTool wired to paths inside a temp dir.
// It creates the fake Claude.app stub if createApp is true.
func newClaudeTest(t *testing.T, createApp bool) (*ClaudeDesktopTool, string) {
	t.Helper()
	dir := t.TempDir()
	appPath := filepath.Join(dir, "Claude.app")
	configFile := filepath.Join(dir, "claude_desktop_config.json")
	if createApp {
		if err := os.Mkdir(appPath, 0o750); err != nil {
			t.Fatalf("mkdir Claude.app: %v", err)
		}
	}
	tool := &ClaudeDesktopTool{appPath: appPath, configFile: configFile}
	return tool, configFile
}

// ---------------------------------------------------------------------------
// ClaudeDesktopTool.Detect
// ---------------------------------------------------------------------------

func TestClaudeDetect_NotInstalled(t *testing.T) {
	tool, _ := newClaudeTest(t, false) // no Claude.app
	got := tool.Detect()
	if got.Status != StatusNotInstalled {
		t.Errorf("status = %q, want %q", got.Status, StatusNotInstalled)
	}
}

func TestClaudeDetect_Unconfigured_NoConfigFile(t *testing.T) {
	tool, _ := newClaudeTest(t, true) // Claude.app exists, no config file
	got := tool.Detect()
	if got.Status != StatusUnconfigured {
		t.Errorf("status = %q, want %q", got.Status, StatusUnconfigured)
	}
}

func TestClaudeDetect_Unconfigured_EmptyMCPServers(t *testing.T) {
	tool, cfgPath := newClaudeTest(t, true)
	writeJSON(t, cfgPath, map[string]any{"mcpServers": map[string]any{}})

	got := tool.Detect()
	if got.Status != StatusUnconfigured {
		t.Errorf("status = %q, want %q", got.Status, StatusUnconfigured)
	}
}

func TestClaudeDetect_Unconfigured_OtherServer(t *testing.T) {
	tool, cfgPath := newClaudeTest(t, true)
	writeJSON(t, cfgPath, map[string]any{
		"mcpServers": map[string]any{
			"other-server": map[string]any{"command": "npx"},
		},
	})

	got := tool.Detect()
	if got.Status != StatusUnconfigured {
		t.Errorf("status = %q, want %q", got.Status, StatusUnconfigured)
	}
}

func TestClaudeDetect_Configured(t *testing.T) {
	tool, cfgPath := newClaudeTest(t, true)
	writeJSON(t, cfgPath, map[string]any{
		"mcpServers": map[string]any{
			"mcp-proxy": map[string]any{"command": "npx"},
		},
	})

	got := tool.Detect()
	if got.Status != StatusConfigured {
		t.Errorf("status = %q, want %q", got.Status, StatusConfigured)
	}
}

func TestClaudeDetect_InvalidJSON(t *testing.T) {
	tool, cfgPath := newClaudeTest(t, true)
	if err := os.WriteFile(cfgPath, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := tool.Detect()
	if got.Status != StatusError {
		t.Errorf("status = %q, want %q", got.Status, StatusError)
	}
	if got.ErrorMessage == "" {
		t.Error("expected non-empty ErrorMessage for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// ClaudeDesktopTool.Configure
// ---------------------------------------------------------------------------

func TestClaudeConfigure_FreshConfig(t *testing.T) {
	tool, cfgPath := newClaudeTest(t, true)

	if err := tool.Configure("http://localhost:9753/mcp/tok123"); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	cfg := readJSONFile(t, cfgPath)
	entry := mcpProxyEntry(t, cfg)
	if entry["command"] != "npx" {
		t.Errorf("command = %q, want %q", entry["command"], "npx")
	}
	args, _ := entry["args"].([]any)
	if len(args) != 4 {
		t.Fatalf("args len = %d, want 4; args = %v", len(args), args)
	}
	if args[2] != "http://localhost:9753/mcp/tok123" {
		t.Errorf("args[2] = %q, want proxy URL", args[2])
	}
	if args[3] != "--allow-http" {
		t.Errorf("args[3] = %q, want --allow-http", args[3])
	}
}

func TestClaudeConfigure_PreservesOtherServers(t *testing.T) {
	tool, cfgPath := newClaudeTest(t, true)
	writeJSON(t, cfgPath, map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"command": "other-cmd"},
		},
	})

	if err := tool.Configure("http://localhost:9753/mcp/tok"); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	cfg := readJSONFile(t, cfgPath)
	servers, _ := cfg["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("existing 'other' server was removed; Configure must preserve it")
	}
	if _, ok := servers["mcp-proxy"]; !ok {
		t.Error("mcp-proxy entry not written")
	}
}

func TestClaudeConfigure_Idempotent(t *testing.T) {
	tool, cfgPath := newClaudeTest(t, true)

	if err := tool.Configure("http://localhost:9753/mcp/tok"); err != nil {
		t.Fatalf("first Configure: %v", err)
	}
	if err := tool.Configure("http://localhost:9753/mcp/tok-new"); err != nil {
		t.Fatalf("second Configure: %v", err)
	}

	cfg := readJSONFile(t, cfgPath)
	servers, _ := cfg["mcpServers"].(map[string]any)
	if n := len(servers); n != 1 {
		t.Errorf("mcpServers len = %d, want 1 (no duplicates)", n)
	}
	entry := mcpProxyEntry(t, cfg)
	args, _ := entry["args"].([]any)
	if len(args) > 2 && args[2] != "http://localhost:9753/mcp/tok-new" {
		t.Errorf("URL not updated on second call; got %v", args[2])
	}
}

func TestClaudeConfigure_CreatesConfigDir(t *testing.T) {
	dir := t.TempDir()
	appPath := filepath.Join(dir, "Claude.app")
	_ = os.Mkdir(appPath, 0o750)
	// Config file is inside a subdirectory that does not yet exist.
	configFile := filepath.Join(dir, "subdir", "claude_desktop_config.json")
	tool := &ClaudeDesktopTool{appPath: appPath, configFile: configFile}

	if err := tool.Configure("http://localhost:9753/mcp/tok"); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if _, err := os.Stat(configFile); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestClaudeConfigure_InvalidExistingJSON(t *testing.T) {
	tool, cfgPath := newClaudeTest(t, true)
	if err := os.WriteFile(cfgPath, []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	original, _ := os.ReadFile(cfgPath) //nolint:gosec // path is from t.TempDir(), not user input

	err := tool.Configure("http://localhost:9753/mcp/tok")
	if err == nil {
		t.Fatal("expected error for invalid existing JSON")
	}

	// Original file must be unchanged.
	after, _ := os.ReadFile(cfgPath) //nolint:gosec // path is from t.TempDir(), not user input
	if string(after) != string(original) {
		t.Error("config file was modified despite Configure failing")
	}
}

func TestClaudeDetect_InstallURLAlwaysSet(t *testing.T) {
	cases := []struct {
		name      string
		createApp bool
	}{
		{"not_installed", false},
		{"unconfigured", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool, _ := newClaudeTest(t, tc.createApp)
			got := tool.Detect()
			if got.InstallURL == "" {
				t.Errorf("InstallURL is empty for status %q", got.Status)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GeminiCLITool helpers
// ---------------------------------------------------------------------------

// fakeBin creates an executable shell script in a temp dir and returns its path.
func fakeBin(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "gemini")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil { //nolint:gosec // 0755 required so exec can run the script
		t.Fatalf("write fake gemini: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// GeminiCLITool.Detect
// ---------------------------------------------------------------------------

func TestGeminiDetect_NotInstalled(t *testing.T) {
	tool := &GeminiCLITool{geminiBin: "/nonexistent/gemini-binary"}
	got := tool.Detect()
	if got.Status != StatusNotInstalled {
		t.Errorf("status = %q, want %q", got.Status, StatusNotInstalled)
	}
}

func TestGeminiDetect_Unconfigured(t *testing.T) {
	bin := fakeBin(t, `echo "no servers listed"`)
	tool := &GeminiCLITool{geminiBin: bin}
	got := tool.Detect()
	if got.Status != StatusUnconfigured {
		t.Errorf("status = %q, want %q", got.Status, StatusUnconfigured)
	}
}

func TestGeminiDetect_Configured(t *testing.T) {
	bin := fakeBin(t, `echo "  mcp-proxy  http://localhost:9753/mcp/tok"`)
	tool := &GeminiCLITool{geminiBin: bin}
	got := tool.Detect()
	if got.Status != StatusConfigured {
		t.Errorf("status = %q, want %q", got.Status, StatusConfigured)
	}
}

func TestGeminiDetect_MCPListFails_TreatedAsUnconfigured(t *testing.T) {
	bin := fakeBin(t, `exit 1`)
	tool := &GeminiCLITool{geminiBin: bin}
	got := tool.Detect()
	// A broken mcp list should not surface as StatusError — just StatusUnconfigured.
	if got.Status != StatusUnconfigured {
		t.Errorf("status = %q, want %q", got.Status, StatusUnconfigured)
	}
}

func TestGeminiDetect_InstallURLAlwaysSet(t *testing.T) {
	cases := []struct {
		name   string
		script string
	}{
		{"not_installed", ""},  // geminiBin set to nonexistent path below
		{"unconfigured", `echo "no servers"`},
		{"configured", `echo "  mcp-proxy  http://localhost/mcp/tok"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var tool *GeminiCLITool
			if tc.name == "not_installed" {
				tool = &GeminiCLITool{geminiBin: "/nonexistent/gemini-binary"}
			} else {
				tool = &GeminiCLITool{geminiBin: fakeBin(t, tc.script)}
			}
			got := tool.Detect()
			if got.InstallURL == "" {
				t.Errorf("InstallURL is empty for status %q", got.Status)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GeminiCLITool.Configure
// ---------------------------------------------------------------------------

func TestGeminiConfigure_Success(t *testing.T) {
	bin := fakeBin(t, `exit 0`)
	tool := &GeminiCLITool{geminiBin: bin}
	if err := tool.Configure("http://localhost:9753/mcp/tok"); err != nil {
		t.Errorf("Configure: %v", err)
	}
}

func TestGeminiConfigure_CommandFails(t *testing.T) {
	bin := fakeBin(t, `echo "permission denied" >&2; exit 1`)
	tool := &GeminiCLITool{geminiBin: bin}
	err := tool.Configure("http://localhost:9753/mcp/tok")
	if err == nil {
		t.Fatal("expected error when gemini mcp add exits non-zero")
	}
	if !strings.Contains(err.Error(), "gemini mcp add failed") {
		t.Errorf("error message = %q, want to contain 'gemini mcp add failed'", err.Error())
	}
}

func TestGeminiConfigure_NotInstalled(t *testing.T) {
	tool := &GeminiCLITool{geminiBin: "/nonexistent/gemini-binary"}
	err := tool.Configure("http://localhost:9753/mcp/tok")
	if err == nil {
		t.Fatal("expected error when gemini binary missing")
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // path is from t.TempDir(), not user input
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal %s: %v", path, err)
	}
	return out
}

func mcpProxyEntry(t *testing.T, cfg map[string]any) map[string]any {
	t.Helper()
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing or wrong type in config")
	}
	entry, ok := servers["mcp-proxy"].(map[string]any)
	if !ok {
		t.Fatalf("mcp-proxy entry missing or wrong type")
	}
	return entry
}

