"""Shrink a `treemand --output=json` snapshot for the site's Try page, in place.

    python3 -I scripts/trim_web_tree.py TREE_JSON

Caps each node's help text at 1500 characters (the finder reads the first
2000, the help pane shows a screenful), drops discovery errors from nodes that
still have a description (non-fatal noise such as "Unknown help topic"), and
writes the result minified. Used by scripts/gen-web-trees.sh.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

HELP_CAP = 1500


def trim(node: dict[str, Any]) -> None:
    if node.get("help_text"):
        node["help_text"] = node["help_text"][:HELP_CAP]
    if node.get("discovery_err") and node.get("description"):
        node.pop("discovery_err")
    for child in node.get("children") or []:
        trim(child)


def main(argv: list[str]) -> int:
    if len(argv) != 1:
        sys.stderr.write("usage: trim_web_tree.py TREE_JSON\n")
        return 2
    path = Path(argv[0])
    tree = json.loads(path.read_text(encoding="utf-8"))
    trim(tree)
    path.write_text(json.dumps(tree, separators=(",", ":"), ensure_ascii=False), encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
