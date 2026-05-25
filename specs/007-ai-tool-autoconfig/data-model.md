# Data Model: AI Tool Auto-Configuration

## Entities

### AITool

Represents a supported AI application that can be configured to use the proxy.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Stable identifier, e.g. `"claude-desktop"`, `"gemini-cli"` |
| DisplayName | string | Human-readable name shown in the dashboard |
| Status | ToolStatus | Detected state at request time |
| ErrorMessage | string (optional) | Set when Status is `error`; explains what went wrong |

**Validation rules**:
- ID must be non-empty and URL-safe (used as path segment in `/api/tools/{id}/configure`)
- Status is computed at read time; never stored persistently

**State transitions**:

```
not_installed → (tool installed) → unconfigured
unconfigured  → (Configure click) → configured
configured    → (Configure click with new URL) → configured  (idempotent update)
configured    → (tool uninstalled) → not_installed
any           → (error during detection/write) → error
```

---

### ToolStatus

Enumeration of possible states for a detected AI tool.

| Value | Meaning |
|-------|---------|
| `not_installed` | Detection check found no installation evidence |
| `unconfigured` | Tool is installed but proxy is not in its MCP config |
| `configured` | Tool is installed and proxy endpoint is present in its MCP config |
| `error` | Detection or config read failed; ErrorMessage describes the problem |

---

### ToolConfigEntry (internal, not persisted)

The MCP server entry written into a tool's config file. Shape varies per tool.

**Claude Desktop** — written into `~/Library/Application Support/Claude/claude_desktop_config.json` under `mcpServers["mcp-proxy"]`:

```json
{
  "command": "npx",
  "args": ["-y", "mcp-remote", "<proxy-url>/mcp", "--allow-http"]
}
```

**Gemini CLI** — registered via subprocess; no file is written directly by the proxy:

```
gemini mcp add mcp-proxy <proxy-url>/mcp
```

---

## Notes

- No new database tables or persistent storage are introduced.
- The proxy URL written into tool configs is derived at runtime from the current request (scheme + host).
- All detection checks are performed on every request; no caching of ToolStatus between requests.
