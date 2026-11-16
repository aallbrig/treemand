#!/usr/bin/env bash
# Tests for the measurement engine: budgets, ratchets, recording, the gate.
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
  printf 'echo '"'"'{"probe":"demo","metrics":{"cold_s.big":%s,"cold_s.small":0.1,"score":%s}}'"'"'\n' "$1" "$2" >"$MEASURE_PROBES/demo.sh"
}
cat >"$MEASURE_RULES" <<'J'
{"demo": {"cold_s.*": {"budget": {"max": 5}, "ratchet": "lower", "tolerance": 0.1, "min_delta": 0.5, "host_bound": true},
          "score":    {"budget": {"min": 90}, "ratchet": "higher"}}}
J
m() { python3 -I scripts/metrics/measure.py "$@"; }

probe 4 95
out="$(m demo --record)"; [[ "$out" == *"no findings"* ]] && ok "first run: within budgets, nothing to compare" || bad "first run: $out"
[ "$(wc -l <"$MEASURE_HISTORY/demo.jsonl")" = 1 ] && ok "--record appends history" || bad "--record appends history"

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
exit "$fail"
