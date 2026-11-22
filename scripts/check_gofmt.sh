#!/bin/sh
# Fail if any Go file needs formatting (a check, never a rewrite: see #178).
set -eu
cd "$(dirname "$0")/.."
files="$(gofmt -l ./cli/treemand)"
if [ -n "$files" ]; then
  echo "needs gofmt (run: task fmt, then re-stage):" >&2
  printf '%s\n' "$files" >&2
  exit 1
fi
