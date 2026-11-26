"""Turn `go test -bench ./tui` output into the tui probe's JSON object.

    go test -run '^$' -bench . -benchmem -count 5 ./tui/ | python3 -I scripts/metrics/tui_report.py

Keeps the best (lowest) ns/op and allocs/op of each benchmark across runs, the
least load-affected sample, and reports frame_ms.<case> and allocs.<case>.
"""

from __future__ import annotations

import json
import re
import sys
from typing import Any, TextIO

BENCH = re.compile(r"^Benchmark(\S+?)-\d+\s+\d+\s+(\d+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op")
SOURCE = "go test -bench ./tui (bench_test.go), best of 5, 100 iterations each"


def report(lines: TextIO) -> dict[str, Any]:
    best: dict[str, tuple[int, int]] = {}
    for line in lines:
        m = BENCH.match(line)
        if not m:
            continue
        name, ns, allocs = m.group(1).lower(), int(m.group(2)), int(m.group(4))
        prev_ns, prev_allocs = best.get(name, (ns, allocs))
        best[name] = (min(prev_ns, ns), min(prev_allocs, allocs))
    metrics: dict[str, float] = {}
    for name, (ns, allocs) in best.items():
        metrics[f"frame_ms.{name}"] = round(ns / 1e6, 2)
        metrics[f"allocs.{name}"] = allocs
    return {"probe": "tui", "metrics": metrics, "details": [{"source": SOURCE}]}


if __name__ == "__main__":
    print(json.dumps(report(sys.stdin)))
