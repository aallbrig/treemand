// Package render provides ASCII/Unicode tree rendering for CLI hierarchies.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
)

// Options controls tree rendering behavior.
type Options struct {
	MaxDepth     int
	Filter       string
	Exclude      string
	CommandsOnly bool
	FullPath     bool
	NoColor      bool
	Output       string // text, json, yaml
	Colors       config.ColorScheme
}

// DefaultOptions returns rendering options with sensible defaults.
func DefaultOptions() Options {
	return Options{
		MaxDepth: -1,
		Output:   "text",
		Colors:   config.DefaultColors(),
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
		return enc.Encode(root)
	case "text", "":
		r.renderNode(w, root, "", true, 0)
		return nil
	default:
		return fmt.Errorf("unknown output format: %s", r.opts.Output)
	}
}

const (
	iconBranch   = "▼ "
	iconLeaf     = "• "
	connLast     = "└── "
	connMid      = "├── "
	connLastPad  = "    "
	connMidPad   = "│   "
)

func (r *Renderer) renderNode(w io.Writer, node *models.Node, prefix string, isLast bool, depth int) {
	if r.opts.MaxDepth >= 0 && depth > r.opts.MaxDepth {
		return
	}
	if r.opts.Exclude != "" && strings.Contains(node.Name, r.opts.Exclude) {
		return
	}
	if r.opts.Filter != "" && !strings.Contains(node.Name, r.opts.Filter) &&
		!r.hasMatchingDescendant(node, r.opts.Filter) {
		return
	}

	// Choose connector
	conn := connMid
	if isLast {
		conn = connLast
	}

	// Choose icon
	icon := iconLeaf
	if len(node.Children) > 0 {
		icon = iconBranch
	}

	// Format the node name
	var namePart string
	switch depth {
	case 0:
		namePart = r.styles.base.Render(node.Name)
	default:
		if r.opts.CommandsOnly || node.IsLeaf() {
			namePart = r.styles.subcmd.Render(node.Name)
		} else {
			namePart = r.styles.subcmd.Render(node.Name)
		}
	}

	// Build inline metadata
	var meta []string
	if !r.opts.CommandsOnly {
		for _, p := range node.Positionals {
			if p.Required {
				meta = append(meta, r.styles.pos.Render("<"+p.Name+">"))
			} else {
				meta = append(meta, r.styles.pos.Render("["+p.Name+"]"))
			}
		}
		if len(node.Flags) > 0 && len(node.Flags) <= 5 {
			var flagStrs []string
			for _, f := range node.Flags {
				fs := r.flagStyle(f.ValueType).Render(f.Name)
				if f.ValueType != "" && f.ValueType != "bool" {
					fs += "=" + r.styles.value.Render("<"+f.ValueType+">")
				}
				flagStrs = append(flagStrs, fs)
			}
			meta = append(meta, "["+strings.Join(flagStrs, ",")+"]")
		} else if len(node.Flags) > 5 {
			meta = append(meta, r.styles.dim.Render(fmt.Sprintf("[%d flags]", len(node.Flags))))
		}
	}

	// Description (dimmed)
	desc := ""
	if node.Description != "" {
		desc = "  " + r.styles.dim.Render(node.Description)
	}

	line := prefix
	if depth > 0 {
		line += conn
	}
	line += icon + namePart
	if len(meta) > 0 {
		line += " " + strings.Join(meta, " ")
	}
	line += desc

	fmt.Fprintln(w, line)

	// Determine padding for children
	childPrefix := prefix
	if depth > 0 {
		if isLast {
			childPrefix += connLastPad
		} else {
			childPrefix += connMidPad
		}
	}

	for i, child := range node.Children {
		r.renderNode(w, child, childPrefix, i == len(node.Children)-1, depth+1)
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

// RenderToString renders the tree to a string.
func RenderToString(root *models.Node, opts Options) (string, error) {
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
