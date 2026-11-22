#!/bin/sh
# Behaviour test for the cli-ux probe: a fake treemand that follows every rule
# must pass every check, and one that breaks every rule must fail every check.
# CLI_UX_BIN points the probe at the fake instead of building treemand.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fails=0
check() { # check <name> <want> <got>
  if [ "$2" = "$3" ]; then echo "  ok   $1"; else
    echo "  FAIL $1: want '$2', got '$3'"
    fails=$((fails + 1))
  fi
}

# A well-behaved treemand.
cat >"$work/good" <<'FAKE'
#!/bin/sh
usage() { printf 'Usage:\n  treemand %s [flags]\n\nExamples:\n  treemand git\n' "$1"; }
case "$*" in
  --help | -h)
    usage '<cli>'
    printf '\nAvailable Commands:\n  cache       Manage the cache\n  completion  Generate completion\n  config      Manage configuration\n\nFlags:\n  --output\n' ;;
  "config --help") usage config; printf '\nAvailable Commands:\n  view        Show the configuration\n' ;;
  "cache --help") usage cache; printf '\nAvailable Commands:\n  list        List cached CLIs\n' ;;
  *" --help") usage "$1" ;;
  --version | version) echo "treemand v1.2.3" ;;
  "cache lst") printf 'Error: unknown command "lst" for "treemand cache"\nDid you mean this? list\n' >&2; exit 2 ;;
  confg) printf 'Error: unknown command "confg"\nDid you mean "treemand config"?\n' >&2; exit 2 ;;
  --outptu=json*) printf 'Error: unknown flag: --outptu\nDid you mean --output?\n' >&2; exit 2 ;;
  "" | "config set") printf 'Error: missing argument\nRun treemand --help\n' >&2; exit 2 ;;
  completion\ *) printf '#compdef treemand\ncomplete -o default treemand\n' ;;
  *treemand-no-such-cli) printf 'Error: command "treemand-no-such-cli" not found\nHint: check spelling and PATH\n' >&2; exit 1 ;;
  *--output=json*) echo '{"name": "treemand"}' ;;
  *--no-color*) echo "treemand" ;;
  *)
    if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then printf '\033[1mtreemand\033[0m\n'; else echo treemand; fi ;;
esac
FAKE

# A treemand that breaks every rule: color and progress everywhere, every
# answer on stderr with exit 0, and an undescribed subcommand.
cat >"$work/bad" <<'FAKE'
#!/bin/sh
printf '\033[31m⠋ working\r\033[0m\nAvailable Commands:\n  cache\n\n'
echo oops >&2
FAKE
chmod +x "$work/good" "$work/bad"

metric() { printf '%s' "$1" | python3 -I -c "import json,sys; print(json.load(sys.stdin)$2)"; }
total=19
tty_skipped=0
command -v script >/dev/null 2>&1 || tty_skipped=2

good="$(CLI_UX_BIN="$work/good" sh "$here/probes/cli-ux.sh")"
check "good: no failing checks" 0 "$(metric "$good" '["metrics"]["failing_checks"]')"
check "good: every check measured" "$((total - tty_skipped))" "$(metric "$good" '["metrics"]["passing_checks"]')"
check "good: every subcommand described" 0 "$(metric "$good" '["metrics"]["undescribed_subcommands"]')"
check "good: subcommands walked two levels" 5 "$(metric "$good" '["details"][1]["subcommands"].__len__()')"
if [ "$tty_skipped" = 0 ]; then
  check "good: NO_COLOR honoured on a TTY" 1 "$(metric "$good" '["metrics"]["check.tty_no_color_env"]')"
fi

bad="$(CLI_UX_BIN="$work/bad" sh "$here/probes/cli-ux.sh")"
check "bad: every check fails" "$((total - tty_skipped))" "$(metric "$bad" '["metrics"]["failing_checks"]')"
check "bad: undescribed subcommands counted" 2 "$(metric "$bad" '["metrics"]["undescribed_subcommands"]')"
check "bad: a failing check carries its evidence" "True" \
  "$(metric "$bad" '["details"][0]["failing"][0]["evidence"].__contains__("exit=")')"

[ "$fails" = 0 ] || { echo "$fails failed"; exit 1; }
echo "cli-ux: all passed"
