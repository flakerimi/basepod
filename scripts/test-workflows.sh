#!/bin/sh
set -eu

fail() {
  echo "workflow test failed: $*" >&2
  exit 1
}

go_job="$(awk '
  /^  go:/ { in_go = 1 }
  /^  [a-zA-Z0-9_-]+:/ && $1 != "go:" && in_go { in_go = 0 }
  in_go { print }
' .github/workflows/ci.yml)"

printf "%s\n" "$go_job" | grep -q 'make test' || fail "CI go job must run make test"
grep -q '^test: web' Makefile || fail "make test must build web/dist before Go tests"
grep -q 'ARTIFACT_VERSION="${VERSION#v}"' scripts/build.sh || fail "build script must strip leading v from artifact version"
grep -q 'basepod-${ARTIFACT_VERSION}-darwin-arm64.tar.gz' scripts/build.sh || fail "release artifact name must use stripped version"
git ls-files --error-unmatch cmd/basepod/main.go cmd/basepod-server/main.go >/dev/null 2>&1 || fail "command source directories must be tracked"

echo "workflow tests passed"
