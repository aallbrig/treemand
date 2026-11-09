#!/usr/bin/env bash
# Revert the most recent merge on main through a PR and merge it.  The caller
# then runs release.sh patch to ship the reverted state.   usage: rollback.sh
set -euo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$LOOP_ROOT"
git fetch origin main --quiet
merge="$(git rev-list --merges -n1 origin/main)"
[ -n "$merge" ] || { echo "no merge commit to revert" >&2; exit 1; }
wt="$LOOP_DIR/worktrees/revert-${merge:0:8}"
rm -rf "$wt"
git worktree add -q -B "loop/revert-${merge:0:8}" "$wt" origin/main
(
  cd "$wt"
  git revert -m 1 --no-edit "$merge"
  git push -u origin HEAD
  url="$(gh pr create --fill --title "Revert ${merge:0:8}: failed production verification" \
    --body "Automatic revert by the dev loop after a post-release smoke test failed.")"
  bash "$LOOP_ROOT/scripts/pr_merge.sh" "$url"
)
git worktree remove --force "$wt"
