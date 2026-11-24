#!/bin/sh
# Status line for the loop session (.claude/loop-settings.json). Claude Code
# pipes the session's state to it as JSON on each update; it records the context
# window's used percentage in .loop/context ("<pct> <window tokens> <epoch>") for
# the supervisor and post_tool.sh, which read it without spending tokens, and
# prints a one-line status.
input="$(cat)"
loop="$(dirname "$0")/../../../.loop"
line="$(printf '%s' "$input" | jq -r '.context_window | select(.used_percentage != null) | "\(.used_percentage | floor) \(.context_window_size // 0)"' 2>/dev/null)"
if [ -n "$line" ] && [ -d "$loop" ]; then
  printf '%s %s\n' "$line" "$(date +%s)" >"$loop/context.tmp" && mv "$loop/context.tmp" "$loop/context"
  echo "treemand loop · context ${line%% *}%"
else
  echo "treemand loop"
fi
