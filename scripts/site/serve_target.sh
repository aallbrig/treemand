# Sourced (POSIX sh) by the site probes (scripts/metrics/probes/site-health.sh,
# site-quality.sh) to pick Chrome and the site they measure. The caller sets
# `root` (the repo root) and `probe_name` (for messages) first; afterwards
# `chrome`, `base` and `target` are set and an EXIT trap stops the local server.
#
# Target, in order of precedence:
#   SITE_BASE=https://treemand.dev/   an already-served site (production)
#   SITE_DIR=<dir>                    a built site directory, served locally
#   (neither)                         build the site as `task site:audit` does
#                                     and serve it on a free local port
# `target` names it for metric subjects (local, production, remote);
# SITE_TARGET overrides the name. Needs node 22+ (.nvmrc), Chrome or Chromium
# (CHROME overrides), and hugo when building.
# shellcheck shell=sh

: "${root:?set root before sourcing serve_target.sh}" "${probe_name:?set probe_name before sourcing serve_target.sh}"
die() { echo "$probe_name: $*" >&2; exit 2; }

mac_chrome="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
[ -x "$mac_chrome" ] || mac_chrome=""
chrome="${CHROME:-$(command -v google-chrome || command -v chromium || command -v chromium-browser || echo "$mac_chrome")}"
[ -n "$chrome" ] || die "set CHROME to a Chrome or Chromium binary"
command -v node >/dev/null || die "node is required"
# The drivers use the global WebSocket client (Node 22+); .nvmrc pins the version.
node_major="$(node -p 'process.versions.node.split(".")[0]')"
[ "$node_major" -ge 22 ] || die "node $node_major is too old; need 22+ (nvm use, per .nvmrc)"

site_out=""
server_pid=""
site_cleanup() {
  [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null || true
  [ -z "$site_out" ] || rm -rf "$site_out"
}
trap site_cleanup EXIT INT TERM

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
    site_out="$(mktemp -d)"
    dir="$site_out"
    echo "$probe_name: building the site" >&2
    # The Try page needs the compiled wasm next to the site.
    bash "$root/cli/treemand/web/wasm/build.sh" "$root/www/treemand/static/try" >/dev/null
    (cd "$root/www/treemand" && hugo --quiet -b / -d "$dir") >&2
    bash "$root/scripts/site_search_index.sh" "$dir" >/dev/null
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
