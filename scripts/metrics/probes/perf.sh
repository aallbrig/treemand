#!/usr/bin/env bash
# Probe: treemand performance on real installed CLIs (PERF_CLIS overrides the set).
#   metrics per CLI: cold_s (uncached full discovery), warm_s (cache hit),
#   cpu_s (user+sys CPU of the cold run, child processes included: the cost
#   that load cannot hide), rss_mb, nodes, and tti_s (seconds until -i shows
#   the tree) for the TUI_CLIS subset.
# Runs from an empty scratch directory, per treemand-safe-real-cli-run, and
# fails if discovery changed git branches or the Docker context.
set -euo pipefail
repo="$(cd "$(dirname "$0")/../../.." && pwd)"
clis="${PERF_CLIS:-git docker kubectl gh npm pip go helm apt systemctl tar aws}"
tui_clis="${TUI_CLIS:-docker gh kubectl aws}"
work="$(mktemp -d)"
mkdir -p "$work/cwd"
trap 'rm -rf "$work"' EXIT
(cd "$repo/cli/treemand" && go build -o "$work/treemand" .)
state() { git -C "$repo" branch --list; docker context show 2>/dev/null || true; }
state >"$work/before"

rows=""
for cli in $clis; do
  command -v "$cli" >/dev/null 2>&1 || continue
  cache="$work/cache-$cli"
  /usr/bin/time -o "$work/t" -f "%e %U %S %M" \
    env -C "$work/cwd" TREEMAND_CACHE_DIR="$cache" "$work/treemand" --no-cache --output=json --timeout=120 "$cli" >"$work/out.json" 2>/dev/null || true
  read -r cold user sys rss <"$work/t"
  env -C "$work/cwd" TREEMAND_CACHE_DIR="$cache" "$work/treemand" --output=json --timeout=120 "$cli" >/dev/null 2>&1 || true
  /usr/bin/time -o "$work/t" -f "%e" env -C "$work/cwd" TREEMAND_CACHE_DIR="$cache" "$work/treemand" --output=json "$cli" >/dev/null 2>&1 || true
  warm="$(cat "$work/t")"
  nodes="$(python3 -I -c 'import json,sys
def n(x): return 1 + sum(n(c) for c in x.get("children") or [])
try: print(n(json.load(open(sys.argv[1]))))
except Exception: print(0)' "$work/out.json")"
  tti=""
  if [[ " $tui_clis " == *" $cli "* ]]; then
    tti="$(cd "$work/cwd" && TREEMAND_CACHE_DIR="$work/tti-$cli" python3 -I "$repo/scripts/metrics/tti.py" "$work/treemand" "$cli")"
  fi
  rows+="$cli $cold $user $sys $rss $warm $nodes ${tti:--}"$'\n'
done

state >"$work/after"
if ! diff -q "$work/before" "$work/after" >/dev/null || [ -n "$(ls -A "$work/cwd")" ]; then
  echo "perf probe: discovery changed this machine (see treemand-safe-real-cli-run):" >&2
  diff "$work/before" "$work/after" >&2 || true
  ls -A "$work/cwd" >&2
  exit 1
fi

python3 -I - "$rows" "$(cat /proc/loadavg)" <<'PY'
import json, sys
metrics, details = {}, []
for line in sys.argv[1].splitlines():
    if not line.strip():
        continue
    cli, cold, user, sys_, rss, warm, nodes, tti = line.split()
    metrics[f"cold_s.{cli}"] = float(cold)
    metrics[f"warm_s.{cli}"] = float(warm)
    metrics[f"cpu_s.{cli}"] = round(float(user) + float(sys_), 2)
    metrics[f"rss_mb.{cli}"] = round(int(rss) / 1024, 1)
    metrics[f"nodes.{cli}"] = int(nodes)
    if tti != "-":
        metrics[f"tti_s.{cli}"] = float(tti)
    details.append({"cli": cli, "cold_s": float(cold), "cpu_s": round(float(user) + float(sys_), 2),
                    "cpu_per_wall": round((float(user) + float(sys_)) / max(float(cold), 0.01), 1)})
print(json.dumps({"probe": "perf", "metrics": metrics,
                  "details": [{"loadavg": sys.argv[2]}, {"per_cli": details}]}))
PY
