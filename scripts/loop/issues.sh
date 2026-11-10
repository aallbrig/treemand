#!/usr/bin/env bash
# The loop's only door to the work tracker. Everything it reads passes the
# trusted-author filter (LOOP_TRUSTED_AUTHORS), so text written by anyone else
# never reaches the agent as work or instructions.
#   ready       open loop:ready issues by trusted authors, oldest first (TSV: number, title)
#   proposed    open loop:proposed issues by trusted authors
#   view N      one issue: title, body and trusted-author comments; exit 3 if the author is untrusted
#   untrusted   count of open issues by other authors (numbers only, for the retro log)
set -euo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trusted_json="$(jq -nc --arg t "$LOOP_TRUSTED_AUTHORS" '$t | split(",") | map(gsub("\\s";""))')"

list() { # list <label>
  gh issue list --state open --label "$1" --limit 100 \
    --json number,title,author,labels,createdAt |
    jq -r --argjson t "$trusted_json" '
      [ .[]
        | select(.author.login as $a | $t | index($a))
        | select([.labels[].name] | any(. == "loop:blocked" or . == "human:wip" or . == "loop:in-progress") | not) ]
      | sort_by(.createdAt) | .[] | "\(.number)\t\(.title)"'
}

case "${1:-}" in
  ready) list loop:ready ;;
  proposed) list loop:proposed ;;
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
  *) echo "usage: $0 {ready|proposed|view N|untrusted}" >&2; exit 2 ;;
esac
