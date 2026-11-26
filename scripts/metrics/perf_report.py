"""Helpers for the perf probe (scripts/metrics/probes/perf.sh).

    python3 -I scripts/metrics/perf_report.py nodes TREE_JSON
    python3 -I scripts/metrics/perf_report.py report ROWS SKIPPED

`nodes` prints how many nodes a `treemand --output=json` tree holds, or 0 when
the file is missing or not a tree. `report` turns the probe's rows (one line
per CLI: "cli cold cpu rss_kb warm nodes tti|- calls run,run,...") and its
space-separated skipped CLIs into the probe's JSON object on stdout.
"""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path
from typing import Any

USAGE = "usage: perf_report.py nodes TREE_JSON | report ROWS SKIPPED"


def count_nodes(tree: Any) -> int:  # noqa: ANN401 - decoded JSON
    """Count a tree node and its descendants; anything not a node counts 0."""
    if not isinstance(tree, dict):
        return 0
    return 1 + sum(count_nodes(c) for c in tree.get("children") or [])


def nodes(path: str) -> int:
    try:
        return count_nodes(json.loads(Path(path).read_text(encoding="utf-8")))
    except (OSError, ValueError, RecursionError):
        return 0


def report(rows: str, skipped: str) -> dict[str, Any]:
    """Build the probe's JSON object from its rows and skipped CLIs."""
    metrics: dict[str, float] = {}
    per_cli: list[dict[str, Any]] = []
    for line in rows.splitlines():
        if not line.strip():
            continue
        cli, cold, cpu, rss, warm, node_count, tti, calls, spread = line.split()
        metrics[f"help_calls.{cli}"] = int(calls)
        metrics[f"cold_s.{cli}"] = float(cold)
        metrics[f"warm_s.{cli}"] = float(warm)
        metrics[f"cpu_s.{cli}"] = round(float(cpu), 2)
        metrics[f"rss_mb.{cli}"] = round(int(rss) / 1024, 1)
        metrics[f"nodes.{cli}"] = int(node_count)
        if tti != "-":
            metrics[f"tti_s.{cli}"] = float(tti)
        per_cli.append(
            {
                "cli": cli,
                "cold_s": float(cold),
                "cpu_s": round(float(cpu), 2),
                "cpu_per_wall": round(float(cpu) / max(float(cold), 0.01), 1),
                "cold_runs": [float(x) for x in spread.strip(",").split(",") if x],
            }
        )
    loadavg = " ".join(f"{x:.2f}" for x in os.getloadavg())
    return {
        "probe": "perf",
        "metrics": metrics,
        "skipped": dict.fromkeys(skipped.split(), "not installed"),
        "details": [{"loadavg": loadavg}, {"per_cli": per_cli}],
    }


def main(argv: list[str]) -> int:
    if len(argv) == 2 and argv[0] == "nodes":  # noqa: PLR2004 - nodes FILE
        print(nodes(argv[1]))
        return 0
    if len(argv) == 3 and argv[0] == "report":  # noqa: PLR2004 - report ROWS SKIPPED
        print(json.dumps(report(argv[1], argv[2])))
        return 0
    sys.stderr.write(USAGE + "\n")
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
