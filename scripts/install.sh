#!/usr/bin/env bash
set -euo pipefail

REPO="moralpriest/derotui"
BIN_NAME="derotui"

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "error: required command not found: $1" >&2
    exit 1
  }
}

need_cmd curl
need_cmd tar
need_cmd sha256sum

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

is_termux=0
if [ -n "${TERMUX_VERSION:-}" ] || [ -d "/data/data/com.termux" ]; then
  is_termux=1
fi

case "$OS" in
  linux)
    if [ "$is_termux" -eq 1 ]; then
      GOOS="android"
    else
      GOOS="linux"
    fi
    ;;
  darwin) GOOS="darwin" ;;
  *)
    echo "error: unsupported OS: $OS" >&2
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64) GOARCH="amd64" ;;
  aarch64|arm64) GOARCH="arm64" ;;
  *)
    echo "error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

API_URL="https://api.github.com/repos/${REPO}/releases/latest"
TAG="$(curl -fsSL "$API_URL" | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p' | head -n1)"

if [ -z "$TAG" ]; then
  echo "error: failed to detect latest release tag from ${REPO}" >&2
  exit 1
fi

VERSION="${TAG#v}"
ASSET="${BIN_NAME}-${VERSION}-${GOOS}-${GOARCH}"
CHECKSUMS="SHA256SUMS.txt"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Installing ${BIN_NAME} ${TAG} for ${GOOS}/${GOARCH}..."
curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMP_DIR}/${ASSET}"
curl -fsSL "${BASE_URL}/${CHECKSUMS}" -o "${TMP_DIR}/${CHECKSUMS}"

(
  cd "$TMP_DIR"
  CHECK_LINE="$(grep -E "(\./)?${ASSET}$" "$CHECKSUMS" | head -n1 || true)"
  if [ -z "$CHECK_LINE" ]; then
    echo "error: checksum entry not found for ${ASSET} in ${CHECKSUMS}" >&2
    exit 1
  fi
  printf '%s\n' "$CHECK_LINE" | sed 's#  \./#  #' | sha256sum -c -
)

chmod +x "${TMP_DIR}/${ASSET}"

if [ "$is_termux" -eq 1 ]; then
  TARGET_DIR="${PREFIX:-$HOME/.termux}/bin"
  mkdir -p "$TARGET_DIR"
else
  TARGET_DIR="/usr/local/bin"
  if [ ! -w "$TARGET_DIR" ]; then
    TARGET_DIR="$HOME/.local/bin"
    mkdir -p "$TARGET_DIR"
  fi
fi

install -m 0755 "${TMP_DIR}/${ASSET}" "${TARGET_DIR}/${BIN_NAME}"

echo "Installed to ${TARGET_DIR}/${BIN_NAME}"
if ! command -v "$BIN_NAME" >/dev/null 2>&1; then
  echo "warning: ${TARGET_DIR} is not in PATH for this shell." >&2
  echo "Add this to your shell profile:" >&2
  echo "  export PATH=\"${TARGET_DIR}:\$PATH\"" >&2
fi

echo "Run: ${BIN_NAME} --help"
