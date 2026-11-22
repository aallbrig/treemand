#!/usr/bin/env bash
# Tests for the measurement engine: budgets, ratchets, recording, the gate.
# The `check && ok || bad` idiom is safe here: ok only echoes, so it never fails
# and bad never runs after a passing check (SC2015). Exit codes are read from $?
# right after the command under test, by design (SC2181).
# shellcheck disable=SC2015,SC2181
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
export MEASURE_PROBES="$work/probes" MEASURE_RULES="$work/rules.json" MEASURE_HISTORY="$work/history"
mkdir -p "$MEASURE_PROBES"
fail=0
ok() { echo "  ok   $1"; }
bad() { echo "  FAIL $1"; fail=1; }
probe() { # probe <cold> <score>
  printf 'echo '"'"'{"probe":"demo","metrics":{"cold_s.big":%s,"cold_s.small":0.1,"score":%s,"calls":%s}}'"'"'\n' "$1" "$2" "${CALLS:-3}" >"$MEASURE_PROBES/demo.sh"
}
cat >"$MEASURE_RULES" <<'J'
{"demo": {"cold_s.*": {"budget": {"max": 5}, "ratchet": "lower", "tolerance": 0.1, "min_delta": 0.5, "host_bound": true, "timing": true},
          "calls":    {"ratchet": "lower", "host_bound": true},
          "score":    {"budget": {"min": 90}, "ratchet": "higher"}}}
J
m() { python3 -I scripts/metrics/measure.py "$@"; }
export MEASURE_LOAD="${MEASURE_LOAD:-0.1}"

probe 4 95
out="$(m demo --record)"; [[ "$out" == *"no findings"* ]] && ok "first run: within budgets, nothing to compare" || bad "first run: $out"
[ "$(wc -l <"$MEASURE_HISTORY/demo.jsonl")" -eq 1 ] && ok "--record appends history" || bad "--record appends history"

probe 6 95
out="$(m demo)"; [[ "$out" == *"BUDGET     cold_s.big = 6"* ]] && ok "budget breach reported" || bad "budget breach: $out"
[[ "$out" == *"REGRESSION cold_s.big"* ]] && ok "ratchet regression vs last record" || bad "ratchet regression: $out"
m demo --gate >/dev/null; [ $? = 3 ] && ok "--gate fails on a regression" || bad "--gate fails on a regression"

probe 4.2 95
out="$(m demo)"; [[ "$out" == *"no findings"* ]] && ok "small drift within tolerance/min_delta is not a regression" || bad "tolerance: $out"
m demo --gate >/dev/null; [ $? = 0 ] && ok "--gate passes without regressions" || bad "--gate passes"

probe 2 80
out="$(m demo)"; [[ "$out" == *"IMPROVED   cold_s.big"* ]] && ok "improvement reported" || bad "improvement: $out"
[[ "$out" == *"BUDGET     score = 80"* && "$out" == *"REGRESSION score"* ]] && ok "higher-is-better ratchet and min budget" || bad "higher ratchet: $out"
m demo --findings | python3 -c 'import json,sys; d=json.load(sys.stdin); assert {f["kind"] for f in d["findings"]} >= {"budget","regression","improved"}' \
  && ok "--findings emits JSON verdicts" || bad "--findings JSON"

python3 - "$MEASURE_HISTORY/demo.jsonl" <<'PY'
import json,sys
p=sys.argv[1]; r=json.loads(open(p).read().splitlines()[0]); r["host"]="other-host"
open(p,"w").write(json.dumps(r)+"\n")
PY
probe 9 95
out="$(m demo)"; [[ "$out" != *"REGRESSION cold_s.big"* ]] && ok "host-bound metrics only compare against the same host" || bad "host_bound: $out"

# Load awareness: on a busy machine host-bound metrics are not judged, and a
# record taken while busy is never a ratchet baseline.
rm -f "$MEASURE_HISTORY/demo.jsonl"
probe 4 95
MEASURE_LOAD=0.3 m demo --record >/dev/null 2>&1
probe 9 95
out="$(MEASURE_LOAD=2.5 m demo)"
if [[ "$out" == *"UNRELIABLE"* && "$out" != *"cold_s.big = 9"* ]]; then ok "busy machine: host-bound metrics reported unreliable, not judged"; else bad "busy: $out"; fi
probe 2 95
MEASURE_LOAD=2.5 m demo --record >/dev/null 2>&1
probe 4.1 95
out="$(MEASURE_LOAD=0.3 m demo)"
if [[ "$out" != *"REGRESSION cold_s.big"* ]]; then ok "a busy record is not a ratchet baseline"; else bad "busy baseline: $out"; fi
out="$(CALLS=5 probe 4 95; MEASURE_LOAD=2.5 m demo)"
if [[ "$out" == *"REGRESSION calls"* ]]; then ok "counts are judged on a busy machine"; else bad "busy counts: $out"; fi

# MEASURE_HOST names the host: CI runners get a new hostname every run.
rm -f "$MEASURE_HISTORY/demo.jsonl"
probe 4 95
MEASURE_HOST=ci-runner m demo --record >/dev/null 2>&1
probe 9 95
out="$(MEASURE_HOST=ci-runner m demo)"
if [[ "$out" == *"REGRESSION cold_s.big"* ]]; then ok "MEASURE_HOST: history from the named host is the baseline"; else bad "MEASURE_HOST: $out"; fi
exit "$fail"
