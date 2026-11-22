#!/bin/sh
# Decide the next release from the pull requests merged since the last tag.
#
#   sh scripts/release/plan.sh        prints "<bump> <next-tag>", or "none"
#
# A merged pull request asks for a release with the label release:minor or
# release:patch. Every PR merged since the last v* tag counts, so a run that
# GitHub cancels while it waits in the release concurrency group loses nothing:
# the next run releases the same PRs. minor beats patch. A release that would
# reach 1.0.0 is refused (exit 3): declaring 1.0 is a maintainer decision.
#
# Needs git with full history and tags, and gh (REPO=owner/name, default: the
# current repository).
set -eu

repo="${REPO:-$(gh repo view --json nameWithOwner --jq .nameWithOwner)}"
last="$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n 1)"
if [ -n "$last" ]; then
  range="$last..HEAD"
else
  last=v0.0.0
  range=HEAD
fi

bump=none
# First-parent commits on main are the merge commits, one per pull request.
for sha in $(git rev-list --first-parent "$range"); do
  labels="$(gh api "repos/$repo/commits/$sha/pulls" \
    --jq '.[] | select(.merged_at != null) | .labels[].name')"
  case "$labels" in
    *release:minor*) bump="minor" ;;
    *release:patch*) [ "$bump" = minor ] || bump="patch" ;;
  esac
done

if [ "$bump" = none ]; then
  echo none
  exit 0
fi

ver="${last#v}"
major="${ver%%.*}"
rest="${ver#*.}"
minor="${rest%%.*}"
patch="${rest#*.}"
if [ "$bump" = minor ]; then
  minor=$((minor + 1))
  patch=0
else
  patch=$((patch + 1))
fi
if [ "$major" -ge 1 ]; then
  echo "plan: refusing to release $major.x: 1.0 and later are a maintainer decision" >&2
  exit 3
fi
echo "$bump v$major.$minor.$patch"
