#!/bin/sh
# Behaviour test for the loop runtime: loopctl (start, stop, resume), the home
# worktree, the supervisor (work check, cycle hand-off, idle stand-down, relaunch,
# stop) and crash recovery. Everything runs in a temp dir on an ISOLATED tmux
# server against a throwaway git repo, with fake `gh` and `claude` on PATH. It
# never touches the live tmux server, the real repo, GitHub or a real claude.
# POSIX sh: run it with dash too.
# shellcheck disable=SC2016,SC2317,SC2329
set -u
here="$(cd "$(dirname "$0")" && pwd)"
src="$(cd "$here/../.." && pwd)"
for tool in tmux git jq python3; do
  command -v "$tool" >/dev/null || { echo "supervisor_test: $tool missing, skipping"; exit 0; }
done

tmp="$(cd -P "$(mktemp -d)" && pwd)"
S=treemand_dev
export TMUX_TMPDIR="$tmp" TREEMAND_SESSION="$S" FAKE_DIR="$tmp/fake"
unset TMUX TMUX_PANE LOOP_HOME LOOP_CLAUDE_BIN
export LOOP_AGENT_CMD="sh scripts/loop/supervisor.sh"
mkdir -p "$FAKE_DIR" "$tmp/bin" "$tmp/src"
export PATH="$tmp/bin:$PATH"
cleanup() {
  tmux kill-server >/dev/null 2>&1
  pkill -f "$tmp/bin/claude" 2>/dev/null
  rm -rf "$tmp"
}
trap cleanup EXIT
trap 'exit 1' INT TERM

# --- fakes -------------------------------------------------------------------
# gh: issues live in files under $FAKE_DIR; every call is logged.
cat >"$tmp/bin/gh" <<'EOF'
#!/bin/sh
d="$FAKE_DIR"
echo "$*" >>"$d/gh.log"
case "$1 $2" in
  "api user") echo loopbot ;;
  "run list") echo success ;;
  "api --paginate")
    case "$*" in
      *"issues?"*loop:ready*) jq -c '[.]' "$d/ready.json" 2>/dev/null || echo '[[]]' ;;
      *) echo '[[]]' ;;
    esac ;;
  "issue list")
    case "$*" in
      *loop:in-progress*) cat "$d/in-progress" 2>/dev/null ;;
      *) echo '[]' ;;
    esac ;;
  "issue edit")
    case "$*" in *"--remove-label loop:in-progress"*)
      grep -vx "$3" "$d/in-progress" >"$d/ip.tmp"; mv "$d/ip.tmp" "$d/in-progress" ;;
    esac ;;
  "issue view") echo loopbot ;;
  *) echo "fake gh: unhandled: $*" >&2; exit 1 ;;
esac
EOF
# claude: logs its args and each line typed into the pane. The initial prompt and
# each typed /treemand-loop run one fake cycle: wait while $FAKE_DIR/hold exists
# (using tools, so the heartbeat stays fresh), ship the first ready issue (or stop
# if STOP is present), then end-cycle. With $FAKE_DIR/hang the cycle goes quiet
# (no tool use, no end); with $FAKE_DIR/fat it reports 80% context and waits. A
# typed park instruction parks the cycle, unless $FAKE_DIR/deaf exists.
cat >"$tmp/bin/claude" <<'EOF'
#!/bin/sh
log="$FAKE_DIR/claude.log"
echo "START $*" >>"$log"
echo "ENV autocompact=${CLAUDE_AUTOCOMPACT_PCT_OVERRIDE:-}" >>"$log"
beat() { : >.loop/heartbeat; }
ship() { jq -c '.[1:]' "$FAKE_DIR/ready.json" >"$FAKE_DIR/ready.tmp" && mv "$FAKE_DIR/ready.tmp" "$FAKE_DIR/ready.json"; }
cycle() {
  echo CYCLE >>"$log"
  if [ -e "$FAKE_DIR/fat" ]; then beat; echo "80 1000000 0" >.loop/context; return; fi
  if [ -e "$FAKE_DIR/hang" ]; then return; fi
  while [ -e "$FAKE_DIR/hold" ]; do beat; sleep 1; done
  if [ -e .loop/STOP ]; then python3 -I scripts/loop/state.py end-cycle stopped; return; fi
  ship
  python3 -I scripts/loop/state.py end-cycle released
}
case "$*" in *" /treemand-loop") cycle ;; esac
while IFS= read -r line; do
  echo "KEY $line" >>"$log"
  case "$line" in
    /exit) echo EXIT >>"$log"; exit 0 ;;
    /treemand-loop) cycle ;;
    *"park this cycle now"*)
      if [ ! -e "$FAKE_DIR/deaf" ]; then echo PARKED >>"$log"; ship; python3 -I scripts/loop/state.py end-cycle parked; fi ;;
  esac
