#!/usr/bin/env bash
# First phase of every cycle. Prints exactly one verdict line:
#   STOP:<why>      stop file present; end the loop quietly
#   HALT:<why>      breaker open or daily cap hit; sleep, do no work
#   FIX_MAIN:<why>  main is red or the latest release is unverified; fixing it is this cycle
#   GO              proceed to Select
set -uo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$LOOP_ROOT" || exit 1
state() { python3 -I "$LOOP_ROOT/scripts/loop/state.py" "$@"; }

if [ -e "$LOOP_STOP_FILE" ]; then echo "STOP:.loop/STOP present"; exit 0; fi
if ! why="$(state breaker)"; then echo "HALT:$why"; exit 0; fi
if ! why="$(state budget)"; then echo "HALT:$why"; exit 0; fi

git fetch origin main --tags --quiet || { echo "HALT:cannot reach origin"; exit 0; }
# Keep the checkout the skills are read from in step with main (clean, on main only).
if [ "$(git branch --show-current)" = main ] && git diff --quiet && git diff --cached --quiet; then
  git merge --ff-only origin/main --quiet 2>/dev/null || true
fi
conclusion="$(gh run list --branch main --workflow ci.yml --limit 1 --json conclusion --jq '.[0].conclusion // "pending"' 2>/dev/null || echo unknown)"
case "$conclusion" in
  failure | cancelled | timed_out) echo "FIX_MAIN:CI on main is $conclusion"; exit 0 ;;
esac
latest="$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n1)"
rel="$(gh run list --workflow release.yml --branch "$latest" --limit 1 --json conclusion --jq '.[0].conclusion // "none"' 2>/dev/null || echo unknown)"
case "$rel" in
  failure | cancelled | timed_out) echo "FIX_MAIN:release $latest workflow is $rel"; exit 0 ;;
esac
echo GO
