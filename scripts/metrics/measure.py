#!/usr/bin/env python3
"""measure.py: the measurement engine behind every treemand measurement skill.

    measure.py <probe> [--record] [--gate] [--findings] [--input FILE]
    measure.py --list

A probe is scripts/metrics/probes/<probe>.sh. It prints one JSON object:

    {"probe": "<name>", "metrics": {"<name>": <number>, ...}, "details": [...]}

Metric names may carry a subject after a dot ("cold_s.docker"). Rules live in
scripts/metrics/rules.json, keyed by probe and then by a metric glob:

    {"perf": {"cold_s.*": {"budget": {"max": 5}, "ratchet": "lower",
                           "tolerance": 0.3, "min_delta": 1.0, "host_bound": true}}}

- budget:    absolute limit (max and/or min); a breach is a finding every run.
- ratchet:   "lower" or "higher" is better; a value worse than the last record
             (same host when host_bound) beyond the tolerance (relative) and
             min_delta (absolute) is a regression.
- History:   --record appends {date, version, sha, host, metrics} to
             docs/metrics/<probe>.jsonl, which is what ratchets compare against.
- --gate:    exit 3 when there is any regression (for CI); budget breaches alone
             never fail the gate, they are direction, not a floor.
- --findings print the verdicts as JSON for the interpretation step.
- Load:      above 1.0 load per CPU, wall-clock metrics (rule "timing": true)
             are reported as "unreliable" (not judged), and busy records never
             become their baseline. Counts, CPU time and allocations are judged
             at any load. Each record stores the load it was taken at.
- Host:      host_bound metrics compare against the same host; MEASURE_HOST
             names it when the hostname is not stable (CI runners).
- Skipped:   a probe may name subjects it could not measure on this host,
             {"skipped": {"apt": "not installed"}}. They, and any subject whose
             metrics have a baseline but are absent this run, are reported as
             "skipped", never as a regression; --record keeps the skipped map.
"""

import datetime as dt
import fnmatch
import json
import os
import platform
import subprocess
import sys
from collections.abc import Callable
from pathlib import Path
from typing import Any, cast

# A probe result, a rule, a finding or a history record: parsed JSON objects.
type Obj = dict[str, Any]
# Metric name -> (value, the history record it came from).
type Latest = dict[str, tuple[float, Obj]]

ROOT = Path(__file__).resolve().parents[2]
# Overridable so the engine can be tested in isolation (measure_test.sh).
PROBES = Path(os.environ.get("MEASURE_PROBES", ROOT / "scripts" / "metrics" / "probes"))
RULES = Path(os.environ.get("MEASURE_RULES", ROOT / "scripts" / "metrics" / "rules.json"))
HISTORY = Path(os.environ.get("MEASURE_HISTORY", ROOT / "docs" / "metrics"))

# Above this 1-minute load average per CPU, timing is measuring the machine,
# not treemand: host-bound metrics are reported as unreliable instead of judged,
# and records taken that busy are never used as a ratchet's baseline.
MAX_LOAD_PER_CPU = 1.0
# --gate's exit status when there is a regression.
EXIT_REGRESSION = 3


def host_name() -> str:
    # MEASURE_HOST names a stable host for history recorded elsewhere (CI
    # runners get a new hostname every run).
    return os.environ.get("MEASURE_HOST") or platform.node()


def load_per_cpu() -> float:
    if "MEASURE_LOAD" in os.environ:  # tests
        return float(os.environ["MEASURE_LOAD"])
    return round(os.getloadavg()[0] / (os.cpu_count() or 1), 2)


def git(*args: str) -> str:
    p = subprocess.run(["git", *args], check=False, cwd=ROOT, capture_output=True, text=True)
    return p.stdout.strip()


def history_file(probe: str) -> Path:
    return HISTORY / (probe + ".jsonl")


def run_probe(probe: str, input_file: str | None) -> Obj:
    if input_file:
        with Path(input_file).open() as f:
            return cast("Obj", json.load(f))
    path = PROBES / (probe + ".sh")
    if not path.exists():
        sys.exit(f"measure: no probe {probe!r} (have: {', '.join(list_probes())})")
    p = subprocess.run(["bash", str(path)], check=False, cwd=ROOT, stdout=subprocess.PIPE, text=True)
    if p.returncode != 0:
        sys.exit(f"measure: probe {probe} failed (exit {p.returncode})")
    return cast("Obj", json.loads(p.stdout))


