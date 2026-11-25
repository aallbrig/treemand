#!/bin/sh
# The pre-commit gate (task precommit), scoped to what is staged so a commit
# takes seconds, not a test run. The full suite runs at pre-push (prepush.sh).
#
#   always             gitleaks on the staged changes (task secrets)
#   staged Go          gofmt on the staged files; go vet and golangci-lint on
#                      the packages that hold them
#   staged shell       shellcheck at warning severity, POSIX sh unless the
#                      script's #! names bash (as task shell:check does)
#   staged JavaScript  Biome on the staged .js and .mjs files (task js:lint);
#                      files biome.jsonc excludes are skipped
#   staged infra/      cfn-lint and checkov (task infra:lint)
#
# The tools read the files on disk, so a partly staged file is checked as it
# is in the working tree. Every tool is required: a missing one fails the commit with its name.
set -u
root="$(git rev-parse --show-toplevel)" || exit 1
cd "$root" || exit 1

fail=0
step() { echo "precommit: $*"; }
need() {
  command -v "$1" >/dev/null 2>&1 && return 0
  echo "precommit: $1 not found: install it (sh scripts/dev/setup.sh --install)" >&2
  exit 1
}
run() { "$@" || fail=1; }

staged="$(git diff --cached --name-only --diff-filter=ACMR)"
# Deleted files count for their package (the package may still build).
touched="$(git diff --cached --name-only --diff-filter=ACMRD)"

need task
step "secrets (gitleaks on staged changes)"
run task secrets

go_files="$(printf '%s\n' "$staged" | grep -E '^cli/treemand/.*\.go$')"
# lintrules holds only ruleguard rules (build tag ruleguard) and web/wasm builds
# only for js/wasm: neither has a package to vet on this host.
go_pkgs="$(printf '%s\n' "$touched" | grep -E '^cli/treemand/.*\.go$' | grep -Ev '^cli/treemand/(lintrules|web/wasm)/' |
  sed 's|/[^/]*$||' | sort -u | while read -r d; do
    for f in "$d"/*.go; do [ -e "$f" ] && echo "./$d" && break; done
  done)"
if [ -n "$go_files" ] || [ -n "$go_pkgs" ]; then
  need gofmt
  need go
  need golangci-lint
  # One path per line and no spaces in Go paths: split on newlines (SC2086).
  IFS='
'
  if [ -n "$go_files" ]; then
    step "gofmt on $(printf '%s\n' "$go_files" | grep -c .) staged Go file(s)"
    # shellcheck disable=SC2086
    unformatted="$(gofmt -l $go_files)" || fail=1
    if [ -n "$unformatted" ]; then
      echo "needs gofmt (run: task fmt, then re-stage):" >&2
      printf '%s\n' "$unformatted" >&2
      fail=1
    fi
  fi
  if [ -n "$go_pkgs" ]; then
    step "go vet and golangci-lint on $(printf '%s\n' "$go_pkgs" | tr '\n' ' ')"
    # shellcheck disable=SC2086
    run go vet $go_pkgs
    # shellcheck disable=SC2086
    run golangci-lint run $go_pkgs
  fi
  unset IFS
fi

posix="" other=""
for f in $staged; do
  [ -f "$f" ] || continue
  first="$(head -n 1 "$f" 2>/dev/null)"
  case "$first" in
    '#!/bin/sh' | '#!/bin/sh '* | '#!/usr/bin/env sh' | '#!/usr/bin/env sh '*) posix="$posix $f" ;;
    '#!'*bash* | '#!'*zsh* | '#!'*ksh*) other="$other $f" ;;
  esac
done
if [ -n "$posix$other" ]; then
  need shellcheck
  step "shellcheck on staged shell scripts:$posix$other"
  # Word splitting on spaces is the intent; tracked script names have none.
  # shellcheck disable=SC2086
  [ -z "$posix" ] || run shellcheck -x --shell=sh -S warning $posix
  # shellcheck disable=SC2086
  [ -z "$other" ] || run shellcheck -x -S warning $other
fi

js_files="$(printf '%s\n' "$staged" | grep -E '\.m?js$')"
if [ -n "$js_files" ]; then
  need npx
  step "biome on $(printf '%s\n' "$js_files" | grep -c .) staged JavaScript file(s)"
  # One path per line; tracked script names have no spaces.
  # shellcheck disable=SC2086
  run sh scripts/dev/js_lint.sh $js_files
fi

if printf '%s\n' "$touched" | grep -q '^infra/'; then
  step "infra lint (infra/ is staged)"
  run task infra:lint
fi

[ "$fail" = 0 ] && step "ok"
exit "$fail"
