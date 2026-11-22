#!/bin/sh
# Every `task <name>` mentioned in docs, scripts, workflows or the loop skill must exist.
set -eu
cd "$(dirname "$0")/.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
task --list-all --json | jq -r '.tasks[].name' | sort -u >"$work/defined"
grep -rhoE '(`|^[[:space:]]*(- |run: )?)task [a-z][a-z0-9:_-]*' \
  README.md CONTRIBUTING.md docs .github scripts .claude Taskfile.yml .tmuxinator 2>/dev/null |
  sed -E 's/.*task //' | sort -u >"$work/referenced" || true
LC_ALL=C sort -u -o "$work/defined" "$work/defined"
LC_ALL=C sort -u -o "$work/referenced" "$work/referenced"
undefined="$(LC_ALL=C comm -13 "$work/defined" "$work/referenced")"
if [ -n "$undefined" ]; then
  printf '%s\n' "$undefined" | sed 's/^/undefined task referenced: /' >&2
  exit 1
fi
