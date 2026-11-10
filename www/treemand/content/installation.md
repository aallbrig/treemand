---
title: "Installation"
description: "Install treemand on macOS, Linux or Windows: Homebrew, a checksum-verified install script, release binaries or go install, with copy buttons for each."
---

# Installation

Pick your system and copy the commands. We guess your system from your browser; use the buttons to change it. The install script and the release archives are checksum-verified, and nothing here needs admin rights unless you ask for `--system`.

{{< install >}}

## Install script options

The script is [`install.sh`](https://aallbrig.github.io/treemand/install.sh): read it before you pipe it to a shell, and run it with `--help` for every option. The checksum guards against a corrupted or truncated download; it comes from the same GitHub release as the archive, so it does not defend against a compromised release.

```bash
curl -fsSL https://aallbrig.github.io/treemand/install.sh | sh -s -- --version v0.6.1   # a specific release
curl -fsSL https://aallbrig.github.io/treemand/install.sh | sh -s -- --dir ~/bin         # another directory
curl -fsSL https://aallbrig.github.io/treemand/install.sh | sh -s -- --system            # /usr/local/bin, asks for admin rights only if needed
curl -fsSL https://aallbrig.github.io/treemand/install.sh | sh -s -- --dry-run           # show what it would do
```

## Build from source

Requires Go 1.26+.

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
