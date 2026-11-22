#!/bin/sh
# Start the Release workflow on an existing tag and wait for it to succeed.
#
#   sh scripts/release/dispatch.sh <vX.Y.Z>
#
# GitHub sometimes fails a run before its build job exists (no logs, no jobs).
# That is infrastructure, not the code, so such a run is retried twice.
# Needs gh with actions:write (REPO=owner/name, default: the current repository).
set -eu
tag="${1:?usage: dispatch.sh <vX.Y.Z>}"
repo="${REPO:-$(gh repo view --json nameWithOwner --jq .nameWithOwner)}"

gh workflow run release.yml --repo "$repo" --ref "$tag"

run_id=""
i=0
while [ "$i" -lt 30 ]; do
  run_id="$(gh run list --repo "$repo" --workflow release.yml --branch "$tag" --limit 1 \
    --json databaseId --jq '.[0].databaseId // empty')"
  [ -n "$run_id" ] && break
  i=$((i + 1))
  sleep 5
done
[ -n "$run_id" ] || { echo "dispatch: the Release run for $tag never appeared" >&2; exit 4; }
echo "dispatch: Release run $run_id for $tag"

attempt=0
until gh run watch "$run_id" --repo "$repo" --exit-status >/dev/null; do
  built="$(gh run view "$run_id" --repo "$repo" --json jobs \
    --jq '[.jobs[].name] | index("Build & Release")')"
  attempt=$((attempt + 1))
  if [ "$built" != null ] || [ "$attempt" -gt 2 ]; then
    echo "dispatch: the Release workflow failed for $tag (attempt $attempt)" >&2
    exit 5
  fi
  echo "dispatch: the run never started its build job; rerunning (attempt $attempt)"
  gh run rerun "$run_id" --repo "$repo"
  sleep 20
done
echo "dispatch: Release succeeded for $tag"
