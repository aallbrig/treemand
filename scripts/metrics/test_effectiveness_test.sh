#!/bin/sh
# Behaviour tests for the test-effectiveness probe (scripts/metrics/
# test_effectiveness.py) on a fixture Go module with real go and a fake
# gremlins that lists the fixture's mutants. One mutant of each fate (killed,
# lived, not viable, not covered), a test that fails on its second run (flaky),
# tests that always fail (one in the mutated package), then the same module without gremlins.
here="$(cd "$(dirname "$0")" && pwd)"
probe="$here/test_effectiveness.py"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0

# get <json-file> <python expression over o, d its details> -> prints its value (or "absent")
get() { python3 -I "$here/json_get.py" "$1" "$2"; }
expect() { # expect <json-file> <expression> <value> <what>
  got="$(get "$1" "$2")"
  if [ "$got" = "$3" ]; then echo "ok    $4"; else echo "FAIL  $4: $2 = $got, want $3"; fail=1; fi
}

# ── the fixture module ────────────────────────────────────────────────────
mod="$tmp/mod"
mkdir -p "$mod/calc" "$mod/broken" "$mod/util"
printf 'module example.com/fixture\n\ngo 1.22\n' >"$mod/go.mod"
cat >"$mod/calc/calc.go" <<'GO'
package calc

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Double(x int) int { return x * 2 }

func Greet(n string) string { return "hi " + n }

func One(a int) bool { return a == 1 }
GO
cat >"$mod/calc/calc_test.go" <<'GO'
package calc

import (
	"os"
	"testing"
)

// Fails once when $FLAKE_ONCE names an existing file (and removes it): a
// timing test failing under load during one mutant's run.
func TestOnce(t *testing.T) {
	if p := os.Getenv("FLAKE_ONCE"); p != "" {
		if _, err := os.Stat(p); err == nil {
			_ = os.Remove(p)
			t.Fatal("failed once")
		}
	}
}

// Max(2, 2) is never asked, so a > b -> a >= b survives.
func TestMax(t *testing.T) {
	if Max(3, 1) != 3 || Max(1, 3) != 3 {
		t.Fatal("max")
	}
}

func TestDouble(t *testing.T) {
	if Double(4) != 8 {
		t.Fatal("double")
	}
}

func TestGreet(t *testing.T) {
	if Greet("x") != "hi x" {
		t.Fatal("greet")
	}
}

var runs int

// Fails every run, as a timing test does on a loaded host: it must not
// block the package's mutation run.
func TestAlwaysFailsHere(t *testing.T) { t.Fatal("always") }

// Fails on its second run in one process: flaky under -count=3.
func TestFlaky(t *testing.T) {
	runs++
	if runs == 2 {
		t.Fatal("second run")
	}
}
GO
cat >"$mod/util/util.go" <<'GO'
package util

func Neg(x int) int { return -x }
GO
cat >"$mod/util/util_test.go" <<'GO'
package util

import "testing"

func TestNeg(t *testing.T) {
	if Neg(2) != -2 {
		t.Fatal("neg")
	}
}
GO
cat >"$mod/broken/broken_test.go" <<'GO'
package broken

import "testing"

func TestAlwaysFails(t *testing.T) { t.Fatal("always") }
GO

