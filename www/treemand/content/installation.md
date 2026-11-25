---
title: "Installation"
description: "Install treemand on macOS, Linux or Windows: Homebrew, a signed and checksum-verified install script, release binaries or a source build, with copy buttons for each."
---

# Installation

Pick your system and copy the commands. We guess your system from your browser; use the buttons to change it. The install script verifies each release's signature and checksum, and nothing here needs admin rights unless you ask for `--system`.

{{< install >}}

## Install script options

The script is [`install.sh`](https://treemand.dev/install.sh): read it before you pipe it to a shell, and run it with `--help` for every option. It downloads from `https://treemand.dev/releases/`. Every release's `checksums.txt` is signed by this repository's release workflow ([Sigstore](https://www.sigstore.dev/), no private key involved). With [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) installed, the script refuses anything that signature doesn't vouch for, which protects you from a tampered download or mirror. Without cosign it still checks the SHA-256 checksum, which catches a corrupted or truncated download; add `--require-signature` to refuse installing without cosign.

```bash
curl -fsSL https://treemand.dev/install.sh | sh -s -- --require-signature # refuse to install unless cosign verifies it
curl -fsSL https://treemand.dev/install.sh | sh -s -- --version v0.6.2   # a specific release
curl -fsSL https://treemand.dev/install.sh | sh -s -- --dir ~/bin         # another directory
curl -fsSL https://treemand.dev/install.sh | sh -s -- --system            # /usr/local/bin, asks for admin rights only if needed
curl -fsSL https://treemand.dev/install.sh | sh -s -- --dry-run           # show what it would do
```

## Linux packages

Each release from v0.10.0 on also ships `.deb`, `.rpm` and `.apk` packages for amd64 and arm64. They install `treemand` to `/usr/bin` and its man page, and your package manager can remove it again. The package checksums are in the release's signed `checksums.txt`.

```bash
# Debian, Ubuntu
curl -fsSLO https://github.com/aallbrig/treemand/releases/latest/download/treemand_linux_amd64.deb
sudo apt install ./treemand_linux_amd64.deb

# Fedora, RHEL
sudo dnf install https://github.com/aallbrig/treemand/releases/latest/download/treemand_linux_amd64.rpm

# Alpine
curl -fsSLO https://github.com/aallbrig/treemand/releases/latest/download/treemand_linux_amd64.apk
sudo apk add --allow-untrusted ./treemand_linux_amd64.apk
```

On arm64, replace `amd64` with `arm64`. Each archive also has a software bill of materials (SPDX JSON) next to it on the release page, named after the archive with `.sbom.json` added.

## Verify a download by hand

Releases after v0.10.1 carry a [build provenance attestation](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations) for every archive and package: a signed statement that this repository's release workflow built that exact file. With the [GitHub CLI](https://cli.github.com/), check any file you downloaded from the release page:

```bash
gh attestation verify treemand_linux_amd64.tar.gz --repo aallbrig/treemand
```

To check the signed checksum list itself, as the install script does, use [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) (replace `vX.Y.Z` with the release):

```bash
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/aallbrig/treemand/.github/workflows/release.yml@refs/tags/vX.Y.Z" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

## Build from source

Requires Go 1.26+. To install the latest release without a checkout, run `go install github.com/aallbrig/treemand/cli/treemand@latest`. To build the current source:

```bash
git clone https://github.com/aallbrig/treemand.git
cd treemand/cli/treemand
go build -o treemand .
```

## Shell completions

After installing, set up completions for your shell:

```bash
# Bash — add to ~/.bashrc
source <(treemand completion bash)

# Zsh — add to ~/.zshrc
source <(treemand completion zsh)

# Fish
treemand completion fish > ~/.config/fish/completions/treemand.fish

# PowerShell — add to profile
treemand completion powershell | Out-String | Invoke-Expression
```

## Verify the installation

```bash
treemand version
# prints the version, commit and build date
```

## Update or uninstall

Re-run the install script (or `brew upgrade treemand`) to update. To uninstall a script install, delete `~/.local/bin/treemand` and `~/.local/share/man/man1/treemand.1`; for Homebrew, `brew uninstall treemand`.
