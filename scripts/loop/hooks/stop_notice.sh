#!/bin/sh
# PostToolUse hook for the loop session (.claude/loop-settings.json), run after
# every tool call, so it stays a file test: `task loop:stop` writes .loop/STOP and
# this relays it to the agent at its next tool call.
cat >/dev/null
[ -e "$(dirname "$0")/../../../.loop/STOP" ] || exit 0
printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"treemand loop: a stop was requested (.loop/STOP). Finish or park the current issue, remove its worktree, unclaim it (bash scripts/loop/issues.sh unclaim <n>), run python3 -I scripts/loop/state.py end-cycle stopped, then reply '"'"'stopped'"'"' and take no further action."}}'
