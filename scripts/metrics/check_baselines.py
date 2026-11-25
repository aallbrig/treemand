#!/usr/bin/env python3
"""check_baselines.py: fail when a branch loosens a gate's own baseline (#469).

    check_baselines.py [--base REF]

Compares the work tree with REF (default: the merge base with origin/main):

- docs/metrics/*.jsonl: history is append-only, and a new record of a ratcheted
  metric is no worse than the base's last record, judged with the base's rules
  and measure.py's own ratchet logic (tolerance, host, load).
- scripts/coverage_floor.tsv: no floor falls, no package loses its floor.
- scripts/metrics/rules.json: no budget, ratchet, tolerance or min_delta
  loosens, no rule starts skipping busy runs (timing), and no rule or probe
  disappears.

Tightening always passes. A deliberate loosening passes with GATES_LOOSEN=1, or
when GATES_PR names a pull request labelled gates:loosen (read live with gh, so
re-running the CI job after labelling is enough). Exit 4 on a loosening.
BASELINES_ROOT points the check at another repository (tests).
"""

import importlib.util
import json
import os
import subprocess
import sys
from pathlib import Path
from types import ModuleType
from typing import Any

type Obj = dict[str, Any]

HERE = Path(__file__).resolve().parent
ROOT = Path(os.environ.get("BASELINES_ROOT") or HERE.parents[1])
HISTORY_DIR = "docs/metrics"
FLOORS = "scripts/coverage_floor.tsv"
RULES = "scripts/metrics/rules.json"
LABEL = "gates:loosen"
FLOOR_COLUMNS = 2  # package, floor
EXIT_LOOSENED = 4


def load_measure() -> ModuleType:
    # measure.py is a script, not a package: load it by path (python3 -I does
    # not put this directory on sys.path).
    spec = importlib.util.spec_from_file_location("measure", HERE / "measure.py")
    if spec is None or spec.loader is None:
        sys.exit("gates:baselines: cannot load measure.py")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


measure = load_measure()


