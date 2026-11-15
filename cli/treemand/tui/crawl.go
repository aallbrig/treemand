package tui

import (
	"context"
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aallbrig/treemand/discovery"
	"github.com/aallbrig/treemand/models"
)

// CrawlOptions turns on background discovery: the TUI opens on whatever tree
// it was given and fills in pending stubs (see discovery.Pending) while the
// user works, breadth-first, without blocking input.
type CrawlOptions struct {
	MaxDepth      int // -1 = unlimited
	StubThreshold int
	Workers       int
}

// crawlJob is one stub to discover. The worker only reads path, never the
// node: the tree belongs to the UI goroutine.
type crawlJob struct {
	stub   *models.Node
	parent *models.Node // UI goroutine only
	path   []string
}

// crawlMsg is one finished job.
type crawlMsg struct {
	job        crawlJob
	discovered *models.Node
	err        error
}

// crawlBatchMsg delivers every finished job that was waiting, so a burst of
// results costs one update and one redraw, not one each.
type crawlBatchMsg []crawlMsg

// crawler runs discovery jobs on a worker pool. Its queue is unbounded so the
// UI goroutine never blocks when it enqueues a node's children.
type crawler struct {
	opts    CrawlOptions
	c       *discovery.Crawler
	ctx     context.Context
	cancel  context.CancelFunc
	results chan crawlMsg

	mu      sync.Mutex
	queue   []crawlJob
	ready   *sync.Cond // signalled when jobs are queued or the crawler stops
	stopped bool

	// UI-goroutine state.
	outstanding int
	done        int
	started     time.Time
	finished    bool
}

func newCrawler(cliName string, opts CrawlOptions) *crawler {
	if opts.Workers <= 0 {
		opts.Workers = discovery.DefaultMaxConcurrentHelp()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cr := &crawler{
		opts:    opts,
		c:       discovery.NewCrawler(cliName, opts.MaxDepth),
		ctx:     ctx,
		cancel:  cancel,
		results: make(chan crawlMsg, opts.Workers),
		started: time.Now(),
	}
	cr.ready = sync.NewCond(&cr.mu)
	for i := 0; i < opts.Workers; i++ {
		go cr.work()
	}
	return cr
}

func (cr *crawler) work() {
	for {
		job, ok := cr.next()
		if !ok {
			return
		}
		n, err := cr.c.DiscoverOne(cr.ctx, job.path)
		select {
		case cr.results <- crawlMsg{job: job, discovered: n, err: err}:
		case <-cr.ctx.Done():
			return
		}
	}
}

// next blocks until a job is queued or the crawler stops.
func (cr *crawler) next() (crawlJob, bool) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	for len(cr.queue) == 0 && !cr.stopped {
		cr.ready.Wait()
	}
	if cr.stopped {
		return crawlJob{}, false
	}
	job := cr.queue[0]
	cr.queue = cr.queue[1:]
	return job, true
}

// enqueue queues a parent's stubs for discovery. UI goroutine only.
func (cr *crawler) enqueue(parent *models.Node, stubs []*models.Node) {
	if len(stubs) == 0 {
		return
	}
	cr.mu.Lock()
	for _, s := range stubs {
		cr.queue = append(cr.queue, crawlJob{stub: s, parent: parent, path: append([]string(nil), s.FullPath[1:]...)})
	}
	cr.mu.Unlock()
	cr.outstanding += len(stubs)
	cr.ready.Broadcast() // wake every idle worker, not just one
}

// wait returns the command that delivers the next results: it blocks for one,
// then takes whatever else is already waiting.
func (cr *crawler) wait() tea.Cmd {
	return func() tea.Msg {
		var batch crawlBatchMsg
		select {
		case m := <-cr.results:
			batch = append(batch, m)
		case <-cr.ctx.Done():
			return nil
		}
		for {
			select {
			case m := <-cr.results:
				batch = append(batch, m)
			default:
				return batch
			}
		}
	}
}

func (cr *crawler) stop() {
	cr.cancel()
	cr.mu.Lock()
	cr.stopped = true
	cr.mu.Unlock()
	cr.ready.Broadcast()
}

// EnableCrawl turns on background discovery for this model; call before the
// program starts. The pending stubs already in the tree are queued at once.
func (m *Model) EnableCrawl(opts CrawlOptions) {
	if Offline {
		return
	}
	m.crawl = newCrawler(m.root.Name, opts)
	m.root.Walk(func(n *models.Node) {
		var pending []*models.Node
		for _, c := range n.Children {
			if discovery.Pending(c, n, opts.MaxDepth, opts.StubThreshold) {
				pending = append(pending, c)
			}
		}
		m.crawl.enqueue(n, pending)
	})
	if m.crawl.outstanding == 0 {
		m.crawl.finished = true
		m.crawl.stop()
		return
	}
}

// crawlProgress is the status-bar indicator while background discovery runs.
func (m *Model) crawlProgress() string {
	if !m.Crawling() {
		return ""
	}
	return fmt.Sprintf("discovering… %d/%d", m.crawl.done, m.crawl.done+m.crawl.outstanding)
}

// Crawling reports whether background discovery is still running.
func (m *Model) Crawling() bool { return m.crawl != nil && !m.crawl.finished }

// handleCrawl applies one background result.
func (m *Model) handleCrawl(msg crawlMsg) tea.Cmd { return m.handleCrawlBatch(crawlBatchMsg{msg}) }

// handleCrawlBatch applies background results: fills each stub, queues the
// pending stubs they revealed, and redraws once without moving the selection.
func (m *Model) handleCrawlBatch(batch crawlBatchMsg) tea.Cmd {
	cr := m.crawl
	if cr == nil || cr.finished {
		return nil
	}
	changed := false
	sel := m.tree.Selected()
	for _, msg := range batch {
		cr.outstanding--
		cr.done++
		stub := msg.job.stub
		// Navigation may have expanded this stub on demand already.
		if !stub.Stub || msg.err != nil || msg.discovered == nil {
			continue
		}
		if p := msg.job.parent; p != nil && msg.discovered.HelpText != "" && msg.discovered.HelpText == p.HelpText {
			msg.discovered.Children = nil // repeats its parent's help: see discovery.FillUnder
		}
		m.tree.PatchNodeQuiet(stub, msg.discovered)
		changed = true
		var next []*models.Node
		for _, c := range stub.Children {
			if discovery.Pending(c, stub, cr.opts.MaxDepth, cr.opts.StubThreshold) {
				next = append(next, c)
			}
		}
		cr.enqueue(stub, next)
		if sel == stub {
			m.preview.SetNode(stub)
			m.helpPane.SetNode(stub)
		}
	}
	if changed {
		m.rebuildKeepingSelection()
	}
	if cr.outstanding <= 0 {
		cr.finished = true
		cr.stop()
		models.MarkInheritedFlags(m.root)
		m.tree.Rebuild()
		m.setTimedMsg(fmt.Sprintf("discovered %d commands in %.1fs", cr.done, time.Since(cr.started).Seconds()))
		return m.timedMsgCmd()
	}
	return cr.wait()
}

// rebuildKeepingSelection rebuilds the rows and puts the cursor back on the
// item that was selected, so rows filled in above it do not move it.
func (m *Model) rebuildKeepingSelection() {
	m.tree.rebuildKeepingSelection()
}
