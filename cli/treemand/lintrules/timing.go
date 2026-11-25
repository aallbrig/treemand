//go:build ruleguard

// Package gorules holds the ruleguard rules gocritic runs (.golangci.yml).
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// wallClockBound flags a unit test that compares a measured duration against a
// fixed bound. Such asserts pass alone and fail under the load of a full CI
// run (#335, #477). Prove the behaviour with a deterministic proxy instead: a
// call count, allocations on a larger input, or an endless input that only a
// kill can end. Latency belongs in the perf and tui probes. A test that truly
// must bound time opts out with a reason:
//
//	//nolint:gocritic // timing: <why a count cannot prove it>
//
// e2e tests drive the real binary and are not unit tests, so they are skipped;
// so are sign checks against 0, which measure nothing.
func wallClockBound(m dsl.Matcher) {
	m.Match(`$x > $y`, `$x >= $y`, `$x < $y`, `$x <= $y`,
		`$y > $x`, `$y >= $x`, `$y < $x`, `$y <= $x`).
		Where(m.File().Name.Matches(`_test\.go$`) &&
			!m.File().PkgPath.Matches(`/e2e$`) &&
			m["x"].Type.Is(`time.Duration`) && !m["x"].Const &&
			m["y"].Const && m["y"].Text != `0`).
		At(m["x"]).
		Report(`unit test compares a duration against a fixed bound; wall-clock asserts flake under load: assert a count or allocations, or opt out with //nolint:gocritic // timing: <reason>`)
}
