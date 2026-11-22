---
name: treemand-tui-color-contrast
description: Measure the WCAG contrast of every colour pair treemand's TUI draws, on dark and light terminals, and its NO_COLOR compliance, and turn breaches into colour fixes. Use for TUI colour, theme, readability or accessibility questions, NO_COLOR bugs, or tui-contrast findings.
---

# TUI colour contrast

Follow **treemand-measure** for running, recording and filing; this skill is the colour playbook.

**Measure**: `task measure -- tui-contrast` renders the real model (`cli/treemand/tools/tuicontrast`) at 120×40 in fourteen states (tree rows, the four display styles, status bar and messages, key overlay, filter, search, flag picker, exec modal, pane focus) and reads every colour from the frames' truecolour escape codes, so hard-coded styles count as much as `config.DefaultColors`. Where the TUI leaves a colour to the terminal it is judged twice: `dark` (#D4D4D4 on #1E1E1E) and `light` (#000000 on #FFFFFF). Faint text is its colour at half intensity toward the background.

- `pairs_below_4_5.<terminal>`: distinct text pairs under 4.5:1 (WCAG 1.4.3).
- `pairs_below_3.<terminal>`: bold text and borders under 3:1 (bold is a terminal's only "large" text; borders are UI, WCAG 1.4.11).
- `no_color_env_coloured_states` / `no_color_flag_coloured_states`: states that still emit colour with `NO_COLOR` set, or with `--no-color` / `no_color: true`.
- `pairs_total.<terminal>` is context, not judged.

Each failing pair in `details` carries `fg`, `bg`, `ratio`, `min`, `source` (what set each colour: a hex, the terminal default, faint, bold), up to three `examples` (state, body or status_bar, text), and `suggest`: the nearest colour of the same hue that passes on that background. Hex values can be off by one from the source because the renderer rounds; grep the neighbourhood.

**Act**:

- **One family, one issue**: file `pairs_below_*` per terminal as one issue. The fix is usually one style, not one pair.
- **A colour that fails only on light**: the TUI hard-codes dark-theme hex. Use `lipgloss.AdaptiveColor{Light: <suggest on light>, Dark: <current>}`; never darken it for everyone.
- **Faint text failing**: faint is a terminal's guess. Replace `Faint(true)` with an explicit adaptive grey that passes on both (the `suggest` values for each terminal).
- **A pair with an explicit bg** (the selected row, a modal selection): an inner style's reset breaks the outer background, or an inner foreground overrides the selected text colour. Fix the composition so the selected row uses `SelectedText` throughout.
- **NO_COLOR**: the environment variable is honoured by lipgloss; `cfg.NoColor` must be too (for example `lipgloss.SetColorProfile(termenv.Ascii)` before the TUI starts). Bold and faint stay: NO_COLOR is about colour only.
- **A new theme or built-in colour scheme**: add it to `themes` in `tools/tuicontrast/main.go` so the probe judges it.

**Prove it**: the probe shows **improved** for the metric you fixed; record it with `--record`.
