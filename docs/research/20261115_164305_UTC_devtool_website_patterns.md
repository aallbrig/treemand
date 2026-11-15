# Developer-tool websites: patterns, and how treemand.dev compares

Date: 2026-11-15. Issue: #132. Method: fetched the live pages of 13 reference sites (rendered text
through a fetcher, and the raw HTML with curl for page chrome the fetcher strips), then read the
Hugo source in `www/treemand/` and fetched every top-level page of https://treemand.dev. Claims
about reference sites describe their pages on 2026-11-15 and cite the URL fetched. Claims about
treemand cite a file path or a live URL.

The mobile hamburger menu is being built in another session and is left out of this list.

## 1. Reference sites

| Site | URL fetched | What it shows best |
|---|---|---|
| Bun | https://bun.sh/ | Full landing-page pattern: one-line headline, OS-tabbed install, animated benchmark above the fold, testimonials, comparison table, closing CTA, four-column footer, release callout |
| Deno | https://deno.com/ | OS-tabbed install with copy button, "v2.9" release pill in hero, ⌘K search, stats (100k+ stars) plus a logo wall |
| uv (Astral) | https://docs.astral.sh/uv/ | Docs-as-homepage: tagline, OS-tabbed install, benchmark chart in light and dark variants, nested sidebar, search, on-page TOC, Next link |
| Atuin | https://atuin.sh/ | Install one-liner with Copy button and a `$ get-started` CTA in the hero, terminal mockups as demo media, stats strip, FAQ, closing CTA |
| mise | https://mise.jdx.dev/ | Hero with "Get started" and "Watch the demo", copyable install line, tabbed config example, feature cards that each link to a guide, a 4-step getting-started list, closing CTA, JSON-LD |
| Starship | https://starship.rs/ | Minimal hero with tagline, "Get Started" and `curl` install; 3-card feature grid; search with a keyboard shortcut; theme toggle; 12 languages |
| GoReleaser | https://goreleaser.com/ | Hero with two CTAs, 9-card feature grid, sponsors and "trusted by" logo grid, ⌘K search, Light/Dark/System toggle, "Updates" changelog/blog |
| Charm | https://charm.land/ (charm.sh redirects) | Brand voice and character: playful copy, a mascot per project, card grid, newsletter "Tail the logs.", RSS |
| Zellij | https://zellij.dev/ | "Try Zellij Without Installing" with Copy buttons for each shell and links to read the script first |
| Warp | https://www.warp.dev/ | Live-looking animated hero mockup, logo bar ("Trusted by 800k+ devs"), FAQ accordion, status link in footer |
| fzf | https://junegunn.github.io/fzf/ | Single-author project site: tagline, "Why should I care?" tasks table, demo GIF, docs sidebar, Release Highlights per version, "Last modified" footer |
| lazygit | https://github.com/jesseduffield/lazygit | README as site: tagline, badges, demo video right under them, "Elevator Pitch", one short video per feature |
| Ghostty | https://ghostty.org/ | Fetch returned only the title; not used as evidence |

Docs-chrome counts from raw HTML (`grep -ci` on the fetched page; a count above zero means present):

| Page | Edit link | Prev/next | Copy | Theme | JSON-LD | Skip link | Search |
|---|---|---|---|---|---|---|---|
| mise.jdx.dev/getting-started.html | yes | yes | yes | yes | yes | yes | yes |
| starship.rs/guide/ | yes | yes | yes | yes | no | yes | yes |
| docs.astral.sh/uv/getting-started/installation/ | no | yes | yes | yes | yes | yes | yes |
| goreleaser.com/install/ | no | no | yes | yes | no | yes | yes |
| treemand.dev/getting-started/ | no | no | no (Install page only) | no | no | no | no |

## 2. Common patterns

