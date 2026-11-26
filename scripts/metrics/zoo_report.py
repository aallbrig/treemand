r"""Turn the cli-zoo container's sweep and smoke tables into the zoo probe's JSON.

    python3 -I scripts/metrics/zoo_report.py WORK

WORK holds sweep.tsv ("program\texit\tsecs\tpanicked" rows) and smoke.tsv (a
header, then "cli\texit\tsecs\tnodes\tflags" rows), as written by
scripts/metrics/probes/zoo.sh. Prints the probe's JSON object on stdout.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

SWEEP_COLUMNS = 4
TIMEOUT_EXIT = "124"
MAX_ERRORS_LISTED = 40


def report(work: Path) -> dict[str, Any]:
    lines = (work / "sweep.tsv").read_text(encoding="utf-8").splitlines()
    rows = [line.split("\t") for line in lines if line.count("\t") == SWEEP_COLUMNS - 1]
    errors = [r for r in rows if r[1] != "0"]
    timeouts = [r for r in rows if r[1] == TIMEOUT_EXIT]
    panics = [r for r in rows if r[3] == "1"]
    metrics: dict[str, float] = {
        "sweep_programs": len(rows),
        "sweep_errors": len(errors),
        "sweep_timeouts": len(timeouts),
        "sweep_panics": len(panics),
        "sweep_error_pct": round(100 * len(errors) / max(len(rows), 1), 1),
    }
    for line in (work / "smoke.tsv").read_text(encoding="utf-8").splitlines()[1:]:
        cli, _rc, _secs, nodes, flags = line.split("\t")
        metrics[f"nodes.{cli}"] = int(nodes)
        metrics[f"flags.{cli}"] = int(flags)
    return {
        "probe": "zoo",
        "metrics": metrics,
        "details": [
            {"errors": sorted(f"{r[0]} (exit {r[1]})" for r in errors)[:MAX_ERRORS_LISTED]},
            {"panics": sorted(r[0] for r in panics)},
            {"timeouts": sorted(r[0] for r in timeouts)},
        ],
    }


def main(argv: list[str]) -> int:
    if len(argv) != 1:
        sys.stderr.write("usage: zoo_report.py WORK\n")
        return 2
    print(json.dumps(report(Path(argv[0]))))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
