#!/usr/bin/env bash
# Idempotently create the labels the loop uses.
set -euo pipefail
mk() { gh label create "$1" --color "$2" --description "$3" --force >/dev/null && echo "label: $1"; }
mk "loop:ready" 0e8a16 "Approved work the dev loop should pick up"
mk "loop:in-progress" fbca04 "The dev loop is working on this"
mk "loop:blocked" b60205 "Parked by the dev loop after repeated failure"
mk "loop:proposed" c5def5 "Idea hypothesized by the loop; promote with loop:ready"
mk "human:wip" 5319e7 "A human is working on this; the loop must skip it"
for a in tui cli ux site docs mcp discovery perf quality packaging; do
  mk "area:$a" bfd4f2 "Focus area: $a"
done