def git(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(["git", *args], check=False, cwd=ROOT, capture_output=True, text=True)


def at_base(base: str, path: str) -> str | None:
    p = git("show", f"{base}:{path}")
    return p.stdout if p.returncode == 0 else None


def in_tree(path: str) -> str | None:
    f = ROOT / path
    return f.read_text() if f.exists() else None


def lines(text: str) -> list[str]:
    return [line for line in text.splitlines() if line.strip()]


# ── history ──────────────────────────────────────────────────────────────────


def judge_record(probe: str, rec: Obj, rules: dict[str, Obj], base_lines: list[str]) -> list[str]:
    any_host, same_host, calm = measure.latest(base_lines, rec.get("host", ""))
    busy = rec.get("load", 0) > measure.MAX_LOAD_PER_CPU
    out: list[str] = []
    for metric, value in sorted(rec.get("metrics", {}).items()):
        rule = measure.rule_for(rules, metric)
        if not rule or not rule.get("ratchet") or not isinstance(value, (int, float)):
            continue
        if busy and rule.get("timing"):
            continue  # measure.py never judges a busy timing record
        prev = measure.baseline(rule, metric, any_host, same_host, calm)
        finding = prev and measure.ratchet_finding(metric, value, rule, prev)
        if finding and finding["kind"] == "regression":
            out.append(f"{probe} {metric}: new record {value} is worse than base {finding['previous']}")
    return out


def check_history(base: str, rules: Obj) -> list[str]:
    names = git("ls-tree", "--name-only", f"{base}:{HISTORY_DIR}").stdout.split()
    out: list[str] = []
    for name in sorted(n for n in names if n.endswith(".jsonl")):
        path = f"{HISTORY_DIR}/{name}"
        old, new = lines(at_base(base, path) or ""), in_tree(path)
        probe = name.removesuffix(".jsonl")
        if new is None:
            out.append(f"{path}: the branch deletes metric history")
            continue
        new_lines = lines(new)
        if new_lines[: len(old)] != old:
            out.append(f"{path}: the branch rewrites history (it is append-only)")
            continue
        for line in new_lines[len(old) :]:
            out.extend(judge_record(probe, json.loads(line), rules.get(probe, {}), old))
    return out


# ── coverage floors ──────────────────────────────────────────────────────────


def floors(text: str) -> dict[str, float]:
    rows = (line.split("\t") for line in lines(text) if not line.startswith("#"))
    return {r[0]: float(r[1]) for r in rows if len(r) >= FLOOR_COLUMNS}


def check_floors(base: str) -> list[str]:
    old, new = floors(at_base(base, FLOORS) or ""), floors(in_tree(FLOORS) or "")
    out: list[str] = []
    for pkg, floor in sorted(old.items()):
        if pkg not in new:
            out.append(f"{FLOORS}: {pkg} loses its floor ({floor:g})")
        elif new[pkg] < floor:
            out.append(f"{FLOORS}: {pkg} floor falls from {floor:g} to {new[pkg]:g}")
    return out


# ── rules.json ───────────────────────────────────────────────────────────────


def rule_loosenings(old: Obj, new: Obj) -> list[str]:
    ob, nb = old.get("budget", {}), new.get("budget", {})
    out: list[str] = []
    if "max" in ob and nb.get("max", float("inf")) > ob["max"]:
        out.append(f"budget max {ob['max']} -> {nb.get('max', 'none')}")
    if "min" in ob and nb.get("min", float("-inf")) < ob["min"]:
        out.append(f"budget min {ob['min']} -> {nb.get('min', 'none')}")
    if old.get("ratchet") and new.get("ratchet") != old["ratchet"]:
        out.append(f"ratchet {old['ratchet']} -> {new.get('ratchet', 'none')}")
    out.extend(
        f"{key} {old.get(key, 0)} -> {new[key]}"
        for key in ("tolerance", "min_delta")
        if new.get(key, 0) > old.get(key, 0)
    )
    if new.get("timing") and not old.get("timing"):
        out.append("timing added (busy runs stop being judged)")
    return out


def check_rules(old: Obj, new: Obj) -> list[str]:
    out: list[str] = []
    for probe, old_rules in sorted(old.items()):
        if probe not in new:
            out.append(f"{RULES}: probe {probe} removed")
            continue
        for glob, rule in sorted(old_rules.items()):
            if glob not in new[probe]:
                out.append(f"{RULES}: {probe} {glob} removed")
                continue
            out.extend(f"{RULES}: {probe} {glob} {w}" for w in rule_loosenings(rule, new[probe][glob]))
        out.extend(shadowing(probe, old_rules, new[probe]))
    return out


def shadowing(probe: str, old_rules: Obj, new_rules: Obj) -> list[str]:
    # A new glob that an old one matches ("calls.x" under "calls.*") is more
    # specific, so it wins for those metrics: it may not be looser either.
    out: list[str] = []
    for glob in sorted(set(new_rules) - set(old_rules)):
        parent = measure.rule_for(old_rules, glob)
        if parent is None:
            continue
        out.extend(f"{RULES}: {probe} {glob} shadows a rule: {w}" for w in rule_loosenings(parent, new_rules[glob]))
    return out


# ── main ─────────────────────────────────────────────────────────────────────


def loosen_allowed() -> bool:
    if os.environ.get("GATES_LOOSEN") == "1":
        return True
    pr = os.environ.get("GATES_PR")
    if not pr:
        return False
    p = subprocess.run(
        ["gh", "pr", "view", pr, "--json", "labels", "--jq", ".labels[].name"],
        check=False,
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    return LABEL in p.stdout.split()


def default_base() -> str:
    return git("merge-base", "HEAD", "origin/main").stdout.strip()


def main(argv: list[str]) -> int:
    base = argv[argv.index("--base") + 1] if "--base" in argv else default_base()
    if not base:
        print("gates:baselines: no merge base with origin/main; nothing to compare", file=sys.stderr)
        return 0
    old_rules = json.loads(at_base(base, RULES) or "{}")
    new_rules = json.loads(in_tree(RULES) or "{}")
    found = check_history(base, old_rules) + check_floors(base) + check_rules(old_rules, new_rules)
    for f in found:
        print(f"LOOSENED {f}")
    if not found:
        short = git("rev-parse", "--short", base).stdout.strip() or base
        print(f"gates:baselines: no baseline loosened since {short}")
        return 0
    if loosen_allowed():
        print(f"gates:baselines: {len(found)} loosening(s) accepted ({LABEL})")
        return 0
    print(
        f"gates:baselines: {len(found)} loosening(s). Fix the cause, or a maintainer labels the PR {LABEL}.",
        file=sys.stderr,
    )
    return EXIT_LOOSENED


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
