// Package render provides ASCII/Unicode tree rendering for CLI hierarchies.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"go.yaml.in/yaml/v3"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
)

// Options controls tree rendering behavior.
type Options struct {
	MaxDepth       int
	Filter         string
	Exclude        string
	CommandsOnly   bool
	FullPath       bool
	NoColor        bool
	Output         string // text, json, yaml, html
	Colors         config.ColorScheme
	Icons          config.IconSet
	DescLineLength int // max runes in a description before truncation
}

// DefaultOptions returns rendering options with sensible defaults.
func DefaultOptions() Options {
	return Options{
		MaxDepth:       -1,
		Output:         "text",
		Colors:         config.DefaultColors(),
		Icons:          config.DefaultIconSet(),
		DescLineLength: 80,
	}
}

// Renderer renders a command tree.
type Renderer struct {
	opts   Options
	styles styles
}

type styles struct {
	base       lipgloss.Style
	subcmd     lipgloss.Style
	flag       lipgloss.Style // bool / fallback
	flagBool   lipgloss.Style
	flagString lipgloss.Style
	flagInt    lipgloss.Style
	flagOther  lipgloss.Style
	pos        lipgloss.Style
	value      lipgloss.Style
	invalid    lipgloss.Style
	dim        lipgloss.Style
}

// New creates a Renderer with the given options.
func New(opts Options) *Renderer {
	r := &Renderer{opts: opts}
	if opts.NoColor {
		r.styles = styles{
			base:       lipgloss.NewStyle(),
			subcmd:     lipgloss.NewStyle(),
			flag:       lipgloss.NewStyle(),
			flagBool:   lipgloss.NewStyle(),
			flagString: lipgloss.NewStyle(),
			flagInt:    lipgloss.NewStyle(),
			flagOther:  lipgloss.NewStyle(),
			pos:        lipgloss.NewStyle(),
			value:      lipgloss.NewStyle(),
			invalid:    lipgloss.NewStyle(),
			dim:        lipgloss.NewStyle(),
		}
	} else {
		r.styles = styles{
			base:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(opts.Colors.Base)),
			subcmd:     lipgloss.NewStyle().Foreground(lipgloss.Color(opts.Colors.Subcmd)),
			flag:       lipgloss.NewStyle().Foreground(lipgloss.Color(opts.Colors.Flag)),
			flagBool:   lipgloss.NewStyle().Foreground(lipgloss.Color(opts.Colors.FlagBool)),
			flagString: lipgloss.NewStyle().Foreground(lipgloss.Color(opts.Colors.FlagString)),
			flagInt:    lipgloss.NewStyle().Foreground(lipgloss.Color(opts.Colors.FlagInt)),
			flagOther:  lipgloss.NewStyle().Foreground(lipgloss.Color(opts.Colors.FlagOther)),
			pos:        lipgloss.NewStyle().Foreground(lipgloss.Color(opts.Colors.Pos)),
			value:      lipgloss.NewStyle().Foreground(lipgloss.Color(opts.Colors.Value)),
			invalid:    lipgloss.NewStyle().Foreground(lipgloss.Color(opts.Colors.Invalid)),
			dim:        lipgloss.NewStyle().Faint(true),
		}
	}
	return r
}

// Render writes the tree to w.
func (r *Renderer) Render(w io.Writer, root *models.Node) error {
	switch r.opts.Output {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(versioned(root))
	case "yaml":
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		return enc.Encode(versioned(root))
	case "html":
		return r.renderHTML(w, root)
	case "text", "":
		r.renderNode(w, root, "", true, 0)
		return nil
	default:
		return fmt.Errorf("unknown output format: %s", r.opts.Output)
	}
}

// versioned is a shallow copy of root stamped with the schema version, so
// machine output says which schema it follows without touching the tree.
func versioned(root *models.Node) *models.Node {
	out := *root
	out.SchemaVersion = models.SchemaVersion
	return &out
}

const (
	connLast    = "└── "
	connMid     = "├── "
	connLastPad = "    "
	connMidPad  = "│   "
)

func (r *Renderer) renderNode(w io.Writer, node *models.Node, prefix string, isLast bool, depth int) {
	if !r.visible(node, depth) {
		return
	}

	line := prefix
	if depth > 0 {
		line += connector(isLast, connLast, connMid)
	}
	line += r.icon(node) + r.name(node, depth)
	if meta := r.meta(node); len(meta) > 0 {
		line += " " + strings.Join(meta, " ")
	}
	line += r.description(node) + r.suffix(node)
	fmt.Fprintln(w, line)

	childPrefix := prefix
	if depth > 0 {
		childPrefix += connector(isLast, connLastPad, connMidPad)
	}
	for i, child := range node.Children {
		r.renderNode(w, child, childPrefix, i == len(node.Children)-1, depth+1)
	}
}

// connector picks the last-child or middle-child form of a tree connector.
func connector(isLast bool, last, mid string) string {
	if isLast {
		return last
	}
	return mid
}

