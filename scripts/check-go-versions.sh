#!/usr/bin/env bash
# Checks that every go.mod's `go` directive can be built by the golang image
# in the Dockerfile that builds it. The official golang images set
# GOTOOLCHAIN=local, so a go.mod that asks for a newer Go than the image
# fails with "go.mod requires go >= X (running go Y; GOTOOLCHAIN=local)".
#
# This is what broke every Go example after `go get -u` raised the go
# directive to 1.25 while the images were still golang:1.21.
set -euo pipefail

cd "$(dirname "$0")/.."

# version_le A B: true if A <= B (dotted versions).
version_le() {
  [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" = "$1" ]
}

status=0

while IFS= read -r dockerfile; do
  dir="$(dirname "$dockerfile")"
  # A virtual entry (see below) means the client is built by docker/go.
  [ -f "$dockerfile" ] || dockerfile=docker/go/Dockerfile
  # Lowest Go version among golang build stages, e.g. golang:1.27-alpine3.23 -> 1.27
  image_go="$(grep -ioE '^FROM[[:space:]]+(--platform=[^[:space:]]+[[:space:]]+)?golang:[0-9]+\.[0-9]+(\.[0-9]+)?' "$dockerfile" \
    | sed -E 's/.*golang://' | sort -V | head -n1 || true)"
  [ -z "$image_go" ] && continue

  # go.mod files built by this Dockerfile: those in its directory tree, or
  # the shared docker/go/go.mod for Go example clients without their own.
  mods="$(find "$dir" -name go.mod -not -path '*/vendor/*')"
  if [ -z "$mods" ] && { [ "$(basename "$dir")" = "go" ]; }; then
    mods="docker/go/go.mod"
  fi

  for mod in $mods; do
    want="$(awk '$1 == "go" {print $2; exit}' "$mod")"
    [ -z "$want" ] && continue
    if version_le "$want" "$image_go"; then
      echo "ok    $mod (go $want) <= $dockerfile (golang:$image_go)"
    else
      echo "FAIL  $mod requires go $want but $dockerfile builds with golang:$image_go" >&2
      status=1
    fi
  done
done < <(
  find docker examples -name Dockerfile
  # Go example clients with their own go.mod but no Dockerfile are built by
  # docker/go/Dockerfile (nbe merges docker/go and the client directory).
  for d in examples/*/*/go; do
    if [ -f "$d/go.mod" ] && [ ! -f "$d/Dockerfile" ]; then echo "$d/Dockerfile"; fi
  done | sort -u
)

exit $status
