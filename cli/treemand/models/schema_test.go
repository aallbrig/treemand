package models_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/aallbrig/treemand/cli/treemand/models"
	"github.com/aallbrig/treemand/cli/treemand/models/schemacheck"
)

// publishedSchema is the copy the website serves at the schema's $id.
const publishedSchema = "../../../www/treemand/static/schema/tree.v1.json"

type schemaDef struct {
	Type       string                    `json:"type"`
	Required   []string                  `json:"required"`
	Properties map[string]map[string]any `json:"properties"`
}

type schemaDoc struct {
	ID   string               `json:"$id"`
	Defs map[string]schemaDef `json:"$defs"`
}

func loadSchema(t *testing.T) schemaDoc {
	t.Helper()
	var doc schemaDoc
	if err := json.Unmarshal(models.SchemaJSON, &doc); err != nil {
		t.Fatalf("schema is not JSON: %v", err)
	}
	return doc
}

// propType is the JSON type a schema property declares: "array", "object",
// "string", ... A $ref counts as the referenced definition ("#/$defs/flag").
func propType(p map[string]any) string {
	if ref, ok := p["$ref"].(string); ok {
		return ref
	}
	typ, _ := p["type"].(string)
	if typ == "array" {
		items, _ := p["items"].(map[string]any)
		return "array of " + propType(items)
	}
	return typ
}

// goType is the JSON type encoding/json produces for a Go field type.
func goType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Pointer:
		return goType(t.Elem())
	case reflect.Slice:
		return "array of " + goType(t.Elem())
	case reflect.Struct:
		return "#/$defs/" + strings.ToLower(t.Name())
	}
	return "unsupported " + t.String()
}

// checkField compares one struct field with the schema definition it belongs
// to and returns the key it serializes as ("" when it has none).
func checkField(t *testing.T, defName string, def schemaDef, owner string, f reflect.StructField) string {
	t.Helper()
	jsonKey, jsonOpts, _ := strings.Cut(f.Tag.Get("json"), ",")
	if jsonKey == "-" || jsonKey == "" {
		t.Errorf("%s.%s has no json tag", owner, f.Name)
		return ""
	}
	if got := f.Tag.Get("yaml"); got != f.Tag.Get("json") {
		t.Errorf("%s.%s: yaml tag %q, want it equal to the json tag %q", owner, f.Name, got, f.Tag.Get("json"))
	}
	p, ok := def.Properties[jsonKey]
	if !ok {
		t.Errorf("%s.%s serializes as %q, which the schema does not describe", owner, f.Name, jsonKey)
		return jsonKey
	}
	if got, want := propType(p), goType(f.Type); got != want {
		t.Errorf("$defs.%s.%s: schema type %q, model type %q", defName, jsonKey, got, want)
	}
	if _, ok := p["description"]; !ok {
		t.Errorf("$defs.%s.%s has no description", defName, jsonKey)
	}
	omitempty := strings.Contains(jsonOpts, "omitempty")
	if required := slices.Contains(def.Required, jsonKey); required == omitempty {
		t.Errorf("$defs.%s.%s: required=%v but the field is omitempty=%v", defName, jsonKey, required, omitempty)
	}
	return jsonKey
}

// The schema describes exactly the fields the model serializes, with the same
// types, and YAML uses the same keys as JSON.
func TestSchema_matchesModel(t *testing.T) {
	doc := loadSchema(t)
	for _, typ := range []reflect.Type{
		reflect.TypeFor[models.Node](),
		reflect.TypeFor[models.Flag](),
		reflect.TypeFor[models.Positional](),
	} {
		name := strings.ToLower(typ.Name())
		def, ok := doc.Defs[name]
		if !ok {
			t.Errorf("schema has no $defs.%s", name)
			continue
		}
		seen := map[string]bool{}
		for i := range typ.NumField() {
			seen[checkField(t, name, def, typ.Name(), typ.Field(i))] = true
		}
		for key := range def.Properties {
			if !seen[key] {
				t.Errorf("schema describes $defs.%s.%s, which the model never writes", name, key)
			}
		}
	}
}