done
EOF
chmod +x "$tmp/bin/gh" "$tmp/bin/claude"
ready() { # ready N... : the loop:ready queue, oldest first
  printf '%s\n' "$@" | jq -R 'select(. != "") | {number: tonumber, title: "issue \(.)", user: {login: "maintainer"}, labels: [], created_at: "2026-01-01T00:00:00Z"}' | jq -sc . >"$FAKE_DIR/ready.json"
}
ready
: >"$FAKE_DIR/in-progress"

# --- a throwaway repo with this checkout's loop scripts on its origin/main --
g() { git -c user.name=test -c user.email=test@example.invalid -c init.defaultBranch=main "$@"; }
g init -q --bare "$tmp/origin.git"
repo="$tmp/src/repo"
home="$tmp/src/repo.loop"
g clone -q "$tmp/origin.git" "$repo" 2>/dev/null
mkdir -p "$repo/scripts" "$repo/.claude"
cp -R "$src/scripts/loop" "$repo/scripts/loop"
cp "$src/.claude/loop-settings.json" "$repo/.claude/"
cp "$src/.gitignore" "$repo/.gitignore"
g -C "$repo" add -A && g -C "$repo" commit -qm init && g -C "$repo" push -q origin HEAD:main 2>/dev/null
mkdir -p "$repo/.loop"
cat >"$repo/.loop/config.env" <<EOF
LOOP_CLAUDE_BIN=$tmp/bin/claude
LOOP_TRUSTED_AUTHORS=maintainer
LOOP_POLL_SECONDS=1
LOOP_TICK_SECONDS=1
LOOP_IDLE_EXIT_MINUTES=0
LOOP_KEY_DELAY_SECONDS=0
LOOP_SETTLE_SECONDS=0
LOOP_STALL_SECONDS=3
LOOP_STALL_STEP_SECONDS=2
LOOP_NOTIFY=0
EOF

# --- helpers -----------------------------------------------------------------
fail=0
check() { if ! eval "$2"; then echo "supervisor_test FAIL: $1" >&2; fail=1; fi; }
wait_for() { i=0; while [ "$i" -lt 60 ]; do eval "$1" && return 0; sleep 0.5; i=$((i + 1)); done; return 1; }
loopctl() { sh "$repo/scripts/loop/loopctl.sh" "$@"; }
agent_panes() { tmux list-panes -s -t "=$S" -F '#{@treemand_role}' | grep -c '^agent$'; }
agent_id() { tmux list-panes -s -t "=$S" -F '#{pane_id} #{@treemand_role}' | awk '$2=="agent"{print $1; exit}'; }
count() { if [ -f "$FAKE_DIR/claude.log" ]; then grep -c "$1" "$FAKE_DIR/claude.log"; else echo 0; fi; }
claude_running() { pgrep -f "$tmp/bin/claude" >/dev/null; }
supervisor_says() { grep -q "$1" "$home/.loop/supervisor.log" 2>/dev/null; }

tmux new-session -d -s "$S" -n loops -x 200 -y 50

