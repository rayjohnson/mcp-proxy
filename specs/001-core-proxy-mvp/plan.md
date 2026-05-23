# Implementation Plan: Core MCP Proxy MVP

**Branch**: `001-core-proxy-mvp` | **Date**: 2026-05-23 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/001-core-proxy-mvp/spec.md`

## Summary

Build a cloud-hosted MCP proxy service that aggregates multiple upstream MCP servers
behind a single Streamable HTTP endpoint per developer. A developer registers once,
connects their upstream services (via API key or OAuth2 browser flow), and receives
a stable proxy URL they paste into any MCP-compatible AI tool. An admin default
catalog pre-populates server suggestions for new users and notifies existing users
when new servers become available.

**Stack**: Go 1.23 · Cloud Run · Cloud SQL (PostgreSQL 16) · GCP KMS · Go HTML
templates · `modelcontextprotocol/go-sdk` v1.5.0

## Technical Context

**Language/Version**: Go 1.23

**Primary Dependencies**:
- `github.com/modelcontextprotocol/go-sdk` v1.5.0 — MCP server + client
- `golang.org/x/oauth2` — OAuth2 flows (upstream services + JumpCloud OIDC later)
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `cloud.google.com/go/kms` — credential encryption/decryption
- Standard library `net/http`, `html/template` — HTTP server + management UI

**Storage**:
- Cloud SQL (PostgreSQL 16) — user accounts, upstream configs, catalog, suggestions
- GCP KMS — encryption key for credential ciphertext stored in Cloud SQL
- No Secret Manager (cost: ~$600/month at 10k secrets vs. ~$1/month for KMS)

**Testing**: `go test ./...` with `testify/assert`; integration tests target a real
Cloud Run staging deployment

**Target Platform**: Cloud Run (Linux amd64 container, stateless, scales to zero)

**Performance Goals**: Proxy overhead under 50ms p95 added latency per tool call;
support 500 concurrent developer sessions at MVP launch

**Constraints**:
- No local filesystem state that cannot survive a container restart
- All secrets via GCP KMS + Cloud SQL; no Secret Manager per-credential
- Inbound (AI tools → proxy): Streamable HTTP only (SSE deprecated in MCP spec 2025-06-18)
- Outbound (proxy → upstream): auto-detect Streamable HTTP with SSE fallback; detected
  transport cached in `upstream_configs.detected_transport` and re-probed on failure
- Single binary: proxy endpoint + management web app

**Scale/Scope**: MVP target — hundreds of developers, 5 upstream server types,
single GCP region

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|-----------|--------|---------|
| I. Simplicity First | ✅ Pass | Single binary; Go stdlib HTTP + templates; no SPA framework; minimal dependencies; no microservices |
| II. Cloud-Native by Default | ✅ Pass | Cloud Run (stateless containers); Cloud SQL; GCP KMS; Workload Identity (no credential files); no local filesystem state |
| III. MCP Protocol Fidelity | ✅ Pass | Official `go-sdk` v1.5.0; Streamable HTTP transport (current spec); tool name prefixing transparent to AI tool; upstream responses returned unmodified |
| IV. Security by Design | ✅ Pass | GCP KMS for credential encryption; Workload Identity; HTTP-only JWT cookies; OAuth2 PKCE + state parameter; credentials never logged |

**No complexity violations. Complexity Tracking table not required.**

## Project Structure

### Documentation (this feature)

```text
specs/001-core-proxy-mvp/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   ├── mcp-endpoint.md  # MCP Streamable HTTP contract
│   └── management-api.md # Management REST API contract
└── tasks.md             # Phase 2 output (/speckit-tasks command)
```

### Source Code (repository root)

```text
cmd/
└── server/              # main package — single binary entry point

internal/
├── auth/                # JWT sessions, email/password, IdentityProvider interface
├── catalog/             # Admin default catalog + suggestion fan-out logic
├── handler/             # HTTP handlers: management UI, API routes, OAuth2 callbacks
├── kms/                 # GCP KMS encrypt/decrypt wrapper
├── mcp/                 # MCP proxy core: session management, tool aggregation, routing
├── oauth2client/        # OAuth2 authorization flows for each upstream service type
├── store/               # Cloud SQL data access (users, configs, catalog, suggestions)
└── upstream/            # Per-service upstream MCP client adapters

web/
├── templates/           # Go HTML templates for management UI
└── static/              # CSS, minimal vanilla JS (OAuth2 redirect, status polling)

migrations/              # PostgreSQL schema migrations (numbered SQL files)

deploy/
├── Dockerfile
└── service.yaml         # Cloud Run service definition

tests/
└── integration/         # End-to-end tests against staging Cloud Run instance
```

**Structure Decision**: Web application layout. Single Go project at repository
root. Unit tests co-located with source as `*_test.go` files. Integration tests
isolated in `tests/integration/` to allow running separately from unit tests.

## Complexity Tracking

> No constitution violations — table not applicable.
