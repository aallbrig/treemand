#!/bin/sh
# Probe: site quality. Lighthouse category scores, axe-core accessibility
# violations by impact, CLS and transferred page weight for the home,
# installation and try pages at mobile and desktop sizes
# (scripts/metrics/site_quality.mjs).
#
# Target (scripts/site/serve_target.sh): SITE_BASE=https://treemand.dev/ for
# production, SITE_DIR=<dir> for a built site, or neither to build and serve
# the site locally. Metrics carry <target>.<page>.<form factor> as their
# subject (lh_performance.production.try.mobile).
#
#   SITE_QUALITY_RUNS=3         Lighthouse runs per page and form factor (median)
#   SITE_QUALITY_PAGES="name=/path/ ..."   pages to measure
#   SITE_QUALITY_LIGHTHOUSE=off            skip Lighthouse (axe, CLS, weight only)
#
# Lighthouse and axe-core are fetched from pinned npm versions at run time
# (network on first use); the site gains no dependency. Prints the probe JSON
# on stdout; progress goes to stderr.
set -eu
root="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$root"
probe_name=site-quality
# shellcheck source=scripts/site/serve_target.sh
. "$root/scripts/site/serve_target.sh"

BASE="$base" TARGET="$target" CHROME="$chrome" \
  RUNS="${SITE_QUALITY_RUNS:-3}" PAGES="${SITE_QUALITY_PAGES:-}" LIGHTHOUSE="${SITE_QUALITY_LIGHTHOUSE:-}" \
  node scripts/metrics/site_quality.mjs
