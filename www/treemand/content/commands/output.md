---
title: "Output Formats: JSON and YAML"
description: "treemand output formats: text, JSON and YAML, with examples for scripts and jq."
weight: 2
---

# Output Formats: JSON and YAML

treemand can emit the discovered tree as JSON or YAML for scripting, diffing,
and integration with other tools.

{{< demo src="demos/cmd_output.gif" alt="treemand JSON/YAML output demo" width="900" height="420" >}}

## Usage

```bash
treemand --output=json git          # full tree as JSON
treemand --output=yaml git          # full tree as YAML
treemand --output=text git          # default colored text tree
```

## JSON schema

The output follows a versioned JSON Schema, published at
[/schema/tree.v1.json](/schema/tree.v1.json) and printed by `treemand schema`.
The root carries `"schema_version": 1`; within a version the output only gains
optional properties. See the [reference](/reference/#json--yaml-schema) for an
example and the full compatibility promise.

```bash
treemand schema > tree.schema.json
treemand --output=json git > git.json && check-jsonschema --schemafile tree.schema.json git.json
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
