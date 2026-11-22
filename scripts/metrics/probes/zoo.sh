#!/bin/sh
# Probe (heavy, weekly): treemand across the cli-zoo container. Builds the
# image from this tree, then:
#   sweep: every executable on the container's PATH, one level deep
#          -> sweep_programs, sweep_errors (non-zero exit), sweep_timeouts,
#             sweep_panics, sweep_error_pct
#   smoke: full discovery of each zoo CLI -> nodes.<cli>, flags.<cli>
# Real programs only ever run inside the disposable container.
set -eu
cd "$(dirname "$0")/../../.."
docker build -q -t treemand-cli-zoo -f test/cli-zoo/Dockerfile . >/dev/null
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
docker run --rm treemand-cli-zoo sweep >"$work/sweep.tsv" 2>/dev/null || true
docker run --rm treemand-cli-zoo smoke >"$work/smoke.tsv" 2>/dev/null || true
python3 -I - "$work" <<'PY'
import json, os, sys
work = sys.argv[1]
rows = [l.split("\t") for l in open(os.path.join(work, "sweep.tsv")).read().splitlines() if l.count("\t") == 3]
errors = [r for r in rows if r[1] not in ("0",)]
timeouts = [r for r in rows if r[1] == "124"]
panics = [r for r in rows if r[3] == "1"]
metrics = {
    "sweep_programs": len(rows),
    "sweep_errors": len(errors),
    "sweep_timeouts": len(timeouts),
    "sweep_panics": len(panics),
    "sweep_error_pct": round(100 * len(errors) / max(len(rows), 1), 1),
}
for line in open(os.path.join(work, "smoke.tsv")).read().splitlines()[1:]:
    cli, rc, secs, nodes, flags = line.split("\t")
    metrics[f"nodes.{cli}"] = int(nodes)
    metrics[f"flags.{cli}"] = int(flags)
print(json.dumps({"probe": "zoo", "metrics": metrics, "details": [
    {"errors": sorted(f"{r[0]} (exit {r[1]})" for r in errors)[:40]},
    {"panics": sorted(r[0] for r in panics)},
    {"timeouts": sorted(r[0] for r in timeouts)},
]}))
PY
