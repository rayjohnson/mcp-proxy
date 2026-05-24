# Feature Specification: Local Mode Packaging & Distribution

**Feature Branch**: `003-local-install`

**Created**: 2026-05-24

**Status**: Draft

## User Scenarios & Testing *(mandatory)*

### User Story 1 — One-Command Install from the Internet (Priority: P1)

A developer who has never cloned the repo wants to install the local proxy on their Mac. They run a single command (copy-pasted from the README or project website) in their terminal and, within a minute, have the proxy running as a background service that starts automatically on login.

**Why this priority**: This is the primary acquisition path. Requiring a repo clone, Go toolchain, or Homebrew tap adds friction that discourages adoption. A curl-install script is the lowest barrier to entry and delivers the full value immediately.

**Independent Test**: On a machine with no repo clone and no Go toolchain, run the install command → verify the service is running → open the browser to the local dashboard URL → register an account → confirm it works end-to-end.

**Acceptance Scenarios**:

1. **Given** a Mac with no prior installation, **When** the user runs the install command, **Then** the binary is downloaded, the service is registered, and the dashboard is accessible within 60 seconds.
2. **Given** an existing installation, **When** the user runs the install command again, **Then** the existing configuration and database are preserved and the binary is updated.
3. **Given** the install command is run, **When** the download fails (no network), **Then** an informative error message is shown and no partial state is left behind.

---

### User Story 2 — Service Runs Automatically on Login (Priority: P2)

After installation, the proxy starts automatically whenever the user logs into their Mac. They do not need to remember to launch it manually or keep a terminal window open.

**Why this priority**: A service that requires manual start every session provides much worse UX than one that is always available. This is table stakes for a companion app.

**Independent Test**: Install the service → restart the machine (or log out and back in) → verify the proxy is reachable without any manual action.

**Acceptance Scenarios**:

1. **Given** the service is installed, **When** the user logs in, **Then** the proxy is running and accepting connections within 30 seconds of desktop appearance.
2. **Given** the service crashes, **When** the crash occurs, **Then** the service restarts automatically within 10 seconds.
3. **Given** the user uninstalls the service, **When** they log in next time, **Then** the proxy does not start.

---

### User Story 3 — Persistent Encryption Key Across Restarts (Priority: P3)

Credentials and tokens stored by the proxy (PATs, API keys) survive service restarts. The user never needs to re-enter credentials after a reboot.

**Why this priority**: Without key persistence, every restart wipes stored credentials, making the service functionally useless. This is a correctness requirement for the storage system, not just a convenience.

**Independent Test**: Install the service → add a catalog entry with a PAT → restart the service → verify the catalog entry and its credentials are still intact and the proxy can use them.

**Acceptance Scenarios**:

1. **Given** credentials are stored, **When** the service restarts, **Then** all previously stored credentials remain accessible.
2. **Given** the proxy is reinstalled with an upgrade, **When** the service restarts, **Then** existing credentials are preserved (key is not rotated on upgrade).
3. **Given** the user explicitly uninstalls and reinstalls, **When** they confirm they want a fresh start, **Then** old credentials are discarded.

---

### User Story 4 — Non-Conflicting Default Port (Priority: P4)

The proxy listens on a port that does not conflict with common developer tools (web servers, React dev servers, Vite, etc.) so that installing the proxy does not break existing local development workflows.

**Why this priority**: Port 8080 is extremely common in developer environments. A conflict forces the user to choose between the proxy and their existing workflow.

**Independent Test**: Install the proxy with default settings on a machine already running a service on port 8080 → verify both services coexist without error.

**Acceptance Scenarios**:

1. **Given** default installation settings, **When** the proxy starts, **Then** it listens on a port not in common developer use (not 3000, 4000, 5000, 8000, 8080, 8888, 9000).
2. **Given** the user wants a custom port, **When** they specify it during or after install, **Then** the service restarts on the new port.

---

### User Story 5 — Clean Uninstall (Priority: P5)

