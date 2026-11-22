#!/bin/sh
# Lint every tracked shell script (scripts/shell_inventory.sh): POSIX ones as
# POSIX sh, the exempt bash ones as bash, both at warning severity. The
# shell-portability probe counts which scripts are which; this checks them.
#   sh scripts/check_shell.sh
set -eu
cd "$(dirname "$0")/.."
inventory="$(sh scripts/shell_inventory.sh)"
posix="$(printf '%s\n' "$inventory" | awk '$1 == "posix" { print $2 }')"
other="$(printf '%s\n' "$inventory" | awk '$1 != "posix" { print $2 }')"
# One path per line and no spaces in tracked script names: word splitting on
# newlines is the intent (SC2086).
IFS='
'
# shellcheck disable=SC2086
shellcheck -x --shell=sh -S warning $posix
# shellcheck disable=SC2086
[ -z "$other" ] || shellcheck -x -S warning $other
echo "shellcheck: $(printf '%s\n' "$posix" | grep -c .) POSIX and $(printf '%s\n' "$other" | grep -c .) exempt scripts clean"
