---
name: treemand-measure
description: The measurement engine every treemand measurement skill runs on (probes, rules, history, ratchets) and how a finding becomes an issue. Use before running any probe, interpreting its findings, or adding a new probe.
---

# Measuring treemand

Improvements are driven by numbers, not impressions. A **probe** measures, the **engine** judges, and you interpret: every finding ends as exactly one tracked issue, or as a written reason it needs none.

## The engine

- `task measure -- <probe>` runs `scripts/metrics/probes/<probe>.sh` and judges each metric against `scripts/metrics/rules.json`. `task measure:list` names the probes.
- Add `--record` to append the result to `docs/metrics/<probe>.jsonl`: the history that ratchets compare against. Record on `main`, after a release, or as the "after" of a fix; never record a broken build.
- Add `--findings` for the JSON your interpretation works from: metrics, the probe's `details`, and verdicts.
- `--gate` exits 3 on a regression (CI uses it on cheap probes).

Verdicts:

- **budget**: the metric is past its absolute limit. That is direction: the gap is work to schedule.
- **regression**: worse than the last record (the same host for host-bound metrics) beyond the rule's tolerance. Something recent broke it: find the commit.
- **improved**: better than the last record. A fix proving itself.
- **unreliable**: the machine is too busy (load per CPU above 1.0) for wall-clock metrics (rules with `"timing": true`) to mean anything; they were not judged. Counts, CPU time and allocations still were. Never file timing from a busy run: wall-clock time is measured on a CI runner (`Measure timing` workflow, recorded with `task measure:pull` under host `gha-ubuntu-latest`).

## From finding to issue

For each budget or regression finding:

1. **Read the evidence** in `details` and confirm the number is real. Findings of one metric family (`excess_lines.*`) are one issue.
2. **Deduplicate**: `gh issue list --search "in:title [measure] <probe>: <metric>" --state open`. If an issue exists, comment the new value instead of filing.
3. **File** with the title `[measure] <probe>: <metric> = <value> (<limit or previous>)` and labels `area:<area>`, `measure:finding` and `loop:ready` (a regression, or a budget gap the probe's skill names a fix for), otherwise `loop:proposed`. The body carries the probe command, the finding JSON, the last history lines, and the skill that knows how to act on it.
4. **Fix and prove**: the PR that fixes it re-runs the probe, quotes the before/after verdict (it should read **improved**), and records the new value with `--record`.

## Adding a probe

A probe is a bash script printing `{"probe", "metrics": {name: number}, "details": [...]}`, with numbers only in `metrics` and `name.subject` for per-item metrics (`cold_s.docker`). Give each metric a rule: a budget for where it should be, a ratchet for which way is better, `host_bound` for anything timing- or machine-dependent. `scripts/metrics/measure_test.sh` covers the engine; a probe with heavy setup prints its own `details` explaining what it measured.
