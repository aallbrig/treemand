#!/bin/sh
# Behaviour tests for the dependency-health probe (scripts/metrics/
# dependency_health.py). A fixture repo with one problem of each kind, fake
# go and govulncheck first on PATH, and a local HTTP server standing in for
# go.dev, PyPI and the module proxy. Then the same repo offline, and without
# govulncheck. No real network is used.
here="$(cd "$(dirname "$0")" && pwd)"
probe="$here/dependency_health.py"
tmp="$(mktemp -d)"
server=""
# shellcheck disable=SC2329 # run by the trap
cleanup() { [ -n "$server" ] && kill "$server" 2>/dev/null; rm -rf "$tmp"; }
trap cleanup EXIT INT TERM
fail=0
# The fake PATH below hides the developer's tools; keep this python.
py="$(command -v python3)"

# get <json-file> <python expression over o> -> prints its value (or "absent")
get() { python3 -I "$here/json_get.py" "$1" "$2"; }
expect() { # expect <json-file> <expression> <value> <what>
  got="$(get "$1" "$2")"
  if [ "$got" = "$3" ]; then echo "ok    $4"; else echo "FAIL  $4: $2 = $got, want $3"; fail=1; fi
}

# ── the fixture repo ──────────────────────────────────────────────────────
repo="$tmp/repo"
mkdir -p "$repo/cli/treemand" "$repo/.github/workflows"
cat >"$repo/.github/workflows/ci.yml" <<'YML'
jobs:
  lint:
    steps:
      - uses: golangci/golangci-lint-action@v8
        with:
          version: latest
      - run: |
          go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.0
          pipx install ruff==0.10.0
          pipx install mypy==2.4.1
          pipx install checkov==3.2.529
      - uses: peaceiris/actions-hugo@v3
        with:
          hugo-version: "0.157.0"
YML
cat >"$repo/Taskfile.yml" <<'YML'
tasks:
  hugo:
    cmds:
      - echo hugo-version: "0.170.1"
YML

# ── fake go and govulncheck ───────────────────────────────────────────────
bin="$tmp/bin"
mkdir -p "$bin"
cat >"$bin/go" <<'SH'
#!/bin/sh
case "$*" in
  "mod edit -json") cat <<'J'
{"Module": {"Path": "example.com/m"}, "Go": "1.26.10",
 "Require": [{"Path": "example.com/old", "Version": "v1.0.0"},
             {"Path": "example.com/fresh", "Version": "v2.0.0"},
             {"Path": "example.com/behind", "Version": "v1.2.0"},
             {"Path": "example.com/indirect", "Version": "v0.1.0", "Indirect": true}]}
J
  ;;
  "list -m -u -json example.com/old example.com/fresh example.com/behind")
    [ -n "${FAKE_GO_LIST_ERR:-}" ] && { echo "go: $FAKE_GO_LIST_ERR" >&2; exit 0; }
    cat <<'J'
{"Path": "example.com/old", "Version": "v1.0.0", "Time": "2019-01-01T00:00:00Z"}
{"Path": "example.com/fresh", "Version": "v2.0.0", "Time": "2026-10-01T00:00:00Z"}
{"Path": "example.com/behind", "Version": "v1.2.0", "Time": "2020-01-01T00:00:00Z",
 "Update": {"Path": "example.com/behind", "Version": "v1.3.0", "Time": "2026-09-01T00:00:00Z"}}
J
  ;;
  *) echo "fake go: unexpected: $*" >&2; exit 2 ;;
esac
SH
cat >"$bin/govulncheck" <<'SH'
#!/bin/sh
[ "$*" = "-format json ./..." ] || { echo "fake govulncheck: unexpected: $*" >&2; exit 2; }
cat <<'J'
{"config": {"go_version": "go1.26.10"}}
{"osv": {"id": "GO-1", "summary": "called"}}
{"osv": {"id": "GO-2", "summary": "imported"}}
{"osv": {"id": "GO-3", "summary": "required"}}
{"finding": {"osv": "GO-1", "trace": [{"module": "example.com/a", "version": "v1.0.0"}]}}
{"finding": {"osv": "GO-1", "trace": [{"module": "example.com/a", "package": "example.com/a/p"}]}}
{"finding": {"osv": "GO-1", "trace": [{"module": "example.com/a", "package": "example.com/a/p", "function": "F"}]}}
{"finding": {"osv": "GO-2", "trace": [{"module": "example.com/b", "package": "example.com/b/q"}]}}
{"finding": {"osv": "GO-3", "trace": [{"module": "example.com/c"}]}}
J
SH
chmod +x "$bin/go" "$bin/govulncheck"

# ── the network, served from a directory ──────────────────────────────────
web="$tmp/web"
mkdir -p "$web/dl" "$web/pypi/ruff" "$web/pypi/mypy" "$web/pypi/checkov" \
  "$web/proxy/example.com/old/@latest" "$web/proxy/github.com/gohugoio/hugo" \
  "$web/proxy/github.com/golangci/golangci-lint/v2"
rmdir "$web/proxy/example.com/old/@latest"
echo '[{"version": "go1.27.4", "stable": true}, {"version": "go1.26.11", "stable": true}, {"version": "go1.28rc1", "stable": false}]' >"$web/dl/index.html"
echo '{"info": {"version": "0.17.6"}}' >"$web/pypi/ruff/json"
echo '{"info": {"version": "2.4.1"}}' >"$web/pypi/mypy/json"
# checkov has no fixture: its lookup 404s, which is not "offline".
echo '{"Version": "v1.0.0"}' >"$web/proxy/example.com/old/@latest"
echo '{"Version": "v0.170.1"}' >"$web/proxy/github.com/gohugoio/hugo/@latest"
echo '{"Version": "v2.15.0"}' >"$web/proxy/github.com/golangci/golangci-lint/v2/@latest"
python3 -I -u -m http.server --bind 127.0.0.1 --directory "$web" 0 >"$tmp/http.log" 2>&1 &
server=$!
port=""
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
  port="$(sed -n 's/.* port \([0-9][0-9]*\).*/\1/p' "$tmp/http.log" | head -n 1)"
  [ -n "$port" ] && break
  sleep 0.2