# --- start with no work ------------------------------------------------------
loopctl start >/dev/null
check "start creates the home worktree beside the checkout" '[ -d "$home/.git" ] || [ -f "$home/.git" ]'
check "home is detached at origin/main" '! git -C "$home" symbolic-ref -q HEAD >/dev/null && [ "$(git -C "$home" rev-parse HEAD)" = "$(git -C "$repo" rev-parse origin/main)" ]'
check "start writes ENABLED in the home" '[ -e "$home/.loop/ENABLED" ]'
check "home gets the host loop config" 'cmp -s "$repo/.loop/config.env" "$home/.loop/config.env"'
check "start creates one agent pane" '[ "$(agent_panes)" = 1 ]'
check "agent pane runs in the home" '[ "$(tmux display-message -p -t "$(agent_id)" "#{pane_current_path}")" = "$home" ]'
check "no work: supervisor polls without a session" 'wait_for "supervisor_says \"no work\"" && ! claude_running && [ "$(count START)" = 0 ]'
loopctl start >/dev/null
check "second start adds no pane" '[ "$(agent_panes)" = 1 ]'
check "status reports enabled and running" 'out="$(loopctl status)"; case "$out" in *"enabled: yes"*"supervisor: running"*) true ;; *) false ;; esac'
check "the checkout is untouched" '[ -z "$(git -C "$repo" status --porcelain)" ] && [ ! -e "$repo/.loop/ENABLED" ]'

# --- work appears: launch, cycle hand-off, idle stand-down -------------------
ready 5 6
check "work launches the Remote Control session" 'wait_for "[ \"\$(count START)\" = 1 ]"'
check "session runs interactive with remote control and the skill" 'grep -q -- "^START --permission-mode auto --settings .*/loop-settings.json --remote-control treemand.dev dev loop /treemand-loop$" "$FAKE_DIR/claude.log"'
check "idle after the queue empties: /exit sent" 'wait_for "[ \"\$(count EXIT)\" = 1 ]"'
check "two cycles ran" '[ "$(count CYCLE)" = 2 ]'
check "cycle hand-off typed /clear then /treemand-loop once" '[ "$(count "KEY /clear")" = 1 ] && [ "$(count "KEY /treemand-loop")" = 1 ] && [ "$(grep KEY "$FAKE_DIR/claude.log" | tr "\n" ,)" = "KEY /clear,KEY /treemand-loop,KEY /exit," ]'
check "standing down keeps the loop enabled and the pane" '! claude_running && [ -e "$home/.loop/ENABLED" ] && [ "$(agent_panes)" = 1 ]'
check "stand-down is logged" 'supervisor_says "standing down"'

# --- a new session start (e.g. login) resumes an enabled loop ----------------
tmux kill-pane -t "$(agent_id)"
check "pane gone after a simulated reboot" 'wait_for "[ \"\$(agent_panes)\" = 0 ]"'
loopctl resume >/dev/null
check "resume brings back the agent pane when enabled" '[ "$(agent_panes)" = 1 ]'

# --- relaunch on new work, then stop mid-cycle -------------------------------
touch "$FAKE_DIR/hold"
ready 9
check "new work relaunches the session" 'wait_for "[ \"\$(count START)\" = 2 ] && [ \"\$(count CYCLE)\" = 3 ]"'
loopctl stop >/dev/null
check "stop removes ENABLED and writes STOP" '[ ! -e "$home/.loop/ENABLED" ] && [ -e "$home/.loop/STOP" ]'
sleep 3
check "stop lets the cycle in flight finish" 'claude_running && [ "$(agent_panes)" = 1 ]'
rm -f "$FAKE_DIR/hold"
check "after cycle-done the session gets /exit" 'wait_for "[ \"\$(count EXIT)\" = 2 ]"'
check "then the supervisor exits and the pane closes" 'wait_for "[ \"\$(agent_panes)\" = 0 ]" && ! claude_running'
check "the stopped cycle is recorded" 'python3 -I "$home/scripts/loop/state.py" summary | grep -q "last outcome: stopped"'
loopctl resume >/dev/null
check "resume does nothing once stopped" '[ "$(agent_panes)" = 0 ]'

