#!/bin/sh
# Unit test for issues.sh against a fake gh, run on a copy of scripts/loop so the
# park step's escalation lands in a throwaway .loop/state.json.
#   ready: trusted authors only; loop:blocked, human:wip, loop:in-progress, pull
#     requests and issues with an open native blocker skipped; parked issues
#     first, then loop:next, then oldest; the whole queue fetched.
#   park: comments Parked:, unclaims; the third park blocks the issue, says why,
#     mentions the maintainer and queues an escalation.
set -u
here="$(cd "$(dirname "$0")" && pwd)"
command -v jq >/dev/null || { echo "issues_test: jq missing, skipping"; exit 0; }
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
root="$tmp/repo"
mkdir -p "$root/scripts/loop" "$tmp/bin"
cp "$here/issues.sh" "$here/lib.sh" "$here/config.defaults.env" "$here/state.py" "$root/scripts/loop/"
cat >"$tmp/bin/gh" <<'GH'
#!/bin/sh
echo "$*" >>"$FAKE_LOG"
case "$*" in
  "api user"*) echo me-bot ;;
  *matching-refs*) cat "$FAKE_DIR/refs.json" ;;
  *"/comments"*)
    n="$(echo "$*" | sed -n 's|.*/issues/\([0-9]*\)/comments.*|\1|p')"
    if [ -f "$FAKE_DIR/comments.$n.json" ]; then cat "$FAKE_DIR/comments.$n.json"; else echo '[[]]'; fi ;;
  "api "*issues*) cat "$FAKE_DIR/issues.json" ;;
  "issue comment"*) cat >>"$FAKE_DIR/posted" ;;
  *) : ;;
esac
GH
chmod +x "$tmp/bin/gh"
# lib.sh reads the trusted authors from the config; use its first one.
me="$(sed -n 's/^LOOP_TRUSTED_AUTHORS=\([^ ,#]*\).*/\1/p' "$here/config.defaults.env")"
issue() { # issue <n> <created> <author> <blocked_by> [labels...]
  n="$1" c="$2" a="$3" b="$4"; shift 4
  jq -nc --argjson n "$n" --arg c "$c" --arg a "$a" --argjson b "$b" --args \
    '{number: $n, title: "issue \($n)", user: {login: $a}, created_at: $c,
      issue_dependencies_summary: {blocked_by: $b},
      labels: [$ARGS.positional[] | {name: .}]}' "$@"
}
comment() { jq -nc --arg a "$1" --arg b "$2" '{user: {login: $a}, body: $b}'; }
{
  issue 10 2026-01-01T00:00:00Z "$me" 0 loop:ready
  issue 11 2026-01-02T00:00:00Z "$me" 0 loop:ready loop:blocked
  issue 12 2026-01-03T00:00:00Z stranger 0 loop:ready
  issue 13 2026-01-04T00:00:00Z "$me" 0 loop:ready human:wip
  issue 14 2026-01-05T00:00:00Z "$me" 0 loop:ready loop:next
  issue 15 2026-01-06T00:00:00Z "$me" 0 loop:ready
  issue 16 2026-01-07T00:00:00Z "$me" 0 loop:ready loop:next
  issue 17 2026-01-08T00:00:00Z "$me" 1 loop:ready loop:next
  issue 18 2026-01-09T00:00:00Z "$me" 0 loop:ready
  issue 19 2026-01-10T00:00:00Z "$me" 0 loop:ready
  issue 20 2026-01-11T00:00:00Z "$me" 0 loop:ready
  issue 21 2026-01-12T00:00:00Z "$me" 0 loop:ready
  jq -nc '{number: 22, title: "a pull request", user: {login: "x"}, created_at: "2026-01-01T00:00:00Z",
    pull_request: {}, issue_dependencies_summary: null, labels: [{name: "loop:ready"}]}' |
    jq -c --arg me "$me" '.user.login = $me'
} | jq -sc '[.]' >"$tmp/issues.json"
# 18 and 19 are parked (branch + trusted Parked: comment); 20 has a branch but no
# park comment; 21 has a park comment from a stranger only.
jq -nc '[[ {ref: "refs/heads/loop/18-a"}, {ref: "refs/heads/loop/19-b"}, {ref: "refs/heads/loop/20-c"},
           {ref: "refs/heads/loop/21-d"}, {ref: "refs/heads/loop/181-e"} ]]' >"$tmp/refs.json"
comment "$me" "Parked: half done" | jq -sc '[.]' >"$tmp/comments.18.json"
comment "$me" "Parked: started" | jq -sc '[.]' >"$tmp/comments.19.json"
comment "$me" "a plan" | jq -sc '[.]' >"$tmp/comments.20.json"
comment stranger "Parked: fake" | jq -sc '[.]' >"$tmp/comments.21.json"

run() { PATH="$tmp/bin:$PATH" FAKE_LOG="$tmp/log" FAKE_DIR="$tmp" bash "$root/scripts/loop/issues.sh" "$@"; }
fail=0
no() { echo "issues_test FAIL: $*" >&2; fail=1; }

got="$(run ready | cut -f1 | tr '\n' ' ')"
[ "$got" = "18 19 14 16 10 15 20 21 " ] || no "order: got '$got', want '18 19 14 16 10 15 20 21 '"
grep -q -- "per_page=100" "$tmp/log" && grep -q -- "--paginate" "$tmp/log" || no "the whole queue is not fetched"
grep -q "issues/10/comments" "$tmp/log" && no "comments fetched for an issue with no loop branch"
run ready | awk -F '\t' 'NF != 2 { bad = 1 } END { exit bad }' || no "ready does not print exactly number and title"
got="$(run parked | cut -f1 | tr '\n' ' ')"
[ "$got" = "18 19 " ] || no "parked: got '$got', want '18 19 '"

# First park of 10: a Parked: comment, unclaimed, not blocked.
echo "done: x; left: y" >"$tmp/note"
: >"$tmp/log"
run park 10 "$tmp/note" >/dev/null || no "park exited non-zero"
head -n 1 "$tmp/posted" | grep -q '^Parked: done: x' || no "park comment does not start with Parked:"
grep -q "issue edit 10 --remove-label loop:in-progress" "$tmp/log" || no "park did not unclaim"
grep -q "loop:blocked" "$tmp/log" && no "first park blocked the issue"
[ -f "$root/.loop/state.json" ] && jq -e '.pending_escalation != ""' "$root/.loop/state.json" >/dev/null &&
  no "first park escalated"

# Third park of 19 (two trusted Parked: comments already, one stranger's ignored).
{ comment "$me" "Parked: one"; comment stranger "Parked: noise"; comment "$me" "Parked: two"; } |
  jq -sc '[.]' >"$tmp/comments.19.json"
: >"$tmp/log" && rm -f "$tmp/posted"
out="$(run park 19 "$tmp/note")" || no "third park exited non-zero"
grep -q "issue edit 19 .*--add-label loop:blocked" "$tmp/log" || no "third park did not label loop:blocked"
grep -q "issue edit 19 .*--remove-label loop:ready" "$tmp/log" || no "third park left loop:ready"
grep -q "@$me" "$tmp/posted" || no "third park did not mention the maintainer"
grep -q "parked this issue 3 times" "$tmp/posted" || no "third park did not say why"
jq -e '.pending_escalation | test("#19")' "$root/.loop/state.json" >/dev/null 2>&1 || no "third park queued no escalation"
echo "$out" | grep -q "loop:blocked" || no "third park did not report the block"

[ "$fail" = 0 ] && echo "issues_test: ok"
exit "$fail"
