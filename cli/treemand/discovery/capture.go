package discovery

import (
	"context"
	"os/exec"
	"sync"
)

// MaxHelpBytes is the most output read from one help probe (1MB: about ten
// times the largest real help text seen). A CLI that floods stdout (a bug, or
// `yes`) is stopped once it is reached, so memory and time stay bounded;
// whatever was read is still parsed.
var MaxHelpBytes = 1 << 20

// OnOutputTruncated, when set, is told about every probe stopped at the cap
// (cmd wires it to the debug log).
var OnOutputTruncated func(name string, args []string, limit int)

// cappedBuffer collects up to limit bytes and then asks for the process to be
// stopped. It reports every write as consumed so the child is never blocked
// on a full pipe while it is being killed.
type cappedBuffer struct {
	mu        sync.Mutex
	buf       []byte
	limit     int
	truncated bool
	onFull    func()
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if room := c.limit - len(c.buf); room > 0 {
		if len(p) <= room {
			c.buf = append(c.buf, p...)
			return len(p), nil
		}
		c.buf = append(c.buf, p[:room]...)
	}
	if !c.truncated {
		c.truncated = true
		c.onFull()
	}
	return len(p), nil
}

// runCapped runs name with args and returns its combined stdout and stderr,
// at most limit bytes of it. When the cap is hit the process is killed and the
// prefix read so far is returned with truncated set. extraEnv entries are
// appended to the environment (nil means inherit as is).
func runCapped(ctx context.Context, name string, args, extraEnv []string, limit int) (out []byte, truncated bool, err error) {
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...) //nolint:gosec
	if extraEnv != nil {
		cmd.Env = append(cmd.Environ(), extraEnv...)
	}
	cmd.WaitDelay = killWaitDelay
	cb := &cappedBuffer{limit: limit, onFull: cancel}
	cmd.Stdout = cb
	cmd.Stderr = cb
	err = cmd.Run()
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.truncated && OnOutputTruncated != nil {
		OnOutputTruncated(name, args, limit)
	}
	return cb.buf, cb.truncated, err
}
