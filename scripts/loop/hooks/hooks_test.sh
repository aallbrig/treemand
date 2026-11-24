#!/bin/sh
# Unit tests for the loop's Claude Code hooks (scripts/loop/hooks/) and their
# wiring in .claude/loop-settings.json. Each hook runs from a throwaway copy of
# the loop scripts, so its .loop/ state is a fixture, with sample hook input on
# stdin. Hooks run under dash when it is installed. Never touches the real .loop/.
# Helpers and $s are used inside check's eval strings.
# shellcheck disable=SC2016,SC2034,SC2329
set -u
here="$(cd "$(dirname "$0")" && pwd)"
src="$(cd "$here/../../.." && pwd)"
for tool in jq python3; do
  command -v "$tool" >/dev/null || { echo "hooks_test: $tool missing, skipping"; exit 0; }
done
shell="sh"
if command -v dash >/dev/null; then shell=dash; fi

tmp="$(cd -P "$(mktemp -d)" && pwd)"
trap 'rm -rf "$tmp"' EXIT
fail=0
check() { if ! eval "$2"; then echo "hooks_test FAIL: $1" >&2; fail=1; fi; }

# --- fixture repo ------------------------------------------------------------
r="$tmp/repo"
mkdir -p "$r/scripts/loop/hooks" "$r/.loop" "$r/docs/design" "$r/docs/loop" "$r/docs/metrics"
cp "$src/scripts/loop/state.py" "$src/scripts/loop/config.defaults.env" "$r/scripts/loop/"
cp "$src"/scripts/loop/hooks/*.sh "$r/scripts/loop/hooks/"
# A stub work check: what it prints is the fixture's selected work.
cat >"$r/scripts/loop/work.sh" <<'EOF'
#!/bin/sh
echo RAN >>"$(dirname "$0")/../../.loop/work-ran"
if [ -n "${FIXTURE_WORK:-}" ]; then echo "$FIXTURE_WORK"; exit 0; fi
echo "${FIXTURE_VERDICT:-}" >&2
exit 1
EOF
cat >"$r/docs/design/20200101_000000_UTC_vision.md" <<'EOF'
# old vision
## Principles
1. **Stale goal.** Should not appear.
EOF
cat >"$r/docs/design/20260101_000000_UTC_vision.md" <<'EOF'
# vision
Intro prose that should not appear.
## Principles
1. **Zero setup, any CLI.** Works on tools that never heard of treemand.
2. **Fast and honest.** Cached trees are instant.
## Where it is going
- Discovery that handles more CLI families with fewer stub nodes (nonstandard
  help layouts, subcommand aliases). Second sentence dropped.
## Out of scope
- Telemetry that leaves the machine.
EOF
{
  echo "# Dev loop learnings"
  echo "## Recent cycles"
  for n in 1 2 3 4; do printf '### Cycle %s - 2026-01-0%s - released - #%s\nlesson %s\n\n' "$n" "$n" "$n" "$n"; done
} >"$r/docs/loop/LEARNINGS.md"
printf '%s\n' '{"date":"2026-01-01T00:00:00Z","sha":"aaa","metrics":{"truth_pass_pct":90}}' \
  '{"date":"2026-01-02T00:00:00Z","sha":"bbb","metrics":{"truth_pass_pct":98.4,"unexpected_misses":0}}' >"$r/docs/metrics/parser.jsonl"
jq -nc '{date:"2026-01-03T00:00:00Z",sha:"ccc",metrics:([range(20)|{key:"m\(.)",value:.}]|from_entries)}' >"$r/docs/metrics/perf.jsonl"

hook() { # hook <name> <stdin-json>: run a hook, stdout to $tmp/out, exit code to $tmp/rc
  printf '%s' "$2" | "$shell" "$r/scripts/loop/hooks/$1.sh" >"$tmp/out" 2>"$tmp/err"
  echo $? >"$tmp/rc"
  # bash's sh mode keeps assignments made before a function call; drop them.
  unset FIXTURE_WORK FIXTURE_VERDICT
}
rc() { cat "$tmp/rc"; }
out() { cat "$tmp/out"; }
ctx() { jq -r '.hookSpecificOutput.additionalContext' "$tmp/out"; }
has() { ctx | grep -qF -- "$1"; }
state() { python3 -I "$r/scripts/loop/state.py" "$@" >/dev/null; }

# --- SessionStart --------------------------------------------------------------
start='{"hook_event_name":"SessionStart","source":"clear"}'
FIXTURE_WORK='ready #42 Fix the thing' hook session_start "$start"
check "session_start exits 0" '[ "$(rc)" = 0 ]'
check "session_start emits SessionStart JSON" '[ "$(jq -r .hookSpecificOutput.hookEventName "$tmp/out")" = SessionStart ]'
check "stale work file: runs the work check" '[ -f "$r/.loop/work-ran" ]'
check "verdict GO with selected work" 'has "Preflight: GO" && has "ready #42 Fix the thing"'
check "vision goals from the newest vision" 'has "Zero setup, any CLI." && has "Fast and honest." && ! has "Stale goal"'
check "vision bullets are one sentence, continuation joined" 'has "fewer stub nodes (nonstandard help layouts, subcommand aliases)." && ! has "Second sentence"'
check "vision skips prose and keeps out of scope" '! has "Intro prose" && has "Telemetry that leaves the machine."'
check "last three learnings" 'has "Cycle 2 -" && has "Cycle 4 -" && has "lesson 4" && ! has "Cycle 1 -"'
check "newest metric line per probe" 'has "parser 2026-01-02 bbb: truth_pass_pct=98.4 unexpected_misses=0" && ! has "truth_pass_pct=90"'
check "long metric lines truncated" 'has "(+12 more)" && ! has "m19="'
check "context stays under ~2k tokens" '[ "$(ctx | wc -c)" -lt 8000 ]'

rm -f "$r/.loop/work-ran"
echo 'in-progress #7' >"$r/.loop/work"
FIXTURE_WORK='ready #42 Fix the thing' hook session_start "$start"
check "fresh work file: reused, no work check" '[ ! -f "$r/.loop/work-ran" ] && has "in-progress #7"'

echo 'FIX_MAIN:CI on main is failure' >"$r/.loop/work"
hook session_start "$start"
check "FIX_MAIN verdict" 'has "Preflight: FIX_MAIN:CI on main is failure"'

rm -f "$r/.loop/work"
FIXTURE_VERDICT='HALT:daily cycle cap reached (8)' hook session_start "$start"
check "HALT verdict from the work check" 'has "Preflight: HALT:daily cycle cap reached (8)" && has "Work: none"'

hook session_start "$start"
check "no work, no verdict: GO with no work" 'has "Preflight: GO" && has "Work: none"'

rm -f "$r/.loop/work-ran"
touch "$r/.loop/STOP"
FIXTURE_WORK='ready #42 Fix the thing' hook session_start "$start"
check "STOP short-circuits the work check" 'has "Preflight: STOP" && [ ! -f "$r/.loop/work-ran" ]'
rm -f "$r/.loop/STOP"

# --- Stop -----------------------------------------------------------------------
stop='{"hook_event_name":"Stop","stop_hook_active":false}'
hook stop_check "$stop"
check "no state yet: allow" '[ "$(rc)" = 0 ] && [ -z "$(out)" ]'
state begin-cycle
hook stop_check "$stop"
check "open cycle: block" '[ "$(rc)" = 0 ] && [ "$(jq -r .decision "$tmp/out")" = block ]'
check "block reason names end-cycle" 'jq -r .reason "$tmp/out" | grep -q "state.py end-cycle"'
hook stop_check '{"hook_event_name":"Stop","stop_hook_active":true}'
check "open cycle, stop_hook_active: still blocks" '[ "$(jq -r .decision "$tmp/out")" = block ]'
state end-cycle parked
check "end-cycle wrote the marker" '[ -f "$r/.loop/cycle-done" ]'
hook stop_check "$stop"
check "ended cycle: allow" '[ "$(rc)" = 0 ] && [ -z "$(out)" ]'
check "marker left for the supervisor" '[ -f "$r/.loop/cycle-done" ]'
rm -f "$r/.loop/cycle-done"
hook stop_check "$stop"
check "ended cycle, marker consumed: allow" '[ -z "$(out)" ]'

# --- PostToolUse ------------------------------------------------------------------
post='{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"ls"}}'
rm -f "$r/.loop/heartbeat"
hook post_tool "$post"
check "no STOP: silent" '[ "$(rc)" = 0 ] && [ -z "$(out)" ]'
check "heartbeat touched" '[ -f "$r/.loop/heartbeat" ]'
touch "$r/.loop/STOP"
hook post_tool "$post"
check "STOP: PostToolUse context" '[ "$(jq -r .hookSpecificOutput.hookEventName "$tmp/out")" = PostToolUse ]'
check "STOP: cleanup instructions" 'has "a stop was requested" && has "end-cycle stopped" && has "stopped"'
rm -f "$r/.loop/STOP"
echo "30 1000000 1" >"$r/.loop/context"
hook post_tool "$post"
check "context under the note threshold: silent" '[ -z "$(out)" ]'
echo "55 1000000 1" >"$r/.loop/context"
hook post_tool "$post"
check "context over the note threshold: one note" 'has "used 55% of its context" && has "At 75%"'
hook post_tool "$post"
check "context note only once" '[ -z "$(out)" ]'
hook session_start "$start"
check "a new context re-arms the note" '[ ! -e "$r/.loop/context-noted" ] && [ ! -e "$r/.loop/context" ]'

# --- status line ---------------------------------------------------------------------
echo '{"context_window":{"used_percentage":42.7,"context_window_size":1000000}}' | "$shell" "$r/scripts/loop/hooks/statusline.sh" >"$tmp/out"
check "status line records the context" '[ "$(cut -d" " -f1-2 "$r/.loop/context")" = "42 1000000" ]'
check "status line prints the percentage" 'grep -q "context 42%" "$tmp/out"'
rm -f "$r/.loop/context"
echo '{"context_window":{"used_percentage":null}}' | "$shell" "$r/scripts/loop/hooks/statusline.sh" >"$tmp/out"
check "no usage yet: nothing recorded" '[ ! -e "$r/.loop/context" ] && grep -q "treemand loop" "$tmp/out"'

# --- PreCompact ------------------------------------------------------------------------
echo 'in-progress #7' >"$r/.loop/work"
hook pre_compact '{"hook_event_name":"PreCompact","trigger":"auto"}'
check "pre_compact keeps the work line" '[ "$(rc)" = 0 ] && grep -q "work line: in-progress #7" "$tmp/out" && grep -q "branch" "$tmp/out"'

# --- Stop: unsaved work in the issue's worktree ---------------------------------------
gitc() { git -c user.name=t -c user.email=t@example.invalid -c init.defaultBranch=main "$@"; }
o="$tmp/origin.git"
gitc init -q --bare "$o"
gitc -C "$r" init -q && gitc -C "$r" commit -q --allow-empty -m base && gitc -C "$r" remote add origin "$o" &&
  gitc -C "$r" push -q origin HEAD:main && gitc -C "$r" fetch -q origin
gitc -C "$r" worktree add -q -b loop/7-x "$r/.loop/worktrees/7" origin/main 2>/dev/null
gitc -C "$r/.loop/worktrees/7" branch -q --unset-upstream 2>/dev/null
state end-cycle parked && rm -f "$r/.loop/cycle-done"
hook stop_check "$stop"
check "clean worktree, no open cycle: allow" '[ -z "$(out)" ]'
echo x >"$r/.loop/worktrees/7/new.txt"
hook stop_check "$stop"
check "uncommitted work: block" '[ "$(jq -r .decision "$tmp/out")" = block ] && jq -r .reason "$tmp/out" | grep -q "uncommitted changes"'
hook stop_check '{"hook_event_name":"Stop","stop_hook_active":true}'
check "uncommitted work: blocks only once" '[ -z "$(out)" ]'
gitc -C "$r/.loop/worktrees/7" add -A && gitc -C "$r/.loop/worktrees/7" commit -q -m wip
hook stop_check "$stop"
check "unpushed branch: block" 'jq -r .reason "$tmp/out" | grep -q "never pushed"'
gitc -C "$r/.loop/worktrees/7" push -q -u origin loop/7-x
hook stop_check "$stop"
check "pushed branch: allow" '[ -z "$(out)" ]'
echo y >>"$r/.loop/worktrees/7/new.txt"
gitc -C "$r/.loop/worktrees/7" commit -q -am more
hook stop_check "$stop"
check "commits ahead of upstream: block" 'jq -r .reason "$tmp/out" | grep -q "not pushed to origin/loop/7-x"'
gitc -C "$r/.loop/worktrees/7" push -q

# --- Stop: an open cycle blocks up to LOOP_STOP_BLOCKS times -------------------------
state begin-cycle
again='{"hook_event_name":"Stop","stop_hook_active":true}'
hook stop_check "$stop"; hook stop_check "$again"; hook stop_check "$again"
check "third consecutive block" '[ "$(jq -r .decision "$tmp/out")" = block ]'
hook stop_check "$again"
check "fourth: lets the turn end" '[ -z "$(out)" ]'
hook stop_check "$stop"
check "a fresh stop blocks again" '[ "$(jq -r .decision "$tmp/out")" = block ]'
state end-cycle parked && rm -f "$r/.loop/cycle-done"

# --- settings wiring ------------------------------------------------------------------
s="$src/.claude/loop-settings.json"
check "settings is valid JSON" 'jq -e . "$s" >/dev/null'
for pair in SessionStart:session_start Stop:stop_check PostToolUse:post_tool PreToolUse:guard PreCompact:pre_compact; do
  ev="${pair%%:*}" name="${pair#*:}"
  check "$ev runs $name" 'jq -e --arg e "$ev" --arg n "/$name.sh" "[.hooks[\$e][].hooks[] | select(.type == \"command\" and (.command | endswith(\$n)))] | length == 1" "$s" >/dev/null'
done
check "PostToolUse post_tool has no matcher" 'jq -e "[.hooks.PostToolUse[] | select(has(\"matcher\") | not)] | length == 1" "$s" >/dev/null'
check "PreToolUse guard keeps its matcher" 'jq -e ".hooks.PreToolUse[0].matcher == \"Bash|Edit|Write|NotebookEdit\"" "$s" >/dev/null'
check "status line runs statusline.sh" 'jq -r .statusLine.command "$s" | grep -q "scripts/loop/hooks/statusline.sh"'
p="$src/.claude/settings.json"
check "project settings run go_feedback on Edit|Write" 'jq -e "[.hooks.PostToolUse[] | select(.matcher == \"Edit|Write\") | .hooks[].command | select(endswith(\"/go_feedback.sh\"))] | length == 1" "$p" >/dev/null'
check "loop settings do not repeat go_feedback" '! grep -q go_feedback "$s"'
check "every hook command points at an existing script" 'jq -r ".hooks[][].hooks[].command" "$s" | sed "s|\"\$CLAUDE_PROJECT_DIR\"|$src|" | while read -r p; do [ -f "$p" ] || exit 1; done'

# --- go_feedback -------------------------------------------------------------------------
gf() { jq -nc --arg p "$1" '{hook_event_name:"PostToolUse",tool_name:"Edit",tool_input:{file_path:$p}}'; }
if command -v gofmt >/dev/null; then
  mkdir -p "$tmp/gomod"
  printf 'package x\n\nfunc A() int { return 1 }\n' >"$tmp/gomod/ok.go"
  printf 'package x\nfunc  B( ) {}\n' >"$tmp/gomod/ugly.go"
  printf 'package x\nfunc C( {\n' >"$tmp/gomod/broken.go"
  echo hi >"$tmp/gomod/notes.md"
  export GO_FEEDBACK_COGNIT=0
  hook go_feedback "$(gf "$tmp/gomod/ok.go")"
  check "formatted Go file: silent" '[ "$(rc)" = 0 ] && [ -z "$(out)" ]'
  hook go_feedback "$(gf "$tmp/gomod/ugly.go")"
  check "unformatted Go file: gofmt note" 'has "is not formatted"'
  hook go_feedback "$(gf "$tmp/gomod/broken.go")"
  check "unparsable Go file: parse note" 'has "does not parse"'
  hook go_feedback "$(gf "$tmp/gomod/notes.md")"
  check "non-Go file: silent" '[ "$(rc)" = 0 ] && [ -z "$(out)" ]'
fi

[ "$fail" = 0 ] && echo "hooks_test: ok"
exit "$fail"
