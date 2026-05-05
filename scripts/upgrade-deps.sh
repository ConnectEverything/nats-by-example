#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Ensure govulncheck is installed
if ! command -v govulncheck &>/dev/null; then
  echo "Installing govulncheck..."
  go install golang.org/x/vuln/cmd/govulncheck@latest
fi

MODULES=()
while IFS= read -r mod; do
  MODULES+=("$(dirname "$mod")")
done < <(find "$REPO_ROOT" -name go.mod -not -path '*/vendor/*')

VULN_FOUND=0

for dir in "${MODULES[@]}"; do
  rel="${dir#"$REPO_ROOT"/}"
  echo ""
  echo "=== $rel ==="

  # Upgrade all direct+indirect dependencies
  echo "  Upgrading all dependencies..."
  (cd "$dir" && go get -u ./... 2>&1 | sed 's/^/  /' || true)
  (cd "$dir" && go mod tidy 2>&1 | sed 's/^/  /' || true)

  # Check for known vulnerabilities
  echo "  Scanning for vulnerabilities..."
  if (cd "$dir" && govulncheck ./... 2>&1 | sed 's/^/  /'); then
    echo "  No vulnerabilities found."
  else
    echo "  ⚠ Vulnerabilities detected (may require manual attention)."
    VULN_FOUND=1
  fi
done

# Sync the workspace
echo ""
echo "=== Syncing go.work ==="
(cd "$REPO_ROOT" && go work sync 2>&1 || true)

echo ""
if [ "$VULN_FOUND" -eq 1 ]; then
  echo "Done. Some modules still have vulnerabilities that may need manual fixes."
else
  echo "Done. All modules upgraded and vulnerability-free."
fi
