#!/bin/sh
# shellcheck disable=SC2016
# Behaviour tests for the infra-security probe (scripts/metrics/probes/
# infra_security.sh). Static part: fixture workflows and templates with one
# known problem of each kind, and a clean set that must count zero. Live
# part: fake curl, openssl and aws first on PATH, for a healthy site and a
# weak one. Fixtures are written at runtime so nothing flaggable is committed.
here="$(cd "$(dirname "$0")" && pwd)"
probe="$here/infra_security.py"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0

# metric <json-file> <name> -> prints the metric's value (or "absent")
metric() { python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["metrics"].get(sys.argv[2], "absent"))' "$1" "$2"; }
expect() { # expect <json-file> <metric> <value> <what>
  got="$(metric "$1" "$2")"
  if [ "$got" = "$3" ]; then echo "ok    $4"; else echo "FAIL  $4: $2 = $got, want $3"; fail=1; fi
}

# ── static: a world with one problem of each kind ─────────────────────────
bad="$tmp/bad"
mkdir -p "$bad/.github/workflows" "$bad/infra"
sha="$(printf '%040d' 0 | tr 0 a)"
cat >"$bad/.github/workflows/nopermissions.yml" <<YML
name: no permissions
on: push
jobs:
  one:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: example/pinned@$sha # v1
      - uses: ./.github/actions/local
      - uses: docker://alpine:3
  two:
    uses: ./.github/workflows/other.yml
YML
cat >"$bad/.github/workflows/broad.yml" <<'YML'
name: broad
on: push
permissions:
  contents: write
  id-token: write
jobs:
  needs-write:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - run: true
  inherits:
    runs-on: ubuntu-latest
    steps:
      - name: build
        uses: "actions/setup-go@v5"
YML
cat >"$bad/.github/workflows/single.yml" <<'YML'
name: single job, workflow-level write is the job's own
on: push
permissions: {contents: write}
jobs:
  only:
    runs-on: ubuntu-latest
    steps:
      - run: true
