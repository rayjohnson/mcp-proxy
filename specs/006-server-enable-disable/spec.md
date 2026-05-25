# Feature Specification: Per-User Server Enable/Disable

**Feature Branch**: `008-server-enable-disable`

**Created**: 2026-05-24

**Status**: Draft

**Input**: User description: "Enable and disable individual MCP upstream servers per user. Each server shown in the dashboard (both stdio auto-connected and manually connected HTTP servers) should have a toggle to enable or disable it. Disabled servers are not connected in MCP sessions and are visually distinct in the dashboard. The toggle state persists across sessions."

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Disable a Server I Don't Want Right Now (Priority: P1)

A user has several servers connected or auto-active. They want to temporarily stop using one without disconnecting it permanently (losing their credentials or configuration). They toggle it off from the dashboard, and the server stops appearing in their MCP session immediately.

**Why this priority**: Core value of the feature — the ability to reduce noise or exclude a server during focused work without losing configuration.

**Independent Test**: Connect two servers. Disable one. Start a new MCP session and confirm the disabled server's tools are not available but the enabled server's tools are.

**Acceptance Scenarios**:

1. **Given** a connected HTTP server is enabled, **When** the user toggles it off, **Then** the server is marked disabled and new sessions do not include it
2. **Given** an auto-connected stdio server is active, **When** the user toggles it off, **Then** new sessions do not launch that stdio server
3. **Given** a server is disabled, **When** the user starts a new MCP session, **Then** the disabled server's tools are absent from the session
4. **Given** a server is disabled, **When** the user views the dashboard, **Then** the server appears visually distinct from enabled servers (e.g., greyed out or marked "Disabled")

---

### User Story 2 — Re-enable a Previously Disabled Server (Priority: P2)

A user who disabled a server wants to bring it back. They toggle it on from the dashboard, and it reconnects in their next MCP session.

**Why this priority**: Enable/disable is only useful if it is easily reversible without reconfiguring anything.

**Independent Test**: Disable a server, verify it is absent from a new session, re-enable it, verify it is present in the next session.

**Acceptance Scenarios**:

1. **Given** a disabled server, **When** the user toggles it on, **Then** the server is marked enabled and subsequent sessions include it
2. **Given** a re-enabled HTTP server, **When** a new session starts, **Then** the server reconnects using the previously stored credentials without re-entering them
3. **Given** a re-enabled stdio server, **When** a new session starts, **Then** the server launches automatically as before

---

### User Story 3 — Disabled State Persists Across Sessions (Priority: P3)

A user disables a server today. Tomorrow they open a new MCP session and expect the server to still be off — they should not have to re-disable it every time.

**Why this priority**: Persistence is what makes the feature useful over time rather than a one-session workaround.

**Independent Test**: Disable a server, close all sessions, re-authenticate, open a new session, and verify the server is still disabled.

**Acceptance Scenarios**:

1. **Given** a server is disabled, **When** the user's session expires and they log back in, **Then** the server remains disabled
2. **Given** multiple servers in various enabled/disabled states, **When** the user returns the next day, **Then** all toggle states are exactly as they left them

---

### Edge Cases

- What happens when a user disables all servers? The MCP session starts with no tools — no error, just an empty tool list.
- What happens if a disabled stdio server's underlying command is also missing from the system? Disabling it should not trigger any launch attempt; the disabled state takes full precedence.
- What happens if an admin removes a catalog entry for a server the user had disabled? The disabled record is silently cleaned up; no error shown to the user.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Users MUST be able to toggle any connected or auto-active server on or off from the dashboard with a single action
- **FR-002**: The system MUST persist each user's enabled/disabled state for every server independently of other users
- **FR-003**: When a session starts, the system MUST exclude disabled servers — they MUST NOT be connected, launched, or contribute tools to the session
- **FR-004**: Disabling a server MUST NOT remove the server's stored credentials or configuration; re-enabling MUST restore it to full functionality without re-entry of credentials
- **FR-005**: Disabled servers MUST be visually distinct from enabled servers in the dashboard (greyed out, labelled, or otherwise differentiated)
- **FR-006**: The toggle action MUST take effect for all sessions started after the action; already-running sessions are not affected
- **FR-007**: The enabled/disabled state MUST survive user logout, session expiry, and proxy restarts

### Key Entities

- **Server Toggle State**: Per-user, per-server boolean (enabled/disabled); defaults to enabled for all servers
- **Connected HTTP Server**: A server the user has explicitly connected with credentials; subject to the toggle
- **Stdio Server**: A catalog-level auto-connected server; subject to the toggle on a per-user basis

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can disable any server in two interactions or fewer (one click to toggle, one optional confirmation)
- **SC-002**: A disabled server contributes zero tools to any session started after the toggle
- **SC-003**: Toggle state is preserved with 100% fidelity across logout/login cycles
- **SC-004**: Re-enabling a previously disabled server restores full functionality without requiring credential re-entry

## Assumptions

- Toggle state defaults to **enabled** for all servers; new servers added to the catalog appear enabled for all users until explicitly disabled
- The toggle applies to the authenticated user only; other users' states are unaffected
- Already-running MCP sessions are not interrupted when a toggle changes; the new state applies to future sessions only
- There is no bulk enable/disable (e.g., "disable all") in this version — per-server toggle only