// visible reports whether the depth limit, exclude and filter options keep node.
func (r *Renderer) visible(node *models.Node, depth int) bool {
	if r.opts.MaxDepth >= 0 && depth > r.opts.MaxDepth {
		return false
	}
	if r.opts.Exclude != "" && strings.Contains(node.Name, r.opts.Exclude) {
		return false
	}
	return r.opts.Filter == "" || strings.Contains(node.Name, r.opts.Filter) ||
		r.hasMatchingDescendant(node, r.opts.Filter)
}

func (r *Renderer) icon(node *models.Node) string {
	switch {
	case node.Virtual:
		return r.opts.Icons.Virtual
	case len(node.Children) > 0:
		return r.opts.Icons.Branch
	default:
		return r.opts.Icons.Leaf
	}
}

func (r *Renderer) name(node *models.Node, depth int) string {
	if depth == 0 {
		return r.styles.base.Render(node.Name)
	}
	return r.styles.subcmd.Render(node.Name)
}

// meta builds the inline positionals and own-flag summary shown after a name.
func (r *Renderer) meta(node *models.Node) []string {
	if r.opts.CommandsOnly {
		return nil
	}
	meta := r.positionals(node.Positionals)
	if flags := r.ownFlags(node.Flags); flags != "" {
		meta = append(meta, flags)
	}
	return meta
}

func (r *Renderer) positionals(ps []models.Positional) []string {
	var out []string
	for _, p := range ps {
		if p.Required {
			out = append(out, r.styles.pos.Render("<"+p.Name+">"))
		} else {
			out = append(out, r.styles.pos.Render("["+p.Name+"]"))
		}
	}
	return out
}

// maxInlineFlags is the most own flags listed by name; more show as a count.
const maxInlineFlags = 5

// ownFlags lists a node's non-inherited flags inline, or counts them when
// there are too many to list.
func (r *Renderer) ownFlags(flags []models.Flag) string {
	var own []models.Flag
	for _, f := range flags {
		if !f.Inherited {
			own = append(own, f)
		}
	}
	if len(own) == 0 {
		return ""
	}
	if len(own) > maxInlineFlags {
		return r.styles.dim.Render(fmt.Sprintf("[%d flags]", len(own)))
	}
	strs := make([]string, 0, len(own))
	for _, f := range own {
		fs := r.flagStyle(f.ValueType).Render(f.Name)
		if f.ValueType != "" && f.ValueType != "bool" {
			fs += "=" + r.styles.value.Render("<"+f.ValueType+">")
		}
		strs = append(strs, fs)
	}
	return "[" + strings.Join(strs, ",") + "]"
}

// description is the dimmed description, truncated at the configured length.
func (r *Renderer) description(node *models.Node) string {
	if node.Description == "" {
		return ""
	}
	limit := r.opts.DescLineLength
	if limit <= 0 {
		limit = 80
	}
	d := node.Description
	if runes := []rune(d); len(runes) > limit {
		d = string(runes[:limit]) + "…"
	}
	return "  " + r.styles.dim.Render(d)
}

// suffix marks stub and discovery-error nodes.
func (r *Renderer) suffix(node *models.Node) string {
	switch {
	case node.Stub:
		return "  " + r.styles.dim.Render("(…)")
	case node.DiscoveryErr != "":
		return "  " + r.styles.dim.Render("(?)")
	default:
		return ""
	}
}

func (r *Renderer) hasMatchingDescendant(node *models.Node, filter string) bool {
	for _, child := range node.Children {
		if strings.Contains(child.Name, filter) {
			return true
		}
		if r.hasMatchingDescendant(child, filter) {
			return true
		}
	}
	return false
}

// ToString renders the tree to a string.
func ToString(root *models.Node, opts Options) (string, error) {
	var sb strings.Builder
	r := New(opts)
	if err := r.Render(&sb, root); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// Stats returns a count of nodes in the tree.
type Stats struct {
	Commands int
	Flags    int
	MaxDepth int
}

// Collect gathers stats from a tree.
func Collect(root *models.Node) Stats {
	var s Stats
	collectStats(root, 0, &s)
	return s
}

func collectStats(node *models.Node, depth int, s *Stats) {
	s.Commands++
	s.Flags += len(node.Flags)
	if depth > s.MaxDepth {
		s.MaxDepth = depth
	}
	for _, child := range node.Children {
		collectStats(child, depth+1, s)
	}
}

// flagStyle returns the lipgloss style for a flag based on its value type.
func (r *Renderer) flagStyle(valueType string) lipgloss.Style {
	switch valueType {
	case "bool", "":
		return r.styles.flagBool
	case "string", "stringArray", "[]string":
		return r.styles.flagString
	case "int", "int64", "uint", "uint64", "count":
		return r.styles.flagInt
	default:
		return r.styles.flagOther
	}
}