1. **Hero = one line, one command, one button.** Bun ("Bun is a fast JavaScript runtime & toolkit.
   All in one."), Deno ("Better, faster JavaScript"), GoReleaser ("Release engineering,
   simplified."), Atuin ("Making your terminal magical"). Directly under it: the install one-liner
   with a copy button (Bun, Deno, uv, Atuin, mise, Starship), then a primary CTA ("Get started",
   "Quickstart") and a secondary one ("Read the docs", "Watch the demo", "GitHub").
2. **OS tabs on the install command**, not a single command: Bun and Deno ("macOS & Linux" /
   "Windows"), uv ("macOS and Linux" / "Windows"). Full channel lists live on a separate install page
   (lazygit lists about 20 package managers).
3. **Demo media at or near the top.** Bun's animated benchmark, Atuin's terminal mockups, Warp's
   animated grid, lazygit's video directly under the badges, mise's showreel. Short clips per
   feature (lazygit, Charm's VHS framing) beat one long GIF.
4. **Feature grid of 3 to 9 cards**, each a short heading plus one sentence, often with a snippet
   and a link to the matching doc (Starship 3, mise 4, GoReleaser 9).
5. **Social proof scaled to the project.** Big projects show stars, users and logo walls (Deno,
   Atuin, Warp, GoReleaser). Smaller ones show concrete evidence instead: fzf's "Why should I
   care?" table, lazygit's "Elevator Pitch", Zellij's try-without-installing.
6. **A closing CTA that repeats the install command** (Bun "Try it on the repo you have open right
   now.", mise "Allez. Prep your station.", Atuin "Transform your workflow today").
7. **Docs chrome:** left sidebar grouped into Getting started / Guides / Reference (uv, fzf, mise),
   search with a `/` or ⌘K shortcut (Bun, Deno, GoReleaser, Starship), on-page TOC (uv), prev/next
   (uv, Starship, mise), edit-on-GitHub (Starship, mise), copy button on every code block.
8. **Header:** logo, 4 to 6 links, search, GitHub icon, theme toggle (Starship, GoReleaser), and
   often an install or "Get started" button (Bun, Atuin). None of the fetched pages showed a star
   count in the header.
9. **Release visibility:** Deno's "v2.9" pill, Bun's release callout and footer version link,
   GoReleaser's banner and "Updates" list, fzf's "Release Highlights".
10. **Footer:** columns of Product / Docs / Community / Project links, social icons, RSS (Bun,
    Charm), copyright, sometimes a status link (Warp, Deno).
11. **Machine-readable metadata:** JSON-LD on mise, uv, Atuin and Bun; OpenGraph cards everywhere.

## 3. treemand.dev today

What is already good: real OpenGraph and Twitter card tags with a 1200×630 image
(`themes/minimal/layouts/_default/baseof.html`), canonical URLs, an OS-detecting install picker
with copy buttons and a no-JS fallback (`layouts/shortcodes/install.html`, `static/install.js`,
`data/install.yaml`), a real in-browser TUI at `/try/` (8.3 MB wasm served as 2.2 MB brotli),
pages of about 4 to 26 KB, demo images with width and height set, security headers and HSTS.
Few of the reference sites have a live demo as strong as `/try/`.

### Gaps (with evidence)

**G1. Every `go install` command on the site fails or installs an old version.** The module in
`cli/treemand/go.mod` declares `module github.com/aallbrig/treemand`, but its directory is
`cli/treemand`.
- Home (`content/_index.md`), `data/install.yaml` (all three OSes) and `README.md:28` say
  `go install github.com/aallbrig/treemand/cli/treemand@latest`. Run on 2026-11-15 it fails:
  `module declares its path as: github.com/aallbrig/treemand but was required as:
  github.com/aallbrig/treemand/cli/treemand`.
- The Try page (`themes/minimal/layouts/_default/try.html`) says
  `go install github.com/aallbrig/treemand@latest`. The Go proxy resolves that to **v0.3.1 from
  2026-03-26** (https://proxy.golang.org/github.com/aallbrig/treemand/@latest), while the latest
  release is v0.6.2.
This is a product bug that belongs in its own issue, outside the website work.

**G2. The hero has no install command and no demo.** `content/_index.md` gives an emoji h1,
a vague tagline ("Visualize and interact with any CLI command hierarchy as a beautiful tree."),
three shields.io badges and a "Try it in your browser" link. The 1.6 MB `demo.gif` is the last
thing on the page, under "## Demo". Install shows only Homebrew and a broken `go install`, and
omits the install script that `data/install.yaml` recommends for Linux.

**G3. The home page is a README rendered as Markdown,** not a landing page: an emoji bullet list of
nine features, then a Quick Start code block, then Install, then Demo. No feature cards, no
"why", no closing CTA. `themes/minimal/layouts/index.html` only prints `.Content`.

**G4. No docs chrome.** `single.html` and `list.html` print the content alone: no sidebar, no
on-page TOC (Getting Started has 9 h2s, Installation 8), no prev/next, no edit link, no search, no
copy buttons outside the Installation page, no heading anchor links.

**G5. Weak social proof options.** The repo has 0 stars and 0 forks (GitHub API, 2026-11-15), so a
star counter or "Trusted by" wall would hurt. What treemand can show truthfully: the CLIs it is
tested against (the `/try/` snapshots cover git, docker, kubectl, gh, stripe, aws, treemand) and
signed, checksummed releases.

**G6. No release visibility.** No version on the site, no changelog or release-notes page, no RSS
link (Hugo builds `/index.xml` but `baseof.html` has no `<link rel="alternate">`).

**G7. Thin footer.** `baseof.html` footer is "MIT License · GitHub".

**G8. Header gaps beyond the hamburger.** No GitHub icon link, no "Install" button, no search, no
active-page state (`baseof.html` renders the menu without marking the current page), and "Home"
duplicates the logo link. The logo is an emoji (`🌲 Treemand`) although `static/icon.svg` exists.

**G9. SEO and crawl basics.** `https://treemand.dev/robots.txt` returns 404 and the sitemap is not
referenced anywhere. No JSON-LD (`SoftwareApplication`). `hugo.toml` still has
`baseURL = "/treemand/"` and `origin = "https://aallbrig.github.io"`; production overrides both
in `scripts/infra/publish_site.sh`, so local builds and the GitHub Pages workflow produce
different canonical URLs. The install script URL on the site is still
`https://aallbrig.github.io/treemand/install.sh` (tracked by #123 and #124).

**G10. Accessibility and typography.** No skip link; the h1 on home and Try contains an emoji that
screen readers announce; `font-family: 'Fira Code'` in `main.css` is never loaded, so code falls
back to Consolas or the default monospace; dark theme only, with no `prefers-color-scheme: light`.

**G11. Caching.** Every object, including the wasm and GIFs, is served with
`cache-control: public, max-age=300` (`scripts/infra/publish_site.sh:24`), so a returning visitor
re-validates an 8.3 MB wasm every five minutes. Fingerprinted assets (Hugo Pipes) could be cached
for a year.

**G12. Content drift.** `content/reference.md:45` shows `treemand v0.3.0` as example output; the
Try page and home repeat install steps that already live in `data/install.yaml`.

## 4. Ranked improvements

Score = impact (1 to 5) ÷ effort (S=1, M=2, L=3). Each row is sized to be one issue.

| # | Improvement | Impact | Effort | Exemplar | Files |
|---|---|---|---|---|---|
| 1 | Fix the `go install` path everywhere (product bug: module path vs directory) and add a CI check that runs the documented `go install` | 5 | S | — (correctness) | `cli/treemand/go.mod` or `data/install.yaml`, `content/_index.md`, `try.html`, `README.md` |
| 2 | Hero rewrite: a single-sentence value prop, an OS-tabbed install one-liner with copy button (reuse `data/install.yaml` recommended channel), primary CTA "Try it in your browser" and secondary "Get started", remove emoji from h1 | 5 | S | Bun, Deno, Atuin | `content/_index.md`, `layouts/index.html`, new `hero` partial, `main.css` |
| 3 | Demo above the fold: replace the bottom-of-page 1.6 MB GIF with a short autoplaying muted MP4/WebM (or embed the live `/try/` TUI) next to the hero | 5 | M | lazygit, Bun, Warp | `content/_index.md`, `static/`, `demo.tape` |
| 4 | Landing-page sections: feature cards (4 to 6, each linking to its Commands page), a "Why" section showing the problem (`--help` walls) vs treemand, a "Works with" strip of CLIs, and a closing CTA repeating the install command | 4 | M | mise, Starship, GoReleaser, fzf "Why should I care?" | `layouts/index.html`, new `data/features.yaml`, `main.css` |
| 5 | Docs layout: left sidebar (Get started / Guides / Commands / Reference), on-page TOC, prev/next links, heading anchors, "Edit this page on GitHub" | 4 | M | uv, mise, Starship | `_default/single.html`, `list.html`, `hugo.toml` (`tableOfContents`, menus) |
| 6 | Copy button on every code block (generalise `install.js` copy logic into a site-wide script) | 3 | S | uv, mise, Atuin | `static/install.js` → shared `copy.js`, `baseof.html` |
| 7 | Header additions (besides the hamburger): SVG logo instead of emoji, GitHub icon link, "Install" button, active-page state, drop the redundant "Home" item | 3 | S | Bun, GoReleaser, Atuin | `baseof.html`, `hugo.toml` menu, `main.css` |
| 8 | Release visibility: version pill in hero linking to the latest release, a Changelog page built from GitHub releases at build time, RSS link | 3 | M | Deno, Bun, fzf, GoReleaser | new `content/changelog.md` or data fetch, `baseof.html` |
| 9 | SEO basics: `robots.txt` with sitemap line, `SoftwareApplication` JSON-LD, set `baseURL`/`origin` in `hugo.toml` to treemand.dev so every build matches production | 3 | S | mise, uv, Bun (JSON-LD) | `hugo.toml`, `static/robots.txt` or `enableRobotsTXT`, `baseof.html` |
| 10 | Real footer: columns (Docs, Project: GitHub, Releases, Changelog, Security, License; Community: Issues, Discussions), RSS, version | 2 | S | Bun, GoReleaser | `baseof.html`, `main.css` |
| 11 | Accessibility pass: skip link, focus styles for all links, emoji out of headings, `prefers-reduced-motion` for demo media, run axe/Lighthouse in CI (extends #106) | 3 | M | Starship, mise, uv (skip links) | `baseof.html`, `main.css`, CI |
| 12 | Long-lived caching: fingerprint CSS/JS through Hugo Pipes and upload `try/*.wasm`, GIFs and fingerprinted assets with `max-age=31536000, immutable`; keep HTML at 300 s | 2 | S | — (performance) | `scripts/infra/publish_site.sh`, `baseof.html` |
| 13 | Site search (Pagefind runs on static Hugo output, no server) with a `/` shortcut | 2 | M | Bun, Deno, GoReleaser, Starship | `baseof.html`, build step |
| 14 | Self-host a monospace font (or drop the `Fira Code` reference) and add a light theme behind `prefers-color-scheme` with a toggle | 2 | M | Starship, GoReleaser (theme toggle) | `main.css`, `baseof.html` |
| 15 | Brand voice and assets: a consistent tagline across README, `hugo.toml` description and social card, and a small brand/press page with the SVG icon | 1 | S | Charm, Atuin (brand assets) | `hugo.toml`, `README.md`, `assets-src/` |

Not recommended now: GitHub star counter or "Trusted by" logos (0 stars today; they would read as
empty); newsletter or Discord (no community channel to staff); i18n.

## Sources

- Reference pages fetched 2026-11-15: https://bun.sh/, https://deno.com/,
  https://docs.astral.sh/uv/, https://docs.astral.sh/uv/getting-started/installation/,
  https://atuin.sh/, https://mise.jdx.dev/, https://mise.jdx.dev/getting-started.html,
  https://starship.rs/, https://starship.rs/guide/, https://goreleaser.com/,
  https://goreleaser.com/install/, https://charm.land/, https://zellij.dev/,
  https://zellij.dev/documentation/, https://www.warp.dev/, https://junegunn.github.io/fzf/,
  https://github.com/jesseduffield/lazygit, https://ghostty.org/ (no usable content).
- treemand live pages: https://treemand.dev/, /try/, /installation/, /getting-started/,
  /reference/, /commands/, /robots.txt (404), /sitemap.xml, /index.xml, /try/treemand.wasm.
- Go module proxy: https://proxy.golang.org/github.com/aallbrig/treemand/@latest and
  https://proxy.golang.org/github.com/aallbrig/treemand/cli/treemand/@latest; `go install` run
  locally with a scratch `GOPATH`.
- GitHub API: https://api.github.com/repos/aallbrig/treemand and `/releases/latest` (v0.6.2).
- Source: `www/treemand/` (hugo.toml, themes/minimal/layouts, themes/minimal/static/css/main.css,
  content/, data/install.yaml, layouts/shortcodes/), `scripts/infra/publish_site.sh`,
  `infra/site.yaml`, `cli/treemand/go.mod`.
