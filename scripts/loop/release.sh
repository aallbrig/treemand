#!/usr/bin/env bash
# Tag origin/main with the next semver, push the tag, wait for the Release and
# Pages workflows, then smoke-test production.   usage: release.sh <patch|minor>
# Refuses major bumps and anything that would reach 1.0.0.
set -euo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
bump="${1:?usage: release.sh <patch|minor>}"
case "$bump" in patch | minor) ;; *) echo "bump must be patch or minor" >&2; exit 2 ;; esac

cd "$LOOP_ROOT"
git fetch origin main --tags --quiet
last="$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n1)"
last="${last:-v0.0.0}"
IFS=. read -r major minor patch <<<"${last#v}"
if [ "$bump" = minor ]; then minor=$((minor + 1)); patch=0; else patch=$((patch + 1)); fi
next="v${major}.${minor}.${patch}"
if [ "$major" -ge 1 ]; then
  echo "refusing: major version >= 1 is a human decision" >&2
  exit 3
fi

sha="$(git rev-parse origin/main)"
echo "releasing $next from $sha (previous $last)"
git tag -a "$next" "$sha" -m "$next"
git push origin "$next"

# Find the Release run triggered by the tag and wait for it.
run_id=""
for _ in $(seq 1 30); do
  run_id="$(gh run list --workflow release.yml --branch "$next" --limit 1 --json databaseId --jq '.[0].databaseId // empty')"
  [ -n "$run_id" ] && break
  sleep 5
done
[ -n "$run_id" ] || { echo "release workflow never started" >&2; exit 4; }
# GitHub sometimes fails a run before the build job exists (no logs, jobs skipped).
# That is infrastructure, not the code: rerun up to twice before calling it a failure.
attempt=0
until timeout "$((LOOP_CYCLE_BUDGET_MINUTES * 60))" gh run watch "$run_id" --exit-status; do
  jobs_seen="$(gh run view "$run_id" --json jobs --jq '[.jobs[].name]|index("Build & Release")' 2>/dev/null || echo null)"
  attempt=$((attempt + 1))
  if [ "$jobs_seen" != "null" ] || [ "$attempt" -gt 2 ]; then
    echo "release workflow failed (attempt $attempt)" >&2
    exit 5
  fi
  echo "release run never started its build job; rerunning (attempt $attempt)"
  gh run rerun "$run_id"
  sleep 20
done

# Pages deploys from main pushes; wait for the latest Pages run on this sha, if any.
bash "$LOOP_ROOT/scripts/loop/smoke.sh" "$next"
echo "$next"
