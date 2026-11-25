#!/bin/sh
# Tests for `task measure:gate`, the one list of probes that both `task ci` and
# the CI measure job gate on (#468).
# shellcheck disable=SC2015
set -u
cd "$(dirname "$0")/../.." || exit 1
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail=0
ok() { echo "  ok   $1"; }
bad() { echo "  FAIL $1"; fail=1; }

# A complexity regression fails the task: history claims fewer complex
# functions than the tree has, as a PR that adds one would look to CI.
cp -R docs/metrics "$work/history"
printf '%s\n' '{"date": "2026-01-01T00:00:00Z", "host": "test", "load": 0.1, "metrics": {"cognitive_avg": 0.1, "cognitive_max": 1, "files_over_800_lines": 0, "functions_over_15": 0, "functions_over_30": 0, "go_loc": 1}, "sha": "0000000", "version": "test"}' >>"$work/history/complexity.jsonl"
MEASURE_HISTORY="$work/history" task measure:gate >"$work/out" 2>&1
[ $? != 0 ] && ok "measure:gate fails on a complexity regression" || bad "measure:gate passed a complexity regression"
grep -q "REGRESSION functions_over_15" "$work/out" && ok "the failure names the metric" || bad "no REGRESSION line: $(cat "$work/out")"

# Depth metrics (#475): a long file that grows, a new clone or a new deeply
# nested if fails the gate, as a PR that splits nothing but adds to them would.
gate_on() { # $1: metric, $2: a value below the tree's, $3: a tag for the work files
	cp -R docs/metrics "$work/h-$3"
	printf '{"date": "2026-01-01T00:00:00Z", "host": "test", "load": 0.1, "metrics": {"%s": %s}, "sha": "0000000", "version": "test"}\n' "$1" "$2" >>"$work/h-$3/complexity.jsonl"
	MEASURE_HISTORY="$work/h-$3" task measure:gate >"$work/out-$3" 2>&1
}
gate_on "file_lines.cli/treemand/discovery/help.go" 801 lines
[ $? != 0 ] && grep -q "REGRESSION file_lines.cli/treemand/discovery/help.go" "$work/out-lines" && ok "measure:gate fails when a file over 800 lines grows" || bad "a grown long file passed: $(cat "$work/out-lines")"
gate_on dupl_clones -1 dupl
[ $? != 0 ] && grep -q "REGRESSION dupl_clones" "$work/out-dupl" && ok "measure:gate fails on a new clone" || bad "a new clone passed: $(cat "$work/out-dupl")"
gate_on nestif_over_3 -1 nest
[ $? != 0 ] && grep -q "REGRESSION nestif_over_3" "$work/out-nest" && ok "measure:gate fails on a new deeply nested if" || bad "a new nested if passed: $(cat "$work/out-nest")"

# One probe list: the CI job runs the task, never a probe of its own.
job="$(sed -n '/^  measure:$/,/^  [a-z-]*:$/p' .github/workflows/ci.yml)"
case "$job" in *"run: task measure:gate"*) ok "the CI measure job runs task measure:gate" ;; *) bad "the CI measure job does not run task measure:gate" ;; esac
case "$job" in *"measure.py"*) bad "the CI measure job calls measure.py directly" ;; *) ok "the CI measure job has no probe list of its own" ;; esac
grep -q 'measure\.py [a-z-]* --gate' .github/workflows/ci.yml && bad "ci.yml gates a probe outside the task" || ok "ci.yml gates no probe outside the task"

ci="$(sed -n '/^  ci:$/,/^  [a-z:-]*:$/p' Taskfile.yml)"
case "$ci" in *"task: measure:gate"*) ok "task ci runs measure:gate" ;; *) bad "task ci does not run measure:gate" ;; esac

[ "$fail" = 0 ] && echo "measure_gate_test: ok"
exit "$fail"
