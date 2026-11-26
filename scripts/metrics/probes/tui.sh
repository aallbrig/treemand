#!/usr/bin/env bash
# Probe: TUI responsiveness on an aws-sized tree (cli/treemand/tui/bench_test.go).
#   metrics: frame_ms.<case> (best of 5 runs: the least load-affected) and
#            allocs.<case> per key press. One frame at 60 Hz is 16.7 ms.
set -euo pipefail
cd "$(dirname "$0")/../../../cli/treemand"
go test -run '^$' -bench . -benchmem -benchtime 100x -count 5 ./tui/ 2>/dev/null |
  python3 -I ../../scripts/metrics/tui_report.py
