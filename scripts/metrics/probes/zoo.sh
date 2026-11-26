#!/bin/sh
# Probe (heavy, weekly): treemand across the cli-zoo container. Builds the
# image from this tree, then:
#   sweep: every executable on the container's PATH, one level deep
#          -> sweep_programs, sweep_errors (non-zero exit), sweep_timeouts,
#             sweep_panics, sweep_error_pct
#   smoke: full discovery of each zoo CLI -> nodes.<cli>, flags.<cli>
# Real programs only ever run inside the disposable container.
set -eu
cd "$(dirname "$0")/../../.."
docker build -q -t treemand-cli-zoo -f test/cli-zoo/Dockerfile . >/dev/null
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
docker run --rm treemand-cli-zoo sweep >"$work/sweep.tsv" 2>/dev/null || true
docker run --rm treemand-cli-zoo smoke >"$work/smoke.tsv" 2>/dev/null || true
python3 -I scripts/metrics/zoo_report.py "$work"
