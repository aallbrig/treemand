#!/bin/sh
# Wait for the Auto Release run that a merge to main started, and report it.
#
#   sh scripts/release/wait.sh <merge-commit-sha>
#
# Prints the released tag and exits 0 when the run tagged, released and
# smoke-tested the commit; prints "none" when the run decided not to release.
# Exits non-zero when the run failed: the loop treats that as FIX_MAIN.
set -eu
# shellcheck source=scripts/release/lib.sh
. "$(dirname "$0")/lib.sh"
sha="${1:?usage: wait.sh <merge-commit-sha>}"
repo="${REPO:-$(gh repo view --json nameWithOwner --jq .nameWithOwner)}"

run_id=""
i=0
while [ "$i" -lt 60 ]; do
  run_id="$(gh run list --repo "$repo" --workflow auto-release.yml --commit "$sha" --limit 1 \
    --json databaseId --jq '.[0].databaseId // empty')"
  [ -n "$run_id" ] && break
  i=$((i + 1))
  sleep 5
done
[ -n "$run_id" ] || { echo "wait: no Auto Release run for $sha" >&2; exit 4; }

conclusion="$(run_conclusion "$run_id" "$repo" || true)"
if [ "$conclusion" != success ]; then
  # A newer merge's run supersedes a cancelled one and releases this commit too.
  if [ "$conclusion" = cancelled ]; then
    echo "wait: run $run_id was superseded by a newer merge; watch that one" >&2
    exit 6
  fi
  echo "wait: Auto Release run $run_id failed ($conclusion)" >&2
  exit 5
fi
git fetch --quiet --tags origin
tag="$(tag_for "$sha")"
echo "${tag:-none}"
