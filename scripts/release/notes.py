#!/usr/bin/env python3
"""Release notes and the changelog entry from the pull requests in a release.

  python3 -I scripts/release/notes.py --tag vX.Y.Z [--since TAG] [--until REF]
      [--date ISO-8601] [--body FILE] [--changelog-dir DIR]

The release is every pull request merged on main's first-parent history from
--since (default: the latest v* tag, as plan.sh reads it) to --until (default:
HEAD). Each PR's "## Release note" section (skill treemand-release-notes) is
the text; its labels pick the group.

--body writes the GitHub release body: Features, Changes and Fixes (each note
with its PR number), Under the hood (PR titles of internal changes) and a
compare link. --changelog-dir writes <DIR>/<tag>.md, the treemand.dev
changelog entry (skill treemand-changelog): front matter with the date, then
Added / Changed / Fixed, or a one-line maintenance release. Deterministic: the
same history and --date give the same files.

Needs git with full history and tags, and gh (REPO=owner/name, default: the
current repository).
"""

import argparse
import json
import os
import re
import subprocess
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path

# The changelog groups, in page order, and the release body heading for each.
GROUPS = {"Added": "Features", "Changed": "Changes", "Fixed": "Fixes"}
DEFAULT_GROUP = {"release:minor": "Added", "release:patch": "Fixed"}
HEADING = re.compile(r"^##\s+release note\s*$", re.IGNORECASE)
PREFIX = re.compile(r"^(Added|Changed|Fixed):\s*", re.IGNORECASE)
COMMENT = re.compile(r"<!--.*?(-->|$)", re.DOTALL)
MAINTENANCE = "Maintenance release: no user-visible changes."


@dataclass(frozen=True)
class Pull:
    number: int
    title: str
    labels: frozenset[str]
    note: str | None  # None: the body has no note; "none": an internal change


@dataclass(frozen=True)
class Entry:
    group: str | None  # a GROUPS key, or None for under the hood
    text: str
    number: int


def release_note(body: str) -> str | None:
    """Read the PR body's release note exactly as check_note.sh does."""
    lines: list[str] = []
    in_section = False
    for line in body.replace("\r", "").split("\n"):
        if re.match(r"^##[ \t]", line):
            in_section = bool(HEADING.match(line))
            continue
        if in_section:
            lines.append(line)
    # The note is the section's first paragraph once comments are removed.
    paragraphs = re.split(r"\n[ \t]*\n", COMMENT.sub("", "\n".join(lines)))
    text = next((" ".join(p.split()) for p in paragraphs if p.strip()), "")
    if not text:
        return None
    if text.lower().replace("`", "").replace(".", "") == "none":
        return "none"
    return text


def classify(pull: Pull) -> Entry:
    """Where a PR lands: a user-facing group with its note, or under the hood."""
    release = next((DEFAULT_GROUP[label] for label in sorted(pull.labels) if label in DEFAULT_GROUP), None)
    if pull.note == "none" or (pull.note is None and release is None):
        return Entry(None, pull.title, pull.number)
    # A releasing PR without a note falls back to its title (an older PR).
    text = pull.note if pull.note is not None else pull.title
    match = PREFIX.match(text)
    if match:
        return Entry(match.group(1).capitalize(), text[match.end() :], pull.number)
    # A user-visible change that does not release by itself (a site page).
    return Entry(release or "Changed", text, pull.number)


def run(*args: str) -> str:
    return subprocess.run(args, check=True, capture_output=True, text=True).stdout


def latest_tag() -> str | None:
    tags = run("git", "tag", "--list", "v[0-9]*", "--sort=-v:refname").split()
    return tags[0] if tags else None


def merged_pulls(repo: str, since: str | None, until: str) -> list[Pull]:
    """List the PRs merged on first-parent history in since..until, oldest first."""
    shas = run("git", "rev-list", "--first-parent", "--reverse", f"{since}..{until}" if since else until).split()
    pulls: list[Pull] = []
    seen: set[int] = set()
    for sha in shas:
        for raw in json.loads(run("gh", "api", f"repos/{repo}/commits/{sha}/pulls")):
            if raw.get("merged_at") is None or raw["number"] in seen:
                continue
            seen.add(raw["number"])
            labels = frozenset(label["name"] for label in raw.get("labels", []))
            pulls.append(Pull(raw["number"], raw["title"], labels, release_note(raw.get("body") or "")))
    return pulls


def release_body(entries: list[Entry], repo: str, since: str | None, tag: str) -> str:
    sections: list[str] = []
    for group, heading in GROUPS.items():
        items = [f"- {e.text} (#{e.number})" for e in entries if e.group == group]
        if items:
            sections.append(f"## {heading}\n\n" + "\n".join(items))
    internal = [f"- {e.text} (#{e.number})" for e in entries if e.group is None]
    if internal:
        sections.append("## Under the hood\n\n" + "\n".join(internal))
    if since:
        sections.append(f"**Full changelog**: https://github.com/{repo}/compare/{since}...{tag}")
    return "\n\n".join(sections) + "\n"


def changelog_entry(entries: list[Entry], tag: str, date: str) -> str:
    groups = [
        f"### {group}\n\n" + "\n".join(f"- {e.text}" for e in entries if e.group == group)
        for group in GROUPS
        if any(e.group == group for e in entries)
    ]
    body = "\n\n".join(groups) if groups else MAINTENANCE
    return f'---\ntitle: "{tag}"\ndate: {date}\n---\n\n{body}\n'


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--tag", required=True, help="the version being released, vX.Y.Z")
    parser.add_argument("--since", help="the previous tag (default: the latest v* tag)")
    parser.add_argument("--until", default="HEAD", help="the last commit in the release (default: HEAD)")
    parser.add_argument("--date", help="the release time, ISO 8601 UTC (default: now)")
    parser.add_argument("--body", type=Path, help="write the GitHub release body here")
    parser.add_argument("--changelog-dir", type=Path, help="write <DIR>/<tag>.md, the changelog entry")
    args = parser.parse_args()

    repo = os.environ.get("REPO") or run("gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
    repo = repo.strip()
    since: str | None = args.since or latest_tag()
    date: str = args.date or datetime.now(UTC).strftime("%Y-%m-%dT%H:%M:%SZ")
    entries = [classify(pull) for pull in merged_pulls(repo, since, args.until)]

    if args.body:
        args.body.write_text(release_body(entries, repo, since, args.tag))
    if args.changelog_dir:
        args.changelog_dir.mkdir(parents=True, exist_ok=True)
        (args.changelog_dir / f"{args.tag}.md").write_text(changelog_entry(entries, args.tag, date))


if __name__ == "__main__":
    main()
