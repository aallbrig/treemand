# Feature: Shell Completion
Require treemand

Source demos/_settings.tape
Output demos/segments/11_shell_completion.mp4

Hide
Type "clear"
Enter
Show

Sleep 300ms
Type "printf '\\n  \\033[1;36m── Shell Completion ──\\033[0m\\n\\n'"
Enter
Sleep 1s

Type "treemand completion bash | head -10"
Sleep 300ms
Enter
Sleep 3s

Sleep 500ms
