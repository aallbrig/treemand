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

## Addendum 2026-11-10: command-list fallback (root only)

Some CLIs have no per-command help and expose their commands through a
command instead. tmux is the example: `tmux --help` prints only a usage line,
while `tmux list-commands` prints one `name (alias) [synopsis]` line per command.

**Decision.** After the help fallbacks, at the root only, when `--help` yielded
no subcommands, run `<cli> list-commands`. Accept the output only if it exits 0
and is clearly a list (at least four entries, at least 80% of non-empty lines
are `name (alias) synopsis` entries), so prose and error text never match.

- Flags come from the synopsis: a bracketed letter cluster such as `[-dE]` is
  boolean flags, and `[-c working-directory]` is a flag taking a value.
- Descriptions come from the man page: the first sentence of the paragraph
  after the command's heading in its COMMANDS section. The alias is appended
  as `(alias: x)`. Without a man page the node still has name, alias and flags.
- `commands` is deliberately not probed: as a bare word it is too likely to be
  something else.

**make and other target runners are out of scope.** Their "commands" are
project-specific targets, not part of the tool, and probing `make <name>` can
execute a user's build rule. `make`, `gmake`, `bmake`, `ninja`, `just`, `task`
and `rake` are explicitly skipped by the list-commands probe. A future
target-listing feature would need an explicit opt-in flag (for example
`make -qp` or `just --list` behind a flag), not default discovery.

## Addendum 2026-11-22: a refused flag form with a usage synopsis ends the chain

BSD tools reject `--help` and `-h` but still print their usage, on stderr
with a non-zero exit:

```
$ mkdir --help
mkdir: illegal option -- -
usage: mkdir [-pv] [-m mode] directory_name ...
```

`isErrorOutput` rightly calls this a refusal, so the chain went on to the
positional forms, and `mkdir help` created `./help` (#273).

**Decision.** When both flag forms are refused and either printed a usage
synopsis (a line starting with `usage:`, any case), the positional forms
are never tried. The refused output (`--help`'s before `-h`'s) is the node's
help text. A CLI that answers a flag with its usage takes operands, not a
`help` subcommand, so a positional `help` is an operand it acts on.

- Go-style CLIs are unaffected: `go mod --help` prints `go mod: unknown
  command` and `Run 'go help mod' for usage.`, which has no `usage:` line, so
  `help mod` is still reached. aws asks its positional form first via its preset.
- The rejected rule, "try a positional `help` only when the usage lists a
  `help` subcommand", would have dropped the go and aws-style fallbacks.
- Marker test: `TestRunHelp_usageAfterAnIllegalOptionIsHelpAndNoPositionalFormRuns`;
  chaos case `chaos-bsdoperand` fails as `SIDE-EFFECT` if the fake creates a file.

## Addendum 2026-11-23: installed completion scripts as a strategy

Many CLIs ship fish, zsh or bash completion files that list subcommands and
flags even when their `--help` is thin or slow (#99).

**Decision.** A `scripts` strategy (`-s help,scripts`) reads the CLI's
installed completion script and parses it statically. It is a separate
strategy, not a step of the help chain: it never runs the CLI, so it adds no
fork to discovery and cannot trigger side effects.

- **Read, never run.** The file is read (regular files up to 4 MiB) and
  tokenised; it is never sourced, and no `$(...)` in it is evaluated. The
  strategy is safe for any CLI, which the help chain cannot promise.
- **Where.** fish's `completions` and `vendor_completions.d`, zsh's
  `site-functions` and `vendor-completions`, and `bash-completion/completions`
  under the user's data directory, `$XDG_DATA_DIRS`, Homebrew and the system
  prefixes; fish's per-user directory; and the `bash_completion.d` directories.
  fish is tried first, then zsh, then bash; the first script that yields a
  tree is used.
- **What each shell gives.** fish: subcommands, nested levels and flags,
  placed by `-n` conditions it recognises (`__fish_seen_subcommand_from`,
  `__fish_use_subcommand`, clap's `__fish_<cli>_needs_command` and
  `_using_subcommand`). An entry whose condition it cannot place (an `or`, or
  an unknown helper function) is skipped rather than guessed. zsh: root flags
  from the first `_arguments` call, subcommands from the `'name:desc'` arrays
  of `_<cli>_commands` and `_<cli>__<sub>_commands`. bash: only the first
  argument's words, from clap's root `opts` list or a literal `compgen -W`
  guarded by a first-argument test.
- **Merged after help.** Help output stays authoritative; the script fills
  missing subcommands, flags and descriptions. Merge now treats a short-only
  flag (`-v`) as the long flag it abbreviates, and `Run` skips a strategy
  that found nothing instead of failing the merge.
- **Not covered.** Generated wrappers that call the CLI at completion time
  (cobra's `__complete`, argcomplete) contain no static data and yield
  nothing; cobra CLIs have the `completions` strategy for that.
- Marker tests: `TestParseFishScript_clapNestedSubcommands`,
  `TestParseFishScript_skipsWhatItCannotPlace`, and the e2e
  `TestScripts_completionScriptFillsWhatHelpMisses`.