# ── fake gremlins: the dry run lists the fixture's mutants ────────────────
bin="$tmp/bin"
mkdir -p "$bin"
cat >"$bin/gremlins" <<'SH'
#!/bin/sh
# Like gremlins, gather coverage first: any failing test fails the dry run.
go test -count=1 ./calc >/dev/null 2>&1 || { echo "ERROR: failed to gather coverage" >&2; exit 1; }
# The flake fires on the first test run after the dry run.
[ -n "${FLAKE_ONCE:-}" ] && touch "$FLAKE_ONCE"
out=""
while [ $# -gt 0 ]; do [ "$1" = "-o" ] && out="$2"; shift; done
cat >"$out" <<'J'
{"go_module": "example.com/fixture", "files": [{"file_name": "calc.go", "mutations": [
  {"type": "CONDITIONALS_BOUNDARY", "status": "RUNNABLE", "line": 4, "column": 7},
  {"type": "CONDITIONALS_NEGATION", "status": "RUNNABLE", "line": 4, "column": 7},
  {"type": "ARITHMETIC_BASE", "status": "RUNNABLE", "line": 10, "column": 35},
  {"type": "ARITHMETIC_BASE", "status": "RUNNABLE", "line": 12, "column": 44},
  {"type": "CONDITIONALS_NEGATION", "status": "NOT COVERED", "line": 14, "column": 33}]}]}
J
SH
chmod +x "$bin/gremlins"

export TEST_EFFECTIVENESS_PACKAGES=calc TEST_EFFECTIVENESS_FLAKE_PACKAGES="calc broken" TEST_EFFECTIVENESS_WORKERS=2

out="$tmp/out.json"
if ! GREMLINS="$bin/gremlins" python3 -I "$probe" --module "$mod" >"$out"; then
  echo "FAIL  the probe exits 0 even when tests fail"
  fail=1
fi
expect "$out" 'o["metrics"]["mutation_score.calc"]' 66.7 "mutation score: killed and lived counted, not-viable left out"
expect "$out" 'o["metrics"]["mutants_tested.calc"]' 3 "mutants tested"
expect "$out" 'o["metrics"]["mutants_lived.calc"]' 1 "one mutant lived"
expect "$out" 'o["metrics"]["mutants_not_covered.calc"]' 1 "uncovered mutants counted"
expect "$out" 'next(x for x in d if x["kind"] == "mutation")["not_viable"]' 1 "a mutant that does not compile is not viable"
expect "$out" 'sorted(t for t, _ in next(x for x in d if x["kind"] == "mutation")["top_killers"])' "['TestDouble', 'TestMax']" "the tests that killed mutants are named"
expect "$out" 'next(x for x in d if x["kind"] == "survived")["at"]' calc/calc.go:4:7 "the survivor's position"
expect "$out" 'next(x for x in d if x["kind"] == "survived")["mutator"]' CONDITIONALS_BOUNDARY "the survivor's mutator"
expect "$out" 'next(x for x in d if x["kind"] == "survived")["mutated"]' 'if a >= b {' "the survivor's mutated line"
expect "$out" 'o["metrics"]["flaky_tests"]' 1 "a test that fails on one of three runs is flaky"
expect "$out" 'next(x for x in d if x["kind"] == "flaky")["test"]' TestFlaky "the flaky test is named"
expect "$out" 'o["metrics"]["failing_tests"]' 2 "a test failing every run is failing, not flaky"
expect "$out" 'o["metrics"]["coverage_pct.util"]' 100.0 "coverage per package"
expect "$out" '"coverage_pct.calc" in o["metrics"]' False "a package with a failing test has no coverage"

# ── a kill that does not repeat is judged again without the flaky test ──
out="$tmp/once.json"
FLAKE_ONCE="$tmp/flake-once" TEST_EFFECTIVENESS_WORKERS=1 GREMLINS="$bin/gremlins" python3 -I "$probe" --module "$mod" >"$out"
expect "$out" 'next(x for x in d if x["kind"] == "mutation")["unconfirmed_kills"]' 1 "a kill the killing test does not repeat is unconfirmed"
expect "$out" 'o["metrics"]["mutation_score.calc"]' 66.7 "an unconfirmed kill does not raise the score"
expect "$out" 'next(x for x in d if x["kind"] == "survived")["at"]' calc/calc.go:4:7 "the mutant the flake hit is still found to survive"

# ── without gremlins: mutation is skipped with the install command ────────
out="$tmp/none.json"
GREMLINS="$tmp/missing" python3 -I "$probe" --module "$mod" >"$out" ||
  { echo "FAIL  the probe exits 0 without gremlins"; fail=1; }
expect "$out" '"go install" in o["skipped"]["calc"]' True "no gremlins: the package is skipped with how to install it"
expect "$out" '"mutation_score.calc" in o["metrics"]' False "no gremlins: no mutation score"

exit "$fail"
