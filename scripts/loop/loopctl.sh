#!/bin/sh
# start | stop [--now] | resume | status | attach for the treemand dev loop.
#
# The loop runs from its HOME worktree, a sibling of the main checkout named
# <checkout>.loop (e.g. ~/src/treemand.loop), detached at origin/main and
# fast-forwarded by preflight each cycle. Per-issue worktrees live under its
# .loop/worktrees/. A developer's checkout is never touched by the loop.
# LOOP_HOME overrides the location.
#
# Desired state is HOME/.loop/ENABLED: start writes it, stop removes it (and
# writes STOP so the cycle in flight ends), standing down leaves it. `resume`
# (run by the dev session's loops window when it starts, e.g. at login) starts
# the supervisor only when ENABLED exists.
set -eu
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "$0")/lib.sh"

# The main checkout is the parent of the shared git dir, whichever worktree we are in.
common="$(git -C "$LOOP_ROOT" rev-parse --path-format=absolute --git-common-dir)"
main_checkout="$(dirname "$common")"
HOME_WT="${LOOP_HOME:-$(dirname "$main_checkout")/$(basename "$main_checkout").loop}"
HOME_DIR="$HOME_WT/.loop"

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
  pane="$(agent_pane)"
  [ -n "$pane" ] || return 1
  [ "$(tmux display-message -p -t "$pane" '#{pane_dead}')" != 1 ]
}

kill_tree() {
  for child in $(pgrep -P "$1" 2>/dev/null || true); do kill_tree "$child"; done
  kill "$1" 2>/dev/null || true
}

ensure_home() {
  if [ -d "$HOME_WT" ]; then
    git -C "$HOME_WT" rev-parse --git-dir >/dev/null 2>&1 || { echo "$HOME_WT exists but is not a git worktree" >&2; exit 1; }
    return 0
  fi
  git -C "$LOOP_ROOT" fetch -q origin main
  git -C "$LOOP_ROOT" worktree add -q --detach "$HOME_WT" origin/main
  mkdir -p "$HOME_DIR"
  # Carry the host's loop config over once; it is gitignored and never holds secrets.
  if [ -f "$LOOP_DIR/config.env" ]; then cp "$LOOP_DIR/config.env" "$HOME_DIR/config.env"; fi
  echo "loop home worktree created at $HOME_WT (detached at origin/main)"
}

create_agent_pane() {
  window="=$LOOP_SESSION:loops"
  tmux list-panes -t "$window" >/dev/null 2>&1 || { echo "loops window missing; run: task dev:recycle" >&2; exit 1; }
  if [ -n "$_env_claude" ]; then
    pane="$(tmux split-window -d -h -t "$window" -c "$HOME_WT" -e "TREEMAND_SESSION=$LOOP_SESSION" \
      -e "LOOP_CLAUDE_BIN=$_env_claude" -P -F '#{pane_id}' "$LOOP_AGENT_CMD")"
  else
    pane="$(tmux split-window -d -h -t "$window" -c "$HOME_WT" -e "TREEMAND_SESSION=$LOOP_SESSION" \
      -P -F '#{pane_id}' "$LOOP_AGENT_CMD")"
  fi
  tmux set-option -p -t "$pane" @treemand_role "$AGENT_ROLE"
  tmux set-option -p -t "$pane" remain-on-exit off
  tmux select-pane -t "$pane" -T agent
}

launch() {
  if ! have_session; then
    bash "$LOOP_ROOT/scripts/dev_session.sh" start >/dev/null
  fi
  if supervisor_running; then
    echo "loop already running"
    return 0
  fi
  stale="$(agent_pane)"
  if [ -n "$stale" ]; then tmux kill-pane -t "$stale"; fi
  create_agent_pane
  echo "loop started (supervisor running in the loops window's agent pane, from $HOME_WT)"
}

cmd_start() {
  ensure_home
  mkdir -p "$HOME_DIR"
  rm -f "$HOME_DIR/STOP"
  touch "$HOME_DIR/ENABLED"
  launch
}

cmd_resume() {
  if [ ! -e "$HOME_DIR/ENABLED" ]; then
    echo "loop not enabled (task loop:start to enable it)"
    return 0
  fi
  launch
}

cmd_stop() {
  mkdir -p "$HOME_DIR"
  rm -f "$HOME_DIR/ENABLED"
  touch "$HOME_DIR/STOP"
  echo "loop disabled; the cycle in flight ends at its next phase boundary, then the pane closes"
  if [ "${1:-}" = "--now" ] && have_session; then
    pane="$(agent_pane)"
    if [ -n "$pane" ]; then
      kill_tree "$(tmux display-message -p -t "$pane" '#{pane_pid}')"
      tmux kill-pane -t "$pane" 2>/dev/null || true
      echo "agent pane process tree killed and pane closed"
    fi
  fi
}

cmd_status() {
  echo "home: $HOME_WT"
  if [ -e "$HOME_DIR/ENABLED" ]; then echo "enabled: yes"; else echo "enabled: no"; fi
  if [ -e "$HOME_DIR/STOP" ]; then echo "stop file: PRESENT (the cycle in flight ends)"; fi
  if have_session && supervisor_running; then echo "supervisor: running (session $LOOP_SESSION, window loops)"; else echo "supervisor: not running"; fi
  if [ -f "$HOME_DIR/work" ]; then echo "last work check: $(cat "$HOME_DIR/work")"; fi
  if [ -f "$HOME_WT/scripts/loop/state.py" ]; then python3 -I "$HOME_WT/scripts/loop/state.py" summary; fi
}

cmd_attach() {
  have_session || { echo "session not running (task dev)" >&2; exit 1; }
  tmux select-window -t "=$LOOP_SESSION:loops"
  if [ -n "${TMUX:-}" ]; then tmux switch-client -t "=$LOOP_SESSION"; else tmux attach-session -t "=$LOOP_SESSION"; fi
}

case "${1:-}" in
  start) cmd_start ;;
  stop) cmd_stop "${2:-}" ;;
  resume) cmd_resume ;;
  status) cmd_status ;;
  attach) cmd_attach ;;
  *) echo "usage: $0 {start|stop [--now]|resume|status|attach}" >&2; exit 2 ;;
esac
