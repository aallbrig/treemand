---
name: treemand-release-notes
description: Write the "## Release note" line every treemand pull request carries (one user-facing sentence, or none). Use when writing or reviewing a PR body, or when the PR Body check fails.
---

# Writing a PR release note

Every PR body has a `## Release note` section (`.github/pull_request_template.md`).
Auto Release copies it verbatim into the GitHub release notes and the
treemand.dev changelog, with no editing in between, so the line you write is
what users read. `sh scripts/release/check_note.sh < body.md` checks it; the
**PR Body** workflow runs the same check on every PR and re-runs on each edit.

1. **Decide the audience.** Would someone who installs treemand, or reads
   treemand.dev, notice this change: different output, a new flag or key, a
   fixed bug, a faster run, a new install path, a changed page? Done when you
   can name what that person sees differently, or know they see nothing.
2. **Nothing to see: write `none`.** Tests, CI, refactors, measurement probes,
   skills, loop machinery and developer docs are `none`. They still appear in
   the GitHub release under *Under the hood*, from the PR title.
3. **Something to see: write one sentence** about the change, from the user's
   side of the screen:
   - Name the thing they touch: the command, flag, key, output format or page.
   - Say what it does now, in plain present or past tense. "Added", "Fixed",
     "now" are good openers.
   - One sentence ending with a full stop; no PR or issue numbers, handles,
     file paths, function or package names.
   - Professional and specific: the facts, without hype or emojis.
4. **Optionally name the changelog group.** The label decides it by default:
   `release:minor` lands under *Added*, `release:patch` under *Fixed*. Start the
   note with `Added:`, `Changed:` or `Fixed:` when the default is wrong, such as a
   patch that changes a default (`Changed:`). The prefix is dropped from the text.

Done when the section holds exactly one sentence that passes step 3, or `none`,
and `check_note.sh` exits 0 on the body.

## Examples

| Change | Good | Bad, and why |
| --- | --- | --- |
| New TUI key | `Press y in the TUI to copy the built command to the clipboard.` | `Add yank keybinding to TreeModel.Update` (code names, not what a user does) |
| Parser fix | `Fixed missing subcommands for CLIs whose help lists them under "Available Commands:".` | `Fix #212` (says nothing without clicking through) |
| Behaviour change | `Changed: --depth now defaults to 3 so huge CLIs print quickly.` | `Changed depth default` (no value, no reason) |
| Site page | `The installation page shows the steps for your operating system first.` | `Improved the website!!! 🚀` (vague, hype) |
| Speed-up | `treemand starts about twice as fast on CLIs with hundreds of subcommands.` | `Perf work in discovery cache` (an internal area, not an effect) |
| Refactor | `none` | `Split TreeModel.Left into helpers` (users see no change) |
| CI or tests | `none` | `Made CI greener` |
