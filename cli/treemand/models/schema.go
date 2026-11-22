package models

import _ "embed"

// SchemaVersion is the version of the JSON Schema the tree output follows. It
// is written as the root's schema_version. Within a version the output only
// gains optional properties; anything that removes, renames or retypes a
// property, or makes a required one optional, is a new version with its own
// schema file.
const SchemaVersion = 1

// SchemaJSON is the JSON Schema (draft 2020-12) for `--output=json` and
// `--output=yaml`, published at https://treemand.dev/schema/tree.v1.json and
// printed by `treemand schema`.
//
//go:embed schema/tree.v1.json
var SchemaJSON []byte