# --- recovery: resume a clean claimed issue, reset a dirty one ---------------
g -C "$home" worktree add -q -b loop/7-clean "$home/.loop/worktrees/7" origin/main
g -C "$home" worktree add -q -b loop/8-dirty "$home/.loop/worktrees/8" origin/main
echo wip >"$home/.loop/worktrees/8/half-done.txt"
printf '7\n8\n' >"$FAKE_DIR/in-progress"
ready 11
touch "$FAKE_DIR/hold"
: >"$FAKE_DIR/gh.log"
loopctl start >/dev/null
check "start clears STOP" '[ ! -e "$home/.loop/STOP" ]'
check "recovery resumes the clean in-progress issue" 'wait_for "[ \"\$(count START)\" = 3 ]" && grep -qx "in-progress #7" "$home/.loop/work"'
check "recovery resets the dirty one to loop:ready" 'grep -q "^issue edit 8 --remove-label loop:in-progress --remove-assignee loopbot --add-label loop:ready" "$FAKE_DIR/gh.log" && [ ! -e "$home/.loop/worktrees/8" ]'
check "the resumed issue is left alone" '! grep -q "^issue edit 7" "$FAKE_DIR/gh.log" && [ -d "$home/.loop/worktrees/7" ] && [ "$(cat "$FAKE_DIR/in-progress")" = 7 ]'
check "recovery is logged" 'supervisor_says "recovery: resume #7" && supervisor_says "recovery: reset #8 worktree has uncommitted changes"'

loopctl stop --now >/dev/null
check "stop --now kills the session and closes the pane" 'wait_for "[ \"\$(agent_panes)\" = 0 ] && ! claude_running"'
check "stop --now disables the loop" '[ ! -e "$home/.loop/ENABLED" ]'

# --- a stalled session: nudge, park, /exit, then relaunch with recovery -----------
rm -f "$FAKE_DIR/hold"
: >"$FAKE_DIR/in-progress"
g -C "$home" worktree remove --force "$home/.loop/worktrees/7" 2>/dev/null
: >"$FAKE_DIR/claude.log"
touch "$FAKE_DIR/hang" "$FAKE_DIR/deaf"
ready 21
loopctl start >/dev/null
check "stall: the agent is nudged" 'wait_for "grep -q \"^KEY treemand loop supervisor: no tool use\" \"$FAKE_DIR/claude.log\""'
check "stall: then told to park" 'wait_for "grep -q \"treemand loop supervisor: park this cycle now (no tool use\" \"$FAKE_DIR/claude.log\""'
check "stall: then the session is ended" 'wait_for "[ \"\$(count EXIT)\" -ge 1 ]"'
rm -f "$FAKE_DIR/hang" "$FAKE_DIR/deaf"
check "stall: escalation is logged" 'supervisor_says "stalled: no tool use" && supervisor_says "ending the session"'
check "stall: the relaunched session ships the issue" 'wait_for "[ \"\$(jq length \"$FAKE_DIR/ready.json\")\" = 0 ]" && wait_for "! claude_running"'
check "stall: the marker is consumed by recovery" '[ ! -e "$home/.loop/stalled" ]'

# --- a full context: park once ---------------------------------------------------------
: >"$FAKE_DIR/claude.log"
touch "$FAKE_DIR/fat"
ready 22
check "context: the session starts" 'wait_for "[ \"\$(count START)\" = 1 ]"'
check "context: the cycle is parked at 80%" 'wait_for "grep -q \"^KEY treemand loop supervisor: park this cycle now (context at 80%)\" \"$FAKE_DIR/claude.log\"" && wait_for "[ \"\$(count PARKED)\" = 1 ]"'
rm -f "$FAKE_DIR/fat"
check "context: parked once, then idle" 'wait_for "[ \"\$(count EXIT)\" = 1 ]" && [ "$(count "park this cycle")" = 1 ] && [ "$(count "no tool use")" = 0 ]'
check "context: park is logged" 'supervisor_says "context at 80%; parking the cycle"'
check "the session gets the auto-compact threshold" 'grep -qx "ENV autocompact=85" "$FAKE_DIR/claude.log"'

loopctl stop --now >/dev/null
check "stop --now again" 'wait_for "[ \"\$(agent_panes)\" = 0 ] && ! claude_running"'

if [ "$fail" = 0 ]; then echo "supervisor_test: ok"; else
  echo "--- supervisor.log" >&2; cat "$home/.loop/supervisor.log" >&2 2>/dev/null
  echo "--- claude.log" >&2; cat "$FAKE_DIR/claude.log" >&2 2>/dev/null
fi
exit "$fail"
