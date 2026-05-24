# Feature Specification: macOS Status Bar Menu

**Feature Branch**: `008-macos-status-menu`

**Created**: 2026-05-24

**Status**: Draft

**Input**: User description: "Add a macOS Status Bar menu app for the mcp-proxy local mode. It shows whether the service is running or stopped, lets the user start/stop it, and opens a Preferences window that includes the same upstream management functionality as the web dashboard (list, connect, disconnect upstreams)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See Service Status at a Glance (Priority: P1)

A user who installed mcp-proxy in local mode wants to know at any moment whether the proxy is running or stopped — without opening a terminal or a browser. They glance at the macOS menu bar and immediately see the status from the menu bar icon.

**Why this priority**: This is the foundation of the entire feature. A visible status indicator makes the proxy a first-class macOS citizen and is a prerequisite for all other interactions in the menu.

**Independent Test**: Can be fully tested by installing the app, starting the proxy, and confirming the menu bar icon reflects "running." Then stopping the proxy and confirming the icon changes to "stopped." No other functionality is required.

**Acceptance Scenarios**:

1. **Given** the proxy service is running, **When** the user looks at the menu bar, **Then** the icon and/or indicator clearly conveys an active/running state.
2. **Given** the proxy service is stopped, **When** the user looks at the menu bar, **Then** the icon and/or indicator clearly conveys a stopped/inactive state.
3. **Given** the proxy service status changes (started or stopped by any means — terminal, launchd, or the menu itself), **When** the status changes, **Then** the menu bar icon updates within 5 seconds without requiring user interaction.

---

### User Story 2 - Start and Stop the Service from the Menu (Priority: P2)

A user wants to start or stop the local proxy directly from the menu bar without opening a terminal.

**Why this priority**: Control over the service lifecycle is the most common action after checking status. Delivering this removes the last reason a non-technical user would ever need a terminal for routine proxy management.

**Independent Test**: Can be fully tested by clicking the menu bar icon, selecting "Start" when stopped (or "Stop" when running), and confirming the service state changes and the icon updates.

**Acceptance Scenarios**:

1. **Given** the proxy is stopped, **When** the user opens the menu and selects "Start," **Then** the proxy service starts and the icon updates to running state.
2. **Given** the proxy is running, **When** the user opens the menu and selects "Stop," **Then** the proxy service stops and the icon updates to stopped state.
3. **Given** the proxy is in a transitioning state (starting up), **When** the user opens the menu, **Then** the start/stop action is either disabled or shows a transitioning indicator to prevent double-actions.

---

### User Story 3 - Manage Connected Upstreams via Preferences (Priority: P3)

A user wants to add, view, or remove upstream MCP server connections without opening a browser. They open the Preferences window from the menu, see their connected upstreams, and can connect new ones or disconnect existing ones.

**Why this priority**: This is the value-add beyond basic service control — it makes the proxy self-contained on macOS. It depends on the proxy running (P2) and the MCP management tools feature (spec 004).

**Independent Test**: Can be fully tested by opening Preferences, confirming the connected upstreams list matches what is configured, adding a new upstream with an API key, confirming it appears in the list, then disconnecting it and confirming it disappears.

**Acceptance Scenarios**:

1. **Given** the Preferences window is open, **When** it loads, **Then** the user sees a list of their currently connected upstreams with name, server type, and connection status.
2. **Given** the Preferences window is open, **When** the user selects a catalog entry and enters an API key and clicks Connect, **Then** the upstream is added and appears in the connected list.
3. **Given** a connected upstream is shown in the Preferences list, **When** the user clicks Disconnect, **Then** the upstream is removed and no longer appears in the list.
4. **Given** the proxy service is stopped, **When** the user opens Preferences, **Then** the upstream list shows a clear message that the service must be running to manage connections, or the Preferences window is disabled.

---

### User Story 4 - Open Dashboard in Browser (Priority: P4)

A user needs access to the full web dashboard (e.g., for OAuth2 upstream connections that require a browser flow). They open it directly from the menu bar without having to remember the proxy's port number.

**Why this priority**: The web dashboard handles flows the Preferences window cannot (OAuth2). A direct "Open Dashboard" action in the menu keeps those workflows accessible without requiring the user to know the URL.

