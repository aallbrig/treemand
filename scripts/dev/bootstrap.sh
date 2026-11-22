#!/bin/sh
# Dev-session bootstrap: report which tools `task ci` and the dev session need,
# with install hints, then open the local and production sites in a browser.
# Never fails: it is informational and always exits 0.
#
#   TREEMAND_NO_BROWSER=1       do not open a browser
#   TREEMAND_SITE_URL           local site to wait for (default http://localhost:1313/)
#   TREEMAND_BOOTSTRAP_WAIT     seconds to wait for it (default 60)

site_url="${TREEMAND_SITE_URL:-http://localhost:1313/}"
prod_url="https://treemand.dev"
wait_s="${TREEMAND_BOOTSTRAP_WAIT:-60}"

case "$0" in */*) tools_dir="${0%/*}" ;; *) tools_dir=. ;; esac
tools_dir="$(cd "$tools_dir" && pwd)"
repo_root="$(cd "$tools_dir/../.." && pwd)"
# The tool list is shared with setup.sh: scripts/dev/tools.txt.
# shellcheck source=scripts/dev/tools.sh
. "$tools_dir/tools.sh"

total=0
missing=0
# shellcheck disable=SC2329 # called through tools_each
report_row() {
  total=$((total + 1))
  if tool_present "$1"; then
    printf '  %-8s %s\n' ok "$1"
    return 0
  fi
  missing=$((missing + 1))
  hint=""
  if [ "$3" != - ]; then hint="$(pkg_cmd brew "$3") | "; fi
  if [ "$4" != - ]; then hint="${hint}apt install $4"; else hint="$hint$(fallback_cmd "$6")"; fi
  printf '  %-8s %-14s %s\n' missing "$1" "$hint"
}

report() {
  echo "treemand dev host readiness (tools for task ci and the dev session)"
  tools_each report_row
  echo "$missing of $total tools missing"
  if [ "$missing" -gt 0 ]; then echo "(task setup -- --install installs them and the git hooks)"; fi
}

site_up() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsS -o /dev/null --max-time 2 "$site_url" >/dev/null 2>&1
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O /dev/null -T 2 "$site_url" >/dev/null 2>&1
  else
    return 1
  fi
}

opener() {
  if [ "$(uname -s 2>/dev/null)" = Darwin ]; then
    command -v open >/dev/null 2>&1 && echo open
  else
    command -v xdg-open >/dev/null 2>&1 && echo xdg-open
  fi
}

browse() {
  [ -n "${TREEMAND_NO_BROWSER:-}" ] && return 0
  cmd="$(opener)"
  [ -n "$cmd" ] || return 0
  echo
  echo "waiting up to ${wait_s}s for $site_url"
  waited=0
  up=0
  while [ "$waited" -lt "$wait_s" ]; do
    if site_up; then up=1; break; fi
    sleep 1
    waited=$((waited + 1))
  done
  if [ "$up" = 1 ]; then
    "$cmd" "$site_url" >/dev/null 2>&1 || true
    echo "opened $site_url"
  else
    echo "local site not up after ${wait_s}s (see the site window); skipping it"
  fi
  "$cmd" "$prod_url" >/dev/null 2>&1 || true
  echo "opened $prod_url"
}

report
browse
exit 0
