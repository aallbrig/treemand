#!/bin/sh
# Every Taskfile `cmds` entry must be one simple command. Task runs inline
# commands in its own Go shell interpreter, so they are never POSIX-checked
# or shellchecked; anything longer belongs in a script under scripts/.
#   sh scripts/check_taskfile_cmds.sh [Taskfile.yml]
#
# The rule, applied to each string entry and each `cmd:` of a map entry
# (`task:` references are always allowed):
#   1. Remove Go templates ({{...}}), single-quoted strings ('...') and
#      file-descriptor duplications (2>&1, >&2): none of them chain commands.
#   2. The entry fails if what is left
#        - spans more than one line,
#        - contains | & ; ( ) or a backtick (pipes, ||/&& chains, lists,
#          background jobs, subshells, command substitution), or
#        - starts with a compound-command keyword (if for while until case { !).
# Plain `cmd args {{.CLI_ARGS}}` and redirections such as >/dev/null pass.
# There is no exemption list: move the logic into a script instead.
set -eu
taskfile="${1:-$(dirname "$0")/../Taskfile.yml}"

# The regex and the yq program are literal text, not shell expansions.
# shellcheck disable=SC2016
offenders="$(RE='[|&;`()]|\n.|^[[:space:]]*(if|for|while|until|case|\{|!)([[:space:]]|$)' yq -r '
  .tasks | to_entries | .[] | .key as $task | (.value.cmds // []) | .[] |
  ((select(tag == "!!map") | .cmd | select(. != null)), select(tag == "!!str")) |
  select(sub("\{\{.*?\}\}"; "") | sub("'"'[^']*'"'"; "") | sub("[0-9]*>&[0-9-]+"; "") | test(strenv(RE))) |
  "  " + $task + ": " + (split("\n") | .[0])
' "$taskfile")"

if [ -n "$offenders" ]; then
  echo "Taskfile cmds that are more than one simple command (move them into scripts/):" >&2
  printf '%s\n' "$offenders" >&2
  exit 1
fi
echo "Taskfile cmds are single commands"
