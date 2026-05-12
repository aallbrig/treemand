#!/usr/bin/env bash
# Record per-subcommand VHS tapes into GIFs under www/treemand/static/demos/.
#
# Usage:
#   bash scripts/record-subcmd-gifs.sh
#
# Prerequisites: vhs, treemand (on $PATH)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SUBCMDS_DIR="$REPO_ROOT/demos/subcmds"
OUTPUT_DIR="$REPO_ROOT/www/treemand/static/demos"

for cmd in vhs treemand; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "Error: $cmd is not installed or not on \$PATH" >&2
    exit 1
  fi
done

mkdir -p "$OUTPUT_DIR"

echo "==> Recording per-subcommand GIFs..."
for tape in "$SUBCMDS_DIR"/cmd_*.tape; do
  name=$(basename "$tape" .tape)
  echo "  Recording: $name"
  (cd "$REPO_ROOT" && vhs "$tape")
done

echo "==> Done. GIFs written to $OUTPUT_DIR:"
ls -lh "$OUTPUT_DIR"/*.gif 2>/dev/null || echo "  (no GIFs found — check VHS output above)"
