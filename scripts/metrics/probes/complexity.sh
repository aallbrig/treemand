#!/usr/bin/env bash
# Probe: code complexity of treemand's Go code (tests excluded).
#   metrics: cognitive complexity (max, average, functions over 15 and 30),
#            non-test Go lines, files over 800 lines and the length of each
#            (file_lines.<path>), clone groups of 50+ tokens (dupl_clones),
#            if statements of nesting complexity 4+ (nestif_over_3)
#   details: the 10 most complex functions, and churn x complexity hotspots
#            (files changed most in the last 90 days that hold complex functions)
set -euo pipefail
cd "$(dirname "$0")/../../.."
# COMPLEXITY_ROOT points the probe at a fixture tree (complexity_probe_test.sh).
root="${COMPLEXITY_ROOT:-cli/treemand}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
GOFLAGS='' go run github.com/uudashr/gocognit/cmd/gocognit@v1.3.0 -test=false -json "$root" >"$work/cog.json" 2>/dev/null || true
churn="$(git log --since=90.days --numstat --format= -- cli/treemand | awk 'NF==3 && $3 ~ /\.go$/ && $3 !~ /_test\.go$/ {c[$3]+=$1+$2} END {for (f in c) print c[f] "\t" f}')"
find "$root" -name '*.go' ! -name '*_test.go' >"$work/files.txt"
loc="$(tr '\n' '\0' <"$work/files.txt" | xargs -0 wc -l | awk '$2 != "total" {print $1 "\t" $2}')"
# Clones and nesting: the duplicated policy and deep branching a per-function
# score misses (#475). Thresholds sit below golangci-lint's defaults, which
# find nothing in this tree, so a new clone or nest moves the number.
GOFLAGS='' go run github.com/mibk/dupl@v1.0.0 -files -plumbing -t 50 <"$work/files.txt" >"$work/dupl.txt"
files=()
while IFS= read -r f; do files+=("$f"); done <"$work/files.txt"
GOFLAGS='' go run github.com/nakabonne/nestif/cmd/nestif@v0.3.1 --json --min 4 --top 1000000 "${files[@]}" >"$work/nestif.json"
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
with open(os.path.join(work, "nestif.json")) as f:
    nests = json.loads(f.read() or "null") or []
# dupl prints each fragment of a pair ("a:1-9: duplicate of b:3-11"); fragments
# linked by a pair form one clone group.
parent = {}
def find(x):
    parent.setdefault(x, x)
    while parent[x] != x:
        parent[x] = parent[parent[x]]
        x = parent[x]
    return x
with open(os.path.join(work, "dupl.txt")) as f:
    for line in f:
        a, sep, b = line.strip().partition(": duplicate of ")
        if sep:
            parent[find(a)] = find(b)
clone_groups = len({find(x) for x in list(parent)})
long_files = {p: n for p, n in loc.items() if n > 800}
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
        "files_over_800_lines": len(long_files),
        "dupl_clones": clone_groups,
        "nestif_over_3": len(nests),
        **{f"file_lines.{p}": n for p, n in sorted(long_files.items())},
    },
    "details": [
        {"top_functions": [{"complexity": f["Complexity"], "func": f["FuncName"], "pkg": f["PkgName"],
                            "at": f"{f['Pos']['Filename']}:{f['Pos']['Line']}"} for f in top]},
        {"hotspots_churn_x_complexity": [{"score": s, "file": p, "churn_lines_90d": ch, "max_complexity": c}
                                         for s, p, ch, c in hot if s > 0]},
        {"files_over_800_lines": sorted(long_files)},
        {"nestif_over_3": [{"at": f"{n['Pos']['Filename']}:{n['Pos']['Line']}", "complexity": n["Complexity"],
                            "message": n["Message"]} for n in nests]},
    ],
}))
PY
