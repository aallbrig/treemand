#!/usr/bin/env bash
# Run treemand against real CLIs installed on this machine and print one row per
# binary: exit code, seconds, node count, nodes with no description, nodes with
# no flags, and whether the root description looks like a usage synopsis.
# usage: corpus.sh [binary...]   (default: a broad spread; missing ones are skipped)
set -uo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$LOOP_ROOT" || exit 1
bin="$LOOP_DIR/treemand-corpus"
go build -o "$bin" ./cli/treemand || exit 1

targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=(git docker kubectl gh go npm pip helm tar systemctl apt tmux jq make aws gcloud terraform cargo rustc ssh curl ls grep find)
fi

printf 'cli\trc\tsecs\tnodes\tno_desc\tno_flags\troot_desc_is_synopsis\n'
for b in "${targets[@]}"; do
  command -v "$b" >/dev/null 2>&1 || continue
  [ "$(type -t "$b")" = file ] || b="$(command -v "$b")"
  start=$(date +%s.%N)
  out="$(timeout 120 "$bin" --no-cache --output=json "$b" 2>/dev/null)"
  rc=$?
  secs="$(awk -v s="$start" -v e="$(date +%s.%N)" 'BEGIN{printf "%.1f", e-s}')"
  jq -r --arg b "$(basename "$b")" --arg rc "$rc" --arg secs "$secs" '
    [.. | objects | select(has("full_path"))] as $n
    | [ $b, $rc, $secs, ($n|length),
        ([$n[] | select((.description // "") == "")] | length),
        ([$n[] | select((.flags // []) | length == 0)] | length),
        ((.description // "") | test("^\\s*\\[|^\\s*usage:|^\\s*<"; "i")) ]
    | @tsv' <<<"$out" 2>/dev/null || printf '%s\t%s\t%s\tparse-error\t\t\t\n' "$(basename "$b")" "$rc" "$secs"
done
