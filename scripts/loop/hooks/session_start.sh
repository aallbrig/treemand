#!/bin/sh
# SessionStart hook for the loop session (.claude/loop-settings.json). Prints
# hookSpecificOutput.additionalContext with what a cycle needs before it reads
# anything: the preflight verdict, the selected work, the vision's goals, the
# last LEARNINGS entries and the newest measurement per probe. Cheap reads only:
# no probes. The work check (work.sh, which runs the preflight) runs only when
# the supervisor's .loop/work line is missing or older than 10 minutes.
set -u
root="$(cd "$(dirname "$0")/../../.." && pwd)"
loop="$root/.loop"
input="$(cat)"
source="$(printf '%s' "$input" | jq -r '.source // ""' 2>/dev/null)"

# A new context (startup, /clear, compaction): forget the last one's usage, so
# the status line measures this one and post_tool.sh may note it again.
rm -f "$loop/context" "$loop/context-noted" "$loop/stop-blocks"

# --- preflight verdict and work ----------------------------------------------------
line="" verdict=""
if [ -e "$loop/STOP" ]; then
  verdict="STOP:.loop/STOP present"
elif [ -f "$loop/work" ] && { [ "$source" = compact ] || [ -n "$(find "$loop/work" -mmin -10 2>/dev/null)" ]; }; then
  line="$(cat "$loop/work")"
elif line="$(sh "$root/scripts/loop/work.sh" 2>"$loop/work.err")"; then
  printf '%s\n' "$line" >"$loop/work"
else
  line=""
  verdict="$(tail -n 1 "$loop/work.err")"
fi
rm -f "$loop/work.err"
case "$line" in
  FIX_MAIN:*) verdict="$line" line="fix main" ;;
esac
[ -n "$verdict" ] || verdict=GO
[ -n "$line" ] || line="none (end-cycle idle, or stopped on STOP)"

# --- vision goals: list items of the goal sections, first sentence each ------------
vision="$(find "$root/docs/design" -name '*_vision.md' 2>/dev/null | sort | tail -n 1)"
goals() {
  awk '
    function flush() {
      if (item != "") {
        gsub(/\*\*/, "", item)
        s = item
        if (match(s, /^([0-9]+\.|-) /)) { lead = substr(s, 1, RLENGTH); s = substr(s, RLENGTH + 1) } else lead = ""
        if (match(s, /[.!?]( |$)/)) s = substr(s, 1, RSTART)
        if (length(s) > 140) s = substr(s, 1, 137) "..."
        print lead s
      }
      item = ""
    }
    /^## / { flush(); on = ($0 ~ /^## (Principles|Goals|Where|Out of scope)/); if (on) print; next }
    !on { next }
    /^([0-9]+\.|-) / { flush(); item = $0; next }
    /^[ \t]+[^ \t]/ && item != "" { sub(/^[ \t]+/, ""); item = item " " $0; next }
    { flush() }
    END { flush() }
  ' "$1"
}

# --- LEARNINGS: the last three cycle entries ---------------------------------------
learnings() {
  awk '/^### / { n++ } { line[NR] = $0; entry[NR] = n } END { for (i = 1; i <= NR; i++) if (entry[i] > n - 3) print line[i] }' "$1" |
    grep -v '^[[:space:]]*$' | cut -c 1-600
}

# --- measurements: the newest line of each docs/metrics/*.jsonl -------------------
metrics() {
  for f in "$root"/docs/metrics/*.jsonl; do
    [ -f "$f" ] || continue
    name="$(basename "$f" .jsonl)"
    tail -n 1 "$f" | jq -r --arg n "$name" '
      [.metrics | to_entries[] | "\(.key)=\(.value)"] as $m
      | "\($n) \(.date[0:10]) \(.sha): \($m[0:8] | join(" "))"
        + (if ($m | length) > 8 then " (+\(($m | length) - 8) more)" else "" end)' 2>/dev/null
  done
}

ctx="$(
  echo "treemand loop context (from scripts/loop/hooks/session_start.sh)"
  echo
  echo "Preflight: $verdict"
  echo "Work: $line"
  if [ -n "$vision" ]; then
    echo
    echo "Vision goals (${vision#"$root"/}):"
    goals "$vision"
  fi
  if [ -f "$root/docs/loop/LEARNINGS.md" ]; then
    echo
    echo "Recent LEARNINGS (docs/loop/LEARNINGS.md):"
    learnings "$root/docs/loop/LEARNINGS.md"
  fi
  m="$(metrics)"
  if [ -n "$m" ]; then
    echo
    echo "Latest measurements (newest line of docs/metrics/*.jsonl; task measure -- <probe> to re-run):"
    echo "$m"
  fi
)"
# Stay well under ~2k tokens whatever the files hold.
ctx="$(printf '%s' "$ctx" | head -c 7000)"
jq -nc --arg c "$ctx" '{hookSpecificOutput: {hookEventName: "SessionStart", additionalContext: $c}}'
