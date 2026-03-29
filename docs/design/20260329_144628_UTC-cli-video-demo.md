# Design: CLI Feature Demo Video System

**Date:** 2026-03-29
**Status:** Accepted

## Problem

CLI tools need visual documentation that shows features in action. Static
screenshots go stale, hand-recorded screencasts are time-consuming and
inconsistent, and there is no automated way to detect when the video needs
re-recording after code changes.

## Solution

Use [VHS](https://github.com/charmbracelet/vhs) (Charmbracelet's terminal
recorder) to script deterministic demo recordings as `.tape` files. Individual
feature segments are recorded as separate tapes, then stitched into a single
MP4 video. A nightly CI job detects code changes and regenerates the video
automatically.

---

## Part 1: General Design (CLI-Agnostic)

This section describes the system architecture independent of any specific CLI.
It can be reused for any CLI project that wants automated VHS-based demo videos.

### Architecture

```
FEATURES.md           # Canonical list of features to demo
    |
    v
demos/                # Directory of .tape files
  _settings.tape      # Shared VHS settings (included by all tapes)
  01_feature_a.tape   # One tape per feature segment
  02_feature_b.tape
  ...
  NN_feature_z.tape
    |
    v
scripts/
  record-demo.sh      # Orchestrator: records each tape, stitches into one MP4
    |
    v
dist/
  demo.mp4            # Final combined video
```

### Tape File Conventions

#### Shared Settings (`_settings.tape`)

A single file defines visual settings reused across all tapes. Individual tapes
do NOT set `Output` — the orchestrator script handles output paths.

```tape
Set Shell bash
Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set PlaybackSpeed 1.0
Set TypingSpeed 50ms
Set Padding 20
```

#### Feature Tapes (`NN_<name>.tape`)

Each tape file records one feature segment. Naming convention:
- Two-digit prefix for ordering: `01_`, `02_`, etc.
- Descriptive snake_case name after the prefix
- Each tape starts with a **section label** displayed via a printed banner so
  the viewer knows what feature they are watching

Pattern for a feature tape:

```tape
Require <cli-binary>

Source demos/_settings.tape

Output demos/segments/NN_<name>.mp4

# ── Feature: <Human-Readable Feature Name> ──
Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── <Feature Name> ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "<demo command>"
Sleep 300ms
Enter
Sleep 3s

Sleep 500ms
```

Key conventions:
- **Banner labels**: Use ANSI escape codes to print a bold, colored section
  header before each demo so the viewer can identify the feature in the video.
- **Pacing**: `Sleep 300ms` before Enter (lets viewer read the command),
  `Sleep 3s` after (lets viewer read the output), `Sleep 500ms` trailing
  (breathing room before next segment).
- **`Hide`/`Show`**: Hide the `clear` command so segment transitions are clean.
- **`Require`**: Ensures the CLI binary is available before recording.

### Orchestrator Script (`scripts/record-demo.sh`)

The script:

1. Records each `demos/NN_*.tape` file individually via `vhs` to produce
   per-segment MP4 files in `demos/segments/`.
2. Uses `ffmpeg` to concatenate all segment MP4s (in filename order) into a
   single `dist/demo.mp4`.
3. Cleans up intermediate segment files (optional, controlled by flag).

```bash
#!/usr/bin/env bash
set -euo pipefail

DEMOS_DIR="demos"
SEGMENTS_DIR="demos/segments"
OUTPUT="dist/demo.mp4"

mkdir -p "$SEGMENTS_DIR" dist

# Step 1: Record each segment
for tape in "$DEMOS_DIR"/[0-9][0-9]_*.tape; do
  echo "Recording: $tape"
  vhs "$tape"
done

# Step 2: Build ffmpeg concat list
CONCAT_FILE=$(mktemp)
for seg in "$SEGMENTS_DIR"/[0-9][0-9]_*.mp4; do
  echo "file '$(realpath "$seg")'" >> "$CONCAT_FILE"
done

# Step 3: Concatenate
ffmpeg -y -f concat -safe 0 -i "$CONCAT_FILE" -c copy "$OUTPUT"
rm "$CONCAT_FILE"

echo "Demo video: $OUTPUT"
```

