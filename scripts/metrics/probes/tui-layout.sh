#!/bin/sh
# Probe: TUI layout at 80x24, 120x40 and 200x50 across six states
# (cli/treemand/tools/tuiprobe): overflowing lines, frames taller than the
# terminal, truncated status-bar hints, and blank space in the first frame.
set -eu
cd "$(dirname "$0")/../../../cli/treemand"
go run ./tools/tuiprobe
