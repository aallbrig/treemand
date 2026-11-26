"""Write the pull request the fake gh in notes_test.sh says merged a commit.

Usage: python3 -I scripts/release/notes_test_pull.py OUT NUMBER LABELS TITLE BODY

Writes OUT as the JSON list `gh api repos/x/y/commits/<sha>/pulls` returns:
one merged pull request with NUMBER, TITLE, BODY and the comma-separated
LABELS (empty for none).
"""

import json
import sys
from pathlib import Path


def main(out: str, number: str, labels: str, title: str, body: str) -> None:
    names = [n for n in labels.split(",") if n]
    pull = {
        "number": int(number),
        "title": title,
        "body": body,
        "merged_at": "2026-11-22T10:00:00Z",
        "labels": [{"name": n} for n in names],
    }
    Path(out).write_text(json.dumps([pull]))


if __name__ == "__main__":
    main(*sys.argv[1:])
