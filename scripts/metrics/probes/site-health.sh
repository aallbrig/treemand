#!/bin/sh
# Probe: site runtime health. Crawls every same-origin page reachable from the
# sitemap and from links in headless Chrome (scripts/metrics/site_health.mjs)
# and counts HTTP errors, console errors, uncaught exceptions, broken internal
# and external links, and mixed content.
#
# Target, in order of precedence:
#   SITE_BASE=https://treemand.dev/   an already-served site (production)
#   SITE_DIR=<dir>                    a built site directory, served locally
#   (neither)                         build the site as `task site:audit` does
#                                     and serve it on a free local port
# Metrics carry the target as their subject (pages.local, pages.production;
# SITE_TARGET overrides the name). Needs node 22+ (.nvmrc), Chrome or Chromium
# (CHROME overrides), and hugo when building. Prints the probe JSON on stdout; progress goes to stderr.
set -eu
root="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$root"
die() { echo "site-health: $*" >&2; exit 2; }

mac_chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
[ -x "$mac_chrome" ] || mac_chrome=""
chrome="${CHROME:-$(command -v google-chrome || command -v chromium || command -v chromium-browser || echo "$mac_chrome")}"
[ -n "$chrome" ] || die "set CHROME to a Chrome or Chromium binary"
command -v node >/dev/null || die "node is required"
# The driver uses the global WebSocket client (Node 22+); .nvmrc pins the version.
node_major="$(node -p 'process.versions.node.split(".")[0]')"
[ "$node_major" -ge 22 ] || die "node $node_major is too old; need 22+ (nvm use, per .nvmrc)"

out=""
server_pid=""
cleanup() {
  [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true
  [ -z "$out" ] || rm -rf "$out"
}
trap cleanup EXIT INT TERM

base="${SITE_BASE:-}"
case "$base" in
  "") target=local ;;
  https://treemand.dev | https://treemand.dev/*) target=production ;;
  *) target=remote ;;
esac
target="${SITE_TARGET:-$target}"
if [ -z "$base" ]; then
  dir="${SITE_DIR:-}"
  if [ -z "$dir" ]; then
    command -v hugo >/dev/null || die "hugo is required to build the site (or set SITE_BASE or SITE_DIR)"
    out="$(mktemp -d)"
    dir="$out"
    echo "site-health: building the site" >&2
    # The Try page needs the compiled wasm next to the site.
    bash cli/treemand/web/wasm/build.sh www/treemand/static/try >/dev/null
    (cd www/treemand && hugo --quiet -b / -d "$dir") >&2
    bash scripts/site_search_index.sh "$dir" >/dev/null
  fi
  [ -d "$dir" ] || die "SITE_DIR $dir is not a directory"
  # A free port: a fixed one can be held by a server another run left behind.
  port="$(python3 -I -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
  python3 -I -m http.server "$port" --bind 127.0.0.1 --directory "$dir" >/dev/null 2>&1 &
  server_pid=$!
  i=0
  while [ "$i" -lt 50 ]; do
    curl -sf "http://127.0.0.1:$port/" >/dev/null && break
    sleep 0.1
    i=$((i + 1))
  done
  kill -0 "$server_pid" 2>/dev/null || die "the local server on port $port did not start"
  base="http://127.0.0.1:$port/"
fi

BASE="$base" TARGET="$target" CHROME="$chrome" node scripts/metrics/site_health.mjs
