# Feature: Tree Display Styles
Require treemand

Source demos/_settings.tape
Output demos/segments/02_tree_styles.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── Tree Display Styles ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand --tree-style=graph --depth=1 git"
Sleep 300ms
Enter
Sleep 3s

Type "treemand --tree-style=columns --depth=1 git"
Sleep 300ms
Enter
Sleep 3s

Type "treemand --tree-style=compact --depth=1 git"
Sleep 300ms
Enter
Sleep 3s

Sleep 500ms
