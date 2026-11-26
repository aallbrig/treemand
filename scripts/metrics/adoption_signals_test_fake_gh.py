#!/usr/bin/env python3
"""Fake gh for adoption_signals_test.sh: `gh api -X GET <path>` answered from $GH_WORLD.

Usage: copied to a directory first on PATH as `gh`, then called as
`gh api -X GET <path>` (only the last argument is read).

Prints the body $GH_WORLD (written by adoption_signals_test_world.py) holds
for the path, keyed without per_page/page. List bodies are paginated by
per_page/page; an unknown path is a 404, and an {"__error": ...} body exits
with that message, like gh on a forbidden API.
"""

import json
import os
import sys
from pathlib import Path
from urllib.parse import parse_qs, urlencode


def main(path: str) -> None:
    base, _, query = path.partition("?")
    q = {k: v[0] for k, v in parse_qs(query).items()}
    per, page = int(q.pop("per_page", 30)), int(q.pop("page", 1))
    key = base + ("?" + urlencode(q) if q else "")
    world = json.loads(Path(os.environ["GH_WORLD"]).read_text())
    if key not in world:
        sys.exit(f"gh: Not Found (HTTP 404) {key}")
    body = world[key]
    if isinstance(body, dict) and "__error" in body:
        sys.exit(body["__error"])
    if isinstance(body, list):
        body = body[(page - 1) * per : page * per]
    print(json.dumps(body))


if __name__ == "__main__":
    main(sys.argv[-1])
