#!/bin/sh
# Behaviour tests for the adoption-signals probe (scripts/metrics/adoption_signals.py),
# against a fake `gh` first on PATH that serves a small GitHub API world and
# paginates like GitHub, and file:// stand-ins for the module proxy and the
# Homebrew API. Two worlds: a healthy one where traffic is readable, and one
# whose latest release is 8 days old with no downloads and whose traffic API
# is forbidden.
here="$(cd "$(dirname "$0")" && pwd)"
probe="$here/adoption_signals.py"
engine="$here/measure.py"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0
export GH_REPO=acme/tool ADOPTION_PER_PAGE=2 ADOPTION_NOW=2026-03-10T00:00:00+00:00 \
  ADOPTION_PROXY="file://$tmp/proxy" ADOPTION_BREW_API="file://$tmp/brew" ADOPTION_HISTORY="$tmp/history.jsonl"

metric() { python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["metrics"].get(sys.argv[2], "absent"))' "$1" "$2"; }
expect() { # expect <json-file> <metric> <value> <what>
  got="$(metric "$1" "$2")"
  if [ "$got" = "$3" ]; then echo "ok    $4"; else echo "FAIL  $4: $2 = $got, want $3"; fail=1; fi
}
check() { # check <json-file> <what> <python expression over d (the JSON)>
  if python3 -I -c 'import json,sys; d=json.load(open(sys.argv[1])); sys.exit(0 if eval(sys.argv[2]) else 1)' "$1" "$3"; then
    echo "ok    $2"
  else
    echo "FAIL  $2"
    fail=1
  fi
}

python3 -I "$here/adoption_signals_test_world.py" "$tmp"

mkdir -p "$tmp/bin"
cp "$here/adoption_signals_test_fake_gh.py" "$tmp/bin/gh"
chmod +x "$tmp/bin/gh"
PATH="$tmp/bin:$PATH"

GH_WORLD="$tmp/healthy.json" python3 -I "$probe" >"$tmp/healthy.out" 2>"$tmp/err" ||
  { echo "FAIL  probe exited non-zero"; cat "$tmp/err"; exit 1; }
h="$tmp/healthy.out"
expect "$h" stars 12                     "stars from the repository"
expect "$h" watchers 2                   "watchers are subscribers, not stargazers"
expect "$h" downloads_total 100          "archive downloads over every published release, prereleases in, drafts out, all pages"
expect "$h" downloads_latest 10          "the latest release is the newest published non-prerelease; checksums and bundles not counted"
expect "$h" latest_release_age_days 2.0  "the latest release's age in days"
expect "$h" latest_zero_downloads_7d 0   "a downloaded release is not an alert"
expect "$h" views_14d 40                 "traffic views"
expect "$h" clones_uniques_14d 70        "traffic unique clones"
expect "$h" referrers_14d 10             "referred views summed over the top referrers"
expect "$h" tap_clones_14d 25            "clones of the Homebrew tap"
expect "$h" proxy_latest_is_release 1    "the module proxy has seen the latest release"
expect "$h" external_open_issues 2       "external issues: owner, collaborator, bot and pull request left out"
expect "$h" brew_installs_30d absent     "a tap formula has no public install analytics"
check "$h" "the Homebrew skip says why" '"homebrew/core" in d["skipped"]["brew_installs"]'
check "$h" "week over week compares with the newest record at least a week old" \
  'any(x.get("since") == "2026-03-02T12:00:00Z" and x["deltas"] == {"stars": 2, "downloads_total": 80} for x in d["details"])'
check "$h" "the latest release's downloads are listed per asset" \
  'any(x["kind"] == "latest_release" and x["assets"]["checksums.txt"] == 9 and x["assets"]["a1.zip"] == 6 for x in d["details"])'
if grep -q person "$h"; then echo "FAIL  an issue author's name leaked into the result"; fail=1; else echo "ok    no issue author names in the result"; fi

GH_WORLD="$tmp/broken.json" python3 -I "$probe" >"$tmp/broken.out" 2>"$tmp/err" ||
  { echo "FAIL  probe exited non-zero on the broken world"; cat "$tmp/err"; exit 1; }
b="$tmp/broken.out"
expect "$b" latest_zero_downloads_7d 1   "no downloads 8 days after release is the alert"
expect "$b" views_14d absent             "forbidden traffic gives no traffic metrics"
expect "$b" tap_clones_14d 25            "the tap's traffic is read on its own"
expect "$b" proxy_latest_is_release 1    "proxy compared with the latest tag"
check "$b" "forbidden traffic is skipped with gh's reason" '"HTTP 403" in d["skipped"]["traffic"]'

python3 -I "$engine" adoption-signals --input "$b" --findings >"$tmp/findings.json" 2>"$tmp/err" ||
  { echo "FAIL  engine exited non-zero"; cat "$tmp/err"; fail=1; }
check "$tmp/findings.json" "the engine reports the alert as a budget finding" \
  '[f["metric"] for f in d["findings"] if f["kind"] == "budget"] == ["latest_zero_downloads_7d"]'
python3 -I "$engine" adoption-signals --input "$h" --findings >"$tmp/findings.json" 2>"$tmp/err"
check "$tmp/findings.json" "trend metrics carry no rule: a healthy run has no budget or regression" \
  'not [f for f in d["findings"] if f["kind"] in ("budget", "regression")]'

rm "$tmp/history.jsonl"
GH_WORLD="$tmp/healthy.json" python3 -I "$probe" >"$h" 2>"$tmp/err"
check "$h" "without history the delta says so" 'any("no history" in x.get("note", "") for x in d["details"])'
exit "$fail"
