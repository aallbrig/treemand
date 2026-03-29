# Feature: JSON & YAML Output
Require treemand

Source demos/_settings.tape
Output demos/segments/04_json_yaml_output.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── JSON & YAML Output ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand --output=json git | head -20"
Sleep 300ms
Enter
Sleep 3s

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "treemand --output=yaml git | head -20"
Sleep 300ms
Enter
Sleep 3s

Sleep 500ms
