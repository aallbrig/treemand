#!/usr/bin/env bash
# zoo smoke   : full discovery of every zoo CLI (no cache); prints a TSV report
#               and fails if any run errors, crashes or yields an empty tree.
# zoo capture : write each CLI's root help to /out/corpus/<cli>.txt (mount the
#               repo's corpus directory there), sanitized like capture_corpus.sh.
set -euo pipefail

ZOO=(terraform promtool rg fd bat mvn black flask poetry http eslint jest heroku thor)
mode="${1:-smoke}"

# Discovery runs the CLIs with arguments: keep whatever they write in a
# throwaway directory.
work="$(mktemp -d)"
cd "$work"

case "$mode" in
capture)
  out=/out/corpus
  [ -d "$out" ] || { echo "mount the corpus directory at $out" >&2; exit 2; }
  for cli in "${ZOO[@]}"; do
    text="$(treemand --no-cache --output=json --depth=0 --timeout=60 "$cli" 2>/dev/null | jq -r '.help_text // empty')"
    if [ -z "$text" ]; then echo "skip $cli (no help text)"; continue; fi
    printf '%s\n' "$text" | sed -e "s#${HOME}#~#g" -e "s#$(hostname)#host#g" >"$out/$cli.txt"
    echo "captured $cli ($(wc -l <"$out/$cli.txt") lines)"
  done
  ;;
smoke)
  fail=0
  printf 'cli\trc\tseconds\tnodes\tflags\n'
  for cli in "${ZOO[@]}"; do
    start=$(date +%s%N)
    set +e
    treemand --no-cache --output=json --timeout=120 "$cli" >out.json 2>err.txt
    rc=$?
    set -e
    tenths=$(( ($(date +%s%N) - start) / 100000000 ))
    secs="$((tenths / 10)).$((tenths % 10))"
    nodes=$(jq '[.. | objects | select(has("full_path"))] | length' out.json 2>/dev/null || echo 0)
    flags=$(jq '[.. | objects | select(has("full_path")) | (.flags // [])[]] | length' out.json 2>/dev/null || echo 0)
    printf '%s\t%s\t%s\t%s\t%s\n' "$cli" "$rc" "$secs" "$nodes" "$flags"
    if [ "$rc" != 0 ] || [ "$nodes" = 0 ] || grep -q "panic:" err.txt; then
      echo "FAIL $cli: rc=$rc nodes=$nodes $(head -c 300 err.txt)" >&2
      fail=1
    fi
  done
  exit "$fail"
  ;;
*) echo "usage: zoo smoke|capture" >&2; exit 2 ;;
esac
