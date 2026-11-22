"""cfn-lint rule: every taggable resource carries Project=treemand.

Taggability comes from the CloudFormation resource schema bundled with
cfn-lint (its "tagging" block), so new resource types are covered without
maintaining an allowlist here.
"""

from typing import ClassVar

from cfnlint.rules import CloudFormationLintRule, RuleMatch
from cfnlint.schema import PROVIDER_SCHEMA_MANAGER
from cfnlint.template import Template

PROJECT_KEY = "Project"
PROJECT_VALUE = "treemand"


def _tag_value(tags: object) -> object:
    """Return the Project tag's value from a list-of-Key/Value or map tag shape."""
    if isinstance(tags, dict):
        return tags.get(PROJECT_KEY)
    if isinstance(tags, list):
        for tag in tags:
            if isinstance(tag, dict) and tag.get("Key") == PROJECT_KEY:
                return tag.get("Value")
    return None


class ProjectTag(CloudFormationLintRule):  # type: ignore[misc]  # cfn-lint ships no types, so its base is Any
    id = "E9001"
    shortdesc = "Taggable resources must carry Project=treemand"
    description = (
        "Cost Explorer attributes spend by the Project tag; a taggable resource without it is invisible there."
    )
    source_url = "https://github.com/aallbrig/treemand/issues/115"
    tags: ClassVar[list[str]] = ["resources", "tags", "cost"]

    def match(self, cfn: Template) -> list[RuleMatch]:
        matches: list[RuleMatch] = []
        region = cfn.regions[0] if cfn.regions else "us-east-1"
        for name, resource in cfn.template.get("Resources", {}).items():
            rtype = resource.get("Type", "")
            if not rtype.startswith("AWS::"):
                continue
            try:
                schema = PROVIDER_SCHEMA_MANAGER.get_resource_schema(region, rtype).schema
            except Exception:  # noqa: BLE001, S112 - unknown types are E3006's job
                continue
            tagging = schema.get("tagging", {})
            if not tagging.get("taggable"):
                continue
            prop = (tagging.get("tagProperty") or "/properties/Tags").rsplit("/", 1)[-1]
            tags = (resource.get("Properties") or {}).get(prop)
            if _tag_value(tags) != PROJECT_VALUE:
                matches.append(
                    RuleMatch(
                        ["Resources", name, "Properties"],
                        f"{name} ({rtype}) is taggable but lacks {prop} {PROJECT_KEY}={PROJECT_VALUE}",
                    )
                )
        return matches
