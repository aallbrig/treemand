#!/usr/bin/env bash
# PreToolUse hook for the unattended loop. Exit 2 blocks the call (stderr goes to Claude).
# Hard limits: no force-push, no direct push to main, no skipping git hooks, no
# secrets/branch-protection/repo-settings changes, no admin merges, no sudo, no
# writes outside this repo or into .git, and no changes to the loop's own
# permissions, guard or hooks by any tool. A shell can always be talked around a
# pattern; this is a tripwire for mistakes, and CI plus the main ruleset are the
# real gates.
set -uo pipefail
input="$(cat)"
tool="$(jq -r '.tool_name // ""' <<<"$input")"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

deny() {
  echo "guard: blocked: $1" >&2
  exit 2
}

# `git`, then any global options (-C <dir>, -c <key=value>, --git-dir=<dir>, ...),
# then the subcommand, so `git -C . push` reads like `git push`.
git_='git([[:space:]]+(-[Cc][[:space:]]+[^[:space:]]+|--[a-z-]+(=[^[:space:]]+)?))*[[:space:]]+'
# The rest of one command: up to the next pipe, semicolon or ampersand.
rest='[^|;&]*'
# Files the loop may not change: its permissions, guard and hooks.
protected='\.claude/(loop-)?settings\.json|scripts/loop/guard\.sh|scripts/loop/hooks/'
# Shell constructs that write or replace a file.
writes='(^|[^0-9&])>|(^|[[:space:]])(sed|gsed)[[:space:]]+(-[a-zA-Z]*i|--in-place)|perl[[:space:]]+-[a-zA-Z]*i|(^|[;&|[:space:]])(tee|cp|mv|rm|ln|chmod|truncate|install|dd|patch|python3?|ruby|node)[[:space:]]|git[[:space:]]+(checkout|restore|apply|mv|rm|update-index)[[:space:]]'

case "$tool" in
  Bash)
    cmd="$(jq -r '.tool_input.command // ""' <<<"$input")"
    shopt -s nocasematch
    # The command with quoted text removed, so a commit message or PR body that
    # mentions a flag (-n, --no-verify) or a file is not mistaken for using it.
    bare="$(sed -E "s/'[^']*'//g; s/\"([^\"\\\\]|\\\\.)*\"//g" <<<"$cmd")"
    # A heredoc's body is data too (commit messages, PR bodies): drop it.
    bare="${bare%%<<*}"
    # Pushing.
    [[ "$cmd" =~ ${git_}push${rest}(--force|--force-with-lease|--mirror|[[:space:]]-[a-z]*f([[:space:]]|$)) ]] && deny "force push"
    [[ "$cmd" =~ ${git_}push${rest}[[:space:]]\+[^[:space:]]+ ]] && deny "force push (+refspec)"
    [[ "$cmd" =~ ${git_}push${rest}[[:space:]]([^[:space:]]+:)?(refs/heads/)?main([[:space:]]|$) ]] && deny "direct push to main (open a PR; tags are fine)"
    [[ "$cmd" =~ ${git_}push[[:space:]]*($|[|;&]) && "$(git -C "$root" branch --show-current)" == "main" ]] && deny "direct push to main"
    # Skipping git hooks.
    [[ "$bare" =~ ${git_}(commit|push|merge|rebase|cherry-pick|am)${rest}--no-verify ]] && deny "skipping git hooks (--no-verify)"
    [[ "$bare" =~ ${git_}commit${rest}[[:space:]]-[a-z]*n[a-z]*([[:space:]]|$) ]] && deny "skipping git hooks (git commit -n)"
    [[ "$bare" =~ core\.hooksPath(=|[[:space:]]+[^-[:space:]|;&]) || "$bare" =~ --unset${rest}core\.hooksPath ]] && deny "changing core.hooksPath"
    [[ "$bare" =~ (^|[;&|[:space:]])(SKIP|PRE_COMMIT_ALLOW_NO_CONFIG|LEFTHOOK|HUSKY)= ]] && deny "skipping git hooks by environment"
    # GitHub.
    [[ "$cmd" =~ gh[[:space:]]+pr[[:space:]]+merge${rest}--admin ]] && deny "admin merge (bypasses required checks)"
    [[ "$cmd" =~ gh[[:space:]]+(secret|ssh-key|gpg-key|auth[[:space:]]+(logout|refresh)) ]] && deny "gh credential command"
    [[ "$cmd" =~ gh[[:space:]]+repo[[:space:]]+(delete|edit|archive|rename) ]] && deny "gh repo settings"
    [[ "$cmd" =~ gh[[:space:]]+ruleset[[:space:]]+(create|edit|delete) ]] && deny "gh ruleset change"
    [[ "$cmd" =~ gh[[:space:]]+api${rest}(protection|/secrets|/keys|/actions/permissions|/rulesets|-X[[:space:]]*(DELETE|PUT)|--method[[:space:]]*(DELETE|PUT)) ]] && deny "gh api on protected surface or destructive method"
    # The loop's own permissions, guard and hooks, written from the shell.
    [[ "$cmd" =~ $protected && "$bare" =~ $writes ]] && deny "the loop may not edit its own permissions, guard or hooks"
    # Host safety.
    [[ "$cmd" =~ (^|[\;\&\|[:space:]])sudo[[:space:]] ]] && deny "sudo"
    [[ "$cmd" =~ rm[[:space:]]+-[a-z]*r[a-z]*f?[[:space:]]+(/|~|\$HOME)([[:space:]]|$) ]] && deny "destructive rm"
    [[ "$cmd" =~ git[[:space:]]+(reset[[:space:]]+--hard[[:space:]]+origin|clean[[:space:]]+-[a-z]*f[a-z]*d) && "$PWD" == "$root" ]] && deny "destructive git in the main checkout (work in .loop/worktrees)"
    ;;
  Edit | Write | NotebookEdit)
    path="$(jq -r '.tool_input.file_path // .tool_input.notebook_path // ""' <<<"$input")"
    [ -n "$path" ] || exit 0
    # Lexical normalization, portable to macOS (BSD realpath has no -m).
    abs="$(python3 -I -c 'import os, sys; print(os.path.normpath(os.path.abspath(sys.argv[1])))' "$path")"
    case "$abs" in "$root"/*) ;; *) deny "write outside the repo: $abs" ;; esac
    case "$abs" in "$root"/.git/*) deny "write inside .git" ;; esac
    case "$abs" in
      "$root"/.env* | */.env | */id_rsa* | */credentials* | *.pem | *.key) deny "secret-like file: $abs" ;;
    esac
    [[ "${abs#"$root"/}" =~ ^($protected) ]] && deny "the loop may not edit its own permissions, guard or hooks"
    ;;
esac
exit 0
