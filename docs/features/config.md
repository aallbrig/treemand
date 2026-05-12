# Configuration Subcommand

The `treemand config` command manages treemand's YAML configuration file.

## Design decisions

See [ADR: Config Subcommand Design](../design/20260512_002317_UTC_adr_config_subcommand_design.md)
for the rationale behind the subcommand-based interface and Go-based schema
validation.

## Source of truth

- **CLI interface** — `cli/treemand/cmd/config.go` (subcommand definitions and RunE handlers)
- **Configuration keys** — `cli/treemand/config/schema.go` (`KnownKeys` registry)
- **Validation logic** — `cli/treemand/config/schema.go` (`ValidateValue`, `ValidateYAML`, `SuggestKey`)
- **Behavior tests** — `cli/treemand/cmd/cmd_test.go` and `cli/treemand/config/schema_test.go`

## Quick reference

```
treemand config view                   # show merged config + file location
treemand config validate [--strict]    # validate config file
treemand config set <key> <value>      # set a config value
treemand config init [--force]         # create default config with comments
treemand config path                   # print config file path
treemand config edit                   # open config in $EDITOR
```

Config file precedence: **CLI flags > env vars (`TREEMAND_*`) > config file > defaults**

Config file locations (searched in order):
1. `$XDG_CONFIG_HOME/treemand/config.yaml` (typically `~/.config/treemand/`)
2. `$HOME/.treemand/config.yaml`
