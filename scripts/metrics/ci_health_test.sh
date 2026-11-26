#!/bin/sh
# Behaviour tests for the ci-health probe (scripts/metrics/ci_health.py),
# against a fake `gh` first on PATH that serves a small Actions API world and
# paginates like GitHub. The world holds one of each case the probe counts:
# a re-run that turned a failure green, a main commit that failed then passed
# in a later run, a startup failure, a run with no jobs, a cancelled run, and
# a dynamic workflow that must be left out.
here="$(cd "$(dirname "$0")" && pwd)"
probe="$here/ci_health.py"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0
export CI_HEALTH_CACHE="$tmp/cache" CI_HEALTH_JOBS_PER_PAGE=1 GH_CALLS="$tmp/calls.log" GH_WORLD="$tmp/world.json"

metric() { python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["metrics"].get(sys.argv[2], "absent"))' "$1" "$2"; }
expect() { # expect <json-file> <metric> <value> <what>
  got="$(metric "$1" "$2")"
  if [ "$got" = "$3" ]; then echo "ok    $4"; else echo "FAIL  $4: $2 = $got, want $3"; fail=1; fi
}
check() { # check <what> <python expression over d (the probe result)>
  if python3 -I -c 'import json,sys; d=json.load(open(sys.argv[1])); sys.exit(0 if eval(sys.argv[2]) else 1)' "$tmp/out.json" "$2"; then
    echo "ok    $1"
  else
    echo "FAIL  $1"
    fail=1
  fi
}

python3 -I "$here/ci_health_test_world.py" "$GH_WORLD"

mkdir -p "$tmp/bin"
cp "$here/ci_health_test_fake_gh.py" "$tmp/bin/gh"
chmod +x "$tmp/bin/gh"
PATH="$tmp/bin:$PATH"

python3 -I "$probe" >"$tmp/out.json" 2>"$tmp/err" || { echo "FAIL  probe exited non-zero"; cat "$tmp/err"; exit 1; }
expect "$tmp/out.json" success_pct.ci 75.0      "CI success counts main runs only, skipped runs left out"
expect "$tmp/out.json" success_pct.demo 75.0    "a cancelled run is not judged"
expect "$tmp/out.json" success_pct.dependabot-updates absent "dynamic workflows are left out"
expect "$tmp/out.json" ci_duration_p50_s 420    "CI duration p50 over successful runs (nearest rank)"
expect "$tmp/out.json" ci_duration_p90_s 600    "CI duration p90"
expect "$tmp/out.json" ci_longest_job_s 360     "longest CI job by median, latest attempt of successful runs"
expect "$tmp/out.json" ci_queue_p90_s 20        "queue p90 over CI jobs, skipped jobs left out"
expect "$tmp/out.json" startup_failures 2       "startup failures and runs with no jobs"
expect "$tmp/out.json" flaky_rate_pct 16.67     "a green re-run and a main commit that passed later: 2 of 12 commit groups"
check "the re-run names the job that failed first" \
  'any(c["kind"] == "rerun" and c["failed_jobs"] == ["Test"] for c in d["details"][1]["cases"])'
check "the same-commit case names the run it passed in" \
  'any(c["kind"] == "same_sha" and c["passed_in"] == 6 and c["failed_jobs"] == ["Test"] for c in d["details"][1]["cases"])'

first="$(wc -l <"$GH_CALLS")"
if [ "$first" -eq 13 ]; then echo "ok    jobs pages are followed (13 calls)"; else echo "FAIL  first run made $first calls, want 13"; fail=1; fi
: >"$GH_CALLS"
python3 -I "$probe" >"$tmp/out.json" 2>"$tmp/err" || { echo "FAIL  second run exited non-zero"; cat "$tmp/err"; fail=1; }
second="$(wc -l <"$GH_CALLS")"
if [ "$second" -eq 4 ]; then echo "ok    a repeat run reads jobs from the cache (4 calls)"; else echo "FAIL  second run made $second calls, want 4"; fail=1; fi
expect "$tmp/out.json" flaky_rate_pct 16.67     "cached jobs give the same answer"
exit "$fail"
