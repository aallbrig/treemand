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

## Build from source

Requires Go 1.26+. (`go install …@latest` doesn't work yet: the Go module lives in `cli/treemand/`, so there is no remote install path. Build from a checkout instead.)

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
