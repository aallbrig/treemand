# ADR: Help Discovery Fallback Chain

**Date:** 2026-05-11  
**Status:** Accepted

## Context

treemand discovers CLI structure by invoking the target CLI with various help
flags and parsing the output. The original fallback chain was:

1. `<cli> [sub...] --help`
2. `<cli> [sub...] -h`
3. `<cli> [sub...] help` (help as a trailing positional)

This covers most CLIs (cobra, click, docopt, etc.) but fails for the **Go
toolchain** and similar tools that use an inverted argument order:

```
go mod --help   →  "go mod: unknown command" (non-zero exit, error text)
go mod help     →  "go mod: unknown command" (same)
go help mod     →  "Go mod provides access to operations on modules." ✓
```

Go-style CLIs treat `help` as the *first* argument, not a suffix. The
`<cli> help <sub>` pattern is the canonical way to get help for subcommand
groups in the Go toolchain.

### Second issue: error text leaking into descriptions

`isErrorOutput()` detected error messages using `HasPrefix("unknown command")`,
but Go's error messages include the command name as a prefix:

```
"go mod: unknown command"  →  HasPrefix("unknown command") = FALSE
```

This caused `go mod --help` output to pass the error check, be treated as valid
help text, and have "go mod: unknown command" set as the node's description.

## Decision

1. **Add a fourth fallback**: `<cli> help [sub...]` — inverted order. Only
   applied when `args` is non-empty (no effect on root command discovery).

2. **Change `HasPrefix` to `Contains`** for "unknown command" and "invalid
   command" in `isErrorOutput`, so that `"go mod: unknown command"` is correctly
   identified as error output.

## Rationale

### Why not a dedicated "go strategy"?

A separate `GoDiscoverer` would require users to know which CLIs need it, or
add auto-detection heuristics. The fallback chain approach is transparent: it
tries each invocation style and uses the first that returns non-error output.
No user configuration required.

### Why `Contains` instead of `HasPrefix` for "unknown command"?

The prefix check was intentionally conservative to avoid false positives (e.g.
help text that *mentions* "unknown command" in an example). Using `Contains`
on the **first line only** (as `isErrorOutput` already does) is safe: the first
line of legitimate help text does not contain the phrase "unknown command".

### Fallback order and cost

The fallback chain now is:

1. `--help` (primary, zero extra cost when successful)
2. `-h` (fast alternative for POSIX-style CLIs)
3. `[sub...] help` (trailing positional, e.g. aws)
4. `help [sub...]` (prefix positional, e.g. go toolchain)

Each step is only tried when the previous returns empty or error output.
Typical CLIs succeed at step 1, paying no additional cost for the new fallback.
Go-style CLIs fail steps 1–3 (fast failure because the commands exit in < 5ms)
and succeed at step 4.

## Consequences

- `go mod` and `go work` now discover their full subcommand trees.
- Error text from CLIs emitting `<name>: unknown command` is no longer used as
  a node description.
- Discovery latency for CLIs that fail all four fallbacks increases by one
  extra invocation. Acceptable given the 5-second timeout per command.
- Any CLI whose first help line legitimately contains "unknown command" as part
  of description text will be misclassified as error output. This is judged
  acceptable: such a CLI would need to be handled case-by-case.
