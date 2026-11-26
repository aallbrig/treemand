r"""Turn the install probe's path rows into its JSON object.

    python3 -I scripts/metrics/install_report.py ROWS SKIPS LATEST PREVIOUS

ROWS holds tab-separated rows from scripts/metrics/install_paths.sh (one per
install path per environment); SKIPS holds "<env>\t<why>" rows for
environments that never ran. Prints the probe's JSON object on stdout.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

COLS = [
    "env",
    "path",
    "status",
    "exit",
    "version",
    "want",
    "renders",
    "quarantine",
    "quarantine_runs",
    "path_hint",
    "signature",
    "note",
]


def report(rows_file: Path, skips_file: Path, latest: str, previous: str) -> dict[str, Any]:
    details: list[dict[str, str]] = []
    metrics: dict[str, int] = {"paths_ok": 0, "paths_failed": 0}
    skipped: dict[str, str] = {}
    for line in rows_file.read_text(encoding="utf-8").splitlines():
        parts = line.split("\t")
        if len(parts) != len(COLS):
            continue
        row = dict(zip(COLS, parts, strict=True))
        name = f"{row.pop('env')}.{row.pop('path')}"
        if row["status"] == "skip":
            skipped[name] = row["note"]
            continue
        ok = row["status"] == "ok"
        metrics["paths_ok" if ok else "paths_failed"] += 1
        metrics[f"ok.{name}"] = int(ok)
        details.append({"path": name, **row})
    for line in skips_file.read_text(encoding="utf-8").splitlines():
        name, _, why = line.partition("\t")
        skipped[name] = why
    return {
        "probe": "install",
        "metrics": metrics,
        "skipped": skipped,
        "details": [{"latest": latest, "previous": previous}, *details],
    }


def main(argv: list[str]) -> int:
    if len(argv) != 4:  # noqa: PLR2004 - ROWS SKIPS LATEST PREVIOUS
        sys.stderr.write("usage: install_report.py ROWS SKIPS LATEST PREVIOUS\n")
        return 2
    print(json.dumps(report(Path(argv[0]), Path(argv[1]), argv[2], argv[3])))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
