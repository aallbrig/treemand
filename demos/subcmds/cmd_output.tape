# Demo: treemand --output=json/yaml — machine-readable output
Require treemand

Source demos/subcmds/_settings.tape
Output www/treemand/static/demos/cmd_output.gif

Hide
Type "clear"
Enter
Show

# JSON output (truncated for readability)
Type "treemand --output=json --depth=1 git | head -30"
Enter
Sleep 4s

# Extract subcommand names with jq
Type "treemand --output=json --depth=1 git | jq '[.children[].name]'"
Enter
Sleep 3s

# YAML output
Type "treemand --output=yaml --depth=1 git | head -20"
Enter
Sleep 3s

Sleep 500ms
