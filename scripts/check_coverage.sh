#!/bin/sh
# Per-package coverage floor. Fails if any package is below its floor in
# scripts/coverage_floor.tsv, or a tested package has no floor row.
#   sh scripts/check_coverage.sh            check
#   sh scripts/check_coverage.sh --update   deliberately rewrite floors to today's coverage
#                                           (rounded down); review and commit the diff
# Floors are a ratchet: raise them as coverage improves, never lower them to get green.
# A floor must hold on GitHub CI too: if CI measures lower than your machine, take the CI number.
# COVERAGE_OUTPUT names a file of `go test -cover` output to judge instead of
# running the tests (scripts/check_coverage_test.sh).
set -eu
cd "$(dirname "$0")/.."

floors="${COVERAGE_FLOORS:-scripts/coverage_floor.tsv}"
if [ -n "${COVERAGE_OUTPUT:-}" ]; then
  out="$(cat "$COVERAGE_OUTPUT")"
elif ! out="$(cd cli/treemand && go test -count=1 -cover ./... 2>&1)"; then
  printf '%s\n' "$out" >&2
  echo "tests failed; coverage not checked" >&2
  exit 1
fi

# "ok  <pkg>  <time>  coverage: 88.2% of statements" -> "<pkg> 88.2"
current="$(printf '%s\n' "$out" | awk '/^ok/ && /coverage:/ && /statements/ && !/no statements/ {
  for (i = 1; i <= NF; i++) if ($i == "coverage:") { pct = $(i + 1); sub(/%/, "", pct) }
  print $2, pct }')"

if [ "${1:-}" = "--update" ]; then
  {
    echo "# package<TAB>minimum statement coverage (%). Ratchet: raise, never lower. Floors must hold on GitHub CI as well as locally (cmd is lower on CI: some tests use CLIs installed here, see #38). Rewrite with: task coverage:update-floor"
    printf '%s\n' "$current" | awk '{ printf "%s\t%d\n", $1, int($2) }' | sort
  } >"$floors"
  echo "floors rewritten; review the diff"
  exit 0
fi

# The floors file first (package -> minimum), then today's coverage.
if printf '%s\n' "$current" | awk -F'\t' '
  FNR == NR { if ($1 != "" && $1 !~ /^#/) floor[$1] = $2; next }
  {
    split($0, f, " "); pkg = f[1]; pct = f[2]
    if (!(pkg in floor)) {
      printf "no coverage floor for %s (now %s%%); add one with: task coverage:update-floor\n", pkg, pct > "/dev/stderr"; bad = 1
    } else if (pct + 0 < floor[pkg] + 0) {
      printf "coverage below floor: %s %s%% < %s%%\n", pkg, pct, floor[pkg] > "/dev/stderr"; bad = 1
    }
  }
  END { exit bad }' "$floors" -; then
  echo "coverage floors held ($(printf '%s\n' "$current" | grep -c .) packages)"
else
  exit 1
fi
