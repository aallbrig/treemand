#!/usr/bin/env bash
# Exercise the dev session and loop control on an ISOLATED tmux server with a stub
# agent. Never touches the live tmux server or starts a real claude.
# shellcheck disable=SC2015,SC2016,SC2317,SC2329
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
command -v tmux >/dev/null && command -v tmuxinator >/dev/null || { echo "session_test: tmux/tmuxinator missing, skipping"; exit 0; }

tmp="$(mktemp -d)"
export TMUX_TMPDIR="$tmp" TREEMAND_SESSION=treemand
unset TMUX
stub="$tmp/stubclaude"
printf '#!/usr/bin/env bash\necho STUB-AGENT "$@"\nsleep 600\n' >"$stub"
chmod +x "$stub"
export LOOP_CLAUDE_BIN="$stub"
cleanup() { bash "$root/scripts/dev_session.sh" stop >/dev/null 2>&1; tmux kill-server >/dev/null 2>&1; rm -rf "$tmp"; rm -f "$root/.loop/STOP"; }
trap cleanup EXIT

fail=0
check() { if ! eval "$2"; then echo "session_test FAIL: $1" >&2; fail=1; fi; }
wait_for() { for _ in $(seq 1 40); do eval "$1" && return 0; sleep 0.5; done; return 1; }

rm -f "$root/.loop/STOP"
bash "$root/scripts/dev_session.sh" start >/dev/null 2>&1
check "session exists" 'tmux has-session -t "=treemand"'
check "windows dev,watch,sites,loops" '[ "$(tmux list-windows -t "=treemand" -F "#{window_name}" | tr "\n" ,)" = "dev,watch,sites,loops," ]'
check "supervisor starts the stub agent" 'wait_for "tmux capture-pane -p -J -t \"=treemand:loops.1\" | grep -q STUB-AGENT"'
check "status says running" 'out="$(bash "$root/scripts/loop/loopctl.sh" status)"; [[ "$out" == *"supervisor: running"* ]]'

bash "$root/scripts/loop/loopctl.sh" stop --now >/dev/null 2>&1
check "stop kills the agent" 'wait_for "! pgrep -f $stub >/dev/null"'
check "stop file written" '[ -e "$root/.loop/STOP" ]'

bash "$root/scripts/loop/loopctl.sh" start >/dev/null 2>&1
check "start clears stop file" '[ ! -e "$root/.loop/STOP" ]'
check "start relaunches the agent" 'wait_for "tmux capture-pane -p -J -t \"=treemand:loops.1\" | grep -c STUB-AGENT | grep -q ."'

bash "$root/scripts/dev_session.sh" stop >/dev/null 2>&1
check "session gone after stop" '! tmux has-session -t "=treemand" 2>/dev/null'
[ "$fail" = 0 ] && echo "session_test: ok"
exit "$fail"
