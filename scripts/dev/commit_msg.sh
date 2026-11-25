#!/bin/sh
# The commit-msg gate: commit_msg.sh <message file>
#
#   subject      at most 72 characters (GitHub truncates longer ones), and the
#                line after it blank
#   trailers     a commit made from a Claude Code session (CLAUDECODE=1) needs
#                a Co-Authored-By trailer, so agent work is always attributed
#
# Subjects git writes itself (Merge, Revert, fixup!, squash!, amend!) are exempt.
set -u
max=72
[ -f "${1:-}" ] || { echo "usage: commit_msg.sh <message file>" >&2; exit 2; }

# The message as git will record it: comments and anything below the
# scissors line (commit --verbose) are not part of it.
msg="$(awk '/^# -+ >8 -+$/ { exit } !/^#/' "$1")"
subject="$(printf '%s\n' "$msg" | sed -n 1p)"
second="$(printf '%s\n' "$msg" | sed -n 2p)"

case "$subject" in
  "Merge "* | "Revert \""* | "fixup! "* | "squash! "* | "amend! "*) exit 0 ;;
esac

fail=0
len="$(printf '%s' "$subject" | wc -m | tr -d ' ')"
if [ "$len" -gt "$max" ]; then
  echo "commit-msg: the subject is $len characters; keep it to $max:" >&2
  echo "  $subject" >&2
  fail=1
fi
if [ -n "$second" ]; then
  echo "commit-msg: leave a blank line between the subject and the body" >&2
  fail=1
fi
if [ "${CLAUDECODE:-}" = 1 ] &&
  ! printf '%s\n' "$msg" | git interpret-trailers --parse | grep -qi '^Co-Authored-By:'; then
  echo "commit-msg: a commit from a Claude Code session needs a Co-Authored-By trailer" >&2
  fail=1
fi
exit "$fail"
