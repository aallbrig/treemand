#!/usr/bin/env bash
# Build the site at its production base path (/treemand/), serve it locally and
# run scripts/site_audit.mjs against every page in the sitemap at desktop and
# mobile sizes. Needs hugo, node (24 or newer) and Chrome or Chromium; run via
# `task site:audit`. Exits non-zero on any finding.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
chrome="${CHROME:-$(command -v google-chrome || command -v chromium || command -v chromium-browser || true)}"
[ -n "$chrome" ] || { echo "site_audit: set CHROME to a Chrome or Chromium binary" >&2; exit 2; }
command -v hugo >/dev/null || { echo "site_audit: hugo is required" >&2; exit 2; }
command -v node >/dev/null || { echo "site_audit: node is required" >&2; exit 2; }

out="$(mktemp -d)"
server_pid=""
cleanup() { [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true; rm -rf "$out"; }
trap cleanup EXIT

# The Try page needs the compiled wasm next to the site.
bash cli/treemand/web/wasm/build.sh www/treemand/static/try >/dev/null
(cd www/treemand && hugo --quiet -b /treemand/ -d "$out/treemand")

port="${AUDIT_PORT:-18181}"
python3 -m http.server "$port" --bind 127.0.0.1 --directory "$out" >/dev/null 2>&1 &
server_pid=$!
for _ in $(seq 1 50); do
  curl -sf "http://127.0.0.1:$port/treemand/" >/dev/null && break
  sleep 0.1
done

BASE="http://127.0.0.1:$port/treemand/" CHROME="$chrome" node scripts/site_audit.mjs
