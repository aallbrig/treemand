#!/bin/sh
# Tests for `task gates:baselines` (#469): a branch may tighten a ratchet's
# baseline, a coverage floor or a rules.json budget, never loosen one, unless
# the pull request is labelled gates:loosen.
# The `check && ok || bad` idiom is safe here: ok only echoes (SC2015).
# shellcheck disable=SC2015
set -u
cd "$(dirname "$0")/../.." || exit 1
here="$(pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail=0
ok() { echo "  ok   $1"; }
bad() { echo "  FAIL $1"; fail=1; }
has() { case "$1" in *"$2"*) ;; *) return 1 ;; esac; } # has <text> <fragment>

repo="$work/repo"
g() { git -C "$repo" "$@"; }
rec() { # rec <score> [host] [load]: one history line of the demo probe
  printf '{"date": "2026-01-01T00:00:00Z", "host": "%s", "load": %s, "metrics": {"score": %s, "cold_s.x": 1.0, "calls": 3}, "sha": "0", "version": "v0"}\n' "${2:-h1}" "${3:-0.1}" "$1"
}
# A base commit, then reset the work tree to it before each case.
mkdir -p "$repo/scripts/metrics" "$repo/docs/metrics"
git init -q "$repo"
g config user.email test@example.com
g config user.name test
g config commit.gpgsign false
cat >"$repo/scripts/metrics/rules.json" <<'J'
{"demo": {"score":    {"budget": {"min": 90}, "ratchet": "higher"},
          "cold_s.*": {"budget": {"max": 5}, "ratchet": "lower", "tolerance": 0.1, "min_delta": 0.5, "host_bound": true, "timing": true},
          "calls":    {"budget": {"max": 10}, "ratchet": "lower"}},
 "other": {"n": {"budget": {"max": 0}}}}
J
printf '# header\npkg/a\t80\npkg/b\t90\n' >"$repo/scripts/coverage_floor.tsv"
rec 95 >"$repo/docs/metrics/demo.jsonl"
g add -A && g commit -qm base
fresh() { g checkout -q -- . && g clean -qfd; }
check() { BASELINES_ROOT="$repo" python3 -I "$here/scripts/metrics/check_baselines.py" --base HEAD 2>&1; }

out="$(check)"; st=$?
[ "$st" = 0 ] && ok "no change passes" || bad "no change: exit $st: $out"

# History: append-only, and a new record no worse than the base's last.
fresh; rec 97 >>"$repo/docs/metrics/demo.jsonl"
out="$(check)"; st=$?
[ "$st" = 0 ] && ok "a better record passes" || bad "better record: exit $st: $out"

fresh; rec 80 >>"$repo/docs/metrics/demo.jsonl"
out="$(check)"; st=$?
[ "$st" = 4 ] && has "$out" "demo score" && ok "a worse record of a ratcheted metric fails" || bad "worse record: exit $st: $out"

fresh; rec 95 >>"$repo/docs/metrics/demo.jsonl"; sed 's/"calls": 3/"calls": 2/' "$repo/docs/metrics/demo.jsonl" >"$work/h" && cp "$work/h" "$repo/docs/metrics/demo.jsonl"
out="$(check)"; st=$?
[ "$st" = 4 ] && has "$out" "rewrites history" && ok "rewriting a base record fails" || bad "rewritten history: exit $st: $out"

fresh; rm "$repo/docs/metrics/demo.jsonl"
out="$(check)"; st=$?
[ "$st" = 4 ] && has "$out" "deletes" && ok "deleting a history file fails" || bad "deleted history: exit $st: $out"

fresh; printf '{"date": "x", "host": "h2", "load": 0.1, "metrics": {"cold_s.x": 4.0}, "sha": "0", "version": "v0"}\n' >>"$repo/docs/metrics/demo.jsonl"
out="$(check)"; st=$?
[ "$st" = 0 ] && ok "a host-bound metric is judged against its own host only" || bad "other host: exit $st: $out"

fresh; printf '{"date": "x", "host": "h1", "load": 3.0, "metrics": {"cold_s.x": 4.0}, "sha": "0", "version": "v0"}\n' >>"$repo/docs/metrics/demo.jsonl"
out="$(check)"; st=$?
[ "$st" = 0 ] && ok "a busy record of a timing metric is not judged" || bad "busy record: exit $st: $out"

