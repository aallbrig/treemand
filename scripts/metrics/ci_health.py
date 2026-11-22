#!/usr/bin/env python3
"""ci_health.py: the ci-health probe's measurements (GitHub Actions reliability and speed).

    ci_health.py [--runs N] [--cache DIR]

Prints one probe result (see scripts/metrics/measure.py). Reads only the
GitHub Actions API through `gh api`, for the repository `gh` resolves from the
current checkout (or GH_REPO). Each of the repository's own workflows
(.github/workflows/*.yml; GitHub's dynamic ones such as Dependabot are left
out) is judged on its last N (default 50) completed runs.

Metrics:
- success_pct.<workflow>: share of successful runs on main, excluding
  skipped and cancelled runs. "On main" means every run that is not a
  pull_request event: pushes to main, tags cut from it, schedules and
  dispatches. A workflow that runs on pull requests gets a second listing
  filtered to branch main, so its window holds N main runs.
- ci_duration_p50_s, ci_duration_p90_s: wall-clock duration (latest attempt,
  run_started_at to updated_at) of successful CI runs, any branch.
- flaky_rate_pct: flaky cases per 100 (workflow, commit, event) groups. A case
  is a run whose earlier attempt failed and whose final attempt succeeded
  (re-run), or a commit on main that failed in one run and succeeded in a later
  run of the same workflow.
- ci_longest_job_s: median duration of CI's slowest job (detail; the job is named).
- ci_queue_p90_s: p90 of job created_at to started_at across CI jobs (detail).
- startup_failures: runs that concluded startup_failure, or CI runs with no
  jobs at all (detail).

Cost: one call for the workflow list, one per workflow (two for a workflow
that runs on pull requests), and one jobs call per CI run and per re-run run.
Jobs of a completed run attempt never change, so they are cached per run and
attempt in DIR (default $XDG_CACHE_HOME/treemand/ci-health, or
~/.cache/treemand/ci-health); a repeat run costs about a dozen calls.
"""

import argparse
import json
import math
import os
import re
import statistics
import subprocess
import sys
from collections import defaultdict
from collections.abc import Callable
from datetime import datetime
from pathlib import Path
from typing import Any

type Obj = dict[str, Any]

API = "repos/{owner}/{repo}/actions"
CI_PATH = ".github/workflows/ci.yml"
JOBS_PER_PAGE = int(os.environ.get("CI_HEALTH_JOBS_PER_PAGE", "100"))
# Conclusions that say nothing about whether the workflow works.
NOT_JUDGED = {"skipped", "cancelled", "neutral", "stale"}
FAILED = {"failure", "timed_out", "startup_failure"}


def gh(path: str) -> Obj:
    p = subprocess.run(["gh", "api", "-X", "GET", path], check=False, capture_output=True, text=True)
    if p.returncode != 0:
        sys.exit(f"ci-health: gh api {path} failed: {p.stderr.strip()}")
    data: Obj = json.loads(p.stdout)
    return data


def default_cache() -> Path:
    base = os.environ.get("XDG_CACHE_HOME") or str(Path.home() / ".cache")
    return Path(base) / "treemand" / "ci-health"


def ts(value: str | None) -> datetime | None:
    return datetime.fromisoformat(value) if value else None


def seconds(start: str | None, end: str | None) -> float | None:
    a, b = ts(start), ts(end)
    if a is None or b is None:
        return None
    return max(0.0, (b - a).total_seconds())


def percentile(values: list[float], pct: float) -> float:
    # Nearest rank: the smallest value with at least pct% of values at or below it.
    ordered = sorted(values)
    rank = max(1, math.ceil(pct / 100 * len(ordered)))
    return ordered[rank - 1]


def slug(workflow: Obj) -> str:
    name = Path(workflow["path"]).stem
    return re.sub(r"[^a-z0-9]+", "-", name.lower()).strip("-")


