"""Compare two `treemand --output=json` trees of the same CLI.

    python3 -I scripts/compare_trees.py OLD_JSON NEW_JSON

Prints one summary line (node counts, lost nodes, lost and gained flags,
changed descriptions) and up to five examples of each. Used by
scripts/real_cli_ab.sh; exits non-zero when either tree cannot be read.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

EXAMPLES = 5
DESC_CHARS = 50

Facts = tuple[str, set[str]]


def load(path: str) -> dict[str, Facts]:
    """Map each command path ("git remote add") to its description and flag names."""
    out: dict[str, Facts] = {}

    def walk(node: dict[str, Any], parents: list[str]) -> None:
        here = [*parents, node["name"]]
        out[" ".join(here)] = (node.get("description", ""), {f["name"] for f in node.get("flags") or []})
        for child in node.get("children") or []:
            walk(child, here)

    walk(json.loads(Path(path).read_text(encoding="utf-8")), [])
    return out


def compare(a: dict[str, Facts], b: dict[str, Facts]) -> list[str]:
    none: Facts = ("", set())
    lost_nodes = sorted(set(a) - set(b))
    lost_flags = [(k, f) for k in a for f in a[k][1] - b.get(k, none)[1]]
    new_flags = [(k, f) for k in b for f in b[k][1] - a.get(k, none)[1]]
    desc = [(k, a[k][0], b[k][0]) for k in a if k in b and a[k][0] != b[k][0]]
    summary = (
        f"nodes {len(a)}->{len(b)}  lost nodes {len(lost_nodes)}  lost flags {len(lost_flags)}  "
        f"gained flags {len(new_flags)}  changed descriptions {len(desc)}"
    )
    lines = [summary]
    lines += [f"    - node {k}" for k in lost_nodes[:EXAMPLES]]
    lines += [f"    - flag {k}: {f}" for k, f in lost_flags[:EXAMPLES]]
    lines += [f"    + flag {k}: {f}" for k, f in new_flags[:EXAMPLES]]
    lines += [f"    ~ desc {k}: {o[:DESC_CHARS]!r} -> {n[:DESC_CHARS]!r}" for k, o, n in desc[:EXAMPLES]]
    return lines


def main(argv: list[str]) -> int:
    if len(argv) != 2:  # noqa: PLR2004 - OLD NEW
        sys.stderr.write("usage: compare_trees.py OLD_JSON NEW_JSON\n")
        return 2
    print("\n".join(compare(load(argv[0]), load(argv[1]))))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