// v1Surface is the frozen contract of schema version 1: every property with
// its type, and which properties are always present. Removing, renaming or
// retyping one, or making a required one optional, breaks consumers; doing so
// needs SchemaVersion 2 and a new tree.v2.json, never an edit of v1.
var v1Surface = map[string]map[string]string{
	"node": {
		"schema_version": "integer",
		"name":           "string!",
		"full_path":      "array of string!",
		"description":    "string",
		"flags":          "array of #/$defs/flag",
		"positionals":    "array of #/$defs/positional",
		"children":       "array of #/$defs/node",
		"help_text":      "string",
		"discovered":     "boolean!",
		"discovery_err":  "string",
		"stub":           "boolean",
		"virtual":        "boolean",
	},
	"flag": {
		"name":        "string!",
		"short_name":  "string",
		"value_type":  "string",
		"description": "string",
		"required":    "boolean",
		"inherited":   "boolean",
	},
	"positional": {
		"name":        "string!",
		"description": "string",
		"required":    "boolean!",
		"variadic":    "boolean",
	},
}

// breakingChange describes how a schema property breaks its v1 promise, or
// returns "". A trailing "!" marks a property that is always present.
func breakingChange(def schemaDef, key, want string) string {
	p, ok := def.Properties[key]
	if !ok {
		return "was removed"
	}
	got := propType(p)
	if slices.Contains(def.Required, key) {
		got += "!"
	}
	// Optional properties may become required; nothing else may change.
	if got == want || got == want+"!" {
		return ""
	}
	return fmt.Sprintf("is %q, v1 promised %q", got, want)
}

// A breaking change to the schema cannot ship under the same version.
func TestSchema_v1HasNoBreakingChange(t *testing.T) {
	if models.SchemaVersion != 1 {
		t.Skipf("schema version is %d; v1 is frozen in tree.v1.json", models.SchemaVersion)
	}
	doc := loadSchema(t)
	for defName, props := range v1Surface {
		for key, want := range props {
			if why := breakingChange(doc.Defs[defName], key, want); why != "" {
				t.Errorf("breaking change: $defs.%s.%s %s; bump SchemaVersion and add tree.v2.json", defName, key, why)
			}
		}
	}
}

func TestSchema_compilesAndIsVersioned(t *testing.T) {
	doc := loadSchema(t)
	if want := "https://treemand.dev/schema/tree.v1.json"; doc.ID != want {
		t.Errorf("$id = %q, want %q", doc.ID, want)
	}
	if _, err := schemacheck.Compile(); err != nil {
		t.Fatalf("schema does not compile: %v", err)
	}
}

// The website serves the same bytes the binary prints.
func TestSchema_publishedCopyIsCurrent(t *testing.T) {
	b, err := os.ReadFile(publishedSchema)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, models.SchemaJSON) {
		t.Errorf("%s differs from models/schema/tree.v1.json; copy it over", publishedSchema)
	}
}

func TestSchema_validatesAndRejects(t *testing.T) {
	sch, err := schemacheck.Compile()
	if err != nil {
		t.Fatal(err)
	}
	valid := `{"schema_version":1,"name":"git","full_path":["git"],"discovered":true,
	  "flags":[{"name":"--version","value_type":"bool"}],
	  "positionals":[{"name":"path","required":false}],
	  "children":[{"name":"commit","full_path":["git","commit"],"discovered":false,"stub":true}]}`
	invalid := map[string]string{
		"missing name":      `{"full_path":["git"],"discovered":true}`,
		"full_path is null": `{"name":"git","full_path":null,"discovered":true}`,
		"bad child":         `{"name":"git","full_path":["git"],"discovered":true,"children":[{"name":"x"}]}`,
		"flag without name": `{"name":"git","full_path":["git"],"discovered":true,"flags":[{"short_name":"-v"}]}`,
	}
	check := func(doc string) error {
		v, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
		if err != nil {
			t.Fatal(err)
		}
		return sch.Validate(v)
	}
	if err := check(valid); err != nil {
		t.Errorf("valid tree rejected: %v", err)
	}
	names := make([]string, 0, len(invalid))
	for n := range invalid {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if check(invalid[n]) == nil {
			t.Errorf("%s: invalid tree accepted", n)
		}
	}
}
