#!/bin/sh
# PostToolUse hook (Edit|Write) in the project settings (.claude/settings.json),
# so loop and interactive sessions both get it. After a Go file is edited
# it tells the agent, at once, what CI would otherwise tell it minutes later:
#   - gofmt: the file does not parse, or is not formatted;
#   - complexity (non-test files): a function over cognitive complexity 15 that
#     is new or worse than on origin/main, which fails the complexity ratchet.
# Silent when there is nothing to say, and for every file that is not Go.
# GO_FEEDBACK_COGNIT=0 skips the complexity check.
set -u
input="$(cat)"
file="$(printf '%s' "$input" | jq -r '.tool_input.file_path // ""' 2>/dev/null)"
case "$file" in *.go) ;; *) exit 0 ;; esac
[ -f "$file" ] || exit 0
command -v gofmt >/dev/null || exit 0
top="$(git -C "$(dirname "$file")" rev-parse --show-toplevel 2>/dev/null)" || top="$(dirname "$file")"
rel="${file#"$top"/}"
notes=""
add() { notes="$notes
- $1"; }

if ! errs="$(gofmt -l "$file" 2>&1)"; then
  add "gofmt: $rel does not parse: $(printf '%s' "$errs" | head -n 3 | tr '\n' ' ')"
elif [ -n "$errs" ]; then
  add "gofmt: $rel is not formatted; run gofmt -w $rel."
fi

case "$file" in *_test.go) ;; *)
  if [ "${GO_FEEDBACK_COGNIT:-1}" != 0 ] && [ -z "$notes" ]; then
    tmp="$(mktemp -d)"
    cognit() { (cd "$tmp" && GOFLAGS='' go run github.com/uudashr/gocognit/cmd/gocognit@v1.3.0 -over 15 "$1" 2>/dev/null); }
    cognit "$file" | awk '{print $3, $1}' | sort >"$tmp/now"
    if [ -s "$tmp/now" ]; then
      if git -C "$top" show "origin/main:$rel" >"$tmp/base.go" 2>/dev/null; then
        cognit "$tmp/base.go" | awk '{print $3, $1}' | sort >"$tmp/base"
      else
        : >"$tmp/base"
      fi
      worse="$(awk 'NR == FNR { base[$1] = $2; next } !($1 in base) || $2 > base[$1] {
          printf "%s %s (%s); ", $1, $2, ($1 in base) ? "was " base[$1] " on origin/main" : "new" }' "$tmp/base" "$tmp/now")"
      [ -n "$worse" ] && add "complexity: $rel has functions over cognitive complexity 15 that are new or worse than on origin/main: ${worse%; }. The complexity ratchet fails on these. Reduce what the function has to decide, or move the policy into one module; do not just split it into one helper per case."
    fi
    rm -rf "$tmp"
  fi
  ;;
esac

[ -n "$notes" ] || exit 0
jq -nc --arg c "Go feedback for $rel:$notes" '{hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: $c}}'