Requirements: `vhs` (which bundles `ffmpeg` + `ttyd` + Chromium) must be
installed. On CI, use the official VHS Docker image or install via `go install`.

### Makefile Integration

```makefile
## demo: record all VHS tapes and produce dist/demo.mp4
demo:
	bash scripts/record-demo.sh

## demo-clean: remove recorded segments and final video
demo-clean:
	rm -rf demos/segments dist/demo.mp4
```

### CONTRIBUTING.md Section

Add a "Recording Demo Videos" section to the project's CONTRIBUTING.md:

```markdown
## Recording Demo Videos

The project uses [VHS](https://github.com/charmbracelet/vhs) to produce a
single MP4 showcasing all CLI features.

### Prerequisites

- [VHS](https://github.com/charmbracelet/vhs) (`go install github.com/charmbracelet/vhs@latest`)
- `ffmpeg` (bundled with VHS, or install separately)
- The CLI binary installed and on `$PATH`

### Recording

```bash
make demo          # records all segments and produces dist/demo.mp4
make demo-clean    # removes all recorded artifacts
```

Individual tapes live in `demos/`. To re-record a single segment:

```bash
vhs demos/03_json_output.tape
```

### Adding a New Feature Demo

1. Create `demos/NN_<name>.tape` following the existing pattern
2. Add the feature to `docs/FEATURES.md`
3. Run `make demo` to verify
```

### Nightly CI Workflow

```yaml
name: Demo Video

on:
  schedule:
    - cron: '0 4 * * *'    # 4:00 AM UTC daily
  workflow_dispatch:         # manual trigger

jobs:
  check-and-record:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0    # full history for change detection

      - name: Check for changes since last demo
        id: check
        run: |
          # Find the last workflow run's commit (stored as a git note or tag)
          LAST_SHA=$(git tag -l 'demo-recorded-*' \
            | sort -r | head -1 \
            | sed 's/demo-recorded-//')

          if [ -z "$LAST_SHA" ]; then
            echo "changed=true" >> "$GITHUB_OUTPUT"
            exit 0
          fi

          # Check if source files changed since that commit
          CHANGED=$(git diff --name-only "$LAST_SHA" HEAD -- \
            'cli/' 'demos/' 'scripts/record-demo.sh' \
            | wc -l)

          if [ "$CHANGED" -gt 0 ]; then
            echo "changed=true" >> "$GITHUB_OUTPUT"
          else
            echo "changed=false" >> "$GITHUB_OUTPUT"
          fi

      - name: Install VHS
        if: steps.check.outputs.changed == 'true'
        uses: charmbracelet/vhs-action@v2

      - name: Build CLI
        if: steps.check.outputs.changed == 'true'
        run: |
          # Build and install the CLI so tapes can `Require` it
          make install

      - name: Record demo
        if: steps.check.outputs.changed == 'true'
        run: make demo

      - name: Upload artifact
        if: steps.check.outputs.changed == 'true'
        uses: actions/upload-artifact@v4
        with:
          name: demo-video
          path: dist/demo.mp4

      - name: Tag recorded commit
        if: steps.check.outputs.changed == 'true'
        run: |
          SHA=$(git rev-parse HEAD)
          git tag "demo-recorded-$SHA"
          git push origin "demo-recorded-$SHA"
```

Key design decisions:

- **Change detection**: Uses lightweight git tags (`demo-recorded-<sha>`) to
  track the last commit that was recorded. Compares source paths only — changes
  to docs or unrelated files do not trigger re-recording.
- **Artifact storage**: The MP4 is uploaded as a GitHub Actions artifact.
  Optionally, a step can commit it to a `gh-pages` branch or push to a release.
- **Manual trigger**: `workflow_dispatch` allows on-demand re-recording.
- **VHS action**: The official `charmbracelet/vhs-action` handles VHS
  installation on CI runners.

