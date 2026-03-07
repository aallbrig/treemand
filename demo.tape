# treemand demo tape
# Produces: www/treemand/static/demo.gif

Output www/treemand/static/demo.gif

Require treemand

Set Shell bash
Set FontSize 14
Set Width 1100
Set Height 650
Set Theme "Dracula"
Set PlaybackSpeed 1.0
Set TypingSpeed 60ms
Set Padding 20

# ── Part 1: basic tree ────────────────────────────────────────────────────────
Hide
Type "clear"
Enter
Show

Sleep 500ms
Type "treemand --depth=1 git --no-color"
Sleep 400ms
Enter
Sleep 3s

# ── Part 2: colored output with flags ────────────────────────────────────────
Sleep 1s
Type "treemand --depth=1 git"
Sleep 400ms
Enter
Sleep 3s

# ── Part 3: filter ────────────────────────────────────────────────────────────
Sleep 1s
Type "treemand --filter=remote git"
Sleep 400ms
Enter
Sleep 3s

# ── Part 4: ascii icon set ────────────────────────────────────────────────────
Sleep 1s
Type "treemand --icons=ascii --depth=1 git"
Sleep 400ms
Enter
Sleep 3s

# ── Part 5: JSON output ───────────────────────────────────────────────────────
Sleep 1s
Type "treemand --output=json git | head -20"
Sleep 400ms
Enter
Sleep 3s

# ── Part 6: self-dogfooding ───────────────────────────────────────────────────
Sleep 1s
Type "treemand --depth=1 treemand"
Sleep 400ms
Enter
Sleep 3s

Sleep 1s
