#!/bin/sh
# Behaviour tests for the fuzz probe (scripts/metrics/fuzz_campaign.py). A
# fixture module with three Fuzz targets in two packages and a fake `go` first
# on PATH: one target grows the cache corpus, one finds a crasher, one fails
# its committed seed corpus. Then a build failure, which must fail the probe.
here="$(cd "$(dirname "$0")" && pwd)"
probe="$here/fuzz_campaign.py"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0

get() { python3 -I "$here/json_get.py" "$1" "$2"; }
expect() { # expect <json-file> <expression> <value> <what>
  got="$(get "$1" "$2")"
  if [ "$got" = "$3" ]; then echo "ok    $4"; else echo "FAIL  $4: $2 = $got, want $3"; fail=1; fi
}

# ── the fixture module ────────────────────────────────────────────────────
mod="$tmp/mod"
mkdir -p "$mod/parse/testdata/fuzz/FuzzSeedFails" "$mod/conf" "$mod/plain" "$tmp/bin" "$tmp/cache"
printf 'module example.com/m\n\ngo 1.24\n' >"$mod/go.mod"
printf 'package parse\nfunc FuzzGrows(f *testing.F) {}\nfunc FuzzCrashes(f *testing.F) {}\nfunc FuzzSeedFails(f *testing.F) {}\n' >"$mod/parse/fuzz_test.go"
printf 'package conf\nfunc FuzzConf(f *testing.F) {}\n' >"$mod/conf/fuzz_test.go"
printf 'package plain\nfunc TestNothing(t *testing.T) {}\n' >"$mod/plain/plain_test.go"
printf 'go test fuzz v1\nstring("committed")\n' >"$mod/parse/testdata/fuzz/FuzzSeedFails/aaaa"
mkdir -p "$tmp/cache/fuzz/example.com/m/conf/FuzzConf"
: >"$tmp/cache/fuzz/example.com/m/conf/FuzzConf/old1"

# The fake go records each call and acts out one outcome per target.
cat >"$tmp/bin/go" <<SH
#!/bin/sh
echo "\$*" >>"$tmp/calls.log"
[ "\$1 \$2" = "env GOCACHE" ] && { echo "$tmp/cache"; exit 0; }
[ -n "\${FAKE_BUILD_FAIL:-}" ] && { echo "./x.go:1: syntax error"; echo "FAIL	example.com/m/parse [build failed]"; exit 1; }
case "\$*" in
  *"-list"*"./parse"*) printf 'FuzzGrows\nFuzzCrashes\nFuzzSeedFails\nok  \texample.com/m/parse\t0.1s\n'; exit 0;;
  *"-list"*"./conf"*) printf 'FuzzConf\nok  \texample.com/m/conf\t0.1s\n'; exit 0;;
  *"-fuzz ^FuzzGrows\$"*)
    d="$tmp/cache/fuzz/example.com/m/parse/FuzzGrows"; mkdir -p "\$d"; : >"\$d/n1"; : >"\$d/n2"; : >"\$d/n3"
    echo "fuzz: elapsed: 2s, execs: 100 (50/sec), new interesting: 3 (total: 10)"; echo PASS; exit 0;;
  *"-fuzz ^FuzzConf\$"*)
    d="$tmp/cache/fuzz/example.com/m/conf/FuzzConf"; : >"\$d/n1"
    echo "fuzz: elapsed: 2s, execs: 100 (50/sec), new interesting: 1 (total: 5)"; echo PASS; exit 0;;
  *"-fuzz ^FuzzCrashes\$"*)
    mkdir -p testdata/fuzz/FuzzCrashes; printf 'go test fuzz v1\nstring("--\\\\x00")\n' >testdata/fuzz/FuzzCrashes/0123abcd
    cat <<'OUT'
fuzz: elapsed: 0s, gathering baseline coverage: 0/10 completed
fuzz: elapsed: 1s, execs: 40 (40/sec), new interesting: 0 (total: 10)
--- FAIL: FuzzCrashes (1.10s)
    --- FAIL: FuzzCrashes (0.00s)
        fuzz_test.go:9: flag name "--" is not an option name

    Failing input written to testdata/fuzz/FuzzCrashes/0123abcd
    To re-run:
    go test -run=FuzzCrashes/0123abcd
FAIL
exit status 1
FAIL	example.com/m/parse	1.2s
OUT
    exit 1;;
  *"-fuzz ^FuzzSeedFails\$"*)
    cat <<'OUT'
failure while testing seed corpus entry: FuzzSeedFails/aaaa
fuzz: elapsed: 0s, gathering baseline coverage: 0/3 completed
--- FAIL: FuzzSeedFails (0.00s)
    --- FAIL: FuzzSeedFails/aaaa (0.00s)
        panic: runtime error: index out of range [3] with length 3
