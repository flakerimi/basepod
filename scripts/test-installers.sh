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

echo "installer tests passed"
