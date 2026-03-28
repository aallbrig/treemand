package config_test

import (
	"testing"

	"github.com/aallbrig/treemand/config"
)

func TestKnownKeys(t *testing.T) {
	if len(config.KnownKeys) == 0 {
		t.Fatal("KnownKeys should not be empty")
	}
	// Spot-check a few keys.
	for _, key := range []string{"icons", "depth", "colors.base", "tree_style", "no_color"} {
		if !config.IsKnownKey(key) {
			t.Errorf("expected %q to be a known key", key)
		}
	}
}

func TestLookupKey(t *testing.T) {
	e, ok := config.LookupKey("icons")
	if !ok {
		t.Fatal("LookupKey(icons) not found")
	}
	if e.Type != config.TypeString {
		t.Errorf("icons type = %d, want TypeString", e.Type)
	}
	if len(e.AllowedValues) != 3 {
		t.Errorf("icons allowed values = %v, want 3", e.AllowedValues)
	}
}

func TestValidateValue_string(t *testing.T) {
	e, _ := config.LookupKey("icons")
	if err := config.ValidateValue(e, "ascii"); err != nil {
		t.Errorf("ValidateValue(icons, ascii) = %v, want nil", err)
	}
	if err := config.ValidateValue(e, "invalid"); err == nil {
		t.Error("ValidateValue(icons, invalid) = nil, want error")
	}
}

func TestValidateValue_int(t *testing.T) {
	e, _ := config.LookupKey("depth")
	if err := config.ValidateValue(e, "5"); err != nil {
		t.Errorf("ValidateValue(depth, 5) = %v", err)
	}
	if err := config.ValidateValue(e, "-1"); err != nil {
		t.Errorf("ValidateValue(depth, -1) = %v", err)
	}
	if err := config.ValidateValue(e, "abc"); err == nil {
		t.Error("ValidateValue(depth, abc) = nil, want error")
	}
	if err := config.ValidateValue(e, "999"); err == nil {
		t.Error("ValidateValue(depth, 999) = nil, want error for out-of-range")
	}
}

func TestValidateValue_bool(t *testing.T) {
	e, _ := config.LookupKey("no_color")
	if err := config.ValidateValue(e, "true"); err != nil {
		t.Errorf("ValidateValue(no_color, true) = %v", err)
	}
	if err := config.ValidateValue(e, "false"); err != nil {
		t.Errorf("ValidateValue(no_color, false) = %v", err)
	}
	if err := config.ValidateValue(e, "maybe"); err == nil {
		t.Error("ValidateValue(no_color, maybe) = nil, want error")
	}
}

func TestValidateValue_hexColor(t *testing.T) {
	e, _ := config.LookupKey("colors.base")
	for _, v := range []string{"#FFF", "#FF5555", "#FF5555AA"} {
		if err := config.ValidateValue(e, v); err != nil {
			t.Errorf("ValidateValue(colors.base, %q) = %v", v, err)
		}
	}
	for _, v := range []string{"FF5555", "#GG5555", "red", "#F"} {
		if err := config.ValidateValue(e, v); err == nil {
			t.Errorf("ValidateValue(colors.base, %q) = nil, want error", v)
		}
	}
}

func TestSuggestKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"icons", "icons"},      // exact match
		{"icon", "icons"},       // 1 edit away
		{"depht", "depth"},      // 1 edit away
		{"no_colr", "no_color"}, // 1 edit away
		{"zzzzzzzzzzz", ""},     // no close match
	}
	for _, tt := range tests {
		got := config.SuggestKey(tt.input)
		if tt.want == "" && got != "" {
			t.Errorf("SuggestKey(%q) = %q, want empty", tt.input, got)
		}
		if tt.want != "" && got == "" {
			t.Errorf("SuggestKey(%q) = empty, want a suggestion", tt.input)
		}
	}
}

func TestValidateYAML_valid(t *testing.T) {
	data := []byte(`icons: ascii
depth: 5
colors:
  base: "#FF0000"
`)
	result, err := config.ValidateYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got %d: %v", len(result.Diagnostics), result.Diagnostics)
	}
}

func TestValidateYAML_unknownKeys(t *testing.T) {
	data := []byte(`icons: ascii
unknown_key: something
colours:
  base: "#FF0000"
`)
	result, err := config.ValidateYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	warnings := result.Warnings()
	if len(warnings) < 2 {
		t.Errorf("expected at least 2 warnings for unknown keys, got %d", len(warnings))
	}
}

func TestValidateYAML_invalidValue(t *testing.T) {
	data := []byte(`icons: invalid_preset
`)
	result, err := config.ValidateYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	errors := result.Errors()
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestValidateYAML_invalidYAML(t *testing.T) {
	data := []byte(`[invalid yaml: {{{`)
	_, err := config.ValidateYAML(data)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestPromoteWarnings(t *testing.T) {
	r := &config.ValidationResult{
		Diagnostics: []config.Diagnostic{
			{Level: "warning", Key: "x", Message: "test"},
		},
	}
	r.PromoteWarnings()
	if r.Diagnostics[0].Level != "error" {
		t.Errorf("expected warning promoted to error, got %s", r.Diagnostics[0].Level)
	}
}

func TestToYAML(t *testing.T) {
	cfg := config.DefaultConfig()
	out, err := config.ToYAML(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Error("expected non-empty YAML output")
	}
	// Should contain key names.
	for _, key := range []string{"icons:", "depth:", "colors:"} {
		if !contains(out, key) {
			t.Errorf("YAML output missing %q", key)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && findString(s, substr)
}

func findString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestWriteDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/config.yaml"

	// First write should succeed.
	if err := config.WriteDefaultConfig(path, false); err != nil {
		t.Fatalf("WriteDefaultConfig: %v", err)
	}

	// Second write without force should fail.
	if err := config.WriteDefaultConfig(path, false); err == nil {
		t.Error("expected error for existing file without force")
	}

	// Second write with force should succeed.
	if err := config.WriteDefaultConfig(path, true); err != nil {
		t.Errorf("WriteDefaultConfig(force=true): %v", err)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := config.DefaultConfigPath()
	if path == "" {
		t.Error("DefaultConfigPath returned empty string")
	}
}
