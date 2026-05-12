# Demo: treemand <cli> — basic tree output, styles, and filters
Require treemand

Source demos/subcmds/_settings.tape
Output www/treemand/static/demos/cmd_tree.gif

Hide
Type "clear"
Enter
Show

# Default tree
Type "treemand --depth=2 git"
Enter
Sleep 4s

# Graph style
Type "treemand --tree-style=graph --depth=2 git"
Enter
Sleep 4s

# Filter to a subtopic
Type "treemand --filter=remote --depth=3 git"
Enter
Sleep 3s

Sleep 500ms
