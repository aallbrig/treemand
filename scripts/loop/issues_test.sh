#!/bin/sh
# Unit test for issues.sh ready: trusted authors only, loop:blocked, human:wip and
# loop:in-progress skipped, loop:next first, then oldest first, and the whole
# queue fetched (not only the newest 100). A fake gh serves the issue list.
set -u
here="$(cd "$(dirname "$0")" && pwd)"
command -v jq >/dev/null || { echo "issues_test: jq missing, skipping"; exit 0; }
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
cat >"$tmp/gh" <<'GH'
#!/bin/sh
echo "$*" >>"$FAKE_LOG"
cat "$FAKE_ISSUES"
GH
chmod +x "$tmp/gh"
# lib.sh reads the trusted authors from the config; use its first one.
me="$(sed -n 's/^LOOP_TRUSTED_AUTHORS=\([^ ,#]*\).*/\1/p' "$here/config.defaults.env")"
issue() { # issue <n> <created> <author> [labels...]
  n="$1" c="$2" a="$3"; shift 3
  jq -nc --argjson n "$n" --arg c "$c" --arg a "$a" --args \
    '{number: $n, title: "issue \($n)", author: {login: $a}, createdAt: $c, labels: [$ARGS.positional[] | {name: .}]}' "$@"
}
{
  issue 10 2026-01-01T00:00:00Z "$me" loop:ready
  issue 11 2026-01-02T00:00:00Z "$me" loop:ready loop:blocked
  issue 12 2026-01-03T00:00:00Z stranger loop:ready
  issue 13 2026-01-04T00:00:00Z "$me" loop:ready human:wip
  issue 14 2026-01-05T00:00:00Z "$me" loop:ready loop:next
  issue 15 2026-01-06T00:00:00Z "$me" loop:ready
  issue 16 2026-01-07T00:00:00Z "$me" loop:ready loop:next
} | jq -sc . >"$tmp/issues.json"
got="$(PATH="$tmp:$PATH" FAKE_LOG="$tmp/log" FAKE_ISSUES="$tmp/issues.json"  \
  bash "$here/issues.sh" ready | cut -f1 | tr '\n' ' ')"
fail=0
[ "$got" = "14 16 10 15 " ] || { echo "issues_test FAIL: order: got '$got', want '14 16 10 15 '" >&2; fail=1; }
grep -q -- "--limit 1000" "$tmp/log" || { echo "issues_test FAIL: the whole queue is not fetched" >&2; fail=1; }
[ "$fail" = 0 ] && echo "issues_test: ok"
exit "$fail"
