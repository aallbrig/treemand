---
title: "cache"
weight: 4
---

# `treemand cache`

treemand caches discovered CLI trees in an SQLite database so repeat lookups are
instant. The `cache` subcommand lets you inspect and manage those entries.

<img src="/treemand/demos/cmd_cache.gif" alt="treemand cache demo" width="100%">

## Commands

```bash
treemand cache list           # list all cached CLIs with age and size
treemand cache clear git      # clear the cached entry for git
treemand cache clear          # clear all cached entries
```

## Cache details

| Property | Value |
|----------|-------|
| Location | `~/.treemand/cache.db` |
| Format | SQLite |
| TTL | 24 hours |
| Cache key | CLI name + version string + discovery strategies |

## Bypassing the cache

```bash
treemand --no-cache docker       # skip cache for this run
TREEMAND_CACHE_DIR=/tmp treemand git   # use a custom cache directory
```

The cache key includes the CLI's version string (from `<cli> --version`), so
updating a CLI automatically invalidates its cached tree on the next run.
