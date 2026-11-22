// Package schemacheck validates treemand output against the tree schema. Only
// tests import it, so the validator never ships in the binary.
package schemacheck

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"

	"github.com/aallbrig/treemand/models"
)

// Compile parses models.SchemaJSON into a validator.
func Compile() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(models.SchemaJSON))
	if err != nil {
		return nil, err
	}
	const url = "https://treemand.dev/schema/tree.v1.json"
	c := jsonschema.NewCompiler()
	if err := c.AddResource(url, doc); err != nil {
		return nil, err
	}
	return c.Compile(url)
}

// ValidateJSON checks one `--output=json` document.
func ValidateJSON(b []byte) error {
	sch, err := Compile()
	if err != nil {
		return err
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("not JSON: %w", err)
	}
	return sch.Validate(v)
}

// ValidateYAML checks one `--output=yaml` document against the same schema.
func ValidateYAML(b []byte) error {
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("not YAML: %w", err)
	}
	j, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("YAML is not JSON-compatible: %w", err)
	}
	return ValidateJSON(j)
}
