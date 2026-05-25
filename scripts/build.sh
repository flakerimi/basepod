#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-0.0.0-dev}"
DIST="${PWD}/dist"
ROOT="${PWD}"

if [[ "$(uname -m)" != "arm64" ]]; then
  echo "warning: building arm64 binary on $(uname -m)" >&2
fi

mkdir -p "${DIST}"

echo "==> building Vue SPA"
(cd web && pnpm install --frozen-lockfile && pnpm build)

echo "==> building basepod-server"
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
  -o "${DIST}/basepod-server" ./cmd/basepod-server

echo "==> building basepod CLI"
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
  -o "${DIST}/basepod" ./cmd/basepod

echo "==> packaging"
TARBALL="${DIST}/basepod-${VERSION}-darwin-arm64.tar.gz"
tar -C "${DIST}" -czf "${TARBALL}" basepod-server basepod
shasum -a 256 "${TARBALL}" > "${TARBALL}.sha256"

echo "done: ${TARBALL}"
