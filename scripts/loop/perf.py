#!/usr/bin/env python3
"""Track treemand performance, reliability and discovery quality over releases.

  perf.py measure [--bin PATH] [--version V]   print rows for the corpus (TSV, no file changes)
  perf.py record  [--bin PATH] [--version V]   measure and append to docs/perf/history.tsv
  perf.py compare [--bin PATH]                 measure and compare with the latest recorded
                                               version on this host; exit 1 on regression
  perf.py show                                 trend per CLI on this host from the history

Timings depend on the machine, so history rows carry the host and comparisons only
use rows from the same host. Regression = cold >30% and >1s slower, warm >50% and
>100ms slower, memory >30% and >20MB higher, a new failure, nodes down >10%, or
descriptionless/flagless nodes up >25% (and by >15; discovery output varies a little between runs).
"""

import json
import os
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HISTORY = ROOT / "docs/perf/history.tsv"
COLS = ["date", "version", "host", "cli", "cold_s", "warm_s", "rss_mb", "nodes", "no_desc", "no_flags", "rc"]
CORPUS = ["git", "docker", "kubectl", "gh", "npm", "pip", "go", "helm", "apt", "systemctl", "tar", "aws"]


def run(bin_path: str, cli: str, env: dict[str, str], cache: bool) -> tuple[int, float, float, str]:
    args = ["/usr/bin/time", "-f", "%M", bin_path, "--output=json", "--timeout=60"]
    if not cache:
        args.append("--no-cache")
    args.append(cli)
    start = time.monotonic()
    p = subprocess.run(args, capture_output=True, text=True, env=env, timeout=180)
    secs = time.monotonic() - start
    rss_kb = 0.0
    m = re.findall(r"^(\d+)\s*$", p.stderr, re.M)
    if m:
        rss_kb = float(m[-1])
    return p.returncode, secs, rss_kb / 1024, p.stdout


def count(tree: dict) -> tuple[int, int, int]:
    nodes = no_desc = no_flags = 0
    stack = [tree]
    while stack:
        n = stack.pop()
        nodes += 1
        if not (n.get("description") or ""):
            no_desc += 1
        if not (n.get("flags") or []):
            no_flags += 1
        stack.extend(n.get("children") or [])
    return nodes, no_desc, no_flags


def measure_one(bin_path: str, cli: str, version: str) -> dict[str, str]:
    cache_dir = tempfile.mkdtemp(prefix="tm-perf-")
    env = dict(os.environ, TREEMAND_CACHE_DIR=cache_dir)
    try:
        rc, cold, rss, out = run(bin_path, cli, env, cache=False)
        nodes = no_desc = no_flags = 0
        if rc == 0:
            try:
                nodes, no_desc, no_flags = count(json.loads(out))
            except json.JSONDecodeError:
                rc = 99
        warm = 0.0
        if rc == 0:
            run(bin_path, cli, env, cache=True)  # populate
            _, warm, _, _ = run(bin_path, cli, env, cache=True)
    except subprocess.TimeoutExpired:
        rc, cold, warm, rss, nodes, no_desc, no_flags = 124, 180.0, 0.0, 0.0, 0, 0, 0
    finally:
        shutil.rmtree(cache_dir, ignore_errors=True)
    return {
        "date": time.strftime("%Y-%m-%d", time.gmtime()), "version": version, "host": platform.node(), "cli": cli,
        "cold_s": f"{cold:.2f}", "warm_s": f"{warm:.2f}", "rss_mb": f"{rss:.0f}",
        "nodes": str(nodes), "no_desc": str(no_desc), "no_flags": str(no_flags), "rc": str(rc),
    }


def measure(bin_path: str, version: str) -> list[dict[str, str]]:
    return [measure_one(bin_path, cli, version) for cli in CORPUS if shutil.which(cli)]


def best_of(a: dict[str, str], b: dict[str, str]) -> dict[str, str]:
    """Keep the better of two measurements, so one noisy run cannot fail the gate."""
    if a["rc"] != "0" and b["rc"] == "0":
        return b
    if b["rc"] != "0":
        return a
    out = dict(a)
    for k in ("cold_s", "warm_s", "rss_mb", "no_desc", "no_flags"):
        out[k] = min(a[k], b[k], key=float)
    out["nodes"] = max(a["nodes"], b["nodes"], key=float)
    return out


def load_history() -> list[dict[str, str]]:
    if not HISTORY.exists():
        return []
    lines = HISTORY.read_text().splitlines()
    return [dict(zip(COLS, line.split("\t"))) for line in lines[1:] if line.strip()]


