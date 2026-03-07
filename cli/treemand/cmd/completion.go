package cmd

import (
"os"
"strings"

"github.com/spf13/cobra"

"github.com/aallbrig/treemand/cache"
"github.com/aallbrig/treemand/config"
)

// completionCmd provides shell completion script generation.
var completionCmd = &cobra.Command{
Use:   "completion [bash|zsh|fish|powershell]",
Short: "Generate shell completion scripts",
Long: `Generate shell completion scripts for treemand.

Bash:
  treemand completion bash > /etc/bash_completion.d/treemand

Zsh:
  source <(treemand completion zsh)

Fish:
  treemand completion fish > ~/.config/fish/completions/treemand.fish

PowerShell:
  treemand completion powershell | Out-String | Invoke-Expression`,
DisableFlagsInUseLine: true,
ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
RunE: func(cmd *cobra.Command, args []string) error {
switch args[0] {
case "bash":
return cmd.Root().GenBashCompletionV2(os.Stdout, true)
case "zsh":
return cmd.Root().GenZshCompletion(os.Stdout)
case "fish":
return cmd.Root().GenFishCompletion(os.Stdout, true)
case "powershell":
return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
}
return nil
},
}

// completeCLIName provides dynamic tab-completion for the CLI name positional argument.
// It returns CLIs already present in the cache so users can quickly re-explore known tools.
func completeCLIName(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
if len(args) > 0 {
return nil, cobra.ShellCompDirectiveNoFileComp
}

cfg := config.DefaultConfig()
c, err := cache.Open(cfg.CacheDir)
if err != nil {
return nil, cobra.ShellCompDirectiveNoFileComp
}
defer c.Close()

clis, err := c.ListCLIs()
if err != nil || len(clis) == 0 {
return nil, cobra.ShellCompDirectiveNoFileComp
}

var matches []string
for _, name := range clis {
if toComplete == "" || strings.HasPrefix(name, toComplete) {
matches = append(matches, name)
}
}
return matches, cobra.ShellCompDirectiveNoFileComp
}
