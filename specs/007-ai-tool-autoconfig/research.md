# Research: AI Tool Auto-Configuration

## Claude Desktop Config

**Decision**: Read and write `~/Library/Application Support/Claude/claude_desktop_config.json` directly.

**Rationale**: Claude Desktop uses a plain JSON file with a `mcpServers` map. The proxy entry format is `{"command": "npx", "args": ["-y", "mcp-remote", "<url>", "--allow-http"]}`. The file can be safely read, merged, and rewritten by the proxy. Detection: check for `/Applications/Claude.app`.

**Alternatives considered**: Using AppleScript/osascript to trigger Claude Desktop — too fragile and requires UI automation permissions.

---

## Gemini CLI Config

**Decision**: Run `gemini mcp add mcp-proxy <url>` as a subprocess rather than editing a config file.

**Rationale**: Gemini CLI manages its own MCP registry via CLI commands (`gemini mcp add/remove/list`). The `~/.gemini/settings.json` file is internal and not documented as a direct-edit interface. Using the CLI ensures compatibility across Gemini CLI versions.

**Detection**: Check for `gemini` binary in PATH (`/opt/homebrew/bin/gemini`, `/usr/local/bin/gemini`). Run `gemini mcp list` to check if already configured.

**Alternatives considered**: Editing `~/.gemini/settings.json` directly — undocumented format, likely to break across versions.

---

## Detection Strategy

**Decision**: File/binary existence checks at request time (no background polling).

**Rationale**: Tool installation status rarely changes. Checking on page load is sufficient and avoids background goroutines.

- Claude Desktop installed: `/Applications/Claude.app` exists
- Claude Desktop configured: `mcpServers.mcp-proxy` key present in config JSON with matching URL
- Gemini CLI installed: `gemini` resolvable in PATH
- Gemini CLI configured: `gemini mcp list` output contains `mcp-proxy`

---

## Config Write Safety

**Decision**: Atomic write — read existing JSON, merge, write to temp file, rename.

**Rationale**: If the proxy crashes mid-write, the original config must be intact. Temp-file rename is atomic on macOS.

**Alternatives considered**: Backup-and-overwrite — introduces backup file clutter and still leaves a window for corruption.

---

## Scope

Local-mode only. Detection uses macOS paths. Windows paths deferred to a future amendment.
