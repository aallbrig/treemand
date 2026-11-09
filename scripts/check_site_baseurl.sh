#!/usr/bin/env bash
# Build the site at the production baseURL and fail on any root-relative link
# or asset that escapes /treemand/ (e.g. the logo pointing at github.io root).
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
