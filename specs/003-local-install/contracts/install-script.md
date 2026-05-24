# Contract: Install Script (`install.sh`)

The install script is a single shell script distributed via a stable URL. It handles initial install, upgrade, and uninstall.

## Distribution URL

```
https://raw.githubusercontent.com/rayjohnson/mcp-proxy/main/install.sh
```

Canonical install command:
```bash
curl -fsSL https://raw.githubusercontent.com/rayjohnson/mcp-proxy/main/install.sh | sh
```

## CLI Interface

```
install.sh [OPTIONS]

Options:
  --port PORT        Listening port (default: 9753)
  --data-dir DIR     Data directory (default: ~/Library/Application Support/mcp-proxy)
  --uninstall        Remove binary and deregister service (prompts about data)
  --purge-data       With --uninstall: also delete data directory and Keychain entry
  --version VERSION  Install a specific release version (default: latest)
  --help             Show usage
```

## Install Behavior

1. Detect host architecture (`uname -m` → `arm64` or `x86_64`/`amd64`)
2. Fetch latest release tag from GitHub API (or use `--version`)
3. Compare against installed version (`mcp-proxy --version` if binary exists) — exit 0 if already current
4. Download binary: `mcp-proxy_<version>_darwin_<arch>.tar.gz`
5. Download `checksums.txt` and verify SHA-256 of the downloaded archive
6. Extract binary to `/usr/local/bin/mcp-proxy` (or `~/.local/bin/mcp-proxy` if `/usr/local/bin` is not writable)
7. If first install: generate a random 32-byte hex key and store in macOS Keychain under service `mcp-proxy`, account `encryption-key`
8. Write `~/Library/Application Support/mcp-proxy/config.json`
9. Create log directory `~/Library/Logs/mcp-proxy/`
10. Write `~/Library/LaunchAgents/io.mcp-proxy.local.plist`
11. Load the service: `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.mcp-proxy.local.plist`
12. Wait up to 5 seconds and verify service is reachable at `http://localhost:<port>/health`
13. Print dashboard URL

## Upgrade Behavior (binary already installed)

Steps 1–6 as above (skips steps 7 if Keychain entry already exists). Then:
- Unload service: `launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/io.mcp-proxy.local.plist`
- Replace binary
- Update plist if changed
- Reload service

Existing `config.json`, database, and Keychain entry are untouched.

## Uninstall Behavior

```bash
install.sh --uninstall            # removes binary + service, keeps data
install.sh --uninstall --purge-data  # removes everything
```

1. Stop and deregister service: `launchctl bootout gui/$(id -u) ...`
2. Remove plist: `rm ~/Library/LaunchAgents/io.mcp-proxy.local.plist`
3. Remove binary: `rm /usr/local/bin/mcp-proxy`
4. If `--purge-data`:
   - Delete `~/Library/Application Support/mcp-proxy/`
   - Delete `~/Library/Logs/mcp-proxy/`
   - Remove Keychain entry: `security delete-generic-password -s mcp-proxy -a encryption-key`

## Error Handling

| Condition | Behavior |
|-----------|----------|
| No network | Print error, exit 1, no partial state written |
| Checksum mismatch | Print error, delete downloaded file, exit 1 |
| Port in use | Print warning with port number, suggest `--port` flag, exit 1 |
| `/usr/local/bin` not writable | Fall back to `~/.local/bin/` silently |
| Service fails health check after 5s | Print error with log file path, leave service registered for inspection |
