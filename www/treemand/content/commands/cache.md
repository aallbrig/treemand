---
title: "cache"
description: "treemand cache: list, clear and understand the discovery cache that makes repeat runs instant."
weight: 4
---

# `treemand cache`

treemand caches discovered CLI trees in an SQLite database so repeat lookups are
instant. The `cache` subcommand lets you inspect and manage those entries.

{{< demo src="demos/cmd_cache.gif" alt="treemand cache demo" width="900" height="420" >}}

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
| Invalidation | the CLI's executable changes (path, size or modification time) |
| TTL | 30 days (`cache_ttl_hours` in the config; `0` = until the CLI changes) |
| TUI expansions | commands discovered in `-i` are saved back to the entry on exit |
| Cache key | CLI binary fingerprint + discovery strategies + depth/stub settings |

## Bypassing the cache

```bash
treemand --no-cache docker       # skip cache for this run
treemand --refresh docker        # re-crawl now and update the entry
TREEMAND_CACHE_DIR=/tmp treemand git   # use a custom cache directory
```

The cache key includes the CLI's version string (from `<cli> --version`), so
updating a CLI automatically invalidates its cached tree on the next run.
