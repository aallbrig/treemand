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

# One probe list: the CI job runs the task, never a probe of its own.
job="$(sed -n '/^  measure:$/,/^  [a-z-]*:$/p' .github/workflows/ci.yml)"
case "$job" in *"run: task measure:gate"*) ok "the CI measure job runs task measure:gate" ;; *) bad "the CI measure job does not run task measure:gate" ;; esac
case "$job" in *"measure.py"*) bad "the CI measure job calls measure.py directly" ;; *) ok "the CI measure job has no probe list of its own" ;; esac
grep -q 'measure\.py [a-z-]* --gate' .github/workflows/ci.yml && bad "ci.yml gates a probe outside the task" || ok "ci.yml gates no probe outside the task"

ci="$(sed -n '/^  ci:$/,/^  [a-z:-]*:$/p' Taskfile.yml)"
case "$ci" in *"task: measure:gate"*) ok "task ci runs measure:gate" ;; *) bad "task ci does not run measure:gate" ;; esac

[ "$fail" = 0 ] && echo "measure_gate_test: ok"
exit "$fail"
