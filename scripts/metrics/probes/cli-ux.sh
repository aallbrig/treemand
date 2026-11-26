#!/bin/sh
# Probe: treemand's own CLI against rules taken from clig.dev. It builds
# treemand (or uses CLI_UX_BIN) and runs it from an empty directory with a
# throwaway HOME and cache; the only CLI it explores is treemand itself.
#   metrics: check.<rule>            1 when the rule holds, 0 when it does not
#            failing_checks          rules that do not hold
#            passing_checks          rules that hold (not judged)
#            undescribed_subcommands subcommands listed with no short description
#   details: the failing checks with evidence, every subcommand walked, and
#            every check with its evidence
#   skipped: the TTY checks when script(1) is missing
# The rules, and how to judge what this cannot check, are in
# .claude/skills/treemand-cli-ux/SKILL.md.
set -eu
root="$(cd "$(dirname "$0")/../../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/run" "$work/home/.config" "$work/cache" "$work/io"
if [ -n "${CLI_UX_BIN:-}" ]; then
  cp "$CLI_UX_BIN" "$work/bin/treemand"
else
  (cd "$root/cli/treemand" && go build -o "$work/bin/treemand" .)
fi
PATH="$work/bin:$PATH" HOME="$work/home" XDG_CONFIG_HOME="$work/home/.config"
TREEMAND_CACHE_DIR="$work/cache" TERM=xterm-256color
export PATH HOME XDG_CONFIG_HOME TREEMAND_CACHE_DIR TERM
unset NO_COLOR CLICOLOR_FORCE FORCE_COLOR 2>/dev/null || true
cd "$work/run"
io="$work/io"
esc="$(printf '\033')"
cr="$(printf '\r')"
tree="--no-cache --depth=1 treemand" # the tree every output check renders

# run <id> <args...>: stdout, stderr and exit code land in $io/<id>.{out,err,rc}.
run() {
  id=$1
  shift
  rc=0
  treemand "$@" >"$io/$id.out" 2>"$io/$id.err" </dev/null || rc=$?
  echo "$rc" >"$io/$id.rc"
}
rc() { cat "$io/$1.rc"; }
has_out() { [ -s "$io/$1.out" ]; }
has_err() { [ -s "$io/$1.err" ]; }
failed() { [ "$(rc "$1")" != 0 ]; }
sgr() { grep -q "$esc\[[0-9;]*m" "$1"; } # an ANSI color (SGR) sequence
ev() { # the evidence for one run: exit code, stdout size, first stderr lines
  printf 'treemand %s: exit=%s stdout=%sB stderr: %s' "$2" "$(rc "$1")" \
    "$(wc -c <"$io/$1.out" | tr -d ' ')" "$(head -n 3 "$io/$1.err" | tr '\t\n\r' '   ')"
}
# check <name> <evidence> <command...>: the rule holds when the command succeeds.
check() {
  name=$1 evidence=$2
  shift 2
  if "$@"; then pass=1; else pass=0; fi
  printf '%s\t%s\t%s\n' "$name" "$pass" "$evidence" >>"$work/checks.tsv"
}
: >"$work/checks.tsv"
: >"$work/skipped.tsv"
: >"$work/subcommands.tsv"

# --help and -h: exit 0, help on stdout (usage and examples), nothing on stderr.
help_ok() { [ "$(rc "$1")" = 0 ] && ! has_err "$1" && grep -q '^Usage:' "$io/$1.out" && grep -qi '^Examples:' "$io/$1.out"; }
run help --help
check help_long_flag "$(ev help --help)" help_ok help
run h -h
check help_short_flag "$(ev h -h)" help_ok h

# --version: one line on stdout, the same as `treemand version`.
run ver --version
run vercmd version
version_ok() { [ "$(rc ver)" = 0 ] && ! has_err ver && [ "$(grep -c . "$io/ver.out")" = 1 ] &&
  grep -q '^treemand ' "$io/ver.out" && cmp -s "$io/ver.out" "$io/vercmd.out"; }
check version_flag "$(ev ver --version)" version_ok

# A mistake in the invocation: non-zero, nothing on stdout, the error on
# stderr, and the closest valid flag or subcommand suggested.
errors() { failed "$1" && ! has_out "$1" && has_err "$1"; }
# shellcheck disable=SC2086
run flag --outptu=json $tree
check unknown_flag_errors "$(ev flag "--outptu=json $tree")" errors flag
check unknown_flag_suggests "$(ev flag "--outptu=json $tree")" grep -q -- '--output' "$io/flag.err"
run sub cache lst
run rootsub confg
check unknown_subcommand_errors "$(ev sub "cache lst")" errors sub
both_suggest() { grep -q 'list' "$io/sub.err" && grep -q 'config' "$io/rootsub.err"; }
check unknown_subcommand_suggests "$(ev sub "cache lst") | $(ev rootsub confg)" both_suggest
# Usage errors exit 2 (cmd/exitcodes.go): a missing CLI name, missing arguments.
run noargs
run setnoargs config set
exit2() { [ "$(rc flag)" = 2 ] && [ "$(rc noargs)" = 2 ] && [ "$(rc setnoargs)" = 2 ]; }
check usage_errors_exit_2 "$(ev setnoargs "config set") | $(ev noargs '')" exit2

