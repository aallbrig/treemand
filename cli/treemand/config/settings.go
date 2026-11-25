package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Source says where a setting's effective value comes from.
type Source string

// The sources of a setting, lowest precedence first.
const (
	SourceDefault Source = "default"
	SourceFile    Source = "file"
	SourceEnv     Source = "env"
	SourceFlag    Source = "flag"
)

// Setting is one known config key with its effective value and where that
// value comes from.
type Setting struct {
	SchemaEntry
	Value  string
	Source Source
}

// UnknownKeyError is returned for a key the schema does not know.
type UnknownKeyError struct {
	Key        string
	Suggestion string // the closest known key, or ""
}

func (e *UnknownKeyError) Error() string {
	msg := fmt.Sprintf("unknown config key %q", e.Key)
	if e.Suggestion != "" {
		msg += fmt.Sprintf(" (did you mean %q?)", e.Suggestion)
	}
	return msg
}

// InvalidValueError is returned for a value the key's schema refuses.
type InvalidValueError struct{ Err error }

func (e *InvalidValueError) Error() string { return e.Err.Error() }
func (e *InvalidValueError) Unwrap() error { return e.Err }

// Settings is the config file `treemand config` reads and writes, seen with
// the values flags and the environment set for this run. List and Get read
// the file each time; Set validates a value as `treemand config set` does and
// writes it into the file, keeping its comments, order and unknown keys.
type Settings struct {
	// Path is the config file; it need not exist yet.
	Path string
	// Flags are the values flags set for this run, by key. They win over
	// the environment and the file.
	Flags map[string]string
	// Getenv reads the environment; nil means os.Getenv.
	Getenv func(string) string
}

// List returns every known key, in schema order.
func (s *Settings) List() ([]Setting, error) {
	file, err := s.fileValues()
	if err != nil {
		return nil, err
	}
	out := make([]Setting, 0, len(KnownKeys))
	for _, e := range KnownKeys {
		out = append(out, s.resolve(e, file))
	}
	return out, nil
}

// Get returns one key's setting, or an *UnknownKeyError.
func (s *Settings) Get(key string) (Setting, error) {
	e, err := lookup(key)
	if err != nil {
		return Setting{}, err
	}
	file, err := s.fileValues()
	if err != nil {
		return Setting{}, err
	}
	return s.resolve(e, file), nil
}

// Set validates value for key and writes it to the file. A missing file is
// first created with the commented defaults, as `treemand config init` does.
func (s *Settings) Set(key, value string) error {
	e, err := lookup(key)
	if err != nil {
		return err
	}
	if err := ValidateValue(e, value); err != nil {
		return &InvalidValueError{Err: err}
	}
	if e.Type == TypeBool {
		value = strings.ToLower(value)
	}
	if _, statErr := os.Stat(s.Path); errors.Is(statErr, os.ErrNotExist) {
		if err := WriteDefaultConfig(s.Path, false); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	out, err := setYAML(data, e, value)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.Path, out)
}

func lookup(key string) (SchemaEntry, error) {
	e, ok := LookupKey(key)
	if !ok {
		return SchemaEntry{}, &UnknownKeyError{Key: key, Suggestion: SuggestKey(key)}
	}
	return e, nil
}

// resolve applies flags > env > file > default to one key.
func (s *Settings) resolve(e SchemaEntry, file map[string]string) Setting {
	if v, ok := s.Flags[e.Key]; ok {
		return Setting{SchemaEntry: e, Value: v, Source: SourceFlag}
	}
	getenv := s.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	// viper's AutomaticEnv name for the key (no key replacer is set).
	if v := getenv("TREEMAND_" + strings.ToUpper(e.Key)); v != "" {
		return Setting{SchemaEntry: e, Value: v, Source: SourceEnv}
	}
	if v, ok := file[e.Key]; ok {
		return Setting{SchemaEntry: e, Value: v, Source: SourceFile}
	}
	return Setting{SchemaEntry: e, Value: e.Default, Source: SourceDefault}
}

// fileValues reads the file's scalar values by dot-path key; a missing file
// has none.
func (s *Settings) fileValues() (map[string]string, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	out := map[string]string{}
	if len(doc.Content) > 0 {
		collectScalars("", doc.Content[0], out)
	}
	return out, nil
}

func collectScalars(prefix string, n *yaml.Node, out map[string]string) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := prefix + n.Content[i].Value
		switch v := n.Content[i+1]; v.Kind {
		case yaml.ScalarNode:
			out[key] = v.Value
		case yaml.MappingNode:
			collectScalars(key+".", v, out)
		}
	}
}

// setYAML returns data with e's key set to value, creating the mappings on
// its dot path as needed. Everything else in the document is kept.
func setYAML(data []byte, e SchemaEntry, value string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	n := doc.Content[0]
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("parse config: the top level is not a mapping")
	}
	parts := strings.Split(e.Key, ".")
	for _, p := range parts[:len(parts)-1] {
		n = childMapping(n, p)
		if n == nil {
			return nil, fmt.Errorf("parse config: %q is not a mapping", p)
		}
	}
	leaf := mappingValue(n, parts[len(parts)-1])
	leaf.Kind, leaf.Tag, leaf.Value, leaf.Style = yaml.ScalarNode, scalarTag(e.Type), value, 0
	leaf.Content = nil
	if e.Type == TypeHexColor {
		leaf.Style = yaml.DoubleQuotedStyle
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}
	return buf.Bytes(), nil
}

// mappingValue returns key's value node in m, appending the key if absent.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	v := &yaml.Node{Kind: yaml.ScalarNode}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
	return v
}

// childMapping returns key's mapping in m, creating it when absent or empty,
// and nil when key holds something else.
func childMapping(m *yaml.Node, key string) *yaml.Node {
	v := mappingValue(m, key)
	if v.Kind == yaml.ScalarNode && (v.Tag == "" || v.Tag == "!!null") {
		v.Kind, v.Tag, v.Value = yaml.MappingNode, "!!map", ""
	}
	if v.Kind != yaml.MappingNode {
		return nil
	}
	return v
}

func scalarTag(t KeyType) string {
	switch t {
	case TypeInt:
		return "!!int"
	case TypeBool:
		return "!!bool"
	default:
		return "!!str"
	}
}

// writeFileAtomic replaces path with data, keeping its permissions.
func writeFileAtomic(path string, data []byte) error {
	mode := os.FileMode(0o600)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ApplyLive sets key's value on a running config when a running TUI redraws
// with it (the theme and the layout), and reports whether it did. Other keys
// take effect on the next run.
func ApplyLive(cfg *Config, key, value string) bool {
	switch {
	case key == "tree_style":
		cfg.TreeStyle = ParseTreeStyle(value)
	case key == "icons":
		cfg.IconPreset, cfg.Icons = value, IconSetForPreset(value)
	case strings.HasPrefix(key, "colors."):
		f := colorField(&cfg.Colors, strings.TrimPrefix(key, "colors."))
		if f == nil {
			return false
		}
		*f = value
	default:
		return false
	}
	return true
}

// colorField is the scheme's field for a colors.<name> key, or nil.
func colorField(c *ColorScheme, name string) *string {
	return map[string]*string{
		"base": &c.Base, "subcmd": &c.Subcmd, "flag": &c.Flag,
		"flag_bool": &c.FlagBool, "flag_string": &c.FlagString, "flag_int": &c.FlagInt,
		"flag_other": &c.FlagOther, "pos": &c.Pos, "value": &c.Value,
		"invalid": &c.Invalid, "selected": &c.Selected, "selected_text": &c.SelectedText,
	}[name]
}
