#!/bin/sh
# Crash recovery, run by the supervisor when it starts (after a reboot, a killed
# pane or a fresh loop:start). For each open loop:in-progress issue assigned to
# this gh user:
#   - resume it if its worktree .loop/worktrees/<n> exists and is clean (all work
#     committed); work.sh then reports it as `in-progress #<n>`.
#   - otherwise reset it: unclaim it (back to loop:ready) and remove its worktree.
# At most one issue is resumed; any other is reset, so the loop never holds two.
# Prints one line per issue: `resume #N` or `reset #N <why>`.
set -u
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "$0")/lib.sh"
cd "$LOOP_ROOT" || exit 2

resumed=""
for n in $(bash "$LOOP_ROOT/scripts/loop/issues.sh" in-progress); do
  wt="$LOOP_DIR/worktrees/$n"
  why=""
  if [ ! -d "$wt" ]; then
    why="no worktree on this host"
  elif [ -n "$(git -C "$wt" status --porcelain 2>/dev/null || echo broken)" ]; then
    why="worktree has uncommitted changes"
  elif [ -n "$resumed" ]; then
    why="another issue (#$resumed) is resumed"
  fi
  if [ -z "$why" ]; then
    resumed="$n"
    echo "resume #$n"
    continue
  fi
  bash "$LOOP_ROOT/scripts/loop/issues.sh" unclaim "$n" >/dev/null || { echo "reset #$n failed: could not unclaim" >&2; continue; }
  if [ -d "$wt" ]; then
    git worktree remove --force "$wt" 2>/dev/null || rm -rf "$wt"
    git worktree prune
  fi
  echo "reset #$n $why"
done
