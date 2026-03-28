# Configuration Subcommand

The `treemand config` command manages treemand's YAML configuration file.

## CLI Design Research

Before designing the config subcommand, we surveyed how popular CLIs handle
configuration management.

### Approaches Studied

| Tool | Style | Subcommands | Scopes |
|------|-------|-------------|--------|
| **git config** | Flag-based (`--get`, `--set`, `--list`) | n/a (flags as ops) | local, global, system |
| **gh config** | Subcommand-based | `get`, `set`, `list`, `clear-cache` | per-host override |
| **npm config** | Subcommand-based | `get`, `set`, `delete`, `list`, `edit`, `fix` | project, user, global |
| **kubectl config** | Subcommand-based | `view`, `set-context`, `use-context`, … | multi-file merge |
| **gcloud config** | Subcommand-based | `set`, `get-value`, `list`, `configurations` | named profiles |

### Key Takeaways

1. **Subcommand-based is more discoverable** — modern CLIs (gh, npm, gcloud)
   prefer explicit subcommands over flag-based operations (git).
2. **`view` / `list` with file origin** — git's `--show-origin` is excellent
   for debugging precedence. Adopted as default behavior in `config view`.
3. **Validation strategy** — "validate" is the standard nomenclature for CLIs
   (kubectl validate, terraform validate). "lint" is more common for code
   style tools (eslint, golangci-lint). We use `validate`.
4. **Warnings vs errors** — gh's approach: warn on unknown keys (allows
   extensibility), error on invalid values (enforces correctness). A `--strict`
   flag can promote warnings to errors.
5. **Init on demand** — most CLIs create config on first `set`. An explicit
   `init` command is a helpful convenience for generating a commented default
   config.
6. **Schema validation** — JSON Schema is not commonly used for CLI configs.
   Go-based validation provides better UX (clearer error messages, type-aware
   suggestions). We define a registry of known keys in Go.

### Design Decision: No JSON Schema

While JSON Schema can validate YAML (via conversion), it adds a dependency
and produces generic error messages. A Go-based schema registry:
- Enables descriptive, context-aware error messages
- Supports type coercion (e.g. "true" → bool)
- Can suggest corrections for misspelled keys
- Zero external dependencies

## Subcommand Interface

```
treemand config                        # show help / available subcommands
treemand config view                   # show merged config + file location
treemand config validate [--strict]    # validate config file
treemand config set <key> <value>      # set a config value
treemand config init [--force]         # create default config with comments
treemand config path                   # print config file path
treemand config edit                   # open config in $EDITOR
```

### `config view`

Displays the current effective configuration as YAML with the config file
location printed at the top.

```
$ treemand config view
# Config file: ~/.config/treemand/config.yaml

icons: ascii
desc_line_length: 120
stub_threshold: 50
tree_style: graph
no_color: false
depth: -1
no_cache: false
strategies:
  - help
colors:
  base: "#FFFFFF"
  subcmd: "#5EA4F5"
  ...
```

When no config file exists, shows default values and notes that no config
file was found.

### `config validate`

Checks the config file for:
- **Warnings**: unknown/unsupported keys, deprecated keys
- **Errors**: invalid values (e.g. `icons: invalid_preset`, `depth: abc`),
  YAML syntax errors

Exit codes:
- `0` — valid (may include warnings)
- `1` — invalid (errors found, or warnings found with `--strict`)

```
$ treemand config validate
Config file: ~/.config/treemand/config.yaml
⚠ warning: unknown key "colour" (did you mean "colors"?)
⚠ warning: unknown key "cache_ttl"
✓ 0 errors, 2 warnings

$ treemand config validate --strict
Config file: ~/.config/treemand/config.yaml
✗ error: unknown key "colour" (did you mean "colors"?)
✗ error: unknown key "cache_ttl"
✗ 2 errors (warnings promoted to errors with --strict)
```

### `config set`

Writes a value to the config file, creating it if necessary. Supports
dot-notation for nested keys.

```
$ treemand config set icons nerd
$ treemand config set colors.subcmd "#FF5555"
$ treemand config set depth 5
```

Values are validated before writing. Invalid values are rejected:
```
$ treemand config set icons invalid_preset
Error: invalid value "invalid_preset" for key "icons": allowed values are unicode, ascii, nerd
```

### `config init`

Creates a default config file with inline comments explaining each option.
Refuses to overwrite an existing file unless `--force` is passed.

```
$ treemand config init
Created ~/.config/treemand/config.yaml

$ treemand config init
Error: config file already exists at ~/.config/treemand/config.yaml (use --force to overwrite)
```

### `config path`

Prints the resolved config file path. Useful for scripting:

```
$ cat $(treemand config path)
$ $EDITOR $(treemand config path)
```

### `config edit`

Opens the config file in `$EDITOR` (falls back to `vi`). Creates a default
config first if none exists.

## Configuration Keys

| Key | Type | Default | Allowed Values | Description |
|-----|------|---------|----------------|-------------|
| `icons` | string | `unicode` | `unicode`, `ascii`, `nerd` | Icon preset |
| `desc_line_length` | int | `80` | 1–500 | Max description width |
| `stub_threshold` | int | `50` | 1–10000 | Stub creation threshold |
| `tree_style` | string | `default` | `default`, `columns`, `compact`, `graph` | TUI tree style |
| `no_color` | bool | `false` | `true`, `false` | Disable color output |
| `depth` | int | `-1` | -1–100 | Max tree depth (-1 = unlimited) |
| `no_cache` | bool | `false` | `true`, `false` | Disable caching |
| `strategies` | string | `help` | `help`, `completions`, `man`, `help,completions`, … | Discovery strategies |
| `colors.base` | string | `#FFFFFF` | hex color | Base command color |
| `colors.subcmd` | string | `#5EA4F5` | hex color | Subcommand color |
| `colors.flag` | string | `#50FA7B` | hex color | Flag color (fallback) |
| `colors.flag_bool` | string | `#50FA7B` | hex color | Boolean flag color |
| `colors.flag_string` | string | `#8BE9FD` | hex color | String flag color |
| `colors.flag_int` | string | `#FFB86C` | hex color | Integer flag color |
| `colors.flag_other` | string | `#BD93F9` | hex color | Other flag color |
| `colors.pos` | string | `#F1FA8C` | hex color | Positional arg color |
| `colors.value` | string | `#FF79C6` | hex color | Value/type color |
| `colors.invalid` | string | `#FF5555` | hex color | Error color |
| `colors.selected` | string | `#00BFFF` | hex color | Selection background |
| `colors.selected_text` | string | `#000000` | hex color | Selection foreground |

## Edge Cases

- **No config file**: all defaults are used; `config view` says "no config file found"
- **Empty config file**: valid — all defaults apply
- **Partial config**: only specified keys override defaults
- **Config file with unknown keys**: warning (error with `--strict`)
- **Config file with invalid YAML**: error on parse
- **`$EDITOR` not set**: `config edit` falls back to `vi`
