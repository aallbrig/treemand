---
name: treemand-infra-security
description: Measure treemand's AWS and supply-chain security posture (CloudFormation templates, GitHub Actions pinning and permissions, the live site's TLS and headers) and turn findings into neutral public issues. Use for infra or workflow security questions, a change to infra/ or .github/workflows, or infra-security findings.
---

# Infra security

Follow **treemand-measure** for running, recording and filing; this skill is the security playbook and the rules for writing about it in public.

## Measure

- **Static** (offline, deterministic; `task ci` gates on it): `task infra:security`, or `task measure -- infra-security --findings` for the evidence. The probe's docstring (`scripts/metrics/infra_security.py`) defines every metric; `scripts/metrics/rules.json` holds their budgets and ratchets. The template checks are cfn-lint rules in `infra/security_rules/`, kept out of `task infra:lint` so the engine judges their counts.
- **Live** (read-only, optional): `PROBE_LIVE=1 AWS_PROFILE=treemand-readonly task measure -- infra-security --findings`. It sends anonymous HTTPS requests to the site and the buckets' regional endpoints, runs `openssl s_client`, and calls only `cloudfront list-distributions`, which the read-only role allows (`docs/agents/prod-readonly.md`). Without `AWS_PROFILE` the AWS checks report skipped. A check that needs more than the role grants is listed under `not_checked` in details: leave it unchecked and say so; widening the role is a reviewed infra change of its own.
- **Record** live runs with `MEASURE_HOST=macos-arm64` (or another generic host label), never the machine's hostname.

## Act

- **actions_unpinned**: pin each `uses:` to the full commit SHA with the tag in a trailing comment (`@<sha> # v4`), and let Dependabot keep the pins fresh. This work belongs to the supply-chain issue (title starting "Supply chain: pin GitHub Actions by SHA"): comment the new count there instead of filing.
- **workflows_without_permissions, workflow_write_scopes_broad**: give the workflow `permissions: contents: read` at the top and grant each write scope on the job that uses it. The broad count is a heuristic: a write scope at workflow level in a multi-job workflow where some job inherits it. Check that job; if it truly needs the scope, move the grant onto it anyway.
- **Template checks** (budget 0): a non-zero count means the change in front of you weakened a template. Fix the template in the same change; never skip the rule.
- **checkov_skipped**: every skip carries a `comment` saying why. A rising count needs a reason in the PR.
- **Live headers, TLS, buckets**: the template is the source of truth. A live value that disagrees with a passing template is drift: compare with the deployed stack (`task prod:status`) before changing anything.

## Write findings neutrally

Everything filed or committed is world-readable, so findings describe the fix to make, never how to exploit what is missing.

- **Title** by the measure convention: `[measure] infra-security: <metric> = <value> (<limit>)`.
- **Body**: the control that is missing or weak, the file and resource, the change that fixes it, and how to verify. Phrase it as hardening ("add a Content-Security-Policy to the response headers policy"), not as an attack path.
- **Leave out**: account IDs, ARNs (they embed the account), distribution, policy or zone IDs, role session details, hostnames, IP addresses, raw AWS error text, and commands that would exploit the gap. Redact tool output before quoting it; the probe's details already omit these.
- Labels: `area:quality`, `measure:finding`, `loop:ready`.

## Prove it

The fixing PR re-runs the probe (live too, after deploy, for a live metric), quotes the **improved** verdict, and records the new value with `--record`.
