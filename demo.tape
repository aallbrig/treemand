# treemand demo tape
# Produces: www/treemand/static/demo.gif

Output www/treemand/static/demo.gif

Require treemand

Set Shell bash
Set FontSize 14
Set Width 1000
Set Height 600
Set Theme "Dracula"
Set PlaybackSpeed 1.0
Set TypingSpeed 80ms
Set Padding 20

# ── Part 1: non-interactive tree ─────────────────────────────────────────────
Hide
Type "clear"
Enter
Show

Sleep 500ms

Type "# Non-interactive tree for git"
Sleep 800ms
Enter
Sleep 300ms

Type "treemand git"
Sleep 500ms
Enter
Sleep 4s

# ── Part 2: depth limiting ────────────────────────────────────────────────────
Sleep 1s
Type "treemand --depth=1 git"
Sleep 500ms
Enter
Sleep 3s

# ── Part 3: JSON output ───────────────────────────────────────────────────────
Sleep 1s
Type "treemand --output=json git | head -20"
Sleep 500ms
Enter
Sleep 3s

Sleep 2s
