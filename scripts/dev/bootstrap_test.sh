#!/bin/sh
# shellcheck disable=SC2016
# Behaviour tests for scripts/dev/bootstrap.sh. Runs it with a PATH that holds
# only stubs, so the readiness report is deterministic and no browser opens.
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0
check() { if ! eval "$2"; then echo "bootstrap_test FAIL: $1" >&2; fail=1; fi; }

bin="$tmp/bin"
mkdir -p "$bin"
for t in sleep uname cat; do ln -s "$(command -v "$t")" "$bin/$t"; done
stub() { printf '#!/bin/sh\n%s\n' "$2" >"$bin/$1"; chmod +x "$bin/$1"; }
stub go 'exit 0'
stub open "echo \"open \$*\" >>\"$tmp/opened\""
stub xdg-open "echo \"xdg-open \$*\" >>\"$tmp/opened\""
stub curl 'exit 0'

sh_bin="$(command -v dash || command -v sh)"
run() { env -i HOME="$tmp" PATH="$bin" TREEMAND_BOOTSTRAP_WAIT=2 "$@" "$sh_bin" "$here/bootstrap.sh" >"$tmp/out" 2>&1; echo $? >"$tmp/rc"; }

# Site up: report, then both URLs opened.
run
check "exits 0" '[ "$(cat "$tmp/rc")" = 0 ]'
check "present tool reported ok" 'grep -Eq "^ +ok +go( |$)" "$tmp/out"'
check "absent tool reported missing" 'grep -Eq "^ +missing +hugo( |$)" "$tmp/out"'
check "brew hint" 'grep -q "brew install hugo" "$tmp/out"'
check "apt hint" 'grep -q "apt install hugo" "$tmp/out"'
check "summary counts missing" 'grep -Eq "[0-9]+ of [0-9]+ tools missing" "$tmp/out"'
check "opens local site" 'grep -q "http://localhost:1313/" "$tmp/opened"'
check "opens production site" 'grep -q "https://treemand.dev" "$tmp/opened"'

# Site never comes up: still exits 0, opens only production.
rm -f "$tmp/opened"
stub curl 'exit 7'
run
check "site down exits 0" '[ "$(cat "$tmp/rc")" = 0 ]'
check "site down skips local" '! grep -qs localhost "$tmp/opened"'
check "site down still opens production" 'grep -q "https://treemand.dev" "$tmp/opened"'
stub curl 'exit 0'

# Opt out.
rm -f "$tmp/opened"
run TREEMAND_NO_BROWSER=1
check "no-browser exits 0" '[ "$(cat "$tmp/rc")" = 0 ]'
check "no-browser opens nothing" '[ ! -e "$tmp/opened" ]'

# No opener on PATH: quiet skip.
rm -f "$bin/open" "$bin/xdg-open"
run
check "no opener exits 0" '[ "$(cat "$tmp/rc")" = 0 ]'
check "no opener quiet" '! grep -qi "not found" "$tmp/out"'

[ "$fail" = 0 ] && echo "bootstrap_test: ok"
exit "$fail"
