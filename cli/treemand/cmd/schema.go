package cmd

import (
	"github.com/spf13/cobra"

	"github.com/aallbrig/treemand/models"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the JSON Schema of the JSON and YAML output",
	Long: `Print the JSON Schema (draft 2020-12) that --output=json and --output=yaml
follow. The root of every tree carries "schema_version". Within a version the
output only gains optional properties; a breaking change ships as a new
version. The schema is also published at https://treemand.dev/schema/tree.v1.json.`,
	Example: `  treemand schema > tree.schema.json
  treemand --output=json git | check-jsonschema --schemafile tree.schema.json -`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := cmd.OutOrStdout().Write(models.SchemaJSON)
		return err
	},
}
