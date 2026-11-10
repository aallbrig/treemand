#!/usr/bin/env bash
# Capture sanitized root help text for real CLIs into the regression corpus.
#   usage: scripts/capture_corpus.sh [cli...]   (default: the corpus list; missing CLIs are skipped)
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
fi

for cli in "${targets[@]}"; do
  if ! command -v "$cli" >/dev/null 2>&1; then
    echo "skip $cli (not installed)"
    continue
  fi
  # help_text on the root node is exactly the text treemand parsed.
  text="$("$bin" --no-cache --output=json --depth=1 --timeout=60 "$cli" 2>/dev/null | jq -r '.help_text // empty')"
  if [ -z "$text" ]; then
    echo "skip $cli (no help text)"
    continue
  fi
  # Keep machine-specific details out of committed fixtures.
  printf '%s\n' "$text" | sed -e "s#${HOME}#~#g" -e "s#$(id -un)#user#g" -e "s#$(hostname)#host#g" >"$out/$cli.txt"
  echo "captured $cli ($(wc -l <"$out/$cli.txt") lines)"
done
