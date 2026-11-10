# Audit dimensions

For each: the **probe** (run it), what a **3** looks like.

## 1. Discovery quality
Probe: `bash scripts/loop/corpus.sh` (real binaries on this machine); extend with
`compgen -c | shuf -n 15` and any CLI the vision names. Read the TSV for failures,
stub nodes (`no_desc`, `no_flags`), root descriptions that are usage synopses,
single-node results for CLIs that have subcommands (tmux, make), and cold seconds.
3 = every corpus CLI finishes < 10s cold, no failures, descriptions and flags present
or lazily expandable and marked `discovered:false`, root description is the tool's own one-liner.

## 2. CLI experience
Probe: run `treemand` bare, `--help`, bad input, nonexistent CLI, `| cat` (non-TTY),
`NO_COLOR=1`, `--output=json | jq`, exit codes, shell completion install, man page.
3 = every path prints something helpful, exit codes are meaningful, piping and
NO_COLOR are honoured, completions install in one documented step.

## 3. TUI experience
Probe: `task visual:check` and drive `-i` through tmux (`send-keys`, `capture-pane`) at
80x24, 120x40 and 200x50 on a 500-node CLI (aws) and a small one. Try search, expand all,
resize, mouse, value entry, copy/run. Score against the newest `docs/design/*_design_rubric.md`.
3 = clean at all sizes, every key discoverable in-app, responsive on huge trees, no dead ends.

## 4. Command assembly
Probe: assemble commands with flags, enums, repeated flags, positional args, values with
spaces and quotes; run the result.
3 = output is correct and shell-safe for bash, zsh and fish.

## 5. Website
Probe: `task visual:check` screenshots; Lighthouse via `npx --yes lighthouse@12 <url> --only-categories=performance,accessibility,seo,best-practices --chrome-flags=--headless --output=json`;
`task site:check` link test; the Try page end to end.
3 = Lighthouse >= 95 in all four, social card and favicons present, mobile clean, rubric passes.

## 6. Distribution from the site
Probe: the Installation page and the README. For each channel (go install, Homebrew, Scoop,
winget, Chocolatey, AUR, Snap, deb/rpm, Docker image, one-line install script) check it is
documented, copy-pastable, and current against `gh release list`. Test what this machine can.
3 = every mainstream channel works and matches the latest release, the site detects the
visitor's OS and shows the right command first, binaries are checksummed and signed.

## 7. Open-source professionalism
Probe: files present (`SECURITY.md`, `CODE_OF_CONDUCT.md`, `CONTRIBUTING.md`, issue and PR
templates, `CHANGELOG` or curated release notes, `dependabot.yml`), `gh repo view` description
and topics, README badges and first-screen clarity, release cadence, semver discipline,
issue/PR hygiene.
3 = matches the checklist of a well-regarded Go CLI (compare with peers in landscape.md).

## 8. Test adequacy
Probe: `cd cli/treemand && go test -cover ./...`; find untested exported functions in
parsers; check for fuzz tests, golden tests per corpus CLI, an end-to-end test of the built
binary against hermetic fake CLIs, race and flake runs, coverage enforced in `task ci`.
3 = every package >= 85%, parsers fuzzed, corpus regressions caught in CI, coverage floor enforced.

## 9. Stack versus peers
Probe: `go list -m all`, compare libraries and tooling with landscape.md (CLI framework, TUI
stack, release tooling, lint config, CI shape).
3 = nothing a peer uses that treemand lacks to its detriment, nothing obsolete.

## 10. Competitive landscape
Probe: for each project in landscape.md, read its README and feature list (install the Go/Rust
ones to a temp dir when cheap); list features users get there that treemand lacks.
3 = no peer feature is both valued by users and missing here.

## 11. Agent and API surface
Probe: stability and documentation of `--output=json|yaml`; presence of a JSON Schema;
exit codes for scripts; feasibility notes for an MCP server or `treemand mcp`.
3 = documented schema with a compatibility promise; MCP/API proposals filed as `loop:proposed`.

## 12. Performance and reliability
Probe: corpus timings, memory (`/usr/bin/time -v`), cached versus cold, `task test-race`,
hung-CLI behaviour (`--timeout`), huge output, odd locales.
3 = cold < 10s for the largest corpus CLI, cached < 300ms, no hangs, no races.

## 13. Security and supply chain
Probe: `task vuln`, gitleaks, workflow permissions, pinned action versions, provenance/SBOM,
dependency freshness.
3 = least-privilege workflows, pinned actions, signed and attested releases, zero known vulns.

## 14. Documentation
Probe: walk docs as a new user; every flag and subcommand documented (`gendocs`), examples
run, troubleshooting and FAQ exist, README matches the product.
3 = a newcomer reaches a first useful result in under two minutes without leaving the docs.
