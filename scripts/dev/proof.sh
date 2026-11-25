#!/bin/sh
# Stamps that record which checks passed for an exact tree, so pre-push does
# not re-run what task ci just proved.
#
#   proof.sh stamp <step>...    record <step>s as passed for HEAD's tree; does
#                               nothing (and says so) when the working tree has
#                               changes, since the checks did not run on HEAD
#   proof.sh has <tree> <step>  exit 0 when <step> is proven for <tree>
#
# Stamps live in the common git dir (shared by every worktree), one file per
# tree hash listing its steps; stamps older than 30 days are pruned.
set -u
dir="$(git rev-parse --git-common-dir)/treemand-proof" || exit 1
case "$dir" in /*) ;; *) dir="$(pwd)/$dir" ;; esac

case "${1:-}" in
  stamp)
    shift
    [ "$#" -gt 0 ] || { echo "usage: proof.sh stamp <step>..." >&2; exit 2; }
    if [ -n "$(git status --porcelain)" ]; then
      echo "proof: working tree has changes; not stamping $*"
      exit 0
    fi
    tree="$(git rev-parse 'HEAD^{tree}')" || exit 1
    mkdir -p "$dir" || exit 1
    find "$dir" -type f -mtime +30 -exec rm -f {} + 2>/dev/null
    printf '%s\n' "$@" >>"$dir/$tree"
    echo "proof: stamped $* for tree ${tree%"${tree#????????????}"}"
    ;;
  has)
    [ "$#" = 3 ] || { echo "usage: proof.sh has <tree> <step>" >&2; exit 2; }
    [ -f "$dir/$2" ] && grep -qxF -- "$3" "$dir/$2"
    ;;
  *) echo "usage: proof.sh stamp <step>... | has <tree> <step>" >&2; exit 2 ;;
esac
