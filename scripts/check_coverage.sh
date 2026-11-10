#!/usr/bin/env bash
# Per-package coverage floor. Fails if any package is below its floor in
# scripts/coverage_floor.tsv, or a tested package has no floor row.
#   bash scripts/check_coverage.sh            check
#   bash scripts/check_coverage.sh --update   deliberately rewrite floors to today's coverage
#                                             (rounded down); review and commit the diff
# Floors are a ratchet: raise them as coverage improves, never lower them to get green.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

floors=scripts/coverage_floor.tsv
out="$(cd cli/treemand && go test -count=1 -cover ./... 2>&1)" || {
  echo "$out" >&2
  echo "tests failed; coverage not checked" >&2
  exit 1
}

# "ok  <pkg>  <time>  coverage: 88.2% of statements" -> "<pkg> 88.2"
current="$(printf '%s\n' "$out" | awk '/^ok/ && /coverage:/ && /statements/ {
  for (i = 1; i <= NF; i++) if ($i == "coverage:") { pct = $(i + 1); sub(/%/, "", pct) }
  print $2, pct }')"

if [ "${1:-}" = "--update" ]; then
  {
    echo "# package<TAB>minimum statement coverage (%). Ratchet: raise, never lower. Rewrite with: task coverage:update-floor"
    printf '%s\n' "$current" | awk '{ printf "%s\t%d\n", $1, int($2) }' | sort
  } >"$floors"
  echo "floors rewritten; review the diff"
  exit 0
fi

status=0
declare -A floor
while IFS=$'\t' read -r pkg min; do
  case "$pkg" in '' | \#*) continue ;; esac
  floor["$pkg"]="$min"
done <"$floors"

while read -r pkg pct; do
  min="${floor[$pkg]:-}"
  if [ -z "$min" ]; then
    echo "no coverage floor for $pkg (now ${pct}%); add one with: task coverage:update-floor" >&2
    status=1
  elif awk -v p="$pct" -v m="$min" 'BEGIN { exit !(p + 0 < m + 0) }'; then
    echo "coverage below floor: $pkg ${pct}% < ${min}%" >&2
    status=1
  fi
done <<<"$current"

[ "$status" = 0 ] && echo "coverage floors held ($(printf '%s\n' "$current" | wc -l) packages)"
exit "$status"
