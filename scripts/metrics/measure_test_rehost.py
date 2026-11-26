"""Move a history file's first record to another host, for measure_test.sh.

Usage: python3 -I scripts/metrics/measure_test_rehost.py HISTORY_JSONL

Rewrites HISTORY_JSONL to hold only its first record, with "host" set to
"other-host", so the test can show host-bound metrics are not compared
across hosts.
"""

import json
import sys
from pathlib import Path


def main(history: Path) -> None:
    record = json.loads(history.read_text().splitlines()[0])
    record["host"] = "other-host"
    history.write_text(json.dumps(record) + "\n")


if __name__ == "__main__":
    main(Path(sys.argv[1]))
