package config

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// setYAML returns data with e's key set to value. It edits the text, not a
// re-encoded document: an existing value's token is replaced on its line and
// a missing key is inserted under its parent mapping (or appended), so the
// comments, blank lines, order and unknown keys stay byte for byte.
func setYAML(data []byte, e SchemaEntry, value string) ([]byte, error) {
	token, err := scalarToken(e, value)
	if err != nil {
		return nil, err
	}
	root, err := topMapping(data)
	if err != nil {
		return nil, err
	}
	lines := strings.SplitAfter(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	path := strings.Split(e.Key, ".")
	if root == nil {
		return insertKey(lines, len(lines), 0, path, token), nil
	}
	parent := root
	for i, p := range path {
		k, v := mappingEntry(parent, p)
		switch {
		case k == nil:
			return insertKey(lines, lastLine(parent), parent.Content[0].Column-1, path[i:], token), nil
		case i == len(path)-1:
			return replaceToken(lines, e.Key, v, token)
		case isNull(v):
			return insertKey(lines, k.Line, k.Column+1, path[i+1:], token), nil
		case v.Kind != yaml.MappingNode || v.Style&yaml.FlowStyle != 0:
			return nil, fmt.Errorf("cannot set %s: %q in the config is not a block mapping; edit the file by hand", e.Key, p)
		}
		parent = v
	}
	return nil, fmt.Errorf("cannot set %s", e.Key) // unreachable: the loop returns
}

// topMapping returns the document's top-level block mapping, or nil when the
// document is empty (comments only, or a bare ---).
func topMapping(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(doc.Content) == 0 || (isNull(doc.Content[0]) && doc.Content[0].Value == "") {
		return nil, nil
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode || top.Style&yaml.FlowStyle != 0 || len(top.Content) == 0 {
		return nil, fmt.Errorf("parse config: the top level is not a block mapping; edit the file by hand")
	}
	return top, nil
}

// scalarToken is value as YAML text, quoted when it has to be (hex colours
// always are, since # starts a comment).
func scalarToken(e SchemaEntry, value string) (string, error) {
	n := yaml.Node{Kind: yaml.ScalarNode, Tag: scalarTag(e.Type), Value: value}
	if e.Type == TypeHexColor {
		n.Style = yaml.DoubleQuotedStyle
	}
	out, err := yaml.Marshal(&n)
	if err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	token := strings.TrimSuffix(string(out), "\n")
	if strings.Contains(token, "\n") {
		return "", fmt.Errorf("invalid value %q for key %q: must fit on one line", value, e.Key)
	}
	return token, nil
}

func mappingEntry(m *yaml.Node, key string) (k, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}

func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Tag == "!!null"
}

// lastLine is the last line number n or any node under it starts on.
func lastLine(n *yaml.Node) int {
	last := n.Line
	for _, c := range n.Content {
		last = max(last, lastLine(c))
	}
	return last
}

// insertKey inserts path (nested mappings, then the value) after line
// number after, indented by indent spaces.
func insertKey(lines []string, after, indent int, path []string, token string) []byte {
	var add []string
	for j, p := range path {
		line := strings.Repeat(" ", indent+2*j) + p + ":"
		if j == len(path)-1 {
			line += " " + token
		}
		add = append(add, line+"\n")
	}
	if after > 0 && !strings.HasSuffix(lines[after-1], "\n") {
		lines[after-1] += "\n"
	}
	out := append(append(append([]string{}, lines[:after]...), add...), lines[after:]...)
	return []byte(strings.Join(out, ""))
}

// replaceToken swaps v's one-line token for token on v's line.
func replaceToken(lines []string, key string, v *yaml.Node, token string) ([]byte, error) {
	if v.Anchor != "" || (v.Kind != yaml.ScalarNode && v.Kind != yaml.AliasNode) ||
		v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return nil, fmt.Errorf("cannot set %s: its value in the config is an anchor, block or collection; edit the file by hand", key)
	}
	raw := lines[v.Line-1]
	body := strings.TrimRight(raw, "\r\n")
	runes := []rune(body)
	start := v.Column - 1
	end := tokenEnd(runes, start, v.Style)
	if end < 0 {
		return nil, fmt.Errorf("cannot set %s: its value spans lines; edit the file by hand", key)
	}
	lines[v.Line-1] = string(runes[:start]) + token + string(runes[end:]) + raw[len(body):]
	return []byte(strings.Join(lines, "")), nil
}

// tokenEnd is the index just past the scalar token starting at start, or -1
// when a quoted token does not close on the line.
func tokenEnd(r []rune, start int, style yaml.Style) int {
	switch {
	case style&yaml.DoubleQuotedStyle != 0:
		return quoteEnd(r, start, '"', '\\')
	case style&yaml.SingleQuotedStyle != 0:
		return quoteEnd(r, start, '\'', '\'')
	}
	end := len(r)
	if i := strings.Index(string(r[start:]), " #"); i >= 0 {
		end = start + len([]rune(string(r[start:])[:i]))
	}
	for end > start && (r[end-1] == ' ' || r[end-1] == '\t') {
		end--
	}
	return end
}

// quoteEnd is the index just past the closing quote q of the quoted token at
// start, where esc before a character escapes it (” in single quotes), or -1.
func quoteEnd(r []rune, start int, q, esc rune) int {
	for i := start + 1; i < len(r); i++ {
		switch {
		case r[i] == esc && i+1 < len(r) && (esc != q || r[i+1] == q):
			i++
		case r[i] == q:
			return i + 1
		}
	}
	return -1
}
