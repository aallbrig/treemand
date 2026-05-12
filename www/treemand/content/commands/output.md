---
title: "Output Formats"
weight: 2
---

# Output Formats

treemand can emit the discovered tree as JSON or YAML for scripting, diffing,
and integration with other tools.

<img src="/treemand/demos/cmd_output.gif" alt="treemand JSON/YAML output demo" width="100%">

## Usage

```bash
treemand --output=json git          # full tree as JSON
treemand --output=yaml git          # full tree as YAML
treemand --output=text git          # default colored text tree
```

## JSON schema

```json
{
  "name": "git",
  "description": "the stupid content tracker",
  "flags": [
    {"name": "--version", "value_type": "bool", "description": "Print version"}
  ],
  "positionals": [],
  "children": [
    {
      "name": "commit",
      "description": "Record changes to the repository",
      "flags": [
        {"name": "--message", "short_name": "m", "value_type": "string"}
      ],
      "positionals": [],
      "children": []
    }
  ]
}
```

## Scripting with jq

```bash
# List all top-level subcommands
treemand --output=json --depth=1 git | jq '[.children[].name]'

# Find all flags of a specific subcommand
treemand --output=json git | jq '
  .children[] | select(.name == "commit") | .flags[].name'

# Count flags per subcommand
treemand --output=json --depth=1 kubectl | jq '
  [.children[] | {cmd: .name, flags: (.flags | length)}]'

# Extract commands with descriptions
treemand --output=json --depth=1 docker | jq '
  [.children[] | {name, description}]'
```

## Combine with other tools

```bash
# Store a CLI's schema as YAML
treemand --output=yaml kubectl > kubectl-schema.yaml

# Diff two versions of a CLI's command surface
treemand --output=json --depth=3 aws > aws-before.json
# (upgrade aws cli)
treemand --no-cache --output=json --depth=3 aws > aws-after.json
diff aws-before.json aws-after.json
```
