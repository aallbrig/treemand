package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/aallbrig/treemand/models"
	"github.com/aallbrig/treemand/models/schemacheck"
)

// `treemand schema` prints the published JSON Schema, byte for byte.
func TestSchema_commandPrintsTheSchema(t *testing.T) {
	res := run(t, 10*time.Second, nil, "schema")
	if res.exit != 0 {
		t.Fatalf("treemand schema exited %d: %s", res.exit, res.stderr)
	}
	if !bytes.Equal([]byte(res.stdout), models.SchemaJSON) {
		t.Errorf("treemand schema output differs from models/schema/tree.v1.json:\n%s", res.stdout)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(res.stdout), &doc); err != nil {
		t.Fatalf("schema output is not JSON: %v", err)
	}
	if doc["$id"] != "https://treemand.dev/schema/tree.v1.json" {
		t.Errorf("$id = %v", doc["$id"])
	}
}

// sameData reports whether a JSON and a YAML document decode to equal values.
func sameData(t *testing.T, jsonDoc, yamlDoc string) bool {
	t.Helper()
	var j, y any
	if err := json.Unmarshal([]byte(jsonDoc), &j); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if err := yaml.Unmarshal([]byte(yamlDoc), &y); err != nil {
		t.Fatalf("YAML: %v", err)
	}
	yj, err := json.Marshal(y) // normalize YAML ints to JSON numbers
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(yj, &y); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(j, y)
}

// fixtureCLI writes a fake CLI that answers its root help with a corpus
// fixture and fails every other call, so children carry discovery errors.
func fixtureCLI(t *testing.T, fixture string) (name, dir string) {
	t.Helper()
	stem := strings.TrimSuffix(filepath.Base(fixture), ".txt")
	name, _, _ = strings.Cut(stem, "@")
	abs, err := filepath.Abs(fixture)
	if err != nil {
		t.Fatal(err)
	}
	dir = t.TempDir()
	script := "#!/bin/sh\ncase \"$*\" in\n--help|-h|help|-help) cat '" + abs + "';;\n*) echo \"unknown command: $*\" >&2; exit 1;;\nesac\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return name, dir
}

// Every corpus CLI's JSON and YAML output, and every framework fake's,
// validates against the published schema.
func TestSchema_outputValidatesForCorpusAndFakes(t *testing.T) {
	fixtures, err := filepath.Glob("../discovery/testdata/corpus/*.txt")
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no corpus fixtures: %v", err)
	}
	fakes := make([]string, 0, len(fakeCLIs))
	for name := range fakeCLIs {
		if name != "fakeslow" { // sleeps on purpose
			fakes = append(fakes, name)
		}
	}
	sort.Strings(fakes)

	type target struct{ label, cli, dir string }
	var targets []target
	for _, f := range fixtures {
		name, dir := fixtureCLI(t, f)
		targets = append(targets, target{"corpus/" + filepath.Base(f), name, dir})
	}
	for _, name := range fakes {
		targets = append(targets, target{"fake/" + name, name, fakeDir})
	}

	for _, tg := range targets {
		t.Run(tg.label, func(t *testing.T) {
			t.Parallel()
			path := "PATH=" + tg.dir + string(os.PathListSeparator) + fakeDir + string(os.PathListSeparator) + os.Getenv("PATH")
			outputs := map[string]string{}
			defer func() {
				if len(outputs) == 2 && !sameData(t, outputs["json"], outputs["yaml"]) {
					t.Errorf("JSON and YAML output hold different data:\njson: %s\nyaml: %s", outputs["json"], outputs["yaml"])
				}
			}()
			for _, format := range []string{"json", "yaml"} {
				res := run(t, 60*time.Second, []string{path}, "--no-cache", "--output="+format, "--depth=2", "--timeout=10", tg.cli)
				if res.exit != 0 {
					t.Fatalf("--output=%s exited %d: %s", format, res.exit, res.stderr)
				}
				outputs[format] = res.stdout
				validate := schemacheck.ValidateJSON
				if format == "yaml" {
					validate = schemacheck.ValidateYAML
				}
				if err := validate([]byte(res.stdout)); err != nil {
					t.Errorf("--output=%s does not match the schema: %v", format, err)
				}
				if !strings.Contains(res.stdout, "schema_version") {
					t.Errorf("--output=%s has no schema_version", format)
				}
			}
		})
	}
}
