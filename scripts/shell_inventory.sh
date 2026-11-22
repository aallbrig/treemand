#!/bin/sh
# Every tracked shell script and the shell it is written for, one per line:
#   posix <path>       #!/bin/sh or #!/usr/bin/env sh
#   <shell> <path>     #!... bash, zsh, ksh or fish (not POSIX)
# A script is a tracked file whose first line is a #! naming a shell; other
# interpreters (python3, node) are not shell scripts and are not listed.
# Read by the shell-portability probe and by check_shell.sh.
#   sh scripts/shell_inventory.sh
set -eu
cd "$(dirname "$0")/.."
# The awk program is single-quoted on purpose (SC2016).
# shellcheck disable=SC2016
git ls-files -z | LC_ALL=C xargs -0 awk '
  FNR == 1 {
    if ($0 ~ /^#!(\/bin\/|\/usr\/bin\/env +)sh( |$)/) print "posix", FILENAME
    else if (match($0, /^#!.*[\/ ](bash|zsh|ksh|fish)( |$)/)) {
      n = split(substr($0, RSTART, RLENGTH), w, /[\/ ]/)
      shell = w[n] == "" ? w[n - 1] : w[n]
      print shell, FILENAME
    }
    nextfile
  }' | LC_ALL=C sort -k2
