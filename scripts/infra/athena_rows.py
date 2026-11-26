"""Turn Athena's flat cell list into tab-separated rows.

    aws athena get-query-results ... --query 'ResultSet.Rows[].Data[].VarCharValue' --output json |
        python3 -I scripts/infra/athena_rows.py COLUMNS

Reads the JSON array of cell values on stdin (null for an empty cell) and
prints COLUMNS cells per line, tab-separated. Used by metrics_downloads.sh.
"""

from __future__ import annotations

import json
import sys


def rows(cells: list[str | None], cols: int) -> list[str]:
    return ["\t".join(x or "" for x in cells[i : i + cols]) for i in range(0, len(cells), cols)]


def main(argv: list[str]) -> int:
    if len(argv) != 1 or not argv[0].isdigit() or int(argv[0]) < 1:
        sys.stderr.write("usage: athena_rows.py COLUMNS <cells.json\n")
        return 2
    for line in rows(json.load(sys.stdin), int(argv[0])):
        print(line)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
