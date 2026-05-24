# Quickstart: Local Deployment Mode

This guide covers running the proxy locally on your laptop — no cloud account, no Docker, no Postgres required.

---

## Prerequisites

- Go 1.21+ (to build from source) **or** a pre-built binary
- Node.js + npx (only if you want to use stdio-based MCP servers like the filesystem server)

---

## 1. Build the binary

```bash
go build -o bin/mcp-proxy ./cmd/server
```

---

## 2. Start in local mode

```bash
LOCAL_MODE=true KMS_KEY_NAME=local LOCAL_KMS_KEY=$(openssl rand -hex 32) \
  BASE_URL=http://localhost:9753 ./bin/mcp-proxy
```

Or copy `.env.local` and add `LOCAL_MODE=true`:

```bash
make .env.local          # generates .env.local with a random LOCAL_KMS_KEY
echo "LOCAL_MODE=true" >> .env.local
set -a && . ./.env.local && set +a && ./bin/mcp-proxy
```

The proxy creates `mcp-proxy.db` (SQLite) in the current directory on first run.

---

## 3. Create your admin account

Open `http://localhost:9753` in a browser. Because this is the first time the proxy has run, the registration form will grant you admin privileges automatically.

---

## 4. Add an MCP server (HTTP, with PAT)

Navigate to **Admin → Catalog** or use the API:

```bash
curl -s -X POST http://localhost:9753/api/admin/catalog \
  -H "Content-Type: application/json" \
  -H "Cookie: session=<your-session-cookie>" \
  -d '{
    "server_type": "github",
    "display_name": "GitHub",
    "server_url": "https://api.githubcopilot.com/mcp/",
    "auth_type": "pat",
    "transport": "http"
  }'
```

> **Note**: `auth_type: "oauth2"` is not supported in local mode. Use `"pat"` or `"api_key"` instead.

---

## 5. Add a stdio MCP server

```bash
curl -s -X POST http://localhost:9753/api/admin/catalog \
  -H "Content-Type: application/json" \
  -H "Cookie: session=<your-session-cookie>" \
  -d '{
    "server_type": "filesystem",
    "display_name": "Local Filesystem",
    "server_url": "",
    "auth_type": "none",
    "transport": "stdio",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/Users/you/docs"]
  }'
```

The proxy will spawn the `npx` subprocess on demand when an AI tool calls a filesystem tool.

---

## 6. Connect your AI tool

Go to the dashboard (`http://localhost:9753`) to see your personal proxy URL:

```
http://localhost:9753/mcp/<your-token>
```

Add it to Claude Code:

```bash
claude mcp add --transport http mcp-proxy-local http://localhost:9753/mcp/<your-token>
```

---

## 7. Dual-proxy setup (local + company proxy)

If your company also runs a hosted proxy, configure your AI tool with both URLs. Each proxy contributes its own tools to the merged list:

```bash
# Company proxy (full OAuth2, shared catalog)
claude mcp add --transport http mcp-proxy-company https://mcp.example.com/mcp/<company-token>

# Your local proxy (PAT-based, stdio servers)
claude mcp add --transport http mcp-proxy-local http://localhost:9753/mcp/<local-token>
```

---

## Environment reference

| Variable | Required | Default | Notes |
|----------|----------|---------|-------|
| `LOCAL_MODE` | Yes (for local) | `false` | Set to `true` to enable local mode |
| `LOCAL_KMS_KEY` | Yes | — | 32-byte hex key; generate with `openssl rand -hex 32` |
| `KMS_KEY_NAME` | Yes | — | Set to `local` to use `LOCAL_KMS_KEY` |
| `BASE_URL` | Yes | — | e.g., `http://localhost:9753` |
| `PORT` | No | `9753` | |
| `DATA_DIR` | No | `.` (CWD) | Directory for `mcp-proxy.db` |
| `DB_DSN` | No | `file:{DATA_DIR}/mcp-proxy.db` | Override the SQLite path |
