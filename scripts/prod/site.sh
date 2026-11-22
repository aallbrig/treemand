#!/bin/sh
# site.sh [--once]: production site health for the prod window. HTTP status and
# response time for treemand.dev and its key pages, days until the TLS
# certificate expires, and the release the site serves (/releases/latest.txt)
# against the latest GitHub release. See scripts/prod/lib.sh for the modes.
# shellcheck source=scripts/prod/lib.sh
. "$(dirname "$0")/lib.sh"

base="${PROD_SITE_URL:-https://treemand.dev}"
host="${base#*://}"
host="${host%%/*}"

days_until() { # days_until "<openssl notAfter date>": whole days from today (UTC)
  printf '%s\n' "$1" | awk -v now="$(date -u '+%Y %m %d')" '
    function jdn(y, m, d,   a) { a = int((14 - m) / 12); y += 4800 - a; m += 12 * a - 3
      return d + int((153 * m + 2) / 5) + 365 * y + int(y / 4) - int(y / 100) + int(y / 400) - 32045 }
    { mon = index("JanFebMarAprMayJunJulAugSepOctNovDec", $1)
      if (mon == 0 || $4 !~ /^[0-9]+$/) exit 1
      split(now, n, " "); print jdn($4, (mon + 2) / 3, $2) - jdn(n[1] + 0, n[2] + 0, n[3] + 0) }'
}

pages() {
  failed=0
  for p in "" installation/ try/ install.sh llms.txt; do
    if r="$(curl -sS -o /dev/null --max-time "$PROD_TIMEOUT" -w '%{http_code} %{time_total}' "$base/$p" 2>"$PROD_TMP")"; then
      code="${r%% *}"
      ms="$(awk -v t="${r#* }" 'BEGIN { printf "%d", t * 1000 }')"
      mark=ok
      [ "$code" = 200 ] || { mark=FAIL; failed=1; }
      printf '%-4s %s %5sms  /%s\n' "$mark" "$code" "$ms" "$p"
    else
      printf 'error: /%s: %s\n' "$p" "$(prod_err "$PROD_TMP")"
      failed=1
    fi
  done
  return "$failed"
}

tls() {
  end="$(openssl s_client -connect "$host:443" -servername "$host" </dev/null 2>"$PROD_TMP" |
    openssl x509 -noout -enddate 2>>"$PROD_TMP")"
  if ! days="$(days_until "${end#notAfter=}")" || [ -z "$days" ]; then
    printf 'error: TLS certificate: %s\n' "$(prod_err "$PROD_TMP")"
    return 1
  fi
  if [ "$days" -le 0 ]; then
    echo "FAIL TLS certificate expired"; return 1
  elif [ "$days" -lt 14 ]; then
    echo "WARN TLS certificate expires in $days days"
  else
    echo "ok   TLS certificate expires in $days days"
  fi
}

version() {
  if ! site="$(curl -fsS --max-time "$PROD_TIMEOUT" "$base/releases/latest.txt" 2>"$PROD_TMP")"; then
    printf 'error: site version: %s\n' "$(prod_err "$PROD_TMP")"; return 1
  fi
  if ! rel="$(gh release view --repo "$PROD_REPO" --json tagName --jq .tagName 2>"$PROD_TMP")"; then
    printf 'error: latest release: %s\n' "$(prod_err "$PROD_TMP")"; return 1
  fi
  mark=ok
  [ "$site" = "$rel" ] || mark=WARN
  printf '%-4s version: site %s, latest release %s\n' "$mark" "$site" "$rel"
}

report() {
  bad=0
  pages || bad=1
  tls || bad=1
  version || bad=1
  return "$bad"
}

prod_main "site $host" report "$@"
