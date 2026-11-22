#!/bin/sh
# Hermetic tests for the install probe (scripts/metrics/probes/install.sh and
# scripts/metrics/install_paths.sh): fake releases and a fake treemand.dev are
# served from a local HTTP server, so nothing here touches the network, the real
# release or anything outside a temp dir. Run through `task install:probe:test`.
cd "$(dirname "$0")/../.." || exit 1
repo="$PWD"
work="$(mktemp -d)"
server_pid=""
cleanup() { [ -z "$server_pid" ] || kill "$server_pid" 2>/dev/null; rm -rf "$work"; }
trap cleanup EXIT INT TERM

pass=0
fail=0
ok() { pass=$((pass + 1)); printf '  ok   %s\n' "$1"; }
bad() { fail=$((fail + 1)); printf '  FAIL %s\n' "$1"; }
has() { # has NAME HAYSTACK NEEDLE
  case "$2" in *"$3"*) ok "$1" ;; *) bad "$1 (wanted $3 in: $2)" ;; esac
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

# make_release SITE TAG REPORTS RENDERS: a release whose binary says it is
# REPORTS and prints a tree for `ls` when RENDERS is 1.
make_release() {
  dir="$1/releases/$2"
  mkdir -p "$dir"
  : >"$dir/checksums.txt"
  for os in linux darwin; do
    for arch in amd64 arm64; do
      stage="$work/stage.$os.$arch"
      rm -rf "$stage" && mkdir -p "$stage"
      cat >"$stage/treemand_${os}_${arch}" <<EOF
#!/bin/sh
case "\$1" in
  version) echo "treemand $3 (fake)" ;;
  --depth=1) [ "$4" = 1 ] && printf '{\n  "name": "ls",\n  "discovered": true\n}\n' ;;
esac
exit 0
EOF
      chmod +x "$stage/treemand_${os}_${arch}"
      printf '.TH TREEMAND 1\n' >"$stage/treemand.1"
      tar -czf "$dir/treemand_${os}_${arch}.tar.gz" -C "$stage" "treemand_${os}_${arch}" treemand.1
      printf '%s  treemand_%s_%s.tar.gz\n' "$(sha256 "$dir/treemand_${os}_${arch}.tar.gz")" "$os" "$arch" >>"$dir/checksums.txt"
    done
  done
}

# make_site NAME: a fake treemand.dev with v1.0.0 (latest) and v0.9.0.
make_site() {
  s="$work/www/$1"
  mkdir -p "$s/installation" "$s/releases"
  cp "$repo/www/treemand/static/install.sh" "$s/install.sh"
  printf 'v1.0.0\n' >"$s/releases/latest.txt"
  printf '<h2>Update or uninstall</h2><p>delete <code>~/.local/bin/treemand</code> and <code>~/.local/share/man/man1/treemand.1</code></p>\n' >"$s/installation/index.html"
}

make_site good
make_release "$work/www/good" v1.0.0 v1.0.0 1
make_release "$work/www/good" v0.9.0 v0.9.0 1

make_site norender
make_release "$work/www/norender" v1.0.0 v1.0.0 0
make_release "$work/www/norender" v0.9.0 v0.9.0 1

make_site badpin # the v0.9.0 archive holds a binary that says it is v1.0.0
make_release "$work/www/badpin" v1.0.0 v1.0.0 1
make_release "$work/www/badpin" v0.9.0 v1.0.0 1

make_site nodocs
make_release "$work/www/nodocs" v1.0.0 v1.0.0 1
make_release "$work/www/nodocs" v0.9.0 v0.9.0 1
printf '<h2>Installation</h2>\n' >"$work/www/nodocs/installation/index.html"

port="${INSTALL_PROBE_TEST_PORT:-18292}"
python3 -I -m http.server "$port" --bind 127.0.0.1 --directory "$work/www" >"$work/server.log" 2>&1 &
server_pid=$!
i=0
while [ $i -lt 50 ] && ! curl -sf "http://127.0.0.1:$port/good/releases/latest.txt" >/dev/null; do i=$((i + 1)); sleep 0.1; done

# probe SITE: the whole probe against a fake site, containers and Go off.
probe() {
  PROBE_SITE="http://127.0.0.1:$port/$1" PROBE_PREVIOUS=v0.9.0 PROBE_DOCKER=0 \
    PROBE_PATHS="oneliner upgrade reinstall uninstall" sh scripts/metrics/probes/install.sh 2>"$work/probe.err"
}
# field JSON EXPR: evaluate EXPR on the parsed probe output (r).
field() {
  printf '%s' "$1" | python3 -I -c 'import json,sys; r=json.load(sys.stdin); d={x["path"].split(".",1)[1]: x for x in r["details"] if "path" in x}; print(eval(sys.argv[1]))' "$2"
}

echo "install probe"

out="$(probe good)"; rc=$?
if [ $rc -eq 0 ]; then ok "the probe exits 0"; else bad "the probe exits 0 (rc=$rc: $(cat "$work/probe.err"))"; fi
has "every path passes on a good release" "$(field "$out" 'r["metrics"]["paths_ok"], r["metrics"]["paths_failed"]')" "(4, 0)"
has "each path has a per-path metric" "$(field "$out" 'sorted(k for k in r["metrics"] if k.startswith("ok."))')" ".oneliner"
has "the installed version is compared with the latest tag" "$(field "$out" 'd["oneliner"]["version"], d["oneliner"]["want"]')" "('v1.0.0', 'v1.0.0')"
has "the binary renders a tree" "$(field "$out" 'd["oneliner"]["renders"]')" "1"
has "the PATH hint is recorded" "$(field "$out" 'd["oneliner"]["path_hint"]')" "not on your PATH"
has "upgrade goes from the previous release to the latest" "$(field "$out" 'd["upgrade"]["note"]')" "v0.9.0 -> v1.0.0"
has "containers are skipped with a reason when docker is off" "$(field "$out" 'r["skipped"]')" "PROBE_DOCKER=0"
has "brew is skipped unless opted in" "$(field "$out" 'r["skipped"]')" "PROBE_BREW"

out="$(probe norender)"
has "a binary that renders no tree fails its path" "$(field "$out" 'd["oneliner"]["status"], d["oneliner"]["note"]')" "('fail', 'no tree"
has "paths_failed counts it" "$(field "$out" 'r["metrics"]["paths_failed"] > 0')" "True"

out="$(probe badpin)"
has "a pinned install that reports the wrong version fails upgrade" "$(field "$out" 'd["upgrade"]["status"], d["upgrade"]["note"]')" "('fail', '--version v0.9.0 installed v1.0.0"

out="$(probe nodocs)"
has "uninstall fails when the docs do not document it" "$(field "$out" 'd["uninstall"]["status"], d["uninstall"]["note"]')" "('fail', 'no uninstall documented"

# Nothing was written outside the temp dirs: the test HOME is untouched.
leftover="$(find "${TMPDIR:-/tmp}" -maxdepth 1 -name 'treemand-install-probe.*' 2>/dev/null | head -1)"
if [ -z "$leftover" ]; then ok "the probe removes its temp dirs"; else bad "the probe removes its temp dirs ($leftover)"; fi

echo
echo "install probe tests: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
