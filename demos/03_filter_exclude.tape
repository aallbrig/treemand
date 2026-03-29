# Feature: Filter & Exclude
Require treemand

Source demos/_settings.tape
Output demos/segments/03_filter_exclude.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── Filter & Exclude ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand --filter=remote git"
Sleep 300ms
Enter
Sleep 3s

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "treemand --exclude=help git"
Sleep 300ms
Enter
Sleep 3s

Sleep 500ms
