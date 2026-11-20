#!/usr/bin/env bash
# Upgrades the dependencies of every Go module in the repo, then checks that
# each module still builds, is free of known vulnerabilities, and can still be
# built by the golang image of its Dockerfile.
#
# This only covers Go modules. Other ecosystems are updated by Dependabot
# (.github/dependabot.yml) or the upgrade-examples skill
# (.claude/skills/upgrade-examples/SKILL.md).
#
# Upgrading a module is not the same as the example still working: afterwards
# run `nbe verify go cli` (or `nbe verify all`) before committing.
#
# Exits non-zero if any step failed.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# Upgrade with the same Go release the example images build with, so that
# dependencies needing a newer Go are selected (and then caught by
# check-go-versions.sh) consistently, whatever Go is installed locally.
# The golang:X.Y images float to the latest X.Y patch release, so use that.
IMAGE_GO="$(grep -oE '^FROM golang:[0-9]+\.[0-9]+' docker/go/Dockerfile | sed 's/.*golang://')"
LATEST_PATCH="$(curl -fsSL 'https://go.dev/dl/?mode=json&include=all' 2>/dev/null \
  | grep -oE "\"go${IMAGE_GO//./\\.}(\.[0-9]+)?\"" | tr -d '"' | sort -V | tail -n1 || true)"
export GOTOOLCHAIN="${GOTOOLCHAIN_OVERRIDE:-${LATEST_PATCH:-go${IMAGE_GO}.0}}"
echo "Using GOTOOLCHAIN=$GOTOOLCHAIN (golang:$IMAGE_GO from docker/go/Dockerfile)"

export PATH="$PATH:$(go env GOPATH)/bin"
if ! command -v govulncheck &>/dev/null; then
  echo "Installing govulncheck..."
  go install golang.org/x/vuln/cmd/govulncheck@latest
fi

FAILED=()

# Scan the built binary rather than the source: source mode type-checks the
# standard library at the module's go version, which fails when the
# toolchain is newer than the go directive.
vulnscan() {
  local bin
  bin="$(mktemp)"
  go build -o "$bin" . && govulncheck -mode=binary "$bin"
  local rc=$?
  rm -f "$bin"
  return $rc
}

# Keep docker/go/go.mod's dependency list complete before tidying it.
scripts/sync-go-deps.sh

run() {
  # run <module> <description> <command...>
  local mod="$1" what="$2"
  shift 2
  echo "  $what..."
  if ! (cd "$mod" && "$@" 2>&1 | sed 's/^/    /'; exit "${PIPESTATUS[0]}"); then
    echo "  ✗ $what failed"
    FAILED+=("$mod: $what")
    return 1
  fi
}

while IFS= read -r gomod; do
  mod="$(dirname "$gomod")"
  echo ""
  echo "=== $mod ==="
  run "$mod" "upgrade dependencies" go get -u ./... || continue
  run "$mod" "tidy" go mod tidy || continue
  run "$mod" "build" go build -o /dev/null ./... || continue
  run "$mod" "vulnerability scan" vulnscan || true
done < <(find . -name go.mod -not -path '*/vendor/*' | sed 's|^\./||' | sort)

echo ""
echo "=== go.work ==="
run . "sync workspace" go work sync || true

echo ""
echo "=== Go versions vs Docker images ==="
if ! scripts/check-go-versions.sh; then
  echo "  ✗ an upgraded go.mod needs a newer golang image; bump the FROM golang:... lines"
  FAILED+=("go directive newer than Docker image")
fi

echo ""
if [ "${#FAILED[@]}" -gt 0 ]; then
  echo "Done, with failures:"
  printf '  - %s\n' "${FAILED[@]}"
  exit 1
fi
echo "Done. Next: nbe verify go cli"
