#!/bin/sh
# shellcheck disable=SC2016,SC2329
# Behaviour tests for the prod window panes (scripts/prod/*.sh). Each pane runs
# --once with fake curl, gh, aws and openssl first on PATH, in three worlds:
# everything up, no AWS credentials, and the network down.
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0
check() { if ! eval "$2"; then echo "prod_test FAIL: $1" >&2; sed 's/^/    | /' "$tmp/out" >&2; fail=1; fi; }

bin="$tmp/bin"
mkdir -p "$bin"
stub() { printf '#!/bin/sh\n%s\n' "$2" >"$bin/$1"; chmod +x "$bin/$1"; }
down='[ "$FAKE_MODE" = netdown ]'

stub curl "$down"' && { echo "curl: (6) Could not resolve host: treemand.dev" >&2; exit 6; }
case "$*" in
  *-w*) printf "200 0.123" ;;
  *latest.txt*) echo "${FAKE_SITE_VERSION:-v0.8.3}" ;;
esac'

stub openssl "$down"' && { echo "connect: Network is unreachable" >&2; exit 1; }
case "$1" in
  s_client) echo "-----BEGIN CERTIFICATE-----" ;;
  x509) read -r line || { echo "Could not read certificate" >&2; exit 1; }
        echo "notAfter=${FAKE_NOT_AFTER:-Jan  1 00:00:00 2099 GMT}" ;;
esac'

stub gh "$down"' && { echo "error connecting to api.github.com" >&2; exit 1; }
case "$1 $2" in
  "run list") echo "success main 2026-11-22T05:25:10Z" ;;
  "pr list") echo "#101 Add the prod window" ;;
  "issue list") echo "#253 prod window panes" ;;
  "release view") echo "${FAKE_RELEASE:-v0.8.3}" ;;
  "api "*) printf "v0.8.3 18 2026-11-22\nv0.8.2 19 2026-11-16\n" ;;
esac'

stub aws "$down"' && { echo "Could not connect to the endpoint URL: \"https://sts.amazonaws.com/\"" >&2; exit 255; }
case "$1 $2" in
  "sts get-caller-identity")
    [ "$FAKE_MODE" = nocreds ] && { echo "Unable to locate credentials. You can configure credentials by running \"aws configure\"." >&2; exit 253; }
    echo 123456789012 ;;
  "cloudformation describe-stacks") echo E1EXAMPLE ;;
  "cloudformation describe-stack-resource") echo treemand-monthly ;;
  "cloudfront get-distribution")
    [ "$FAKE_MODE" = leak ] && { echo "AccessDenied: arn:aws:iam::$FAKE_ACCT:user/someone is not authorized" >&2; exit 254; }
    printf "Deployed\tTrue\n" ;;
  "budgets describe-budget")
    case "$*" in *123456789012*) ;; *) echo "missing account id" >&2; exit 252 ;; esac
    printf "0.034\t1.2\t5.0\tUSD\n" ;;
esac'

sh_bin="$(command -v dash || command -v sh)"
run() { # run <script> [VAR=value ...]
  s="$1"; shift
  env -i HOME="$tmp" PATH="$bin:/usr/bin:/bin" "$@" "$sh_bin" "$here/$s" --once >"$tmp/out" 2>&1
  echo $? >"$tmp/rc"
}
rc() { cat "$tmp/rc"; }

# ── everything up ───────────────────────────────────────────────────────────
run site.sh FAKE_MODE=ok
check "site ok exits 0" '[ "$(rc)" = 0 ]'
check "site shows home page" 'grep -Eq "^ok +200 +123ms +/$" "$tmp/out"'
for p in installation/ try/ install.sh llms.txt; do
  check "site shows $p" 'grep -Eq "^ok +200 +[0-9]+ms +/$p$" "$tmp/out"'
done
check "site shows TLS days" 'grep -Eq "^ok +TLS certificate expires in [0-9]+ days" "$tmp/out"'
check "site shows matching version" 'grep -Eq "^ok +version: site v0.8.3, latest release v0.8.3" "$tmp/out"'

