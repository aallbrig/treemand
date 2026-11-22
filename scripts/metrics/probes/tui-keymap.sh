#!/bin/sh
# Probe: keys the TUI binds against keys it documents (cli/treemand/tools/keymapprobe):
# shadowed bindings (conflicts), bindings bubbletea never reports, bound keys
# missing from the ? overlay, the website reference or a dialog's hint, and
# documented keys nothing binds. Static: reads source and docs, runs no TUI.
set -eu
cd "$(dirname "$0")/../../../cli/treemand"
go run ./tools/keymapprobe tui ../../www/treemand/content
