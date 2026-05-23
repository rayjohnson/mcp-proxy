# Research: Core MCP Proxy MVP

**Branch**: `001-core-proxy-mvp` | **Date**: 2026-05-23

## MCP Protocol & Go SDK

**Decision**: Use `github.com/modelcontextprotocol/go-sdk` v1.5.0 (official SDK).

**Rationale**: The official SDK is stable, supports both server mode (serving AI
tools) and client mode (connecting to upstream servers), and GitHub's own MCP server
has migrated to it from the community alternative. The proxy needs both roles
simultaneously — server to AI tools, client to each upstream — and the official SDK
handles both.

**Alternatives considered**: `mark3labs/mcp-go` (400+ dependents, popular community
library) — rejected in favor of the official SDK now that it is stable and at v1.5.0.

---

## MCP Transport: Inbound and Outbound Are Independent

**Inbound (AI tools → proxy) decision**: Streamable HTTP only.

**Rationale**: All current AI tools (Claude, Gemini, Cursor) target Streamable HTTP
for cloud-hosted servers. The classic SSE transport is deprecated in MCP spec
2025-06-18. No inbound SSE support is needed.

**Outbound (proxy → upstream servers) decision**: Auto-detect with fallback.

**Rationale**: Upstream servers (Notion, Linear, GitHub, etc.) were built at
different points in the MCP spec lifecycle — many still speak classic SSE and have
not migrated. The proxy cannot assume Streamable HTTP on the outbound side. The
official `go-sdk` v1.5.0 provides both `NewStreamableHTTPClientTransport` and
`NewSSEClientTransport` on the client side, so the proxy can speak either.

**Detection strategy**: On first connection to an upstream server, the proxy
attempts Streamable HTTP. If the upstream does not respond correctly (returns 4xx,
wrong content type, or connection error), the proxy retries using SSE. The detected
transport is cached in `upstream_configs.detected_transport` so subsequent sessions
connect immediately without re-probing. If the cached transport fails (e.g., upstream
migrated to Streamable HTTP), the proxy re-runs detection automatically.

**Alternatives considered**: SSE-only outbound — simpler but wastes the transport
upgrade as upstreams migrate. Require admins to configure transport per server —
rejected because it exposes implementation detail to users unnecessarily.

---

## Database: Cloud SQL (PostgreSQL)

**Decision**: Cloud SQL (PostgreSQL 16) for all persistent application data.

**Rationale**: The data model is inherently relational — users own configurations,
configurations reference catalog entries, suggestions link users to catalog entries.
Cloud SQL provides ACID transactions (critical for credential updates and suggestion
state), schema enforcement, and straightforward migration tooling. Cloud Run connects
via the Cloud SQL Auth Proxy sidecar (no VPC required, no long-lived connections
held open between requests).

**Alternatives considered**: Firestore — rejected because document/collection model
adds complexity for relational queries (e.g., "which users have not dismissed this
catalog entry"), and lacks server-side join capability needed for admin views.

---

## Credential Encryption: GCP KMS + Cloud SQL

**Decision**: Encrypt all upstream credentials (API keys, OAuth2 token pairs) with
GCP Cloud KMS. Store the encrypted ciphertext in Cloud SQL alongside the rest of the
upstream server configuration.

**Rationale**: GCP Secret Manager charges $0.06 per active secret version per month.
At 1,000 users × 10 upstream servers = 10,000 secrets → ~$600/month in secret
storage alone. GCP KMS costs $1/month per key plus $0.03 per 10,000 crypto
operations — orders of magnitude cheaper at any realistic user count. One KMS key
encrypts all credentials; the key is managed and rotated by GCP with no plaintext
ever leaving the KMS boundary.

**Alternatives considered**: GCP Secret Manager (one secret per credential) —
rejected due to cost at scale. Application-level AES encryption with a hardcoded key
— rejected because it moves key management responsibility into the application and
violates the Security by Design principle.

---

## Cloud Run Authentication to GCP Services

**Decision**: Workload Identity (no service account key files).

**Rationale**: Cloud Run services are assigned a service account automatically.
Granting that service account `cloudkms.cryptoKeyEncrypterDecrypter` and
`cloudsql.client` roles via IAM means the application never handles credential
files. This is the GCP-recommended approach and eliminates a class of credential
leak risk.

---

## Management UI: Server-Rendered Go Templates + Minimal JS

**Decision**: Go HTML templates served by the same binary, with vanilla JavaScript
only where browser interactivity is unavoidable (OAuth2 popup flow, status polling).

**Rationale**: The management UI has a small, well-defined surface: list servers,
add/remove servers, view status, handle suggestions. A full SPA framework (React,
Vue, Svelte) would add a build pipeline, NPM dependency surface, and separate
deployment artifact — all in violation of the Simplicity principle. Go templates
handle 95% of the UI; the 5% that needs JS (initiating OAuth2 browser redirect,
polling upstream status) uses plain `fetch()` calls.

**Alternatives considered**: React + separate API — rejected (two deployments, build
step, more moving parts). HTMX — considered but vanilla JS is sufficient for the
small interactive surface and avoids another dependency.

---

## Authentication: JWT + Email/Password + OIDC Interface

**Decision**: JWT session tokens (short-lived, 24h, stored in HTTP-only cookies),
email/password login with bcrypt hashing, and an `IdentityProvider` interface in the
auth layer that JumpCloud OIDC can implement without changes to session handling.

**Rationale**: JWT is stateless and fits Cloud Run's stateless container model
without requiring a session store. The `IdentityProvider` interface decouples
sign-in mechanism from session management — swapping in JumpCloud OIDC later means
implementing the interface, not rearchitecting auth. JumpCloud supports OIDC
(simpler than SAML for a Go application using standard `golang.org/x/oauth2`).

**Alternatives considered**: Server-side sessions in Cloud SQL — rejected because
it adds a database read to every authenticated request. Cookies with session IDs in
Redis — rejected (another dependency, no clear advantage over JWT for this use case).

---

## Project Type and Deployment

**Decision**: Single Go binary deployed as a Cloud Run service. One binary handles
both the MCP proxy endpoint and the management web application.

**Rationale**: The proxy and management app share user/config data and auth logic.
Splitting into two services would require an internal API between them, doubling
operational surface for no benefit at MVP scale. Cloud Run handles traffic to both
endpoints via path-based routing within the single service.

**Structure decision**: Web application layout (Go backend + server-rendered HTML
frontend), single project at repository root.
