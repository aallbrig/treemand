# Feature: Commands-Only Mode
Require treemand

Source demos/_settings.tape
Output demos/segments/06_commands_only.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── Commands-Only Mode ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand --commands-only --depth=2 git"
Sleep 300ms
Enter
Sleep 4s

Sleep 500ms
