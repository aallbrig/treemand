#!/usr/bin/env bash
# Build the site at its production base path (/), serve it locally and
# run scripts/site_audit.mjs against every page in the sitemap at desktop and
# mobile sizes. Needs hugo, node (24 or newer) and Chrome or Chromium; run via
# `task site:audit`. Exits non-zero on any finding.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
mac_chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
[ -x "$mac_chrome" ] || mac_chrome=""
chrome="${CHROME:-$(command -v google-chrome || command -v chromium || command -v chromium-browser || echo "$mac_chrome")}"
[ -n "$chrome" ] || { echo "site_audit: set CHROME to a Chrome or Chromium binary" >&2; exit 2; }
command -v hugo >/dev/null || { echo "site_audit: hugo is required" >&2; exit 2; }
command -v node >/dev/null || { echo "site_audit: node is required" >&2; exit 2; }
# The audit uses the global WebSocket client (Node 22+); .nvmrc pins the version.
node_major="$(node -p 'process.versions.node.split(".")[0]')"
[ "$node_major" -ge 22 ] || { echo "site_audit: node $node_major is too old; need 22+ (nvm use, per .nvmrc)" >&2; exit 2; }

out="$(mktemp -d)"
server_pid=""
cleanup() { [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true; rm -rf "$out"; }
trap cleanup EXIT

# The Try page needs the compiled wasm next to the site.
bash cli/treemand/web/wasm/build.sh www/treemand/static/try >/dev/null
(cd www/treemand && hugo --quiet -b / -d "$out")
bash scripts/site_search_index.sh "$out" >/dev/null

# A free port by default: a fixed one can be held by a server another run left
# behind, and the audit would then test that server's (stale) files.
port="${AUDIT_PORT:-$(python3 -I -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')}"
python3 -m http.server "$port" --bind 127.0.0.1 --directory "$out" >/dev/null 2>&1 &
server_pid=$!
for _ in $(seq 1 50); do
  curl -sf "http://127.0.0.1:$port/" >/dev/null && break
  sleep 0.1
done
kill -0 "$server_pid" 2>/dev/null || { echo "site_audit: the local server on port $port did not start (port in use?)" >&2; exit 2; }

BASE="http://127.0.0.1:$port/" CHROME="$chrome" node scripts/site_audit.mjs
