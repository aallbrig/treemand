#!/usr/bin/env bash
# Deploy infra/site.yaml through the treemand-cfn-exec service role. Used by
# CI (role ARN from the production environment secret) and by hand (role ARN
# read from the bootstrap stack). Nothing account-specific is printed.
set -euo pipefail
cd "$(dirname "$0")/../.."
export AWS_REGION=us-east-1 AWS_DEFAULT_REGION=us-east-1

exec_role="${AWS_CFN_EXEC_ROLE_ARN:-$(aws cloudformation describe-stacks --stack-name treemand-bootstrap \
  --query "Stacks[0].Outputs[?OutputKey=='CfnExecRoleArn'].OutputValue" --output text)}"
if [ -n "${GITHUB_ACTIONS:-}" ]; then echo "::add-mask::$exec_role"; fi

echo "[infra:deploy] deploying treemand-site"
aws cloudformation deploy \
  --stack-name treemand-site \
  --template-file infra/site.yaml \
  --role-arn "$exec_role" \
  --tags Project=treemand \
  --no-fail-on-empty-changeset \
  >/dev/null
echo "[infra:deploy] treemand-site is up to date"
