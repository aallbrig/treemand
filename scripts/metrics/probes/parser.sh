#!/usr/bin/env bash
# Probe: help-parser quality on the corpus (no real CLIs run).
#   metrics: corpus_clis, truth_pass_pct (CLIs meeting expect.json ground
#            truth), clis_with_gaps (known, issue-tracked), unexpected_misses
#            (failures with no gap marker: a regression), subcommands_total,
#            flags_total, described_pct.
#   details: every CLI that misses its ground truth, with what it misses.
set -euo pipefail
cd "$(dirname "$0")/../../../cli/treemand"
corpus=discovery/testdata/corpus
go run ./tools/corpusdump "$corpus" | python3 -I ../../scripts/metrics/parser_report.py "$corpus"
