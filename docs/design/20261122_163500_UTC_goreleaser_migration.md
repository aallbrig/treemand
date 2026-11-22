# Release builds with GoReleaser

Issue #46. The maintainer approved the migration on 2026-11-22 on one condition:
everything downstream of the release keeps working unchanged.

## What changed

`release.yml`'s build job used to build, archive, checksum and sign in a shell
script. GoReleaser v2.19.0 now does that, driven by `cli/treemand/.goreleaser.yaml`,
and also creates the GitHub Release. Everything around it stays as it was: Auto
Release still plans and tags from `release:*` labels, `dispatch.sh` still starts
the `Build & Release` job, the signature check after signing is the same step,
and the Homebrew, mirror and site jobs, the package-manager publish and the
`notes.py` release body are untouched.

New assets:

- `treemand_linux_{amd64,arm64}.{deb,rpm,apk}` from nfpm. They install `/usr/bin/treemand`,
  the man page and the licence. They are listed in `checksums.txt`, so the signature covers them.
- `<archive>.sbom.json`: an SPDX SBOM per archive, from syft v1.56.0.

## The asset contract

These consumers read the release by name. The config reproduces each name and
layout, and `scripts/release/snapshot_test.sh` checks all of them in CI on every PR
(job `Release snapshot`) and locally (`task release:snapshot`):

| Consumer | Depends on |
|---|---|
| `install.sh` | `treemand_<os>_<arch>.tar.gz` with `treemand_<os>_<arch>` and `treemand.1` at the root; a `checksums.txt` line per archive; `checksums.txt.sigstore.json` from `release.yml@refs/tags/<tag>` |
| Homebrew tap job | the same archive members; one `checksums.txt` line per archive, found with a substring `grep` |
| `release-packages.yml` | `treemand_windows_amd64.zip` and `treemand_linux_amd64.tar.gz` lines, found with `grep` |
| mirror, release-health probe | the five archives, `checksums.txt` and its bundle |

## Risks and how each is handled

| Risk | Handling |
|---|---|
| An asset name or layout drifts and breaks installs | The contract test fails the PR. |
| An SBOM line in `checksums.txt` gives the `grep` consumers two hashes | `checksum.ids` keeps SBOMs out of `checksums.txt`. The contract test requires exactly one line per archive. |
| The signing identity changes | GoReleaser runs inside `release.yml`, so the certificate still names it. Signing was checked locally with a stand-in `cosign` (same arguments as before). The existing verify step fails the release before any user downloads it. |
| The man page loses its version | A `before` hook runs `gendocs` with the tag, as the old script did. |
| A broken first release | Auto Release's smoke test and `scripts/loop/rollback.sh`, as for any release. |
| Third-party tool supply chain | GoReleaser, syft and both actions are pinned (actions by commit SHA). |

What the dry run does not cover: keyless signing (it needs the release job's OIDC
token) and the upload to the GitHub Release. The first real release covers both, and
its verify step and the release-health and install probes check the result.

## Not done here

- Scoop, winget and Homebrew via GoReleaser: the current publish jobs work, and
  moving them is a separate change.
- Signing the SBOMs: they are published but not in the signed `checksums.txt`.
- A signed apt/yum repository: the packages are downloaded directly for now.
