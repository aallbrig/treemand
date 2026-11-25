#!/bin/sh
# Tests for test_race.sh with a fake go on PATH (no real tests run): it runs
# go test once with the race detector and a coverage profile, prints the
# output, keeps it for check_coverage.sh only on success, and passes on the
# exit status.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/out"
fails=0
cat >"$work/bin/go" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"$FAKE_GO_LOG"
printf 'ok  \texample/a\t0.1s\tcoverage: 85.0%% of statements\n'
exit "${FAKE_GO_RC:-0}"
EOF
chmod +x "$work/bin/go"

# run <fake go exit status>: runs test_race.sh, leaving its output in $work/log
run() {
  rm -f "$work/go.log"
  set +e
  PATH="$work/bin:$PATH" FAKE_GO_LOG="$work/go.log" FAKE_GO_RC="$1" TEST_RACE_DIR="$work/out" \
    sh "$here/test_race.sh" >"$work/log" 2>&1
  # shellcheck disable=SC2034 # rc is read by the check conditions (eval)
  rc=$?
  set -e
}

# check <name> <shell condition>
check() {
  if eval "$2"; then echo "  ok   $1"; else
    echo "  FAIL $1"
    sed 's/^/       /' "$work/log"
    fails=$((fails + 1))
  fi
}

run 0
check "passing tests exit 0" '[ "$rc" = 0 ]'
check "go test runs once, with race and a coverage profile" \
  '[ "$(grep -c . "$work/go.log")" = 1 ] && grep -q -- "^test -race -covermode=atomic -coverprofile=$work/out/coverage.out ./cli/treemand/...$" "$work/go.log"'
check "the output is printed" 'grep -q "coverage: 85.0%" "$work/log"'
check "the output is kept for the floor check" 'grep -q "coverage: 85.0%" "$work/out/coverage.txt" && [ ! -e "$work/out/coverage.txt.tmp" ]'
check "check_coverage.sh accepts the kept output" \
  'printf "example/a\t80\n" >"$work/floors.tsv" && COVERAGE_FLOORS="$work/floors.tsv" COVERAGE_OUTPUT="$work/out/coverage.txt" sh "$here/../check_coverage.sh" >>"$work/log" 2>&1'

run 1
check "failing tests pass on the exit status" '[ "$rc" = 1 ]'
check "a failed run leaves no output for the floor check" '[ ! -e "$work/out/coverage.txt" ] && [ ! -e "$work/out/coverage.txt.tmp" ]'

[ "$fails" = 0 ] || { echo "$fails failed"; exit 1; }
echo "test_race: all passed"
