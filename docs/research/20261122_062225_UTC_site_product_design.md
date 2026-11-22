# Site product design review: does a developer feel at home on treemand.dev?

Date: 2026-11-22. Issue: #260. Skill: `.claude/skills/treemand-site-product-design/SKILL.md`.
Target: production, https://treemand.dev/ (13 sitemap pages). Evidence: `task site:design`
(`scripts/site/design_evidence.sh`), written to `.loop/design/2026-11-22/` on the host that ran it.
Screenshot paths below are relative to that directory; the screenshots are not committed.

Peers (home and install pages, one load each): taskfile.dev, Deno, Bun, GoReleaser and uv, from
`scripts/site/design_peers.tsv`. They were chosen from the devtool-website research linked in #132;
taskfile.dev is the maintainer's named reference.

Views: desktop 1280x800 and mobile 390x844 (touch), each in light and dark. Tile 1 is the first
screen.

## Verdict

The first screen is at home. It is as good as Bun's and better than taskfile.dev's or GoReleaser's:
- one clear headline and primary action;
- an OS-tabbed install;
- a live kubectl tree;
- an honest "Tested against 69 real CLIs" strip.

Dark mode is clean on every page, and every page loads in under 0.5 s.

The site stops feeling at home once a developer scrolls or goes into the docs:
- the recorded demos are empty or unreadable;
- the current version is not shown anywhere;
- tables and commented code blocks break at phone width;
- small inconsistencies (brand casing, heading styles, sidebar labels) make it read as less finished than the peers.

## Checklist scores (0 absent, 1 weak, 2 solid, 3 as good as the best peer)

| Item | Score | Bar set by | Notes |
| --- | --- | --- | --- |
| First screen | 3 | Bun (`bun/home--desktop-light-1.png`) | `treemand/home--desktop-light-1.png` answers what, why and how, with one primary action, on desktop and mobile. |
| Copy-paste install | 2 | Bun, uv (`uv/install--desktop-light-1.png`) | The commands match `www/treemand/data/install.yaml` and the v0.8.4 release. The home and docs copy buttons appear only on hover on desktop, while peers show them always. |
| Real output demo | 2 | GoReleaser (`goreleaser/home--desktop-light-2.png`), Taskfile (`taskfile/home--desktop-light-2.png`) | The hero tree is better than any peer's. The recorded demos lower the score (gaps 1 and 4). |
| Docs navigation and search | 2 | Bun, Deno install pages | Sidebar, outline, ⌘K search, prev/next and an edit link are all present. There is no outline on mobile, no scroll-spy, and the Reference syntax is incomplete. |
| Versions and changelog | 0 | Taskfile header `v3.54.0` (`taskfile/home--desktop-light-1.png`), Deno pill (`deno/home--desktop-light-1.png`) | No version anywhere, and the only version strings are stale placeholders (gap 2). |
| Dark mode | 3 | Bun (`bun/home--desktop-dark-1.png`) | Every dark tile is fully themed. Deno's home does not theme at all. |
| Typography and spacing | 1 | Taskfile | One type scale, but tables and `cmd  # comment` blocks break at 390px, and heading and label styles vary (gaps 3, 5 and 6). |
| Credibility signals | 2 | Taskfile, uv (stars, adopters) | The project shows honest, scaled signals: MIT, 69 tested CLIs, and cosign plus SHA-256 verification (stronger than any peer). The footer is one line (gap 7). |
| Speed | 2 | Bun (home 356 ms / 368 KB) | Home is 449 ms but 1567 KB, mostly the 1.5 MB `demo.gif`. Command pages carry 90 to 620 KB of GIFs. Try is 2299 KB (wasm, expected). |

## Rubric per page (Website section of `docs/design/20261109_172533_UTC_design_rubric.md`)

