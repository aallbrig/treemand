#!/usr/bin/env bash
# Exercise the dev session and loop control on an ISOLATED tmux server with a stub
# agent. Never touches the live tmux server, opens a browser or starts a real claude.
# shellcheck disable=SC2015,SC2016,SC2317,SC2329
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
command -v tmux >/dev/null && command -v tmuxinator >/dev/null || { echo "session_test: tmux/tmuxinator missing, skipping"; exit 0; }

S=treemand_dev
tmp="$(mktemp -d)"
export TMUX_TMPDIR="$tmp" TREEMAND_SESSION="$S" TREEMAND_NO_BROWSER=1
unset TMUX
# The stub stands in for claude: it prints a marker and waits until killed.
stub="$tmp/stubclaude"
printf '#!/usr/bin/env bash\necho STUB-AGENT "$@"\nsleep 600 & c=$!\ntrap '"'"'kill $c; exit 0'"'"' TERM\nwait\n' >"$stub"
chmod +x "$stub"
export LOOP_CLAUDE_BIN="$stub"
cleanup() { bash "$root/scripts/dev_session.sh" stop >/dev/null 2>&1; tmux kill-server >/dev/null 2>&1; pkill -f "$stub" 2>/dev/null; rm -rf "$tmp"; rm -f "$root/.loop/STOP"; }
trap cleanup EXIT

fail=0
check() { if ! eval "$2"; then echo "session_test FAIL: $1" >&2; fail=1; fi; }
wait_for() { for _ in $(seq 1 40); do eval "$1" && return 0; sleep 0.5; done; return 1; }
loop_panes() { tmux list-panes -t "=$S:loops" -F '#{pane_id}' | wc -l | tr -d ' '; }
agent_panes() { tmux list-panes -s -t "=$S" -F '#{@treemand_role}' | grep -c '^agent$'; }
agent_id() { tmux list-panes -s -t "=$S" -F '#{pane_id} #{@treemand_role}' | awk '$2=="agent"{print $1; exit}'; }

rm -f "$root/.loop/STOP"
bash "$root/scripts/dev_session.sh" start >/dev/null 2>&1
check "session exists" 'tmux has-session -t "=$S"'
check "windows bootstrap,dev,watch,site,prod,loops" '[ "$(tmux list-windows -t "=$S" -F "#{window_name}" | tr "\n" ,)" = "bootstrap,dev,watch,site,prod,loops," ]'
check "loops window starts with only the dev shell" '[ "$(loop_panes)" = 1 ] && [ "$(agent_panes)" = 0 ]'
check "status says not running before start" 'out="$(bash "$root/scripts/loop/loopctl.sh" status)"; [[ "$out" == *"supervisor: not running"* ]]'

bash "$root/scripts/loop/loopctl.sh" start >/dev/null 2>&1
check "start creates one agent pane in loops" '[ "$(loop_panes)" = 2 ] && [ "$(agent_panes)" = 1 ]'
check "agent pane is titled agent" '[ "$(tmux display-message -p -t "$(agent_id)" "#{pane_title}")" = agent ]'
check "supervisor starts the stub agent" 'wait_for "tmux capture-pane -p -J -t \"\$(agent_id)\" | grep -q STUB-AGENT"'
check "status says running" 'out="$(bash "$root/scripts/loop/loopctl.sh" status)"; [[ "$out" == *"supervisor: running"* ]]'
bash "$root/scripts/loop/loopctl.sh" start >/dev/null 2>&1
check "second start adds no pane" '[ "$(agent_panes)" = 1 ]'

bash "$root/scripts/loop/loopctl.sh" stop >/dev/null 2>&1
check "stop file written" '[ -e "$root/.loop/STOP" ]'
check "graceful stop leaves the cycle running" '[ "$(agent_panes)" = 1 ]'
pkill -TERM -f "$stub"   # the agent session ends; the supervisor sees STOP
check "agent pane closes once the supervisor exits" 'wait_for "[ \"\$(agent_panes)\" = 0 ]"'
check "dev shell pane remains" '[ "$(loop_panes)" = 1 ]'

bash "$root/scripts/loop/loopctl.sh" start >/dev/null 2>&1
check "start clears stop file" '[ ! -e "$root/.loop/STOP" ]'
check "start recreates the agent pane" '[ "$(agent_panes)" = 1 ]'
check "start relaunches the agent" 'wait_for "tmux capture-pane -p -J -t \"\$(agent_id)\" | grep -q STUB-AGENT"'

bash "$root/scripts/loop/loopctl.sh" stop --now >/dev/null 2>&1
check "stop --now kills the agent" 'wait_for "! pgrep -f $stub >/dev/null"'
check "stop --now removes the agent pane" 'wait_for "[ \"\$(agent_panes)\" = 0 ]"'
check "stop --now writes the stop file" '[ -e "$root/.loop/STOP" ]'

bash "$root/scripts/dev_session.sh" stop >/dev/null 2>&1
check "session gone after stop" '! tmux has-session -t "=$S" 2>/dev/null'
[ "$fail" = 0 ] && echo "session_test: ok"
exit "$fail"
