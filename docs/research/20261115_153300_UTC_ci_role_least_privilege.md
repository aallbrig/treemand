# Least-privilege CI role and tag propagation for the site stack

Date: 2026-11-15. Ticket: [#116](https://github.com/aallbrig/treemand/issues/116) (map [#114](https://github.com/aallbrig/treemand/issues/114)).

Method: primary sources only. IAM facts come from AWS's machine-readable Service Authorization
Reference (`https://servicereference.us-east-1.amazonaws.com/v1/<service>/<service>.json`, the data
behind the `list_<service>.html` pages). Tagging and handler-permission facts come from the
CloudFormation resource provider schemas
(`https://schema.cloudformation.us-east-1.amazonaws.com/CloudformationSchema.zip`, us-east-1). The rest
comes from the AWS user guides, the GitHub docs and the aws-cli source, cited inline. Nothing here
was run against a live account. Items marked **verify** are inferences to confirm on the first
deploy.

No real account ID, zone ID or ARN appears below. Templates use `${AWS::AccountId}` and parameters,
and prose uses `<ACCOUNT_ID>`.

## Answer in brief

1. **Use two roles plus one boundary, all created in the bootstrap stack.** The CI role
   (`treemand-deploy`, assumed through GitHub OIDC) can only drive change sets on the
   `treemand-site` stack, pass one role, sync the site bucket and invalidate a tagged distribution.
   The resource-creation rights belong to a CloudFormation service role (`treemand-cfn-exec`) that
   only `cloudformation.amazonaws.com` can assume. A permissions boundary (`treemand-boundary`) on
   both roles caps them at the six services in play, us-east-1 only, with no IAM except one
   `iam:PassRole`. This is the routine-mark pattern (`~/src/routine-mark-loop/infra/bootstrap.yaml`),
   with OIDC in place of the IAM group.
2. **Stack-level `--tags` does not reach every resource.** CloudFormation propagates stack tags only
   to resource types whose schema says `tagging.taggable: true`. In this stack that is the S3 buckets,
   the CloudFront distribution, the CloudFront Function and the Athena workgroup. Six types in the
   stack can't hold tags at all: BucketPolicy, OriginAccessControl, ResponseHeadersPolicy,
   Route 53 RecordSet, Glue Database and Glue Table. Through CloudFormation, nothing can tag them,
   but none of them is billed on its own.
3. **Put explicit `Tags: [{Key: Project, Value: treemand}]` on every taggable resource and pass
   `--tags Project=treemand` as well.** The explicit tags let the planned "every resource has
   `Project=treemand`" gate check the template statically, because stack tags never appear in the
   template. That gate must skip the non-taggable types listed above, or it fails on resources that
   can't be fixed.

## 1. Tag propagation per resource type

Sources:

- [AWS::CloudFormation::Stack `Tags`](https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-resource-cloudformation-stack.html):
  "CloudFormation also propagates these tags to the resources created in the stack."
- [Configure stack options](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cfn-console-create-stack.html#configure-stack-options):
  tags apply "to your stack and to any resources that CloudFormation supports tagging for".
  "Adding, updating, or removing stack-level tags will initiate a stack update. All resources that
  support stack-level tag propagation will be updated accordingly."
- `aws cloudformation deploy help` (aws-cli 2.33.1): `--tags` "AWS CloudFormation also propagates
  these tags to resources in the stack if the resource supports it."
- [Resource type schema](https://docs.aws.amazon.com/cloudformation-cli/latest/userguide/resource-type-schema.html):
  `tagging.taggable` is "Whether this resource type supports tagging", and
  `cloudFormationSystemTags` is whether the type gets the `aws:cloudformation:*` system tags.

The flags below are read from each type's schema in the us-east-1 schema zip.

| Resource type (site stack) | `taggable` | `Tags` property | Stack `--tags` propagate? | Explicit `Tags` needed? | `aws:cloudformation:*` system tags | Billed? |
|---|---|---|---|---|---|---|
| `AWS::S3::Bucket` (site, logs) | true | yes | **yes** | recommended (for the static gate) | yes | yes: storage and requests per bucket |
| `AWS::S3::BucketPolicy` | false | no | no | n/a (can't be tagged) | no | no |
| `AWS::CloudFront::Distribution` | true | yes | **yes** | recommended | **no** | yes: data transfer and requests |
| `AWS::CloudFront::Function` | true | yes | **yes** | recommended | no | yes: per invocation (see note) |
| `AWS::CloudFront::ResponseHeadersPolicy` | false | no | no | n/a | no | no |
| `AWS::CloudFront::OriginAccessControl` | false | no | no | n/a | no | no |
| `AWS::Route53::RecordSet` | false | no | no | n/a | no | queries bill to the hosted zone (outside this stack) |
| `AWS::Glue::Database` | false | no | no | n/a (the Glue API can tag a database, but this CloudFormation type exposes no `Tags`) | no | Data Catalog is metered per object and request |
| `AWS::Glue::Table` | false | no | no | n/a | no | as above |
| `AWS::Athena::WorkGroup` | true | yes | **yes** | recommended | yes | yes: per query |

Bootstrap stack, for the same gate:

| Type | `taggable` | Notes |
|---|---|---|
| `AWS::IAM::Role` | true | propagates |
| `AWS::IAM::ManagedPolicy` | **false** | the boundary and permission policies can't be tagged; the gate must exempt them |
| `AWS::CertificateManager::Certificate` | true | propagates; public certificates are free |
| `AWS::Budgets::Budget` | true | the tag property is **`ResourceTags`**, not `Tags`; a gate that looks for `Tags` will misfire |

Cost Explorer caveats, from the [cost allocation tags](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/cost-alloc-tags.html),
[user-defined tags](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/custom-tags.html) and
[activating tags](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/activating-tags.html) docs:

- Tags must be **activated** in Billing → Cost allocation tags before Cost Explorer and Budgets see
  them. A tag key appears for activation only after it is on a resource, "up to 24 hours", and
  activation takes "up to 24 hours" more. Only a management account or a standalone account can
  activate tags.
- "Tags are not applied to resources that were created before the tags were created." Tag from the
  first deploy, and expect `Project` to be filterable about 48 hours after that.
- [Athena](https://docs.aws.amazon.com/athena/latest/ug/tags.html): with workgroup tags activated,
  "costs associated with running queries appear in your Cost and Usage Report with that cost
  allocation tag". Queries can't be tagged, so `task metrics:downloads` must run in the
  `treemand-metrics` workgroup.
- [CloudFront tagging](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/tagging.html)
  names distributions as the billing-tag target. It lists functions as taggable but doesn't say
  whether Functions invocation line items carry the function's tags. Treat Functions cost as
  possibly untagged (**verify** in Cost Explorer after a month). It is small either way.
- **What the `Project=treemand` budget will miss:** the Route 53 hosted zone (monthly charge plus
  queries), unless you tag the zone once by hand, since it isn't in the stack; Glue Data Catalog
  usage; and possibly Functions invocations. All are cents a month, so the ~$5 budget stays
  meaningful, but it isn't a complete total.
- `aws:cloudformation:stack-name` is no substitute for `Project`, because CloudFront types don't get
  CloudFormation system tags (`cloudFormationSystemTags: false`).

## 2. How a deploy flows

```
GitHub Actions (main / v* tag, environment "production")
  └─ sts:AssumeRoleWithWebIdentity → treemand-deploy      [boundary: treemand-boundary]
       ├─ cloudformation:CreateChangeSet … --role-arn treemand-cfn-exec  (iam:PassRole)
       │     └─ CloudFormation assumes treemand-cfn-exec  [boundary: treemand-boundary]
       │           └─ creates/updates S3, CloudFront, Route 53 records, Glue, Athena
       ├─ s3 sync → treemand-site-<ACCOUNT_ID> bucket
       └─ cloudfront:CreateInvalidation on the distribution tagged Project=treemand
```

API calls made by `aws cloudformation deploy` (aws-cli v2 source,
[`awscli/customizations/cloudformation/deployer.py`](https://github.com/aws/aws-cli/blob/v2/awscli/customizations/cloudformation/deployer.py)):
`DescribeStacks`, `GetTemplateSummary` (on update), `CreateChangeSet`, the
`change_set_create_complete` waiter (`DescribeChangeSet`), `ExecuteChangeSet`, and the
`stack_create_complete` / `stack_update_complete` waiters (`DescribeStacks`). `--s3-bucket` is
"required the deployments of templates sized greater than 51,200 bytes". The site template should
stay under that, so the CI role needs no artifact bucket.

The [CloudFormation service role doc](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-iam-servicerole.html)
says: "CloudFormation always uses that role for all operations that are performed on that stack.
It is not possible to remove a service role attached to a stack after the stack is created." The
first CI deploy must pass `--role-arn`, and the `cloudformation:RoleArn` condition below makes it
impossible to forget.

## 3. OIDC trust condition

Sources: [GitHub: Configuring OIDC in AWS](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services),
[IAM: role for GitHub OIDC](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-idp_oidc.html#idp_oidc_Create_GitHub).

- `aud` must be `sts.amazonaws.com` (`StringEquals`).
- IAM rejects a GitHub-OIDC trust policy unless `token.actions.githubusercontent.com:sub` "is
  present and that its value is not solely a wildcard character (\* and ?) or null". This is checked
  when the trust policy is created or updated.
- The `sub` formats are `repo:ORG/REPO:ref:refs/heads/BRANCH` and `repo:ORG/REPO:ref:refs/tags/TAG`.
  When the job names an environment, the format becomes `repo:ORG/REPO:environment:NAME` instead of
  a ref form. Repos created after 2026-07-15 (or opted in) use an immutable-ID form
  `repo:ORG@id/REPO@id:…`. aallbrig/treemand was created 2026-02-21, so the classic form applies
  unless someone opts in.
- Workflows need `permissions: id-token: write` (plus `contents: read` for checkout).

**Recommended: one GitHub environment, one exact `sub`.** Create a `production` environment whose
deployment rules allow branch `main` and tags `v*`, and give both deploy jobs `environment:
production`:

```yaml
AssumeRolePolicyDocument:
  Version: '2012-10-17'
  Statement:
    - Effect: Allow
      Principal:
        Federated: !Sub arn:${AWS::Partition}:iam::${AWS::AccountId}:oidc-provider/token.actions.githubusercontent.com
      Action: sts:AssumeRoleWithWebIdentity
      Condition:
        StringEquals:
          token.actions.githubusercontent.com:aud: sts.amazonaws.com
          token.actions.githubusercontent.com:sub: repo:aallbrig/treemand:environment:production
```

**Alternative without environments** (ref-based):

```yaml
      Condition:
        StringEquals:
          token.actions.githubusercontent.com:aud: sts.amazonaws.com
        StringLike:
          token.actions.githubusercontent.com:sub:
            - repo:aallbrig/treemand:ref:refs/heads/main
            - repo:aallbrig/treemand:ref:refs/tags/v*
```

Either way, `pull_request` runs (sub `repo:aallbrig/treemand:pull_request`) can't assume the role,
and neither can forks. You can't mix the two: once a job names an environment, the ref-form entries
stop matching it. The provider ARN is built from `${AWS::AccountId}`, so it needs no parameter, and
the provider is referenced rather than created (#114 Notes).

## 4. Policy sketches (bootstrap stack, CloudFormation YAML)

Assumed names: stack `treemand-site`; buckets `treemand-site-${AWS::AccountId}` and
`treemand-logs-${AWS::AccountId}`; function `treemand-*`; Glue database `treemand_metrics` (Athena
and Glue prefer underscores, so Glue ARNs use `treemand*` rather than `treemand-*`); workgroup
`treemand-metrics`. `HostedZoneId` is a bootstrap parameter supplied at deploy time and never
committed.

Resource-level support per action is taken from the Service Authorization Reference JSON:

| Action group | Resource scoping available | Condition keys used |
|---|---|---|
| `cloudfront:CreateDistribution`, `CreateFunction` | `*` only | `aws:RequestTag/${TagKey}`, `aws:TagKeys` |
| `cloudfront:CreateResponseHeadersPolicy`, `CreateOriginAccessControl` | `*` only | none |
| `cloudfront:{Get,Update,Delete}Distribution*`, `CreateInvalidation`, `GetInvalidation` | `distribution/${Id}` | resource supports `aws:ResourceTag` |
| `cloudfront:*Function` (non-create) | `function/${Name}`, so a name prefix works | `aws:ResourceTag` |
| `cloudfront:*ResponseHeadersPolicy`, `*OriginAccessControl` | by ID only, **no tag support** | none |
| `route53:ChangeResourceRecordSets` | `hostedzone/${Id}` | `route53:ChangeResourceRecordSets{NormalizedRecordNames,RecordTypes,Actions}` |
| `route53:GetChange` | `change/${Id}` | none |
| `route53:ListHostedZones` | `*` | none |
| `glue:*Table` | catalog + database + table ARNs | none useful |
| `athena:*WorkGroup` | `workgroup/${Name}` | `aws:RequestTag` on create |
| `cloudformation:CreateChangeSet` | `stack/${Name}/${Id}` | `cloudformation:RoleArn`, `aws:RequestTag/${TagKey}` |
| `iam:PassRole` | `role/${Name}` | `iam:PassedToService` |

Per [Route 53 condition docs](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/specifying-rrset-conditions.html),
record names must be lowercase with no trailing dot, matched with `ForAllValues:StringEquals`.
"Route 53 doesn't support tag-based condition keys."

### 4a. CI role: `treemand-deploy`

```yaml
DeployRole:
  Type: AWS::IAM::Role
  Properties:
    RoleName: treemand-deploy
    MaxSessionDuration: 3600
    PermissionsBoundary: !Ref Boundary
    AssumeRolePolicyDocument: { … section 3 … }
    Tags: [{ Key: Project, Value: treemand }]
    Policies:
      - PolicyName: treemand-deploy
        PolicyDocument:
          Version: '2012-10-17'
          Statement:
            - Sid: ChangeSetsOnSiteStackOnlyViaServiceRole
              Effect: Allow
              Action: cloudformation:CreateChangeSet
              Resource: !Sub arn:${AWS::Partition}:cloudformation:us-east-1:${AWS::AccountId}:stack/treemand-site/*
              Condition:
                StringEquals:
                  cloudformation:RoleArn: !GetAtt CfnExecRole.Arn
                  aws:RequestTag/Project: treemand      # refuses a deploy without --tags Project=treemand
            - Sid: DriveSiteStack
              Effect: Allow
              Action:
                - cloudformation:DescribeStacks
                - cloudformation:DescribeStackEvents
                - cloudformation:DescribeChangeSet
                - cloudformation:ExecuteChangeSet
                - cloudformation:DeleteChangeSet
                - cloudformation:GetTemplateSummary
                - cloudformation:TagResource         # stack tags on create; verify whether needed
              Resource: !Sub arn:${AWS::Partition}:cloudformation:us-east-1:${AWS::AccountId}:stack/treemand-site/*
            - Sid: HandOffToServiceRole
              Effect: Allow
              Action: iam:PassRole
              Resource: !GetAtt CfnExecRole.Arn
              Condition:
                StringEquals: { iam:PassedToService: cloudformation.amazonaws.com }
            - Sid: SyncSiteBucket
              Effect: Allow
              Action: s3:ListBucket
              Resource: !Sub arn:${AWS::Partition}:s3:::treemand-site-${AWS::AccountId}
            - Sid: SyncSiteObjects
              Effect: Allow
              Action: [s3:PutObject, s3:DeleteObject]   # sync needs no GetObject
              Resource: !Sub arn:${AWS::Partition}:s3:::treemand-site-${AWS::AccountId}/*
            - Sid: ReleasesAreImmutable
              Effect: Deny
              Action: s3:DeleteObject
              Resource: !Sub arn:${AWS::Partition}:s3:::treemand-site-${AWS::AccountId}/releases/v*
            - Sid: InvalidateOwnDistribution
              Effect: Allow
              Action: [cloudfront:CreateInvalidation, cloudfront:GetInvalidation, cloudfront:GetDistribution]
              Resource: !Sub arn:${AWS::Partition}:cloudfront::${AWS::AccountId}:distribution/*
              Condition:
                StringEquals: { aws:ResourceTag/Project: treemand }
```

Notes:

- The distribution ID never appears in the policy. The `aws:ResourceTag` condition picks the right
  distribution, and CI reads the ID from the stack outputs (`DescribeStacks`), which keeps it out of
  git as #114 requires.
- With the `ReleasesAreImmutable` deny, `aws s3 sync --delete` must `--exclude 'releases/*'`, or the
  sync fails. That is the intended behaviour. An overwrite through `PutObject` is still possible; the
  only S3-side guard is a conditional write (`s3:if-none-match` is a `PutObject` condition key),
  which is optional hardening.
- The CI role needs no `CAPABILITY_IAM`, because the site stack must contain no `AWS::IAM::*`. Add
  that to the cfn-lint/policy gate.

### 4b. Service role: `treemand-cfn-exec`

The actions are the union of the `create`/`read`/`update`/`delete` handler permissions in each
type's provider schema (section 1 source). Only the features this stack uses are kept: no
replication, inventory, S3 Tables or KMS.

```yaml
CfnExecRole:
  Type: AWS::IAM::Role
  Properties:
    RoleName: treemand-cfn-exec
    PermissionsBoundary: !Ref Boundary
    Tags: [{ Key: Project, Value: treemand }]
    AssumeRolePolicyDocument:
      Version: '2012-10-17'
      Statement:
        - Effect: Allow
          Principal: { Service: cloudformation.amazonaws.com }
          Action: sts:AssumeRole
          Condition:
            StringEquals: { aws:SourceAccount: !Ref AWS::AccountId }
            # verify: tighten with ArnLike aws:SourceArn …:stack/treemand-site/*
    Policies:
      - PolicyName: treemand-site-resources
        PolicyDocument:
          Version: '2012-10-17'
          Statement:
            - Sid: Buckets
              Effect: Allow
              Action:
                - s3:CreateBucket
                - s3:DeleteBucket
                - s3:ListBucket
                - s3:GetBucket*            # incl. GetBucketAcl, GetBucketPolicy, GetBucketTagging
                - s3:PutBucket*            # incl. PutBucketAcl (legacy logs), Policy, Tagging, OwnershipControls, PublicAccessBlock
                - s3:DeleteBucketPolicy
                - s3:DeleteBucketTagging
                - s3:GetLifecycleConfiguration
                - s3:PutLifecycleConfiguration
                - s3:GetEncryptionConfiguration
                - s3:PutEncryptionConfiguration
                - s3:ListTagsForResource
                - s3:TagResource
                - s3:UntagResource
              Resource: !Sub arn:${AWS::Partition}:s3:::treemand-*
            - Sid: CreateTaggedDistributionsAndFunctions
              Effect: Allow
              Action: [cloudfront:CreateDistribution, cloudfront:CreateDistributionWithTags, cloudfront:CreateFunction, cloudfront:TagResource]
              Resource: '*'
              Condition:
                StringEquals: { aws:RequestTag/Project: treemand }   # verify on first create
            - Sid: OwnDistributions
              Effect: Allow
              Action:
                - cloudfront:GetDistribution
                - cloudfront:GetDistributionConfig
                - cloudfront:UpdateDistribution
                - cloudfront:DeleteDistribution
                - cloudfront:ListTagsForResource
                - cloudfront:TagResource
                - cloudfront:UntagResource
              Resource: !Sub arn:${AWS::Partition}:cloudfront::${AWS::AccountId}:distribution/*
              Condition:
                StringEquals: { aws:ResourceTag/Project: treemand }
            - Sid: OwnFunctions
              Effect: Allow
              Action:
                - cloudfront:DescribeFunction
                - cloudfront:GetFunction
                - cloudfront:UpdateFunction
                - cloudfront:PublishFunction
                - cloudfront:DeleteFunction
                - cloudfront:ListTagsForResource
                - cloudfront:TagResource
                - cloudfront:UntagResource
              Resource: !Sub arn:${AWS::Partition}:cloudfront::${AWS::AccountId}:function/treemand-*
            - Sid: UntaggableCloudFrontConfig   # IDs only, no tag keys: cannot be scoped tighter
              Effect: Allow
              Action:
                - cloudfront:CreateOriginAccessControl
                - cloudfront:GetOriginAccessControl
                - cloudfront:UpdateOriginAccessControl
                - cloudfront:DeleteOriginAccessControl
                - cloudfront:CreateResponseHeadersPolicy
                - cloudfront:GetResponseHeadersPolicy
                - cloudfront:UpdateResponseHeadersPolicy
                - cloudfront:DeleteResponseHeadersPolicy
              Resource: '*'
            - Sid: DnsRecordsForSiteOnly
              Effect: Allow
              Action: route53:ChangeResourceRecordSets
              Resource: !Sub arn:${AWS::Partition}:route53:::hostedzone/${HostedZoneId}
              Condition:
                ForAllValues:StringEquals:
                  route53:ChangeResourceRecordSetsNormalizedRecordNames: [treemand.dev, www.treemand.dev]
                  route53:ChangeResourceRecordSetsRecordTypes: [A, AAAA]
            - Sid: DnsRead
              Effect: Allow
              Action: [route53:GetHostedZone, route53:ListResourceRecordSets]
              Resource: !Sub arn:${AWS::Partition}:route53:::hostedzone/${HostedZoneId}
            - Sid: DnsGlobalRead
              Effect: Allow
              Action: [route53:GetChange, route53:ListHostedZones]
              Resource: '*'
            - Sid: GlueCatalog
              Effect: Allow
              Action:
                - glue:CreateDatabase
                - glue:GetDatabase
                - glue:UpdateDatabase
                - glue:DeleteDatabase
                - glue:CreateTable
                - glue:GetTable
                - glue:UpdateTable
                - glue:DeleteTable
              Resource:
                - !Sub arn:${AWS::Partition}:glue:us-east-1:${AWS::AccountId}:catalog
                - !Sub arn:${AWS::Partition}:glue:us-east-1:${AWS::AccountId}:database/treemand*
                - !Sub arn:${AWS::Partition}:glue:us-east-1:${AWS::AccountId}:table/treemand*/*
            - Sid: LakeFormationReadsInGlueHandlers   # listed in the Glue handler permissions; no resource-level support
              Effect: Allow
              Action:
                - lakeformation:ListResources
                - lakeformation:DescribeResource
                - lakeformation:DescribeLakeFormationIdentityCenterConfiguration
                - lakeformation:GetDataAccess
              Resource: '*'
            - Sid: AthenaWorkgroup
              Effect: Allow
              Action:
                - athena:CreateWorkGroup
                - athena:GetWorkGroup
                - athena:UpdateWorkGroup
                - athena:DeleteWorkGroup
                - athena:TagResource
                - athena:UntagResource
                - athena:ListTagsForResource
              Resource: !Sub arn:${AWS::Partition}:athena:us-east-1:${AWS::AccountId}:workgroup/treemand-*
```

Notes:

- **Legacy standard logs need ACL rights on the log bucket.** Per
  [CloudFront standard logging (legacy)](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/standard-logging-legacy-s3.html),
  the caller of `CreateDistribution`/`UpdateDistribution` needs `s3:GetBucketAcl` and
  `s3:PutBucketAcl`. The bucket must have ACLs enabled, so `OwnershipControls` must be
  `BucketOwnerPreferred`, not "bucket owner enforced". `s3:GetBucket*`/`s3:PutBucket*` above cover
  this. Standard logging v2 would add `AWS::Logs::Delivery*` resource types and `logs:*` actions
  instead. The legacy path is the smaller grant.
- The handler lists for `AWS::Glue::Table` and `AWS::Athena::WorkGroup` also include `iam:PassRole`,
  `s3:PutObject`, `kms:*` and `lakeformation:CreateTable`. These serve features this stack doesn't
  use (Iceberg/governed tables, KMS result encryption, service-linked roles), so they are left out.
  If the first deploy fails with AccessDenied naming one of them, add it with the narrowest resource.
- **Residual risk:** OAC and response-headers-policy actions can't be scoped by tag or name. This
  role could therefore delete another project's OAC or policy if other projects share the account.
  These objects are free and appear in CloudFormation drift, so the risk
  is accepted rather than engineered away.
- No IAM actions at all, so the site stack has no privilege-escalation path.

### 4c. Permissions boundary: `treemand-boundary`

The boundary sets "the maximum permissions that an identity-based policy can grant"; effective
permissions are "the intersection of both policy types"
([IAM: permissions boundaries](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_boundaries.html)).
It is coarse on purpose. The identity policies above do the fine scoping, and the boundary stops a
later edit that widens them from reaching other services, other regions or IAM.

```yaml
Boundary:
  Type: AWS::IAM::ManagedPolicy        # not taggable (schema): exempt from the tag gate
  Properties:
    ManagedPolicyName: treemand-boundary
    PolicyDocument:
      Version: '2012-10-17'
      Statement:
        - Sid: ServicesInPlay
          Effect: Allow
          Action: [cloudformation:*, cloudfront:*, route53:*, glue:*, athena:*, lakeformation:Get*, lakeformation:List*, lakeformation:Describe*, sts:GetCallerIdentity]
          Resource: '*'
        - Sid: OwnBucketsOnly
          Effect: Allow
          Action: s3:*
          Resource:
            - !Sub arn:${AWS::Partition}:s3:::treemand-*
            - !Sub arn:${AWS::Partition}:s3:::treemand-*/*
        - Sid: PassOnlyTheExecRole
          Effect: Allow
          Action: iam:PassRole
          Resource: !Sub arn:${AWS::Partition}:iam::${AWS::AccountId}:role/treemand-cfn-exec
          Condition:
            StringEquals: { iam:PassedToService: cloudformation.amazonaws.com }
        - Sid: UsEast1Only   # CloudFront, Route 53 and IAM requests are signed for us-east-1 (verify)
          Effect: Deny
          Action: '*'
          Resource: '*'
          Condition:
            StringNotEquals: { aws:RequestedRegion: us-east-1 }
        - Sid: OnlyTheSiteStack
          Effect: Deny
          Action: [cloudformation:CreateStack, cloudformation:UpdateStack, cloudformation:DeleteStack, cloudformation:CreateChangeSet, cloudformation:ExecuteChangeSet]
          NotResource: !Sub arn:${AWS::Partition}:cloudformation:us-east-1:${AWS::AccountId}:stack/treemand-site/*
        - Sid: NoBootstrapSelfEdit   # belt-and-braces; IAM is already absent from the allows
          Effect: Deny
          Action: iam:*
          NotResource: !Sub arn:${AWS::Partition}:iam::${AWS::AccountId}:role/treemand-cfn-exec
```

Because the boundary allows no IAM writes, neither role can edit its own policy, the boundary or the
trust policy. The bootstrap stack stays admin-only, as #114 requires.

## 5. Implications for the gates (#114 Notes)

- **Tag gate:** check that every resource whose type has `tagging.taggable: true` carries
  `Project=treemand` in its tag property (`Tags`, or `ResourceTags` for `AWS::Budgets::Budget`), and
  skip the types that are false. Generate the list from the schema zip, or hard-code it for this
  small set: BucketPolicy, OAC, ResponseHeadersPolicy, RecordSet, Glue Database/Table,
  IAM ManagedPolicy.
- **Site template gate:** no `AWS::IAM::*` in the site template, so CI never needs
  `CAPABILITY_IAM`.
- **Deploy command:** `aws cloudformation deploy --stack-name treemand-site --role-arn <from secret>
  --tags Project=treemand --no-fail-on-empty-changeset …`. The `aws:RequestTag/Project` condition
  enforces the tag at the IAM layer, and the `cloudformation:RoleArn` condition enforces the role.
- **One-off manual step:** activate `Project` as a cost allocation tag after the first deploy (wait
  about 24 hours for the key to appear). Optionally tag the pre-existing hosted zone by hand.

## Open items to verify on first deploy

1. Whether `CreateDistribution` through CloudFormation is authorized with `aws:RequestTag/Project`
   present (the handler lists `CreateDistributionWithTags`). If not, drop that condition and rely on
   the `ResourceTag`-scoped statements.
2. Whether `cloudformation:TagResource` is needed for `CreateChangeSet` with tags on a new stack.
3. Whether `aws:SourceArn` is populated when CloudFormation assumes the service role, which would
   allow tightening the trust policy to the `treemand-site` stack.
4. Whether `aws:RequestedRegion` is `us-east-1` for every CloudFront and Route 53 call made through
   the CLI and CloudFormation. If any call isn't, scope the region deny with `NotAction`.
5. Whether CloudFront Functions invocation charges show up under `Project=treemand` in Cost
   Explorer.
