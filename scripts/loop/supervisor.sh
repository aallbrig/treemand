#!/bin/sh
# The loop supervisor. `loopctl start` runs it (as LOOP_AGENT_CMD) in the agent
# pane of the loops window, from the loop's home worktree. It runs while
# .loop/ENABLED exists and spends no tokens deciding anything:
#
#   - Work check: work.sh (preflight + tracker, no LLM). With work and no session
#     running, it starts one interactive Remote Control session in the FOREGROUND of
#     this pane, so claude owns the pane's TTY and Remote Control works.
#   - Between cycles: a watcher runs in the background of this same pane (its
#     output goes to .loop/supervisor.log, never the TTY). When the cycle-done
#     marker appears (state.py end-cycle) and work remains, it types `/clear` and
#     then `/treemand-loop` into this pane with tmux send-keys.
#   - Idle: with no work for LOOP_IDLE_EXIT_MINUTES it types `/exit` (stand down).
#     The supervisor keeps polling every LOOP_POLL_SECONDS and relaunches the
#     session when work appears.
#   - Stop: once STOP is present (or ENABLED gone) and the cycle is done, it types
#     `/exit`, then exits itself, and tmux closes the pane.
#
# One pane holds both: the supervisor is the pane's process, so the pane lives
# exactly as long as the supervisor, and the session is its foreground child.
set -u
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "$0")/lib.sh"
cd "$LOOP_ROOT" || exit 1

LOG="$LOOP_DIR/supervisor.log"
pane="${TMUX_PANE:-}"
if [ -z "$pane" ]; then
  echo "[loop] supervisor must run inside a tmux pane (task loop:start creates it)" >&2
  exit 2
fi

# Logs go to the pane (while no session owns it) and always to .loop/supervisor.log.
quiet=""
log() {
  line="[loop] $(date -u +%Y-%m-%dT%H:%M:%SZ) $*"
  echo "$line" >>"$LOG"
  [ -n "$quiet" ] || echo "$line"
}
wanted() { [ -e "$LOOP_ENABLED_FILE" ] && [ ! -e "$LOOP_STOP_FILE" ]; }
has_work() { sh "$LOOP_ROOT/scripts/loop/work.sh" >"$LOOP_WORK_FILE.tmp" 2>>"$LOG" && mv "$LOOP_WORK_FILE.tmp" "$LOOP_WORK_FILE"; }
type_line() { # type one line into the session, then submit it
  tmux send-keys -t "$pane" -l "$1"
  sleep "$LOOP_KEY_DELAY_SECONDS"
  tmux send-keys -t "$pane" Enter
}

# Sleeps up to $1 seconds, waking early once the loop is no longer wanted.
nap() {
  waited=0
  while [ "$waited" -lt "$1" ] && wanted; do
    sleep "$LOOP_TICK_SECONDS"
    waited=$((waited + LOOP_TICK_SECONDS))
  done
}

# Runs in the background while the session is up. Returns after typing /exit.
watch_session() {
  quiet=1 in_cycle=1 idle_since="" last_check=0
  while :; do
    sleep "$LOOP_TICK_SECONDS"
    if [ -e "$LOOP_CYCLE_DONE_FILE" ]; then
      log "cycle done: $(cat "$LOOP_CYCLE_DONE_FILE")"
      rm -f "$LOOP_CYCLE_DONE_FILE"
      # end-cycle runs inside the agent's turn; let the turn finish before typing.
      sleep "$LOOP_SETTLE_SECONDS"
      in_cycle=0 last_check=0
    fi
    [ "$in_cycle" = 1 ] && continue
    if ! wanted; then
      log "stop requested; ending the session"
      type_line /exit
      return
    fi
    now=$(date +%s)
    [ $((now - last_check)) -ge "$LOOP_POLL_SECONDS" ] || continue
    last_check=$now
    if has_work; then
      log "next cycle: $(cat "$LOOP_WORK_FILE")"
      type_line /clear
      sleep "$LOOP_KEY_DELAY_SECONDS"
      type_line /treemand-loop
      in_cycle=1 idle_since=""
      continue
    fi
    [ -n "$idle_since" ] || idle_since=$now
    if [ $((now - idle_since)) -ge $((LOOP_IDLE_EXIT_MINUTES * 60)) ]; then
      log "no work for ${LOOP_IDLE_EXIT_MINUTES}m; standing down"
      type_line /exit
      return
    fi
  done
}

log "supervisor up in $LOOP_ROOT"
sh "$LOOP_ROOT/scripts/loop/recover.sh" 2>>"$LOG" | while IFS= read -r line; do log "recovery: $line"; done

backoff=10
while wanted; do
  if ! has_work; then
    log "no work; next check in ${LOOP_POLL_SECONDS}s"
    nap "$LOOP_POLL_SECONDS"
    continue
  fi
  log "work: $(cat "$LOOP_WORK_FILE"); starting '$LOOP_REMOTE_NAME'"
  rm -f "$LOOP_CYCLE_DONE_FILE"
  watch_session >/dev/null 2>>"$LOG" </dev/null &
  watcher=$!
  started=$(date +%s)
  "$LOOP_CLAUDE_BIN" --permission-mode auto --settings "$LOOP_ROOT/.claude/loop-settings.json" \
    --remote-control "$LOOP_REMOTE_NAME" "/treemand-loop"
  rc=$?
  kill "$watcher" 2>/dev/null
  wait "$watcher" 2>/dev/null
  ran=$(($(date +%s) - started))
  log "session exited rc=$rc after ${ran}s"
  if [ "$rc" -ne 0 ] && [ "$ran" -lt 120 ]; then
    log "session crashed fast; retrying in ${backoff}s"
    nap "$backoff"
    backoff=$((backoff * 2 > 600 ? 600 : backoff * 2))
  else
    backoff=10
  fi
done
log "loop not enabled (or STOP present); supervisor exiting"
