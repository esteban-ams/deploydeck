#!/usr/bin/env bash
set -euo pipefail

REPO="esteban-ams/deploydeck"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="deploydeck"

# Detect OS
case "$(uname -s)" in
  Linux*)   OS="linux" ;;
  Darwin*)  OS="darwin" ;;
  *)
    echo "Unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

# Detect architecture
case "$(uname -m)" in
  x86_64)          ARCH="amd64" ;;
  amd64)           ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

ASSET_NAME="${BINARY_NAME}-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"
DEST="${INSTALL_DIR}/${BINARY_NAME}"

echo "Detected: ${OS}/${ARCH}"
echo "Downloading ${ASSET_NAME}..."

# Download to a temp file
TMP_FILE="$(mktemp)"
trap 'rm -f "$TMP_FILE"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$TMP_FILE" "$DOWNLOAD_URL"
else
  echo "Error: curl or wget is required to download DeployDeck." >&2
  exit 1
fi

chmod +x "$TMP_FILE"

# Install — use sudo only if we can't write to the install directory directly
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_FILE" "$DEST"
elif command -v sudo >/dev/null 2>&1; then
  echo "Installing to ${DEST} (requires sudo)..."
  sudo mv "$TMP_FILE" "$DEST"
else
  echo "Error: cannot write to ${INSTALL_DIR} and sudo is not available." >&2
  echo "Run this script as root or install manually:" >&2
  echo "  curl -fsSL ${DOWNLOAD_URL} -o ${DEST} && chmod +x ${DEST}" >&2
  exit 1
fi

# Verify the installed binary works and print the version
VERSION="$("$DEST" version 2>/dev/null | head -n1 || true)"
if [ -n "$VERSION" ]; then
  echo "Successfully installed: ${VERSION}"
else
  echo "Successfully installed DeployDeck to ${DEST}"
fi
