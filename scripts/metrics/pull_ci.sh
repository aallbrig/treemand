#!/usr/bin/env bash
# Records the latest successful "Measure timing" run on main into docs/metrics,
# under the runner's stable host name, so timing ratchets have a baseline that
# a loaded development host cannot distort. Prints each probe's verdicts.
set -euo pipefail
cd "$(dirname "$0")/../.."
run="$(gh run list --workflow measure-timing.yml --branch main --status success --limit 1 \
  --json databaseId --jq '.[0].databaseId // empty')"
if [[ -z "$run" ]]; then
  echo "measure:pull: no successful Measure timing run on main yet" >&2
  exit 1
fi
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
gh run download "$run" -n timing -D "$work"
export MEASURE_HOST=gha-ubuntu-latest
MEASURE_LOAD="$(cat "$work/load.txt")"
export MEASURE_LOAD
echo "measure:pull: run $run (runner load per CPU $MEASURE_LOAD)"
for p in tui perf; do
  python3 -I scripts/metrics/measure.py "$p" --input "$work/$p.json" --record
done
