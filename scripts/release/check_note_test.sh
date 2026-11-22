#!/bin/sh
# Tests for check_note.sh: a PR body passes only with a filled-in
# "## Release note" section, and the note it prints is that section's text.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
fails=0
check() { # check <name> <want> <got>
  if [ "$2" = "$3" ]; then echo "  ok   $1"; else
    echo "  FAIL $1: want '$2', got '$3'"
    fails=$((fails + 1))
  fi
}
# run <body>: prints "<exit status>|<stdout>"
run() {
  rc=0
  out="$(printf '%s\n' "$1" | sh "$here/check_note.sh" 2>/dev/null)" || rc=$?
  echo "$rc|$out"
}

check "a one-line note passes and is printed" \
  "0|The tree view shows aliases next to each command." \
  "$(run '## Summary
Adds aliases.

## Release note

The tree view shows aliases next to each command.

## Test plan
task ci')"

check "none passes" "0|none" "$(run '## Release note
none')"

# shellcheck disable=SC2016 # literal backticks in the body
check "\`none\` in backticks counts as none" "0|none" "$(run '## Release note
`none`')"

check "a note wrapped over two lines is joined" "0|Fixed a crash when help text is empty." \
  "$(run '## Release note
Fixed a crash when
help text is empty.')"

check "the template comment is not a note" "1|" "$(run '## Release note

<!-- One user-facing sentence, or none.
See the skill. -->

## Test plan')"

check "a comment beside the note is dropped" "0|Faster startup." "$(run '## Release note
<!-- hint --> Faster startup.')"

check "text after a blank line is not part of the note" "0|none" "$(run '## Release note

none

Part of #143.')"

check "a missing section fails" "1|" "$(run '## Summary
Something.')"

check "an empty section at the end fails" "1|" "$(run '## Release note
')"

check "an empty body fails" "1|" "$(run '')"

check "the heading is matched case-insensitively" "0|none" "$(run '## Release Note
none')"

check "a similar heading does not count" "1|" "$(run '## Release
none')"

[ "$fails" -eq 0 ] || { echo "check_note_test: $fails failure(s)"; exit 1; }
echo "check_note_test: all passed"
