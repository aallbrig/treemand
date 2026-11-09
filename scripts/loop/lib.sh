#!/usr/bin/env bash
# Shared by the loop scripts: repo root, config, paths.
# shellcheck shell=bash disable=SC2034
LOOP_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
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
LOOP_SESSION="${TREEMAND_SESSION:-treemand}"
LOOP_STOP_FILE="$LOOP_DIR/STOP"
