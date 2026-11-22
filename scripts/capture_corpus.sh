#!/usr/bin/env bash
# Capture sanitized root help text for real CLIs into the regression corpus.
#   usage: scripts/capture_corpus.sh [cli...]   (default: the corpus list; missing CLIs are skipped)
# A name may carry a variant after "@" (tar@bsd): the CLI is the part before it,
# and the fixture keeps the whole name, so one tool's GNU and BSD help can both
# be in the corpus. Capture a variant only on a host whose CLI is that variant.
# Fixtures land in cli/treemand/discovery/testdata/corpus/<cli>.txt. Re-capturing changes
# fixtures, so run `task corpus:update-baseline` afterwards and review the diff.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

out=cli/treemand/discovery/testdata/corpus
mkdir -p "$out"
bin="$(mktemp -d)/treemand"
(cd cli/treemand && CGO_ENABLED=0 go build -o "$bin" .)

targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=(git docker kubectl gh aws npm pip go cargo helm terraform systemctl apt tmux ssh curl tar jq)
  # macOS family: only on macOS, where these are the system's own tools.
  if [ "$(uname -s)" = Darwin ]; then
    targets+=(brew xcrun networksetup tar@bsd ls@bsd sed@bsd find@bsd)
  fi
fi
# Never let a capture update Homebrew or send its analytics.
export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ANALYTICS=1

# Discovery runs the real CLIs; run them from an empty directory so nothing
# they might write lands in the repository.
cwd="$(mktemp -d)"
trap 'rm -rf "$cwd" "$(dirname "$bin")"' EXIT

for name in "${targets[@]}"; do
  cli="${name%%@*}"
  if ! command -v "$cli" >/dev/null 2>&1; then
    echo "skip $name (not installed)"
    continue
  fi
  # help_text on the root node is exactly the text treemand parsed.
  text="$(cd "$cwd" && "$bin" --no-cache --output=json --depth=0 --timeout=60 "$cli" 2>/dev/null | jq -r '.help_text // empty')"
  if [ -z "$text" ]; then
    echo "skip $name (no help text)"
    continue
  fi
  # Keep machine-specific details out of committed fixtures.
  printf '%s\n' "$text" | sed -e "s#${HOME}#~#g" -e "s#$(id -un)#user#g" -e "s#$(hostname)#host#g" >"$out/$name.txt"
  echo "captured $name ($(wc -l <"$out/$name.txt") lines)"
done
