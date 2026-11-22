#!/bin/sh
# Probe: a time-boxed native Go fuzz campaign over every Fuzz* target in
# cli/treemand (skill: treemand-fuzz-campaign). FUZZ_SECONDS per target
# (default 60); the generated corpus persists in $GOCACHE/fuzz, crashers are
# left in <package>/testdata/fuzz/<target>/ for the fix to commit.
set -eu
cd "$(dirname "$0")/../../.."
exec python3 -I scripts/metrics/fuzz_campaign.py "$@"
