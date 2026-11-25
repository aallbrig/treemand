package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
)

// Offline disables live discovery (lazy stub expansion, re-discovery and
// background indexing). The in-browser demo sets it because it runs from a
// pre-built snapshot and cannot spawn the real CLI.
var Offline bool

const offlineMsg = "demo mode: live discovery needs the installed treemand"

// lazyExpandIfStub checks whether the currently selected node is a stub and,
// if so, returns a tea.Cmd that discovers its children asynchronously.
func (m *Model) lazyExpandIfStub() tea.Cmd {
	sel := m.tree.SelectedItem()
	if sel == nil || sel.Kind != SelCommand || !sel.Node.Stub {
		return nil
	}
	if Offline {
		m.statusMsg = offlineMsg
		return nil
	}
	stub := sel.Node
	args := stub.FullPath[1:] // subcommand path below root

	m.statusMsg = "discovering " + stub.Name + "…"

	exp := m.nodeExpander(m.cfg.StubThreshold)
	return func() tea.Msg {
		result, err := exp.Discover(context.Background(), args)
		return LazyExpandMsg{Stub: stub, Discovered: result, Err: err}
	}
}

// forceExpandSelected re-discovers the currently selected command node
// regardless of whether it is a stub. It uses the same async LazyExpandMsg
// pattern as lazyExpandIfStub so the result patches the live tree.
func (m *Model) forceExpandSelected() tea.Cmd {
	sel := m.tree.SelectedItem()
	if sel == nil || sel.Kind != SelCommand {
		return nil
	}
	if Offline {
		m.statusMsg = offlineMsg
		return nil
	}
	node := sel.Node
	args := node.FullPath[1:] // subcommand path below root

	m.statusMsg = "discovering " + node.Name + "…"

	exp := m.nodeExpander(m.cfg.StubThreshold)
	return func() tea.Msg {
		result, err := exp.Discover(context.Background(), args)
		return LazyExpandMsg{Stub: node, Discovered: result, Err: err, Refresh: true}
	}
}

// nodeExpander returns the expander the TUI discovers single nodes with.
// eagerBelow > 0 discovers a node's subcommands too when it lists at most
// that many; 0 leaves them as stubs.
func (m *Model) nodeExpander(eagerBelow int) *discovery.NodeExpander {
	return discovery.NewNodeExpander(m.root.Name, discovery.ExpandOptions{
		EagerBelow: eagerBelow, MaxDepth: -1, Override: m.override,
	})
}

// applyExpansion merges an expand result into the tree and shows it.
func (m *Model) applyExpansion(msg LazyExpandMsg) {
	exp := m.nodeExpander(0)
	if msg.Refresh {
		exp.Refresh(m.root, msg.Stub, msg.Discovered)
	} else {
		exp.Fill(m.root, msg.Stub, msg.Discovered)
	}
	m.tree.NodeChanged(msg.Stub)
}

// openDocsURL attempts to open a documentation URL for the currently selected
// node. It looks for https:// URLs in the node's description and opens them
// with the system browser. Returns a status message command.
func (m *Model) openDocsURL() tea.Cmd {
	node := m.tree.Selected()
	if node == nil {
		m.statusMsg = "no node selected"
		return nil
	}
	url := extractURL(node.Description)
	if url == "" {
		m.statusMsg = "no docs URL found for " + node.Name
		return nil
	}
	var cmd *exec.Cmd
	switch {
	case isWSL():
		cmd = exec.Command("wslview", url)
	case isMacOS():
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		m.statusMsg = "failed to open browser: " + err.Error()
		return nil
	}
	m.statusMsg = "opened: " + url
	return nil
}

// extractURL returns the first https:// URL found in s, or "".
func extractURL(s string) string {
	const prefix = "https://"
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return ""
	}
	rest := s[idx:]
	// trim at any whitespace or punctuation that terminates a URL
	end := strings.IndexAny(rest, " \t\n\r\"'<>)")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func isMacOS() bool {
	_, err := exec.LookPath("open")
	_, errX := exec.LookPath("xdg-open")
	return err == nil && errX != nil
}

func isWSL() bool {
	_, err := exec.LookPath("wslview")
	return err == nil
}

// indexMsg reports one finished node of a background "index all" run.
type indexMsg struct {
	ch    <-chan LazyExpandMsg
	msg   LazyExpandMsg
	total int
	done  bool
}

const indexWorkers = 16

// indexAllStubs discovers every undiscovered node one level deep (children
// are left as name-only stubs) so the fuzzy finder can see inside them.
func (m *Model) indexAllStubs() tea.Cmd {
	if Offline {
		m.statusMsg = offlineMsg
		return nil
	}
	var stubs []*models.Node
	m.root.Walk(func(n *models.Node) {
		if n.Stub && len(n.FullPath) > 1 {
			stubs = append(stubs, n)
		}
	})
	if len(stubs) == 0 {
		return nil
	}
	exp := m.nodeExpander(discovery.OneLevel)
	ch := make(chan LazyExpandMsg)
	go func() {
		defer close(ch)
		sem := make(chan struct{}, indexWorkers)
		var wg sync.WaitGroup
		for _, stub := range stubs {
			wg.Add(1)
			sem <- struct{}{}
			go func(stub *models.Node) {
				defer wg.Done()
				defer func() { <-sem }()
				res, err := exp.Discover(context.Background(), stub.FullPath[1:])
				ch <- LazyExpandMsg{Stub: stub, Discovered: res, Err: err}
			}(stub)
		}
		wg.Wait()
	}()
	m.sm.indexTotal = len(stubs)
	m.sm.indexed = 0
	m.sm.indexing = true
	return waitIndex(ch, len(stubs))
}

func waitIndex(ch <-chan LazyExpandMsg, total int) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		return indexMsg{ch: ch, msg: msg, total: total, done: !ok}
	}
}

// handleIndexMsg applies one background result and schedules the next read.
func (m *Model) handleIndexMsg(im indexMsg) tea.Cmd {
	if im.done {
		m.sm.indexing = false
		m.tree.Rebuild()
		m.refreshSearch()
		m.statusMsg = fmt.Sprintf("indexed %d commands", im.total)
		return nil
	}
	if im.msg.Err == nil && im.msg.Discovered != nil {
		m.nodeExpander(0).Fill(m.root, im.msg.Stub, im.msg.Discovered)
		m.tree.CountChange()
	}
	m.sm.indexed++
	// Re-rank occasionally so results fill in while the user watches.
	if m.sm.active && m.sm.indexed%10 == 0 {
		m.refreshSearch()
	}
	return waitIndex(im.ch, im.total)
}
