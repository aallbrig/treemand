package discovery

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDiscover_aTimedOutRootIsRetriedWithALongerBudget(t *testing.T) {
	// The first two calls (both racing flag forms of the root, like kubectl
	// on a loaded runner) hang; it answers --help after that.
	var n atomic.Int32
	cli := &fakeCLI{answer: func(args []string) reply {
		if n.Add(1) <= 2 {
			return reply{delay: 3 * time.Second}
		}
		return flagOnly(args)
	}}
	d := cli.discoverer(1)
	d.Timeout = 400 * time.Millisecond
	node, err := d.Discover(context.Background(), "formcli", nil)
	if err != nil {
		t.Fatalf("Discover: %v; calls: %q", err, cli.calls())
	}
	if len(node.Children) != 2 || node.DiscoveryErr != "" {
		t.Errorf("root has %d children (err %q), want alpha and beta after a retry; calls: %q",
			len(node.Children), node.DiscoveryErr, cli.calls())
	}
}

func TestDiscover_aRootThatNeverAnswersIsAnError(t *testing.T) {
	cli := &fakeCLI{answer: func([]string) reply { return reply{delay: time.Minute} }}
	d := cli.discoverer(1)
	d.Timeout = 200 * time.Millisecond
	start := time.Now()
	node, err := d.Discover(context.Background(), "formcli", nil)
	if !errors.Is(err, ErrHelpTimedOut) {
		t.Fatalf("Discover = %+v, %v; want ErrHelpTimedOut, not a silent one-node tree; calls: %q",
			node, err, cli.calls())
	}
	//nolint:gocritic // timing: only the timeout ends a one-minute call, so a 30s bound proves the retries stop
	if el := time.Since(start); el > 30*time.Second {
		t.Errorf("took %v; the retry must be bounded by a few command budgets", el)
	}
}

func TestDiscover_aRootPastTheRunDeadlineIsAnError(t *testing.T) {
	cli := &fakeCLI{answer: func([]string) reply { return reply{delay: 3 * time.Second} }}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, err := cli.discoverer(1).Discover(ctx, "formcli", nil); err == nil {
		t.Error("Discover returned a tree although the run deadline passed before the root answered")
	}
}

func TestDiscover_aRootThatRefusesHelpStillYieldsANode(t *testing.T) {
	cli := &fakeCLI{answer: func([]string) reply { return refused("unknown flag") }}
	node, err := cli.discoverer(1).Discover(context.Background(), "formcli", nil)
	if err != nil || node == nil {
		t.Fatalf("Discover = %v, %v; a refusal is not a timeout", node, err)
	}
}

func TestDiscover_aTimedOutChildIsNotAnError(t *testing.T) {
	cli := &fakeCLI{answer: func(args []string) reply {
		if firstArg(args) == "alpha" {
			return reply{delay: 3 * time.Second}
		}
		return flagOnly(args)
	}}
	d := cli.discoverer(2)
	d.Timeout = 300 * time.Millisecond
	node, err := d.Discover(context.Background(), "formcli", nil)
	if err != nil {
		t.Fatalf("Discover: %v; a slow child must not fail the whole tree", err)
	}
	if len(node.Children) != 2 {
		t.Errorf("root has %d children, want 2", len(node.Children))
	}
}
