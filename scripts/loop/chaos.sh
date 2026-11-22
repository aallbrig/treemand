#!/usr/bin/env bash
# Reliability suite: hostile fake CLIs. For each, treemand must finish within its
# budget, never panic, and stay under a memory cap. Hermetic: needs only bash.
# usage: chaos.sh [path-to-treemand]   exit 1 if any case fails
set -uo pipefail
# shellcheck source=scripts/loop/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
cd "$LOOP_ROOT" || exit 1
bin="${1:-$LOOP_DIR/treemand-chaos}"
if [ $# -eq 0 ]; then go build -o "$bin" ./cli/treemand || exit 1; fi

work="$(mktemp -d)"
trap 'rm -rf "$work"; pkill -f "$work" 2>/dev/null || true' EXIT
mk() { printf '#!/usr/bin/env bash\n%s\n' "$2" >"$work/$1"; chmod +x "$work/$1"; }

HELP='Usage: NAME [command]

Commands:
  sub    A subcommand
  other  Another subcommand

Flags:
  -h, --help   help for NAME'

mk chaos-hang       'sleep 1000'
mk chaos-huge       'yes "Commands: lots of text that goes on and on and on" | head -c 300000000'
mk chaos-garbage    'head -c 5000000 /dev/urandom'
mk chaos-orphan     'echo "Usage: chaos-orphan [command]"; echo; echo "Commands:"; echo "  sub  x"; (sleep 120 &) ; exit 0'
mk chaos-recursive  "printf '%s\n' 'Usage: chaos-recursive [command]' '' 'Commands:' '  sub  Goes deeper' '  down  Also deeper'"
mk chaos-trickle    'while :; do printf "x"; sleep 0.5; done'
mk chaos-exit1      "cat <<'H'
$HELP
H
exit 1"
mk chaos-utf8       "printf 'Usage: chaos-utf8\n\nCommands:\n  caf\xc3\xa9  ok\n  bad\xff\xfe  invalid utf8\n  \033[31mred\033[0m  ansi\n'"
# shellcheck disable=SC2016
mk chaos-wide       'echo "Usage: chaos-wide [command]"; echo; echo "Commands:"; for i in $(seq 1 6000); do echo "  cmd$i  Command number $i"; done'
mk chaos-nohelp     'echo "unknown flag" >&2; exit 2'
mk chaos-empty      'exit 0'
# BSD mkdir: flags are rejected with a usage line, every operand is created.
# Discovery must never send it a positional "help" (#273).
mk chaos-bsdoperand "for a in \"\$@\"; do case \"\$a\" in -*) echo \"chaos-bsdoperand: illegal option -- \${a#-}\" >&2; echo 'usage: chaos-bsdoperand [-pv] directory_name ...' >&2; exit 64;; esac; done
touch \"$work/chaos-bsdoperand.acted\""

fail=0
printf 'case\tstatus\tsecs\trss_mb\n'
for c in "$work"/chaos-*; do
  name="$(basename "$c")"
  budget=25
  err="$work/$name.err"
  # timed.py: wall seconds and max RSS (KB) on Linux and macOS alike.
  PATH="$work:$PATH" python3 -I "$LOOP_ROOT/scripts/metrics/timed.py" "$work/$name.cost" "$PWD" -- \
    timeout --kill-after=2 "$budget" "$bin" --no-cache --output=json --timeout=5 "$name" >"$work/$name.out" 2>"$err"
  rc=$?
  secs="$(awk '{printf "%.1f", $1}' "$work/$name.cost" 2>/dev/null || echo 0)"
  rss="$(awk '{printf "%.0f", $4/1024}' "$work/$name.cost" 2>/dev/null || echo 0)"
  status=ok
  [ "$rc" -eq 124 ] || [ "$rc" -eq 137 ] && status="HUNG(rc=$rc)"
  grep -qE 'panic:|goroutine [0-9]+ \[|fatal error' "$err" "$work/$name.out" 2>/dev/null && status="PANIC"
  [ "${rss:-0}" -gt 1024 ] && status="MEMORY(${rss}MB)"
  [ -e "$work/$name.acted" ] && status="SIDE-EFFECT"
  if grep -qx "$name" "$LOOP_ROOT/scripts/loop/chaos.xfail" 2>/dev/null; then
    if [ "$status" = ok ]; then status="XPASS (fixed: remove $name from scripts/loop/chaos.xfail)"; fail=1
    else status="xfail:$status"; fi
  elif [ "$status" != ok ]; then fail=1; fi
  printf '%s\t%s\t%s\t%s\n' "$name" "$status" "$secs" "$rss"
done
exit "$fail"
