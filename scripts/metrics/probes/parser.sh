#!/usr/bin/env bash
# Probe: help-parser quality on the corpus (no real CLIs run).
#   metrics: corpus_clis, truth_pass_pct (CLIs meeting expect.json ground
#            truth), clis_with_gaps (known, issue-tracked), unexpected_misses
#            (failures with no gap marker: a regression), subcommands_total,
#            flags_total, described_pct.
#   details: every CLI that misses its ground truth, with what it misses.
set -euo pipefail
cd "$(dirname "$0")/../../../cli/treemand"
corpus=discovery/testdata/corpus
go run ./tools/corpusdump "$corpus" | python3 -I -c '
import json, sys
corpus = sys.argv[1]
facts = {}
for line in sys.stdin:
    cli, kind, value = line.rstrip("\n").split("\t", 2)
    f = facts.setdefault(cli, {"desc": "", "subs": set(), "flags": set()})
    if kind == "desc": f["desc"] = value
    elif kind == "sub": f["subs"].add(value)
    elif kind == "flag": f["flags"].add(value.split(" ")[0])
expect = {k: v for k, v in json.load(open(corpus + "/expect.json")).items() if not k.startswith("_")}
passed, gapped, unexpected, details = 0, 0, 0, []
for cli, f in sorted(facts.items()):
    e = expect.get(cli, {})
    miss = [f"subcommand {s}" for s in e.get("subcommands_include", []) if s not in f["subs"]]
    if "max_subcommands" in e and len(f["subs"]) > e["max_subcommands"]:
        miss.append(f"{len(f['subs'])} subcommands > {e['max_subcommands']}")
    miss += [f"flag {x}" for x in e.get("flags_include", []) if x not in f["flags"]]
    if e.get("description_include") and e["description_include"] not in f["desc"]:
        miss.append("description")
    if e.get("description_exclude") and e["description_exclude"] in f["desc"]:
        miss.append("garbage description")
    if not miss:
        passed += 1
        continue
    if e.get("gaps"):
        gapped += 1
    else:
        unexpected += 1
    details.append({"cli": cli, "misses": miss, "gaps": e.get("gaps", [])})
n = len(facts)
print(json.dumps({"probe": "parser", "metrics": {
    "corpus_clis": n,
    "truth_pass_pct": round(100 * passed / max(n, 1), 1),
    "clis_with_gaps": gapped,
    "unexpected_misses": unexpected,
    "subcommands_total": sum(len(f["subs"]) for f in facts.values()),
    "flags_total": sum(len(f["flags"]) for f in facts.values()),
    "described_pct": round(100 * sum(bool(f["desc"]) for f in facts.values()) / max(n, 1), 1),
}, "details": details}))
' "$corpus"