def list_probes() -> list[str]:
    return sorted(f.name[:-3] for f in PROBES.iterdir() if f.name.endswith(".sh"))


def load_rules(probe: str) -> dict[str, Obj]:
    with RULES.open() as f:
        return cast("dict[str, Obj]", json.load(f).get(probe, {}))


def rule_for(rules: dict[str, Obj], metric: str) -> Obj | None:
    # The most specific glob wins: longest pattern without wildcards counts most.
    matches = [p for p in rules if fnmatch.fnmatchcase(metric, p)]
    if not matches:
        return None
    return rules[max(matches, key=lambda p: (len(p.replace("*", "")), len(p)))]


def last_record(probe: str, host_bound_host: str) -> tuple[Latest, Latest, Latest]:
    path = history_file(probe)
    if not path.exists():
        return {}, {}, {}
    any_host: Latest = {}
    same_host: Latest = {}
    same_host_calm: Latest = {}
    with path.open() as f:
        for line in f:
            if not line.strip():
                continue
            rec = json.loads(line)
            calm = rec.get("load", 0) <= MAX_LOAD_PER_CPU
            for k, v in rec["metrics"].items():
                any_host[k] = (v, rec)
                if rec.get("host") == host_bound_host:
                    same_host[k] = (v, rec)
                    if calm:
                        same_host_calm[k] = (v, rec)
    return any_host, same_host, same_host_calm


def budget_findings(metric: str, value: float, budget: Obj) -> list[Obj]:
    findings: list[Obj] = []
    if "max" in budget and value > budget["max"]:
        findings.append({"kind": "budget", "metric": metric, "value": value, "limit": f"<= {budget['max']}"})
    if "min" in budget and value < budget["min"]:
        findings.append({"kind": "budget", "metric": metric, "value": value, "limit": f">= {budget['min']}"})
    return findings


def ratchet_finding(metric: str, value: float, rule: Obj, prev: tuple[float, Obj]) -> Obj | None:
    before, rec = prev
    delta = value - before
    lower = rule["ratchet"] == "lower"
    worse = delta > 0 if lower else delta < 0
    better = delta < 0 if lower else delta > 0
    tol = rule.get("tolerance", 0.0) * abs(before)
    min_delta = rule.get("min_delta", 0.0)
    if not (abs(delta) > tol and abs(delta) > min_delta):
        return None
    since = f"{rec.get('version', '?')} ({rec.get('date', '?')})"
    if worse:
        return {"kind": "regression", "metric": metric, "value": value, "previous": before, "since": since}
    if better:
        return {"kind": "improved", "metric": metric, "value": value, "previous": before, "since": since}
    return None


def baseline(
    rule: Obj, metric: str, any_host: Latest, same_host: Latest, same_host_calm: Latest
) -> tuple[float, Obj] | None:
    # Timing compares with a calm record of this host, host-bound metrics with
    # this host, anything else with the latest record of any host.
    if rule.get("timing", False):
        return same_host_calm.get(metric)
    if rule.get("host_bound"):
        return same_host.get(metric)
    return any_host.get(metric)


def subject(metric: str) -> str:
    # "cold_s.docker" measures docker; a metric without a dot is its own subject.
    return metric.split(".", 1)[1] if "." in metric else metric


def skipped(
    result: Obj,
    rules: dict[str, Obj],
    history: Latest,
    prev_for: Callable[[Obj, str], tuple[float, Obj] | None],
) -> list[Obj]:
    """Report subjects not measured this run; never judge them.

    A subject is skipped when the probe names it, or when one of its metrics
    has a baseline (prev_for) in history and the run has no metric for it.
    """
    out: dict[str, str] = dict(result.get("skipped") or {})
    metrics = result.get("metrics", {})
    measured = {subject(m) for m in metrics}
    for metric in sorted(history):
        rule = rule_for(rules, metric)
        s = subject(metric)
        if metric in metrics or s in measured or s in out or rule is None or not rule.get("ratchet"):
            continue
        if prev_for(rule, metric) is not None:
            out[s] = "absent this run"
    return [{"kind": "skipped", "metric": s, "value": why} for s, why in sorted(out.items())]


