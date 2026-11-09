# Select: choose the cycle's work

Pick the first that applies:

1. **FIX_MAIN** from preflight.
2. Oldest open issue labelled `loop:ready` that is not `human:wip` or
   `loop:blocked`. Human-filed issues rank above loop-filed ones.
3. Oldest `loop:proposed` issue a human reacted to or commented approval on
   (`gh issue view --comments`). Do not self-promote your own proposals.
4. **Idle audit**: nothing queued, so create work.

## Idle audit

1. Read the newest `docs/design/*_vision.md` and `docs/FEATURES.md`.
2. `area=$(python3 -I scripts/loop/state.py pick-area)` - least recently touched
   focus area. Areas: tui-cli-ux, discovery-quality, website-design, docs-try-page,
   performance-reliability, test-quality-security, packaging-distribution,
   mcp-api-exploration.
3. Audit that area against reality, not just the code:
   - tui-cli-ux / website-design: `task visual:check`, read the evidence, score it
     against the design rubric.
   - discovery-quality: run `treemand --output=json <cli>` on a spread of real CLIs
     installed here (git, docker, kubectl, gh, go, cargo, npm, ...) and look for stub
     nodes, missing flags, wrong descriptions.
   - performance-reliability: time cold and cached runs, run `task test-race`.
   - test-quality-security: coverage gaps, `task vuln`, flaky tests.
   - packaging-distribution: compare `packaging/` versions to the latest release.
   - docs-try-page: walk the docs and Try page as a new user.
   - mcp-api-exploration: **propose only** (see below).
4. Hypothesize 1-3 concrete improvements; pick the highest value-for-effort that
   fits in one small release. File it as an issue (`area:*` label, acceptance
   criteria) with `loop:ready` - except mcp-api-exploration and anything that
   changes the vision, which get `loop:proposed` and the cycle ends `idle`.
5. Nothing worth doing in this area? Say so in LEARNINGS, `touch-area` it, try the
   next least-recent area once, and if still nothing, `end-cycle idle`. Do not
   invent marginal work.

Duplicate check before filing: `gh issue list --search`.
