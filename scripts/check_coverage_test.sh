#!/bin/sh
# Tests for check_coverage.sh, judging canned `go test -cover` output against
# canned floors (no tests run).
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fails=0
printf '# comment\nexample/a\t80\nexample/b\t50\n' >"$work/floors.tsv"

# judge <want pass|fail> <name> <expected output fragment> <go test output>
judge() {
  printf '%s\n' "$4" >"$work/out.txt"
  if COVERAGE_FLOORS="$work/floors.tsv" COVERAGE_OUTPUT="$work/out.txt" sh "$here/check_coverage.sh" >"$work/log" 2>&1; then got=pass; else got=fail; fi
  if [ "$got" = "$1" ] && grep -qF -- "$3" "$work/log"; then echo "  ok   $2"; else
    echo "  FAIL $2: want $1 with '$3', got $got:"
    sed 's/^/       /' "$work/log"
    fails=$((fails + 1))
  fi
}

a='ok  	example/a	0.1s	coverage: 85.0% of statements'
b='ok  	example/b	0.1s	coverage: 50.0% of statements'
judge pass "every package at or above its floor" "floors held (2 packages)" "$a
$b"
judge fail "a package below its floor" "coverage below floor: example/a 79.9% < 80%" "ok  	example/a	0.1s	coverage: 79.9% of statements
$b"
judge fail "a tested package with no floor" "no coverage floor for example/c" "$a
$b
ok  	example/c	0.1s	coverage: 10.0% of statements"
judge pass "packages without statements are ignored" "floors held (2 packages)" "$a
$b
ok  	example/d	0.1s	coverage: [no statements]"

[ "$fails" = 0 ] || { echo "$fails failed"; exit 1; }
echo "check_coverage: all passed"