| Page | Hierarchy | Spacing | Type | Colour | Responsive | Consistency | Polish |
| --- | --- | --- | --- | --- | --- | --- | --- |
| home | pass | fail: empty terminal box | pass | pass | pass, but hero tree descriptions are cut off on mobile | fail: OS tabs here, pills on Installation | fail: demo contradicts its caption |
| installation | pass | pass | pass | pass | fail: wrapped commands and comments on mobile | fail: mixed sidebar label case | fail: Copy over the code in "Go (from source)"; two from-source sections |
| getting-started | pass | pass | pass | pass | fail: comment columns jumble on mobile, clip on desktop | fail: Title Case headings | fail: no install step, no output shown |
| try | pass | fail: collapsed tree, empty pane | pass | pass | fail: hint bar clipped on mobile | fail: emoji and "Treemand" heading | fail: help pane opens at 33% |
| commands (index) | pass | pass | pass | pass | pass | fail: label case | pass |
| commands/cache, config, completion, output, version | pass | fail: GIF frames mostly empty | pass | pass | fail: GIFs unreadable at 390px; comments wrap | fail: monospace H1 against sans H1 elsewhere | fail: GIF content (gap 4) |
| commands/tree, interactive | pass | pass | pass | pass | fail: code clipped on desktop; key table clipped on mobile | fail | fail: GIF wraps mid-word; status bar cut |
| reference | pass | pass | pass | pass | fail: Global Flags table unreadable at 390px | fail: Default column styling | fail: syntax lists 3 of 7 commands; stale version example |

## Ranked gaps

Ranked by cost to a first-time developer, then effort.

### 1. The home page's "terminal UI" demo is an empty box that shows a different command from its caption

- **Item**: Real output demo; rubric Polish and Spacing (home).
- **Evidence**:
  - `treemand/home--desktop-light-3.png` shows an 860x510 box with only a `>` prompt.
  - `treemand/home--mobile-light-4.png`: the text is unreadable at 390px.
  - The caption says `treemand -i kubectl`, but `demo.tape` types `treemand --depth=1 git` and other static commands.
  - `manifest.json`: home is 1567 KB, of which `static/demo.gif` is 1.5 MB.
  - Peer: `goreleaser/home--desktop-light-2.png` shows a terminal that is full on first view.
- **Change**:
  - Re-record `demo.tape` as a short `treemand -i kubectl` session.
  - Ship it as a muted, looping `<video>` (webm or mp4) with a poster of a populated frame, in `www/treemand/layouts/index.html` (the `tui-demo` section and the `demo-img.html` partial).
  - Alternatively, drop the section, since the hero already has a live tree.
- **Impact**: high. **Effort**: medium. Not tracked.

### 2. The current version and changelog are not visible anywhere, and the visible version strings are stale

- **Item**: Versions and changelog.
- **Evidence**:
  - No version appears on any page. The footer is "MIT License · GitHub" (`treemand/home--desktop-light-4.png`, `treemand/installation--desktop-light-4.png`).
  - `treemand/commands_version--desktop-light-1.png` and `treemand/reference--desktop-light-2.png` show `treemand v0.3.0 (abc1234)`, with dates that disagree, while the latest release is v0.8.4. Sources: `www/treemand/content/commands/version.md:17`, `www/treemand/content/reference.md:46`.
  - Peers: `taskfile/home--desktop-light-1.png` (header version menu), `deno/home--desktop-light-1.png` (hero "v2.9" pill), `bun/home--desktop-light-1.png` (release line).
- **Change**:
  - Render the latest release (already published as `/releases/latest.txt`) in the header or as a hero pill, linked to a changelog or GitHub releases page.
  - Generate the version examples from the same data, or write `vX.Y.Z`.
- **Impact**: high. **Effort**: medium. **Tracked in #143** (and the footer version in #145).

### 3. Wide tables are clipped at phone width

- **Item**: Typography and spacing; rubric Responsive.
- **Evidence**:
  - `treemand/reference--mobile-light-1.png`: the Global Flags table's Description column is cut to 1–3 letters a line.
  - `treemand/commands_interactive--mobile-light-2.png`: the key-bindings table is clipped ("Move u").
  - Peers' tables fit (`taskfile/install--mobile-light-1.png`).
- **Change**:
  - In `www/treemand/themes/minimal/static/css/main.css`, below 600px, stack each table row as a card (one `td` per line with its header as a label).
  - Or show a visible scroll affordance (an edge fade) on the existing `display:block; overflow-x:auto` tables.
  - Drop the mostly empty Short column into the Flag cell.
- **Impact**: high. **Effort**: low. Not tracked.

### 4. The command-page demo GIFs are empty on first view, unreadable on mobile and sometimes show the wrong thing

