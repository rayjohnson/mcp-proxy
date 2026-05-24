# MCP Proxy — Infrastructure Requirements & Questions

**Date**: 2026-05-23
**Author**: Ray Johnson
**Status**: Draft — seeking infra team input

---

## What We're Building

MCP Proxy is a cloud-hosted SaaS that acts as a single gateway for AI developer
tools (Claude, Gemini, Cursor, etc.) to connect to multiple MCP (Model Context
Protocol) servers. Developers currently have to install and configure 10–15 separate
MCP server integrations per AI tool — MCP Proxy eliminates that by providing a
single endpoint URL that aggregates all their configured upstream services.

The application is a single Go binary deployed as a Cloud Run service. It handles
both the MCP proxy traffic (AI tools connecting to it) and a web management UI
(developers configuring their upstream servers).

**Key characteristics**:
- Stateless compute — Cloud Run, scales to zero
- Persistent data — Cloud SQL (PostgreSQL 16) for user accounts, configurations,
  and an admin-managed server catalog
- Credential encryption — GCP KMS (one key encrypts all user credentials stored
  in Cloud SQL; we are NOT using Secret Manager per-credential due to cost at scale)
- Single GCP region for MVP
- No locally-running components; entirely cloud-hosted

---

## GCP Resources We Will Need

The following is our current understanding of what the app requires. We may be
missing things — please flag anything we've overlooked.

### Cloud Run
- One Cloud Run service (the MCP Proxy binary)
- Needs a stable public HTTPS URL (custom domain preferred)
- Minimum instances: 0 (scales to zero is fine for MVP)
- Expected traffic: low to moderate initially (hundreds of developers)

### Cloud SQL
- PostgreSQL 16 instance
- Single region, single zone for MVP (HA can be added later)
- Instance size: TBD — starting small (db-f1-micro or db-g1-small)
- The Cloud Run service connects via the **Cloud SQL Auth Proxy** (sidecar pattern)
- Database migrations are SQL files run as part of the deployment process —
  we need to discuss how to handle this in your pipeline

### GCP KMS
- One key ring (e.g., `mcp-proxy`)
- One symmetric encryption key (e.g., `credential-key`) for encrypting user
  credentials stored in Cloud SQL
- The Cloud Run service account needs the `cloudkms.cryptoKeyEncrypterDecrypter`
  role on this key

### Workload Identity
- A dedicated GCP service account for the Cloud Run service
- Roles needed:
  - `cloudsql.client` (Cloud SQL Auth Proxy connection)
  - `cloudkms.cryptoKeyEncrypterDecrypter` (credential encryption)
- No service account key files — Workload Identity only

### Container Registry / Artifact Registry
- A repository to push the application container image to
- We will need guidance on which registry to target and any naming conventions

### Networking
- We don't believe we need a VPC for MVP (Cloud SQL Auth Proxy handles Cloud SQL
  connectivity without VPC peering) — but please confirm
- The Cloud Run service needs outbound HTTPS to upstream MCP servers
  (api.githubcopilot.com, mcp.notion.com, mcp.linear.app, etc.)

---

## JumpCloud SSO Requirements

JumpCloud SSO is **not required for our initial launch** — we will ship with
email/password authentication first. However, we are building the auth layer to
support it from day one, and we will need your help to configure it before we can
turn it on.

When we are ready to enable JumpCloud SSO, we will need:

1. **A JumpCloud OIDC application** configured for MCP Proxy
   - Grant type: Authorization Code
   - Redirect URI: `https://<our-domain>/api/auth/callback/jumpcloud`
   - Scopes required: `openid`, `email`, `profile`
   - Token endpoint authentication method: `client_secret_post` (or `client_secret_basic` — we can accommodate either)

2. **OIDC configuration values** for us to deploy:
   - OIDC Discovery URL (e.g., `https://oauth.id.jumpcloud.com/.well-known/openid-configuration`)
   - Client ID
   - Client Secret (we will store this via KMS/environment, not in source code)

3. **User provisioning guidance**:
   - Should all JumpCloud users be allowed to sign in, or should access be
     restricted to a specific group?
   - Do you want to pre-provision admin accounts via JumpCloud, or should the
     first user to sign in with JumpCloud be able to self-promote to admin?

We do not need any of this to start development. We will reach out when we are
ready to wire it in.

---

## Questions for the Infra Team

We want to make sure we structure this correctly from the start rather than
rework it later. Any guidance you can offer on the following would be helpful:

### 1. Terraform: App Repo vs. Infra Mono-Repo

We know you manage Terraform from the infra mono-repo. We're unsure how you
want to handle the Cloud Run service definition and related resources for a
new application. Options we can imagine:

- All Terraform for MCP Proxy lives in the infra mono-repo (we contribute a PR)
- App-specific Cloud Run config lives in the app repo; shared infrastructure
  (Cloud SQL, KMS, IAM) lives in the infra mono-repo
- Some other split you prefer

**What we'd like to know**: What does a new application typically look like in
your Terraform setup, and what should we be contributing vs. what will you manage?

### 2. Deployment Pipeline

We will build a Dockerfile. What does the deployment process look like from
there? Specifically:

- Which CI/CD system do you use? (Cloud Build, GitHub Actions, other?)
- Where does the container image get pushed? (Artifact Registry path/project?)
- How does Cloud Run pick up a new image version? (manual trigger, automatic on push?)
- How do database migrations get run? (as a Cloud Run job before deployment, manually, other?)

### 3. Environment Configuration

The application will need runtime configuration injected (database connection
string, KMS key name, JumpCloud client ID/secret when ready, etc.). How do you
prefer to manage this?

- Cloud Run environment variables set in Terraform?
- A configuration file mounted via Cloud Storage?
- Something else?

### 4. GCP Project Structure

Should MCP Proxy get its own GCP project, or should it live as a set of services
within an existing project? We have no strong preference — just want to follow
your standard.

### 5. Anything We've Missed

Is there anything about deploying a new Cloud Run + Cloud SQL application in your
environment that we should know about before we start writing code? Known
gotchas, required labels/tags, mandatory security policies, etc.?

---

## Timeline

We are in the planning phase now and will begin implementation shortly. We do not
need any infrastructure provisioned immediately, but we wanted to get this in front
of you early so there are no surprises.

We would appreciate a conversation to work through the questions above before
we get too far into implementation. Happy to set up a meeting whenever works
for the team.
