#!/usr/bin/env bash
# Every `task <name>` mentioned in docs, scripts, workflows or the loop skill must exist.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
mapfile -t defined < <(task --list-all --json | jq -r '.tasks[].name')
status=0
while IFS= read -r ref; do
  found=0
  for d in "${defined[@]}"; do [ "$d" = "$ref" ] && found=1 && break; done
  if [ "$found" = 0 ]; then
    echo "undefined task referenced: $ref" >&2
    status=1
  fi
done < <(grep -rhoE '(`|^[[:space:]]*(- |run: )?)task [a-z][a-z0-9:_-]*' \
    README.md CONTRIBUTING.md docs .github scripts .claude Taskfile.yml .tmuxinator 2>/dev/null |
  sed -E 's/.*task //' | sort -u)
exit "$status"
