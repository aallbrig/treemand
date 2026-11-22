#!/bin/sh
# Token-free work check: is there something for a loop cycle to do? It reuses the
# preflight and the tracker door (issues.sh) and never calls an LLM, so the
# supervisor can run it as often as it likes.
# Exit 0 with one line on stdout when there is work, the first that applies:
#   FIX_MAIN:<why>            main is red or the latest release is unverified
#   in-progress #N            an issue this host claimed and still has a worktree for
#   ready #N <title>          the oldest trusted loop:ready issue
# Exit 1 when there is none (the preflight verdict, for STOP or HALT, goes to stderr).
set -u
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "$0")/lib.sh"
cd "$LOOP_ROOT" || exit 2

verdict="$(bash "$LOOP_ROOT/scripts/loop/preflight.sh")"
case "$verdict" in
  FIX_MAIN:*) echo "$verdict"; exit 0 ;;
  GO) ;;
  *) echo "$verdict" >&2; exit 1 ;;
esac

for n in $(bash "$LOOP_ROOT/scripts/loop/issues.sh" in-progress); do
  if [ -d "$LOOP_DIR/worktrees/$n" ]; then echo "in-progress #$n"; exit 0; fi
done

first="$(bash "$LOOP_ROOT/scripts/loop/issues.sh" ready | head -n 1)"
if [ -n "$first" ]; then
  printf '%s\n' "$first" | awk -F '\t' '{ printf "ready #%s %s\n", $1, $2 }'
  exit 0
fi
exit 1