def vkey(v: str) -> tuple[int, ...]:
    return tuple(int(x) for x in re.findall(r"\d+", v)[:3]) or (0,)


def compare(rows: list[dict[str, str]]) -> list[str]:
    hist = [h for h in load_history() if h["host"] == platform.node()]
    if not hist:
        return []
    prev_ver = max((h["version"] for h in hist), key=vkey)
    prev = {h["cli"]: h for h in hist if h["version"] == prev_ver}
    bad = []
    for r in rows:
        p = prev.get(r["cli"])
        if not p:
            continue
        f = lambda d, k: float(d[k])  # noqa: E731
        if r["rc"] != "0" and p["rc"] == "0":
            bad.append(f"{r['cli']}: now fails rc={r['rc']} (was ok in {prev_ver})")
            continue
        if f(r, "cold_s") > f(p, "cold_s") * 1.3 and f(r, "cold_s") - f(p, "cold_s") > 1.0:
            bad.append(f"{r['cli']}: cold {p['cold_s']}s -> {r['cold_s']}s vs {prev_ver}")
        if f(r, "warm_s") > f(p, "warm_s") * 1.5 and f(r, "warm_s") - f(p, "warm_s") > 0.1:
            bad.append(f"{r['cli']}: warm {p['warm_s']}s -> {r['warm_s']}s vs {prev_ver}")
        if f(r, "rss_mb") > f(p, "rss_mb") * 1.3 and f(r, "rss_mb") - f(p, "rss_mb") > 20:
            bad.append(f"{r['cli']}: memory {p['rss_mb']}MB -> {r['rss_mb']}MB vs {prev_ver}")
        if f(p, "nodes") and f(r, "nodes") < f(p, "nodes") * 0.9:
            bad.append(f"{r['cli']}: nodes {p['nodes']} -> {r['nodes']} vs {prev_ver}")
        for k in ("no_desc", "no_flags"):
            if f(r, k) > f(p, k) * 1.25 and f(r, k) - f(p, k) > 15:
                bad.append(f"{r['cli']}: {k} {p[k]} -> {r[k]} vs {prev_ver}")
    return bad


def emit(rows: list[dict[str, str]]) -> None:
    print("\t".join(COLS))
    for r in rows:
        print("\t".join(r[c] for c in COLS))


def default_bin() -> str:
    out = ROOT / ".loop/treemand-perf"
    subprocess.run(["go", "build", "-o", str(out), "./cli/treemand"], cwd=ROOT, check=True)
    return str(out)


def default_version() -> str:
    p = subprocess.run(["git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*"], cwd=ROOT, capture_output=True, text=True)
    sha = subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, capture_output=True, text=True).stdout.strip()
    return f"{p.stdout.strip() or 'v0.0.0'}+{sha}"


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print(__doc__)
        return 2
    cmd = argv[1]
    opts = dict(zip(argv[2::2], argv[3::2]))
    if cmd == "show":
        hist = [h for h in load_history() if h["host"] == platform.node()]
        for cli in sorted({h["cli"] for h in hist}):
            print(f"== {cli}")
            for h in sorted((x for x in hist if x["cli"] == cli), key=lambda x: vkey(x["version"])):
                print(f"  {h['version']:<16} cold {h['cold_s']:>6}s  warm {h['warm_s']:>5}s  {h['rss_mb']:>5}MB  nodes {h['nodes']:>4}  no_desc {h['no_desc']:>4}  no_flags {h['no_flags']:>4}  rc {h['rc']}")
        return 0
    bin_path = opts.get("--bin") or default_bin()
    version = opts.get("--version") or default_version()
    rows = measure(bin_path, version)
    if cmd == "measure":
        emit(rows)
        return 0
    if cmd == "record":
        HISTORY.parent.mkdir(parents=True, exist_ok=True)
        new = not HISTORY.exists()
        with HISTORY.open("a") as fh:
            if new:
                fh.write("\t".join(COLS) + "\n")
            for r in rows:
                fh.write("\t".join(r[c] for c in COLS) + "\n")
        emit(rows)
        return 0
    if cmd == "compare":
        bad = compare(rows)
        # A regression must reproduce: re-measure each flagged CLI once and keep the better run.
        flagged = {b.split(":")[0] for b in bad}
        if flagged:
            rows = [best_of(r, measure_one(bin_path, r["cli"], version)) if r["cli"] in flagged else r for r in rows]
            bad = compare(rows)
        emit(rows)
        for b in bad:
            print("REGRESSION", b, file=sys.stderr)
        return 1 if bad else 0
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
