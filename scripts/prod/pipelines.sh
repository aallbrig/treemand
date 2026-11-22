#!/bin/sh
# pipelines.sh [--once]: the latest run of each production workflow (CI, Auto
# Release, Release, Site, Packages; CI and Site on main), then open PRs and loop:in-progress issues.
# See scripts/prod/lib.sh for the modes.
# shellcheck source=scripts/prod/lib.sh
. "$(dirname "$0")/lib.sh"

latest_runs() {
  failed=0
  while IFS='|' read -r name file branch; do
    set -- --workflow "$file"
    [ -z "$branch" ] || set -- "$@" --branch "$branch"
    if r="$(gh run list --repo "$PROD_REPO" "$@" --limit 1 \
        --json status,conclusion,headBranch,createdAt \
        --jq '.[0] | select(.) | [(if .status == "completed" then .conclusion else .status end), .headBranch, .createdAt] | join(" ")' \
        2>"$PROD_TMP")"; then
      printf '%-12s %s\n' "$name" "${r:-no runs}"
    else
      printf 'error: %s: %s\n' "$name" "$(prod_err "$PROD_TMP")"
      failed=1
    fi
  done <<EOF
CI|ci.yml|main
Auto Release|auto-release.yml
Release|release.yml
Site|site.yml|main
Packages|release-packages.yml
EOF
  return "$failed"
}

listing() { # listing <heading> <gh subcommand> [args]
  heading="$1"
  shift
  if r="$(gh "$@" --repo "$PROD_REPO" --state open --limit 20 --json number,title \
      --jq '.[] | "#\(.number) \(.title)"' 2>"$PROD_TMP")"; then
    printf '\n%s (%s)\n' "$heading" "$(printf '%s' "$r" | grep -c .)"
    [ -n "$r" ] && printf '%s\n' "$r" | cut -c1-72 | sed 's/^/  /'
    return 0
  fi
  printf 'error: %s: %s\n' "$heading" "$(prod_err "$PROD_TMP")"
  return 1
}

report() {
  bad=0
  latest_runs || bad=1
  listing "Open PRs" pr list || bad=1
  listing "In progress (loop:in-progress)" issue list --label loop:in-progress || bad=1
  return "$bad"
}

prod_main "pipelines $PROD_REPO" report "$@"
