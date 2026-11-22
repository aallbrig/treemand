#!/bin/sh
# Exercise the dev session layout on an ISOLATED tmux server. Never touches the
# live tmux server, opens a browser or starts a real claude. Loop control and the
# supervisor are covered by supervisor_test.sh.
# shellcheck disable=SC2016,SC2317,SC2329
set -u
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"
{ command -v tmux >/dev/null && command -v tmuxinator >/dev/null; } || { echo "session_test: tmux/tmuxinator missing, skipping"; exit 0; }

S=treemand_dev
tmp="$(mktemp -d)"
# LOOP_HOME points at an empty temp path, so the loops window's `task loop:resume`
# finds the loop disabled whatever the real host's loop is doing.
export TMUX_TMPDIR="$tmp" TREEMAND_SESSION="$S" TREEMAND_NO_BROWSER=1 LOOP_HOME="$tmp/loop-home"
unset TMUX
cleanup() { sh "$root/scripts/dev_session.sh" stop >/dev/null 2>&1; tmux kill-server >/dev/null 2>&1; rm -rf "$tmp"; }
trap cleanup EXIT

fail=0
check() { if ! eval "$2"; then echo "session_test FAIL: $1" >&2; fail=1; fi; }
loop_panes() { tmux list-panes -t "=$S:loops" -F '#{pane_id}' | wc -l | tr -d ' '; }
agent_panes() { tmux list-panes -s -t "=$S" -F '#{@treemand_role}' | grep -c '^agent$'; }

sh "$root/scripts/dev_session.sh" start >/dev/null 2>&1
check "session exists" 'tmux has-session -t "=$S"'
check "windows bootstrap,dev,watch,site,prod,loops" '[ "$(tmux list-windows -t "=$S" -F "#{window_name}" | tr "\n" ,)" = "bootstrap,dev,watch,site,prod,loops," ]'
check "prod window has the site, pipelines, aws and downloads panes" '[ "$(tmux list-panes -t "=$S:prod" -F "#{pane_title}" | tr "\n" ,)" = "site,pipelines,aws,downloads," ]'
sleep 3 # let the loops shell run `task loop:resume`
check "loops window holds only the dev shell while the loop is disabled" '[ "$(loop_panes)" = 1 ] && [ "$(agent_panes)" = 0 ]'
check "status says disabled and not running" 'out="$(sh "$root/scripts/loop/loopctl.sh" status)"; case "$out" in *"enabled: no"*"supervisor: not running"*) true ;; *) false ;; esac'
check "resume leaves a disabled loop alone" 'sh "$root/scripts/loop/loopctl.sh" resume | grep -q "not enabled" && [ ! -e "$LOOP_HOME" ]'

sh "$root/scripts/dev_session.sh" stop >/dev/null 2>&1
check "session gone after stop" '! tmux has-session -t "=$S" 2>/dev/null'
[ "$fail" = 0 ] && echo "session_test: ok"
exit "$fail"
