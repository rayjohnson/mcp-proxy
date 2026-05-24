#!/bin/sh
# install.sh — install, upgrade, or uninstall mcp-proxy on macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/rayjohnson/mcp-proxy/main/install.sh | sh
set -e

REPO="rayjohnson/mcp-proxy"
BINARY_NAME="mcp-proxy"
PLIST_LABEL="io.mcp-proxy.local"
PLIST_PATH="${HOME}/Library/LaunchAgents/${PLIST_LABEL}.plist"
LOG_DIR="${HOME}/Library/Logs/mcp-proxy"
DATA_DIR_DEFAULT="${HOME}/Library/Application Support/mcp-proxy"
KEYCHAIN_SERVICE="mcp-proxy"
KEYCHAIN_ACCOUNT="encryption-key"

# Defaults
PORT="9753"
DATA_DIR="${DATA_DIR_DEFAULT}"
INSTALL_VERSION=""
UNINSTALL=0
PURGE_DATA=0

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
  case "$1" in
    --port)
      PORT="$2"
      shift 2
      ;;
    --data-dir)
      DATA_DIR="$2"
      shift 2
      ;;
    --version)
      INSTALL_VERSION="$2"
      shift 2
      ;;
    --uninstall)
      UNINSTALL=1
      shift
      ;;
    --purge-data)
      PURGE_DATA=1
      shift
      ;;
    --help|-h)
      cat <<EOF
Usage: install.sh [OPTIONS]

Options:
  --port PORT        Listening port (default: 9753)
  --data-dir DIR     Data directory (default: ~/Library/Application Support/mcp-proxy)
  --version VERSION  Install a specific release version (default: latest)
  --uninstall        Remove binary and deregister service (keeps data)
  --purge-data       With --uninstall: also delete data directory and Keychain entry
  --help             Show this help
EOF
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Run with --help for usage." >&2
      exit 1
      ;;
  esac
done

# ---------------------------------------------------------------------------
# Platform check
# ---------------------------------------------------------------------------
if [ "$(uname -s)" != "Darwin" ]; then
  echo "Error: mcp-proxy local install only supports macOS." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Uninstall flow
# ---------------------------------------------------------------------------
if [ "$UNINSTALL" -eq 1 ]; then
  echo "==> Uninstalling mcp-proxy..."

  # Stop and deregister service
  if [ -f "$PLIST_PATH" ]; then
    launchctl bootout "gui/$(id -u)" "$PLIST_PATH" 2>/dev/null || true
    rm -f "$PLIST_PATH"
    echo "    Removed launchd service"
  fi

  # Remove binary
  for candidate in "/usr/local/bin/${BINARY_NAME}" "${HOME}/.local/bin/${BINARY_NAME}"; do
    if [ -f "$candidate" ]; then
      rm -f "$candidate"
      echo "    Removed binary: $candidate"
    fi
  done

  if [ "$PURGE_DATA" -eq 1 ]; then
    rm -rf "${DATA_DIR_DEFAULT}"
    rm -rf "$LOG_DIR"
    security delete-generic-password -s "$KEYCHAIN_SERVICE" -a "$KEYCHAIN_ACCOUNT" 2>/dev/null || true
    echo "    Removed data directory, logs, and Keychain entry"
  else
    echo "    Data directory preserved at: ${DATA_DIR_DEFAULT}"
    echo "    Run with --uninstall --purge-data to also remove data and Keychain entry"
  fi

  echo "==> mcp-proxy uninstalled."
  exit 0
fi

# ---------------------------------------------------------------------------
# Architecture detection
# ---------------------------------------------------------------------------
ARCH="$(uname -m)"
case "$ARCH" in
  arm64)
    GOARCH="arm64"
    ;;
  x86_64)
    GOARCH="amd64"
    ;;
  *)
    echo "Error: Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# ---------------------------------------------------------------------------
# Resolve install version
# ---------------------------------------------------------------------------
if [ -z "$INSTALL_VERSION" ]; then
  echo "==> Fetching latest release tag..."
  INSTALL_VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | head -1 \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
  if [ -z "$INSTALL_VERSION" ]; then
    echo "Error: Could not determine latest release version." >&2
    exit 1
  fi
fi
echo "==> Version to install: $INSTALL_VERSION"

# ---------------------------------------------------------------------------
# Check if already up to date
# ---------------------------------------------------------------------------
INSTALLED_VERSION=""
for candidate in "/usr/local/bin/${BINARY_NAME}" "${HOME}/.local/bin/${BINARY_NAME}"; do
  if [ -x "$candidate" ]; then
    INSTALLED_VERSION="$("$candidate" --version 2>/dev/null | awk '{print $2}')" || true
    break
  fi
done

if [ -n "$INSTALLED_VERSION" ] && [ "$INSTALLED_VERSION" = "$INSTALL_VERSION" ]; then
  echo "==> mcp-proxy $INSTALL_VERSION is already installed and up to date."
  exit 0
fi

