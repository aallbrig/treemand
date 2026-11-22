#!/bin/sh
# Probe: WCAG 2.x contrast of every foreground/background pair the TUI renders
# (cli/treemand/tools/tuicontrast), on a dark and a light terminal, plus
# NO_COLOR compliance (environment variable and --no-color / config).
set -eu
cd "$(dirname "$0")/../../../cli/treemand"
go run ./tools/tuicontrast
