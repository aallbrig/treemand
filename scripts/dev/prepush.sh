#!/bin/sh
# The pre-push gate, on every branch: the measurement ratchets, the tests with
# coverage floors, and the race tests. A step already proven for every pushed
# tree (proof.sh: task ci stamps them when it passes) is skipped, so the loop
# does not run the suite twice. Reads git's pre-push lines on stdin:
#   <local ref> <local sha> <remote ref> <remote sha>
set -u
case "$0" in */*) here="${0%/*}" ;; *) here=. ;; esac
zero=0000000000000000000000000000000000000000

trees=""
while read -r _ sha _ _; do
  # A deleted remote ref pushes the zero sha: nothing to check.
  [ -n "$sha" ] && [ "$sha" != "$zero" ] || continue
  t="$(git rev-parse "$sha^{tree}")" || exit 1
  trees="$trees $t"
done
[ -n "$trees" ] || exit 0

if ! command -v task >/dev/null 2>&1; then
  echo "pre-push: task not found: install go-task (sh scripts/dev/setup.sh --install)" >&2
  exit 1
fi

proven() {
  for t in $trees; do sh "$here/proof.sh" has "$t" "$1" || return 1; done
}

for step in measure:gate coverage:check test-race; do
  if proven "$step"; then
    echo "pre-push: $step already proven for this tree (task ci): skipped"
    continue
  fi
  echo "pre-push: task $step"
  task "$step" || exit 1
  sh "$here/proof.sh" stamp "$step" >/dev/null
done
