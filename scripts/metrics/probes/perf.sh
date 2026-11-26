#!/usr/bin/env bash
# Probe: treemand performance on real installed CLIs (PERF_CLIS overrides the set).
#   metrics per CLI: cold_s (uncached full discovery), warm_s (cache hit),
#   Every cold metric is the median of PERF_REPEAT runs (default 1).
#   cpu_s (user+sys CPU of the cold run, child processes included: the cost
#   that load cannot hide), rss_mb, nodes, and tti_s (seconds until -i shows
#   the tree) for the TUI_CLIS subset, and help_calls (processes of the CLI
#   that the cold run started, counted by a logging PATH shim: the cost that
#   load cannot distort at all).
# A CLI this host lacks is reported as skipped ("not installed"), never as a
# regression; baselines are per host (host_bound rules).
# Runs from an empty scratch directory, per treemand-safe-real-cli-run, and
# fails if discovery changed git branches or the Docker context.
set -euo pipefail
repo="$(cd "$(dirname "$0")/../../.." && pwd)"
clis="${PERF_CLIS:-git docker kubectl gh npm pip go helm apt systemctl tar aws}"
tui_clis="${TUI_CLIS:-docker gh kubectl aws}"
# Cold runs per CLI; every metric is the median. One sample is noise even on a
# CI runner (gh 3.8 vs 5.4 s for the same commit, #230).
repeat="${PERF_REPEAT:-1}"
med() { sort -g | awk '{ v[NR] = $1 } END { print v[int((NR + 1) / 2)] }'; }
count_nodes() { python3 -I "$repo/scripts/metrics/perf_report.py" nodes "$1"; }
work="$(mktemp -d)"
mkdir -p "$work/cwd"
trap 'rm -rf "$work"' EXIT
(cd "$repo/cli/treemand" && go build -o "$work/treemand" .)
# Wall, user+sys CPU (children included) and max RSS in KB, on Linux and macOS.
timed() { out="$1"; shift; python3 -I "$repo/scripts/metrics/timed.py" "$out" "$work/cwd" -- "$@"; }
state() { git -C "$repo" branch --list; docker context show 2>/dev/null || true; }
state >"$work/before"

rows=""
skipped=""
for cli in $clis; do
  if ! command -v "$cli" >/dev/null 2>&1; then
    skipped+="$cli "
    continue
  fi
  cache="$work/cache-$cli"
  mkdir -p "$work/shim-$cli"
  printf '#!/bin/sh\necho >>"%s"\nexec "%s" "$@"\n' "$work/calls-$cli" "$(command -v "$cli")" >"$work/shim-$cli/$cli"
  chmod +x "$work/shim-$cli/$cli"
  : >"$work/runs-$cli"
  for _ in $(seq "$repeat"); do
    : >"$work/calls-$cli"
    PATH="$work/shim-$cli:$PATH" TREEMAND_CACHE_DIR="$cache" timed "$work/t" \
      "$work/treemand" --no-cache --output=json --timeout=120 "$cli" >"$work/out.json" 2>/dev/null || true
    read -r c u s m <"$work/t"
    echo "$c $(awk -v u="$u" -v s="$s" 'BEGIN { print u + s }') $m $(wc -l <"$work/calls-$cli") $(count_nodes "$work/out.json")" >>"$work/runs-$cli"
  done
  col() { awk -v k="$1" '{ print $k }' "$work/runs-$cli" | med; }
  cold="$(col 1)" cpu="$(col 2)" rss="$(col 3)" calls="$(col 4)" nodes="$(col 5)"
  spread="$(awk '{ print $1 }' "$work/runs-$cli" | sort -g | tr '\n' ' ')"
  (cd "$work/cwd" && TREEMAND_CACHE_DIR="$cache" "$work/treemand" --output=json --timeout=120 "$cli" >/dev/null 2>&1) || true
  TREEMAND_CACHE_DIR="$cache" timed "$work/t" "$work/treemand" --output=json "$cli" >/dev/null 2>&1 || true
  read -r warm _ <"$work/t"
  tti=""
  if [[ " $tui_clis " == *" $cli "* ]]; then
    tti="$(for r in $(seq "$repeat"); do
      (cd "$work/cwd" && TREEMAND_CACHE_DIR="$work/tti-$cli-$r" python3 -I "$repo/scripts/metrics/tti.py" "$work/treemand" "$cli")
    done | med)"
  fi
  rows+="$cli $cold $cpu $rss $warm $nodes ${tti:--} $calls ${spread// /,}"$'\n'
done

state >"$work/after"
if ! diff -q "$work/before" "$work/after" >/dev/null || [ -n "$(ls -A "$work/cwd")" ]; then
  echo "perf probe: discovery changed this machine (see treemand-safe-real-cli-run):" >&2
  diff "$work/before" "$work/after" >&2 || true
  ls -A "$work/cwd" >&2
  exit 1
fi

python3 -I "$repo/scripts/metrics/perf_report.py" report "$rows" "$skipped"
