#!/bin/sh
# BasePod CLI installer.
#
# Usage:
#   curl -fsSL https://get.basepod.dev/cli | sh
#   curl -fsSL https://get.basepod.dev/cli | BASEPOD_VERSION=v0.1.0 sh
#
# Installs only the `basepod` CLI to /usr/local/bin. It does not install or
# start the server, Podman, Caddy, or a launchd service.

set -eu

REPO="${BASEPOD_REPO:-flakerimi/basepod}"
VERSION="${BASEPOD_VERSION:-}"
PREFIX="${BASEPOD_PREFIX:-/usr/local/bin}"

red()    { printf "\033[31m%s\033[0m\n" "$*"; }
green()  { printf "\033[32m%s\033[0m\n" "$*"; }
yellow() { printf "\033[33m%s\033[0m\n" "$*"; }

require() {
  command -v "$1" >/dev/null 2>&1 || {
    red "missing required tool: $1"
    exit 1
  }
}

if [ "$(uname)" != "Darwin" ]; then
  red "BasePod CLI release artifacts currently support macOS only."
  exit 1
fi
if [ "$(uname -m)" != "arm64" ]; then
  red "BasePod CLI release artifacts currently require Apple Silicon (arm64). Detected: $(uname -m)"
  exit 1
fi

require curl
require tar

if [ -z "${VERSION}" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
fi
if [ -z "${VERSION}" ]; then
  red "could not determine latest release; set BASEPOD_VERSION to override"
  exit 1
fi

green "==> installing BasePod CLI ${VERSION}"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

URL="https://github.com/${REPO}/releases/download/${VERSION}/basepod-${VERSION#v}-darwin-arm64.tar.gz"
echo "downloading ${URL}"
curl -fsSL "${URL}" -o "${TMP}/basepod.tar.gz"
tar -C "${TMP}" -xzf "${TMP}/basepod.tar.gz"

if [ ! -w "${PREFIX}" ]; then
  yellow "installing to ${PREFIX} requires sudo"
  sudo install -m 0755 "${TMP}/basepod" "${PREFIX}/basepod"
else
  install -m 0755 "${TMP}/basepod" "${PREFIX}/basepod"
fi

green "==> BasePod CLI ${VERSION} installed"
echo
echo "Binary:"
echo "  cli: ${PREFIX}/basepod"
echo
echo "Next:"
echo "  basepod login --server http://<server-ip-or-domain>:<port>"
echo
echo "Examples:"
echo "  basepod login --server http://192.168.1.20:8081"
echo "  basepod login --server https://bp.example.com"
