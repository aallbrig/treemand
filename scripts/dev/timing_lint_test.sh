#!/bin/sh
# Tests the wallClockBound ruleguard rule (#477, cli/treemand/lintrules): the
# tree as it is passes, each planted wall-clock assert in a unit test fails
# lint, and the cases the rule leaves alone pass. Runs on a copy of the module
# so the working tree is never touched.
set -u
cd "$(dirname "$0")/../.." || exit 1
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail=0
ok() { echo "  ok   $1"; }
bad() { echo "  FAIL $1"; fail=1; }

mkdir -p "$work/cli"
cp go.work .golangci.yml "$work/"
cp -R cli/treemand "$work/cli/treemand"

lint() { (cd "$work" && golangci-lint run --enable-only gocritic "./cli/treemand/$1/...") >"$work/out" 2>&1; }

if lint .; then ok "the tree as it is passes"; else bad "the tree as it is fails:"; cat "$work/out"; fi

# plant <package dir> <file> <package clause> <statements> <want: fail|pass> <case name>
plant() {
	f="$work/cli/treemand/$1/$2"
	printf 'package %s\n\nimport "time"\n\nfunc zzTimingPlant(t0 time.Time) bool {\n%s\n}\n' "$3" "$4" >"$f"
	if lint "$1"; then got=pass; else got=fail; fi
	if [ "$got" = "$5" ] && { [ "$5" = pass ] || grep -q 'ruleguard: unit test compares a duration' "$work/out"; }; then
		ok "$6"
	else
		bad "$6: lint gave $got, want $5:"; cat "$work/out"
	fi
	rm -f "$f"
}

plant fuzzy zz_plant_test.go fuzzy_test '	return time.Since(t0) > 2*time.Second' fail "time.Since against a literal bound"
plant fuzzy zz_plant_test.go fuzzy_test '	elapsed := time.Since(t0)
	return 500*time.Millisecond <= elapsed' fail "an elapsed variable on either side"
plant fuzzy zz_plant_test.go fuzzy_test '	//nolint:gocritic // timing: only a kill ends the input
	return time.Since(t0) > 2*time.Second' pass "an opt-out with a reason"
plant fuzzy zz_plant_test.go fuzzy_test '	return time.Since(t0) >= 0' pass "a sign check against 0"
plant fuzzy zz_plant.go fuzzy '	return time.Since(t0) > 2*time.Second' pass "production code may bound time"
plant e2e zz_plant_test.go e2e '	return time.Since(t0) > 2*time.Second' pass "e2e tests are not unit tests"

exit "$fail"
