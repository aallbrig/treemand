package config

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Diagnostic represents a single validation finding.
type Diagnostic struct {
	Level   string // "warning" or "error"
	Key     string
	Message string
}

func (d Diagnostic) String() string {
	if d.Level == "error" {
		return fmt.Sprintf("✗ error: %s", d.Message)
	}
	return fmt.Sprintf("⚠ warning: %s", d.Message)
}

// ValidationResult holds all diagnostics from a config validation.
type ValidationResult struct {
	Diagnostics []Diagnostic
}

// Warnings returns only warning-level diagnostics.
func (r *ValidationResult) Warnings() []Diagnostic {
	var out []Diagnostic
	for _, d := range r.Diagnostics {
		if d.Level == "warning" {
			out = append(out, d)
		}
	}
	return out
}

// Errors returns only error-level diagnostics.
func (r *ValidationResult) Errors() []Diagnostic {
	var out []Diagnostic
	for _, d := range r.Diagnostics {
		if d.Level == "error" {
			out = append(out, d)
		}
	}
	return out
}

// HasErrors reports whether any error-level diagnostics were found.
func (r *ValidationResult) HasErrors() bool {
	for _, d := range r.Diagnostics {
		if d.Level == "error" {
			return true
		}
	}
	return false
}

// PromoteWarnings converts all warnings to errors (for --strict mode).
func (r *ValidationResult) PromoteWarnings() {
	for i := range r.Diagnostics {
		if r.Diagnostics[i].Level == "warning" {
			r.Diagnostics[i].Level = "error"
		}
	}
}

// ValidateYAML parses raw YAML config bytes and returns diagnostics for
// unknown keys and invalid values.
func ValidateYAML(data []byte) (*ValidationResult, error) {
	result := &ValidationResult{}

	// Parse into a generic map to detect unknown keys.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}

	// Flatten the map into dot-path keys.
	flat := flattenMap("", raw)

	// Check each key.
	for key, value := range flat {
		entry, known := LookupKey(key)
		if !known {
			msg := fmt.Sprintf("unknown key %q", key)
			if suggestion := SuggestKey(key); suggestion != "" {
				msg += fmt.Sprintf(" (did you mean %q?)", suggestion)
			}
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Level:   "warning",
				Key:     key,
				Message: msg,
			})
			continue
		}
		// Validate value.
		valStr := fmt.Sprintf("%v", value)
		if err := ValidateValue(entry, valStr); err != nil {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Level:   "error",
				Key:     key,
				Message: err.Error(),
			})
		}
	}

	return result, nil
}

// flattenMap converts a nested map into dot-path keys with leaf values.
func flattenMap(prefix string, m map[string]interface{}) map[string]interface{} {
	flat := make(map[string]interface{})
	for k, v := range m {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		if nested, ok := v.(map[string]interface{}); ok {
			for nk, nv := range flattenMap(fullKey, nested) {
				flat[nk] = nv
			}
		} else {
			flat[fullKey] = v
		}
	}
	return flat
}

// ToYAML renders a Config as YAML suitable for display.
func ToYAML(cfg *Config) (string, error) {
	// Build an ordered representation for clean output.
	m := map[string]interface{}{
		"icons":            cfg.IconPreset,
		"desc_line_length": cfg.DescLineLength,
		"stub_threshold":   cfg.StubThreshold,
		"tree_style":       displayStyleToString(cfg.TreeStyle),
		"no_color":         cfg.NoColor,
		"depth":            cfg.Depth,
		"no_cache":         cfg.NoCache,
		"strategies":       strings.Join(cfg.Strategies, ","),
		"colors": map[string]interface{}{
			"base":          cfg.Colors.Base,
			"subcmd":        cfg.Colors.Subcmd,
			"flag":          cfg.Colors.Flag,
			"flag_bool":     cfg.Colors.FlagBool,
			"flag_string":   cfg.Colors.FlagString,
			"flag_int":      cfg.Colors.FlagInt,
			"flag_other":    cfg.Colors.FlagOther,
			"pos":           cfg.Colors.Pos,
			"value":         cfg.Colors.Value,
			"invalid":       cfg.Colors.Invalid,
			"selected":      cfg.Colors.Selected,
			"selected_text": cfg.Colors.SelectedText,
		},
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func displayStyleToString(s DisplayStyle) string {
	if int(s) < len(DisplayStyleNames) {
		return DisplayStyleNames[s]
	}
	return "default"
}
