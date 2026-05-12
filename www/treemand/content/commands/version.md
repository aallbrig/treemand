---
title: "version"
weight: 6
---

# `treemand version`

Print the version, git commit, and build date of the installed binary.

<img src="/treemand/demos/cmd_version.gif" alt="treemand version demo" width="100%">

## Usage

```bash
treemand version
# treemand v0.3.0 (abc1234) built 2026-05-11
```

The version is embedded at build time from the git tag (`git describe --tags`).
Development builds show `dev` when no tag is present.
