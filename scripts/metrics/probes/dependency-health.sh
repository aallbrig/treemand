#!/bin/sh
# Probe: dependency health (skill: treemand-dependency-health). govulncheck
# findings (reachable, imported but unreachable), direct Go modules behind
# their latest minor or patch or with no release in two years, the Go
# toolchain behind its latest stable patch, and CI and Taskfile tool pins
# behind their latest release. Few network calls, cached for the run; what
# needs the network is reported skipped when offline.
set -eu
cd "$(dirname "$0")/../../.."
exec python3 -I scripts/metrics/dependency_health.py "$@"
