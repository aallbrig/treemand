#!/bin/sh
# Build the site under a sub-path and fail on any root-relative link or asset
# that escapes it: every internal URL must go through relURL/absURL.
set -eu
cd "$(dirname "$0")/../www/treemand"
out="$(mktemp -d)"
trap 'rm -rf "$out"' EXIT
hugo --quiet -b /treemand/ -d "$out"
bad="$(grep -rhoE '(href|src)="/([^/"][^"]*)?"' "$out" --include='*.html' | grep -vE '="/treemand(/|")' | sort -u || true)"
if [ -n "$bad" ]; then
  echo "root-relative URLs outside /treemand/:" >&2
  echo "$bad" >&2
  exit 1
fi
# The agent files (llms.txt, llms-full.txt) exist, point inside the base URL
# and carry no unrendered shortcodes.
for f in llms.txt llms-full.txt; do
  [ -s "$out/$f" ] || { echo "missing $f" >&2; exit 1; }
  if grep -q '{{<' "$out/$f"; then echo "$f contains an unrendered shortcode" >&2; exit 1; fi
done
grep -q '(/treemand/getting-started/)' "$out/llms.txt" ||
  { echo "llms.txt does not link the docs under the base URL" >&2; exit 1; }
echo "site baseURL check passed"
