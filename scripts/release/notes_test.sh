#!/bin/sh
# Tests for notes.py (release body and changelog entry from the PRs merged
# since the last tag) and tag.sh (commit the entry, tag it, put it on main),
# against a scratch repository, a bare origin and a fake gh that answers
# "which PR merged this commit" from one JSON file per commit.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fails=0
check() { # check <name> <want> <got>
  if [ "$2" = "$3" ]; then echo "  ok   $1"; else
    printf '  FAIL %s:\n--- want\n%s\n--- got\n%s\n' "$1" "$2" "$3"
    fails=$((fails + 1))
  fi
}

mkdir -p "$work/bin" "$work/pulls"
# Fake gh: "gh api repos/x/y/commits/<sha>/pulls" prints $PULLS/<sha>.json, or [].
cat >"$work/bin/gh" <<'EOF'
#!/bin/sh
sha="$(printf '%s\n' "$2" | sed 's#.*/commits/\([0-9a-f]*\)/pulls#\1#')"
if [ -f "$PULLS/$sha.json" ]; then cat "$PULLS/$sha.json"; else echo '[]'; fi
EOF
chmod +x "$work/bin/gh"
export PATH="$work/bin:$PATH" REPO=example/repo PULLS="$work/pulls"

git init -q --bare -b main "$work/origin.git"
git clone -q "$work/origin.git" "$work/repo" 2>/dev/null
cd "$work/repo"
git config user.email test@example.com
git config user.name test
git commit -q --allow-empty -m start
git tag -a v0.9.4 -m v0.9.4
git push -q origin main v0.9.4

# pr <number> <labels, comma-separated> <title> <body>: one first-parent commit
# that the fake gh says PR <number> merged.
pr() {
  git commit -q --allow-empty -m "Merge pull request #$1"
  python3 -I - "$PULLS/$(git rev-parse HEAD).json" "$@" <<'PY'
import json, sys
out, number, labels, title, body = sys.argv[1:]
names = [n for n in labels.split(",") if n]
pull = {"number": int(number), "title": title, "body": body, "merged_at": "2026-11-22T10:00:00Z",
        "labels": [{"name": n} for n in names]}
open(out, "w").write(json.dumps([pull]))
PY
}
notes() { python3 -I "$here/notes.py" "$@"; }

# ── a maintenance release ────────────────────────────────────────────────
pr 10 "" "Split TreeModel.Left into helpers" "## Summary
x

## Release note

none"
git commit -q --allow-empty -m "chore: a direct bot commit, no PR"
notes --tag v0.9.5 --date 2026-11-22T12:00:00Z --body "$work/body.md" --changelog-dir "$work/cl"
check "maintenance: the release body lists internals under the hood" "## Under the hood

- Split TreeModel.Left into helpers (#10)

**Full changelog**: https://github.com/example/repo/compare/v0.9.4...v0.9.5" "$(cat "$work/body.md")"
check "maintenance: the changelog entry is one line" '---
title: "v0.9.5"
date: 2026-11-22T12:00:00Z
---

Maintenance release: no user-visible changes.' "$(cat "$work/cl/v0.9.5.md")"

# ── a feature release, every kind of PR ──────────────────────────────────
pr 11 "release:minor,area:tui" "Add yank" "## Release note
<!-- hint -->
Press y in the TUI to copy the built command."
pr 12 "release:patch" "Fix aliases" "## Release note
Fixed aliases missing
from the tree.

Part of #1."
pr 13 "release:patch" "Depth default" "## Release note
Changed: --depth now defaults to 3."
pr 14 "" "Install page OS first" "## Release note
The installation page shows your operating system first."
pr 15 "release:patch" "Old-style PR title" "## Summary
no note section"
pr 16 "dependencies" "Bump x from 1 to 2" "Bumps x."
pr 17 "" "Tune CI" "## Release note
\`none\`"
notes --tag v0.10.0 --date 2026-11-23T08:30:00Z --body "$work/body.md" --changelog-dir "$work/cl"
check "feature: the release body groups notes, then internals, then the compare link" "## Features

- Press y in the TUI to copy the built command. (#11)

## Changes

- --depth now defaults to 3. (#13)
- The installation page shows your operating system first. (#14)

## Fixes

- Fixed aliases missing from the tree. (#12)
- Old-style PR title (#15)

## Under the hood

- Split TreeModel.Left into helpers (#10)
- Bump x from 1 to 2 (#16)
- Tune CI (#17)

**Full changelog**: https://github.com/example/repo/compare/v0.9.4...v0.10.0" "$(cat "$work/body.md")"
check "feature: the changelog entry has Added, Changed and Fixed, with no numbers" '---
title: "v0.10.0"
date: 2026-11-23T08:30:00Z
---

### Added

- Press y in the TUI to copy the built command.

### Changed

- --depth now defaults to 3.
- The installation page shows your operating system first.

### Fixed

- Fixed aliases missing from the tree.
- Old-style PR title' "$(cat "$work/cl/v0.10.0.md")"

rc=0
notes --tag v0.10.0 --since v0.9.4 --until v0.9.4 --body "$work/empty.md" >/dev/null 2>&1 || rc=$?
check "an empty range still writes a body" "0" "$rc"

# ── tag.sh: commit the entry, tag that commit, put it on main ────────────
sh "$here/tag.sh" v0.10.0 "$work/tag-body.md" >/dev/null
check "tag.sh: the tag holds its own changelog entry" "v0.10.0" \
  "$(git show v0.10.0:www/treemand/content/changelog/v0.10.0.md | sed -n 's/^title: "\(.*\)"/\1/p')"
check "tag.sh: origin main fast-forwards to the tagged commit" "$(git rev-parse 'v0.10.0^{commit}')" \
  "$(git rev-parse origin/main)"
check "tag.sh: the release body is written for the later edit" "## Features" "$(head -n 1 "$work/tag-body.md")"

# main moves while the release runs: the entry arrives in a merge, never a force-push.
git clone -q "$work/origin.git" "$work/other" 2>/dev/null
(cd "$work/other" && git -c user.email=t@e -c user.name=t commit -q --allow-empty -m "Merge pull request #20" && git push -q origin main)
moved="$(git -C "$work/other" rev-parse HEAD)"
pr 18 "release:patch" "Fix y" "## Release note
Fixed y."
sh "$here/tag.sh" v0.10.1 "$work/tag-body.md" >/dev/null
git fetch -q origin
check "tag.sh race: the tag is the entry commit on the planned history" "$(git rev-parse 'v0.10.1^{commit}')" \
  "$(git rev-parse 'origin/main^2')"
check "tag.sh race: the newer merge stays first-parent on main" "$moved" "$(git rev-parse 'origin/main^1')"

[ "$fails" -eq 0 ] || { echo "notes_test: $fails failure(s)"; exit 1; }
echo "notes_test: all passed"
