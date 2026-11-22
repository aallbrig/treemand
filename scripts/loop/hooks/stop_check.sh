#!/bin/sh
# Stop hook for the loop session (.claude/loop-settings.json). A turn may not end
# while a cycle has begun (state.py begin-cycle) without a recorded end state
# (state.py end-cycle, which also writes .loop/cycle-done for the supervisor).
# Blocks once per stop: with stop_hook_active set, Claude is already continuing
# because of this hook, so it allows the stop rather than loop.
set -u
root="$(cd "$(dirname "$0")/../../.." && pwd)"
loop="$root/.loop"
input="$(cat)"
[ "$(printf '%s' "$input" | jq -r '.stop_hook_active // false' 2>/dev/null)" = true ] && exit 0
# end-cycle already ran: the supervisor has its marker; nothing to add.
[ -e "$loop/cycle-done" ] && exit 0
[ -f "$loop/state.json" ] || exit 0
open="$(python3 -I "$root/scripts/loop/state.py" cycle-open)" || exit 0
reason="The loop $open and has no recorded end state. Do not stop mid-cycle or wait for a human. Finish or park the issue, then end the cycle in a state: python3 -I scripts/loop/state.py end-cycle <released|idle|parked|failed|stopped> (stopped when .loop/STOP exists)."
jq -nc --arg r "$reason" '{decision: "block", reason: $r}'
