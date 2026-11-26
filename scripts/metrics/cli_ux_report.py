r"""Turn the cli-ux probe's check tables into its JSON object.

    python3 -I scripts/metrics/cli_ux_report.py WORK UNDESCRIBED

WORK holds what scripts/metrics/probes/cli-ux.sh wrote: checks.tsv
("check\tpass\tevidence"), skipped.tsv ("check\twhy") and subcommands.tsv
("path\tdescribed"). UNDESCRIBED is the count of subcommands without a
description. Prints the probe's JSON object on stdout.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any


def rows(work: Path, name: str) -> list[list[str]]:
    text = (work / name).read_text(encoding="utf-8")
    return [line.split("\t") for line in text.splitlines() if line]


def report(work: Path, undescribed: int) -> dict[str, Any]:
    checks: list[dict[str, Any]] = [
        {"check": n, "pass": p == "1", "evidence": e} for n, p, e in rows(work, "checks.tsv")
    ]
    metrics: dict[str, int] = {f"check.{c['check']}": int(c["pass"]) for c in checks}
    metrics["failing_checks"] = sum(not c["pass"] for c in checks)
    metrics["passing_checks"] = sum(bool(c["pass"]) for c in checks)
    metrics["undescribed_subcommands"] = undescribed
    subcommands = [{"path": p, "described": d == "1"} for p, d in rows(work, "subcommands.tsv")]
    return {
        "probe": "cli-ux",
        "metrics": metrics,
        "skipped": {k: v for k, v in rows(work, "skipped.tsv")},  # noqa: C416 - rows are lists, not pairs
        "details": [
            {"failing": [c for c in checks if not c["pass"]]},
            {"subcommands": subcommands},
            {"checks": checks},
        ],
    }


def main(argv: list[str]) -> int:
    if len(argv) != 2:  # noqa: PLR2004 - WORK UNDESCRIBED
        sys.stderr.write("usage: cli_ux_report.py WORK UNDESCRIBED\n")
        return 2
    print(json.dumps(report(Path(argv[0]), int(argv[1]))))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
