---
title: "Treemand"
---

<div class="hero">
  <h1>🌲 Treemand</h1>
  <p>Visualize and interact with any CLI command hierarchy as a beautiful tree.</p>
  <div class="badges">
    <img src="https://img.shields.io/github/v/release/aallbrig/treemand" alt="release">
    <img src="https://img.shields.io/github/actions/workflow/status/aallbrig/treemand/ci.yml" alt="CI">
    <img src="https://img.shields.io/github/license/aallbrig/treemand" alt="license">
  </div>
</div>

## What is Treemand?

`treemand` is a Go CLI tool that **discovers and visualizes** any CLI command hierarchy — `git`, `kubectl`, `aws`, `docker` — as an intuitive tree, inspired by the classic `tree` command.

```
▼ git  [--version --verbose]
├── ▼ remote
│   ├── • add <name> <url>
│   └── • remove <name>
├── • commit [--message=<string>]
└── • status
```

## Features

- 🔍 **Auto-discovery** — discovers commands via `--help` recursion (no config needed)
- 🎨 **Colored output** — syntax-highlighted tree with configurable color schemes
- 🖥️ **Interactive TUI** (`-i`) — k9s-inspired explorer with live preview, keyboard + mouse
- 💾 **Caching** — SQLite cache for instant repeat lookups
- 🎭 **Configurable icons** — unicode (default), ASCII-safe, or Nerd Font icon sets
- ⚙️ **Config file** — `~/.config/treemand/config.yaml` for persistent settings
- 🔁 **Shell completions** — bash, zsh, fish, PowerShell (with cache-aware CLI suggestions)
- 📦 **Zero config** — works with any CLI out of the box
- 🔄 **Self-dogfooding** — `treemand treemand` works!

## Quick Start

```bash
# Non-interactive tree
treemand git

# Interactive TUI
treemand -i kubectl

# Limit depth
treemand --depth=2 aws

# ASCII-safe output (no Unicode)
treemand --icons=ascii docker

# JSON output
treemand --output=json docker
```

## Install

```bash
# Homebrew (macOS / Linux)
brew tap aallbrig/tap
brew install treemand

# go install
go install github.com/aallbrig/treemand/cli/treemand@latest
```

Or download a [pre-built binary](https://github.com/aallbrig/treemand/releases).

## Demo

<img src="/treemand/demo.gif" alt="treemand demo" width="100%">
