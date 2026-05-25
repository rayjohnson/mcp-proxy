# Feature Specification: AI Tool Auto-Configuration

**Feature Branch**: `009-ai-tool-autoconfig`

**Created**: 2026-05-24

**Status**: Draft

**Input**: User description: "Add a button to the Connect Your AI Tools section of the dashboard that automatically writes the mcp-proxy MCP endpoint into the config file of installed AI tools (Claude Desktop, Cursor, etc.) instead of only showing copy-paste instructions. Detects which tools are installed, shows their status, and configures them in one click."

## User Scenarios & Testing *(mandatory)*

### User Story 1 — One-Click Configure Claude Desktop (Priority: P1)

A user has Claude Desktop installed and mcp-proxy running. They visit the "Connect Your AI Tools" section of the dashboard. Instead of manually copying a JSON snippet and editing a config file, they click a "Configure" button next to Claude Desktop. The proxy writes the correct entry into Claude Desktop's config file and confirms success.

**Why this priority**: The most common tool users will want to configure. Eliminates the most error-prone manual step in the setup flow.

**Independent Test**: Fresh install with Claude Desktop present but not configured. Click Configure. Without touching any files manually, restart Claude Desktop and confirm the mcp-proxy tools are available.

**Acceptance Scenarios**:

1. **Given** Claude Desktop is installed and not yet configured, **When** the user clicks Configure, **Then** the proxy entry is written to Claude Desktop's config file and the dashboard shows "Configured ✓"
2. **Given** Claude Desktop is already configured with the proxy, **When** the user clicks Configure, **Then** the existing entry is updated (not duplicated) and the dashboard shows "Configured ✓"
3. **Given** Claude Desktop is not installed, **When** the user views the dashboard, **Then** Claude Desktop is shown as "Not installed" with no Configure button
4. **Given** the config file exists but is not writable, **When** the user clicks Configure, **Then** an error message explains the problem and suggests a manual fix

---

### User Story 2 — Configure Other Installed AI Tools (Priority: P2)

Beyond Claude Desktop, users may have other AI coding tools (Cursor, Windsurf, VS Code with MCP extensions, etc.) installed. The dashboard detects each one and offers a Configure button for those that are installed.

**Why this priority**: Extends the value of one-click setup to the broader ecosystem without requiring a separate workflow per tool.

**Independent Test**: With Cursor installed alongside Claude Desktop, confirm both show as detected with Configure buttons, and both can be configured independently.

**Acceptance Scenarios**:

1. **Given** Cursor is installed, **When** the user views the dashboard, **Then** Cursor appears with its installation status and a Configure button
2. **Given** a tool is configured, **When** the user clicks Configure again, **Then** the entry is updated to the current proxy URL without creating duplicates
3. **Given** a tool is not installed, **When** the user views the dashboard, **Then** that tool row shows "Not installed" and no Configure button is shown

---

### User Story 3 — View and Verify Configuration Status (Priority: P3)

A user wants to know which tools are currently pointing at their proxy. The dashboard shows a status for each supported tool: Configured, Not configured, or Not installed — without the user having to open any config files manually.

**Why this priority**: Reduces confusion and support burden by making the current state visible at a glance.

**Independent Test**: Manually add the proxy entry to one tool's config, then open the dashboard and confirm it shows as "Configured" without clicking anything.

**Acceptance Scenarios**:

1. **Given** a tool's config already contains the proxy entry, **When** the dashboard loads, **Then** that tool shows as "Configured ✓"
2. **Given** a tool is installed but not configured, **When** the dashboard loads, **Then** that tool shows as "Not configured" with a Configure button
3. **Given** the proxy URL changes (e.g., different port), **When** the user clicks Configure, **Then** the tool's config is updated to the new URL

---

### Edge Cases

- What if the tool's config file is valid JSON but the proxy writes malformed content? The proxy must validate its output before writing and roll back on error.
- What if the user is running multiple mcp-proxy instances on different ports? The Configure button uses the current proxy's own URL.
- What if the config file is managed by another tool (e.g., symlinked or read-only)? Show a clear error with the path and suggested manual steps.
- What if a new version of an AI tool changes its config file location? The feature should fail gracefully with a "config file not found" message rather than writing to a wrong location.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The dashboard MUST display a list of supported AI tools with their detected installation and configuration status
- **FR-002**: For each installed, unconfigured tool, the dashboard MUST show a "Configure" button that writes the proxy entry to the tool's config file in one action
- **FR-003**: The system MUST detect whether each supported tool is installed by checking known installation paths for the current operating system
- **FR-004**: The system MUST detect whether each installed tool is already configured to use this proxy instance
- **FR-005**: Clicking Configure MUST be idempotent — running it multiple times MUST NOT create duplicate entries
- **FR-006**: If writing to a config file fails, the dashboard MUST display a human-readable error message; the original config file MUST be left unchanged
- **FR-007**: The dashboard MUST refresh the tool status after a successful Configure action without requiring a full page reload
- **FR-008**: Supported tools at launch MUST include at minimum: Claude Desktop (macOS and Windows paths) and Gemini CLI

### Key Entities

- **AI Tool**: A supported AI application with a known config file location and known MCP config schema; has a detected status (not installed / installed-unconfigured / configured)
- **Tool Config Entry**: The specific JSON (or equivalent) block that registers mcp-proxy as an MCP server in a given tool's config file

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can configure a freshly installed AI tool to use mcp-proxy in 2 interactions or fewer (one click on Configure, one confirmation of success)
- **SC-002**: The dashboard correctly detects installation and configuration status for all supported tools on page load with no manual input
- **SC-003**: Clicking Configure on an already-configured tool leaves exactly one proxy entry in the config (no duplicates)
- **SC-004**: A failed Configure attempt leaves the target config file byte-for-byte identical to its pre-attempt state

## Assumptions

- Initial support targets macOS only, matching the existing local-mode install target; Windows paths may be added later
- Supported tools at launch: Claude Desktop and Gemini CLI; additional tools (Cursor, Windsurf) are additive scope
- The proxy uses its own base URL to construct the MCP endpoint written into tool configs; no manual URL entry is needed
- The feature is only available in local mode (single-user install); multi-tenant deployments are out of scope
