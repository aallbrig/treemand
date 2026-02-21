---
title: "Installation"
---

## Go Install (recommended)

```bash
go install github.com/aallbrig/treemand@latest
```

Requires Go 1.22+. The binary is placed in `$GOPATH/bin` (usually `~/go/bin`).

## Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/aallbrig/treemand/releases):

| Platform | Architecture | Download |
|----------|-------------|---------|
| Linux | amd64 | `treemand_linux_amd64.tar.gz` |
| Linux | arm64 | `treemand_linux_arm64.tar.gz` |
| macOS | amd64 | `treemand_darwin_amd64.tar.gz` |
| macOS | arm64 (Apple Silicon) | `treemand_darwin_arm64.tar.gz` |
| Windows | amd64 | `treemand_windows_amd64.zip` |

### Linux/macOS

```bash
curl -L https://github.com/aallbrig/treemand/releases/latest/download/treemand_linux_amd64.tar.gz | tar xz
sudo mv treemand /usr/local/bin/
```

## Build from Source

```bash
git clone https://github.com/aallbrig/treemand.git
cd treemand/cli/treemand
go build -o treemand .
```

## Verify Installation

```bash
treemand version
# treemand v1.0.0
```
