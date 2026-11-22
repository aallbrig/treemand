---
title: "Changelog"
description: "What changed in each treemand release: new features, changes and fixes, newest first."
# One file per version, newest first by date. Auto Release adds an entry per
# release (scripts/release/tag.sh); skill treemand-changelog edits them.
# Entries appear only on this page and in its feed, not as pages of their own.
outputs: ["HTML", "RSS"]
cascade:
  - target:
      kind: page
    build:
      render: never
      list: local
---

# Changelog

What changed in each treemand release, newest first. Subscribe with the
[RSS feed](index.xml), or see every release's downloads on
[GitHub](https://github.com/aallbrig/treemand/releases).
