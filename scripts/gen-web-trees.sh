#!/usr/bin/env bash
# Generate the CLI snapshots used by the in-browser "Try it" page.
# Usage: scripts/gen-web-trees.sh [cli...]   (default: all that are installed)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/www/treemand/static/try/trees"
TM="${TREEMAND:-treemand}"
mkdir -p "$OUT"

# name:depth:extra-args
SPECS=(
  "treemand:3:"
  "git:3:"
  "docker:3:"
  "kubectl:3:"
  "gh:3:"
  "stripe:1:--expand-stubs 1"
  "aws:1:--expand-stubs 1"
)

for spec in "${SPECS[@]}"; do
  IFS=: read -r name depth extra <<<"$spec"
  if [ $# -gt 0 ] && [[ " $* " != *" $name "* ]]; then continue; fi
  if ! command -v "$name" >/dev/null 2>&1 && [ "$name" != treemand ]; then
    echo "skip $name (not installed)"; continue
  fi
  echo "snapshot $name (depth $depth)…"
  # shellcheck disable=SC2086
  "$TM" "$name" --no-cache --depth "$depth" --timeout 900 $extra --output json >"$OUT/$name.json"
  # Precompute for the browser: minify and cap help text (the finder only
  # reads the first 2000 chars; the help pane shows the first screenful).
  python3 -I "$ROOT/scripts/trim_web_tree.py" "$OUT/$name.json"
  du -h "$OUT/$name.json" | awk '{print "  " $1}'
done
