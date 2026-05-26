#!/bin/sh
set -eu

fail() {
  echo "installer test failed: $*" >&2
  exit 1
}

test -f scripts/install.sh || fail "scripts/install.sh is missing"
test -f scripts/install-cli.sh || fail "scripts/install-cli.sh is missing"

sh -n scripts/install.sh
sh -n scripts/install-cli.sh

grep -q 'basepod-server' scripts/install.sh || fail "full installer should install basepod-server"
grep -q 'launchctl load' scripts/install.sh || fail "full installer should load launchd service"
grep -q 'BASEPOD_HTTP_ADDR' scripts/install.sh || fail "full installer should set the server HTTP address"
grep -q 'choose_http_port' scripts/install.sh || fail "full installer should choose an available HTTP port"
grep -q 'detect_browser_host' scripts/install.sh || fail "full installer should print a browser-reachable host"
grep -q 'SSH_CONNECTION' scripts/install.sh || fail "full installer should account for SSH installs"

grep -q 'basepod"' scripts/install-cli.sh || fail "CLI installer should install basepod"
if grep -q 'basepod-server' scripts/install-cli.sh; then
  fail "CLI installer must not install basepod-server"
fi
if grep -q 'launchctl' scripts/install-cli.sh; then
  fail "CLI installer must not manage launchd"
fi
if grep -q 'podman' scripts/install-cli.sh; then
  fail "CLI installer must not require Podman"
fi
grep -q '<server-ip-or-domain>:<port>' scripts/install-cli.sh || fail "CLI installer should not assume localhost:8080"

echo "installer tests passed"
