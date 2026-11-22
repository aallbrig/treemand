#!/usr/bin/env bash
# start | stop | status | attach for the treemand dev loop.
set -euo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"


have_session() { tmux has-session -t "=$LOOP_SESSION" 2>/dev/null; }

# The agent pane is marked with the pane option @treemand_role=agent (titles are
# not reliable and positions shift). It exists only while the loop runs: start
# creates it in the loops window running $LOOP_AGENT_CMD directly (no shell), so
# the pane closes when the supervisor exits.
AGENT_ROLE=agent

agent_pane() {
  tmux list-panes -s -t "=$LOOP_SESSION" -F '#{pane_id} #{@treemand_role}' 2>/dev/null |
    awk -v r="$AGENT_ROLE" '$2 == r { print $1; exit }'
}

supervisor_running() {
  local pane
  pane="$(agent_pane)"
  [ -n "$pane" ] || return 1
  [ "$(tmux display-message -p -t "$pane" '#{pane_dead}')" != 1 ]
}

kill_tree() {
  local pid="$1" child
  for child in $(pgrep -P "$pid" 2>/dev/null || true); do kill_tree "$child"; done
  kill "$pid" 2>/dev/null || true
}

create_agent_pane() {
  local window="=$LOOP_SESSION:loops" pane
  tmux list-panes -t "$window" >/dev/null 2>&1 || { echo "loops window missing; run: task dev:recycle" >&2; exit 1; }
  local env_args=(-e "TREEMAND_SESSION=$LOOP_SESSION")
  if [ -n "$_env_claude" ]; then env_args+=(-e "LOOP_CLAUDE_BIN=$_env_claude"); fi
  pane="$(tmux split-window -d -h -t "$window" -c "$LOOP_ROOT" "${env_args[@]}" -P -F '#{pane_id}' "$LOOP_AGENT_CMD")"
  tmux set-option -p -t "$pane" @treemand_role "$AGENT_ROLE"
  tmux set-option -p -t "$pane" remain-on-exit off
  tmux select-pane -t "$pane" -T agent
}

cmd_start() {
  rm -f "$LOOP_STOP_FILE"
  if ! have_session; then
    bash "$LOOP_ROOT/scripts/dev_session.sh" start >/dev/null
  fi
  if supervisor_running; then
    echo "loop already running"
    return 0
  fi
  local stale
  stale="$(agent_pane)"
  if [ -n "$stale" ]; then tmux kill-pane -t "$stale"; fi
  create_agent_pane
  echo "loop started (supervisor running in the loops window's agent pane)"
}

cmd_stop() {
  touch "$LOOP_STOP_FILE"
  echo "STOP file written; the loop exits at its next phase boundary"
  if [ "${1:-}" = "--now" ] && have_session; then
    local pane pid
    pane="$(agent_pane)"
    if [ -n "$pane" ]; then
      pid="$(tmux display-message -p -t "$pane" '#{pane_pid}')"
      kill_tree "$pid"
      tmux kill-pane -t "$pane" 2>/dev/null || true
      echo "agent pane process tree killed and pane closed"
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
