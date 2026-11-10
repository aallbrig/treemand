---
title: "completion"
description: "treemand completion: install shell completion for bash, zsh, fish and PowerShell."
weight: 7
---

# `treemand completion`

Generate shell completion scripts for bash, zsh, fish, or PowerShell. Once set
up, pressing `Tab` after `treemand` suggests CLIs you've already explored from
the cache.

{{< demo src="demos/cmd_completion.gif" alt="treemand completion demo" width="900" height="420" >}}

## Setup

### Bash

```bash
# Add to ~/.bashrc
source <(treemand completion bash)
```

### Zsh

```bash
# Add to ~/.zshrc
source <(treemand completion zsh)
```

### Fish

```bash
treemand completion fish > ~/.config/fish/completions/treemand.fish
```

### PowerShell

```powershell
# Add to your PowerShell profile
treemand completion powershell | Out-String | Invoke-Expression
```

## How it works

The completion script hooks into your shell's completion system. When you type
`treemand [Tab]`, it suggests CLI names from the discovery cache — so the more
CLIs you explore, the more useful tab-completion becomes.

```bash
treemand git[Tab]        # → git
treemand kube[Tab]       # → kubectl (if you've explored it before)
```
