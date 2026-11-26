#!/usr/bin/env python3
"""Fake gh for ci_health_test.sh: `gh api -X GET <path>` answered from $GH_WORLD.

Usage: copied to a directory first on PATH as `gh`, then called as
`gh api -X GET <path>` (only the last argument is read).

Appends each path to $GH_CALLS and prints the body $GH_WORLD (written by
ci_health_test_world.py) holds for it. Run lists are keyed with their
?branch= filter and must ask for status=completed; job and run lists are
paginated by per_page/page and carry total_count, like GitHub.
"""

import json
import os
import sys
from pathlib import Path
from urllib.parse import parse_qs


def main(path: str) -> None:
    with Path(os.environ["GH_CALLS"]).open("a") as f:
        f.write(path + "\n")
    base, _, query = path.partition("?")
    q = {k: v[0] for k, v in parse_qs(query).items()}
    world = json.loads(Path(os.environ["GH_WORLD"]).read_text())
    key = base + ("?branch=" + q.get("branch", "") if base.endswith("/runs") else "")
    if key not in world:
        sys.exit(f"fake gh: no {key}")
    body = world[key]
    for field in ("jobs", "workflow_runs"):
        if field in body:
            items = body[field]
            if field == "workflow_runs" and q.get("status") != "completed":
                sys.exit("fake gh: runs must be listed with status=completed")
            per, page = int(q.get("per_page", 30)), int(q.get("page", 1))
            body = {"total_count": len(items), field: items[(page - 1) * per : page * per]}
    print(json.dumps(body))


if __name__ == "__main__":
    main(sys.argv[-1])
