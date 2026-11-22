#!/bin/sh
# downloads.sh [--once]: release asset download counts from the GitHub API, the
# total per release for the latest 5 releases. See scripts/prod/lib.sh for the modes.
# shellcheck source=scripts/prod/lib.sh
. "$(dirname "$0")/lib.sh"

report() {
  if ! r="$(gh api "repos/$PROD_REPO/releases?per_page=5" \
      --jq '.[] | "\(.tag_name) \([.assets[].download_count] | add // 0) \(.published_at[0:10])"' \
      2>"$PROD_TMP")"; then
    printf 'error: releases: %s\n' "$(prod_err "$PROD_TMP")"
    return 1
  fi
  printf '%s\n' "$r" | awk 'NF == 3 { printf "%-10s %s %6d\n", $1, $3, $2; total += $2; n++ }
    END { if (n == 0) print "no releases"; else printf "%-10s %s %6d\n", "total", "          ", total }'
}

prod_main "downloads (latest 5 releases)" report "$@"
