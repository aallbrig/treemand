#!/bin/sh
# Tests for lib.sh: run_conclusion rides out transient gh API errors.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
# Fake gh: fails twice (an API error), reports in_progress once, then success.
cat >"$work/gh" <<'GH'
#!/bin/sh
n=$(cat "$COUNT" 2>/dev/null || echo 0); n=$((n + 1)); echo "$n" >"$COUNT"
case "$n" in 1 | 2) exit 1 ;; 3) echo "in_progress " ;; *) echo "completed success" ;; esac
GH
chmod +x "$work/gh"
# shellcheck source=scripts/release/lib.sh
. "$here/lib.sh"
got="$(PATH="$work:$PATH" COUNT="$work/n" RELEASE_POLL_SECONDS=0 run_conclusion 1 example/repo)"
if [ "$got" = success ]; then echo "  ok   run_conclusion survives API errors and waits for completion"; else
  echo "  FAIL run_conclusion: want success, got '$got'"
  exit 1
fi

# tag_for finds the tag on the merge itself or on the changelog commit above it.
repo="$work/repo"
git init -q -b main "$repo"
g() { git -C "$repo" -c user.email=t@e -c user.name=t "$@"; }
g commit -q --allow-empty -m one && one="$(g rev-parse HEAD)" && g tag -a v0.1.0 -m v0.1.0
g commit -q --allow-empty -m two && two="$(g rev-parse HEAD)"
g commit -q --allow-empty -m "Add the v0.1.1 changelog entry" && g tag -a v0.1.1 -m v0.1.1
g commit -q --allow-empty -m three && three="$(g rev-parse HEAD)"
got="$(cd "$repo" && tag_for "$one") $(cd "$repo" && tag_for "$two") $(cd "$repo" && tag_for "$three")"
if [ "$got" = "v0.1.0 v0.1.1 " ]; then echo "  ok   tag_for: the merge's own tag, the entry commit's tag, or none"; else
  echo "  FAIL tag_for: want 'v0.1.0 v0.1.1 ', got '$got'"
  exit 1
fi
