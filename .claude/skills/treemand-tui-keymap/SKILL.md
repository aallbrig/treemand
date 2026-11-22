---
name: treemand-tui-keymap
description: Measure whether treemand's TUI key bindings match what the ? overlay, footer and dialog hints, and website say (conflicts, dead bindings, undocumented or phantom keys) and turn mismatches into fixes. Use when adding or changing a TUI key, editing key documentation, or for tui-keymap findings.
---

# TUI keymap

Follow **treemand-measure** for running, recording and filing; this skill is the keymap playbook.

**Measure**: `task measure -- tui-keymap` runs `cli/treemand/tools/keymapprobe`. It is static: it reads the `case "k":` labels of every key switch in `cli/treemand/tui/*.go` and the documentation, and runs no TUI. Its mode table (`DefaultConfig`) says which switches make up each mode, in dispatch order (`tree:arrows` = `updateKeys` then `handleArrows`, `help-pane`, `filter`, each dialog). Metrics:

- `conflicts`: a key a later switch in a mode lists but an earlier one always consumes. The later binding never runs.
- `unreachable_bindings`: a string bubbletea never reports. Shift plus a letter arrives as the capital (`"L"`, not `"shift+l"`).
- `undocumented_bindings.overlay` / `.reference`: tree and help-pane keys missing from the ? overlay (`keybindText`) or from the website reference's key tables (`www/treemand/content/reference.md`).
- `undocumented_bindings.dialogs`: dialog, filter and preview keys missing from that mode's own hint. Ratchet only: hints stay short.
- `documented_unbound.overlay` / `.hints` / `.website`: a key the overlay, a hint or a website table names that the mode does not bind. Website columns headed `Keys (vim)` are checked against that scheme only.

`details` gives each finding's key, mode and `file:line`. Documented keys are read as the terminal sends them: `Shift+L` is `"L"`, `gg` is `"g"`.

**Act**:

- **Probe error "not in the mode table"**: a new key switch. Add it to `DefaultConfig` in its mode's dispatch order; don't silence it.
- **conflict**: decide which pane should own the key in that mode, and make the dispatcher route it there (for example, pane keys before global ones when that pane has focus). Test the routing in `tui/tui_test.go`.
- **unreachable**: bind what the terminal sends. If that key already means something else, the binding needs a new key: change code, overlay, footer and website together.
- **undocumented**: document the key in the overlay and in `reference.md`, or remove the binding if it is an accident. `TestEveryTreeKeyIsDocumentedInTheOverlay` in `tui/keys_internal_test.go` guards the tree keys.
- **documented but unbound**: fix the doc, or bind the key if the doc is the intent.

**Prove it**: re-run the probe; the metric shows **improved**. Record on `main` with `--record`.
