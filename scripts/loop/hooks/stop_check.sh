#!/bin/sh
# Stop hook for the loop session (.claude/loop-settings.json).
#   - A turn may not end while a cycle has begun (state.py begin-cycle) without a
#     recorded end state (state.py end-cycle, which also writes .loop/cycle-done
#     for the supervisor). It blocks up to LOOP_STOP_BLOCKS times in a row; after
#     that it lets the turn end and the supervisor's stall check takes over,
#     rather than spinning forever.
#   - The current issue's worktree (.loop/worktrees/<n>, n from .loop/work) may
#     not be left with uncommitted changes or unpushed commits, open cycle or not;
#     parked work lives on its pushed branch. Blocked once per stop.
set -u
root="$(cd "$(dirname "$0")/../../.." && pwd)"
loop="$root/.loop"
LOOP_STOP_BLOCKS=3
# shellcheck source=scripts/loop/config.defaults.env
. "$root/scripts/loop/config.defaults.env"
# shellcheck disable=SC1091
[ -f "$loop/config.env" ] && . "$loop/config.env"
input="$(cat)"
active="$(printf '%s' "$input" | jq -r '.stop_hook_active // false' 2>/dev/null)"

block() {
  jq -nc --arg r "$1" '{decision: "block", reason: $r}'
  exit 0
}

# Uncommitted or unpushed work in the current issue's worktree.
unsaved() {
  n="$(sed -n 's/.*#\([0-9][0-9]*\).*/\1/p' "$loop/work" 2>/dev/null | head -n 1)"
  [ -n "$n" ] || return 1
  wt="$loop/worktrees/$n"
  [ -e "$wt/.git" ] || return 1
  if [ -n "$(git -C "$wt" status --porcelain 2>/dev/null)" ]; then
    echo "uncommitted changes in .loop/worktrees/$n"
  elif up="$(git -C "$wt" rev-parse --abbrev-ref '@{upstream}' 2>/dev/null)"; then
    [ "$(git -C "$wt" rev-list --count "$up..HEAD" 2>/dev/null || echo 0)" -gt 0 ] || return 1
    echo "commits in .loop/worktrees/$n not pushed to $up"
  elif [ "$(git -C "$wt" rev-list --count origin/main..HEAD 2>/dev/null || echo 0)" -gt 0 ]; then
    echo "commits in .loop/worktrees/$n on a branch never pushed"
  else
    return 1
  fi
}

if [ -e "$loop/cycle-done" ] || ! [ -f "$loop/state.json" ] ||
  ! open="$(python3 -I "$root/scripts/loop/state.py" cycle-open)"; then
  # No open cycle: only unsaved work stops the turn, once.
  rm -f "$loop/stop-blocks"
  [ "$active" = true ] && exit 0
  why="$(unsaved)" || exit 0
  block "The loop's current issue has $why. Commit and push it to the issue's branch (or remove the worktree if the work is abandoned) before ending the turn."
fi

count="$(cat "$loop/stop-blocks" 2>/dev/null || echo 0)"
[ "$active" = true ] || count=0
if [ "$count" -ge "$LOOP_STOP_BLOCKS" ]; then
  rm -f "$loop/stop-blocks"
  exit 0
fi
echo $((count + 1)) >"$loop/stop-blocks"
extra=""
why="$(unsaved)" && extra=" The issue's worktree has $why: commit and push it first."
block "The loop $open and has no recorded end state. Do not stop mid-cycle or wait for a human. Finish or park the issue, then end the cycle in a state: python3 -I scripts/loop/state.py end-cycle <released|idle|parked|failed|stopped> (stopped when .loop/STOP exists).$extra"
