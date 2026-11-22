#!/bin/sh
# What else is running on this machine before touching real infrastructure:
# tmux sessions, listeners on the dev ports (1313 hugo, 18080 site audit),
# git worktrees and the dev loop's state.
cd "$(dirname "$0")/.." || exit 1
tmux ls 2>/dev/null || echo "no tmux sessions"
listeners="$(lsof -nP -iTCP -sTCP:LISTEN 2>/dev/null || ss -ltnp 2>/dev/null)"
printf '%s\n' "$listeners" | grep -E ':(1313|18080)\b' || echo "dev ports free"
git worktree list
sh scripts/loop/loopctl.sh status
