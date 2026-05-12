# Demo: treemand completion — generate shell completion scripts
Require treemand

Source demos/subcmds/_settings.tape
Output www/treemand/static/demos/cmd_completion.gif

Hide
Type "clear"
Enter
Show

# Show available shells
Type "treemand completion --help"
Enter
Sleep 3s

# Preview a snippet of the bash completion script
Type "treemand completion bash | head -8"
Enter
Sleep 3s

# Preview zsh
Type "treemand completion zsh | head -8"
Enter
Sleep 3s

Sleep 500ms