# Coverage floors: none falls, no package disappears.
fresh; printf '# header\npkg/a\t81\npkg/b\t90\npkg/c\t50\n' >"$repo/scripts/coverage_floor.tsv"
out="$(check)"; st=$?
[ "$st" = 0 ] && ok "raising a floor and adding a package passes" || bad "tighter floors: exit $st: $out"

fresh; printf '# header\npkg/a\t79\npkg/b\t90\n' >"$repo/scripts/coverage_floor.tsv"
out="$(check)"; st=$?
[ "$st" = 4 ] && has "$out" "pkg/a" && ok "a lower coverage floor fails" || bad "lower floor: exit $st: $out"

fresh; printf '# header\npkg/a\t80\n' >"$repo/scripts/coverage_floor.tsv"
out="$(check)"; st=$?
[ "$st" = 4 ] && has "$out" "pkg/b" && ok "a package losing its floor fails" || bad "dropped floor: exit $st: $out"

# rules.json: no budget loosens, no rule or probe disappears.
rules() { python3 -I -c "import json,sys; r=json.load(open(sys.argv[1])); exec(sys.argv[2]); json.dump(r, open(sys.argv[1], 'w'))" "$repo/scripts/metrics/rules.json" "$1"; }
fresh; rules "r['demo']['calls']['budget']['max']=5; r['demo']['cold_s.*']['tolerance']=0.05; r['demo']['new']={'ratchet':'lower'}; r['demo']['cold_s.fast']={'budget':{'max':1},'ratchet':'lower','timing':True,'host_bound':True}"
out="$(check)"; st=$?
[ "$st" = 0 ] && ok "tightening rules passes" || bad "tighter rules: exit $st: $out"

for case in \
  "r['demo']['calls']['budget']['max']=20|budget max" \
  "r['demo']['score']['budget']['min']=80|budget min" \
  "del r['demo']['score']['budget']['min']|budget min" \
  "del r['demo']['calls']['ratchet']|ratchet" \
  "r['demo']['calls']['ratchet']='higher'|ratchet" \
  "r['demo']['cold_s.*']['tolerance']=0.5|tolerance" \
  "r['demo']['cold_s.*']['min_delta']=1|min_delta" \
  "r['demo']['calls']['timing']=True|timing" \
  "del r['demo']['calls']|removed" \
  "del r['other']|removed" \
  "r['demo']['cold_s.slow']={'budget':{'max':9}}|shadows a rule"; do
  fresh; rules "${case%|*}"
  out="$(check)"; st=$?
  [ "$st" = 4 ] && has "$out" "${case#*|}" && ok "rules loosening fails: ${case%|*}" || bad "rules ${case%|*}: exit $st: $out"
done

# A deliberate loosening is accepted with the label.
fresh; printf '# header\npkg/a\t70\npkg/b\t90\n' >"$repo/scripts/coverage_floor.tsv"
out="$(GATES_LOOSEN=1 check)"; st=$?
[ "$st" = 0 ] && has "$out" "gates:loosen" && ok "GATES_LOOSEN=1 accepts a loosening and says so" || bad "GATES_LOOSEN: exit $st: $out"

# Wiring: task ci:gates (the CI gates job, and part of task ci) runs the check and its test.
ci="$(sed -n '/^  ci:gates:$/,/^  [a-z:-]*:$/p' Taskfile.yml)"
case "$ci" in *"task: gates:baselines"*) ok "task ci:gates runs gates:baselines" ;; *) bad "task ci:gates does not run gates:baselines" ;; esac
case "$ci" in *"task: gates:baselines:test"*) ok "task ci:gates runs gates:baselines:test" ;; *) bad "task ci:gates does not run gates:baselines:test" ;; esac
grep -q 'GATES_PR:' .github/workflows/ci.yml && ok "the CI gates job passes the pull request number" || bad "ci.yml does not set GATES_PR"

[ "$fail" = 0 ] && echo "check_baselines_test: ok"
exit "$fail"
