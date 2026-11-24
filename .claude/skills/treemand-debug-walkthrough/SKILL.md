---
name: treemand-debug-walkthrough
description: Walk the maintainer through treemand's Go code in GoLand's debugger, stop by stop, to answer a question about how it behaves. Use in an interactive maintainer session when asked to step through, trace or "show me in the debugger" how a code path runs. Not for the dev loop, which debugs with dlv.
---

# Debugger walkthrough

The maintainer asks a question about the code ("how does discovery decide a node is a stub?", "why does this flag end up inherited?"). The answer is a guided tour of real stops in GoLand's debugger: each stop shows where execution is, what the values are and why it went this way. Reading code explains what *could* happen; this shows what *does*.

**Not in the loop.** Loop sessions deny the GoLand tools; the loop debugs with `dlv` ([docs/agents/dev-tooling.md](../../../docs/agents/dev-tooling.md)). If you are the loop, stop here.

## Before the first stop

1. **GoLand is reachable.** The tools are `mcp__goland__*` (deferred: load the `xdebug_*`, `search_symbol`, `get_symbol_info` and `get_repositories` tools with ToolSearch). If the server fails to connect, say so: GoLand must be running with **Settings → Tools → MCP Server** enabled. Offer the same walkthrough with `dlv` instead (dev-tooling.md) rather than giving up.
2. **The right project.** Every tool takes `projectPath`, an open GoLand project root: the developer checkout, or the loop home when the code lives in a loop worktree (then paths are `.loop/worktrees/<n>/...`, and `search_symbol` needs `paths` to avoid one hit per worktree). `get_repositories` with that path confirms it is open.
3. **A plan, said out loud.** Turn the question into a hypothesis and 2-5 stops. Find them with LSP or `get_symbol_info`, not grep. Pick an entry point that reaches them quickly and safely: an existing test first (often `discovery/*_test.go` with a corpus fixture), a new throwaway test second, the CLI itself last, and then only against a fixture or a target CLI that is safe to run (treemand-safe-real-cli-run). Tell the maintainer the plan and adjust it to their interest before starting.

## The walkthrough

1. Set the planned breakpoints with `xdebug_set_breakpoints` (`temporary: true` for one-off stops; conditions to skip uninteresting iterations).
2. Start with `xdebug_start_debugger_session` on the test function's file and line.
3. At each stop, in this order:
   - **Where:** `xdebug_get_stack`, trimmed to the frames that matter, as `file:line` links.
   - **What:** `xdebug_get_frame_values` for the variables the question is about; `xdebug_evaluate_expression` for derived values. Show values, not dumps.
   - **Why:** one or two sentences tying this stop to the question, naming the condition that sent execution here.
   - **Next:** say where the next stop is and why, then `xdebug_control_session` (resume, step) or `xdebug_run_to_line`.
4. Let the maintainer steer: "go into that call", "what's in X", "skip ahead" are the point of doing this live.

Delve can call plain functions in expressions but not methods on receivers that live on the stack, and step-out can land mid-statement; when that happens, say so and set a breakpoint on the next line instead.

## Finish

- A short answer to the original question, citing the stops that showed it.
- Anything surprising becomes an offer: a failing test, an issue, or a GLOSSARY or ADR clarification. Do not file or fix without the maintainer's yes.
- Clean up: stop the session, `xdebug_remove_breakpoints` for the breakpoints you set, and delete any throwaway test.