**Independent Test**: Can be fully tested by clicking "Open Dashboard" in the menu and confirming the proxy's web UI opens in the default browser at the correct URL.

**Acceptance Scenarios**:

1. **Given** the proxy is running, **When** the user selects "Open Dashboard" from the menu, **Then** the system default browser opens the proxy dashboard URL.
2. **Given** the proxy is stopped, **When** the user selects "Open Dashboard," **Then** the user sees a message that the service is not running (the menu item is disabled or shows a warning).

---

### Edge Cases

- What happens when the menu bar app launches but the proxy is not installed? The app shows "Not installed" or "Service unavailable" and disables start/stop controls.
- What if the proxy crashes unexpectedly? The status indicator should detect the stopped state within 5 seconds via polling and update accordingly.
- What if the Preferences window is already open and the user clicks "Preferences" again? The existing window is brought to the foreground rather than opening a second window.
- What if the user tries to add an upstream while the service is momentarily unreachable? An error message is shown in the Preferences window; no partial record is created.
- What happens when the menu bar app is quit by the user? The proxy service continues running — quitting the menu app does not stop the proxy.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The menu bar app MUST display a status indicator that reflects whether the proxy service is running or stopped.
- **FR-002**: The status indicator MUST update automatically within 5 seconds of a service state change, without requiring the user to click or refresh.
- **FR-003**: The menu MUST include a context-sensitive action to start the service when stopped, and stop it when running.
- **FR-004**: The menu MUST include an "Open Dashboard" item that opens the proxy web UI in the system default browser.
- **FR-005**: The menu MUST include a "Preferences…" item that opens the Preferences window.
- **FR-006**: The menu MUST include a "Quit" item that exits the menu bar app without stopping the proxy service.
- **FR-007**: The Preferences window MUST display the list of connected upstreams for the local user, including name, server type, and status.
- **FR-008**: The Preferences window MUST allow the user to connect a new upstream by selecting from the available catalog and providing an API key (mirroring the api_key/PAT flow from the web dashboard).
- **FR-009**: The Preferences window MUST allow the user to disconnect an existing upstream.
- **FR-010**: The Preferences window MUST show a clear message and disable connection management when the proxy service is not running.
- **FR-011**: Only one Preferences window instance MAY be open at a time; a second click brings the existing window to the foreground.
- **FR-012**: Quitting the menu bar app MUST NOT stop or restart the proxy service.
- **FR-013**: The menu bar app MUST launch automatically at login alongside the proxy service (installed as part of the same local-mode install flow).

### Key Entities

- **Service State**: Whether the proxy process is running or stopped — the app observes this but does not own it.
- **Catalog Entry**: An available upstream server definition, read from the running proxy (read-only).
- **Upstream Connection**: A user's configured connection to a catalog entry, including credentials — created and deleted via the Preferences window.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can determine the proxy service status within 2 seconds of looking at their menu bar, with no clicks required.
- **SC-002**: A user can start or stop the proxy service from the menu bar in under 10 seconds, end-to-end.
- **SC-003**: A user can connect a new api_key upstream from the Preferences window in under 2 minutes without opening a browser or a terminal.
- **SC-004**: Status reflects service reality within 5 seconds of any state change (start, stop, crash).
- **SC-005**: Quitting the menu bar app never causes the proxy service to stop — 100% of quit actions leave the service in its current state.

## Assumptions

- This feature targets macOS local-mode installations only — it is not relevant to cloud or server deployments.
- The menu bar app ships as part of the existing local-mode install package, not as a separate download.
- The Preferences window surfaces the same upstream management operations as the web dashboard for api_key and PAT auth types. OAuth2 upstreams still require the web dashboard.
- The menu bar app communicates with the running proxy using the same programmatic interface as the web dashboard — no direct database access.
- The app uses the proxy's admin token or local-mode authentication — no separate credential is introduced.
- The proxy's port (9753 per the local-install spec) is known to the menu bar app at install time via the same config file used by the install script.
- The menu bar app depends on the MCP management tools spec (004) being implemented first, or is delivered concurrently — the Preferences window calls those APIs.
