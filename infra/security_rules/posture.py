"""cfn-lint rules for the infra-security probe (scripts/metrics/infra_security.py).

They run only in the probe, never in `task infra:lint`: the probe counts their
matches and the measurement engine judges the counts (budget 0, ratchet), so a
regression fails the gate while the evidence stays in the probe's details.
"""

from typing import Any, ClassVar

from cfnlint.rules import CloudFormationLintRule, RuleMatch
from cfnlint.template import Template

SOURCE = "https://github.com/aallbrig/treemand/issues/264"

# Viewer TLS policies at or above TLSv1.2_2021 (TLS 1.2+ with forward-secret
# AEAD ciphers only). Anything else, or no policy at all, allows weaker suites.
STRONG_TLS = frozenset({"TLSv1.2_2021", "TLSv1.2_2025", "TLSv1.3_2025"})
# AWS-managed "SecurityHeadersPolicy": sets HSTS, nosniff and frame options.
MANAGED_SECURITY_HEADERS = "67f7725c-6f97-4210-82d7-5512b31e9d03"
REQUIRED_HEADERS = ("StrictTransportSecurity", "ContentTypeOptions", "FrameOptions")
PUBLIC_ACCESS_FLAGS = ("BlockPublicAcls", "BlockPublicPolicy", "IgnorePublicAcls", "RestrictPublicBuckets")
IAM_POLICY_HOLDERS = frozenset(
    {"AWS::IAM::Role", "AWS::IAM::User", "AWS::IAM::Group", "AWS::IAM::Policy", "AWS::IAM::ManagedPolicy"}
)


def _resources(cfn: Template, rtype: str) -> list[tuple[str, dict[str, Any]]]:
    return [
        (name, r.get("Properties") or {})
        for name, r in cfn.template.get("Resources", {}).items()
        if isinstance(r, dict) and r.get("Type") == rtype
    ]


def _as_list(value: object) -> list[Any]:
    return value if isinstance(value, list) else [value]


class S3PublicAccessBlock(CloudFormationLintRule):  # type: ignore[misc]  # cfn-lint ships no types
    id = "W9101"
    shortdesc = "S3 buckets block every kind of public access"
    description = "Each AWS::S3::Bucket sets all four PublicAccessBlockConfiguration flags to true."
    source_url = SOURCE
    tags: ClassVar[list[str]] = ["security", "s3"]

    def match(self, cfn: Template) -> list[RuleMatch]:
        matches: list[RuleMatch] = []
        for name, props in _resources(cfn, "AWS::S3::Bucket"):
            pab = props.get("PublicAccessBlockConfiguration") or {}
            if not all(pab.get(flag) in (True, "true") for flag in PUBLIC_ACCESS_FLAGS):
                matches.append(RuleMatch(["Resources", name], f"{name} lacks a full PublicAccessBlockConfiguration"))
        return matches


def _policy_sets_headers(cfn: Template, policy: object) -> bool:
    if policy == MANAGED_SECURITY_HEADERS:
        return True
    if not (isinstance(policy, dict) and "Ref" in policy):
        return False
    resource = cfn.template.get("Resources", {}).get(policy["Ref"]) or {}
    config = ((resource.get("Properties") or {}).get("ResponseHeadersPolicyConfig") or {}).get(
        "SecurityHeadersConfig"
    ) or {}
    return all(config.get(h) for h in REQUIRED_HEADERS)


class CloudFrontSecurityHeaders(CloudFormationLintRule):  # type: ignore[misc]
    id = "W9102"
    shortdesc = "CloudFront behaviours send HSTS, nosniff and frame options"
    description = (
        "Every cache behaviour attaches a ResponseHeadersPolicy whose SecurityHeadersConfig sets "
        "StrictTransportSecurity, ContentTypeOptions and FrameOptions."
    )
    source_url = SOURCE
    tags: ClassVar[list[str]] = ["security", "cloudfront"]

    def match(self, cfn: Template) -> list[RuleMatch]:
        matches: list[RuleMatch] = []
        for name, props in _resources(cfn, "AWS::CloudFront::Distribution"):
            config = props.get("DistributionConfig") or {}
            behaviours = [("DefaultCacheBehavior", config.get("DefaultCacheBehavior") or {})]
            behaviours += [
                (f"CacheBehaviors/{b.get('PathPattern', i)}", b)
                for i, b in enumerate(_as_list(config.get("CacheBehaviors") or []))
                if isinstance(b, dict)
            ]
            for label, behaviour in behaviours:
                if not _policy_sets_headers(cfn, behaviour.get("ResponseHeadersPolicyId")):
                    matches.append(
                        RuleMatch(["Resources", name], f"{name} {label} lacks HSTS, nosniff or frame options")
                    )
        return matches


class CloudFrontTlsMinimum(CloudFormationLintRule):  # type: ignore[misc]
    id = "W9103"
    shortdesc = "CloudFront viewer TLS minimum is TLSv1.2_2021 or newer"
    description = "ViewerCertificate.MinimumProtocolVersion is TLSv1.2_2021 or a newer policy."
    source_url = SOURCE
    tags: ClassVar[list[str]] = ["security", "cloudfront", "tls"]

    def match(self, cfn: Template) -> list[RuleMatch]:
        matches: list[RuleMatch] = []
        for name, props in _resources(cfn, "AWS::CloudFront::Distribution"):
            cert = (props.get("DistributionConfig") or {}).get("ViewerCertificate") or {}
            version = cert.get("MinimumProtocolVersion", "TLSv1 (default)")
            if version not in STRONG_TLS:
                matches.append(RuleMatch(["Resources", name], f"{name} allows viewer TLS below TLSv1.2_2021"))
        return matches


def _boundaries(cfn: Template) -> set[str]:
    """Names of managed policies used as permissions boundaries: ceilings, not grants."""
    names: set[str] = set()
    for r in cfn.template.get("Resources", {}).values():
        boundary = ((r or {}).get("Properties") or {}).get("PermissionsBoundary")
        if isinstance(boundary, dict) and "Ref" in boundary:
            names.add(boundary["Ref"])
    return names


def _statements(rtype: str, props: dict[str, Any]) -> list[Any]:
    if rtype in ("AWS::IAM::Policy", "AWS::IAM::ManagedPolicy"):
        docs = [props.get("PolicyDocument") or {}]
    else:
        docs = [(p or {}).get("PolicyDocument") or {} for p in _as_list(props.get("Policies") or [])]
    return [s for d in docs if isinstance(d, dict) for s in _as_list(d.get("Statement") or [])]


class IamFullServiceWildcard(CloudFormationLintRule):  # type: ignore[misc]
    id = "W9104"
    shortdesc = "IAM grants name actions, not whole services"
    description = "No Allow statement in an identity policy grants '*' or '<service>:*'."
    source_url = SOURCE
    tags: ClassVar[list[str]] = ["security", "iam"]

    def match(self, cfn: Template) -> list[RuleMatch]:
        matches: list[RuleMatch] = []
        boundaries = _boundaries(cfn)
        for name, r in cfn.template.get("Resources", {}).items():
            rtype = (r or {}).get("Type")
            if rtype not in IAM_POLICY_HOLDERS or name in boundaries:
                continue
            for statement in _statements(rtype, (r or {}).get("Properties") or {}):
                if not isinstance(statement, dict) or statement.get("Effect") != "Allow":
                    continue
                matches.extend(
                    RuleMatch(["Resources", name], f"{name} allows every action of {action}")
                    for action in _as_list(statement.get("Action") or [])
                    if isinstance(action, str) and (action == "*" or action.endswith(":*"))
                )
        return matches