def evaluate(probe: str, result: Obj) -> list[Obj]:
    rules = load_rules(probe)
    any_host, same_host, same_host_calm = last_record(probe, host_name())
    load = load_per_cpu()
    busy = load > MAX_LOAD_PER_CPU
    findings: list[Obj] = []
    unreliable: list[str] = []

    def prev_for(rule: Obj, metric: str) -> tuple[float, Obj] | None:
        return baseline(rule, metric, any_host, same_host, same_host_calm)

    for metric, value in sorted(result.get("metrics", {}).items()):
        rule = rule_for(rules, metric)
        if rule is None or not isinstance(value, (int, float)):
            continue
        # Only wall-clock ("timing") metrics depend on load: counts, CPU time
        # and allocations are judged on any machine.
        timing = rule.get("timing", False)
        if busy and timing:
            unreliable.append(metric)
            continue
        findings.extend(budget_findings(metric, value, rule.get("budget", {})))
        if not rule.get("ratchet"):
            continue
        prev = prev_for(rule, metric)
        if prev is None:
            continue
        finding = ratchet_finding(metric, value, rule, prev)
        if finding:
            findings.append(finding)
    findings.extend(skipped(result, rules, any_host, prev_for))
    if unreliable:
        findings.append(
            {
                "kind": "unreliable",
                "metric": ",".join(unreliable),
                "value": load,
                "limit": f"load per CPU <= {MAX_LOAD_PER_CPU}",
            }
        )
    return findings


def record(probe: str, result: Obj) -> None:
    HISTORY.mkdir(parents=True, exist_ok=True)
    rec = {
        "date": dt.datetime.now(dt.UTC).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "version": git("describe", "--tags", "--always") or "unknown",
        "sha": git("rev-parse", "--short", "HEAD"),
        "host": host_name(),
        "load": load_per_cpu(),
        "metrics": result.get("metrics", {}),
    }
    if result.get("skipped"):
        rec["skipped"] = result["skipped"]
    with history_file(probe).open("a") as f:
        f.write(json.dumps(rec, sort_keys=True) + "\n")


def report(result: Obj, findings: list[Obj]) -> None:
    width = max((len(m) for m in result.get("metrics", {})), default=10)
    for m, v in sorted(result.get("metrics", {}).items()):
        print(f"  {m:<{width}}  {v}")
    for f in findings:
        if f["kind"] == "unreliable":
            print(
                f"UNRELIABLE host-bound metrics skipped: load per CPU {f['value']} > {MAX_LOAD_PER_CPU} "
                f"({len(f['metric'].split(','))} metrics); re-run when the machine is quieter"
            )
            continue
        if f["kind"] == "skipped":
            print(f"{'SKIPPED':<10} {f['metric']}: {f['value']}")
            continue
        extra = f"limit {f['limit']}" if f["kind"] == "budget" else f"was {f['previous']} at {f['since']}"
        print(f"{f['kind'].upper():<10} {f['metric']} = {f['value']} ({extra})")
    if all(f["kind"] == "skipped" for f in findings):
        print("no findings")


def main(argv: list[str]) -> int:
    if "--list" in argv:
        print("\n".join(list_probes()))
        return 0
    args = [a for a in argv if not a.startswith("--")]
    if not args:
        print(__doc__)
        return 2
    probe = args[0]
    input_file = argv[argv.index("--input") + 1] if "--input" in argv else None
    result = run_probe(probe, input_file)
    findings = evaluate(probe, result)
    if "--findings" in argv:
        print(
            json.dumps(
                {
                    "probe": probe,
                    "metrics": result.get("metrics", {}),
                    "details": result.get("details", []),
                    "findings": findings,
                },
                indent=2,
            )
        )
    else:
        report(result, findings)
    if "--record" in argv:
        record(probe, result)
        print(f"recorded to {history_file(probe)}", file=sys.stderr)
    if "--gate" in argv and any(f["kind"] == "regression" for f in findings):
        return EXIT_REGRESSION
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
