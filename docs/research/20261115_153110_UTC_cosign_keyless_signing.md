# Cosign keyless signing of `checksums.txt` and verification from `install.sh`

Research for [#117](https://github.com/aallbrig/treemand/issues/117) (map [#114](https://github.com/aallbrig/treemand/issues/114)).
Sources were checked on 2026-11-15 against cosign v3.1.4, cosign-installer v4.1.3, and Fulcio `main`.

## Answer in brief

- **Publish one file, `checksums.txt.sigstore.json`** (a Sigstore bundle), made by
  `cosign sign-blob --bundle checksums.txt.sigstore.json --yes checksums.txt` in the
  release job. Do not publish `.sig` + `.pem`. In cosign v3, `--output-signature` and
  `--output-certificate` are deprecated and the new bundle format is the default.
- **Permissions:** `id-token: write` on the job that signs, with `contents: write`
  kept for the release upload. No secrets and no key.
- **Certificate identity:** Fulcio sets the certificate SAN to
  `https://github.com/aallbrig/treemand/.github/workflows/release.yml@refs/tags/<tag>`, and
  the issuer is `https://token.actions.githubusercontent.com`. `install.sh` should pass
  the exact identity when it knows the tag. Otherwise it uses an **anchored** regexp
  (cosign's regexp match is unanchored).
- **Order:** verify the signature on `checksums.txt` first, then the archive's SHA-256
  against it.
- **If cosign is absent:** warn on stderr and continue with the checksum check.
  `--require-signature` turns the warning into a hard failure. If cosign is present,
  a missing bundle or a failed verification aborts and nothing is installed.
  goreleaser's `run` script and opengrep's `install.sh` work the same way.

## 1. Release workflow changes

### Facts

- `cosign sign-blob` keyless: "Cosign supports identity-based signing, associating an
  ephemeral signing key with an identity from an OpenID Connect provider", and "using a
  bundle is the recommended way of signing a blob". The command is
  `cosign sign-blob <file> --bundle bundle.sigstore.json`.
  ([docs.sigstore.dev: Signing blobs](https://docs.sigstore.dev/cosign/signing/signing_with_blobs/))
- cosign v3.1.4 flag source:
  `--output-signature`, `--output-certificate` and `--output` are `MarkDeprecated` with
  "please use --bundle …". `--new-bundle-format` now defaults to `true` and is deprecated
  because "this will be the only supported format in future versions".
  `--use-signing-config` also defaults to `true`, and `--tlog-upload` is deprecated.
  ([cosign v3.1.4 `cmd/cosign/cli/options/signblob.go`](https://github.com/sigstore/cosign/blob/v3.1.4/cmd/cosign/cli/options/signblob.go))
- The cosign v3.1.4 `sign-blob` reference lists `--bundle` ("write everything required to
  verify the blob to a FILE"), `--oidc-provider` (which includes `github-actions`) and `-y, --yes`.
  ([doc/cosign_sign-blob.md @ v3.1.4](https://github.com/sigstore/cosign/blob/v3.1.4/doc/cosign_sign-blob.md))
- GitHub: "the job or workflow must grant `id-token: write`". Without it, the OIDC JWT
  cannot be requested. The permission can be set on a single job.
  ([GitHub Docs: OIDC reference](https://docs.github.com/en/actions/reference/security/oidc))
- cosign-installer: the latest release is **v4.1.3** (2026-11-04,
  commit `756fe2d284c7292c4e9b62c8a28f1e5629c59532`), and its default `cosign-release` is
  `v3.1.4`. The action "verifies the integrity of the cosign release during installation" and
  "does not need any GitHub permission to run". The previous release is v4.1.2
  (`6f9f17788090df1f26f669e9d70d6ae9567deba6`), whose default is cosign v3.0.6.
  ([sigstore/cosign-installer README and action.yml](https://github.com/sigstore/cosign-installer))
- goreleaser's own migration note (Nov 2025, "Upgrading to Cosign v3") replaces
  `--output-certificate/--output-signature` with `--bundle=${artifact}.sigstore.json`.
  ([goreleaser blog: cosign-v3](https://github.com/goreleaser/goreleaser/blob/main/www/content/blog/cosign-v3.md))

### Recommended steps (in the `release` job, after "Generate checksums")

```yaml
jobs:
  release:
    permissions:
      contents: write   # create the GitHub Release
      id-token: write   # mint the GitHub OIDC token Fulcio exchanges for a signing cert
    steps:
      # ... existing checkout / setup-go / test / build steps ...

      - name: Install cosign
        uses: sigstore/cosign-installer@756fe2d284c7292c4e9b62c8a28f1e5629c59532 # v4.1.3
        with:
          cosign-release: v3.1.4

      - name: Sign checksums.txt (keyless)
        run: cosign sign-blob --yes --bundle dist/checksums.txt.sigstore.json dist/checksums.txt

      - name: Verify the signature we just made
        run: |
          cosign verify-blob \
            --bundle dist/checksums.txt.sigstore.json \
            --certificate-identity "https://github.com/${GITHUB_REPOSITORY}/.github/workflows/release.yml@${GITHUB_REF}" \
            --certificate-oidc-issuer https://token.actions.githubusercontent.com \
            dist/checksums.txt

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: |
            cli/treemand/dist/*.tar.gz
            cli/treemand/dist/*.zip
            cli/treemand/dist/checksums.txt
            cli/treemand/dist/checksums.txt.sigstore.json
```

Notes:

- The job has `working-directory: cli/treemand`, so the `run` paths above are relative
  to it, as the existing build step's paths are.
- Today `release.yml` grants `contents`, `pages` and `id-token: write` at **workflow** level, so
  every job (including `update-homebrew` and `deploy-site`) can mint an OIDC token.
  Move `id-token: write` to the `release` job only. `pages` goes away when Pages is
  retired (#114). The S3 mirror job will need `id-token: write` too, for the AWS role.
- The verify step proves the identity string install.sh will use before any user sees
  the release.
- Mirroring (#114): `checksums.txt.sigstore.json` is uploaded next to `checksums.txt` under
  `/releases/vX.Y.Z/` and `/releases/latest/`.
- Pin to a SHA with a version comment. v4.1.3 was 11 days old when this was written. If
  a two-week cooldown on new action releases applies, v4.1.2 (`6f9f177…`) with
  `cosign-release: v3.1.4` gives the same cosign.
- The signature goes to the public-good Rekor log. That is expected for a public repo.
  The bundle carries the inclusion proof, so verifiers need no Rekor lookup.

## 2. Bundle vs `.sig` + `.pem`

**Use the bundle (`checksums.txt.sigstore.json`).** Reasons:

1. In cosign v3 it is the default output, and the `.sig`/`.pem` flags are deprecated
   (see the signblob.go citations above).
2. One file holds the signature, the Fulcio certificate and the Rekor inclusion
   proof/timestamp. `verify-blob` docs: "The preferred way to provide verification material
   is via a Sigstore bundle using --bundle, which contains the signature, certificate,
   and transparency log proof."
   ([doc/cosign_verify-blob.md @ v3.1.4](https://github.com/sigstore/cosign/blob/v3.1.4/doc/cosign_verify-blob.md))
3. Client compatibility. `verify-blob` in cosign **v2.4.2 and later** auto-detects a
   new-format bundle (`NewBundleFormat: … || checkNewBundle(c.BundlePath)`). v2.4.0 and
   v2.4.1 need an explicit `--new-bundle-format`, and older v2 releases cannot read it.
   ([verify_blob.go @ v2.6.5](https://github.com/sigstore/cosign/blob/v2.6.5/cmd/cosign/cli/verify/verify_blob.go);
   `checkNewBundle` is absent at v2.4.0/v2.4.1 and present at v2.4.2, released 2025-02-04.)
   Users with older cosign get a clear failure, so the error message should name the minimum version.

There is no need to publish both formats. goreleaser's `run` script falls back to
`.sig`/`.pem` only because its own older releases were signed that way. treemand has
no signed releases yet, so there is nothing to stay compatible with.

## 3. The exact `cosign verify-blob` call

### Facts

- Fulcio builds the SAN for GitHub Actions from `subject-alternative-name-template: "{{ .url }}/{{ .job_workflow_ref }}"`,
  with `url` defaulting to `https://github.com`. The issuer URL is
  `https://token.actions.githubusercontent.com`.
  ([fulcio config/identity/config.yaml](https://github.com/sigstore/fulcio/blob/main/config/identity/config.yaml))
- `job_workflow_ref` has the form `owner/repo/.github/workflows/file.yml@refs/...`.
  ([GitHub Docs: OIDC reference](https://docs.github.com/en/actions/reference/security/oidc))
  For a tag-triggered, non-reusable workflow it names the workflow itself at the tag ref.
  The certificate also carries Source Repository Ref (`ref`) and Build Signer Digest
  (`job_workflow_sha`).
  ([fulcio docs/oid-info.md](https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md))
- `--certificate-identity-regexp` uses Go RE2 syntax. sigstore-go matches it with
  `regexp.MatchString`, which is **unanchored**, so `^…$` must be written explicitly.
  ([sigstore-go pkg/verify/certificate_identity.go](https://github.com/sigstore/sigstore-go/blob/main/pkg/verify/certificate_identity.go))

### Command

When `install.sh` knows the tag (`--version v0.7.0`, or after resolving `latest` to a tag):

```sh
cosign verify-blob \
  --bundle "$tmp/checksums.txt.sigstore.json" \
  --certificate-identity "https://github.com/aallbrig/treemand/.github/workflows/release.yml@refs/tags/$version" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  "$tmp/checksums.txt"
```

When it does not know the tag (plain `latest` without resolving it):

```sh
cosign verify-blob \
  --bundle "$tmp/checksums.txt.sigstore.json" \
  --certificate-identity-regexp '^https://github\.com/aallbrig/treemand/\.github/workflows/release\.yml@refs/tags/v[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  "$tmp/checksums.txt"
```

Prefer the exact form. It also binds the signature to the version asked for, so a
mirror cannot serve an older, validly signed `checksums.txt` under a newer version path.
The regexp form only proves "some tagged release of treemand". Resolving `latest` to
a concrete tag first (for example a small `/releases/latest/VERSION` file on the mirror, or the
GitHub `releases/latest` redirect) gets the exact form for every install. That is a
#122/#123 decision.

Caveat: the identity includes the workflow filename. If `release.yml` is renamed, keep a
regexp alternation for the old name, or older releases stop verifying.

## 4. Fallback UX when cosign is absent

How well-known scripts handle this:

- **goreleaser** (`goreleaser.com/static/run`,
  [www/static/run](https://github.com/goreleaser/goreleaser/blob/main/www/static/run)):
  it always checks `sha256sum`. When `command -v cosign` succeeds, it downloads
  `checksums.txt.sigstore.json`, falling back to `.sig`+`.pem`. If neither exists it
  prints `No signatures to verify.` and exits 1. Without cosign it prints
  `Could not verify signatures, cosign is not installed.` and continues. Its identity is
  `--certificate-identity-regexp "https://github.com/goreleaser/goreleaser.*/.github/workflows/.*.yml@$REF"`
  with `REF="refs/tags/$VERSION"`. That regexp is unanchored, with unescaped dots and any workflow file,
  which is looser than recommended here.
- **opengrep** ([install.sh](https://github.com/opengrep/opengrep/blob/main/install.sh)):
  if cosign is present and signatures exist, it always verifies ("if we have signatures and we
  also have cosign installed, then always verify"). On failure it prints
  `Error: Signature validation error.` and exits 1. Without cosign:
  `Warning: cosign needed for signature validation; the package will still be installed.`
  and `If this was not intended, delete and rerun with --verify-signatures or install cosign.`
  `--verify-signatures` makes cosign mandatory.
- **kubescape** ([install.sh](https://github.com/kubescape/kubescape/blob/master/install.sh))
  makes the opposite choice: it only checks the checksum and documents the cosign
  command (anchored regexp `'^https://github\.com/kubescape/kubescape/\.github/workflows/02-release\.yaml@refs/tags/'`)
  plus `gh attestation verify` as manual steps. Its reason is that tools absent on a stock machine would
  "either hard-fail every install or degrade to a check that reports success without
  having verified anything".

### Recommendation for treemand's `install.sh`

Keep it POSIX sh. The flow after downloading `checksums.txt`:

1. Download `checksums.txt.sigstore.json` alongside it.
2. **cosign found:**
   - Bundle missing: `die "checksums.txt.sigstore.json is missing for $version; refusing to install (pass --no-verify-signature for releases older than vX.Y.Z)"`.
     This fails closed, because otherwise anyone who can delete one file on the mirror turns verification off.
   - `cosign verify-blob …` fails: `die "signature check failed for checksums.txt (needs cosign v2.4.2 or newer); nothing was installed"`.
   - Success: `say "signature OK (signed by aallbrig/treemand release workflow, $version)"`.
3. **cosign not found:**
   - Default: `warn "cosign not found; skipped signature check (checksum is still verified). Install cosign to verify: https://docs.sigstore.dev/cosign/system_config/installation/"`, then continue.
   - With `--require-signature` (or `TREEMAND_REQUIRE_SIGNATURE=1`): `die "cosign is required by --require-signature but is not installed"`.
4. Then do the existing SHA-256 check against the now-trusted `checksums.txt`.
5. `--dry-run` adds `would verify    checksums.txt signature with cosign` or
   `would skip      signature check (cosign not found)`.
6. Rewrite the header comment ("does not defend against a compromised release"). With
   cosign present the script does defend against a tampered mirror or release asset.
   Without it, it still does not.

Tests (the script already has tests from #111): stub a fake `cosign` on `PATH` that
exits 0 or 1 and records its argv. Assert that the exact identity and issuer are
passed, that a missing bundle fails when cosign is present, that cosign absent gives a
warning and success, and that `--require-signature` without cosign fails.

## 5. Alternative: GitHub artifact attestations (`gh attestation verify`)

- `actions/attest` (v4.2.2, `1e69f48acb82d1966a394da916b4c1698aa569d6`).
  `actions/attest-build-provenance` v4 is now "simply a wrapper on top of `actions/attest`".
  It signs a SLSA provenance attestation with Sigstore (public-good for public repos)
  and stores it in GitHub's attestations API, not as a release asset. It needs
  `id-token: write`, `attestations: write` and `artifact-metadata: write`. It accepts
  `subject-checksums`, so it can attest every archive listed in `checksums.txt`.
  ([actions/attest README](https://github.com/actions/attest),
  [actions/attest-build-provenance README](https://github.com/actions/attest-build-provenance))
- Users verify with `gh attestation verify <file> --repo aallbrig/treemand`
  ([gh manual](https://cli.github.com/manual/gh_attestation_verify)).
- **Trade-off:** it gives stronger provenance (per binary, SLSA, no extra release asset)
  and costs one workflow step. But verification needs `gh` and a GitHub API call, and the
  attestation does not live on the treemand.dev mirror. It cannot replace the
  cosign path for a `curl … | sh` installer that defaults to treemand.dev.
  Worth adding alongside cosign later (kubescape documents both), not instead of it.

## Sources

- https://docs.sigstore.dev/cosign/signing/signing_with_blobs/
- https://github.com/sigstore/cosign/blob/v3.1.4/doc/cosign_sign-blob.md
- https://github.com/sigstore/cosign/blob/v3.1.4/doc/cosign_verify-blob.md
- https://github.com/sigstore/cosign/blob/v3.1.4/cmd/cosign/cli/options/signblob.go
- https://github.com/sigstore/cosign/blob/v2.6.5/cmd/cosign/cli/verify/verify_blob.go
- https://github.com/sigstore/cosign/releases (v3.1.4 latest, 2026-10-20)
- https://github.com/sigstore/cosign-installer (v4.1.3, action.yml default `cosign-release: v3.1.4`)
- https://github.com/sigstore/fulcio/blob/main/config/identity/config.yaml
- https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md
- https://github.com/sigstore/sigstore-go/blob/main/pkg/verify/certificate_identity.go
- https://docs.github.com/en/actions/reference/security/oidc
- https://github.com/goreleaser/goreleaser/blob/main/www/static/run
- https://github.com/goreleaser/goreleaser/blob/main/www/content/blog/cosign-v3.md
- https://github.com/opengrep/opengrep/blob/main/install.sh
- https://github.com/kubescape/kubescape/blob/master/install.sh
- https://github.com/actions/attest
- https://github.com/actions/attest-build-provenance
- https://cli.github.com/manual/gh_attestation_verify
