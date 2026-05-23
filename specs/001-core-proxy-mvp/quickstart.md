# Quickstart: Core MCP Proxy MVP

This guide validates a working end-to-end deployment. Follow these steps in order
after the implementation is complete.

---

## Prerequisites

- GCP project with Cloud Run, Cloud SQL, and Cloud KMS APIs enabled
- A deployed instance of the MCP Proxy service (Cloud Run URL)
- An MCP-compatible AI tool (Claude Desktop, Cursor, etc.)
- A GitHub account (used as the test upstream server)

---

## Step 1: Admin — Seed the Default Catalog

Sign in as an administrator and add GitHub to the default catalog.

1. Open `https://<your-cloud-run-url>/dashboard` in a browser.
2. Sign in with the admin account created during deployment.
3. Navigate to **Admin → Default Catalog → Add Server**.
4. Select server type `GitHub`, confirm the URL, click **Add**.
5. Verify GitHub appears in the catalog with status **Active**.

---

## Step 2: Developer — Register and Connect GitHub

1. Open a new browser session (or incognito) and navigate to the management UI.
2. Click **Create Account**, enter an email and password, click **Register**.
3. The dashboard appears. GitHub is pre-listed under **Suggested Servers**.
4. Click **Add GitHub** on the suggestion.
5. Click **Authorize with GitHub** — a browser window opens to GitHub's OAuth2
   consent screen.
6. Approve access. The browser redirects back to the dashboard.
7. GitHub shows status **Reachable** in your server list.
8. Copy the **Proxy Endpoint URL** shown at the top of the dashboard.

---

## Step 3: Connect an AI Tool

**Claude Desktop example** — add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "mcp-proxy": {
      "url": "https://<your-cloud-run-url>/mcp/<your-proxy-token>"
    }
  }
}
```

Restart Claude Desktop.

---

## Step 4: Validate Tool Availability

In Claude Desktop, open a new conversation and type:

> "What MCP tools do you have available?"

**Expected**: Claude lists tools prefixed with `github__`
(e.g., `github__create_issue`, `github__search_repositories`).

---

## Step 5: Validate Tool Execution

In Claude, type:

> "Using your GitHub tools, search for repositories named 'mcp-proxy'."

**Expected**: Claude calls `github__search_repositories` and returns results from
GitHub's API. The response should be identical to what you would get calling the
GitHub MCP server directly.

---

## Step 6: Validate Failure Isolation

1. In the management dashboard, remove the GitHub server configuration.
2. Add a second server (e.g., Cloudflare) with an intentionally invalid API key.
3. In Claude, ask for available tools again.

**Expected**:
- GitHub tools no longer appear (server removed).
- Cloudflare shows a `credential_error` status in the dashboard.
- No error is thrown in Claude — the tool list is simply empty (no servers
  configured correctly).

---

## Step 7: Validate Suggestion Notification

1. Sign back in as admin.
2. Add **Linear** to the default catalog.
3. Switch back to the developer account and refresh the dashboard.

**Expected**: A **New Server Available** suggestion for Linear appears within
60 seconds (SC-006).

4. Click **Dismiss** on the Linear suggestion.
5. Refresh the page.

**Expected**: The Linear suggestion does not reappear (FR-021).

---

## Validation Complete

If all seven steps pass, the core proxy MVP is working correctly end-to-end.
