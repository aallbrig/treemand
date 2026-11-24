#!/bin/sh
# PostToolUse hook for the loop session (.claude/loop-settings.json). It runs
# after every tool call, so it stays a few file operations and no subprocesses:
#   - heartbeat: touches .loop/heartbeat; the supervisor reads its age to spot a
#     stalled session.
#   - stop relay: `task loop:stop` writes .loop/STOP and this tells the agent at
#     its next tool call.
#   - context note: once the session's context passes LOOP_CONTEXT_NOTE_PCT
#     (.loop/context, written by statusline.sh), one note to keep the rest of the
#     cycle lean. session_start.sh re-arms it for each new context.
cat >/dev/null
loop="$(dirname "$0")/../../../.loop"
[ -d "$loop" ] || exit 0
: >"$loop/heartbeat"
if [ -e "$loop/STOP" ]; then
  printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"treemand loop: a stop was requested (.loop/STOP). Finish or park the current issue, remove its worktree, unclaim it (bash scripts/loop/issues.sh unclaim <n>), run python3 -I scripts/loop/state.py end-cycle stopped, then reply '"'"'stopped'"'"' and take no further action."}}'
  exit 0
fi
[ -f "$loop/context" ] && [ ! -e "$loop/context-noted" ] || exit 0
LOOP_CONTEXT_NOTE_PCT=50
# shellcheck source=scripts/loop/config.defaults.env
. "$(dirname "$0")/../config.defaults.env"
# shellcheck disable=SC1091
[ -f "$loop/config.env" ] && . "$loop/config.env"
read -r pct _ <"$loop/context" || exit 0
[ "${pct:-0}" -ge "$LOOP_CONTEXT_NOTE_PCT" ] 2>/dev/null || exit 0
: >"$loop/context-noted"
printf '{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"treemand loop: this session has used %s%% of its context. Finish the current step before starting another, delegate wide searches and reviews to subagents, and read only the lines you need. At %s%% the supervisor parks the cycle: work in progress is pushed to the branch and the next cycle resumes it."}}\n' "$pct" "${LOOP_CONTEXT_PARK_PCT:-75}"
