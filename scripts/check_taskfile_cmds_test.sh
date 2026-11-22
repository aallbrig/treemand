#!/bin/sh
# Tests for check_taskfile_cmds.sh against fixture Taskfiles: one entry per
# case, so each verdict is about exactly one rule.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fails=0

# verdict <want pass|fail> <name> <yaml for one cmds entry>
verdict() {
  printf 'version: "3"\ntasks:\n  t:\n    cmds:\n      %s\n' "$3" >"$work/Taskfile.yml"
  # A failure counts only when the entry is named: a yq error is not a verdict.
  if sh "$here/check_taskfile_cmds.sh" "$work/Taskfile.yml" >"$work/out" 2>&1; then got=pass
  elif grep -q '^  t: ' "$work/out"; then got=fail
  else got=error; fi
  if [ "$got" = "$1" ]; then echo "  ok   $2"; else
    echo "  FAIL $2: want $1, got $got"
    sed 's/^/       /' "$work/out"
    fails=$((fails + 1))
  fi
}

verdict pass "plain command" '- go test ./...'
verdict pass "CLI_ARGS template" '- bash scripts/ship.sh pr {{.CLI_ARGS}}'
verdict pass "template with a pipe inside" '- go test -fuzz={{index .X 0 | default "F"}} ./d'
verdict pass "pipe inside single quotes" "- grep -E 'a|b' f"
verdict pass "redirection to /dev/null" '- tmuxinator debug >/dev/null'
verdict pass "fd duplication" '- cmd >/dev/null 2>&1'
verdict pass "task reference" '- task: build'
verdict pass "cmd map, simple" '- cmd: go vet ./...'
verdict pass "keyword as an argument" '- echo if'
verdict fail "pipe" '- a | b'
verdict fail "or-fallback" '- a || echo none'
verdict fail "and-chain" '- a && b'
verdict fail "semicolon list" '- a; b'
verdict fail "background job" '- a &'
verdict fail "subshell" '- (a)'
# shellcheck disable=SC2016 # literal Taskfile text, not an expansion
verdict fail "command substitution" '- echo "$(id -u)"'
# shellcheck disable=SC2016
verdict fail "backticks" '- echo "`id -u`"'
verdict fail "if" '- if true then x fi'
verdict fail "for" '- for x in a b'
verdict fail "while" '- while true'
verdict fail "negation" '- "! a"'
verdict fail "multi-line block" '- |
        a
        b'
verdict fail "cmd map, pipe" '- cmd: a | b'
# The repo's own Taskfile passes.
if sh "$here/check_taskfile_cmds.sh" >"$work/out" 2>&1; then echo "  ok   repo Taskfile"; else
  echo "  FAIL repo Taskfile"
  sed 's/^/       /' "$work/out"
  fails=$((fails + 1))
fi

[ "$fails" = 0 ] || { echo "$fails failed"; exit 1; }
echo "check_taskfile_cmds: all passed"
