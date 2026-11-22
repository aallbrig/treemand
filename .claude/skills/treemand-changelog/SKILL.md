---
name: treemand-changelog
description: Write or edit a treemand.dev changelog entry (www/treemand/content/changelog/, one file per release). Use when a release's entry reads badly, a past release needs an entry, or someone asks what changed for users in a version.
---

# Editing the treemand.dev changelog

The changelog at treemand.dev/changelog/ is for people who use treemand: what
they can now do, what behaves differently, what was broken and works now. The
GitHub release is the developer's list (every PR, numbers, *Under the hood*);
the changelog is the user's.

## Where entries come from

Auto Release writes `www/treemand/content/changelog/<tag>.md` from the PRs'
`## Release note` lines (`scripts/release/notes.py`, skill
**treemand-release-notes**) and commits it with the tag. Notes pass through
unedited, so an entry can need an editor's pass: a note that leaks an internal
name, two notes about one feature, or an older PR that fell back to its title.
Releases before v0.10 have one hand-written entry per minor line (`v0.9.md`,
with `versions: "v0.9.0 to v0.9.4"`).

## The entry

```markdown
---
title: "v0.10.0"
date: 2026-11-23T08:30:00Z
---

### Added

- Press y in the TUI to copy the built command.

### Changed

- --depth now defaults to 3.

### Fixed

- Fixed aliases missing from the tree.
```

- `date` is the release time in UTC; the page sorts newest first by it.
- Groups appear in the order Added, Changed, Fixed, each only when it has items.
- An entry with nothing user-visible is the single line
  `Maintenance release: no user-visible changes.`

## Editing an entry

1. **Read the release.** `gh release view <tag>` lists every PR; read the
   notes and, where one is unclear, the PR. Done when you can say, for each
   bullet, what a user sees.
2. **Rewrite in user language.** Every bullet answers *what can I do, or what
   works differently, now?*:
   - Name the command, flag, key, output or page the user touches; use
     treemand's terms as the README and site use them, in backticks.
   - One sentence per bullet; merge bullets about the same feature.
   - Move a bullet to the group it belongs in: new capability is *Added*,
     different behaviour of something that existed is *Changed* (say what to do
     if it can break a script), a bug that is gone is *Fixed*.
   - Keep only what users observe; a bullet about tests, CI, refactors,
     tooling or the dev loop goes, and an entry left empty becomes the
     maintenance line.
   - PR and issue numbers, handles and file paths stay on GitHub.
3. **Check it builds.** `cd www/treemand && hugo --quiet -d "$(mktemp -d)"`, then
   look at `/changelog/` with `hugo server`. Done when the entry renders under its
   version heading in date order and every bullet passes step 2.
4. **Ship** as a PR without a release label (Deploy Site publishes `www/` on
   merge) and the release note `none`: a changelog edit is not itself a change
   to list in the next release.
