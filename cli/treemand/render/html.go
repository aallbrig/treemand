package render

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/aallbrig/treemand/models"
)

// htmlPage is the self-contained page --output=html writes: inline CSS and
// script, a Content-Security-Policy that forbids every network fetch, and
// <details> lists so the tree collapses even with scripts off.
//
//go:embed html.tmpl
var htmlPage string

var htmlTemplate = template.Must(template.New("page").Parse(htmlPage))

// htmlNode is one visible command, prepared for the template.
type htmlNode struct {
	Name        string
	Command     string
	Description string
	Positionals []string
	Flags       []htmlFlag
	Children    []*htmlNode
	Stub        bool
	Failed      bool
	Open        bool
}

// htmlFlag is one of a command's own flags.
type htmlFlag struct {
	Name        string
	Value       string
	Description string
}

// Expandable reports whether the node gets a <details> disclosure.
func (n *htmlNode) Expandable() bool { return len(n.Children) > 0 || len(n.Flags) > 0 }

type htmlData struct {
	Root     *htmlNode
	Commands int
	Flags    int
}

func (r *Renderer) renderHTML(w io.Writer, root *models.Node) error {
	data := htmlData{}
	data.Root = r.htmlNode(root, 0, &data)
	if data.Root == nil {
		data.Root = &htmlNode{Name: root.Name, Command: root.Name}
	}
	return htmlTemplate.Execute(w, data)
}

// htmlNode converts node and its visible descendants, counting what it keeps.
func (r *Renderer) htmlNode(node *models.Node, depth int, data *htmlData) *htmlNode {
	if !r.visible(node, depth) {
		return nil
	}
	out := &htmlNode{
		Name:        node.Name,
		Command:     node.FullCommand(),
		Description: node.Description,
		Stub:        node.Stub,
		Failed:      node.DiscoveryErr != "",
		Open:        depth == 0,
	}
	if !r.opts.CommandsOnly {
		out.Positionals = positionalNames(node.Positionals)
		out.Flags = ownHTMLFlags(node.Flags)
	}
	data.Commands++
	data.Flags += len(out.Flags)
	for _, child := range node.Children {
		if c := r.htmlNode(child, depth+1, data); c != nil {
			out.Children = append(out.Children, c)
		}
	}
	return out
}

func positionalNames(ps []models.Positional) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		name := "[" + p.Name + "]"
		if p.Required {
			name = "<" + p.Name + ">"
		}
		if p.Variadic {
			name += "…"
		}
		out = append(out, name)
	}
	return out
}

func ownHTMLFlags(flags []models.Flag) []htmlFlag {
	var out []htmlFlag
	for _, f := range flags {
		if f.Inherited {
			continue
		}
		name := f.Name
		if f.ShortName != "" {
			name = dashed(f.ShortName) + ", " + name
		}
		value := ""
		if f.ValueType != "" && f.ValueType != "bool" {
			value = fmt.Sprintf("<%s>", f.ValueType)
		}
		out = append(out, htmlFlag{Name: name, Value: value, Description: f.Description})
	}
	return out
}

// dashed prefixes a bare short flag name ("v") with its dash ("-v").
func dashed(short string) string {
	if strings.HasPrefix(short, "-") {
		return short
	}
	return "-" + short
}
