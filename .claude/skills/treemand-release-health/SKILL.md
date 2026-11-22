---
name: treemand-release-health
description: Measure treemand's release pipeline from the GitHub API and public URLs (merge-to-release lead time, release assets, checksums, cosign signatures, Homebrew tap and treemand.dev mirror lag, package-manager publish, Auto Release failure rate) and turn findings into fixes. Use for a release, signing, mirror or Homebrew question, after a failed or slow release, or release-health findings.
---

# Release health

Follow **treemand-measure** for running, recording and filing. This skill is the release playbook. For whether the documented install paths actually install, use **treemand-install-experience**.

## Measure

`task release:health` (the same as `task measure -- release-health`) takes about 20 seconds and installs nothing. It reads the GitHub API, release downloads, the tap's raw formula and treemand.dev. Add `-- --findings` for the evidence, and `-- --record` to append history (on `main`, after a release has settled for an hour). A logged-in `gh`, `GH_TOKEN` or `GITHUB_TOKEN` raises the API rate limit. The probe works without one.

Signatures are only checked when `cosign` is installed (`RELEASE_HEALTH_COSIGN=<path>` points at a binary that is not on PATH). Without it, `signature_failures` is **skipped**, not passed. The weekly `Measure (weekly)` workflow installs the same cosign that `release.yml` signs with, so its run always checks signatures.

| metric | measures | rule |
| --- | --- | --- |
| `lead_time_min` | median, over the last 10 releases, of minutes from the tagged commit's merge (its committer date) to the release being published | ratchet down, 50% tolerance, `min_delta` 5 |
| `assets_missing` | expected assets absent: 5 archives, `checksums.txt`, `checksums.txt.sigstore.json` | budget 0 |
| `checksum_mismatches` | archives whose `checksums.txt` line is missing or differs from the SHA-256 GitHub reports for the uploaded bytes (the archive is downloaded when GitHub reports none) | budget 0 |
| `signature_failures` | bundles `cosign verify-blob` rejects for the identity install.sh pins (`release.yml@refs/tags/<tag>`) | budget 0 |
| `tap_lag`, `mirror_lag`, `mirror_assets_missing` | the Homebrew formula version, `treemand.dev/releases/latest.txt`, and the mirror's copies of the latest tag's files, behind the latest tag **more than 1 hour after it was published** | budget 0 |
| `install_script_missing` | `treemand.dev/install.sh` is not served or does not resolve through the mirror's `latest.txt` | budget 0 |
| `package_publish_failed` | the first Publish to Package Managers run after the latest release did not succeed, or none started within the hour | budget 0 |
| `auto_release_failure_pct` | failed runs among the last 20 completed Auto Release runs (cancelled runs are superseded by the next one and not counted) | ratchet down, `min_delta` 5 |

Inside the hour, a lag shows as `pending` in `details` and counts 0. That is the normal state while `release.yml` mirrors and deploys. Lead time measures the commit that was tagged. In a batch release, earlier PRs waited longer, and `scripts/release/plan.sh` shows which PRs a tag included.

The code is in `scripts/metrics/release_health.py`, with `scripts/metrics/probes/release-health.sh` as its wrapper. `task release:health:test` runs it offline against fixture releases with a fixed clock and a stand-in cosign. Extend that fixture when you change what the probe counts.

## Read the findings

`details[0]` is the latest tag: `tap`, `mirror` and `mirror_files` (each with a `state`), `install_script`, `package_publish` (the run URL and its conclusion) and `auto_release` (the failed run URLs). Every later entry is one release: `lead_time_min`, `assets_missing`, `checksum_mismatches` and, with cosign, `signature` (`ok` or cosign's last error line).

## Act

Every release-health finding is `area:packaging`. File it per **treemand-measure** (title `[measure] release-health: <metric> = <value> (<limit>)`, labels `area:packaging`, `measure:finding`, `loop:ready`) after deduplicating with `gh issue list --search "in:title [measure] release-health" --state open`. One broken release is one issue, even when it fails several metrics.

- **assets_missing**: open that tag's Release run (`gh run list --workflow release.yml`) and see whether the build loop, signing or `softprops/action-gh-release` dropped the file. Re-running the workflow on the tag re-uploads. The fix goes in `.github/workflows/release.yml`.
- **checksum_mismatches**: an asset was re-uploaded after `checksums.txt` was written, or `checksums.txt` was built from other bytes. Treat it as a supply-chain incident: do not re-sign over it. Cut a new patch release and name the bad tag in the issue.
- **signature_failures**: compare cosign's error with the identity in `release.yml`'s "Verify the signature" step and in `www/treemand/static/install.sh`. An identity mismatch usually means the Release ran on a branch ref instead of the tag (`scripts/release/dispatch.sh`).
- **tap_lag**: check the `Update Homebrew Tap` job of that tag's Release run and the Homebrew job of Publish to Package Managers. `task channels:check` shows the formula.
- **mirror_lag**, **mirror_assets_missing**: check the `Mirror to treemand.dev` job (`scripts/infra/mirror_release.sh`) and the site deploy that follows it.
- **install_script_missing**: the site deploy or `www/treemand/static/install.sh` changed. Reproduce with `curl -fsSL https://treemand.dev/install.sh | sh -s -- --dry-run`.
- **package_publish_failed**: read the failed job of the linked run. A job whose secret is absent skips by design. A job that ran and failed is the finding.
- **auto_release_failure_pct** regression, or a **lead_time_min** regression: open the failed or slow Auto Release runs and find the step (plan, dispatch, smoke or publish) that failed or took the time. Fix it in `scripts/release/` or `scripts/loop/smoke.sh`, test-first in their `*_test.sh`.

Fixes to the pipeline are tooling and carry no release label unless they change what users download.

**Prove it**: after the next release has settled for an hour, re-run `task release:health -- --findings`. The metric should read **improved**. Then record it with `--record`.
