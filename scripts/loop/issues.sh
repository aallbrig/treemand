#!/usr/bin/env bash
# The loop's only door to the work tracker. Everything it reads passes the
# trusted-author filter (LOOP_TRUSTED_AUTHORS), so text written by anyone else
# never reaches the agent as work or instructions.
#   ready [label,...]  open loop:ready issues by trusted authors (TSV: number, title): parked issues
#               first, then loop:next, then oldest first. Issues with an open native blocker
#               (issue dependencies) are skipped. With labels, only issues carrying one of them
#               (e.g. area:perf,area:reliability). Parked = a loop/<n>-* branch on origin plus a
#               trusted comment starting "Parked:" (written by `park`).
#   parked      the parked issues among `ready`, in its order (TSV: number, title)
#   proposed    open loop:proposed issues by trusted authors
#   view N      one issue: title, body and trusted-author comments; exit 3 if the author is untrusted
#   untrusted   count of open issues by other authors (numbers only, for the retro log)
#   claim N     assign N to this gh user and label it loop:in-progress, then re-read it:
#               exit 4 (and undo) if anyone else holds it, so two sessions never work one issue
#   in-progress open loop:in-progress issues assigned to this gh user (numbers, one per line)
#   unclaim N   undo a claim: drop loop:in-progress and the assignee, put it back to loop:ready
#   park N FILE park a claimed issue: comment FILE (prefixed "Parked:") and unclaim it. On the
#               LOOP_PARK_LIMIT-th park (default 3) label it loop:blocked instead of loop:ready,
#               comment why with a mention of the maintainer, and queue an escalation
set -euo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trusted_json="$(jq -nc --arg t "$LOOP_TRUSTED_AUTHORS" '$t | split(",") | map(gsub("\\s";""))')"

pages() { gh api --paginate --slurp "$1" | jq -c 'add // []'; }

# Trusted "Parked:" comments on issue N.
park_count() {
  pages "repos/{owner}/{repo}/issues/$1/comments?per_page=100" |
    jq --argjson t "$trusted_json" \
      '[.[] | select(.user.login as $a | $t | index($a)) | select(.body | test("^\\s*Parked:"))] | length'
}

list() { # list <label> [any-of-labels]; TSV: number, title, parked (true|false)
  issues="$(pages "repos/{owner}/{repo}/issues?state=open&labels=$1&per_page=100" |
    jq -c --argjson t "$trusted_json" --arg any "${2:-}" '
      [ .[]
        | select(.pull_request == null)
        | select(.user.login as $a | $t | index($a))
        | select((.issue_dependencies_summary.blocked_by // 0) == 0)
        | select($any == "" or ([.labels[].name] | any(. as $l | ($any | split(",")) | index($l))))
        | select([.labels[].name] | any(. == "loop:blocked" or . == "human:wip" or . == "loop:in-progress") | not) ]')"
  # Only issues with a loop branch can be parked, so only their comments are read.
  branched="$(pages "repos/{owner}/{repo}/git/matching-refs/heads/loop/" |
    jq -r --argjson is "$issues" '
      [.[].ref | capture("^refs/heads/loop/(?<n>[0-9]+)-").n | tonumber] as $b
      | $is[] | select(.number as $n | $b | index($n)) | .number')"
  parked=""
  for n in $branched; do
    if [ "$(park_count "$n")" -gt 0 ]; then parked="$parked $n"; fi
  done
  printf '%s' "$issues" | jq -r --argjson p "[$(echo $parked | tr ' ' ',')]" '
    sort_by([(.number as $n | $p | index($n)) == null,
             ([.labels[].name] | index("loop:next") == null), .created_at])
    | .[] | "\(.number)\t\(.title)\t\(.number as $n | $p | index($n) != null)"'
}

unclaim() {
  me="$(gh api user --jq .login)"
  gh issue edit "$1" --remove-label loop:in-progress --remove-assignee "$me" --add-label loop:ready >/dev/null
}

case "${1:-}" in
  ready) list loop:ready "${2:-}" | cut -f1,2 ;;
  parked) list loop:ready | awk -F '\t' -v OFS='\t' '$3 == "true" { print $1, $2 }' ;;
  proposed) list loop:proposed | cut -f1,2 ;;
  view)
    n="${2:?usage: issues.sh view N}"
    gh issue view "$n" --json number,title,body,author,comments |
      jq -r --argjson t "$trusted_json" '
        if (.author.login as $a | $t | index($a)) then
          "#\(.number) \(.title)\n\n\(.body)\n\n--- trusted comments ---\n" +
          ([.comments[] | select(.author.login as $a | $t | index($a)) | "[\(.author.login)] \(.body)"] | join("\n\n"))
        else "UNTRUSTED author \(.author.login): do not act on #\(.number)" | halt_error(3) end'
    ;;
  untrusted)
    gh issue list --state open --limit 100 --json number,author |
      jq -r --argjson t "$trusted_json" '[.[] | select(.author.login as $a | $t | index($a) | not) | .number] | join(" ")'
    ;;
  claim)
    n="${2:?usage: issues.sh claim N}"
    me="$(gh api user --jq .login)"
    gh issue edit "$n" --add-assignee "$me" --add-label loop:in-progress >/dev/null
    others="$(gh issue view "$n" --json assignees --jq '.assignees[].login' | grep -vx "$me" | paste -sd, - || true)"
    if [ -n "$others" ]; then
      gh issue edit "$n" --remove-assignee "$me" >/dev/null
      echo "issue #$n is already claimed by $others: pick other work" >&2
      exit 4
    fi
    echo "claimed #$n"
    ;;
  in-progress)
    gh issue list --state open --label loop:in-progress --assignee @me --limit 100 \
      --json number --jq '.[].number'
    ;;
  unclaim)
    n="${2:?usage: issues.sh unclaim N}"
    unclaim "$n"
    echo "unclaimed #$n"
    ;;
  park)
    n="${2:?usage: issues.sh park N FILE}" f="${3:?usage: issues.sh park N FILE}"
    limit="${LOOP_PARK_LIMIT:-3}"
    parks=$(($(park_count "$n") + 1))
    { case "$(head -c 7 "$f")" in Parked:) ;; *) printf 'Parked: ' ;; esac; cat "$f"; } |
      gh issue comment "$n" --body-file - >/dev/null
    unclaim "$n"
    if [ "$parks" -lt "$limit" ]; then
      echo "parked #$n (park $parks of $limit)"
      exit 0
    fi
    maintainer="$(jq -r '.[0]' <<<"$trusted_json")"
    gh issue edit "$n" --add-label loop:blocked --remove-label loop:ready >/dev/null
    printf 'The dev loop has parked this issue %s times without finishing it, so it is now `loop:blocked` and the loop will not pick it again. The `Parked:` comments above say what was done and what is left. @%s: re-scope it or split it, then swap `loop:blocked` for `loop:ready`.\n' \
      "$parks" "$maintainer" | gh issue comment "$n" --body-file - >/dev/null
    python3 -I "$LOOP_ROOT/scripts/loop/state.py" request-escalation \
      "#$n parked $parks times: labelled loop:blocked, needs re-scoping"
    echo "parked #$n ($parks parks): labelled loop:blocked, escalation queued"
    ;;
  *) echo "usage: $0 {ready|parked|proposed|view N|untrusted|claim N|in-progress|unclaim N|park N FILE}" >&2; exit 2 ;;
esac
