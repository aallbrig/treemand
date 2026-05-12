# Demo: treemand -i <cli> — interactive TUI explorer
Require treemand

Source demos/subcmds/_settings.tape
Output www/treemand/static/demos/cmd_interactive.gif

Hide
Type "clear"
Enter
Show

Type "treemand -i git"
Enter
Sleep 2s

# Navigate down the tree
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms

# Expand the selected node
Right
Sleep 1s

# Enter its first child
Right
Sleep 800ms

# Press Enter to pick the command
Enter
Sleep 1s

# Open the flag picker
Type "f"
Sleep 2s

# Move down to a flag and select it
Down
Sleep 300ms
Down
Sleep 300ms
Enter
Sleep 1s

# Close flag picker and show preview
Escape
Sleep 1s

# Cycle display style
Type "T"
Sleep 1.5s
Type "T"
Sleep 1.5s

# Show key bindings overlay
Type "?"
Sleep 2s
Escape
Sleep 800ms

# Quit
Type "q"
Sleep 500ms
