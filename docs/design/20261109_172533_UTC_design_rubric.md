# Design rubric

> Human-owned, like the vision. The dev loop scores screenshots and TUI captures
> from `task visual:check` against this rubric and treats a failing score as a
> merge blocker for UI-facing changes. Draft: correct it.

Score each item pass / fail with a one-line reason. Any fail blocks the merge
unless the change is the fix for that failure.

## Website (desktop 1280 and mobile 390)

- **Hierarchy**: one clear primary message and action above the fold.
- **Spacing**: consistent rhythm; no cramped or floating elements.
- **Type**: at most two families; readable body size (>= 16px mobile); clear scale.
- **Colour & contrast**: WCAG AA text contrast; accent used sparingly; dark mode
  is not broken if present.
- **Responsive**: no horizontal scroll at 390px; tap targets >= 44px; nav usable.
- **Consistency**: buttons, code blocks, headings look the same on every page.
- **Polish**: no broken images, no layout shift, no lorem/placeholder text.

## TUI (80x24 and 120x40)

- **Fits**: nothing clipped or overflowing at 80x24; layout uses extra space at 120x40.
- **Legible**: selected item, focus and key hints always visible; colours distinct.
- **Orientation**: the user always knows where they are in the tree and what to press next.
- **No artefacts**: no stray escape codes, flicker, or half-drawn borders.
- **Consistent**: key bindings and wording match the rest of the TUI and the docs.

## CLI output

- Aligned, truncates gracefully on narrow terminals, `--icons=ascii` stays clean.
