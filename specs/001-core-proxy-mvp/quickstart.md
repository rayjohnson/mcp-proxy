# Quickstart: Core MCP Proxy MVP

This guide validates a working end-to-end deployment. Follow these steps in order
after the implementation is complete.

---

## Prerequisites

- GCP project with Cloud Run, Cloud SQL, and Cloud KMS APIs enabled
- A deployed instance of the MCP Proxy service (Cloud Run URL), or a local dev
  instance at `http://localhost:8080`
- An MCP-compatible AI tool (Claude Desktop, Claude Code, Cursor, etc.)
- A GitHub Personal Access Token (used as the test upstream server)

---

## Step 1: First Admin Account

The first user to register automatically becomes admin. No manual seeding required.

1. Open `https://<your-cloud-run-url>/register` in a browser.
2. Enter an email address and password and click **Create Account**.
3. You are redirected to the dashboard with admin privileges.
4. Verify the nav bar shows **Catalog** and **Users** links.

---

## Step 2: Add GitHub to the Default Catalog

1. Click **Catalog** in the nav bar.
2. Fill in the form:
   - **Server Type**: `github`
   - **Server URL**: `https://api.githubcopilot.com/mcp/`
   - **Display Name**: `GitHub`
   - **Description**: `GitHub repositories, issues, PRs, and code search`
   - **Auth Type**: `API Key`
3. Click **Add Server**.
4. Verify GitHub appears in the catalog table with auth type **api_key**.

---

## Step 3: Developer — Register and Connect GitHub

1. Open a new browser session (or incognito) and navigate to the management UI.
2. Click **Create Account**, enter an email and password, click **Register**.
3. The dashboard appears with GitHub listed under **Available MCP Servers**.
4. Click **Connect** on the GitHub card.
5. Paste your GitHub Personal Access Token into the API key field and click **Connect**.
6. You are redirected to the dashboard. GitHub shows status **Reachable** in the
   **Connected Servers** table.
7. Copy the **Proxy Endpoint URL** shown at the top of the dashboard.

---

## Step 4: Connect an AI Tool

Scroll down on the dashboard to the **Connect Your AI Tools** section. Expand the
entry for your AI tool — the config snippet is pre-filled with your proxy URL.

**Claude Code (CLI) example:**

```bash
claude mcp add --transport http mcp-proxy https://<your-cloud-run-url>/mcp/<your-proxy-token>
```

**Claude Desktop example** — add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "mcp-proxy": {
      "type": "http",
      "url": "https://<your-cloud-run-url>/mcp/<your-proxy-token>"
    }
  }
}
```

Restart your AI tool after saving.

---

## Step 5: Validate Tool Availability

In your AI tool, open a new conversation and ask:

> "What MCP tools do you have available?"

**Expected**: The tool lists tools prefixed with `github__`
(e.g., `github__create_issue`, `github__search_repositories`).

---

## Step 6: Validate Tool Execution

Ask:

> "Using your GitHub tools, search for repositories named 'mcp-proxy'."

**Expected**: The tool calls `github__search_repositories` and returns results from
GitHub's API, identical to calling the GitHub MCP server directly.

---

## Step 7: Validate Failure Isolation

1. In the management dashboard, remove the GitHub server configuration.
2. Add a second server (e.g., Cloudflare) with an intentionally invalid API key.
3. Ask your AI tool for available tools again.

**Expected**:
- GitHub tools no longer appear (server removed).
- Cloudflare shows a `credential_error` status in the dashboard.
- No error is thrown in the AI tool — the tool list is simply empty.

---

## Step 8: Validate Suggestion Notification

1. Sign back in as admin.
2. Add **Linear** to the default catalog (`https://api.linear.app/mcp`, API key).
3. Switch back to the developer account and refresh the dashboard.

**Expected**: Linear appears under **Available MCP Servers** immediately (FR-020).

4. Connect to Linear, then disconnect it.

**Expected**: Linear moves back to Available (connection removed, catalog entry
remains for future re-connection).

---

## Step 9: Admin User Management

1. Sign in as admin and navigate to **Admin → Users**.
2. Find the developer account created in Step 3.
3. Click **Make Admin**.
4. Verify the role badge changes to **admin**.
5. Click **Remove Admin** to revert.

**Expected**: Role changes take effect immediately. The admin cannot remove their
own admin role (button is absent for the signed-in user).

---

## Step 10: Programmatic Catalog Management (JSON API)

Using an HTTP client or AI assistant with your admin session cookie:

```bash
# List catalog
GET /api/admin/catalog

# Add a server
POST /api/admin/catalog
Content-Type: application/json
{ "server_type": "cloudflare", "server_url": "https://...", "display_name": "Cloudflare",
  "auth_type": "api_key" }

# Remove a server
DELETE /api/admin/catalog/{id}
```

**Expected**: Catalog changes are immediately reflected in the developer dashboard.

---

## Validation Complete

If all ten steps pass, the core proxy MVP is working correctly end-to-end.
