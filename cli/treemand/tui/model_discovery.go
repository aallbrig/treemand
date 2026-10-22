package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
)

// lazyExpandIfStub checks whether the currently selected node is a stub and,
// if so, returns a tea.Cmd that discovers its children asynchronously.
func (m *Model) lazyExpandIfStub() tea.Cmd {
	sel := m.tree.SelectedItem()
	if sel == nil || sel.Kind != SelCommand || !sel.Node.Stub {
		return nil
	}
	stub := sel.Node
	stubThreshold := m.cfg.StubThreshold
	cliName := m.root.Name
	args := stub.FullPath[1:] // subcommand path below root

	m.statusMsg = "discovering " + stub.Name + "…"

	return func() tea.Msg {
		d := discovery.NewHelpDiscoverer(1) // one level deep for the stub
		d.StubThreshold = stubThreshold
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := d.Discover(ctx, cliName, args)
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
	node := sel.Node
	stubThreshold := m.cfg.StubThreshold
	cliName := m.root.Name
	args := node.FullPath[1:] // subcommand path below root

	m.statusMsg = "discovering " + node.Name + "…"

	return func() tea.Msg {
		d := discovery.NewHelpDiscoverer(1)
		d.StubThreshold = stubThreshold
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := d.Discover(ctx, cliName, args)
		return LazyExpandMsg{Stub: node, Discovered: result, Err: err}
	}
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
		cmd = exec.Command("wslview", url) //nolint:gosec
	case isMacOS():
		cmd = exec.Command("open", url) //nolint:gosec
	default:
		cmd = exec.Command("xdg-open", url) //nolint:gosec
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
	var stubs []*models.Node
	m.root.Walk(func(n *models.Node) {
		if n.Stub && len(n.FullPath) > 1 {
			stubs = append(stubs, n)
		}
	})
	if len(stubs) == 0 {
		return nil
	}
	cliName := m.root.Name
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
				d := discovery.NewHelpDiscoverer(1)
				d.StubThreshold = 1 // children stay name-only stubs
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				res, err := d.Discover(ctx, cliName, stub.FullPath[1:])
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
		m.tree.PatchNodeQuiet(im.msg.Stub, im.msg.Discovered)
	}
	m.sm.indexed++
	// Re-rank occasionally so results fill in while the user watches.
	if m.sm.active && m.sm.indexed%10 == 0 {
		m.refreshSearch()
	}
	return waitIndex(im.ch, im.total)
}
