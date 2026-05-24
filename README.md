# MCP Proxy

A self-hosted proxy that aggregates multiple upstream MCP servers behind a single personal endpoint. Connect your AI tools once — the proxy handles routing to all your configured servers.

## How it works

1. You (admin) add MCP servers to the catalog — GitHub, Linear, Notion, Cloudflare, Google Cloud, etc.
2. Each user registers, connects to the servers they want (API key or OAuth2), and gets a personal proxy URL.
3. Paste that URL into Claude, Cursor, VS Code, or any MCP-compatible tool. All your servers' tools appear through the single endpoint.

```
AI tool  →  https://your-proxy/mcp/<token>  →  GitHub MCP
                                             →  Linear MCP
                                             →  Notion MCP
```

Tool names are prefixed to avoid collisions: `github__create_issue`, `linear__create_issue`, etc.

## Features

- **Single endpoint** — one stable URL per user, never changes when you add or remove servers
- **Admin catalog** — admins pre-configure available servers; users just connect
- **OAuth2 + API key** — OAuth2 flows handled entirely by the proxy; users never touch tokens
- **Failure isolation** — one unreachable server doesn't break the others
- **Background health probes** — server status kept fresh automatically
- **Admin JSON API** — programmatic catalog management (useful for AI-assisted setup)
- **Local mode** — run on your laptop with zero external services; SQLite storage, stdio MCP server support

## Quickstart (local mode — no cloud required)

The fastest way to install on macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/rayjohnson/mcp-proxy/main/install.sh | sh
```

This downloads the latest pre-built binary, stores an encryption key in the macOS Keychain, and registers a launchd service so the proxy starts automatically on login.

Open `http://localhost:9753`, register — the first user is automatically admin. No Docker, no Postgres, no cloud account needed.

**To build and run from source** (requires Go 1.24+):

```bash
make run-local
```

See [`specs/003-local-install/quickstart.md`](specs/003-local-install/quickstart.md) for the full install guide including upgrade and uninstall.
See [`specs/002-local-mode-stdio/quickstart.md`](specs/002-local-mode-stdio/quickstart.md) for stdio MCP server setup.

## Quickstart (hosted dev, with Postgres)

**Prerequisites**: Go 1.23+, Docker (OrbStack or Docker Desktop), `golangci-lint`

```bash
# Start Postgres, build, and run
make run
```

The server starts at `http://localhost:8080`. Open it in a browser, register — the first user becomes admin.


Other useful targets:

```bash
make test          # unit tests
make lint          # golangci-lint
make db-reset      # drop and recreate the database
```

## Connecting an AI tool

After registering, your proxy URL is shown on the dashboard. The dashboard also has copy-paste config snippets for:

- **Claude Code** — `claude mcp add --transport http mcp-proxy <url>`
- **Claude Desktop** — `claude_desktop_config.json`
- **Cursor** — `.cursor/mcp.json`
- **VS Code** — `.vscode/mcp.json`
- **Windsurf** — `~/.codeium/windsurf/mcp_config.json`

## Adding servers to the catalog (admin)

Via the UI at `/admin/catalog`, or via the JSON API:

```bash
curl -X POST https://your-proxy/api/admin/catalog \
  -H "Cookie: session=<token>" \
  -H "Content-Type: application/json" \
  -d '{
    "server_type": "github",
    "server_url": "https://api.githubcopilot.com/mcp/",
    "display_name": "GitHub",
    "auth_type": "api_key"
  }'
```

For OAuth2 servers, include `oauth_client_id` and `oauth_client_secret` (your OAuth app credentials — obtained once from the service's developer console).

## Deployment (GCP)

The service is designed for Cloud Run + Cloud SQL (Postgres 16) + Cloud KMS.

Required environment variables:

| Variable | Description |
|----------|-------------|
| `DB_DSN` | PostgreSQL connection string (hosted mode) |
| `KMS_KEY_NAME` | GCP KMS key resource name (or `local` for dev) |
| `BASE_URL` | Public URL of the service (used in OAuth2 callbacks) |
| `PORT` | Port to listen on (default `9753` in local mode, `8080` otherwise) |
| `LOCAL_KMS_KEY` | 32-byte hex key (dev/local mode, when `KMS_KEY_NAME=local`) |
| `LOCAL_MODE` | Set to `true` for single-user local deployment (SQLite, stdio support) |
| `DATA_DIR` | Directory for SQLite database in local mode (default: `.`) |

See `deploy/service.yaml` for a Cloud Run service definition and `deploy/Dockerfile` for the container build.

## Supported upstream servers

| Server | Auth type |
|--------|-----------|
| GitHub | API key (PAT) or OAuth2 |
| Linear | OAuth2 |
| Notion | OAuth2 |
| Cloudflare | API key |
| Google Cloud | OAuth2 |

## Project structure

```
cmd/server/          # main entry point
internal/
  auth/              # JWT + bcrypt
  catalog/           # catalog service
  config/            # environment config
  handler/           # HTTP handlers and middleware
  kms/               # GCP KMS + local AES shim
  mcp/               # MCP proxy core (aggregator, session, router, stdio bridge)
  oauth2client/      # OAuth2 flow + token refresh
  store/             # store interfaces + Postgres implementation
  store/sqlite/      # SQLite implementation (local mode)
  upstream/          # per-service adapters
migrations/          # SQL migrations (applied at startup)
specs/               # feature specifications and implementation plans
web/
  static/            # CSS
  templates/         # HTML templates
```
