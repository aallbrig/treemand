---
name: treemand-tui-ux
description: Judge the product design of treemand's interactive TUI from frames of real task flows and rank its usability gaps. Use for TUI usability or design questions, a UX review, or before reworking how the TUI behaves.
---

# TUI UX

This is a **judgement** skill: you read frames as a product designer would and rank the gaps. The numbers live with the probes; cite them rather than re-measuring them:

- **Layout** (overflow, frames taller than the terminal, truncated hints): **treemand-tui-layout**.
- **Speed** per key press: **treemand-tui-responsiveness**.
- **Keymap completeness** (keys that are bound but not documented, conflicts between schemes): **treemand-tui-keymap** (#197).
- **Colour contrast** (WCAG ratios, NO_COLOR): **tui-color-contrast**, #198.

When a gap falls in one of those areas, name it, point at that skill or issue, and move on.

## 1. Capture

`task tui:ux` runs `scripts/tui/ux_evidence.sh`. It plays each flow in `scripts/tui/ux_flows.txt` (open, search, expand, flags, assemble, help, error_broken, error_missing) at 80x24, 120x40 and 200x60, inside a private tmux server. The TUI runs from an empty scratch directory with a throwaway HOME, config and cache, so every run is a **first run**. Output goes to `.loop/tui-ux/<UTC date>/`, which is gitignored:

- `<size>/<flow>/<step>.png`: the frame as an image, rendered by vhs.
- `.txt`: the plain text of the frame, for quoting.
- `.ans`: the raw ANSI capture.
- `index.tsv`: the keys pressed before each frame.
- `checks.txt`: deterministic failures (a crash, a panic, an empty frame). The run exits 1 when it has any.

Use `-- --sizes 80x24 --flows "help" --no-png` to iterate on one flow.

**Done when** the run finishes. If `checks.txt` is not empty, that is a **bug** rather than an opinion: reproduce it by hand, then file it as `loop:ready` `area:tui` with the frame and its keys.

## 2. Look

Read the PNGs as images with the Read tool. Start with every frame at 80x24, the size where design debt shows first. Then look at the same steps at 120x40 and 200x60, and judge whether the extra space is used. Use `index.tsv` to know what the user just pressed, and quote text from the `.txt` frame.

Two things to keep straight about the PNGs:

- **Captions.** Each PNG ends with a caption, `[frame N: <size>/<flow>/<step>]`. If the caption names a different step than the file, the render lagged. Re-render with `task tui:ux -- --render-only --out <dir>` before judging that frame.
- **Which frame to trust.** The PNG is the evidence for colour, emphasis and selection. The `.txt` frame is the evidence for layout, because it is tmux's own grid. If the two disagree about where a border or a character sits, the renderer (vhs) is the suspect: say so in the report rather than calling it a TUI bug.

For each frame, ask what a newcomer sees, what they would press next, and whether the screen confirmed their last action.

**Done when** you have viewed every row of `index.tsv` at 80x24 and every flow at all three sizes.

## 3. Score

Apply every rubric item below to every flow. Each item gets a **pass** or one or more **gaps**. Every gap names its evidence: the frame paths and what in them shows the problem. A gap without a frame is a hunch, so either find the frame or drop it.

## 4. Rank and report

Rank the gaps by **cost to the user's task**, highest first:

1. **dead end**: the user cannot finish the task, or loses their work.
2. **misleading**: the screen says something false, or a key does something other than what is shown.
3. **friction**: the task finishes, but only through guessing or extra steps.
4. **polish**.

Within a tier, rank by **reach**: first run and every session come before a rare path.

Write the report to `docs/research/<YYYYMMDD_HHMMSS>_UTC_tui_ux_review.md`. It contains:

- The run's date and commit.
- A ranked table with these columns: rank, gap, rubric item, tier, evidence frames, and a direction (what good looks like, not a patch).
- The passes.
- The gaps you deferred to #197 or #198.

Post the report as **one** comment on the asking issue, headed "for maintainer triage". Opinions become issues only when the maintainer promotes them. You file only deterministic bugs, such as a crash or a frame that a check proves broken.

## Rubric

The audience is a developer who has met an unfamiliar CLI (see `docs/design/*_vision.md`, "A TUI worth keeping open") and wants to find a command and assemble it.

- **Discoverability of keys**: the keys for the next likely action are visible on the screen the user is on. That includes modals, the filter and the help pane, not only the tree. Hints are ranked by usefulness at 80 columns. `?` reaches the full list from anywhere.
- **Feedback for every action**: every key press changes something visible within one frame, such as the cursor, the status line or the command preview. Work that takes time (discovery, lazy expansion) shows progress and an end state. A key that does nothing in the current context says so rather than staying silent.
- **Orientation**: the user can always tell where they are. The breadcrumb, the selected row and the help pane agree with each other. Filtering, clearing a filter, expanding all and closing a modal keep the cursor on the node the user was looking at.
- **Density versus clarity**: at 80x24 the tree is not starved by chrome and truncation keeps the distinguishing part of the text. At 200x60 the space adds information (descriptions, flags) rather than blank area. Repeated or raw text (man-page headers, duplicated flag aliases) earns its rows.
- **First run**: the opening frame says what this is, what is selected, and the first three useful keys. The tree opens to something worth seeing, not one collapsed row.
- **Error states**: a CLI that is missing, has broken help, or yields no commands produces a message that names the problem and the next step. It never produces an empty tree or a raw dump that looks like success.
- **Recovering from mistakes**: every mode has a visible way out (Esc). The user can undo a token, clear the command, and cancel a value prompt without losing other work. Destructive or irreversible keys (Run, quit) are distinct and confirmed.
- **Conventions**: behaviour matches common TUI conventions: vim and less keys (j/k, g/G, /, n/N, q), `?` for help, Tab for focus, and Esc to go back. Any deviation is deliberate and explained on screen. Check behaviour only; key-by-key completeness belongs to #197.
- **Accessibility**: meaning never rests on colour alone; marks and words carry it too. The cursor and selection are visible without colour. Content reads in a narrow terminal and in a screen reader's linear order. Contrast ratios belong to #198.

## Changing the flows

Each line of `scripts/tui/ux_flows.txt` is `flow|target|step|keys`. Every key press runs a real TUI against a real CLI, so the flows stay inside safe keys:

- Close the command modal (C-e) with **Escape**. Enter, r and R there run the command, and c writes the system clipboard.
- Never send d or D, which open a browser.

`task tui:ux:test` covers the script.
