---
name: treemand-install-experience
description: Measure treemand's install experience on every host (one-liner, upgrade, reinstall, uninstall, go install, brew) natively and in Linux containers, and turn failures into fixes. Use for install, packaging or upgrade questions, a user's install report, or install findings.
---

# Install experience

Follow **treemand-measure** for running, recording and filing; this skill is the install playbook.

## Measure

`task install:experience` (the same as `task measure -- install`) runs every documented path against the **real latest release** on treemand.dev and reports per path. Add `-- --findings` for the per-path rows, `-- --record` to append history (on `main`, after a release).

- **Host**: `oneliner` (`curl -fsSL https://treemand.dev/install.sh | sh`), `upgrade` (`--version <previous>`, then latest into the same dir), `reinstall` (latest twice), `uninstall` (default location, then the Installation page's uninstall steps), `go-source` (the docs' clone plus `go install .`), `go-latest` (`go install …/cli/treemand@latest`, the path a Go user tries first).
- **Containers**: `oneliner`, `upgrade` and `reinstall` in ubuntu:24.04, debian:stable-slim, alpine:3 and fedora. On macOS the probe starts Docker Desktop; with no Docker they are **skipped** with a reason, never failed.
- **brew** runs only with `PROBE_BREW=1` on macOS, because it installs into the real Homebrew prefix (and uninstalls again if treemand was not installed before). Every other path is sandboxed: temp `HOME`, `--dir`, `GOPATH`, `GOBIN` and build cache.

Each row records: exit status, installed version against the latest tag, `renders` (`treemand --depth=1 --output=json ls` returned a node), on macOS the `com.apple.quarantine` xattr, whether a quarantined binary still runs, and the code signature, and the PATH hint the installer printed. A path is **ok** only when all of these hold; `note` names the first one that failed.

The code: `scripts/metrics/probes/install.sh` (orchestration, latest and previous tags, containers) and `scripts/metrics/install_paths.sh` (one row per path). `task install:probe:test` runs both against a fake site; `task install:test` covers `install.sh` itself.

## Act

Read the failing row's `note`, then reproduce that path by hand from a `mktemp -d` dir with `HOME` pointed there.

- **version mismatch**: the mirror's `latest.txt`, a release archive, or the Homebrew formula lags the tag. Check the Release and Packages workflows for that tag; `task channels:check` shows the formula.
- **no tree**: the release binary itself is broken on that platform. Reproduce with the archive from treemand.dev/releases, then fix the build or the code, and add an e2e test.
- **quarantined and does not run** (macOS): Gatekeeper refused it. The fix is packaging (signing and notarization), not the installer.
- **no PATH hint**, a stray `.treemand.*` after a reinstall, or files the documented uninstall leaves behind: fix `www/treemand/static/install.sh` test-first in `scripts/install_test.sh`.
- **no uninstall documented**: fix `www/treemand/content/installation.md`.
- **go-latest**: the module path does not match its location (#135). Comment the new evidence on that issue rather than filing another.
- **container did not run** (in `skipped`): the image or its package mirror, not treemand. Rerun before reading anything into it.

Every install failure is `area:packaging`. Fixes that change what users download or read carry `release:patch`.

**Prove it**: the failing path's `ok.<env>.<path>` reads 1 and `paths_failed` drops, shown as **improved**.
