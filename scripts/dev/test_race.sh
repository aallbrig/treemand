#!/bin/sh
# The one run of the Go tests: race detector on, coverage recorded. It writes
#   cli/treemand/coverage.out   the profile codecov uploads
#   cli/treemand/coverage.txt   the per-package output check_coverage.sh judges
#                               (COVERAGE_OUTPUT), so the floor check does not
#                               run the tests again (#478)
# coverage.txt is written only when the tests pass, so a failed run never
# leaves output behind for the floor check to judge.
# TEST_RACE_DIR overrides the output directory (scripts/dev/test_race_test.sh).
set -u
cd "$(dirname "$0")/../.." || exit 1
dir="${TEST_RACE_DIR:-cli/treemand}"
rm -f "$dir/coverage.txt"
go test -race -covermode=atomic -coverprofile="$dir/coverage.out" ./cli/treemand/... >"$dir/coverage.txt.tmp" 2>&1
rc=$?
cat "$dir/coverage.txt.tmp"
if [ "$rc" = 0 ]; then
  mv "$dir/coverage.txt.tmp" "$dir/coverage.txt"
else
  rm -f "$dir/coverage.txt.tmp"
fi
exit "$rc"
