#!/bin/sh
# Probe: test effectiveness (skill: treemand-test-effectiveness). Mutation
# score per core package (a stable sample of gremlins mutants, applied with
# go test -overlay), statement coverage per package, and tests whose result
# differs across go test -count=3. About ten minutes; weekly, not per PR.
set -eu
cd "$(dirname "$0")/../../.."
exec python3 -I scripts/metrics/test_effectiveness.py "$@"