# ---------------------------------------------------------------------------
# Download and verify
# ---------------------------------------------------------------------------
VERSION_NUM="${INSTALL_VERSION#v}"
ARCHIVE_NAME="${BINARY_NAME}_${VERSION_NUM}_darwin_${GOARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${INSTALL_VERSION}"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

echo "==> Downloading ${ARCHIVE_NAME}..."
curl -fsSL "${BASE_URL}/${ARCHIVE_NAME}" -o "${TMP_DIR}/${ARCHIVE_NAME}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt"

echo "==> Verifying checksum..."
cd "$TMP_DIR"
if ! grep "${ARCHIVE_NAME}" checksums.txt | shasum -a 256 -c - 2>/dev/null; then
  echo "Error: Checksum verification failed." >&2
  exit 1
fi
cd - > /dev/null

echo "==> Extracting binary..."
tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "$TMP_DIR"

# ---------------------------------------------------------------------------
# Resolve install path
# ---------------------------------------------------------------------------
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi
BINARY_PATH="${INSTALL_DIR}/${BINARY_NAME}"

# ---------------------------------------------------------------------------
# Stop existing service before replacing binary
# ---------------------------------------------------------------------------
if [ -f "$PLIST_PATH" ]; then
  echo "==> Stopping existing service for upgrade..."
  launchctl bootout "gui/$(id -u)" "$PLIST_PATH" 2>/dev/null || true
fi

# ---------------------------------------------------------------------------
# Install binary
# ---------------------------------------------------------------------------
echo "==> Installing binary to ${BINARY_PATH}..."
install -m 0755 "${TMP_DIR}/${BINARY_NAME}" "$BINARY_PATH"

# ---------------------------------------------------------------------------
# Keychain key management
# ---------------------------------------------------------------------------
if ! security find-generic-password -s "$KEYCHAIN_SERVICE" -a "$KEYCHAIN_ACCOUNT" > /dev/null 2>&1; then
  echo "==> Generating encryption key and storing in Keychain..."
  ENC_KEY="$(openssl rand -hex 32)"
  security add-generic-password -s "$KEYCHAIN_SERVICE" -a "$KEYCHAIN_ACCOUNT" -w "$ENC_KEY"
  echo "    Encryption key stored in Keychain"
else
  echo "==> Keychain key already exists — skipping key generation"
fi

# ---------------------------------------------------------------------------
# Create data and log directories
# ---------------------------------------------------------------------------
mkdir -p "$DATA_DIR"
mkdir -p "$LOG_DIR"

# ---------------------------------------------------------------------------
# Write config.json
# ---------------------------------------------------------------------------
CONFIG_FILE="${DATA_DIR}/config.json"
cat > "$CONFIG_FILE" <<EOF
{
  "port": "${PORT}",
  "data_dir": "${DATA_DIR}",
  "version": "${INSTALL_VERSION}",
  "keychain": {
    "service": "${KEYCHAIN_SERVICE}",
    "account": "${KEYCHAIN_ACCOUNT}"
  }
}
EOF

# ---------------------------------------------------------------------------
# Write LaunchAgent plist
# ---------------------------------------------------------------------------
mkdir -p "${HOME}/Library/LaunchAgents"
cat > "$PLIST_PATH" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
    "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${BINARY_PATH}</string>
        <string>--port</string>
        <string>${PORT}</string>
        <string>--data-dir</string>
        <string>${DATA_DIR}</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>${HOME}</string>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin</string>
        <key>LOCAL_MODE</key>
        <string>true</string>
        <key>KMS_KEY_NAME</key>
        <string>local</string>
        <key>BASE_URL</key>
        <string>http://localhost:${PORT}</string>
    </dict>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${LOG_DIR}/mcp-proxy.log</string>
    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/mcp-proxy.log</string>
</dict>
</plist>
EOF

# ---------------------------------------------------------------------------
# Bootstrap and health check
# ---------------------------------------------------------------------------
echo "==> Starting mcp-proxy service..."
launchctl bootstrap "gui/$(id -u)" "$PLIST_PATH"

echo "==> Waiting for service to be ready..."
HEALTH_URL="http://localhost:${PORT}/health"
ATTEMPTS=0
MAX_ATTEMPTS=10
while [ "$ATTEMPTS" -lt "$MAX_ATTEMPTS" ]; do
  if curl -fsS "$HEALTH_URL" > /dev/null 2>&1; then
    break
  fi
  ATTEMPTS=$((ATTEMPTS + 1))
  sleep 1
done

if [ "$ATTEMPTS" -ge "$MAX_ATTEMPTS" ]; then
  echo "Warning: Service did not respond at ${HEALTH_URL} within ${MAX_ATTEMPTS}s"
  echo "         Check logs at: ${LOG_DIR}/mcp-proxy.log"
else
  echo "==> Service is healthy."
fi

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo "mcp-proxy ${INSTALL_VERSION} installed successfully."
echo ""
echo "  Dashboard: http://localhost:${PORT}/dashboard"
echo "  Logs:      ${LOG_DIR}/mcp-proxy.log"
echo "  Data:      ${DATA_DIR}"
echo ""
