#!/bin/sh
# Check that a pull request body carries its release note.
#
#   sh scripts/release/check_note.sh < body.md
#
# The body needs a "## Release note" section whose first paragraph is one
# user-facing sentence, or "none" for a change users never see (skill treemand-release-notes).
# HTML comments (the template's hint) are not a note. Prints the note ("none"
# for an internal change) and exits 0; exits 1 with the reason on stderr when
# the section is missing or empty. scripts/release/notes.py reads it the same way.
set -eu

note="$(tr -d '\r' | awk '
  BEGIN { insec = 0; incomment = 0; note = "" }
  /^##[ \t]/ {
    h = tolower($0)
    sub(/[ \t]+$/, "", h)
    insec = (h ~ /^##[ \t]+release note$/)
    next
  }
  insec {
    line = $0
    out = ""
    while (line != "") {
      if (incomment) {
        i = index(line, "-->")
        if (i == 0) { line = "" } else { line = substr(line, i + 3); incomment = 0 }
      } else {
        i = index(line, "<!--")
        if (i == 0) { out = out line; line = "" } else {
          out = out substr(line, 1, i - 1); line = substr(line, i + 4); incomment = 1
        }
      }
    }
    gsub(/^[ \t]+|[ \t]+$/, "", out)
    if (out != "") note = (note == "" ? out : note " " out)
    # The note is the first paragraph: a blank line after it ends the note.
    else if (note != "" && !incomment && $0 !~ /<!--|-->/) insec = 0
  }
  END { print note }
')"

case "$(printf '%s' "$note" | tr '[:upper:]' '[:lower:]' | tr -d '`.')" in
"")
  cat >&2 <<'EOF'
check_note: the PR body has no release note. Add a section

## Release note

with one sentence a treemand user would care about, or `none` for a change
users never see (tests, CI, refactors, tooling). How to write it:
.claude/skills/treemand-release-notes/SKILL.md
EOF
  exit 1
  ;;
none) echo none ;;
*) printf '%s\n' "$note" ;;
esac
