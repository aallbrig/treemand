# Feature: Interactive TUI
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

# Select a command with Enter
Enter
Sleep 1.5s

# Open flag picker
Type "f"
Sleep 2s

# Close flag picker
Escape
Sleep 1s

# Cycle display style
Type "T"
Sleep 1.5s
Type "T"
Sleep 1.5s

# Toggle help pane
Type "H"
Sleep 2s

# Quit
Type "q"
Sleep 1s
