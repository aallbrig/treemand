---
name: treemand-ci-health
description: Measure treemand's GitHub Actions health (success rate on main per workflow, CI duration, flaky re-runs, slowest job, queue time, startup failures) and turn findings into fixes. Use for a red or slow CI question, a flaky test or job, a workflow that keeps failing on main, or ci-health findings.
---

# CI health

Follow **treemand-measure** for running, recording and filing; this skill is the CI playbook.

## Measure

- `task ci:health` (same as `task measure -- ci-health`; add `--findings` for the evidence, `--record` for history). It reads only the GitHub Actions API through `gh api`, the last 50 completed runs per workflow in `.github/workflows/` (`CI_HEALTH_RUNS` changes the window). GitHub's dynamic workflows (Dependabot, Pages, Copilot) are left out.
- Cost: one call per workflow (two for one that runs on pull requests, to fill a window of main runs) plus one jobs call per CI run. Jobs of a finished attempt never change, so they are cached per run and attempt under `~/.cache/treemand/ci-health` (`CI_HEALTH_CACHE`): the first run takes about a minute and some 65 calls, a repeat about a dozen calls.
- The weekly workflow (`measure-weekly.yml`) runs it with the workflow token and uploads `ci-health.json` with the other findings.
- The metrics are not host-bound, since every run is on GitHub's runners: record from any machine with `MEASURE_HOST=github-actions-api` so the history carries no machine name.

The probe's docstring (`scripts/metrics/ci_health.py`) defines each metric; `scripts/metrics/rules.json` holds the rules.

| metric | measures | rule |
| --- | --- | --- |
| `success_pct.<workflow>` | successful share of non-pull-request runs (main pushes, tags, schedules, dispatches); skipped and cancelled runs left out | budget at least 90 (95 for `ci`); ratchet up |
| `ci_duration_p50_s`, `ci_duration_p90_s` | wall-clock time of successful CI runs | ratchet down, 20% tolerance |
| `flaky_rate_pct` | flaky cases per 100 commit groups: a run turned green by a re-run, or a main commit that failed and passed in a later run | ratchet down |
| `ci_longest_job_s`, `ci_queue_p90_s`, `startup_failures` | slowest CI job by median, job queue p90, runs that never started | detail, no rule |

## Act

1. **Read the failing runs first.** `details` lists each workflow's failed run IDs and each flaky case with the jobs that failed. `gh run view <run>` names the failed job; `gh run view --job <job> --log-failed` shows why. Group runs by cause: one cause is one issue, whatever the number of runs.
2. **A test that fails then passes** is a flaky test. Search open issues for its name (`gh issue list --search "<TestName> in:title,body"`): a tracked one gets a comment with the new run IDs, not a new issue. A new one is filed with the test, the failure line, the run IDs and the condition it depends on (load, timing, ordering).
3. **A workflow failing on main for a reason outside the code** (a push race, a runner image change, a cache collision, an expired token) gets an issue against the workflow file. Fix the workflow, not the symptom: a step that pushes to main rebases and retries; a cache restore that collides uses a unique key or `restore-keys`.
4. **CI got slower**: `longest_jobs_median_s` names the jobs; compare with the last record's details. A job that grew is usually a new gate in `task ci` or a lost cache. Split or cache it; never drop a gate to save time.
5. **Queue time or startup failures rising** are GitHub- or configuration-side: a startup failure is almost always a workflow syntax or permissions error, visible on the run page.

## Prove it

The fixing PR cannot prove itself in one run: the window is 50 runs. Comment the fix on the issue, and the next weekly run (or `task ci:health` a few days later) shows the rate move; record it then and close the issue with the **improved** verdict.
