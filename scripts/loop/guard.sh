#!/usr/bin/env bash
# PreToolUse hook for the unattended loop. Exit 2 blocks the call (stderr goes to Claude).
# Hard limits: no force-push, no direct push to main, no secrets/branch-protection/
# repo-settings changes, no sudo, no writes outside this repo or into .git.
set -uo pipefail
input="$(cat)"
tool="$(jq -r '.tool_name // ""' <<<"$input")"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

deny() {
  echo "guard: blocked: $1" >&2
  exit 2
}

case "$tool" in
  Bash)
    cmd="$(jq -r '.tool_input.command // ""' <<<"$input")"
    shopt -s nocasematch
    [[ "$cmd" =~ git[[:space:]]+push[^|\;\&]*(--force|--force-with-lease|[[:space:]]-f([[:space:]]|$)) ]] && deny "force push"
    [[ "$cmd" =~ git[[:space:]]+push[^|\;\&]*[[:space:]]([^[:space:]]+:)?(refs/heads/)?main([[:space:]]|$) ]] && deny "direct push to main (open a PR; tags are fine)"
    [[ "$cmd" =~ git[[:space:]]+push[[:space:]]*$ && "$(git -C "$root" branch --show-current)" == "main" ]] && deny "direct push to main"
    [[ "$cmd" =~ gh[[:space:]]+(secret|ssh-key|gpg-key|auth[[:space:]]+(logout|refresh)) ]] && deny "gh credential command"
    [[ "$cmd" =~ gh[[:space:]]+repo[[:space:]]+(delete|edit|archive|rename) ]] && deny "gh repo settings"
    [[ "$cmd" =~ gh[[:space:]]+api[^|\;\&]*(protection|/secrets|/keys|/actions/permissions|/rulesets|-X[[:space:]]*(DELETE|PUT)|--method[[:space:]]*(DELETE|PUT)) ]] && deny "gh api on protected surface or destructive method"
    [[ "$cmd" =~ (^|[\;\&\|[:space:]])sudo[[:space:]] ]] && deny "sudo"
    [[ "$cmd" =~ rm[[:space:]]+-[a-z]*r[a-z]*f?[[:space:]]+(/|~|\$HOME)([[:space:]]|$) ]] && deny "destructive rm"
    [[ "$cmd" =~ git[[:space:]]+(reset[[:space:]]+--hard[[:space:]]+origin|clean[[:space:]]+-[a-z]*f[a-z]*d) && "$PWD" == "$root" ]] && deny "destructive git in the main checkout (work in .loop/worktrees)"
    ;;
  Edit | Write | NotebookEdit)
    path="$(jq -r '.tool_input.file_path // .tool_input.notebook_path // ""' <<<"$input")"
    [ -n "$path" ] || exit 0
    abs="$(realpath -m -- "$path")"
    case "$abs" in "$root"/*) ;; *) deny "write outside the repo: $abs" ;; esac
    case "$abs" in "$root"/.git/*) deny "write inside .git" ;; esac
    case "$abs" in
      "$root"/.env* | */.env | */id_rsa* | */credentials* | *.pem | *.key) deny "secret-like file: $abs" ;;
    esac
    case "$abs" in
      "$root"/.claude/loop-settings.json | "$root"/scripts/loop/guard.sh) deny "the loop may not edit its own permissions or guard" ;;
    esac
    ;;
esac
exit 0
