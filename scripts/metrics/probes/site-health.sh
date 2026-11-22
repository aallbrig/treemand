#!/bin/sh
# Probe: site runtime health. Crawls every same-origin page reachable from the
# sitemap and from links in headless Chrome (scripts/metrics/site_health.mjs)
# and counts HTTP errors, console errors, uncaught exceptions, broken internal
# and external links, and mixed content.
#
# Target (scripts/site/serve_target.sh): SITE_BASE=https://treemand.dev/ for
# production, SITE_DIR=<dir> for a built site, or neither to build and serve
# the site locally. Metrics carry the target as their subject (pages.local,
# pages.production). Prints the probe JSON on stdout; progress goes to stderr.
set -eu
root="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$root"
probe_name=site-health
# shellcheck source=scripts/site/serve_target.sh
. "$root/scripts/site/serve_target.sh"

BASE="$base" TARGET="$target" CHROME="$chrome" node scripts/metrics/site_health.mjs
