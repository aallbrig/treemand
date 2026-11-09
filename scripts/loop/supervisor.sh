#!/usr/bin/env bash
# Keeps the Remote Control loop session alive. Runs in the `agent` pane of the
# `loops` window. Restarts claude if it exits, backing off when it crashes fast,
# and stops for good when .loop/STOP exists.
set -uo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$LOOP_ROOT" || exit 1

backoff=10
while :; do
  if [ -e "$LOOP_STOP_FILE" ]; then
    echo "[loop] STOP file present; supervisor exiting (task loop:start to resume)"
    exit 0
  fi
  started=$(date +%s)
  echo "[loop] $(date -u +%FT%TZ) starting '$LOOP_REMOTE_NAME'"
  "$LOOP_CLAUDE_BIN" --permission-mode auto --settings "$LOOP_ROOT/.claude/loop-settings.json" --remote-control "$LOOP_REMOTE_NAME" "/loop /treemand-loop"
  rc=$?
  ran=$(( $(date +%s) - started ))
  echo "[loop] $(date -u +%FT%TZ) claude exited rc=$rc after ${ran}s"
  [ -e "$LOOP_STOP_FILE" ] && continue
  if [ "$ran" -lt 120 ]; then
    backoff=$(( backoff * 2 > 600 ? 600 : backoff * 2 ))
  else
    backoff=10
  fi
  echo "[loop] restarting in ${backoff}s"
  sleep "$backoff"
done