# Not a TTY: no color and no spinner or progress on stdout or stderr.
# shellcheck disable=SC2086
run pipe $tree
no_ansi() { [ "$(rc pipe)" = 0 ] && ! sgr "$io/pipe.out" && ! sgr "$io/pipe.err"; }
check pipe_no_ansi "$(ev pipe "$tree")" no_ansi
no_progress() { ! grep -q "$cr\|⠋\|⠙\|⠹" "$io/pipe.out" "$io/pipe.err"; }
check pipe_no_progress "$(ev pipe "$tree")" no_progress

# On a TTY (script(1) fakes one; BSD and util-linux take different arguments):
# color by default, none with NO_COLOR=1 or --no-color.
tty_run() { # tty_run <typescript> <command...>
  out=$1
  shift
  if script --version 2>/dev/null | grep -q util-linux; then
    script -qec "$*" "$out" </dev/null >/dev/null 2>&1 || true
  else
    script -q "$out" "$@" </dev/null >/dev/null 2>&1 || true
  fi
}
if command -v script >/dev/null 2>&1; then
  # shellcheck disable=SC2086
  tty_run "$io/tty.ts" treemand $tree
  # shellcheck disable=SC2086
  tty_run "$io/ttyenv.ts" env NO_COLOR=1 treemand $tree
  # shellcheck disable=SC2086
  tty_run "$io/ttyflag.ts" treemand --no-color $tree
  tty_ev() { printf 'TTY via script(1): color by default: %s; color with %s: %s' "$(sgr "$io/tty.ts" && echo yes || echo no)" "$1" "$(sgr "$2" && echo yes || echo no)"; }
  colorless() { sgr "$io/tty.ts" && [ -s "$1" ] && ! sgr "$1"; }
  check tty_no_color_env "$(tty_ev NO_COLOR=1 "$io/ttyenv.ts")" colorless "$io/ttyenv.ts"
  check tty_no_color_flag "$(tty_ev --no-color "$io/ttyflag.ts")" colorless "$io/ttyflag.ts"
else
  printf 'tty_no_color_env\tscript(1) not installed\ntty_no_color_flag\tscript(1) not installed\n' >>"$work/skipped.tsv"
fi

# --output=json: stdout is one JSON document and nothing else.
# shellcheck disable=SC2086
run json --output=json $tree
json_ok() { [ "$(rc json)" = 0 ] && python3 -I -c 'import json,sys; json.loads(sys.stdin.read())' <"$io/json.out" 2>/dev/null; }
check json_output_clean "$(ev json "--output=json $tree")" json_ok

# A target CLI that does not exist: non-zero, one error line and one hint.
run missing treemand-no-such-cli
two_lines() { errors missing && [ "$(grep -c . "$io/missing.err")" = 2 ] && head -n 1 "$io/missing.err" | grep -q '^Error'; }
check missing_cli_error "$(ev missing treemand-no-such-cli)" two_lines

# Help walk: the subcommands --help lists, two levels deep, with descriptions.
listed() { # listed <helpfile>: the "name description" lines under Available Commands
  awk '/^Available Commands:/ { on = 1; next } on && /^[[:space:]]*$/ { on = 0 } on' "$1"
}
walk() { # walk <path> <helpfile>: append "path<TAB>0|1" per listed subcommand
  listed "$2" | while read -r name desc; do
    printf '%s\t%s\n' "${1:+$1 }$name" "$([ -n "$desc" ] && echo 1 || echo 0)"
  done >>"$work/subcommands.tsv"
}
walk "" "$io/help.out"
cp "$work/subcommands.tsv" "$work/level1.tsv"
while IFS="$(printf '\t')" read -r path _; do
  [ "$path" = help ] && continue
  # One word per level, and subcommand names have no spaces (SC2086).
  # shellcheck disable=SC2086
  treemand $path --help >"$io/walk.out" 2>/dev/null </dev/null || true
  walk "$path" "$io/walk.out"
done <"$work/level1.tsv"
config_ok() { grep -q '^  config ' "$io/help.out" && run cfg config --help && [ -n "$(listed "$io/cfg.out")" ]; }
check config_discoverable "exit=$(rc help) treemand --help and treemand config --help list config and its subcommands" config_ok
undescribed="$(awk -F '\t' '$2 == 0' "$work/subcommands.tsv" | grep -c . || true)"
described() { [ -s "$work/subcommands.tsv" ] && [ "$undescribed" = 0 ]; }
check subcommand_descriptions "exit=$(rc help) $(grep -c . "$work/subcommands.tsv" || true) subcommands listed, $undescribed without a description" described

# Completion scripts for bash, zsh and fish, parsed by the shell when installed.
for sh in bash zsh fish; do
  run "comp$sh" completion "$sh"
  comp_ok() {
    [ "$(rc "comp$1")" = 0 ] && ! has_err "comp$1" && grep -q treemand "$io/comp$1.out" || return 1
    ! command -v "$1" >/dev/null 2>&1 || "$1" -n "$io/comp$1.out" >/dev/null 2>&1
  }
  check "completion_$sh" "$(ev "comp$sh" "completion $sh")" comp_ok "$sh"
done

python3 -I "$root/scripts/metrics/cli_ux_report.py" "$work" "$undescribed"
