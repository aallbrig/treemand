# Feature: Version Info
Require treemand

Source demos/_settings.tape
Output demos/segments/12_version.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── Version Info ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand version"
Sleep 300ms
Enter
Sleep 2s

Sleep 500ms