- **Item**: Real output demo; Speed; rubric Polish.
- **Evidence**:
  - Empty frames: `treemand/commands_cache--desktop-light-1.png`, `commands_output--desktop-light-1.png`, `commands_version--desktop-light-1.png`.
  - Unreadable at 390px: `treemand/commands_tree--mobile-light-1.png`.
  - Output wrapping mid-word ("fla/gs"): `treemand/commands_tree--desktop-dark-1.png`.
  - The completion demo shows the flag list, not completion: `treemand/commands_completion--desktop-dark-1.png`.
  - A CI home path `/home/runner/...`: `treemand/commands_config--desktop-dark-1.png`.
  - The version demo shows an untagged hash while the text says untagged builds show `dev`: `treemand/commands_version--desktop-light-1.png`.
  - `git` is listed twice in the cache demo: `treemand/commands_cache--desktop-dark-1.png`. It is worth checking whether `cache list` duplicates entries.
  - The GIFs weigh 90 to 630 KB each (`manifest.json`).
- **Change**:
  - Fix the tapes behind `scripts/record-subcmd-gifs.sh`:
    - a tagged build;
    - more columns;
    - a neutral `HOME`;
    - a real Tab completion;
    - terminal height sized to the output.
  - Publish them as `<video>` with a poster of the final frame.
  - On mobile, show the poster at native size with horizontal scroll.
- **Impact**: high. **Effort**: medium. Not tracked.

### 5. Commented command blocks jumble at phone width and clip on desktop

- **Item**: Typography and spacing; rubric Responsive.
- **Evidence**:
  - `treemand/getting-started--mobile-light-1.png` ("# explore / kubectl" wraps under the command).
  - `treemand/installation--mobile-light-3.png`.
  - `treemand/commands_cache--mobile-light-1.png`.
  - On desktop the comments hide behind scrollbars: `treemand/getting-started--desktop-light-1.png`, `treemand/commands_tree--desktop-light-1.png`.
- **Change**: in the Markdown under `www/treemand/content/`, put each comment on its own line above its command instead of aligning comments in a column. That reads well at every width; the desktop "Shell completions" block already does it.
- **Impact**: medium. **Effort**: low. Not tracked.

### 6. Brand and heading styles are inconsistent

- **Item**: Typography and spacing; rubric Consistency.
- **Evidence**:
  - The page titles read "Installation | Treemand" (`hugo.toml` has `title = "Treemand"`).
  - The Try page heading is "🌲 Try Treemand" (`treemand/try--desktop-light-1.png`).
  - H1s are monospace on cache, config, version and completion but sans elsewhere (`treemand/commands_cache--desktop-light-1.png` against `treemand/commands_tree--desktop-light-1.png`).
  - Sidebar labels mix "Tree Output" and "Output Formats: JSON and YAML" (which wraps) with "cache" and "config" (`treemand/commands--desktop-light-1.png`).
  - Headings are Title Case in the docs and sentence case on home and in the header.
  - The OS picker is tabs on home but pills on Installation (`treemand/installation--desktop-light-1.png`).
- **Change**:
  - Set `title = "treemand"`.
  - Drop the emoji.
  - Use one H1 style, one case convention for headings and sidebar labels (short names such as "Tree output", "Output formats"), and one OS-picker component.
- **Impact**: medium. **Effort**: low. Not tracked.

### 7. The footer is one line

- **Item**: Credibility signals.
- **Evidence**: `treemand/home--desktop-light-4.png` shows only "MIT License · GitHub". Peers have multi-column footers, and uv shows stars and the version in its header (`uv/home--desktop-light-1.png`).
- **Change**: add Docs, Project and Community columns, the version, and links to releases, security and checksums, in `www/treemand/themes/minimal/layouts/_default/baseof.html`.
- **Impact**: medium. **Effort**: low. **Tracked in #145.**

### 8. The Try page opens in a weak state

- **Item**: Real output demo; rubric Spacing, Responsive and Polish (try).
- **Evidence**:
  - The tree is collapsed to "Subcommands (22)" beside an empty pane, and the help pane opens at "[33%]" in the middle of the synopsis (`treemand/try--desktop-light-1.png`).
  - The TUI is taller than the 800px viewport, so its key-hint bar falls below the fold (`treemand/try--desktop-light-2.png`).
  - On mobile, the hint bar is clipped at the right edge (`treemand/try--mobile-light-2.png`).
  - "Ctrl+F find" looks stuck on (`treemand/try--desktop-light-2.png`).
