#!/usr/bin/env bash
# Probe: TUI responsiveness on an aws-sized tree (cli/treemand/tui/bench_test.go).
#   metrics: frame_ms.<case> (best of 5 runs: the least load-affected) and
#            allocs.<case> per key press. One frame at 60 Hz is 16.7 ms.
set -euo pipefail
cd "$(dirname "$0")/../../../cli/treemand"
go test -run '^$' -bench . -benchmem -benchtime 100x -count 5 ./tui/ 2>/dev/null |
  python3 -I -c '
import json, re, sys
best = {}
for line in sys.stdin:
    m = re.match(r"^Benchmark(\S+?)-\d+\s+\d+\s+(\d+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op", line)
    if not m:
        continue
    name, ns, allocs = m.group(1).lower(), int(m.group(2)), int(m.group(4))
    b = best.setdefault(name, [ns, allocs])
    b[0], b[1] = min(b[0], ns), min(b[1], allocs)
metrics = {}
for name, (ns, allocs) in best.items():
    metrics[f"frame_ms.{name}"] = round(ns / 1e6, 2)
    metrics[f"allocs.{name}"] = allocs
print(json.dumps({"probe": "tui", "metrics": metrics,
                  "details": [{"source": "go test -bench ./tui (bench_test.go), best of 5, 100 iterations each"}]}))
'
