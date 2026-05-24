# Quickstart: Installing the Local Proxy

This guide walks through installing mcp-proxy as a persistent background service on macOS — no repo clone or Go toolchain required.

---

## Prerequisites

- macOS 13 (Ventura) or later
- `curl` (pre-installed on all macOS versions)

---

## 1. Install

```bash
curl -fsSL https://raw.githubusercontent.com/rayjohnson/mcp-proxy/main/install.sh | sh
```

The script will:
- Detect your Mac's architecture (Apple Silicon or Intel)
- Download the latest release binary and verify its checksum
- Generate an encryption key and store it securely in your Keychain
- Register a login service that starts the proxy automatically
- Print your dashboard URL when done

**Expected output:**
```
→ Detecting architecture... arm64
→ Latest release: v1.2.3
→ Downloading mcp-proxy_v1.2.3_darwin_arm64.tar.gz...
→ Verifying checksum... OK
→ Installing to /usr/local/bin/mcp-proxy
→ Generating encryption key in Keychain...
→ Registering login service...
→ Waiting for service to start...
✓ mcp-proxy is running at http://localhost:9753
  Open your browser to get started.
```

---

## 2. Open the dashboard

Navigate to `http://localhost:9753` in your browser. The first account you register is automatically granted admin access.

---

## 3. Add catalog entries and connect your AI tool

See the [Local Mode quickstart](../../002-local-mode-stdio/quickstart.md) for adding catalog entries (HTTP and stdio servers) and connecting Claude Code or other AI tools.

---

## Upgrade

Run the same install command again. The script detects the installed version, downloads the new release, and restarts the service without touching your data or credentials:

```bash
curl -fsSL https://raw.githubusercontent.com/rayjohnson/mcp-proxy/main/install.sh | sh
```

---

## Custom port

If port 9753 conflicts with something on your machine:

```bash
curl -fsSL https://raw.githubusercontent.com/rayjohnson/mcp-proxy/main/install.sh | sh -s -- --port 9877
```

---

## Uninstall

```bash
# Remove binary and service, keep your data
install.sh --uninstall

# Remove everything including your database and stored credentials
install.sh --uninstall --purge-data
```

If you no longer have the install script, re-download it:
```bash
curl -fsSL https://raw.githubusercontent.com/rayjohnson/mcp-proxy/main/install.sh | sh -s -- --uninstall
```

---

## Troubleshooting

**Service not starting**: Check the log:
```bash
tail -f ~/Library/Logs/mcp-proxy/stderr.log
```

**Port in use**: Reinstall with a custom port (see above).

**Check service status**:
```bash
launchctl print gui/$(id -u)/io.mcp-proxy.local
```

**Restart the service manually**:
```bash
launchctl kickstart -k gui/$(id -u)/io.mcp-proxy.local
```
