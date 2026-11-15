#!/usr/bin/env bash
# Deploy infra/bootstrap.yaml with admin credentials, then wire GitHub:
# a "production" environment (main + v* tags only) holding the role ARNs as
# secrets. Account-specific values are looked up here and never printed.
set -euo pipefail
cd "$(dirname "$0")/../.."

export AWS_REGION=us-east-1 AWS_DEFAULT_REGION=us-east-1
domain="${DOMAIN:-treemand.dev}"
repo="${GITHUB_REPO:-aallbrig/treemand}"
env_name=production
email="${BUDGET_EMAIL:-$(git config user.email)}"
[ -n "$email" ] || { echo "set BUDGET_EMAIL" >&2; exit 1; }

zone_id="$(aws route53 list-hosted-zones-by-name --dns-name "$domain" \
  --query "HostedZones[?Name=='${domain}.'] | [0].Id" --output text | sed 's#/hostedzone/##')"
if [ -z "$zone_id" ] || [ "$zone_id" = None ]; then echo "no hosted zone for $domain" >&2; exit 1; fi

echo "[bootstrap] deploying treemand-bootstrap (certificate validation can take a few minutes)"
aws cloudformation deploy \
  --stack-name treemand-bootstrap \
  --template-file infra/bootstrap.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --tags Project=treemand \
  --no-fail-on-empty-changeset \
  --parameter-overrides "DomainName=$domain" "HostedZoneId=$zone_id" "BudgetEmail=$email" \
  >/dev/null

output() {
  aws cloudformation describe-stacks --stack-name treemand-bootstrap \
    --query "Stacks[0].Outputs[?OutputKey=='$1'].OutputValue" --output text
}

echo "[bootstrap] configuring GitHub environment '$env_name' (main and v* tags only)"
gh api --method PUT "repos/$repo/environments/$env_name" --silent \
  -F 'deployment_branch_policy[protected_branches]=false' \
  -F 'deployment_branch_policy[custom_branch_policies]=true'
existing="$(gh api "repos/$repo/environments/$env_name/deployment-branch-policies" \
  --jq '.branch_policies[] | "\(.type):\(.name)"')"
for policy in branch:main tag:v*; do
  grep -qxF "$policy" <<<"$existing" && continue
  gh api --method POST "repos/$repo/environments/$env_name/deployment-branch-policies" --silent \
    -f "name=${policy#*:}" -f "type=${policy%%:*}"
done

output DeployRoleArn | gh secret set AWS_DEPLOY_ROLE_ARN --env "$env_name" --repo "$repo"
output CfnExecRoleArn | gh secret set AWS_CFN_EXEC_ROLE_ARN --env "$env_name" --repo "$repo"
echo "[bootstrap] done: roles treemand-deploy / treemand-cfn-exec, certificate for $domain; ARNs stored as '$env_name' secrets"
