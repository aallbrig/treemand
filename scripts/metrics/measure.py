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
"""
import datetime
import fnmatch
import json
import os
import platform
import subprocess
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
# Overridable so the engine can be tested in isolation (measure_test.sh).
PROBES = os.environ.get("MEASURE_PROBES", os.path.join(ROOT, "scripts", "metrics", "probes"))
RULES = os.environ.get("MEASURE_RULES", os.path.join(ROOT, "scripts", "metrics", "rules.json"))
HISTORY = os.environ.get("MEASURE_HISTORY", os.path.join(ROOT, "docs", "metrics"))


def git(*args):
    p = subprocess.run(["git", *args], cwd=ROOT, capture_output=True, text=True)
    return p.stdout.strip()


def run_probe(probe, input_file):
    if input_file:
        with open(input_file) as f:
            return json.load(f)
    path = os.path.join(PROBES, probe + ".sh")
    if not os.path.exists(path):
        sys.exit(f"measure: no probe {probe!r} (have: {', '.join(list_probes())})")
    p = subprocess.run(["bash", path], cwd=ROOT, stdout=subprocess.PIPE, text=True)
    if p.returncode != 0:
        sys.exit(f"measure: probe {probe} failed (exit {p.returncode})")
    return json.loads(p.stdout)


def list_probes():
    return sorted(f[:-3] for f in os.listdir(PROBES) if f.endswith(".sh"))


def load_rules(probe):
    with open(RULES) as f:
        return json.load(f).get(probe, {})


def rule_for(rules, metric):
    # The most specific glob wins: longest pattern without wildcards counts most.
    matches = [p for p in rules if fnmatch.fnmatchcase(metric, p)]
    if not matches:
        return None
    return rules[max(matches, key=lambda p: (len(p.replace("*", "")), len(p)))]


def last_record(probe, host_bound_host):
    path = os.path.join(HISTORY, probe + ".jsonl")
    if not os.path.exists(path):
        return {}, {}
    any_host, same_host = {}, {}
    with open(path) as f:
        for line in f:
            if not line.strip():
                continue
            rec = json.loads(line)
            for k, v in rec["metrics"].items():
                any_host[k] = (v, rec)
                if rec.get("host") == host_bound_host:
                    same_host[k] = (v, rec)
    return any_host, same_host


def evaluate(probe, result):
    rules = load_rules(probe)
    host = platform.node()
    any_host, same_host = last_record(probe, host)
    findings = []
    for metric, value in sorted(result.get("metrics", {}).items()):
        rule = rule_for(rules, metric)
        if rule is None or not isinstance(value, (int, float)):
            continue
        budget = rule.get("budget", {})
        if "max" in budget and value > budget["max"]:
            findings.append({"kind": "budget", "metric": metric, "value": value, "limit": f"<= {budget['max']}"})
        if "min" in budget and value < budget["min"]:
            findings.append({"kind": "budget", "metric": metric, "value": value, "limit": f">= {budget['min']}"})
        direction = rule.get("ratchet")
        if not direction:
            continue
        prev = (same_host if rule.get("host_bound") else any_host).get(metric)
        if prev is None:
            continue
        before, rec = prev
        delta = value - before
        worse = delta > 0 if direction == "lower" else delta < 0
        better = delta < 0 if direction == "lower" else delta > 0
        tol = rule.get("tolerance", 0.0) * abs(before)
        min_delta = rule.get("min_delta", 0.0)
        since = f"{rec.get('version', '?')} ({rec.get('date', '?')})"
        if worse and abs(delta) > tol and abs(delta) > min_delta:
            findings.append({"kind": "regression", "metric": metric, "value": value, "previous": before, "since": since})
        elif better and abs(delta) > tol and abs(delta) > min_delta:
            findings.append({"kind": "improved", "metric": metric, "value": value, "previous": before, "since": since})
    return findings


def record(probe, result):
    os.makedirs(HISTORY, exist_ok=True)
    rec = {
        "date": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "version": git("describe", "--tags", "--always") or "unknown",
        "sha": git("rev-parse", "--short", "HEAD"),
        "host": platform.node(),
        "metrics": result.get("metrics", {}),
    }
    with open(os.path.join(HISTORY, probe + ".jsonl"), "a") as f:
        f.write(json.dumps(rec, sort_keys=True) + "\n")


def main(argv):
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
        print(json.dumps({"probe": probe, "metrics": result.get("metrics", {}),
                          "details": result.get("details", []), "findings": findings}, indent=2))
    else:
        width = max((len(m) for m in result.get("metrics", {})), default=10)
        for m, v in sorted(result.get("metrics", {}).items()):
            print(f"  {m:<{width}}  {v}")
        for f in findings:
            extra = f"limit {f['limit']}" if f["kind"] == "budget" else f"was {f['previous']} at {f['since']}"
            print(f"{f['kind'].upper():<10} {f['metric']} = {f['value']} ({extra})")
        if not findings:
            print("no findings")
    if "--record" in argv:
        record(probe, result)
        print(f"recorded to {os.path.join(HISTORY, probe + '.jsonl')}", file=sys.stderr)
    if "--gate" in argv and any(f["kind"] == "regression" for f in findings):
        return 3
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
