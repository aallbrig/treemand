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

python3 -I - "$tmp" <<'PY'
import json, os, sys
tmp = sys.argv[1]

def rel(tag, published, counts, draft=False, pre=False):
    return {"tag_name": tag, "published_at": published, "draft": draft, "prerelease": pre,
            "assets": [{"name": f"a{i}.tar.gz", "download_count": c} for i, c in enumerate(counts)]}

def issue(n, assoc, kind="User", pr=False):
    i = {"number": n, "author_association": assoc, "user": {"login": f"person{n}", "type": kind}}
    if pr:
        i["pull_request"] = {}
    return i

issues = [issue(1, "OWNER"), issue(2, "NONE"), issue(3, "CONTRIBUTOR"), issue(4, "NONE", kind="Bot"),
          issue(5, "NONE", pr=True), issue(6, "COLLABORATOR")]
repo = {"full_name": "acme/tool", "stargazers_count": 12, "forks_count": 3, "subscribers_count": 2}
healthy = {
    "repos/acme/tool": repo,
    "repos/acme/tool/releases": [
        rel("v1.3.0-rc1", "2026-03-09T00:00:00Z", [50], pre=True),
        rel("v1.2.0", "2026-03-08T00:00:00Z", [4, 6]) | {"assets": [
            {"name": "a0.tar.gz", "download_count": 4}, {"name": "a1.zip", "download_count": 6},
            {"name": "checksums.txt", "download_count": 9}, {"name": "checksums.txt.sigstore.json", "download_count": 9}]},
        rel("v1.3.0", "2026-03-11T00:00:00Z", [99], draft=True),
        rel("v1.1.0", "2026-02-01T00:00:00Z", [30, 10]),
    ],
    "repos/acme/tool/issues?state=open": issues,
    "repos/acme/tool/traffic/views": {"count": 40, "uniques": 9},
    "repos/acme/tool/traffic/clones": {"count": 300, "uniques": 70},
    "repos/acme/tool/traffic/popular/referrers": [{"referrer": "news.example", "count": 7, "uniques": 5},
                                                  {"referrer": "github.com", "count": 3, "uniques": 2}],
    "repos/acme/homebrew-tap/traffic/clones": {"count": 25, "uniques": 11},
}
broken = dict(healthy)
broken["repos/acme/tool/releases"] = [rel("v1.2.0", "2026-03-02T00:00:00Z", [0, 0]), rel("v1.1.0", "2026-02-01T00:00:00Z", [5])]
for k in ("views", "clones", "popular/referrers"):
    broken[f"repos/acme/tool/traffic/{k}"] = {"__error": "gh: Must have push access to repository (HTTP 403)"}
json.dump(healthy, open(f"{tmp}/healthy.json", "w"))
json.dump(broken, open(f"{tmp}/broken.json", "w"))
os.makedirs(f"{tmp}/proxy/github.com/acme/tool/@v")
json.dump({"Version": "v1.2.0"}, open(f"{tmp}/proxy/github.com/acme/tool/@latest", "w"))
os.makedirs(f"{tmp}/brew/formula")
with open(f"{tmp}/history.jsonl", "w") as f:
    for date, stars in (("2026-03-01T00:00:00Z", 8), ("2026-03-02T12:00:00Z", 10), ("2026-03-08T00:00:00Z", 11)):
        f.write(json.dumps({"date": date, "host": "github-api", "metrics": {"stars": stars, "downloads_total": 20}}) + "\n")
PY

mkdir -p "$tmp/bin"
cat >"$tmp/bin/gh" <<'PY'
#!/usr/bin/env python3
# Fake gh: `gh api -X GET <path>` answered from $GH_WORLD; lists are paginated by per_page/page.
import json, os, sys
from urllib.parse import parse_qs, urlencode
path = sys.argv[-1]
base, _, query = path.partition("?")
q = {k: v[0] for k, v in parse_qs(query).items()}
per, page = int(q.pop("per_page", 30)), int(q.pop("page", 1))
key = base + ("?" + urlencode(q) if q else "")
world = json.load(open(os.environ["GH_WORLD"]))
if key not in world:
    sys.exit(f"gh: Not Found (HTTP 404) {key}")
body = world[key]
if isinstance(body, dict) and "__error" in body:
    sys.exit(body["__error"])
if isinstance(body, list):
    body = body[(page - 1) * per : page * per]
print(json.dumps(body))
PY
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