run site.sh FAKE_MODE=ok FAKE_RELEASE=v0.8.4
check "version mismatch warns" 'grep -Eq "^WARN +version: site v0.8.3, latest release v0.8.4" "$tmp/out"'

run site.sh FAKE_MODE=ok "FAKE_NOT_AFTER=Jan  1 00:00:00 2001 GMT"
check "expired certificate fails" '[ "$(rc)" = 1 ] && grep -Eq "^FAIL +TLS certificate expired" "$tmp/out"'

run pipelines.sh FAKE_MODE=ok
check "pipelines ok exits 0" '[ "$(rc)" = 0 ]'
for w in CI "Auto Release" Release Site Packages; do
  check "pipelines shows $w" 'grep -Eq "^$w +success main" "$tmp/out"'
done
check "pipelines shows open PRs" 'grep -q "#101 Add the prod window" "$tmp/out"'
check "pipelines shows in-progress issues" 'grep -q "#253 prod window panes" "$tmp/out"'

run aws.sh FAKE_MODE=ok
check "aws ok exits 0" '[ "$(rc)" = 0 ]'
check "aws shows CloudFront" 'grep -Eq "^ok +CloudFront Deployed, enabled" "$tmp/out"'
check "aws shows budget" 'grep -Eq "^ok +budget treemand-monthly: 0.03 of 5.00 USD" "$tmp/out"'
check "aws never prints the account id" '! grep -q 123456789012 "$tmp/out"'

run aws.sh FAKE_MODE=leak FAKE_ACCT=123456789012
check "aws redacts account ids in errors" '! grep -q 123456789012 "$tmp/out" && grep -q "error: CloudFront" "$tmp/out"'

run downloads.sh FAKE_MODE=ok
check "downloads ok exits 0" '[ "$(rc)" = 0 ]'
check "downloads per release" 'grep -Eq "^v0.8.3 +2026-11-22 +18$" "$tmp/out" && grep -Eq "^v0.8.2 +2026-11-16 +19$" "$tmp/out"'
check "downloads total" 'grep -Eq "^total +37$" "$tmp/out"'

# ── no AWS credentials ──────────────────────────────────────────────────────
run aws.sh FAKE_MODE=nocreds
check "no creds exits 0" '[ "$(rc)" = 0 ]'
check "no creds shows the hint" 'grep -q "set AWS_PROFILE to a read-only profile" "$tmp/out" && grep -q "#254" "$tmp/out"'
check "no creds skips the AWS calls" '! grep -q CloudFront "$tmp/out"'

# ── network down: an error line, no crash ───────────────────────────────────
for s in site.sh pipelines.sh aws.sh downloads.sh; do
  run "$s" FAKE_MODE=netdown
  check "$s netdown exits 0 or 1" '[ "$(rc)" -le 1 ]'
  check "$s netdown shows an error or hint" 'grep -Eq "^(error|hint):" "$tmp/out"'
done

# ── loop mode keeps going after a failure ──────────────────────────────────
env -i HOME="$tmp" PATH="$bin:/usr/bin:/bin" FAKE_MODE=nocreds PROD_REFRESH_SECONDS=1 \
  "$sh_bin" "$here/aws.sh" >"$tmp/out" 2>&1 &
pid=$!
sleep 3
check "loop mode still running" 'kill -0 "$pid" 2>/dev/null'
kill "$pid" 2>/dev/null
wait "$pid" 2>/dev/null
check "loop mode refreshed" '[ "$(grep -c "set AWS_PROFILE" "$tmp/out")" -ge 2 ]'

# ── usage ────────────────────────────────────────────────────────────────────
"$sh_bin" "$here/site.sh" --bogus >"$tmp/out" 2>&1; echo $? >"$tmp/rc"
check "unknown flag exits 2" '[ "$(rc)" = 2 ]'

[ "$fail" = 0 ] && echo "prod_test: ok"
exit "$fail"
