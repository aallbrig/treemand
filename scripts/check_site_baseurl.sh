#!/usr/bin/env bash
# Build the site under a sub-path and fail on any root-relative link or asset
# that escapes it: every internal URL must go through relURL/absURL.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../www/treemand"
out="$(mktemp -d)"
trap 'rm -rf "$out"' EXIT
hugo --quiet -b /treemand/ -d "$out"
bad="$(grep -rhoE '(href|src)="/([^/"][^"]*)?"' "$out" --include='*.html' | grep -vE '="/treemand(/|")' | sort -u || true)"
if [ -n "$bad" ]; then
  echo "root-relative URLs outside /treemand/:" >&2
  echo "$bad" >&2
  exit 1
fi
echo "site baseURL check passed"