FAIL
OUT
    exit 1;;
esac
echo "unexpected go call: \$*" >&2; exit 2
SH
chmod +x "$tmp/bin/go"

# ── a campaign ────────────────────────────────────────────────────────────
out="$tmp/out.json"
PATH="$tmp/bin:$PATH" FUZZ_SECONDS=2 python3 -I "$probe" "$mod" >"$out" 2>"$tmp/err"
rc=$?
if [ "$rc" -eq 0 ]; then echo "ok    the probe succeeds although targets crash"; else echo "FAIL  probe exit $rc: $(cat "$tmp/err")"; fail=1; fi
expect "$out" 'o["probe"]' fuzz "probe name"
expect "$out" 'o["metrics"]["crashers"]' 2 "a new crasher and a failing committed seed are both crashers"
expect "$out" 'o["metrics"]["corpus_size"]' 7 "corpus_size: cache entries plus committed seeds after the run"
expect "$out" 'o["metrics"]["new_coverage_per_minute"]' 48.0 "new entries (4) per minute fuzzed (2s+2s full runs, 1s and 0s until the crash)"
expect "$out" 'sorted(d["target"] for d in o["details"] if "target" in d)' "['FuzzConf', 'FuzzCrashes', 'FuzzGrows', 'FuzzSeedFails']" "every Fuzz target, none from a package without one"
expect "$out" '[d for d in o["details"] if d.get("target")=="FuzzGrows"][0]["new_entries"]' 3 "new entries per target"
expect "$out" '[d for d in o["details"] if d.get("target")=="FuzzConf"][0]["corpus_size"]' 2 "corpus size per target"
expect "$out" '[d for d in o["details"] if d.get("target")=="FuzzCrashes"][0]["crasher"]["rerun"]' "go test -run=FuzzCrashes/0123abcd ./parse" "the crasher's re-run command"
expect "$out" '[d for d in o["details"] if d.get("target")=="FuzzCrashes"][0]["crasher"]["input_file"]' "parse/testdata/fuzz/FuzzCrashes/0123abcd" "the crasher's input file"
expect "$out" '[d for d in o["details"] if d.get("target")=="FuzzCrashes"][0]["crasher"]["input"].splitlines()[0]' "go test fuzz v1" "the crasher's input"
expect "$out" '"is not an option name" in [d for d in o["details"] if d.get("target")=="FuzzCrashes"][0]["crasher"]["failure"]' True "the crasher's failure message"
expect "$out" '[d for d in o["details"] if d.get("target")=="FuzzSeedFails"][0]["crasher"]["rerun"]' "go test -run=FuzzSeedFails/aaaa ./parse" "a failing seed's re-run command"
expect "$out" '[d for d in o["details"] if d.get("target")=="FuzzSeedFails"][0]["crasher"]["committed"]' True "a failing seed is flagged as committed"
if grep -q -- '-fuzz ^FuzzGrows$ -fuzztime 2s' "$tmp/calls.log"; then echo "ok    FUZZ_SECONDS sets each target's fuzztime"; else echo "FAIL  fuzztime not passed: $(cat "$tmp/calls.log")"; fail=1; fi

# ── FUZZ_KEEP_CRASHERS=0 takes a new crasher out of the tree ──────────────
rm -rf "$mod/parse/testdata/fuzz/FuzzCrashes"
PATH="$tmp/bin:$PATH" FUZZ_KEEP_CRASHERS=0 FUZZ_SECONDS=2 python3 -I "$probe" "$mod" >"$tmp/nk.json" 2>"$tmp/nk.err"
expect "$tmp/nk.json" '[d for d in o["details"] if d.get("target")=="FuzzCrashes"][0]["crasher"]["input"].splitlines()[0]' "go test fuzz v1" "a removed crasher's input stays in details"
if [ ! -e "$mod/parse/testdata/fuzz/FuzzCrashes/0123abcd" ] && [ -e "$mod/parse/testdata/fuzz/FuzzSeedFails/aaaa" ]; then
  echo "ok    the new crasher is removed, the committed seed is kept"
else echo "FAIL  FUZZ_KEEP_CRASHERS=0 left the tree wrong: $(find "$mod/parse/testdata" -type f)"; fail=1; fi

# ── a build failure is a probe error, not a crasher ───────────────────────
PATH="$tmp/bin:$PATH" FAKE_BUILD_FAIL=1 FUZZ_SECONDS=2 python3 -I "$probe" "$mod" >"$tmp/bf.json" 2>"$tmp/bf.err"
rc=$?
if [ "$rc" -ne 0 ] && grep -q 'syntax error' "$tmp/bf.err"; then echo "ok    a build failure fails the probe and shows go's output"; else echo "FAIL  build failure: exit $rc, stderr $(cat "$tmp/bf.err")"; fail=1; fi

exit "$fail"
