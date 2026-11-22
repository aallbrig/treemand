#!/bin/sh
# Probe: GitHub Actions health from the Actions API (skill: treemand-ci-health).
# Success on main per workflow, CI duration p50/p90, flaky rate, slowest CI
# job, queue p90 and startup failures over the last 50 runs per workflow.
# Needs `gh` authenticated with actions read access; jobs are cached per run.
set -eu
cd "$(dirname "$0")/../../.."
exec python3 -I scripts/metrics/ci_health.py "$@"