done
[ -n "$port" ] || { echo "FAIL  local HTTP server did not start"; cat "$tmp/http.log"; exit 1; }
url="http://127.0.0.1:$port"

run_probe() { # run_probe <out-file> [VAR=value ...]
  o="$1"; shift
  env PATH="$bin:/usr/bin:/bin" GOPROXY="$url/proxy" DEPENDENCY_HEALTH_GO_DL="$url/dl/?mode=json" \
    DEPENDENCY_HEALTH_PYPI="$url/pypi" "$@" "$py" -I "$probe" --root "$repo" >"$o" 2>"$o.err" \
    || { echo "FAIL  probe exited non-zero: $(cat "$o.err")"; fail=1; }
}

# ── online: one problem of each kind ──────────────────────────────────────
run_probe "$tmp/online.json"
expect "$tmp/online.json" 'o["metrics"]["vulns_reachable"]' 1 "a called vulnerable symbol is reachable (counted once over its three findings)"
expect "$tmp/online.json" 'o["metrics"]["vulns_imported_unreachable"]' 1 "an imported package without a call is imported-but-unreachable"
expect "$tmp/online.json" 'next(d for d in o["details"] if d["check"] == "vulns_required_only")["items"][0]["id"]' GO-3 "a module-only finding is listed, not counted"
expect "$tmp/online.json" 'o["metrics"]["go_modules_behind"]' 1 "direct modules with an update are behind; indirect ones are not looked at"
expect "$tmp/online.json" 'o["metrics"]["go_modules_stale"]' 1 "a module whose latest release is over two years old is stale; an old version with a fresh update is not"
expect "$tmp/online.json" 'o["metrics"]["go_toolchain_behind"]' 1 "go 1.26.10 is behind go1.26.11 on its line"
expect "$tmp/online.json" 'o["metrics"]["tools_behind"]' 3 "ruff, hugo (one of two pins) and golangci-lint are behind; mypy is current"
expect "$tmp/online.json" 'next(d for d in o["details"] if d["check"] == "tools_behind")["lookup_failed"]' "['checkov']" "a tool whose lookup 404s is reported, not counted"
expect "$tmp/online.json" 'len(next(d for d in o["details"] if d["check"] == "tools_behind")["unpinned"])' 1 "golangci-lint at version: latest is unpinned"
expect "$tmp/online.json" '"skipped" in o' False "nothing is skipped online"
gets="$(grep -c '"GET ' "$tmp/http.log")"
# go.dev once, the proxy check once, one lookup per tool (hugo's two pins share one).
if [ "$gets" -eq 7 ]; then echo "ok    each URL is fetched once per run ($gets fetches)"; else echo "FAIL  $gets fetches, want 7"; fail=1; fi

# ── toolchain on the latest patch, and on an unsupported line ─────────────
echo '[{"version": "go1.26.10", "stable": true}]' >"$web/dl/index.html"
run_probe "$tmp/current.json"
expect "$tmp/current.json" 'o["metrics"]["go_toolchain_behind"]' 0 "go 1.26.10 is current when it is the newest stable on its line"
echo '[{"version": "go1.28.1", "stable": true}, {"version": "go1.27.9", "stable": true}]' >"$web/dl/index.html"
run_probe "$tmp/unsupported.json"
expect "$tmp/unsupported.json" 'o["metrics"]["go_toolchain_behind"]' 1 "a line go.dev no longer lists is behind"

# ── go list errors: modules skipped, not reported as up to date ───────────
run_probe "$tmp/golist.json" FAKE_GO_LIST_ERR="loading module retractions: connection refused"
expect "$tmp/golist.json" 'o["skipped"]["go_modules_behind"].startswith("go list -m -u failed")' True "a go list error skips go_modules_behind"
expect "$tmp/golist.json" '"go_modules_stale" in o["metrics"]' False "and go_modules_stale"

# ── offline: everything that needs the network is skipped ─────────────────
kill "$server" 2>/dev/null
wait "$server" 2>/dev/null
server=""
run_probe "$tmp/offline.json"
expect "$tmp/offline.json" 'len(o["metrics"])' 0 "offline: no metric is reported"
expect "$tmp/offline.json" 'sorted(o["skipped"])' "['go_modules_behind', 'go_modules_stale', 'go_toolchain_behind', 'tools_behind', 'vulns_imported_unreachable', 'vulns_reachable']" "offline: every metric is skipped"
expect "$tmp/offline.json" 'next(d for d in o["details"] if d["check"] == "network")["fetches"]' 1 "offline: one failed fetch stops the rest"
run_probe "$tmp/forced.json" DEPENDENCY_HEALTH_OFFLINE=1
expect "$tmp/forced.json" 'next(d for d in o["details"] if d["check"] == "network")["fetches"]' 0 "DEPENDENCY_HEALTH_OFFLINE=1 makes no fetch"

# ── without govulncheck ───────────────────────────────────────────────────
rm "$bin/govulncheck"
run_probe "$tmp/novuln.json" DEPENDENCY_HEALTH_OFFLINE=1
expect "$tmp/novuln.json" 'o["skipped"]["vulns_reachable"].startswith("offline")' True "offline wins over a missing govulncheck"

exit "$fail"
