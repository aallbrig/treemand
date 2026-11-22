# ADR: Raise Default Stub Threshold from 50 to 150

**Date:** 2026-05-11  
**Status:** Accepted

## Context

treemand's `HelpDiscoverer` uses a *stub threshold*: when a CLI has more than N
subcommands, discovery stops eagerly recursing and creates lightweight stub
nodes instead. This prevents O(N²) help invocations on tools like AWS (200+
services) or kubectl.

The original default of **50** was calibrated for deeply-nested CLIs. It did
not anticipate wide, flat CLIs whose *root command* has 50+ direct children.
Observed failures:

| CLI | Top-level commands | Behavior at threshold=50 |
|-----|--------------------|--------------------------|
| docker | 53 | All root children stubbed |
| npm | 60+ | All root children stubbed |
| openssl | 50+ | All root children stubbed |
| systemctl | 50+ | All root children stubbed |

Every one of these CLIs is flat: subcommands are leaf commands with no further
children. Stubbing them at the root produces a tree that looks like a skeleton
— names with no descriptions, no flags, nothing actionable.

## Decision

Raise the default stub threshold from **50 to 150**.

## Rationale

### Why 150 specifically?

Real-world CLI surveys:

| CLI | Subcommand count |
|-----|-----------------|
| git | ~30 |
| docker | 53 |
| kubectl | ~40 |
| npm | ~65 |
| aws | 200+ (intentionally stubbed) |
| gh | ~20 |
| openssl | ~60 |

A threshold of 150 covers all commonly-used CLIs while still protecting against
genuinely massive tools like aws (200+ services). The performance difference
between running 53 help calls and 150 help calls is negligible on modern
hardware (< 3 seconds with 8 parallel workers).

### Alternatives considered

**Per-depth threshold** — apply a higher threshold at depth=0 than deeper
levels. Adds complexity; the root-vs-deep distinction is better handled by
adjusting the single value than by introducing a multi-value config.

**Auto-detection** — detect flat CLIs and skip stubbing entirely. Requires
speculative discovery and is harder to reason about.

**Leave at 50, document the flag** — users can pass `--stub-threshold=150`.
But the default should work well for the common case.

## Consequences

- docker, npm, openssl, systemctl and similar CLIs now discover fully at the
  default threshold.
- aws and other 200+ command CLIs continue to produce stubs (200 > 150).
- Existing users with `stub_threshold: 50` in their config file will need to
  update; the `config init` template is updated to reflect 150.
- The `--stub-threshold` flag and `stub_threshold` config key remain available
  for users who need to tune in either direction.
