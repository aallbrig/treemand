#!/bin/sh
# Design evidence for the treemand-site-product-design skill: screenshots every
# sitemap page of a treemand site at desktop (1280x800) and mobile (390x844), in
# light and dark, and the home and install pages of the peer developer-tool
# sites in scripts/site/design_peers.tsv, into .loop/design/<UTC date>/
# (gitignored; never commit the screenshots).
#
#   sh scripts/site/design_evidence.sh                       production (https://treemand.dev/) + peers
#   sh scripts/site/design_evidence.sh --target <url>        another served site
#   sh scripts/site/design_evidence.sh --dir <built site>    a local build (task site:build: www/treemand/public)
#   options: --out <dir>  --no-peers   env: PEERS_FILE, CHROME, DESIGN_TILES, DESIGN_PEER_TILES
#
# Each page is loaded once in headless Chrome (scripts/site/cdp.mjs); the four
# views re-render that load. Needs node 22+ (.nvmrc) and Chrome or Chromium.
set -eu
root="$(cd "$(dirname "$0")/../.." && pwd)"
die() { echo "design-evidence: $*" >&2; exit 2; }

target="https://treemand.dev/"
dir=""
out=""
peers="${PEERS_FILE:-$root/scripts/site/design_peers.tsv}"
while [ $# -gt 0 ]; do
  case "$1" in
    --target) [ $# -ge 2 ] || die "--target needs a URL"; target="$2"; shift 2 ;;
    --dir) [ $# -ge 2 ] || die "--dir needs a directory"; dir="$2"; shift 2 ;;
    --out) [ $# -ge 2 ] || die "--out needs a directory"; out="$2"; shift 2 ;;
    --no-peers) peers=""; shift ;;
    -h | --help) sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -z "$peers" ] || [ -f "$peers" ] || die "peer list $peers not found"
out="${out:-$root/.loop/design/$(date -u +%Y-%m-%d)}"

mac_chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
[ -x "$mac_chrome" ] || mac_chrome=""
chrome="${CHROME:-$(command -v google-chrome || command -v chromium || command -v chromium-browser || echo "$mac_chrome")}"
[ -n "$chrome" ] || die "set CHROME to a Chrome or Chromium binary"
command -v node >/dev/null || die "node is required"
node_major="$(node -p 'process.versions.node.split(".")[0]')"
[ "$node_major" -ge 22 ] || die "node $node_major is too old; need 22+ (nvm use, per .nvmrc)"

server_pid=""
cleanup() { [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true; }
trap cleanup EXIT INT TERM

if [ -n "$dir" ]; then
  [ -d "$dir" ] || die "$dir is not a directory"
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
  target="http://127.0.0.1:$port/"
fi

# Replace an earlier run's evidence, and nothing else.
if [ -d "$out" ] && [ -n "$(ls -A "$out")" ]; then
  [ -f "$out/manifest.json" ] || die "$out is not empty and holds no earlier evidence; pick another --out"
  rm -rf "$out"
fi
mkdir -p "$out"
echo "design-evidence: capturing $target into $out" >&2
OUT="$out" BASE="$target" PEERS="$peers" CHROME="$chrome" node "$root/scripts/site/design_evidence.mjs"
echo "$out"
