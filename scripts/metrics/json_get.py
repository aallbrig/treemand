"""Print a Python expression evaluated over a JSON file, for the probe tests.

Usage: python3 -I scripts/metrics/json_get.py JSON_FILE EXPRESSION

Loads JSON_FILE as `o` (and its "details" list, when it has one, as `d`),
evaluates EXPRESSION over them and prints the value with print(), or prints
"absent" when the expression raises KeyError, IndexError, StopIteration or
ValueError. The *_test.sh probe tests compare that line with the value they
expect. EXPRESSION is test code from the calling script, never input data.
"""

import json
import sys
from pathlib import Path


def main(json_file: Path, expression: str) -> None:
    o = json.loads(json_file.read_text())
    scope = {"o": o, "d": o.get("details") if isinstance(o, dict) else None}
    try:
        print(eval(expression, scope))  # noqa: S307 - the expression is the calling test's own code
    except (KeyError, IndexError, StopIteration, ValueError):
        print("absent")


if __name__ == "__main__":
    main(Path(sys.argv[1]), sys.argv[2])