YML
cat >"$bad/infra/weak.yaml" <<'YML'
AWSTemplateFormatVersion: "2010-09-09"
Resources:
  Bucket:
    Type: AWS::S3::Bucket
    Properties:
      Tags: [{Key: Project, Value: treemand}]
  Headers:
    Type: AWS::CloudFront::ResponseHeadersPolicy
    Properties:
      ResponseHeadersPolicyConfig:
        Name: partial
        SecurityHeadersConfig:
          ContentTypeOptions: {Override: true}
  Distribution:
    Type: AWS::CloudFront::Distribution
    Properties:
      DistributionConfig:
        Enabled: true
        ViewerCertificate:
          AcmCertificateArn: !ImportValue certificate-arn
          SslSupportMethod: sni-only
          MinimumProtocolVersion: TLSv1.2_2019
        Origins:
          - Id: o
            DomainName: !GetAtt Bucket.RegionalDomainName
            S3OriginConfig: {OriginAccessIdentity: ""}
        DefaultCacheBehavior:
          TargetOriginId: o
          ViewerProtocolPolicy: redirect-to-https
          CachePolicyId: 658327ea-f89d-4fab-a63d-7e88639e58f6
          ResponseHeadersPolicyId: !Ref Headers
        CacheBehaviors:
          - PathPattern: /raw/*
            TargetOriginId: o
            ViewerProtocolPolicy: redirect-to-https
            CachePolicyId: 658327ea-f89d-4fab-a63d-7e88639e58f6
      Tags: [{Key: Project, Value: treemand}]
  Role:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: "2012-10-17"
        Statement: [{Effect: Allow, Principal: {Service: lambda.amazonaws.com}, Action: sts:AssumeRole}]
      Policies:
        - PolicyName: wide
          PolicyDocument:
            Version: "2012-10-17"
            Statement:
              - {Effect: Allow, Action: ["s3:*", "s3:Get*"], Resource: "*"}
              - {Effect: Deny, Action: "iam:*", Resource: "*"}
      Tags: [{Key: Project, Value: treemand}]
YML
python3 -I "$probe" --root "$bad" >"$tmp/bad.json" 2>"$tmp/bad.err" || { echo "FAIL  probe exited non-zero on the weak world"; cat "$tmp/bad.err"; fail=1; }
expect "$tmp/bad.json" actions_unpinned 3                  "counts tag- and docker-tag refs, not SHA pins or local actions"
expect "$tmp/bad.json" workflows_without_permissions 1     "counts a workflow with no top-level permissions"
expect "$tmp/bad.json" workflow_write_scopes_broad 2       "counts workflow-level write scopes a job inherits in a multi-job workflow"
expect "$tmp/bad.json" s3_without_public_access_block 1    "counts a bucket without a PublicAccessBlock"
expect "$tmp/bad.json" cloudfront_without_security_headers 2 "counts behaviours without HSTS, nosniff and frame options"
expect "$tmp/bad.json" cloudfront_tls_below_1_2_2021 1     "counts a viewer TLS minimum below TLSv1.2_2021"
expect "$tmp/bad.json" iam_full_service_wildcards 1        "counts an Allow of a whole service, not a Deny or a prefix"
if [ "$(metric "$tmp/bad.json" checkov_failed)" -gt 0 ]; then echo "ok    counts checkov failures"; else echo "FAIL  checkov_failed should be > 0"; fail=1; fi
expect "$tmp/bad.json" live_headers_missing absent         "live metrics are absent without PROBE_LIVE"

# ── static: this repository's own templates pass every template check ─────
python3 -I "$probe" --root "$here/../.." >"$tmp/repo.json" 2>"$tmp/repo.err" || { echo "FAIL  probe exited non-zero on the repo"; cat "$tmp/repo.err"; fail=1; }
for m in s3_without_public_access_block cloudfront_without_security_headers cloudfront_tls_below_1_2_2021 iam_full_service_wildcards cfn_lint_findings checkov_failed; do
  expect "$tmp/repo.json" "$m" 0 "repo templates: $m"
done

# ── live: fake network tools ──────────────────────────────────────────────
bin="$tmp/bin"
mkdir -p "$bin"
stub() { printf '#!/bin/sh\n%s\n' "$2" >"$bin/$1"; chmod +x "$bin/$1"; }
stub curl 'case "$*" in
  *http://treemand.dev*) printf "HTTP/1.1 301 Moved Permanently\r\nLocation: https://treemand.dev/\r\n\r\n" ;;
  *s3.us-east-1.amazonaws.com*) [ "$FAKE_MODE" = weak ] && echo 200 || echo 403 ;;
  *) printf "HTTP/2 200\r\ncontent-type: text/html\r\n"
     if [ "$FAKE_MODE" != weak ]; then
       printf "strict-transport-security: max-age=63072000; includeSubDomains; preload\r\nx-content-type-options: nosniff\r\nx-frame-options: DENY\r\nreferrer-policy: strict-origin-when-cross-origin\r\n"
     else
       printf "strict-transport-security: max-age=300\r\n"
     fi
     printf "\r\n" ;;
esac'
stub openssl 'case "$1" in
  s_client)
    case "$*" in
      *-tls1_1*|*-tls1\ *) [ "$FAKE_MODE" = weak ] && { echo "Protocol  : TLSv1.1"; exit 0; }; echo "alert protocol version" >&2; exit 1 ;;
    esac
    echo "-----BEGIN CERTIFICATE-----"; echo "Protocol  : TLSv1.3"; echo "Verify return code: 0 (ok)" ;;
  x509) cat >/dev/null; echo "notAfter=Jan  1 00:00:00 2099 GMT" ;;
esac'
stub aws '[ -n "$AWS_PROFILE" ] || exit 253
case "$*" in
  *list-distributions*) [ "$FAKE_MODE" = weak ] && printf "TLSv1\tNone\n" || printf "TLSv1.2_2021\tpolicy-id\n" ;;
  *) echo "not allowed: $*" >&2; exit 254 ;;
esac'

PATH="$bin:$PATH" PROBE_LIVE=1 AWS_PROFILE=fake python3 -I "$probe" --root "$here/../.." >"$tmp/live.json" 2>"$tmp/live.err" || { echo "FAIL  live probe exited non-zero"; cat "$tmp/live.err"; fail=1; }
for m in live_headers_missing live_http_not_redirected live_legacy_tls_accepted live_cert_invalid live_buckets_listable live_bucket_objects_public live_cf_tls_below_policy live_cf_without_headers_policy; do
  expect "$tmp/live.json" "$m" 0 "healthy site: $m"
done
expect "$tmp/live.json" live_headers_recommended_missing 2 "names CSP and Permissions-Policy as recommended"
if grep -Eq '[0-9]{12}|arn:' "$tmp/live.json"; then echo "FAIL  live output carries an account ID"; fail=1; else echo "ok    live output carries no account ID"; fi

PATH="$bin:$PATH" PROBE_LIVE=1 AWS_PROFILE=fake FAKE_MODE=weak python3 -I "$probe" --root "$here/../.." >"$tmp/weak.json" 2>"$tmp/weak.err" || { echo "FAIL  weak live probe exited non-zero"; cat "$tmp/weak.err"; fail=1; }
expect "$tmp/weak.json" live_headers_missing 4             "weak site: short HSTS and three missing headers"
expect "$tmp/weak.json" live_legacy_tls_accepted 2         "weak site: TLS 1.0 and 1.1 accepted"
expect "$tmp/weak.json" live_buckets_listable 2            "weak site: both buckets list publicly"
expect "$tmp/weak.json" live_cf_tls_below_policy 1         "weak site: deployed TLS minimum below policy"
expect "$tmp/weak.json" live_cf_without_headers_policy 1   "weak site: no response headers policy attached"

PATH="$bin:$PATH" PROBE_LIVE=1 python3 -I "$probe" --root "$here/../.." >"$tmp/noaws.json" 2>/dev/null
expect "$tmp/noaws.json" live_cf_tls_below_policy absent   "without AWS_PROFILE the AWS checks are skipped"
if grep -q '"live_cf_tls_below_policy"' "$tmp/noaws.json"; then echo "ok    skipped AWS checks are named"; else echo "FAIL  skipped AWS checks are not named"; fail=1; fi

exit "$fail"
