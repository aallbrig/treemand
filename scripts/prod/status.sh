#!/bin/sh
# status.sh: every prod window pane once (site, pipelines, aws, downloads).
# Runs them all, then exits 1 if any reported a failed check.
here="$(dirname "$0")"
rc=0
for pane in site pipelines aws downloads; do
  sh "$here/$pane.sh" --once || rc=1
  echo
done
exit "$rc"
