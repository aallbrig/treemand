#!/bin/sh
# ship.sh start <branch>
#   Branch from a freshly fetched origin/main (never a stale local main).
# ship.sh pr --subject "<commit subject>" --title "<PR title>" --body <file> [--trailer "K: v"]... [--no-merge]
#   Commit what is staged (the pre-commit gate must pass), push, open the PR,
#   then wait for its checks and merge it (task pr:merge). Each step must
#   succeed before the next runs: a failed gate stops everything.
# Run via task ship:start -- <branch> / task ship:pr -- <args>.
set -eu
cd "$(dirname "$0")/.."

die() { echo "ship: $*" >&2; exit 1; }

case "${1:-}" in
start)
  branch="${2:?usage: ship.sh start <branch>}"
  [ -z "$(git status --porcelain --untracked-files=no)" ] || die "the working tree has uncommitted changes"
  git fetch -q origin main
  git switch -q -c "$branch" origin/main
  echo "ship: on $branch from origin/main ($(git rev-parse --short HEAD))"
  ;;
pr)
  shift
  subject="" title="" body="" merge=1
  # Each option is consumed from the front; trailers are re-appended as
  # "--trailer V" pairs, so once all n arguments are read "$@" holds exactly
  # the git commit trailer options.
  n=$#
  while [ "$n" -gt 0 ]; do
    case "$1" in
    --subject | --title | --body | --trailer) [ "$n" -ge 2 ] || die "$1 needs a value" ;;
    esac
    case "$1" in
    --subject) subject="$2"; shift 2; n=$((n - 2)) ;;
    --title) title="$2"; shift 2; n=$((n - 2)) ;;
    --body) body="$2"; shift 2; n=$((n - 2)) ;;
    --trailer) set -- "$@" --trailer "$2"; shift 2; n=$((n - 2)) ;;
    --no-merge) merge=0; shift; n=$((n - 1)) ;;
    *) die "unknown argument: $1" ;;
    esac
  done
  if [ -z "$subject" ] || [ -z "$title" ] || [ ! -f "$body" ]; then
    die "need --subject, --title and an existing --body file"
  fi
  branch="$(git branch --show-current)"
  [ "$branch" != main ] || die "ship from a branch (ship.sh start <branch>), not main"
  git fetch -q origin main
  git merge-base --is-ancestor origin/main HEAD || die "$branch is not based on the latest origin/main; rebase onto it first"

  if ! git diff --cached --quiet; then
    # --trailer writes the trailer block after a blank line, as git expects.
    git commit -q -m "$subject" "$@"
  fi
  [ "$(git rev-list --count origin/main..HEAD)" -gt 0 ] || die "no commits to ship"
  if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    die "unstaged changes remain (did the gate reformat something?); stage them or stash them first"
  fi

  git push -q -u origin "$branch"
  url="$(gh pr create --title "$title" --body-file "$body")"
  pr="${url##*/}"
  echo "ship: opened $url"
  if [ "$merge" = 1 ]; then
    sh scripts/pr_merge.sh "$pr"
    echo "ship: merged #$pr"
  fi
  ;;
*)
  sed -n '2,8p' "$0" >&2
  exit 2
  ;;
esac