class Jobs:
    """Jobs of a run across all its attempts, cached per run and attempt."""

    def __init__(self, cache: Path) -> None:
        self.cache = cache
        self.calls = 0
        self.cache_hits = 0
        self.used: set[str] = set()
        self.memo: dict[str, list[Obj]] = {}
        cache.mkdir(parents=True, exist_ok=True)

    def of(self, run: Obj) -> list[Obj]:
        name = f"jobs-{run['id']}-{run['run_attempt']}.json"
        self.used.add(name)
        if name in self.memo:
            return self.memo[name]
        path = self.cache / name
        if path.exists():
            self.cache_hits += 1
            self.memo[name] = json.loads(path.read_text())
            return self.memo[name]
        jobs: list[Obj] = []
        page = 1
        while True:
            data = gh(f"{API}/runs/{run['id']}/jobs?filter=all&per_page={JOBS_PER_PAGE}&page={page}")
            self.calls += 1
            jobs.extend(data.get("jobs", []))
            if len(data.get("jobs", [])) < JOBS_PER_PAGE or len(jobs) >= data.get("total_count", 0):
                break
            page += 1
        path.write_text(json.dumps(jobs))
        self.memo[name] = jobs
        return jobs

    def prune(self) -> None:
        # Runs that left the window are never asked for again.
        for f in self.cache.glob("jobs-*.json"):
            if f.name not in self.used:
                f.unlink()


def runs_of(workflow: Obj, n: int, branch: str | None = None) -> list[Obj]:
    query = f"status=completed&per_page={n}" + (f"&branch={branch}" if branch else "")
    data = gh(f"{API}/workflows/{workflow['id']}/runs?{query}")
    runs: list[Obj] = data.get("workflow_runs", [])
    return runs


def success_pct(runs: list[Obj]) -> tuple[float | None, int]:
    judged = [r for r in runs if r.get("conclusion") not in NOT_JUDGED]
    if not judged:
        return None, 0
    ok = sum(1 for r in judged if r.get("conclusion") == "success")
    return round(100 * ok / len(judged), 1), len(judged)


def failed_jobs(jobs: list[Obj], attempt: int) -> list[str]:
    return sorted({j["name"] for j in jobs if j.get("run_attempt") == attempt and j.get("conclusion") in FAILED})


def rerun_cases(wf: str, runs: list[Obj], jobs: Jobs) -> list[Obj]:
    cases = []
    for r in runs:
        if r.get("run_attempt", 1) == 1 or r.get("conclusion") != "success":
            continue
        all_jobs = jobs.of(r)
        failed = sorted(
            {
                j["name"]
                for j in all_jobs
                if j.get("run_attempt", 1) < r["run_attempt"] and j.get("conclusion") in FAILED
            }
        )
        if failed:
            cases.append(
                {
                    "kind": "rerun",
                    "workflow": wf,
                    "run": r["id"],
                    "sha": r["head_sha"][:12],
                    "branch": r.get("head_branch"),
                    "attempts": r["run_attempt"],
                    "failed_jobs": failed,
                }
            )
    return cases


def same_sha_cases(wf: str, runs: list[Obj], jobs_of: Callable[[Obj], list[Obj]]) -> list[Obj]:
    by_sha: dict[str, list[Obj]] = defaultdict(list)
    for r in runs:
        if r.get("event") != "pull_request" and r.get("head_branch") == "main":
            by_sha[r["head_sha"]].append(r)
    cases = []
    for sha, group in by_sha.items():
        group.sort(key=lambda r: r["created_at"])
        first_fail = next((r for r in group if r.get("conclusion") in FAILED), None)
        if first_fail is None:
            continue
        later_ok = [r for r in group if r["created_at"] > first_fail["created_at"] and r.get("conclusion") == "success"]
        if later_ok:
            jobs = jobs_of(first_fail)
            cases.append(
                {
                    "kind": "same_sha",
                    "workflow": wf,
                    "run": first_fail["id"],
                    "passed_in": later_ok[0]["id"],
                    "sha": sha[:12],
                    "branch": "main",
                    "failed_jobs": failed_jobs(jobs, first_fail.get("run_attempt", 1)),
                }
            )
    return cases


def groups(runs: list[Obj]) -> set[tuple[str, str]]:
    return {(r["head_sha"], r.get("event", "")) for r in runs}


