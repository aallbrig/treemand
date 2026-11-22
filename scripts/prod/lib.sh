# shellcheck shell=sh
# Shared runner for the prod window panes. A pane script defines one function
# that prints its report (lines start with ok, WARN, FAIL, error: or hint:) and
# returns 1 if a check failed, then calls: prod_main <title> <function> "$@".
#   --once   print the report once and exit with the function's status
#   (none)   redraw every PROD_REFRESH_SECONDS (default 300); never exits on failure
# All output passes through prod_redact, so a 12-digit number (an AWS account
# ID) never reaches the terminal, whatever a tool printed.

PROD_REPO="${PROD_REPO:-aallbrig/treemand}"
PROD_TIMEOUT="${PROD_TIMEOUT:-10}"

prod_redact() { sed -E 's/[0-9]{12,}/<redacted>/g'; }

# prod_err <file>: the first line of a captured error, trimmed for a pane.
prod_err() { sed -n '/./{p;q;}' "$1" 2>/dev/null | cut -c1-120; }

prod_report() { # prod_report <title> <function>: header + report, redacted; returns the function's status
  out="$("$2" 2>&1)"
  rc=$?
  { printf '== %s  %s\n' "$1" "$(date -u '+%Y-%m-%d %H:%M:%SZ')"; printf '%s\n' "$out"; } | prod_redact
  return "$rc"
}

prod_main() { # prod_main <title> <function> [--once]
  title="$1" fn="$2"
  shift 2
  PROD_TMP="$(mktemp)" || exit 1
  trap 'rm -f "$PROD_TMP"' EXIT
  trap 'exit 0' INT TERM
  case "${1:-}" in
    --once) prod_report "$title" "$fn"; exit $? ;;
    '') ;;
    *) echo "usage: $0 [--once]" >&2; exit 2 ;;
  esac
  every="${PROD_REFRESH_SECONDS:-300}"
  case "$every" in '' | *[!0-9]* | 0) every=300 ;; esac
  while :; do
    report="$(prod_report "$title" "$fn")"
    [ -t 1 ] && printf '\033[H\033[2J'
    printf '%s\n(refreshes every %ss)\n' "$report" "$every"
    sleep "$every"
  done
}
