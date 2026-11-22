#!/bin/sh
# Shared helpers for the release scripts. Source it: . scripts/release/lib.sh

# run_conclusion <run-id> [repo]: wait until a workflow run completes and print
# its conclusion (success, failure, cancelled...). It polls instead of using
# `gh run watch --exit-status`, which exits non-zero on a transient API error
# (an HTTP 500 while listing jobs) although the run itself is fine.
run_conclusion() {
  _id="$1"
  _repo="${2:-${REPO:-}}"
  _errors=0
  while :; do
    if _s="$(gh run view "$_id" ${_repo:+--repo "$_repo"} --json status,conclusion \
      --jq '"\(.status) \(.conclusion)"' 2>/dev/null)"; then
      _errors=0
      case "$_s" in
        "completed "*) echo "${_s#completed }"; return 0 ;;
      esac
    else
      _errors=$((_errors + 1))
      if [ "$_errors" -ge 20 ]; then
        echo "unknown"
        return 1
      fi
    fi
    sleep "${RELEASE_POLL_SECONDS:-15}"
  done
}

# tag_for <merge-sha>: the v* tag Auto Release cut for that merge, or nothing.
# The tag is either the merge itself (releases before changelog entries) or
# the changelog-entry commit tag.sh adds on top of it.
tag_for() {
  for _t in $(git tag --contains "$1" --list 'v[0-9]*' --sort=v:refname); do
    _c="$(git rev-parse "$_t^{commit}")"
    if [ "$_c" = "$1" ] || [ "$(git rev-parse "$_c^" 2>/dev/null)" = "$1" ]; then
      echo "$_t"
      return 0
    fi
  done
}
