#!/bin/sh
# Manage the treemand_dev tmuxinator session.
#   up       attach if the session exists, else create it and attach (default)
#   attach   attach; fail if absent
#   start    create detached; no-op if it exists (safe at login: autostart runs it)
#   stop     kill every pane's process tree, then the session
#   recycle  stop, then start detached
set -eu

SESSION="${TREEMAND_SESSION:-treemand_dev}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONFIG="$ROOT/.tmuxinator/treemand.yml"
TREEMAND_ROOT="$ROOT"
TREEMAND_SESSION="$SESSION"
export TREEMAND_ROOT TREEMAND_SESSION

exists() { tmux has-session -t "=$SESSION" 2>/dev/null; }

kill_tree() {
  for child in $(pgrep -P "$1" 2>/dev/null || true); do
    kill_tree "$child"
  done
  kill "$1" 2>/dev/null || true
}

start() {
  if exists; then
    echo "session '$SESSION' already running"
    return 0
  fi
  (cd "$ROOT" && tmuxinator start --project-config "$CONFIG" --no-attach)
  echo "session '$SESSION' started"
}

attach() {
  if [ -n "${TMUX:-}" ]; then
    tmux switch-client -t "=$SESSION"
  else
    tmux attach-session -t "=$SESSION"
  fi
}

stop() {
  if ! exists; then
    echo "session '$SESSION' not running"
    return 0
  fi
  for pid in $(tmux list-panes -s -t "=$SESSION" -F '#{pane_pid}'); do
    kill_tree "$pid"
  done
  tmux kill-session -t "=$SESSION" 2>/dev/null || true
  echo "session '$SESSION' stopped"
}

case "${1:-up}" in
  up)
    exists || start
    attach
    ;;
  attach)
    exists || { echo "session '$SESSION' is not running (task dev)" >&2; exit 1; }
    attach
    ;;
  start) start ;;
  stop) stop ;;
  recycle)
    stop
    start
    ;;
  *)
    echo "usage: $0 {up|attach|start|stop|recycle}" >&2
    exit 2
    ;;
esac
