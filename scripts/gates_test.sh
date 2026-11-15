#!/usr/bin/env bash
# Proves the account-identifier and Project-tag gates fire. Fixtures are
# assembled at runtime so no flaggable literal is ever committed.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail=0

acct="$(printf '%s%s' 123456 789012)"     # 12 digits, split so this file stays clean
zone="$(printf 'Z%s' 0742QX9TJ3K8LMRW)"

leaks() { # leaks <relative-path> <content> -> exit 0 when gitleaks flags it
  local dir="$work/scan-$RANDOM"
  mkdir -p "$dir/$(dirname "$1")"
  printf '%s\n' "$2" >"$dir/$1"
  ! gitleaks dir --config "$root/.gitleaks.toml" --no-banner --log-level error "$dir" >/dev/null 2>&1
}

expect_leak() {
  if leaks "$1" "$2"; then echo "ok    flags $3"; else echo "FAIL  missed $3"; fail=1; fi
}
expect_clean() {
  if leaks "$1" "$2"; then echo "FAIL  false positive: $3"; fail=1; else echo "ok    allows $3"; fi
}

expect_leak  docs/notes.md        "role: arn:aws:iam::${acct}:role/deploy"      "ARN with account ID anywhere"
expect_leak  README.md            "AWS_ACCOUNT_ID=${acct}"                       "account ID assignment anywhere"
expect_leak  infra/site.yaml      "  Default: '${acct}'"                         "bare account ID in infra/"
expect_leak  .github/workflows/x.yml "account: ${acct}"                          "account ID in a workflow"
expect_leak  infra/site.yaml      "  HostedZoneId: ${zone}"                      "hosted zone ID in infra/"
expect_clean infra/site.yaml      "  Resource: !Sub arn:\${AWS::Partition}:s3:::treemand-site-\${AWS::AccountId}/*" "pseudo-parameter ARNs"
expect_clean cli/treemand/x_test.go "const nanos = ${acct}"                      "12-digit numbers in Go code"

lint() { # lint <template-body> -> cfn-lint exit status with the project rule
  printf '%s\n' "$1" >"$work/t.yaml"
  cfn-lint --append-rules "$root/infra/rules" --regions us-east-1 -- "$work/t.yaml" >"$work/lint.out" 2>&1
}

untagged='AWSTemplateFormatVersion: "2010-09-09"
Resources:
  Site:
    Type: AWS::S3::Bucket'
tagged='AWSTemplateFormatVersion: "2010-09-09"
Resources:
  Site:
    Type: AWS::S3::Bucket
    Properties:
      Tags: [{Key: Project, Value: treemand}]
  Policy:
    Type: AWS::S3::BucketPolicy
    Properties:
      Bucket: !Ref Site
      PolicyDocument: {Version: "2012-10-17", Statement: []}'
budget='AWSTemplateFormatVersion: "2010-09-09"
Resources:
  Budget:
    Type: AWS::Budgets::Budget
    Properties:
      Budget: {BudgetType: COST, TimeUnit: MONTHLY, BudgetLimit: {Amount: 5, Unit: USD}}'

if lint "$untagged"; then echo "FAIL  untagged bucket passed"; fail=1
elif grep -q E9001 "$work/lint.out"; then echo "ok    flags untagged taggable resource (E9001)"
else echo "FAIL  untagged bucket failed for another reason:"; cat "$work/lint.out"; fail=1; fi

if ! lint "$budget" && grep -q E9001 "$work/lint.out"; then echo "ok    flags budget without ResourceTags"
else echo "FAIL  budget without ResourceTags not flagged by E9001"; fail=1; fi

if lint "$tagged"; then echo "ok    allows tagged bucket + untaggable policy"
else echo "FAIL  tagged template rejected:"; cat "$work/lint.out"; fail=1; fi

exit "$fail"
