#!/bin/sh
# Tests for ship.sh pr against a scratch clone with a bare origin and a fake
# gh: the commit carries every --trailer, the branch is pushed, and the PR is
# opened (--no-merge, so no checks are awaited).
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fails=0
check() { # check <name> <want> <got>
  if [ "$2" = "$3" ]; then echo "  ok   $1"; else
    echo "  FAIL $1: want '$2', got '$3'"
    fails=$((fails + 1))
  fi
}

mkdir -p "$work/bin" "$work/hooks"
cat >"$work/bin/gh" <<'GH'
#!/bin/sh
echo "$*" >>"$GH_LOG"
echo "https://github.com/example/repo/pull/42"
GH
chmod +x "$work/bin/gh"
export PATH="$work/bin:$PATH" GH_LOG="$work/gh.log"
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com

git init -q --bare -b main "$work/origin.git"
git clone -q "$work/origin.git" "$work/repo" 2>/dev/null
cd "$work/repo"
git config core.hooksPath "$work/hooks"
mkdir scripts && cp "$here/ship.sh" scripts/
git add scripts && git commit -q -m init && git push -q origin HEAD:main

sh scripts/ship.sh start feature >/dev/null
check "start branches from origin/main" feature "$(git branch --show-current)"

echo change >file && git add file
echo "body" >"$work/body.md"
out="$(sh scripts/ship.sh pr --subject "Add a file" --title "Title" --body "$work/body.md" \
  --trailer "Co-Authored-By: A <a@example.com>" --trailer "Session: s 1" --no-merge 2>&1)"
check "the commit subject" "Add a file" "$(git log -1 --format=%s)"
check "every trailer, in order" "Co-Authored-By: A <a@example.com>
Session: s 1" "$(git log -1 --format=%B | git interpret-trailers --parse)"
check "the branch is pushed" "$(git rev-parse HEAD)" "$(git rev-parse origin/feature)"
check "the PR is opened with the title and body" "pr create --title Title --body-file $work/body.md" "$(cat "$GH_LOG")"
case "$out" in *"opened https://github.com/example/repo/pull/42"*) echo "  ok   reports the PR" ;; *)
  echo "  FAIL reports the PR: $out"; fails=$((fails + 1)) ;; esac

if sh scripts/ship.sh pr --subject S --title T --body "$work/body.md" --trailer >/dev/null 2>&1; then
  echo "  FAIL a --trailer without a value is refused"; fails=$((fails + 1))
else echo "  ok   a --trailer without a value is refused"; fi
if sh scripts/ship.sh pr --bogus >/dev/null 2>&1; then
  echo "  FAIL an unknown argument is refused"; fails=$((fails + 1))
else echo "  ok   an unknown argument is refused"; fi

[ "$fails" = 0 ] || { echo "$fails failed"; exit 1; }
echo "ship: all passed"
