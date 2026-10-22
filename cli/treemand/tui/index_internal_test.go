package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/config"
	"github.com/aallbrig/treemand/models"
)

func TestIndexMsg_patchesStubsAndFinishes(t *testing.T) {
	stub := &models.Node{Name: "s3", FullPath: []string{"aws", "s3"}, Stub: true}
	root := &models.Node{Name: "aws", FullPath: []string{"aws"}, Children: []*models.Node{stub}}
	m := NewModel(root, config.DefaultConfig())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.openSearchModal()

	ch := make(chan LazyExpandMsg, 1)
	disc := &models.Node{Description: "Object storage", HelpText: "help", Children: []*models.Node{
		{Name: "mb", FullPath: []string{"aws", "s3", "mb"}, Stub: true},
	}}
	ch <- LazyExpandMsg{Stub: stub, Discovered: disc}
	close(ch)

	// First read yields the result, second reports completion.
	_, cmd := m.Update(indexMsg{ch: ch, msg: <-ch, total: 1})
	if cmd == nil {
		t.Fatal("expected a follow-up command to keep reading the stream")
	}
	if stub.Stub || stub.Description != "Object storage" || stub.HelpText != "help" {
		t.Errorf("stub not patched: %+v", stub)
	}
	if m.tree.nodeExpanded[nodeKey(stub, 1)] {
		t.Error("bulk indexing must not auto-expand nodes")
	}
	m.Update(indexMsg{done: true, total: 1})
	if m.sm.indexing {
		t.Error("indexing flag should clear when done")
	}
	m.sm.input.SetValue("mb")
	m.refreshSearch()
	if len(m.sm.results) == 0 || m.sm.results[0].Entry.Name != "mb" {
		t.Errorf("finder should see indexed child, got %+v", m.sm.results)
	}
}
