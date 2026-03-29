# Feature: Basic Tree Output
Require treemand

Source demos/_settings.tape
Output demos/segments/01_basic_tree.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── Basic Tree Output ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand --depth=2 git"
Sleep 300ms
Enter
Sleep 4s

Sleep 500ms
