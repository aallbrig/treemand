// Package tui implements the interactive Bubble Tea TUI for treemand.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/cli/treemand/config"
	"github.com/aallbrig/treemand/cli/treemand/history"
	"github.com/aallbrig/treemand/cli/treemand/models"
	"github.com/aallbrig/treemand/cli/treemand/render"
	"github.com/aallbrig/treemand/cli/treemand/savedpresets"
)

// NavScheme is the keyboard navigation scheme.
type NavScheme int

const (
	SchemeArrows NavScheme = iota
	SchemeVim
	SchemeWASD
)

// pane identifies which pane currently has focus.
type pane int

const (
	paneTree    pane = 0
	paneHelp    pane = 1
	panePreview pane = 2
	paneCount        = 3
)

// LazyExpandMsg carries the result of an async discovery run for a stub node.
type LazyExpandMsg struct {
	Stub       *models.Node // original stub node (pointer identity for patching)
	Discovered *models.Node // freshly-discovered tree rooted at that node
	Err        error
	// Refresh marks a re-discovery the user asked for: the result replaces
	// what the node held instead of filling only what it lacked.
	Refresh bool
}

// executeModal is the Ctrl+E dialog for running or copying the built command.
type executeModal struct {
	active  bool
	command string
}

// valueInputModal is the inline value-entry dialog for flag/positional rows.
type valueInputModal struct {
	active bool
	label  string // e.g. "--flag-name <string>"
	prefix string // token prefix e.g. "--flag-name=" or ""
	input  textinput.Model
	owner  *models.Node // node this flag/positional belongs to
}

// Model is the root Bubble Tea model.
type Model struct {
	root         *models.Node
	cfg          *config.Config
	scheme       NavScheme
	tree         *TreeModel
	preview      *PreviewModel
	helpPane     *HelpPaneModel
	showHelpPane bool
	filter       textinput.Model
	filtering    bool
	focusedPane  pane
	width        int
	height       int
	statusMsg    string
	timedMsg     string    // shown for a fixed duration (e.g. style name on T press)
	timedMsgExp  time.Time // when timedMsg should clear
	quitting     bool
	modal        *executeModal
	commandToRun string // set when user picks "Run" in the modal
	fm           flagModal
	vm           valueInputModal
	kb           keybindModal        // ? key overlay
	sm           searchModal         // Ctrl+F fuzzy finder
	pendingG     bool                // true after first 'g' press, waiting for second 'g'
	lastSearch   string              // last filter/search term for n/N cycling
	crawl        *crawler            // background discovery, when enabled
	override     func(*models.Node)  // re-applies the CLI's override after a merge
	hist         *history.Store      // command history; nil when off
	rm           recentsModal        // Ctrl+R recent commands
	saved        *savedpresets.Store // saved presets; nil when unavailable
	pm           presetsModal        // Ctrl+O saved presets
	settings     *config.Settings    // the config file; nil when unavailable
	st           settingsModal       // , settings screen
}

// clearTimedMsgMsg is fired by a tea.Tick to clear a timed status message.
type clearTimedMsgMsg struct{ expiry time.Time }

// NewModel creates a new root TUI model.
func NewModel(root *models.Node, cfg *config.Config) *Model {
	filter := textinput.New()
	filter.Placeholder = "filter…"
	filter.CharLimit = 64

	m := &Model{
		root:         root,
		cfg:          cfg,
		tree:         NewTreeModel(root, cfg),
		preview:      NewPreviewModel(cfg),
		helpPane:     NewHelpPaneModel(cfg),
		filter:       filter,
		showHelpPane: true,
		focusedPane:  paneTree,
		modal:        &executeModal{},
	}
	m.tree.SetFocused(true)
	m.preview.SetNode(root)
	m.helpPane.SetNode(root)
	return m
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.EnableMouseAllMotion}
	if m.Crawling() {
		cmds = append(cmds, m.crawl.wait())
	}
	if m.timedMsg != "" {
		cmds = append(cmds, m.timedMsgCmd())
	}
	return tea.Batch(cmds...)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Background index results must never be dropped by a modal, or the
	// worker goroutines would block forever.
	if im, ok := msg.(indexMsg); ok {
		return m, m.handleIndexMsg(im)
	}
	// Same for background discovery results.
	if cb, ok := msg.(crawlBatchMsg); ok {
		return m, m.handleCrawlBatch(cb)
	}

	// An open dialog (or the fuzzy finder) owns the input: keys go to it and
	// clicks behind it are ignored. Everything else (discovery results,
	// resizes, timers) still reaches the model below, because Bubble Tea
	// never redelivers a message that was dropped.
	if dialog := m.activeDialog(); dialog != nil {
		switch km := msg.(type) {
		case tea.KeyMsg:
			return dialog(km)
		case tea.MouseMsg:
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case LazyExpandMsg:
		if msg.Err == nil && msg.Discovered != nil {
			m.applyExpansion(msg)
			m.statusMsg = "expanded: " + msg.Stub.Name
		} else if msg.Err != nil {
			m.statusMsg = "expand failed: " + msg.Err.Error()
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyLayout()
		return m, nil

	case tea.KeyMsg:
		if m.filtering {
			return m.updateFilter(msg)
		}
		if m.focusedPane == panePreview {
			return m.updatePreviewInput(msg)
		}
		return m.updateKeys(msg)

	case tea.MouseMsg:
		return m.updateMouse(msg)

	case clearTimedMsgMsg:
		// Only clear if this tick corresponds to the current expiry (prevents
		// a rapid re-press of T from clearing the new message early).
		if !msg.expiry.Before(m.timedMsgExp) {
			m.timedMsg = ""
		}
		return m, nil
	}
	return m, nil
}

// activeDialog returns the key handler of the open dialog, or nil.
func (m *Model) activeDialog() func(tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.sm.active:
		return m.updateSearchModal
	case m.kb.active:
		return m.updateKeybindModal
	case m.vm.active:
		return m.updateValueModal
	case m.fm.active:
		return m.updateFlagModal
	case m.rm.active:
		return m.updateRecentsModal
	case m.pm.active:
		return m.updatePresetsModal
	case m.st.active:
		return m.updateSettingsModal
	case m.modal.active:
		return m.updateModal
	}
	return nil
}