The user can completely remove the proxy — binary, service registration, and optionally their data — with a single command.

**Why this priority**: A service that is hard to remove erodes trust. Users should feel safe trying it.

**Independent Test**: Install the service → run the uninstall command → verify no binary remains, no login service is registered, and no background process is running.

**Acceptance Scenarios**:

1. **Given** the service is installed, **When** the user runs the uninstall command, **Then** the binary, service registration, and launch-on-login entry are removed.
2. **Given** the user runs the uninstall command, **When** prompted about data retention, **Then** they can choose to keep or delete their database and stored credentials.

---

### Edge Cases

- What happens if the chosen port is already in use at service start time?
- What happens if the binary download is interrupted mid-transfer?
- What happens on macOS versions that require explicit approval to load login services?
- What if the user's machine is Apple Silicon vs Intel — does the correct binary get downloaded?
- What if a new release requires a database schema migration — is it applied automatically on startup?
- What happens if a migration fails partway through?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: An install script MUST be downloadable and executable without cloning the repository or installing a Go toolchain.
- **FR-002**: The install script MUST detect the host architecture (Apple Silicon / Intel) and download the correct pre-built binary from the project's release page.
- **FR-003**: The proxy MUST be registered as a login service that starts automatically on user login and restarts automatically if it crashes.
- **FR-004**: The encryption key used to protect stored credentials MUST be generated once at install time and persisted in secure local storage, not regenerated on each start.
- **FR-005**: The default listening port MUST NOT be 3000, 4000, 5000, 8000, 8080, 8888, or 9000.
- **FR-006**: The proxy's data directory (database, config) MUST follow platform conventions for user application data.
- **FR-007**: The install script MUST be idempotent — running it on an already-installed system upgrades the binary without destroying existing configuration or data.
- **FR-008**: An uninstall script (or uninstall flag on the install script) MUST remove the binary and deregister the login service, with an option to also delete user data.
- **FR-009**: The release pipeline MUST produce pre-built binaries for macOS (Apple Silicon and Intel) on each tagged release.
- **FR-010**: The install script MUST validate the downloaded binary (checksum or signature) before installing it.
- **FR-011**: Each release binary MUST embed its version number so the install script can compare the installed version against the latest release and skip the download when already up to date.
- **FR-012**: The proxy MUST apply any pending database schema migrations automatically on startup, without requiring user action or causing data loss, so that upgrading the binary never leaves the database in an unusable state.

### Key Entities

- **Install configuration**: Port, data directory path, encryption key reference — stored in a config file in the user data directory.
- **Encryption key**: Generated once, stored in OS secure storage (e.g., macOS Keychain), referenced by a stable key name so it survives binary upgrades.
- **Login service registration**: Platform-specific descriptor (launchd plist on macOS) placed in the user's login services directory.
- **Release artifact**: Versioned, architecture-specific binary published to the project's release page with accompanying checksums.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with no prior setup can have the proxy installed and serving requests within 60 seconds of running the install command.
- **SC-002**: Stored credentials survive 100% of service restarts (zero credential loss on normal restart or crash recovery).
- **SC-003**: The proxy starts automatically within 30 seconds of user login with no manual action required.
- **SC-004**: The install command works on both Apple Silicon and Intel Macs without user intervention.
- **SC-005**: Zero port conflicts with the default configuration on a standard developer machine running common tools.
- **SC-006**: A user upgrading from any prior release loses zero data — all catalog entries, credentials, and settings are intact after the upgrade completes.

## Assumptions

- Target platform for this feature is macOS; Linux and Windows support are out of scope for this iteration.
- The project will publish versioned releases with pre-built binaries to GitHub Releases (or equivalent).
- Users have `curl` and basic shell available (standard on all macOS versions).
- The encryption key is stored in the macOS Keychain; direct file-based key storage is not acceptable for production use.
- The default port will be chosen from the range 9000–9999, specifically one not in common use.
- Homebrew formula packaging is a future concern and out of scope for this feature.
- The install script targets macOS 13 (Ventura) and later.
