r"""Turn the complexity probe's tool outputs into its JSON object.

    python3 -I scripts/metrics/complexity_report.py WORK

WORK holds what scripts/metrics/probes/complexity.sh collected: cog.json
(gocognit), nestif.json (nestif), dupl.txt (dupl), churn.tsv and loc.tsv
("<lines>\t<path>" rows). Prints the probe's JSON object on stdout.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

LONG_FILE = 800
OVER_15 = 15
OVER_30 = 30
TOP = 10


def tsv(path: Path) -> dict[str, int]:
    rows = (line.split("\t") for line in path.read_text(encoding="utf-8").splitlines() if line.strip())
    return {p: int(n) for n, p in rows}


def load_list(path: Path) -> list[dict[str, Any]]:
    """Read a JSON array; the tools print null (or nothing) when they find nothing."""
    data: list[dict[str, Any]] | None = json.loads(path.read_text(encoding="utf-8") or "null")
    return data or []


def clone_groups(dupl: Path) -> int:
    """Count clone groups of dupl's output.

    dupl prints each fragment of a pair ("a:1-9: duplicate of b:3-11"); fragments
    linked by a pair form one group.
    """
    parent: dict[str, str] = {}

    def find(x: str) -> str:
        parent.setdefault(x, x)
        while parent[x] != x:
            parent[x] = parent[parent[x]]
            x = parent[x]
        return x

    for line in dupl.read_text(encoding="utf-8").splitlines():
        a, sep, b = line.strip().partition(": duplicate of ")
        if sep:
            parent[find(a)] = find(b)
    return len({find(x) for x in list(parent)})


def at(item: dict[str, Any]) -> str:
    return f"{item['Pos']['Filename']}:{item['Pos']['Line']}"


def report(work: Path) -> dict[str, Any]:
    all_funcs = load_list(work / "cog.json")
    funcs = [f for f in all_funcs if not f["Pos"]["Filename"].endswith("_test.go")]
    churn, loc = tsv(work / "churn.tsv"), tsv(work / "loc.tsv")
    nests = load_list(work / "nestif.json")
    long_files = {p: n for p, n in loc.items() if n > LONG_FILE}
    cx = [f["Complexity"] for f in funcs]
    per_file: dict[str, int] = {}
    for f in funcs:
        path = f["Pos"]["Filename"]
        per_file[path] = max(per_file.get(path, 0), f["Complexity"])
    top = sorted(funcs, key=lambda f: -f["Complexity"])[:TOP]
    hot = sorted(((churn.get(p, 0) * c, p, churn.get(p, 0), c) for p, c in per_file.items()), reverse=True)[:TOP]
    avg = round(sum(f["Complexity"] for f in all_funcs) / len(all_funcs), 2) if all_funcs else 0
    return {
        "probe": "complexity",
        "metrics": {
            "cognitive_max": max(cx, default=0),
            "cognitive_avg": avg,
            "functions_over_15": sum(c > OVER_15 for c in cx),
            "functions_over_30": sum(c > OVER_30 for c in cx),
            "go_loc": sum(loc.values()),
            "files_over_800_lines": len(long_files),
            "dupl_clones": clone_groups(work / "dupl.txt"),
            "nestif_over_3": len(nests),
            **{f"file_lines.{p}": n for p, n in sorted(long_files.items())},
        },
        "details": [
            {
                "top_functions": [
                    {"complexity": f["Complexity"], "func": f["FuncName"], "pkg": f["PkgName"], "at": at(f)}
                    for f in top
                ]
            },
            {
                "hotspots_churn_x_complexity": [
                    {"score": s, "file": p, "churn_lines_90d": ch, "max_complexity": c} for s, p, ch, c in hot if s > 0
                ]
            },
            {"files_over_800_lines": sorted(long_files)},
            {"nestif_over_3": [{"at": at(n), "complexity": n["Complexity"], "message": n["Message"]} for n in nests]},
        ],
    }


def main(argv: list[str]) -> int:
    if len(argv) != 1:
        sys.stderr.write("usage: complexity_report.py WORK\n")
        return 2
    print(json.dumps(report(Path(argv[0]))))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
