#!/bin/sh
# Test treemand.dev's security headers policy (infra/site.yaml) against the
# site as production builds it (hugo --minify): the policy has a
# Content-Security-Policy and a Permissions-Policy, and every inline script
# is allowed by a hash in script-src. Also tests the header reader on a
# fixture template. Needs hugo and python3; run via `task site:csp:test`.
set -eu
cd "$(dirname "$0")/../.."
command -v hugo >/dev/null || { echo "csp_test: hugo is required" >&2; exit 2; }

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail=0
reader=scripts/site/serve_with_headers.py

# The reader: folded and quoted scalars, CSP first, then custom headers.
cat >"$work/site.yaml" <<'YAML'
Resources:
  SecurityHeaders:
    Type: AWS::CloudFront::ResponseHeadersPolicy
    Properties:
      ResponseHeadersPolicyConfig:
        SecurityHeadersConfig:
          ContentSecurityPolicy:
            ContentSecurityPolicy: >-
              default-src 'self';
              frame-ancestors 'none'
            Override: true
        CustomHeadersConfig:
          Items:
            - Header: Permissions-Policy
              Value: "camera=()"
              Override: true
  Distribution:
    Type: AWS::CloudFront::Distribution
YAML
got="$(python3 -I "$reader" --print x "$work/site.yaml")"
want="Content-Security-Policy: default-src 'self'; frame-ancestors 'none'
Permissions-Policy: camera=()"
if [ "$got" != "$want" ]; then
  printf 'FAIL reader: got\n%s\nwant\n%s\n' "$got" "$want"; fail=1
fi

# An inline script the fixture policy has no hash for is reported.
mkdir -p "$work/fixture"
printf '<html><head><script>alert(1)</script><script type="application/ld+json">{}</script></head></html>\n' >"$work/fixture/index.html"
if out="$(python3 -I "$reader" --check "$work/fixture" "$work/site.yaml")"; then
  echo "FAIL check: an unhashed inline script passed"; fail=1
else
  case "$out" in
    *"index.html: inline script 'sha256-"*) ;;
    *) printf 'FAIL check: unexpected report\n%s\n' "$out"; fail=1 ;;
  esac
  [ "$(printf '%s\n' "$out" | wc -l)" -eq 1 ] || { echo "FAIL check: a JSON-LD data block was reported"; fail=1; }
fi

# The real policy has both headers.
headers="$(python3 -I "$reader" --print x)"
for h in Content-Security-Policy Permissions-Policy; do
  printf '%s\n' "$headers" | grep -q "^$h: " || { echo "FAIL infra/site.yaml sends no $h"; fail=1; }
done
printf '%s\n' "$headers" | grep "^Content-Security-Policy:" | grep -q "'unsafe-inline';\{0,1\} *script-src\|script-src[^;]*'unsafe-inline'" \
  && { echo "FAIL script-src allows 'unsafe-inline'"; fail=1; }

# The production build's inline scripts are all hashed in the real policy.
(cd www/treemand && hugo --quiet --minify -b / -d "$work/site")
python3 -I "$reader" --check "$work/site" || fail=1

[ "$fail" -eq 0 ] && echo "PASS: site security headers"
exit "$fail"
