#!/usr/bin/env bash
# start | stop | status | attach for the treemand dev loop.
set -euo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"


have_session() { tmux has-session -t "=$LOOP_SESSION" 2>/dev/null; }

# The agent pane is the second pane of the loops window (pane 0 is the dev shell).
# Pane titles are not reliable: the shell prompt overwrites them.
agent_pane() {
  tmux list-panes -t "=$LOOP_SESSION:loops" -F '#{pane_id}' 2>/dev/null | sed -n 2p
}

supervisor_running() {
  local pane cmd
  pane="$(agent_pane)"
  [ -n "$pane" ] || return 1
  cmd="$(tmux display-message -p -t "$pane" '#{pane_current_command}')"
  case "$cmd" in bash | zsh | sh | fish) return 1 ;; *) return 0 ;; esac
}

kill_tree() {
  local pid="$1" child
  for child in $(pgrep -P "$pid" 2>/dev/null || true); do kill_tree "$child"; done
  kill "$pid" 2>/dev/null || true
}

cmd_start() {
  rm -f "$LOOP_STOP_FILE"
  if ! have_session; then
    bash "$LOOP_ROOT/scripts/dev_session.sh" start
    echo "loop started (session created; supervisor launching in loops:agent)"
    return 0
  fi
  if supervisor_running; then
    echo "loop already running"
    return 0
  fi
  local pane
  pane="$(agent_pane)"
  [ -n "$pane" ] || { echo "loops:agent pane missing; run: task dev:recycle" >&2; exit 1; }
  tmux send-keys -t "$pane" "task loop:agent" Enter
  echo "loop started"
}

cmd_stop() {
  touch "$LOOP_STOP_FILE"
  echo "STOP file written; the loop exits at its next phase boundary"
  if [ "${1:-}" = "--now" ] && have_session; then
    local pane pid
    pane="$(agent_pane)"
    if [ -n "$pane" ]; then
      pid="$(tmux display-message -p -t "$pane" '#{pane_pid}')"
      for c in $(pgrep -P "$pid" 2>/dev/null || true); do kill_tree "$c"; done
      echo "agent pane process tree killed"
    fi
  fi
}

cmd_status() {
  if [ -e "$LOOP_STOP_FILE" ]; then echo "stop file: PRESENT (loop will not run)"; else echo "stop file: absent"; fi
  if have_session && supervisor_running; then echo "supervisor: running (session $LOOP_SESSION, window loops)"; else echo "supervisor: not running"; fi
  python3 -I "$LOOP_ROOT/scripts/loop/state.py" summary
}

cmd_attach() {
  have_session || { echo "session not running (task dev)" >&2; exit 1; }
  tmux select-window -t "=$LOOP_SESSION:loops"
  if [ -n "${TMUX:-}" ]; then tmux switch-client -t "=$LOOP_SESSION"; else tmux attach-session -t "=$LOOP_SESSION"; fi
}

case "${1:-}" in
  start) cmd_start ;;
  stop) shift; cmd_stop "${1:-}" ;;
  status) cmd_status ;;
  attach) cmd_attach ;;
  *) echo "usage: $0 {start|stop [--now]|status|attach}" >&2; exit 2 ;;
esac
