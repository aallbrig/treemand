# Feature: Config Subcommand
Require treemand

Source demos/_settings.tape
Output demos/segments/08_config_management.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── Config Management ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand config view"
Sleep 300ms
Enter
Sleep 3s

Type "treemand config path"
Sleep 300ms
Enter
Sleep 2s

Type "treemand config validate"
Sleep 300ms
Enter
Sleep 2s

Sleep 500ms
