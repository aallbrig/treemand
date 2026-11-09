#!/usr/bin/env bash
# Produce visual evidence for review under .loop/shots/:
#   tui-<cols>x<rows>.txt   the real TUI captured from an isolated tmux server (ANSI kept)
#   site-<name>.png         the built Hugo site at desktop and mobile widths
# Exits non-zero if the TUI fails to render or the site fails to build/serve.
# The judging against docs/design rubric is done by the loop (it reads the files).
set -euo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$LOOP_ROOT"

shots="$LOOP_DIR/shots"
rm -rf "$shots"
mkdir -p "$shots"
sock="treemand-visual-$$"
tm() { tmux -L "$sock" "$@"; }
server_pid=""
cleanup() {
  tm kill-server 2>/dev/null || true
  if [ -n "$server_pid" ]; then kill "$server_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT

bin="$LOOP_DIR/treemand-visual"
go build -o "$bin" ./cli/treemand

# ── TUI ────────────────────────────────────────────────────────────────────
target="${VISUAL_TARGET:-git}"
for size in 80x24 120x40; do
  cols="${size%x*}"
  rows="${size#*x}"
  tm kill-server 2>/dev/null || true
  env -u TMUX tmux -L "$sock" new-session -d -x "$cols" -y "$rows" -s v "$bin -i $target"
  sleep 4
  tm capture-pane -p -e -t v >"$shots/tui-$size.txt"
  if ! tm has-session -t v 2>/dev/null; then
    echo "visual: TUI exited early at $size" >&2
    exit 1
  fi
  [ "$(wc -c <"$shots/tui-$size.txt")" -gt 200 ] || { echo "visual: TUI frame at $size is empty" >&2; exit 1; }
  grep -qi "panic\|fatal error" "$shots/tui-$size.txt" && { echo "visual: panic in TUI at $size" >&2; exit 1; }
done

# ── Site ───────────────────────────────────────────────────────────────────
site_out="$LOOP_DIR/site-build"
rm -rf "$site_out"
(cd www/treemand && hugo --minify --baseURL "/" --destination "$site_out" >/dev/null)

port="${VISUAL_PORT:-18080}"
python3 -m http.server "$port" --directory "$site_out" >/dev/null 2>&1 &
server_pid=$!
sleep 1

pw="npx --yes playwright@1.49.1"
# Idempotent: a no-op once the pinned browser is cached.
$pw install chromium >/dev/null
for page in "" try/ installation/ getting-started/; do
  name="${page%/}"
  name="${name:-home}"
  for vp in "desktop:1280,800" "mobile:390,844"; do
    $pw screenshot --viewport-size="${vp#*:}" --full-page "http://127.0.0.1:$port/$page" \
      "$shots/site-$name-${vp%%:*}.png" >/dev/null
  done
done
echo "visual: evidence written to $shots"
ls "$shots"