- **Change**:
  - Expand the first level on load and start the help pane at the top, in the wasm entry or the try page script.
  - Size the stage to the viewport height.
  - Fit the hint bar at 390px.
- **Impact**: medium. **Effort**: medium. Not tracked.

### 9. Installation page details

- **Item**: Copy-paste install; rubric Polish.
- **Evidence**:
  - The "Go (from source)" Copy button sits on top of the code, above a horizontal scrollbar (`treemand/installation--desktop-light-2.png`).
  - "Go (from source)" and "Build from source" give different commands for the same task (`-2.png`, `-3.png`).
  - The macOS release-binary card uses `mv … /usr/local/bin`, which can need sudo, while the page says no admin rights are needed.
  - The outline lists sections but not channels, unlike `taskfile/install--desktop-light-1.png`.
- **Change**:
  - Merge the two from-source sections.
  - Put the long clone command on two lines.
  - Use `~/.local/bin` in the release-binary step (in `www/treemand/data/install.yaml` and `content/installation.md`).
- **Impact**: medium. **Effort**: low. Not tracked.

### 10. Getting Started has no install step and shows no output

- **Item**: Real output demo; Docs navigation.
- **Evidence**: `treemand/getting-started--desktop-light-1.png` starts with `treemand git`, with no link to Installation, and shows no output in the first four screens.
- **Change**: in `www/treemand/content/getting-started.md`, open with "Install first" linked to Installation, and show a short real `treemand --depth=1 git` output under the first block, the way Taskfile pairs input with output.
- **Impact**: medium. **Effort**: low. Not tracked.

### 11. Reference "Command Syntax" lists only 3 of the commands

- **Item**: Docs navigation; rubric Polish.
- **Evidence**: `treemand/reference--desktop-light-1.png` lists `treemand <cli>`, `version` and `cache` only. `config`, `completion` and `-i` are missing.
- **Change**: list every subcommand in `www/treemand/content/reference.md`, or generate the list from `treemand --output=json treemand`.
- **Impact**: low. **Effort**: low. Not tracked.

### 12. Copy buttons on desktop appear only on hover

- **Item**: Copy-paste install.
- **Evidence**:
  - `treemand/home--desktop-light-1.png` and `treemand/getting-started--desktop-light-1.png` show no Copy button.
  - `main.css:118` sets `.copyable > .copy-btn { opacity: 0 }` until hover.
  - Touch views show it (`treemand/home--mobile-light-1.png`).
  - Bun, Deno and uv always show a copy icon (`bun/home--desktop-light-1.png`).
- **Change**: show the button at low emphasis by default, and at full emphasis on hover.
- **Impact**: low. **Effort**: low. Not tracked.

### 13. Code highlighting differs between themes and colours arguments as keywords

- **Item**: Typography and spacing.
- **Evidence**:
  - `help` in `--exclude=help` and `set` are coloured as shell keywords (`treemand/getting-started--desktop-light-2.png`, `treemand/commands_config--desktop-light-1.png`).
  - Comments are italic in dark mode and upright in light mode.
- **Change**: mark these blocks `console` or `text`, and use one Chroma style pair with matching token rules.
- **Impact**: low. **Effort**: low. Not tracked.

### 14. Docs outline: no scroll-spy and no mobile outline

- **Item**: Docs navigation and search.
- **Evidence**:
  - `treemand/commands_interactive--desktop-light-2.png`: no active section is shown while scrolling.
  - Mobile has a "Docs menu" but no "On this page" control (`treemand/getting-started--mobile-light-1.png`).
  - Peer: Taskfile's "Documentation | On this page" bar (`taskfile/install--mobile-light-1.png`).
- **Change**: add scroll-spy to the outline, and an "On this page" disclosure beside "Docs menu" on mobile.
- **Impact**: low. **Effort**: medium. Not tracked.

## Not judged

- **Below the tile cap**: tiles stop at four screens per view, so the lower parts of `reference` (10041px tall on desktop) and `getting-started` were not seen.
- **Try page loading state**: the screenshots were taken after the page loaded, so they don't show whether `/try/` gives progress feedback while its wasm loads.
