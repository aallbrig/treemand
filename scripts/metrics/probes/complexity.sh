#!/usr/bin/env bash
# Probe: code complexity of treemand's Go code (tests excluded).
#   metrics: cognitive complexity (max, average, functions over 15 and 30),
#            non-test Go lines, files over 800 lines
#   details: the 10 most complex functions, and churn x complexity hotspots
#            (files changed most in the last 90 days that hold complex functions)
set -euo pipefail
cd "$(dirname "$0")/../../.."
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
GOFLAGS='' go run github.com/uudashr/gocognit/cmd/gocognit@v1.3.0 -json cli/treemand >"$work/cog.json" 2>/dev/null || true
churn="$(git log --since=90.days --numstat --format= -- cli/treemand | awk 'NF==3 && $3 ~ /\.go$/ && $3 !~ /_test\.go$/ {c[$3]+=$1+$2} END {for (f in c) print c[f] "\t" f}')"
loc="$(find cli/treemand -name '*.go' ! -name '*_test.go' -print0 | xargs -0 wc -l | awk '$2 != "total" {print $1 "\t" $2}')"
printf '%s\n' "$churn" >"$work/churn.tsv"
printf '%s\n' "$loc" >"$work/loc.tsv"
python3 -I - "$work" <<'PY'
import json, os, sys
work = sys.argv[1]
def tsv(name):
    with open(os.path.join(work, name)) as f:
        return {p: int(n) for n, p in (l.rstrip("\n").split("\t") for l in f if l.strip())}
with open(os.path.join(work, "cog.json")) as f:
    funcs = json.loads(f.read() or "[]")
churn, loc = tsv("churn.tsv"), tsv("loc.tsv")
cx = [f["Complexity"] for f in funcs]
per_file = {}
for f in funcs:
    path = f["Pos"]["Filename"]
    per_file[path] = max(per_file.get(path, 0), f["Complexity"])
top = sorted(funcs, key=lambda f: -f["Complexity"])[:10]
hot = sorted(((churn.get(p, 0) * c, p, churn.get(p, 0), c) for p, c in per_file.items()), reverse=True)[:10]
print(json.dumps({
    "probe": "complexity",
    "metrics": {
        "cognitive_max": max(cx, default=0),
        "cognitive_avg": round(sum(cx) / len(cx), 2) if cx else 0,
        "functions_over_15": sum(c > 15 for c in cx),
        "functions_over_30": sum(c > 30 for c in cx),
        "go_loc": sum(loc.values()),
        "files_over_800_lines": sum(n > 800 for n in loc.values()),
    },
    "details": [
        {"top_functions": [{"complexity": f["Complexity"], "func": f["FuncName"], "pkg": f["PkgName"],
                            "at": f"{f['Pos']['Filename']}:{f['Pos']['Line']}"} for f in top]},
        {"hotspots_churn_x_complexity": [{"score": s, "file": p, "churn_lines_90d": ch, "max_complexity": c}
                                         for s, p, ch, c in hot if s > 0]},
        {"files_over_800_lines": sorted(p for p, n in loc.items() if n > 800)},
    ],
}))
PY
