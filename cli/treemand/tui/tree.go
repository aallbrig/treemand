package tui

import (
	"github.com/aallbrig/treemand/cli/treemand/config"
	"github.com/aallbrig/treemand/cli/treemand/models"
)

// TreeModel is the scrollable, filterable tree pane: the row model
// (treeRows: flattening, expansion, filter, cursor and scrolling) and the
// renderer that draws it in the configured display style.
type TreeModel struct {
	*treeRows
	render treeRenderer
	width  int
}

func NewTreeModel(root *models.Node, cfg *config.Config) *TreeModel {
	return &TreeModel{treeRows: newTreeRows(root), render: treeRenderer{cfg: cfg}}
}

func (t *TreeModel) SetSize(w, h int) { t.width = w; t.height = h }

func (t *TreeModel) SetCmdTokens(tokens []string) { t.render.cmdTokens = tokens }
func (t *TreeModel) SetFocused(f bool)            { t.render.focused = f }

// SetDisplayStyle changes the presentation variant.
func (t *TreeModel) SetDisplayStyle(s config.DisplayStyle) { t.render.cfg.TreeStyle = s }

func (t *TreeModel) View() string { return t.ViewSized(t.width, t.height) }

// ViewSized draws the pane at w by h cells; the size also becomes the one
// scrolling and paging use.
func (t *TreeModel) ViewSized(w, h int) string {
	t.SetSize(w, h)
	return t.render.view(t.treeRows, w, h)
}
