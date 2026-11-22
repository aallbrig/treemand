#!/bin/sh
# Probe (network, about a minute): release health from the GitHub API and
# public URLs only (skill: treemand-release-health). Per recent release: lead
# time from merge to publish, expected assets, checksums against the published
# bytes, and the cosign bundle when cosign is installed. For the latest tag:
# the Homebrew tap, the treemand.dev mirror and install script, and the
# package-manager publish run; plus the Auto Release failure rate.
# GH_TOKEN (or GITHUB_TOKEN, or a logged-in gh) raises the API rate limit.
set -eu
cd "$(dirname "$0")/../../.."
if [ -z "${GH_TOKEN:-}${GITHUB_TOKEN:-}${RELEASE_HEALTH_ROUTES:-}" ] && command -v gh >/dev/null 2>&1; then
  GH_TOKEN="$(gh auth token 2>/dev/null || true)"
  export GH_TOKEN
fi
exec python3 -I scripts/metrics/release_health.py "$@"
