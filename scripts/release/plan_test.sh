#!/bin/sh
# Tests for plan.sh against a scratch git repository and a fake gh that
# answers "which PR merged this commit, with which labels" from a table.
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

mkdir -p "$work/bin" "$work/repo"
# Fake gh: "gh api repos/x/y/commits/<sha>/pulls --jq ..." prints the labels
# recorded for <sha> in $LABELS (lines "sha label..."), one per line.
cat >"$work/bin/gh" <<'EOF'
#!/bin/sh
sha="$(printf '%s\n' "$2" | sed 's#.*/commits/\([0-9a-f]*\)/pulls#\1#')"
grep "^$sha " "$LABELS" 2>/dev/null | cut -d' ' -f2- | tr ' ' '\n' | sed '/^$/d'
exit 0
EOF
chmod +x "$work/bin/gh"
export PATH="$work/bin:$PATH" REPO=example/repo LABELS="$work/labels"
: >"$LABELS"

cd "$work/repo"
git init -q -b main .
git config user.email test@example.com
git config user.name test
merge() { # merge <label...>: one first-parent commit with the given PR labels
  git commit -q --allow-empty -m "merge"
  echo "$(git rev-parse HEAD) $*" >>"$LABELS"
}
plan() { sh "$here/plan.sh"; }

merge
git tag v0.8.2
check "no PRs since the last tag: none" none "$(plan)"

merge area:cli
check "an unlabelled PR does not release" none "$(plan)"

merge release:patch
check "release:patch bumps the patch" "patch v0.8.3" "$(plan)"

merge release:minor area:tui
merge release:patch
check "minor beats patch across PRs since the tag" "minor v0.9.0" "$(plan)"

git tag v0.9.0
merge release:patch
check "only PRs after the latest tag count" "patch v0.9.1" "$(plan)"

git tag v1.0.0
merge release:patch
rc=0
plan >/dev/null 2>&1 || rc=$?
check "1.0 and later are refused (exit 3)" 3 "$rc"

[ "$fails" -eq 0 ] || { echo "plan_test: $fails failure(s)"; exit 1; }
echo "plan_test: all passed"
