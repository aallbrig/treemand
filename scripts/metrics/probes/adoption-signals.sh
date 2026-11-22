#!/bin/sh
# Probe (network, a few seconds): adoption trend signals from free sources
# (skill: treemand-adoption-signals). Stars, forks, watchers; release
# downloads overall and for the latest release; GitHub traffic and the
# Homebrew tap's clones (skipped without push access); the Go module proxy's
# latest version; open issues by people outside the project (a count).
# Needs `gh` authenticated; GH_REPO overrides the repository.
set -eu
cd "$(dirname "$0")/../../.."
exec python3 -I scripts/metrics/adoption_signals.py "$@"
