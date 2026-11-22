#!/bin/sh
# shellcheck disable=SC2016,SC2034
# Behaviour tests for scripts/tui/ux_evidence.sh. A fake TUI stands in for
# treemand: it echoes each line it reads, "panics" on the line "panic", and
# fails like treemand does for a CLI on no PATH. Needs tmux; skips without it.
here="$(cd "$(dirname "$0")" && pwd)"
command -v tmux >/dev/null 2>&1 || { echo "ux_evidence_test: SKIP (no tmux)"; exit 0; }
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0
check() { if ! eval "$2"; then echo "ux_evidence_test FAIL: $1" >&2; fail=1; fi; }

cat >"$tmp/fake-tui" <<'EOF'
#!/bin/sh
if [ "$2" = no-such-cli-for-ux ]; then echo "Error: command \"$2\" not found in PATH" >&2; exit 1; fi
echo "fake tui for $2"
while IFS= read -r line; do
  [ "$line" = panic ] && { echo "panic: boom"; echo "goroutine 1 [running]:"; exit 2; }
  echo "got: $line"
done
EOF
chmod +x "$tmp/fake-tui"

cat >"$tmp/flows.txt" <<'EOF'
# comment lines and blank lines are skipped

walk|git|01_opened|
walk||02_typed|lit:hello Enter
crash|git|01_opened|
crash||02_boom|lit:panic Enter
crash||03_after|lit:never Enter
error_missing|@missing|01_opened|
EOF

run() { UX_BIN="$tmp/fake-tui" UX_FLOW_FILE="$tmp/flows.txt" dash "$here/ux_evidence.sh" --out "$tmp/out" --sizes 60x12 --no-png "$@" >"$tmp/stdout" 2>"$tmp/stderr"; }

# A walk: every step leaves an ANSI and a text frame and an index row.
run --flows walk
code=$?
check "a clean walk exits 0" '[ "$code" -eq 0 ]'
check "the opened frame shows the TUI" 'grep -q "fake tui for git" "$tmp/out/60x12/walk/01_opened.txt"'
check "keys reach the TUI before the frame is taken" 'grep -q "got: hello" "$tmp/out/60x12/walk/02_typed.txt"'
check "an ANSI frame sits beside each text frame" '[ -f "$tmp/out/60x12/walk/02_typed.ans" ]'
check "the index has a header and one row per step" '[ "$(wc -l <"$tmp/out/index.tsv")" -eq 3 ]'
check "index rows carry size, flow, step, keys, frame and verdict" \
  'grep -q "^60x12	walk	02_typed	lit:hello Enter	60x12/walk/02_typed	ok$" "$tmp/out/index.tsv"'
check "only the selected flows run" '[ ! -d "$tmp/out/60x12/crash" ]'

# A crash fails the run, is named in checks.txt, and ends its flow.
rm -rf "$tmp/out"
run --flows crash
code=$?
check "a crash exits 1" '[ "$code" -eq 1 ]'
check "checks.txt names the crashed frame" 'grep -q "60x12/crash/02_boom: panic" "$tmp/out/checks.txt"'
check "the flow stops after a crash" '[ ! -f "$tmp/out/60x12/crash/03_after.txt" ]'

# A CLI on no PATH is an error state to look at, not a failed check.
rm -rf "$tmp/out"
run --flows error_missing
code=$?
check "a missing CLI is captured and passes" '[ "$code" -eq 0 ] && grep -q "not found in PATH" "$tmp/out/60x12/error_missing/01_opened.txt"'

[ "$fail" -eq 0 ] && echo "ux_evidence_test: ok"
exit "$fail"
