#!/usr/bin/env bash
# Prove the guard hook blocks what it must and allows what the loop needs.
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
fail=0

run() { # run <expected-exit> <json>
  local want="$1" json="$2" got
  got="$(printf '%s' "$json" | bash "$here/guard.sh" >/dev/null 2>&1; echo $?)"
  if [ "$got" != "$want" ]; then
    echo "guard_test FAIL (want $want got $got): $json" >&2
    fail=1
  fi
}
bash_cmd() { jq -nc --arg c "$1" '{tool_name:"Bash",tool_input:{command:$c}}'; }
edit_path() { jq -nc --arg p "$1" '{tool_name:"Edit",tool_input:{file_path:$p}}'; }

run 2 "$(bash_cmd 'git push --force origin feat/x')"
run 2 "$(bash_cmd 'git push -f origin feat/x')"
run 2 "$(bash_cmd 'git push origin main')"
run 2 "$(bash_cmd 'git push origin HEAD:main')"
run 2 "$(bash_cmd 'gh secret set X')"
run 2 "$(bash_cmd 'gh api -X PUT repos/a/b/branches/main/protection')"
run 2 "$(bash_cmd 'sudo rm -rf /var')"
run 0 "$(bash_cmd 'git push -u origin loop/12-fix-thing')"
run 0 "$(bash_cmd 'git push origin v0.4.0')"
run 0 "$(bash_cmd 'gh pr create --fill')"
run 0 "$(bash_cmd 'task ci')"
run 2 "$(edit_path /etc/passwd)"
run 2 "$(edit_path "$root/.git/config")"
run 2 "$(edit_path "$root/.claude/loop-settings.json")"
run 2 "$(edit_path "$root/scripts/loop/guard.sh")"
run 0 "$(edit_path "$root/cli/treemand/main.go")"
run 0 "$(edit_path "$root/.github/workflows/ci.yml")"
run 0 "$(edit_path "$root/.loop/worktrees/x/y.go")"
exit "$fail"
