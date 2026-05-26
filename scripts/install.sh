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
HTTP_ADDR="${BASEPOD_HTTP_ADDR:-}"

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

port_in_use() {
  port="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
    return $?
  fi
  if command -v nc >/dev/null 2>&1; then
    nc -z 127.0.0.1 "${port}" >/dev/null 2>&1
    return $?
  fi
  return 1
}

choose_http_port() {
  for port in 8080 8081 8082 8083 8084 8085 8086 8087 8088 8089 8090; do
    if ! port_in_use "${port}"; then
      printf "%s\n" "${port}"
      return 0
    fi
  done
  red "could not find an available BasePod HTTP port in 8080-8090"
  echo "Set BASEPOD_HTTP_ADDR=:PORT and rerun the installer."
  exit 1
}

http_addr_port() {
  addr="$1"
  case "$addr" in
    :*) printf "%s\n" "${addr#:}" ;;
    *:*) printf "%s\n" "${addr##*:}" ;;
    *) printf "%s\n" "$addr" ;;
  esac
}

detect_browser_host() {
  if [ -n "${BASEPOD_BROWSER_HOST:-}" ]; then
    printf "%s\n" "${BASEPOD_BROWSER_HOST}"
    return 0
  fi
  if [ -n "${SSH_CONNECTION:-}" ]; then
    # SSH_CONNECTION is: client_ip client_port server_ip server_port.
    set -- ${SSH_CONNECTION}
    if [ "$#" -ge 3 ] && [ -n "$3" ]; then
      printf "%s\n" "$3"
      return 0
    fi
  fi
  for iface in en0 en1; do
    if command -v ipconfig >/dev/null 2>&1; then
      ip="$(ipconfig getifaddr "$iface" 2>/dev/null || true)"
      if [ -n "$ip" ]; then
        printf "%s\n" "$ip"
        return 0
      fi
    fi
  done
  host="$(hostname 2>/dev/null || true)"
  if [ -n "$host" ]; then
    printf "%s\n" "$host"
    return 0
  fi
  printf "%s\n" "127.0.0.1"
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

if [ -z "${HTTP_ADDR}" ]; then
  HTTP_ADDR=":$(choose_http_port)"
fi
HTTP_PORT="$(http_addr_port "${HTTP_ADDR}")"
BROWSER_HOST="$(detect_browser_host)"
BROWSER_URL="http://${BROWSER_HOST}:${HTTP_PORT}"
LOCAL_URL="http://localhost:${HTTP_PORT}"

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
    <key>BASEPOD_HTTP_ADDR</key><string>${HTTP_ADDR}</string>
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
echo "  listen:  ${HTTP_ADDR}"
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
echo "Open from your browser:"
echo "  ${BROWSER_URL}"
echo
echo "On the BasePod Mac itself:"
echo "  ${LOCAL_URL}"
echo
echo "First run:"
echo "  1. Create your admin user"
echo "  2. Optional: configure a root domain for app subdomains"
echo
echo "Overrides:"
echo "  BASEPOD_HTTP_ADDR=:9090          choose the server listen port"
echo "  BASEPOD_BROWSER_HOST=example.com choose the host printed above"