### Feature Labeling in Video

Each segment starts with a printed ANSI banner before the demo command:

```
  ── Tree Output ──
```

This ensures a viewer scrubbing through the video can identify which feature is
being demonstrated at any point. The banner is printed by a `printf` command
inside the tape, not by VHS itself, so it appears naturally in the terminal
recording.

---

## Part 2: Treemand-Specific Implementation

This section applies the general design above to the treemand CLI specifically.

### Features to Demo

Based on `docs/FEATURES.md`, the following features will be demonstrated in
order. Each maps to one `.tape` file:

| # | Tape File | Feature | Demo Command |
|---|-----------|---------|-------------|
| 01 | `01_basic_tree.tape` | Basic tree output | `treemand --depth=2 git` |
| 02 | `02_tree_styles.tape` | Tree display styles | `treemand --tree-style=graph --depth=1 git` then `--tree-style=columns` |
| 03 | `03_filter_exclude.tape` | Filter & exclude | `treemand --filter=remote git` then `--exclude=help git` |
| 04 | `04_json_yaml_output.tape` | JSON/YAML output | `treemand --output=json git \| head -20` then `--output=yaml` |
| 05 | `05_icon_presets.tape` | Icon presets | `treemand --icons=ascii --depth=1 git` then `--icons=nerd` |
| 06 | `06_commands_only.tape` | Commands-only mode | `treemand --commands-only --depth=2 kubectl` |
| 07 | `07_cache_management.tape` | Cache list & clear | `treemand cache list` then `treemand cache clear git` |
| 08 | `08_config_management.tape` | Config subcommand | `treemand config view` then `config set` then `config validate` |
| 09 | `09_interactive_tui.tape` | Interactive TUI | `treemand -i git` with navigation, expand, flag picker |
| 10 | `10_self_introspection.tape` | Self-introspection | `treemand treemand` |
| 11 | `11_shell_completion.tape` | Shell completion | `treemand completion bash \| head -5` |
| 12 | `12_version.tape` | Version info | `treemand version` |

### Tape Details

#### Shared Settings (`demos/_settings.tape`)

```tape
Set Shell bash
Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set PlaybackSpeed 1.0
Set TypingSpeed 50ms
Set Padding 20
```

#### TUI Demo (`09_interactive_tui.tape`)

The TUI demo requires special handling since it involves keypresses:

```tape
Require treemand

Source demos/_settings.tape
Output demos/segments/09_interactive_tui.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── Interactive TUI ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand -i git"
Enter
Sleep 2s

# Navigate down through the tree
Down
Sleep 500ms
Down
Sleep 500ms
Down
Sleep 500ms

# Expand a node
Right
Sleep 1s

# Navigate into children
Down
Sleep 500ms
Down
Sleep 500ms

# Select a command (Enter)
Enter
Sleep 1s

# Open flag picker
Type "f"
Sleep 1.5s

# Close flag picker
Escape
Sleep 1s

# Toggle display style
Type "T"
Sleep 1.5s
Type "T"
Sleep 1.5s

# Toggle help pane
Type "H"
Sleep 1.5s

# Quit
Type "q"
Sleep 1s
```

### CI Workflow: Change Detection Paths

For treemand specifically, the paths to watch for changes are:

```yaml
git diff --name-only "$LAST_SHA" HEAD -- \
  'cli/' 'demos/' 'scripts/record-demo.sh' 'Makefile'
```

This covers all Go source code, tape files, and build configuration.

### Output Location

The final video is placed at `dist/demo.mp4`. The existing `demo.tape` (which
produces a GIF for the Hugo site) remains unchanged. The new system produces
an MP4 video as the primary artifact.

### Prerequisites for Developers

Treemand developers need the following CLIs available on `$PATH` for the demo
to record correctly:

- `treemand` (via `make install`)
- `git` (used in most demos)
- `kubectl` (used in commands-only demo — falls back gracefully if missing)

If `kubectl` is not installed, the `06_commands_only.tape` segment can use
`treemand` itself as the target CLI instead.
