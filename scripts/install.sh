#!/usr/bin/env bash
# BasePod installer.
#
# Usage:
#   curl -fsSL https://get.basepod.dev | sh
#   curl -fsSL https://get.basepod.dev | BASEPOD_VERSION=v0.1.0 sh
#
# Installs `basepod-server` and `basepod` to /usr/local/bin and registers a
# launchd agent at ~/Library/LaunchAgents/dev.basepod.server.plist.

set -euo pipefail

REPO="${BASEPOD_REPO:-flakerimi/basepod}"
VERSION="${BASEPOD_VERSION:-}"
PREFIX="${BASEPOD_PREFIX:-/usr/local/bin}"
DATA_DIR="${BASEPOD_DATA_DIR:-${HOME}/BasePodData}"

red()    { printf "\033[31m%s\033[0m\n" "$*"; }
green()  { printf "\033[32m%s\033[0m\n" "$*"; }
yellow() { printf "\033[33m%s\033[0m\n" "$*"; }

require() {
  command -v "$1" >/dev/null 2>&1 || {
    red "missing required tool: $1"
    exit 1
  }
}

if [[ "$(uname)" != "Darwin" ]]; then
  red "BasePod only supports macOS in v1."
  exit 1
fi
if [[ "$(uname -m)" != "arm64" ]]; then
  red "BasePod requires Apple Silicon (arm64). Detected: $(uname -m)"
  exit 1
fi

require curl
require tar
if ! command -v podman >/dev/null 2>&1; then
  yellow "podman is not installed. Install it with: brew install podman"
  yellow "Then run: podman machine init && podman machine start"
fi

if [[ -z "${VERSION}" ]]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
fi
if [[ -z "${VERSION}" ]]; then
  red "could not determine latest release; set BASEPOD_VERSION to override"
  exit 1
fi

green "==> installing BasePod ${VERSION}"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

URL="https://github.com/${REPO}/releases/download/${VERSION}/basepod-${VERSION#v}-darwin-arm64.tar.gz"
echo "downloading ${URL}"
curl -fsSL "${URL}" -o "${TMP}/basepod.tar.gz"
tar -C "${TMP}" -xzf "${TMP}/basepod.tar.gz"

if [[ ! -w "${PREFIX}" ]]; then
  yellow "installing to ${PREFIX} requires sudo"
  sudo install -m 0755 "${TMP}/basepod-server" "${PREFIX}/basepod-server"
  sudo install -m 0755 "${TMP}/basepod"        "${PREFIX}/basepod"
else
  install -m 0755 "${TMP}/basepod-server" "${PREFIX}/basepod-server"
  install -m 0755 "${TMP}/basepod"        "${PREFIX}/basepod"
fi

mkdir -p "${DATA_DIR}/_basepod"

PLIST_DIR="${HOME}/Library/LaunchAgents"
PLIST_PATH="${PLIST_DIR}/dev.basepod.server.plist"
LOG_DIR="${DATA_DIR}/_basepod/logs"
mkdir -p "${PLIST_DIR}" "${LOG_DIR}"

cat > "${PLIST_PATH}" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>dev.basepod.server</string>
  <key>ProgramArguments</key>
  <array>
    <string>${PREFIX}/basepod-server</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>BASEPOD_DATA_DIR</key><string>${DATA_DIR}</string>
    <key>PATH</key><string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>${LOG_DIR}/server.out.log</string>
  <key>StandardErrorPath</key><string>${LOG_DIR}/server.err.log</string>
</dict>
</plist>
PLIST

launchctl unload "${PLIST_PATH}" 2>/dev/null || true
launchctl load   "${PLIST_PATH}"

green "==> BasePod ${VERSION} installed"
echo "  server binary:  ${PREFIX}/basepod-server"
echo "  cli binary:     ${PREFIX}/basepod"
echo "  data dir:       ${DATA_DIR}"
echo "  launchd plist:  ${PLIST_PATH}"
echo
echo "Open http://localhost:8080 to set up your admin user."
echo "View logs:  tail -f ${LOG_DIR}/server.{out,err}.log"
