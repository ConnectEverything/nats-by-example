#!/usr/bin/env bash
# Prints a GitHub Actions matrix (JSON) of example verification jobs, one per
# language, each with the `nbe verify` targets for that language.
#
# Usage:
#   scripts/verify-plan.sh all                 # every example, grouped by language
#   scripts/verify-plan.sh go rust             # only these languages
#   scripts/verify-plan.sh --changed <base>    # only what changed since <base>
#
# With --changed, a change to shared infrastructure (docker/docker-compose*.yaml,
# cmd/nbe) verifies everything, a change under docker/<lang>/ verifies all
# examples for that language, and a change under examples/<cat>/<ex>/<client>/
# verifies just that client.
set -euo pipefail

cd "$(dirname "$0")/.."

all_langs() {
  find examples -mindepth 3 -maxdepth 3 -type d | awk -F/ '{print $4}' | sort -u
}

emit() {
  # stdin: lines of "<lang> <target>"
  sort -u | awk '
    { t[$1] = (t[$1] == "" ? $2 : t[$1] " " $2) }
    END {
      printf "{\"include\":["
      n = 0
      for (l in t) { printf "%s{\"name\":\"%s\",\"targets\":\"%s\"}", (n++ ? "," : ""), l, t[l] }
      printf "]}\n"
    }'
}

if [ "${1:-all}" = "--changed" ]; then
  base="${2:?base ref required}"
  changed="$(git diff --name-only "$base"...HEAD)"
  {
    while IFS= read -r f; do
      case "$f" in
        docker/docker-compose*.yaml|cmd/nbe/*|.github/workflows/verify-examples.yaml|scripts/verify-plan.sh)
          all_langs | awk '{print $1, $1}' ;;
        docker/*/*)
          lang="$(echo "$f" | cut -d/ -f2)"
          if [ -n "$(find examples -mindepth 3 -maxdepth 3 -type d -name "$lang" -print -quit)" ]; then
            echo "$lang $lang"
          fi ;;
        examples/*/*/*/*)
          dir="$(echo "$f" | cut -d/ -f1-4)"
          [ -d "$dir" ] && echo "$(basename "$dir") $dir" ;;
      esac
    done <<< "$changed"
  } | awk '
    # A whole-language target supersedes individual examples of that language.
    { lines[NR] = $0; if ($1 == $2) whole[$1] = 1 }
    END { for (i = 1; i <= NR; i++) { split(lines[i], p, " "); if (!whole[p[1]] || p[1] == p[2]) print lines[i] } }
  ' | emit
  exit 0
fi

if [ "$#" -eq 0 ] || [ "$1" = "all" ]; then
  set -- $(all_langs)
fi
for l in "$@"; do echo "$l $l"; done | emit
