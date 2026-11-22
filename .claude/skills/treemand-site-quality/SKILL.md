---
name: treemand-site-quality
description: Measure treemand.dev's Lighthouse scores, axe-core accessibility violations, layout shift (CLS) and page weight for the home, installation and try pages at mobile and desktop sizes, and turn findings into fixes. Use for a Lighthouse or accessibility question, a slow or heavy page, layout jumps, after a site change, or site-quality findings.
---

# Site quality

Follow **treemand-measure** for running, recording and filing; this skill is the site-quality playbook. For crawl-level problems (dead links, console errors, HTTP errors) use **treemand-site-runtime-health** instead.

## Measure

- Run `. ~/.nvm/nvm.sh && nvm use` first: the driver needs Node 22+ and Chrome (`CHROME` overrides the binary).
- **Production**: `SITE_BASE=https://treemand.dev/ task site:quality` (same as `task measure -- site-quality`). This is the number readers get; record it after a deploy.
- **Local**: `task site:quality` builds the site as `task site:audit` does and serves it on a free port (`SITE_DIR=<built dir>` serves an existing build). Use it to prove a fix before it ships. The local server sends no compression, so local `weight_bytes` are larger than production's; each target has its own history.
- About 5 minutes: each page and form factor gets one Chrome load (weight, CLS, axe) and then 3 Lighthouse runs (`SITE_QUALITY_RUNS`). `SITE_QUALITY_PAGES="try=/try/"` narrows it while iterating; `SITE_QUALITY_LIGHTHOUSE=off` skips Lighthouse.
- Lighthouse (`npx --yes lighthouse@13.6.0`) and axe-core (4.14.0, checked against a pinned SHA-256 and cached under `~/.cache/treemand/`) are pinned in `scripts/metrics/site_quality.mjs` and fetched at run time. They are measurement tools, never site dependencies. To upgrade, pick a release at least two weeks old, update the pin (and the axe hash), and record a fresh baseline in the same PR: a new Lighthouse moves scores by itself.

Every metric's subject is `<target>.<page>.<form factor>`, for example `lh_accessibility.production.try.mobile`.

| metric | measures | rule |
| --- | --- | --- |
| `lh_performance` | Lighthouse performance score, median of the runs | ratchet up, `min_delta` 8, same host, timing (not judged on a busy machine) |
| `lh_accessibility`, `lh_best_practices`, `lh_seo` | Lighthouse category scores | budget at least 95; ratchet up |
| `axe_critical`, `axe_serious` | axe-core rules violated, by impact | budget 0; ratchet down |
| `axe_moderate`, `axe_minor` | the same, lower impact | ratchet down |
| `cls` | largest layout-shift session window (Core Web Vitals) during load | budget 0.1; ratchet down, `min_delta` 0.02 |
| `weight_bytes` | bytes transferred for a cold load, after compression | per-page budget (current plus margin, in `rules.json`); ratchet down, 10% tolerance |

Lighthouse performance carries no budget: it is noisy and host-bound, so it only stops backsliding. An honest drop of more than 8 points on one host is a regression worth a look; a single low run is not.

## Read the findings

`task site:quality -- --findings` prints the evidence. `details[0]` names the target and the tool versions. Every later entry is one page and form factor:

- `axe`: each violated rule with `id`, `impact`, `help`, the number of `nodes` and up to three CSS `targets`.
- `lighthouse.scores` and `performance_runs` (each run's score, so you can see the noise), `lab` (FCP, LCP, TBT, speed index, Lighthouse's own CLS and byte weight) and `failing`: per category, the audits that cost points (`id`, `score`, `title`).
- `cls` and `cls_shifts`: each shift in the worst window with its time and the element that moved most.
- `weight_bytes`, `requests` and `largest`: the five biggest transfers.

Confirm before acting:

- **Lighthouse versus axe**: Lighthouse's accessibility category is built on axe but scores only some rules and weights them, so the two disagree in detail. A rule both name is certain; one only axe names is still a real violation.
- **Performance**: look at `performance_runs` and `lab` before blaming code. A spread of 10 points between runs is noise; a consistent drop with a larger LCP or TBT is real.
- **Local versus production**: weight and performance differ by design (compression, CDN). A score or violation on one target only points at a build or deploy difference.

## Turn a finding into an issue

File per **treemand-measure** (title `[measure] site-quality: <metric>.<subject> = <value> (<limit>)`, labels `area:site`, `measure:finding`, `loop:ready`) after deduplicating with `gh issue list --search "in:title [measure] site-quality" --state open`. Group by cause, not by page or form factor: one axe rule failing on a shared template is one issue, titled after its worst subject, with every affected subject in the body. Quote the `details` entries.

## Fix and prove

- **axe or Lighthouse accessibility**: the rule `id` names the fix (`color-contrast`: raise the contrast in `www/treemand/assets/` CSS; `button-name`, `link-name`, `image-alt`: give the element a name; `aria-*`: use the ARIA role and attributes the element's behaviour actually has). Add a deterministic check to `scripts/site_audit.mjs` when the rule is one it can enforce on every page.
- **Best practices or SEO**: the `failing` audit ids name the gap (missing meta, console errors, image aspect ratios, deprecated APIs).
- **cls**: reserve space for content that appears late (fixed dimensions, `min-height`, setting the initial state before first paint) for the element `cls_shifts` names.
- **weight_bytes**: start from `largest`. Compress or resize images, load heavy media only where it is seen, and keep the Try page's wasm behind its own page.
- **lh_performance**: read `lab` and the performance `failing` audits (render-blocking resources, LCP element, unused JavaScript).

Prove it: re-run on the local target before merging, and on production after the release deploys; the verdict should read **improved**, and record it with `--record`. `task site:quality:test` covers the probe itself against a fixture page with known violations, a layout shift and a known weight, using a stand-in Lighthouse; extend the fixture when you change what the probe counts.
