#!/bin/sh
# Fail when a community-health file GitHub's checklist counts is missing, or
# when Dependabot or the release-notes config points at something that does
# not exist in this repo. Hermetic: reads only the checkout.
#   sh scripts/community_check.sh [repo-root]
set -eu

root=${1:-$(git rev-parse --show-toplevel)}
fail=0
err() { echo "community_check: $*" >&2; fail=1; }

for f in README.md LICENSE CONTRIBUTING.md CODE_OF_CONDUCT.md SECURITY.md \
  .github/pull_request_template.md .github/ISSUE_TEMPLATE/config.yml \
  .github/dependabot.yml .github/release.yml; do
  [ -s "$root/$f" ] || err "missing $f"
done

forms=0
for f in "$root"/.github/ISSUE_TEMPLATE/*.yml; do [ -e "$f" ] && forms=$((forms + 1)); done
[ "$forms" -ge 2 ] || err "expected at least one issue form besides config.yml"

dep="$root/.github/dependabot.yml"
if [ -s "$dep" ]; then
  for eco in gomod github-actions; do
    grep -q "package-ecosystem: \"\?$eco\"\?" "$dep" || err "dependabot.yml has no $eco entry"
  done
  # Every gomod directory must hold a go.mod.
  awk '/package-ecosystem:/ { eco=$NF; gsub(/"/, "", eco) }
       /directory:/ && eco=="gomod" { d=$NF; gsub(/"/, "", d); print d }' "$dep" |
    while read -r d; do
      [ -f "$root/${d#/}/go.mod" ] || { echo "community_check: dependabot gomod directory $d has no go.mod" >&2; exit 1; }
    done || fail=1
fi

rel="$root/.github/release.yml"
if [ -s "$rel" ]; then
  # Labels the release notes sort by must be ones the auto-release reads.
  for label in release:minor release:patch; do
    grep -q "$label" "$rel" || err "release.yml has no category for $label"
  done
fi

if [ "$fail" -ne 0 ]; then exit 1; fi
echo "community_check: ok"
