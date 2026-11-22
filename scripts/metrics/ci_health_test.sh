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

python3 -I - "$GH_WORLD" <<'PY'
import json, sys

def run(i, event, branch, conclusion, created, started=None, updated=None, attempt=1, sha=None):
    return {"id": i, "event": event, "head_branch": branch, "conclusion": conclusion, "run_attempt": attempt,
            "head_sha": sha or f"{i:040d}", "created_at": created, "run_started_at": started or created,
            "updated_at": updated or created}

def job(name, attempt, conclusion, created, started, completed):
    return {"name": name, "run_attempt": attempt, "conclusion": conclusion,
            "created_at": created, "started_at": started, "completed_at": completed}

d = "2026-01-01T"
same = "f" * 40
r1 = run(1, "push", "main", "success", d + "00:00:00Z", updated=d + "00:10:00Z")
r2 = run(2, "pull_request", "topic", "success", d + "01:00:00Z", updated=d + "01:05:00Z", attempt=2)
r3 = run(3, "pull_request", "topic", "startup_failure", d + "01:30:00Z")
r4 = run(4, "pull_request", "topic", "failure", d + "01:40:00Z")
r5 = run(5, "push", "main", "failure", d + "02:00:00Z", updated=d + "02:03:00Z", sha=same)
r6 = run(6, "push", "main", "success", d + "03:00:00Z", updated=d + "03:07:00Z", sha=same)
r7 = run(7, "push", "main", "success", d + "04:00:00Z", updated=d + "04:08:00Z")
r8 = run(8, "push", "main", "skipped", d + "05:00:00Z")
demo = [run(10 + i, "push", "main", c, d + f"0{i}:30:00Z")
        for i, c in enumerate(["success", "success", "failure", "success", "cancelled"])]
api = "repos/{owner}/{repo}/actions"
world = {
    f"{api}/workflows": {"workflows": [
        {"id": 100, "path": ".github/workflows/ci.yml", "state": "active"},
        {"id": 200, "path": ".github/workflows/demo.yml", "state": "active"},
        {"id": 300, "path": "dynamic/dependabot/dependabot-updates", "state": "active"},
    ]},
    f"{api}/workflows/100/runs?branch=": {"workflow_runs": [r6, r5, r4, r3, r2, r1]},
    f"{api}/workflows/100/runs?branch=main": {"workflow_runs": [r8, r7, r6, r5, r1]},
    f"{api}/workflows/200/runs?branch=": {"workflow_runs": demo},
    f"{api}/runs/1/jobs": {"jobs": [
        job("Test", 1, "success", d + "00:00:00Z", d + "00:00:05Z", d + "00:07:00Z"),
        job("Build", 1, "success", d + "00:00:00Z", d + "00:00:20Z", d + "00:01:00Z")]},
    f"{api}/runs/2/jobs": {"jobs": [
        job("Test", 1, "failure", d + "00:50:00Z", d + "00:50:05Z", d + "00:55:00Z"),
        job("Test", 2, "success", d + "01:00:00Z", d + "01:00:05Z", d + "01:04:05Z"),
        job("Lint", 2, "skipped", d + "01:00:00Z", d + "01:00:00Z", d + "01:00:00Z")]},
    f"{api}/runs/3/jobs": {"jobs": []},
    f"{api}/runs/4/jobs": {"jobs": []},
    f"{api}/runs/5/jobs": {"jobs": [job("Test", 1, "failure", d + "02:00:00Z", d + "02:00:05Z", d + "02:03:00Z")]},
    f"{api}/runs/6/jobs": {"jobs": [job("Test", 1, "success", d + "03:00:00Z", d + "03:00:05Z", d + "03:06:05Z")]},
}
json.dump(world, open(sys.argv[1], "w"))
PY

mkdir -p "$tmp/bin"
cat >"$tmp/bin/gh" <<'PY'
#!/usr/bin/env python3
# Fake gh: `gh api -X GET <path>` answered from $GH_WORLD, paginated by per_page/page.
import json, os, sys
from urllib.parse import parse_qs
path = sys.argv[-1]
with open(os.environ["GH_CALLS"], "a") as f:
    f.write(path + "\n")
base, _, query = path.partition("?")
q = {k: v[0] for k, v in parse_qs(query).items()}
world = json.load(open(os.environ["GH_WORLD"]))
key = base + ("?branch=" + q.get("branch", "") if base.endswith("/runs") else "")
if key not in world:
    sys.exit(f"fake gh: no {key}")
body = world[key]
for field in ("jobs", "workflow_runs"):
    if field in body:
        items = body[field]
        if field == "workflow_runs" and q.get("status") != "completed":
            sys.exit("fake gh: runs must be listed with status=completed")
        per, page = int(q.get("per_page", 30)), int(q.get("page", 1))
        body = {"total_count": len(items), field: items[(page - 1) * per : page * per]}
print(json.dumps(body))
PY
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
