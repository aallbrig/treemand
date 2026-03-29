# Feature: Icon Presets
Require treemand

Source demos/_settings.tape
Output demos/segments/05_icon_presets.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── Icon Presets ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand --icons=ascii --depth=1 git"
Sleep 300ms
Enter
Sleep 3s

Type "treemand --icons=nerd --depth=1 git"
Sleep 300ms
Enter
Sleep 3s

Sleep 500ms
