# Demo: treemand cache — list and clear the discovery cache
Require treemand

Source demos/subcmds/_settings.tape
Output www/treemand/static/demos/cmd_cache.gif

Hide
Type "clear"
Enter
# Populate the cache with a quick discovery
Type "treemand --depth=1 git > /dev/null 2>&1"
Enter
Sleep 2s
Show

# Show cached entries
Type "treemand cache list"
Enter
Sleep 3s

# Clear a specific entry
Type "treemand cache clear git"
Enter
Sleep 2s

# Verify it is gone
Type "treemand cache list"
Enter
Sleep 2s

Sleep 500ms
