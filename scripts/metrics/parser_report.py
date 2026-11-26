r"""Score the parser corpus against its ground truth for the parser probe.

    go run ./tools/corpusdump CORPUS | python3 -I scripts/metrics/parser_report.py CORPUS

Reads corpusdump's "cli\tkind\tvalue" rows (kind: desc, sub or flag) on stdin,
checks each CLI against CORPUS/expect.json and prints the probe's JSON object.
"""

from __future__ import annotations

import json
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, TextIO


@dataclass
class Facts:
    desc: str = ""
    subs: set[str] = field(default_factory=set)
    flags: set[str] = field(default_factory=set)


def read_facts(rows: TextIO) -> dict[str, Facts]:
    facts: dict[str, Facts] = {}
    for line in rows:
        cli, kind, value = line.rstrip("\n").split("\t", 2)
        f = facts.setdefault(cli, Facts())
        if kind == "desc":
            f.desc = value
        elif kind == "sub":
            f.subs.add(value)
        elif kind == "flag":
            f.flags.add(value.split(" ")[0])
    return facts


def misses(f: Facts, e: dict[str, Any]) -> list[str]:
    """List what a CLI's parsed facts miss of its ground truth."""
    miss = [f"subcommand {s}" for s in e.get("subcommands_include", []) if s not in f.subs]
    if "max_subcommands" in e and len(f.subs) > e["max_subcommands"]:
        miss.append(f"{len(f.subs)} subcommands > {e['max_subcommands']}")
    miss += [f"flag {x}" for x in e.get("flags_include", []) if x not in f.flags]
    if e.get("description_include") and e["description_include"] not in f.desc:
        miss.append("description")
    if e.get("description_exclude") and e["description_exclude"] in f.desc:
        miss.append("garbage description")
    return miss


def report(facts: dict[str, Facts], expect: dict[str, dict[str, Any]]) -> dict[str, Any]:
    passed, gapped, unexpected = 0, 0, 0
    details: list[dict[str, Any]] = []
    for cli, f in sorted(facts.items()):
        e = expect.get(cli, {})
        miss = misses(f, e)
        if not miss:
            passed += 1
            continue
        if e.get("gaps"):
            gapped += 1
        else:
            unexpected += 1
        details.append({"cli": cli, "misses": miss, "gaps": e.get("gaps", [])})
    n = len(facts)
    return {
        "probe": "parser",
        "metrics": {
            "corpus_clis": n,
            "truth_pass_pct": round(100 * passed / max(n, 1), 1),
            "clis_with_gaps": gapped,
            "unexpected_misses": unexpected,
            "subcommands_total": sum(len(f.subs) for f in facts.values()),
            "flags_total": sum(len(f.flags) for f in facts.values()),
            "described_pct": round(100 * sum(bool(f.desc) for f in facts.values()) / max(n, 1), 1),
        },
        "details": details,
    }


def main(argv: list[str]) -> int:
    if len(argv) != 1:
        sys.stderr.write("usage: parser_report.py CORPUS <corpusdump-rows\n")
        return 2
    raw = json.loads((Path(argv[0]) / "expect.json").read_text(encoding="utf-8"))
    expect = {k: v for k, v in raw.items() if not k.startswith("_")}
    print(json.dumps(report(read_facts(sys.stdin), expect)))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