def ci_timings(runs: list[Obj], ci_jobs: dict[int, list[Obj]]) -> tuple[Obj, Obj]:
    metrics: Obj = {}
    detail: Obj = {"name": "ci timing", "runs": len(runs)}
    durations = [
        d
        for r in runs
        if r.get("conclusion") == "success"
        if (d := seconds(r.get("run_started_at"), r.get("updated_at"))) is not None
    ]
    if durations:
        metrics["ci_duration_p50_s"] = round(percentile(durations, 50))
        metrics["ci_duration_p90_s"] = round(percentile(durations, 90))
        detail["successful_runs_timed"] = len(durations)
    per_job: dict[str, list[float]] = defaultdict(list)
    queues: list[float] = []
    for r in runs:
        for j in ci_jobs.get(r["id"], []):
            if j.get("run_attempt") != r.get("run_attempt") or j.get("conclusion") in NOT_JUDGED:
                continue
            if (q := seconds(j.get("created_at"), j.get("started_at"))) is not None:
                queues.append(q)
            if r.get("conclusion") == "success" and (d := seconds(j.get("started_at"), j.get("completed_at"))):
                per_job[j["name"]].append(d)
    if per_job:
        medians = {name: statistics.median(ds) for name, ds in per_job.items()}
        top = sorted(medians.items(), key=lambda kv: -kv[1])
        metrics["ci_longest_job_s"] = round(top[0][1])
        detail["longest_jobs_median_s"] = {name: round(s) for name, s in top[:5]}
    if queues:
        metrics["ci_queue_p90_s"] = round(percentile(queues, 90))
        detail["queue_p50_s"] = round(percentile(queues, 50))
        detail["jobs_timed"] = len(queues)
    return metrics, detail


def measure(n: int, cache: Path) -> Obj:
    workflows = [
        w
        for w in gh(f"{API}/workflows?per_page=100").get("workflows", [])
        if w.get("path", "").startswith(".github/workflows/") and w.get("state") == "active"
    ]
    calls = 1
    jobs = Jobs(cache)
    metrics: Obj = {}
    details: list[Obj] = []
    success: Obj = {}
    flaky: list[Obj] = []
    all_groups: set[tuple[str, str, str]] = set()
    startup: list[Obj] = []
    ci_runs: list[Obj] = []
    ci_jobs: dict[int, list[Obj]] = {}
    for w in sorted(workflows, key=slug):
        name = slug(w)
        runs = runs_of(w, n)
        calls += 1
        main_runs = [r for r in runs if r.get("event") != "pull_request"]
        if len(main_runs) < len(runs):
            main_runs = runs_of(w, n, branch="main")
            main_runs = [r for r in main_runs if r.get("event") != "pull_request"]
            calls += 1
        pct, judged = success_pct(main_runs)
        if pct is not None:
            metrics[f"success_pct.{name}"] = pct
            success[name] = {
                "pct": pct,
                "judged": judged,
                "failed_runs": [r["id"] for r in main_runs if r.get("conclusion") in FAILED][:10],
            }
        window = {r["id"]: r for r in runs + main_runs}
        everything = list(window.values())
        all_groups |= {(name, *g) for g in groups(everything)}
        startup += [{"workflow": name, "run": r["id"]} for r in everything if r.get("conclusion") == "startup_failure"]
        is_ci = w["path"] == CI_PATH
        if is_ci:
            # Jobs (durations, queue, empty runs) for the last N runs of any branch.
            ci_runs = runs
            for r in runs:
                ci_jobs[r["id"]] = jobs.of(r)
            startup += [
                {"workflow": name, "run": r["id"], "no_jobs": True}
                for r in runs
                if not ci_jobs[r["id"]] and r.get("conclusion") != "startup_failure"
            ]
        flaky += rerun_cases(name, everything, jobs)
        flaky += same_sha_cases(name, everything, jobs.of)
    jobs.prune()
    metrics["flaky_rate_pct"] = round(100 * len(flaky) / len(all_groups), 2) if all_groups else 0
    metrics["startup_failures"] = len(startup)
    timing_metrics, timing_detail = ci_timings(ci_runs, ci_jobs)
    metrics.update(timing_metrics)
    details.append({"name": "success on main", "window": n, "workflows": success})
    details.append({"name": "flaky", "cases": flaky, "groups": len(all_groups)})
    details.append(timing_detail)
    details.append({"name": "startup failures", "runs": startup})
    details.append({"name": "api", "calls": calls + jobs.calls, "jobs_cache_hits": jobs.cache_hits})
    return {"probe": "ci-health", "metrics": metrics, "details": details}


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--runs", type=int, default=int(os.environ.get("CI_HEALTH_RUNS", "50")))
    ap.add_argument("--cache", type=Path, default=Path(os.environ.get("CI_HEALTH_CACHE") or default_cache()))
    args = ap.parse_args()
    print(json.dumps(measure(args.runs, args.cache), indent=2))


if __name__ == "__main__":
    main()
