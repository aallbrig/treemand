#!/usr/bin/env bash
# Probe: code complexity of treemand's Go code (tests excluded).
#   metrics: cognitive complexity (max and functions over 15 and 30 of non-test
#            code; cognitive_avg still averages test functions too, as its
#            history does, until a gates:loosen PR redefines it, #475),
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
GOFLAGS='' go run github.com/uudashr/gocognit/cmd/gocognit@v1.3.0 -json "$root" >"$work/cog.json" 2>/dev/null || true
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
python3 -I scripts/metrics/complexity_report.py "$work"
