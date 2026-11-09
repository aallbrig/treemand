#!/usr/bin/env bash
# Pass the PR's CI gate, then merge with a merge commit: scripts/pr_merge.sh N
set -euo pipefail
n="${1:?usage: pr_merge.sh <pr-number>}"

# Checks may not be registered the instant a PR opens; wait for them to appear.
for _ in $(seq 1 30); do
  if [ "$(gh pr checks "$n" --json name --jq 'length' 2>/dev/null || echo 0)" -gt 0 ]; then
    break
  fi
  sleep 2
done

gh pr checks "$n" --watch --fail-fast
gh pr merge "$n" --merge
