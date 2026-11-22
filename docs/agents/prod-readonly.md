# Read-only AWS access for the prod window

The AWS pane of the `prod` window (`scripts/prod/aws.sh`, also run by
`task prod:status`) needs only read access. Monitoring never uses write
credentials: it assumes the `treemand-prod-readonly` role from
`infra/bootstrap.yaml`.

## What the role can do

The role grants exactly the calls `scripts/prod/aws.sh` makes, and no write
action:

| Action | Scope |
| --- | --- |
| `sts:GetCallerIdentity` | needs no grant |
| `cloudformation:DescribeStacks` | the `treemand-site` stack |
| `cloudformation:DescribeStackResource` | the `treemand-bootstrap` stack (finds the budget name) |
| `cloudfront:GetDistribution` | distributions tagged `Project=treemand` |
| `cloudfront:ListDistributions` | `*` (the API has no resource scope; used to find the distribution by alias) |
| `budgets:ViewBudget` | the `treemand-monthly` budget |

If the pane gains a call, add its action to the role in the same change.

**Trust**: any principal in the same account whose own IAM policy allows
`sts:AssumeRole` on the role. There is no MFA condition: the pane refreshes
unattended, and an MFA prompt each time the role's credentials expire would
stop it.

**Boundary**: none. The automation boundary leaves out Budgets, and the role
has no write action to cap.

## Deploying it

The role lives in the bootstrap stack, which is deployed with admin
credentials (`task infra:bootstrap`). To review the change before applying it,
create a change set and check that it only adds the role:

```sh
aws cloudformation create-change-set --region us-east-1 \
  --stack-name treemand-bootstrap --change-set-name add-readonly-role \
  --template-body file://infra/bootstrap.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --tags Key=Project,Value=treemand \
  --parameters ParameterKey=DomainName,UsePreviousValue=true ...
aws cloudformation describe-change-set --region us-east-1 \
  --stack-name treemand-bootstrap --change-set-name add-readonly-role \
  --query 'Changes[].ResourceChange.[Action,LogicalResourceId,Replacement]'
```

Pass every stack parameter with `UsePreviousValue=true`, and keep the stack tag:
a change to stack tags updates every resource. Execute the change set only when
its sole change is `Add ReadOnlyRole`.

## Configuring the profile

Add a profile to `~/.aws/config`. The source profile holds the credentials you
already use, and the role ARN comes from the stack output `ReadOnlyRoleArn`.
Replace `<account-id>` with your account's ID. Never commit this file.

```ini
[profile treemand-readonly]
role_arn = arn:aws:iam::<account-id>:role/treemand-prod-readonly
source_profile = default
region = us-east-1
```

Then run the panes under it:

```sh
AWS_PROFILE=treemand-readonly task prod:status
AWS_PROFILE=treemand-readonly sh scripts/prod/aws.sh --once
```

The `prod` window inherits `AWS_PROFILE` from the environment that starts the
dev session.

The AWS CLI caches the role's credentials under `~/.aws/cli/cache` and renews
them from the source profile when they expire. The same profile works from a
container if `~/.aws` is mounted read-only into it.
