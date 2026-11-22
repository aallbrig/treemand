#!/bin/sh
# aws.sh [--once]: production AWS status for the prod window, through free,
# read-only APIs only (STS, CloudFormation describe, CloudFront, Budgets; never
# CloudWatch metrics, Athena or Cost Explorer). Honors AWS_PROFILE. Without
# working credentials it prints a hint and exits 0, so the pane keeps refreshing.
# The account ID is used as an argument only and redacted from every line.
# Permissions a read-only profile needs: sts:GetCallerIdentity,
# cloudformation:DescribeStacks, cloudformation:DescribeStackResource,
# cloudfront:GetDistribution, cloudfront:ListDistributions, budgets:ViewBudget.
# See scripts/prod/lib.sh for the modes.
# shellcheck source=scripts/prod/lib.sh
. "$(dirname "$0")/lib.sh"

export AWS_REGION=us-east-1 AWS_DEFAULT_REGION=us-east-1 AWS_PAGER=''
site_stack="${PROD_SITE_STACK:-treemand-site}"
bootstrap_stack="${PROD_BOOTSTRAP_STACK:-treemand-bootstrap}"
site_alias="${PROD_SITE_ALIAS:-treemand.dev}"

distribution_id() { # from the site stack's outputs, else by the distribution's alias
  id="$(aws cloudformation describe-stacks --stack-name "$site_stack" \
    --query "Stacks[0].Outputs[?OutputKey=='DistributionId'].OutputValue" --output text 2>"$PROD_TMP")"
  case "$id" in '' | None) ;; *) printf '%s' "$id"; return 0 ;; esac
  id="$(aws cloudfront list-distributions \
    --query "DistributionList.Items[?Aliases.Items && contains(Aliases.Items, '$site_alias')].Id | [0]" \
    --output text 2>>"$PROD_TMP")"
  case "$id" in '' | None) return 1 ;; *) printf '%s' "$id" ;; esac
}

cloudfront() {
  if ! id="$(distribution_id)"; then
    printf 'error: CloudFront: no distribution found (%s)\n' "$(prod_err "$PROD_TMP")"; return 1
  fi
  if ! r="$(aws cloudfront get-distribution --id "$id" \
      --query '[Distribution.Status, Distribution.DistributionConfig.Enabled]' --output text 2>"$PROD_TMP")"; then
    printf 'error: CloudFront: %s\n' "$(prod_err "$PROD_TMP")"; return 1
  fi
  status="$(printf '%s' "$r" | cut -f1)"
  enabled="$(printf '%s' "$r" | cut -f2)"
  mark=ok
  state=enabled
  [ "$enabled" = True ] || { state=DISABLED; mark=FAIL; }
  [ "$status" = Deployed ] || [ "$mark" = FAIL ] || mark=WARN
  printf '%-4s CloudFront %s, %s\n' "$mark" "$status" "$state"
  [ "$mark" != FAIL ]
}

budget() { # budget <account id>
  name="$(aws cloudformation describe-stack-resource --stack-name "$bootstrap_stack" \
    --logical-resource-id MonthlyBudget --query StackResourceDetail.PhysicalResourceId \
    --output text 2>/dev/null)" || name=''
  [ -n "$name" ] || name=treemand-monthly
  if ! r="$(aws budgets describe-budget --account-id "$1" --budget-name "$name" \
      --query '[Budget.CalculatedSpend.ActualSpend.Amount, Budget.CalculatedSpend.ForecastedSpend.Amount, Budget.BudgetLimit.Amount, Budget.BudgetLimit.Unit]' \
      --output text 2>"$PROD_TMP")"; then
    printf 'error: budget %s: %s\n' "$name" "$(prod_err "$PROD_TMP")"; return 1
  fi
  printf '%s\n' "$r" | awk -F '\t' -v name="$name" '{
    pct = ($3 > 0) ? 100 * $1 / $3 : 0
    mark = (pct >= 100) ? "FAIL" : (pct >= 80) ? "WARN" : "ok"
    forecast = ($2 ~ /^[0-9.]+$/) ? sprintf(", forecast %.2f", $2) : ""
    printf "%-4s budget %s: %.2f of %.2f %s (%d%%)%s\n", mark, name, $1, $3, $4, pct, forecast
    exit (mark == "FAIL") }'
}

report() {
  if ! acct="$(aws sts get-caller-identity --query Account --output text 2>"$PROD_TMP")"; then
    printf 'hint: no AWS access (%s); set AWS_PROFILE to a read-only profile, see #254\n' "$(prod_err "$PROD_TMP")"
    return 0
  fi
  echo "ok   AWS access (profile ${AWS_PROFILE:-default})"
  bad=0
  cloudfront || bad=1
  budget "$acct" || bad=1
  return "$bad"
}

prod_main "aws" report "$@"
