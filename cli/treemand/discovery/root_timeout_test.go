package discovery

import (
	"context"
	"errors"
	"testing"
	"time"
)

// slowStartCLI hangs on its first two calls (both racing flag forms of the
// root, like kubectl on a loaded runner) and answers --help after that.
const slowStartCLI = `n=$(wc -l <"$CALLS_LOG")
if [ "$n" -le 2 ]; then sleep 3; exit 0; fi
` + flagOnly

func TestDiscover_aTimedOutRootIsRetriedWithALongerBudget(t *testing.T) {
	bin, logPath := formCLI(t, 0, slowStartCLI)
	t.Setenv("CALLS_LOG", logPath)
	d := NewHelpDiscoverer(1)
	d.Timeout = 400 * time.Millisecond
	node, err := d.Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatalf("Discover: %v; calls: %q", err, calls(t, logPath))
	}
	if len(node.Children) != 2 || node.DiscoveryErr != "" {
		t.Errorf("root has %d children (err %q), want alpha and beta after a retry; calls: %q",
			len(node.Children), node.DiscoveryErr, calls(t, logPath))
	}
}

func TestDiscover_aRootThatNeverAnswersIsAnError(t *testing.T) {
	bin, logPath := formCLI(t, 0, "exec sleep 3\n")
	d := NewHelpDiscoverer(1)
	d.Timeout = 200 * time.Millisecond
	start := time.Now()
	node, err := d.Discover(context.Background(), bin, nil)
	if !errors.Is(err, ErrHelpTimedOut) {
		t.Fatalf("Discover = %+v, %v; want ErrHelpTimedOut, not a silent one-node tree; calls: %q",
			node, err, calls(t, logPath))
	}
	if el := time.Since(start); el > 2500*time.Millisecond {
		t.Errorf("took %v; the retry must be bounded by a few command budgets", el)
	}
}

func TestDiscover_aRootPastTheRunDeadlineIsAnError(t *testing.T) {
	bin, _ := formCLI(t, 0, "exec sleep 3\n")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, err := NewHelpDiscoverer(1).Discover(ctx, bin, nil); err == nil {
		t.Error("Discover returned a tree although the run deadline passed before the root answered")
	}
}

func TestDiscover_aRootThatRefusesHelpStillYieldsANode(t *testing.T) {
	bin, _ := formCLI(t, 0, "echo 'unknown flag' >&2\nexit 2\n")
	node, err := NewHelpDiscoverer(1).Discover(context.Background(), bin, nil)
	if err != nil || node == nil {
		t.Fatalf("Discover = %v, %v; a refusal is not a timeout", node, err)
	}
}

func TestDiscover_aTimedOutChildIsNotAnError(t *testing.T) {
	bin, _ := formCLI(t, 0, `case "$1" in alpha) sleep 3; exit 0;; esac
`+flagOnly)
	d := NewHelpDiscoverer(2)
	d.Timeout = 300 * time.Millisecond
	node, err := d.Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatalf("Discover: %v; a slow child must not fail the whole tree", err)
	}
	if len(node.Children) != 2 {
		t.Errorf("root has %d children, want 2", len(node.Children))
	}
}
