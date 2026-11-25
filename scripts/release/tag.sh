#!/bin/sh
# Tag a release with its changelog entry inside it.
#
#   sh scripts/release/tag.sh <vX.Y.Z> <body-file>
#
# Writes the release body to <body-file> (Auto Release sets it on the GitHub
# release once Release succeeds) and the treemand.dev changelog entry
# (notes.py), commits the entry on HEAD, tags that commit (vX.Y.Z and the Go
# module tag cli/treemand/vX.Y.Z) and pushes both tags.
# The tag's own site deploy therefore shows the entry. Then the commit goes to
# main: a fast-forward, or, if a PR merged meanwhile, a merge commit whose first
# parent is that newer main, so the newer PR stays outside this tag and the
# next plan.sh still counts it. Never a force-push. Run from the repository,
# with git's user.name and user.email set.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
tag="${1:?usage: tag.sh <vX.Y.Z> <body-file>}"
body="${2:?usage: tag.sh <vX.Y.Z> <body-file>}"
cd "$(git rev-parse --show-toplevel)"
dir=www/treemand/content/changelog

python3 -I "$here/notes.py" --tag "$tag" --body "$body" --changelog-dir "$dir"
git add "$dir/$tag.md"
git commit -q -m "Add the $tag changelog entry"
entry="$(git rev-parse HEAD)"
git tag -a "$tag" -m "$tag" "$entry"
# The Go module lives in cli/treemand, so the Go proxy resolves
# `go install github.com/aallbrig/treemand/cli/treemand@$tag` from this tag.
git tag -a "cli/treemand/$tag" -m "$tag" "$entry"
git push -q --atomic origin "$tag" "cli/treemand/$tag"
echo "tag: pushed $tag and cli/treemand/$tag ($entry)"

i=0
until git push -q origin "$entry:refs/heads/main" 2>/dev/null; do
  i=$((i + 1))
  if [ "$i" -gt 3 ]; then
    echo "tag: could not put the $tag changelog entry on main; it is on the tag" >&2
    exit 0
  fi
  git fetch -q origin main
  git checkout -q --detach FETCH_HEAD
  git merge -q --no-ff -m "Merge the $tag changelog entry" "$entry"
  entry="$(git rev-parse HEAD)"
done
echo "tag: the $tag changelog entry is on main"
