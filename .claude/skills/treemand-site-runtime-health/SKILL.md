---
name: treemand-site-runtime-health
description: Crawl treemand.dev (local build or production) in headless Chrome for HTTP errors, console errors, JS exceptions, dead links and mixed content, and turn findings into fixes. Use for a broken page or link report, after a site or deploy change, or site-health findings.
---

# Site runtime health

Follow **treemand-measure** for running, recording and filing; this skill is the site-health playbook.

## Measure

- **Local**: `task measure -- site-health` (or `task site:health`) builds the site as `task site:audit` does, serves it on a free port and crawls it. `SITE_DIR=<built dir>` serves an existing build instead.
- **Production**: `SITE_BASE=https://treemand.dev/ task measure -- site-health`. Run it after a deploy, or when a reader reports a broken page.
- Run `. ~/.nvm/nvm.sh && nvm use` first: the driver needs Node 22+ and Chrome (`CHROME` overrides the binary).

The crawl starts from `/` and every `sitemap.xml` path, follows every same-origin link, and loads each HTML page once in Chrome. Every metric carries the target as its subject (`http_errors.local`, `http_errors.production`), so both targets share `docs/metrics/site-health.jsonl` and a run of one reports the other as skipped.

| metric | counts | rule |
| --- | --- | --- |
| `pages` | HTML pages crawled | ratchet up: a drop is a page that went missing or unlinked |
| `http_errors` | pages and sitemap entries with a status of 400 or more, plus failed subresource requests (4xx, 5xx, network) | budget 0 |
| `console_errors` | `console.error` calls and browser error logs (network failures are counted once, as `http_errors`) | budget 0 |
| `js_exceptions` | uncaught exceptions | budget 0 |
| `broken_internal_links` | same-origin links that fail, or whose `#fragment` names no element on the target page | budget 0 |
| `broken_external_links` | external links that fail (HEAD, then GET; 8s timeout; at most 5 at a time; cached per run) | ratchet only: the outside world flakes |
| `mixed_content` | `http:` subresources on an https page, and cross-origin `http:` resources on any page (they would be mixed content in production) | budget 0 |

## Read the findings

`task measure -- site-health --findings` prints the evidence. `details[0]` describes the run: target, base, the pages crawled, every external link with its final status, `unverifiable_external` (401, 403, 429 or 999: a site refusing automated clients, not counted as broken) and `sitemap_errors`. Every later entry is one page with a problem, naming its status, `console_errors`, `js_exceptions`, `failed_requests`, `mixed_content` and `broken_links` (`href`, `internal`, `status`, `error`).

Before acting, confirm the finding is real:

- **External link**: re-check it by hand (`curl -sIL <url>`). A single timeout or 5xx is the outside world; a 404 or 410 seen twice is a dead link.
- **Local versus production**: a finding on one target only points at the cause. Local only means the build or the local server differs from what is deployed (a path published outside Hugo, such as `releases/`). Production only means a stale deploy, a CDN or S3 object, or a third-party script.
- **Runtime errors**: reproduce them by opening the page with DevTools; the message names the script and line.

## Turn a finding into an issue

File per **treemand-measure** (title `[measure] site-health: <metric>.<target> = <value> (<limit>)`, labels `area:site`, `measure:finding`, `loop:ready`) after deduplicating with `gh issue list --search "in:title [measure] site-health" --state open`. Group findings by cause, not by page: one dead link repeated in a shared template is one issue. Quote the page entries from `details` (redacted per the public-repo standard).

## Fix and prove

- **Broken internal link or anchor**: fix the content or the template that renders it; for a renamed page, add a Hugo `aliases` entry so old links keep working.
- **http_errors**: a missing asset is a build or publish gap (`task site:build`, `scripts/infra/publish_site.sh`); a 404 page in the sitemap is a page that should exist or be dropped from it.
- **console_errors, js_exceptions**: fix the script; give the fix a check in `scripts/site_audit.mjs` when the failure is page behaviour it can drive.
- **mixed_content**: serve the resource over https or from the site itself.
- **broken_external_links**: point the link at the moved page, or remove it.

Prove it: the probe shows **improved** on the target that had the finding (production after the fix is deployed), and record the new value with `--record`. `task site:health:test` covers the probe itself against a fixture with one of each problem; extend the fixture when you change what the probe counts.
