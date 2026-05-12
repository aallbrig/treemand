# ADR: Config Subcommand Design

**Date:** 2026-05-12  
**Status:** Accepted

## Context

treemand needed a way to manage its YAML configuration file through the CLI.
Two broad approaches existed: flag-based operations (git style) or explicit
subcommands (gh/npm/kubectl style).

## Decision

Use explicit subcommands (`view`, `validate`, `set`, `init`, `path`, `edit`)
and a Go-based schema registry for validation — not flags-as-operations and
not JSON Schema.

## Rationale

### Subcommands over flags

We surveyed how popular CLIs handle configuration:

| Tool | Style | Notes |
|------|-------|-------|
| **git config** | Flag-based (`--get`, `--set`, `--list`) | Discoverable only if you already know the flags |
| **gh config** | Subcommand-based | `get`, `set`, `list`, `clear-cache` with per-host override |
| **npm config** | Subcommand-based | `get`, `set`, `delete`, `list`, `edit`, `fix` |
| **kubectl config** | Subcommand-based | `view`, `set-context`, `use-context`, multi-file merge |
| **gcloud config** | Subcommand-based | `set`, `get-value`, `list`, `configurations` |

Key takeaways that shaped our design:

- **Subcommand-based is more discoverable** — modern CLIs prefer explicit
  subcommands over flag-based operations. `treemand config --help` shows a
  menu; `git config --help` shows a wall of flags.
- **`view` shows origin** — git's `--show-origin` is an excellent debugging
  aid. `treemand config view` always prints the resolved config file path at
  the top of output.
- **`validate` not `lint`** — "validate" is the standard verb for config
  checkers (kubectl, terraform). "lint" belongs to code style tools.
- **Warnings vs errors** — unknown keys are warnings (allows forward
  compatibility); invalid values are errors (enforces correctness). A
  `--strict` flag promotes all warnings to errors.
- **`init` as explicit convenience** — most CLIs create config on first `set`,
  but an explicit `init` generating a commented default file is more welcoming.

### Go-based schema validation over JSON Schema

JSON Schema can validate YAML (via format conversion), but we chose a Go
schema registry (`config.KnownKeys`, `config.ValidateValue`) instead because:

- **Descriptive error messages** — Go validation can produce context-aware
  messages ("allowed values are unicode, ascii, nerd") rather than generic
  JSON Schema violations.
- **Fuzzy suggestions** — a Levenshtein-based `SuggestKey` function catches
  typos and offers corrections ("did you mean 'no_color'?").
- **Type coercion** — the registry distinguishes TypeString, TypeInt, TypeBool,
  TypeHexColor with range checks and enumeration.
- **Zero additional dependencies** — no JSON Schema library needed.

## Consequences

- All config keys are defined authoritatively in `config.KnownKeys`
  (`cli/treemand/config/schema.go`). Adding a new config key requires updating
  that registry.
- Validation behavior (warnings vs errors, `--strict`) is tested in
  `cli/treemand/cmd/cmd_test.go` and `cli/treemand/config/schema_test.go`.
- `config edit` falls back to `vi` if `$EDITOR` is unset — a deliberate
  convention matching git and other Unix tools.
- Config file precedence is: CLI flags > env vars (`TREEMAND_*`) > config
  file > built-in defaults.
