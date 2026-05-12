# Demo: treemand config — view, validate, set, and inspect config
Require treemand

Source demos/subcmds/_settings.tape
Output www/treemand/static/demos/cmd_config.gif

Hide
Type "clear"
Enter
Show

# Show config file path
Type "treemand config path"
Enter
Sleep 2s

# Show current merged config
Type "treemand config view"
Enter
Sleep 3s

# Validate the config
Type "treemand config validate"
Enter
Sleep 2s

# Set a config value
Type "treemand config set icons unicode"
Enter
Sleep 2s

# Show the change took effect
Type "treemand config view"
Enter
Sleep 3s

Sleep 500ms
