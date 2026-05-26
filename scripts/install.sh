#!/bin/sh
# BasePod installer.
#
# Usage:
#   curl -fsSL https://get.basepod.dev | sh
#   curl -fsSL https://get.basepod.dev | BASEPOD_VERSION=v0.1.0 sh
#
# Installs `basepod-server` and `basepod` to /usr/local/bin and registers a
# launchd agent at ~/Library/LaunchAgents/dev.basepod.server.plist.

set -eu

REPO="${BASEPOD_REPO:-flakerimi/basepod}"
VERSION="${BASEPOD_VERSION:-}"
PREFIX="${BASEPOD_PREFIX:-/usr/local/bin}"
DATA_DIR="${BASEPOD_DATA_DIR:-${HOME}/BasePodData}"
INSTALL_DEPS="${BASEPOD_INSTALL_DEPS:-}"

red()    { printf "\033[31m%s\033[0m\n" "$*"; }
green()  { printf "\033[32m%s\033[0m\n" "$*"; }
yellow() { printf "\033[33m%s\033[0m\n" "$*"; }

require() {
  command -v "$1" >/dev/null 2>&1 || {
    red "missing required tool: $1"
    exit 1
  }
}

find_brew() {
  if command -v brew >/dev/null 2>&1; then
    command -v brew
    return 0
  fi
  if [ -x /opt/homebrew/bin/brew ]; then
    printf "%s\n" /opt/homebrew/bin/brew
    return 0
  fi
  if [ -x /usr/local/bin/brew ]; then
    printf "%s\n" /usr/local/bin/brew
    return 0
  fi
  return 1
}

is_truthy() {
  case "$1" in
    1|true|TRUE|yes|YES|y|Y) return 0 ;;
    *) return 1 ;;
  esac
}

prompt_yes_no() {
  question="$1"
  if [ -r /dev/tty ] && [ -w /dev/tty ]; then
    printf "%s [y/N] " "$question" >/dev/tty
    IFS= read -r answer </dev/tty || answer=""
    case "$answer" in
      y|Y|yes|YES|Yes) return 0 ;;
    esac
  fi
  return 1
}

ensure_podman() {
  if command -v podman >/dev/null 2>&1; then
    return 0
  fi

  yellow "Podman is required and is not installed."
  BREW="$(find_brew || true)"
  if [ -z "$BREW" ]; then
    red "Homebrew was not found, so this installer cannot install Podman automatically."
    echo "Install Homebrew from https://brew.sh, then run: brew install podman"
    echo "After that, rerun this installer."
    exit 1
  fi

  if is_truthy "$INSTALL_DEPS" || prompt_yes_no "Install Podman now with Homebrew?"; then
    green "==> installing Podman with Homebrew"
    "$BREW" install podman
    return 0
  fi

  red "Podman is required to run BasePod."
  echo "Install it with: brew install podman"
  echo "Or rerun this installer with: BASEPOD_INSTALL_DEPS=1"
  exit 1
}

if [ "$(uname)" != "Darwin" ]; then
  red "BasePod only supports macOS in v1."
  exit 1
fi
if [ "$(uname -m)" != "arm64" ]; then
  red "BasePod requires Apple Silicon (arm64). Detected: $(uname -m)"
  exit 1
fi

require curl
require tar
ensure_podman

if [ -z "${VERSION}" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
fi
if [ -z "${VERSION}" ]; then
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

if [ ! -w "${PREFIX}" ]; then
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
echo
echo "Binaries:"
echo "  server: ${PREFIX}/basepod-server"
echo "  cli:    ${PREFIX}/basepod"
echo
echo "Service:"
echo "  launchd: loaded"
echo "  status:  starting"
echo "  plist:   ${PLIST_PATH}"
echo "  logs:    tail -f ${LOG_DIR}/server.out.log ${LOG_DIR}/server.err.log"
echo
echo "Dependencies:"
echo "  Podman: installed"
echo "  Caddy:  managed automatically as a container"
echo
echo "Data:"
echo "  ${DATA_DIR}"
echo
echo "Open:"
echo "  http://localhost:8080"
echo
echo "First run:"
echo "  1. Create your admin user"
echo "  2. Optional: configure a root domain for app subdomains"
