<!--
## Sync Impact Report
- **Version change**: N/A → 1.0.0 (initial ratification)
- **Modified principles**: N/A (initial)
- **Added sections**: Core Principles (I–IV), Cloud Deployment Constraints,
  Upstream Server Catalog, Governance
- **Removed sections**: N/A
- **Templates**:
  - `.specify/templates/plan-template.md` ✅ No changes required — Constitution Check
    gates derive from this document at plan time
  - `.specify/templates/spec-template.md` ✅ No changes required — generic structure
    suits MCP Proxy feature specs
  - `.specify/templates/tasks-template.md` ✅ No changes required — phase structure
    is appropriately generic
- **Deferred TODOs**:
  - `TODO(STACK)`: Technology stack not yet selected; to be determined during the
    first feature specification based on MCP library availability, cloud deployment
    fit, and operational complexity.
-->

# MCP Proxy Constitution

## Core Principles

### I. Simplicity First

The simplest solution that satisfies the requirement MUST be chosen. Abstractions,
frameworks, and dependencies are introduced only when a concrete, present problem
demands them — not in anticipation of future needs (YAGNI). Every architectural
decision MUST be justifiable against a real, existing constraint.

**Rationale**: A proxy is inherently complex at the network layer. Keeping
application-level code minimal prevents the proxy from becoming a framework unto
itself and reduces the operational surface area in a cloud deployment.

**Compliance gate**: Any pull request introducing a new abstraction, pattern, or
dependency MUST include a one-sentence justification explaining the concrete problem
it solves today.

### II. Cloud-Native by Default

MCP Proxy runs exclusively in cloud environments. Local execution, embedded databases,
and filesystem-based persistent state are not valid architectural choices. All state
MUST be externalized to cloud-appropriate storage. Deployment artifacts MUST be
containerizable and stateless.

**Rationale**: The product's core value is eliminating per-machine MCP server
installation. A locally-runnable variant would undermine that value proposition and
create two maintenance paths with divergent operational requirements.

**Compliance gate**: No feature MUST assume local filesystem availability or
process-local state that cannot survive a container restart.

### III. MCP Protocol Fidelity

The proxy MUST be a transparent MCP intermediary. AI tools (Claude, Gemini, Cursor,
etc.) MUST connect using standard MCP client libraries with no modification or
awareness of the proxy layer. Tool call semantics, error codes, and streaming behavior
MUST be preserved exactly as defined by each upstream MCP server.

**Rationale**: If the proxy requires AI tool vendors to implement custom handling,
adoption collapses. Transparency is the product. The single endpoint value proposition
depends entirely on the proxy being invisible to both the AI tool and the upstream
server.

**Compliance gate**: Integration tests MUST verify that each supported upstream
server's tools are callable end-to-end through the proxy without behavioral
difference from calling the upstream directly.

### IV. Security by Design

Credential handling is the highest-risk surface in this system. User API keys, OAuth
tokens, and service credentials MUST never be logged, cached in plaintext, or exposed
in error messages. Credential storage and transmission MUST follow least-privilege
principles. Each user's credentials MUST be isolated from all other users' credentials
at every layer.

**Rationale**: The proxy holds authentication secrets for 10–15 services per user. A
single credential leak exposes users across multiple third-party platforms
simultaneously. Security cannot be retrofitted onto a credential proxy — it MUST be
foundational from day one.

**Compliance gate**: Credential handling code MUST be reviewed against this principle
in every pull request that touches auth, storage, or logging paths.

## Cloud Deployment Constraints

The following constraints apply to all features and MUST inform every architectural
decision:

- **No local state**: All persistent data lives in cloud storage (object storage,
  managed databases, or secret managers). Nothing is written to the container
  filesystem beyond ephemeral request processing.
- **Stateless compute**: Proxy instances MUST be horizontally scalable with no
  session-affinity requirements.
- **Secret management**: Upstream service credentials MUST be stored in a dedicated
  secret manager (e.g., AWS Secrets Manager, GCP Secret Manager, HashiCorp Vault).
  Hard-coded credentials and environment-variable-only credential storage are
  prohibited in production.
- **Stack**: TODO(STACK): Technology stack not yet selected. The first feature
  specification will include an explicit stack decision based on MCP protocol library
  availability, cloud deployment fit, and operational complexity.

## Upstream Server Catalog

The following cloud-hosted MCP servers represent the initial target integration set.
The catalog is non-exhaustive; additional servers may be added over time.

| Server | Provider | Category |
|---|---|---|
| GitHub | GitHub (Microsoft) | Source control / CI |
| Notion | Notion Labs | Productivity / docs |
| Linear | Linear Orbit | Issue tracking |
| Cloudflare | Cloudflare | Infrastructure / CDN |
| Google Cloud | Google | Cloud platform |

New upstream integrations MUST follow the same proxy pattern established by the
initial implementations. Each upstream server MUST have at least one end-to-end
integration test before being listed as supported.

## Governance

This constitution supersedes all other project practices and conventions. Where a
practice conflicts with this document, this document governs.

**Amendment procedure**:
1. Open a pull request with the proposed constitution change.
2. State the motivation and identify downstream artifacts requiring updates
   (templates, specs, plans).
3. Obtain approval from the project maintainer(s).
4. Update all affected artifacts in the same pull request.
5. Increment the version number per the versioning policy below.

**Versioning policy**:
- MAJOR: Removal or redefinition of an existing principle.
- MINOR: Addition of a new principle or section with materially new guidance.
- PATCH: Wording clarification, typo fix, or non-semantic refinement.

**Compliance review**: Every pull request MUST pass the Constitution Check gate
defined in `.specify/templates/plan-template.md`. Complexity violations MUST be
documented in the Complexity Tracking table of the relevant `plan.md`.

**Version**: 1.0.0 | **Ratified**: 2026-05-23 | **Last Amended**: 2026-05-23
