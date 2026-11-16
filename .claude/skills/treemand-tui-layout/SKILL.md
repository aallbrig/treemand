---
name: treemand-tui-layout
description: Measure treemand's TUI layout at several terminal sizes (overflow, frames taller than the terminal, truncated hints, wasted space) and turn anomalies into fixes. Use for TUI layout or visual bugs, or tui-layout findings.
---

# TUI layout

Follow **treemand-measure** for running, recording and filing; this skill is the layout playbook.

**Measure**: `task measure -- tui-layout` renders the real model (`cli/treemand/tools/tuiprobe`) at 80×24, 120×40 and 200×50 in six states (initial, expanded all, help pane off, key overlay, filter open, deep scroll). It counts `overflow_lines` (wider than the terminal), `excess_lines` (frame taller than the terminal: the top scrolls away), `truncated_hints` (status bar cut with …) and `blank_pct` (unused rows in the first frame). `details` names the size and state of every anomaly.

**Act**:

- **Reproduce** the named state in a test first: `TestLayoutFitsTheTerminal` in `tui/scale_test.go` has the states and sizes; give it the probe's tree shape (long descriptions, long flag names) so it goes red.
- **excess or overflow**: a pane's height or width is computed without the space another element takes (status bar, borders, wrapped text). Fix the sizing, not the symptom.
- **truncated hints**: shorten or rank the hints for that width.
- **blank_pct**: a high value at large sizes is layout that does not use the space; judge against the design rubric (`docs/design/*_design_rubric.md`) before acting.

To confirm visually, `task visual:check` captures the TUI at real sizes.

**Prove it**: the probe shows **improved** and the layout test that went red passes.
