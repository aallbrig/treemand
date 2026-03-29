# Feature: Cache Management
Require treemand

Source demos/_settings.tape
Output demos/segments/07_cache_management.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── Cache Management ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand cache list"
Sleep 300ms
Enter
Sleep 3s

Type "treemand cache clear git"
Sleep 300ms
Enter
Sleep 2s

Sleep 500ms
