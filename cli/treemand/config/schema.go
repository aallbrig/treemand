package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// KeyType enumerates the value types we support in the config schema.
type KeyType int

const (
	TypeString KeyType = iota
	TypeInt
	TypeBool
	TypeHexColor
)

// SchemaEntry describes a single known configuration key.
type SchemaEntry struct {
	Key           string // dot-path key, e.g. "icons" or "colors.base"
	Type          KeyType
	Default       string
	AllowedValues []string // empty means any value of the correct type is accepted
	Description   string
	MinInt        int // only for TypeInt
	MaxInt        int // only for TypeInt
}

// KnownKeys is the authoritative registry of supported config keys.
var KnownKeys = buildKnownKeys()

// knownKeyMap provides O(1) lookup by key name.
var knownKeyMap = buildKeyMap()

func buildKnownKeys() []SchemaEntry {
	colorKeys := []struct {
		suffix, def, desc string
	}{
		{"base", "#FFFFFF", "Base/root command color"},
		{"subcmd", "#5EA4F5", "Subcommand color"},
		{"flag", "#50FA7B", "Flag color (fallback)"},
		{"flag_bool", "#50FA7B", "Boolean flag color"},
		{"flag_string", "#8BE9FD", "String-value flag color"},
		{"flag_int", "#FFB86C", "Integer-value flag color"},
		{"flag_other", "#BD93F9", "Other typed flag color"},
		{"pos", "#F1FA8C", "Positional argument color"},
		{"value", "#FF79C6", "Value/type annotation color"},
		{"invalid", "#FF5555", "Error/invalid color"},
		{"selected", "#00BFFF", "TUI selection background"},
		{"selected_text", "#000000", "TUI selection foreground"},
	}

	entries := []SchemaEntry{
		{Key: "icons", Type: TypeString, Default: "unicode", AllowedValues: []string{"unicode", "ascii", "nerd"}, Description: "Icon preset for tree rendering"},
		{Key: "desc_line_length", Type: TypeInt, Default: "80", MinInt: 1, MaxInt: 500, Description: "Max description characters before truncation"},
		{Key: "stub_threshold", Type: TypeInt, Default: "50", MinInt: 1, MaxInt: 10000, Description: "Max eager children before creating stubs"},
		{Key: "tree_style", Type: TypeString, Default: "default", AllowedValues: []string{"default", "columns", "compact", "graph"}, Description: "TUI tree presentation style"},
		{Key: "no_color", Type: TypeBool, Default: "false", Description: "Disable colored output"},
		{Key: "depth", Type: TypeInt, Default: "-1", MinInt: -1, MaxInt: 100, Description: "Max tree depth (-1 = unlimited)"},
		{Key: "no_cache", Type: TypeBool, Default: "false", Description: "Disable discovery cache"},
		{Key: "strategies", Type: TypeString, Default: "help", Description: "Comma-separated discovery strategies (help, completions, man)"},
	}

	for _, c := range colorKeys {
		entries = append(entries, SchemaEntry{
			Key:         "colors." + c.suffix,
			Type:        TypeHexColor,
			Default:     c.def,
			Description: c.desc,
		})
	}

	return entries
}

func buildKeyMap() map[string]SchemaEntry {
	m := make(map[string]SchemaEntry, len(KnownKeys))
	for _, e := range KnownKeys {
		m[e.Key] = e
	}
	return m
}

// LookupKey returns the schema entry for a key, if known.
func LookupKey(key string) (SchemaEntry, bool) {
	e, ok := knownKeyMap[key]
	return e, ok
}

// IsKnownKey reports whether key is in the schema.
func IsKnownKey(key string) bool {
	_, ok := knownKeyMap[key]
	return ok
}

// hexColorRe validates hex color strings like #RGB, #RRGGBB, #RRGGBBAA.
var hexColorRe = regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})$`)

// ValidateValue checks whether value is valid for the given schema entry.
// Returns a user-friendly error or nil.
func ValidateValue(entry SchemaEntry, value string) error {
	switch entry.Type {
	case TypeString:
		if len(entry.AllowedValues) > 0 {
			for _, av := range entry.AllowedValues {
				if value == av {
					return nil
				}
			}
			return fmt.Errorf("invalid value %q for key %q: allowed values are %s",
				value, entry.Key, strings.Join(entry.AllowedValues, ", "))
		}
	case TypeInt:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value %q for key %q: expected an integer", value, entry.Key)
		}
		if n < entry.MinInt || n > entry.MaxInt {
			return fmt.Errorf("invalid value %q for key %q: must be between %d and %d",
				value, entry.Key, entry.MinInt, entry.MaxInt)
		}
	case TypeBool:
		v := strings.ToLower(value)
		if v != "true" && v != "false" {
			return fmt.Errorf("invalid value %q for key %q: expected true or false", value, entry.Key)
		}
	case TypeHexColor:
		if !hexColorRe.MatchString(value) {
			return fmt.Errorf("invalid value %q for key %q: expected a hex color like #RRGGBB", value, entry.Key)
		}
	}
	return nil
}

// SuggestKey returns a suggestion for a misspelled key, or "" if no close match.
func SuggestKey(unknown string) string {
	best := ""
	bestDist := 999
	for _, e := range KnownKeys {
		d := levenshtein(unknown, e.Key)
		if d < bestDist && d <= 3 {
			bestDist = d
			best = e.Key
		}
	}
	return best
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = minOf(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func minOf(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
