#!/bin/sh
# Shared by the loop scripts: repo root, config, paths. POSIX sh, so both the
# bash scripts and the sh ones (supervisor, work check, loopctl) can source it.
# Every caller lives in scripts/loop/, so under sh (no BASH_SOURCE) the caller's
# $0 locates the repo just as well.
# shellcheck shell=sh disable=SC2034,SC3028,SC3054
LOOP_ROOT="$(cd "$(dirname "${BASH_SOURCE:-$0}")/../.." && pwd)"
_env_claude="${LOOP_CLAUDE_BIN:-}"
LOOP_DIR="$LOOP_ROOT/.loop"
mkdir -p "$LOOP_DIR"
# shellcheck source=scripts/loop/config.defaults.env
. "$LOOP_ROOT/scripts/loop/config.defaults.env"
if [ -f "$LOOP_DIR/config.env" ]; then
  # shellcheck disable=SC1091
  . "$LOOP_DIR/config.env"
fi
if [ -n "$_env_claude" ]; then LOOP_CLAUDE_BIN="$_env_claude"; fi
LOOP_SESSION="${TREEMAND_SESSION:-treemand_dev}"
# What the agent pane runs; `loopctl start` creates the pane with this command.
LOOP_AGENT_CMD="${LOOP_AGENT_CMD:-task loop:agent}"
LOOP_STOP_FILE="$LOOP_DIR/STOP"
# Desired state: present while the loop is wanted. Survives reboots; only stop clears it.
LOOP_ENABLED_FILE="$LOOP_DIR/ENABLED"
# Written by `state.py end-cycle`; the supervisor consumes it to start the next cycle.
LOOP_CYCLE_DONE_FILE="$LOOP_DIR/cycle-done"
# The supervisor's latest work verdict (one line from work.sh), for the agent and hooks.
LOOP_WORK_FILE="$LOOP_DIR/work"
