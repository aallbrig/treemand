---
name: treemand-adoption-signals
description: Measure whether people are finding and installing treemand (stars, forks, watchers, release downloads, GitHub traffic and referrers, Homebrew tap clones, Go module proxy version, issues from outside the project) from free sources, and read the trends without over-reading small numbers. Use for adoption, growth or download questions, deciding what to prioritise from usage, or an adoption-signals finding.
---

# Adoption signals

Follow **treemand-measure** for running and recording. This probe is different from the others: it measures **trends, not quality**. It has no budgets and no ratchets, and a falling number is never filed as a regression. The only rule is one alert for a broken channel.

## Measure

- `task adoption:signals` (the same as `task measure -- adoption-signals`). Add `-- --findings` for the details, and `-- --record` to append history. It takes a few seconds and reads only free sources: the GitHub API through `gh`, the Go module proxy and formulae.brew.sh.
- Record **once a week** from a maintainer's `gh` login, with `MEASURE_HOST=github-api` so that the history does not carry a machine name. GitHub keeps traffic for only 14 days. A week with no record leaves a gap that cannot be filled later.
- The weekly `Measure (weekly)` workflow runs the probe with the workflow token. That token cannot read traffic, so views, clones, referrers and tap clones are **skipped** there. Use its output for the alert, and use a maintainer run for the history.
- `metrics:downloads` (treemand.dev CloudFront logs through Athena) is the only source for install-script users. It costs money per query. Run it only when the maintainer asks.

| metric | source | notes |
| --- | --- | --- |
| `stars`, `forks`, `watchers` | repository | watchers are subscribers, not stargazers |
| `downloads_total`, `downloads_latest` | release assets | archives only. Checksums and bundles are left out, because one install fetches all three |
| `latest_release_age_days` | latest release | context for `downloads_latest` |
| `latest_zero_downloads_7d` | latest release | **the one rule**: budget 0 |
| `views_14d`, `clones_14d` and their `_uniques_14d` | traffic API | needs push access. Rolling 14 days |
| `referrers_14d` | traffic API | the sites that sent visitors are in details, not people |
| `tap_clones_14d`, `tap_clones_uniques_14d` | homebrew-tap traffic | the closest free signal of Homebrew installs and updates |
| `brew_installs_30d` | formulae.brew.sh | homebrew/core only. A tap formula is skipped with that reason |
| `proxy_latest_is_release` | proxy.golang.org `@latest` | the version the proxy actually saw is in details |
| `external_open_issues` | open issues | the author is not the owner, a member, a collaborator or a bot. A count only |

`details` holds the latest release's downloads per asset and the week-over-week delta of every metric. The delta compares this run with the newest record in `docs/metrics/adoption-signals.jsonl` that is at least 6.5 days old. When no record is that old, details say so.

## Read the trends

- **Know where each install shows up.** Homebrew and manual downloads fetch from GitHub releases, so they appear in `downloads_*`. The install script fetches from the treemand.dev mirror, so it does **not** appear there. `go install` from a clone appears nowhere. A flat download count can mean that people switched channels.
- **Subtract the floor before reading growth.** treemand's own tooling downloads releases and clones the repository: release-health hashes archives, install-experience installs through Homebrew, and CI, the dev loop and Dependabot clone on every run. So `clones_14d` in the thousands is mostly automation, and every release starts with a few downloads. Look at uniques and at the change from week to week, not at levels.
- **Downloads are cumulative and per release.** `downloads_total` only ever rises. Its weekly delta is the signal. A release cut on the same day as the probe has had no time to be downloaded, so compare releases of similar age.
- **Read the referrers for priorities.** A new referrer site shows where people found treemand. Check what that page says, and whether the install page answers the question that brought those visitors.
- **Issues from outside are a strong signal at any size.** One new external issue means someone used treemand enough to hit a problem. Answer it before anything else.

## What not to conclude

- **Do not treat small numbers as trends.** When counts are in single digits, a change of 1 to 5 is noise: one person, one bot, one CI run. Ask for at least three consecutive weekly records moving the same way before calling it a trend, and say "too small to tell" when it is.
- **Do not read a percentage change on a small base.** Going from 2 to 4 stars is "+2", not "doubled".
- **Do not prioritise work from adoption numbers alone.** They show reach, not quality. Use them to choose between equally sound options, such as which channel to document first, and never to drop a fix.
- **Do not file falling numbers.** A quiet week is not a bug. Only the alert below is filed.
- **Do not chase `proxy_latest_is_release` = 0 on its own.** The module lives in `cli/treemand` with tags at the repository root, so the proxy's `@latest` for the root path is an old version from before that move. The documented Go path (clone, then `go install .`) does not use the proxy. This becomes worth an issue only if `go install github.com/aallbrig/treemand@latest` is documented.
- **Do not put names in issues or history.** Record aggregate counts only. Never list stargazers, forkers or issue authors.

## Act on the alert

`latest_zero_downloads_7d` = 1 means that no archive of the latest release was downloaded in the 7 days after it was published. Because our own tooling downloads every release, a true 0 almost always means that the channel is broken, not that nobody wanted the release.

1. Run `task release:health` and `sh scripts/channels_check.sh`. They show whether the assets exist, whether the Homebrew formula points at the release, and whether the mirror carries it.
2. File the issue as **treemand-measure** describes, as a `[measure] adoption-signals: latest_zero_downloads_7d = 1 (0)` issue with `area:packaging`, `measure:finding` and `loop:ready`. Include the release-health and channels output.
3. The fix is proven when a later run shows downloads for the latest release and the alert reads 0.
