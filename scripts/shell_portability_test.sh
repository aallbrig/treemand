#!/bin/sh
# Tests for the shell-portability probe and shell_inventory.sh against a
# scratch repository: which scripts count as POSIX, bash, exempt or stale.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fails=0
check() { # check <name> <want> <got>
  if [ "$2" = "$3" ]; then echo "  ok   $1"; else
    echo "  FAIL $1: want '$2', got '$3'"
    fails=$((fails + 1))
  fi
}

repo="$work/repo"
mkdir -p "$repo/scripts/metrics/probes" "$repo/tools"
cp "$here/shell_inventory.sh" "$repo/scripts/"
cp "$here/metrics/probes/shell-portability.sh" "$repo/scripts/metrics/probes/"
cd "$repo"
printf '#!/bin/sh\n' >tools/a.sh
printf '#!/usr/bin/env sh\n' >tools/b.sh
printf '#!/usr/bin/env bash\n' >tools/c.sh
printf '#!/bin/bash -e\n' >tools/d.sh
printf '#!/usr/bin/env zsh\n' >tools/e
printf '#!/usr/bin/env python3\n' >tools/f.py
printf 'echo no shebang\n' >tools/g.sh
printf '#!/bin/bash\n' >tools/untracked.sh
cat >scripts/shell-exempt.txt <<'X'
# comment

tools/c.sh — needs arrays
tools/a.sh -- already POSIX, so stale
tools/gone.sh — no such file
X
git init -q .
git add scripts tools/a.sh tools/b.sh tools/c.sh tools/d.sh tools/e tools/f.py tools/g.sh

check "inventory: shells by shebang, tracked files only" "posix scripts/metrics/probes/shell-portability.sh
posix scripts/shell_inventory.sh
posix tools/a.sh
posix tools/b.sh
bash tools/c.sh
bash tools/d.sh
zsh tools/e" "$(dash scripts/shell_inventory.sh)"

json="$(dash scripts/metrics/probes/shell-portability.sh)"
metric() { printf '%s' "$json" | python3 -I -c "import json,sys; print(json.load(sys.stdin)$1)"; }
check "non-POSIX scripts that are not exempt" 2 "$(metric '["metrics"]["non_posix_scripts"]')"
check "exempt scripts" 1 "$(metric '["metrics"]["exempt_scripts"]')"
check "stale exemptions (POSIX now, or gone)" 2 "$(metric '["metrics"]["stale_exemptions"]')"
check "POSIX scripts" 4 "$(metric '["metrics"]["posix_scripts"]')"
check "details name the offenders" "['tools/d.sh', 'tools/e']" "$(metric '["details"][0]["non_posix_not_exempt"]')"
check "details carry each exemption's reason" "needs arrays" "$(metric '["details"][2]["exempt"][0]["reason"]')"

[ "$fails" = 0 ] || { echo "$fails failed"; exit 1; }
echo "shell-portability: all passed"