// ---------- focus management ----------

func (m *Model) cycleFocus(delta int) {
	next := (int(m.focusedPane) + delta + paneCount) % paneCount
	if pane(next) == paneHelp && !m.showHelpPane {
		next = (next + delta + paneCount) % paneCount
	}
	m.setFocus(pane(next))
	m.statusMsg = "focus: " + paneName(pane(next))
}

// setTimedMsg sets a status message that is shown for cfg.StatusMsgTimeout,
// after which the status bar reverts to the normal key-hint text.
func (m *Model) setTimedMsg(msg string) {
	m.timedMsg = msg
	m.timedMsgExp = time.Now().Add(m.cfg.StatusMsgTimeout)
}

// timedMsgCmd returns a tea.Cmd that fires clearTimedMsgMsg after the timeout.
func (m *Model) timedMsgCmd() tea.Cmd {
	exp := m.timedMsgExp
	return tea.Tick(m.cfg.StatusMsgTimeout, func(_ time.Time) tea.Msg {
		return clearTimedMsgMsg{expiry: exp}
	})
}

// TreeModel returns the underlying TreeModel for testing.
func (m *Model) TreeModel() *TreeModel { return m.tree }

// Preview returns the underlying PreviewModel for testing.
func (m *Model) Preview() *PreviewModel { return m.preview }

// SetScheme sets the active navigation scheme.
func (m *Model) SetScheme(s NavScheme) { m.scheme = s }

func (m *Model) setFocus(p pane) {
	m.focusedPane = p
	m.tree.SetFocused(p == paneTree)
	m.preview.SetFocused(p == panePreview)
	m.helpPane.SetFocused(p == paneHelp)
}

func (m *Model) syncSelected() {
	sel := m.tree.SelectedItem()
	if sel == nil {
		return
	}
	switch sel.Kind {
	case SelCommand:
		m.helpPane.SetNode(sel.Node)
	case SelFlag:
		m.helpPane.SetFlagContext(sel.Flag, sel.Owner)
	case SelPositional:
		m.helpPane.SetPositionalContext(sel.Positional, sel.Owner)
	}
}

// ---------- shared helpers ----------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RunOptions configures RunWithOptions.
type RunOptions struct {
	// Save receives the tree on exit when the session discovered anything.
	Save func(*models.Node) error
	// Crawl, when set, discovers pending stubs in the background while the
	// TUI is open (see EnableCrawl).
	Crawl *CrawlOptions
	// History, when set, records commands copied or run from the execute
	// dialog and lists them on Ctrl+R.
	History *history.Store
	// Presets, when set, are the saved presets Ctrl+O lists, loads, saves
	// and deletes.
	Presets *savedpresets.Store
	// Preset, when set, is loaded into the assembled command at start.
	Preset *savedpresets.Preset
	// Settings, when set, is the config file the , settings screen lists
	// and edits.
	Settings *config.Settings
	// Override, when set, re-applies the CLI's override to the tree after
	// discovery fills or refreshes a node.
	Override func(root *models.Node)
}

// RunWithOptions starts the TUI with the given options.
func RunWithOptions(root *models.Node, cfg *config.Config, opts RunOptions) error {
	save := opts.Save
	m := NewModel(root, cfg)
	m.override = opts.Override
	if opts.Crawl != nil {
		m.EnableCrawl(*opts.Crawl)
	}
	m.SetHistory(opts.History)
	m.SetSavedPresets(opts.Presets)
	m.SetSettings(opts.Settings)
	if opts.Preset != nil {
		m.loadStartPreset(*opts.Preset)
	}
	defer func() {
		if m.crawl != nil {
			m.crawl.stop()
		}
	}()
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if fm, ok := finalModel.(*Model); ok && save != nil && fm.tree.Patches() > 0 {
		if serr := save(fm.root); serr != nil {
			fmt.Fprintf(os.Stderr, "treemand: could not save discovered commands to the cache: %v\n", serr)
		}
	}
	if fm, ok := finalModel.(*Model); ok && fm.commandToRun != "" {
		parts := strings.Fields(fm.commandToRun)
		if len(parts) > 0 {
			c := exec.Command(parts[0], parts[1:]...)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		}
	}
	return nil
}

// NodePreview returns a color-coded command preview string.
func NodePreview(node *models.Node, cfg *config.Config) string {
	opts := render.DefaultOptions()
	opts.NoColor = cfg.NoColor
	opts.Colors = cfg.Colors
	opts.MaxDepth = 0
	s, _ := render.ToString(node, opts)
	return strings.TrimSpace(s)
}
