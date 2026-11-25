// Command corpusdump prints what the help parser extracts from corpus
// fixtures, one fact per line, for `task corpus:show` and `task corpus:diff`.
//
//	corpusdump <fixture-dir> [cli...]
//
// Output lines are tab-separated: cli, kind (desc|sub|flag), value. It uses
// only exported parser API, so the same file builds against older trees.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aallbrig/treemand/cli/treemand/discovery"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: corpusdump <fixture-dir> [cli...]")
		os.Exit(2)
	}
	dir, clis := os.Args[1], os.Args[2:]
	if len(clis) == 0 {
		files, err := filepath.Glob(filepath.Join(dir, "*.txt"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, f := range files {
			clis = append(clis, strings.TrimSuffix(filepath.Base(f), ".txt"))
		}
	}
	sort.Strings(clis)
	for _, cli := range clis {
		b, err := os.ReadFile(filepath.Join(dir, cli+".txt")) //nolint:gosec // fixture path from the caller
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		// A fixture may name a variant after "@" (tar@bsd); the parser
		// sees the CLI's own name.
		name, _, _ := strings.Cut(cli, "@")
		p := discovery.ParseHelpOutputFor(discovery.SanitizeHelp(string(b)), name)
		fmt.Printf("%s\tdesc\t%s\n", cli, p.Description)
		for _, s := range p.Subcommands {
			fmt.Printf("%s\tsub\t%s\n", cli, s)
		}
		for _, f := range p.Flags {
			fmt.Printf("%s\tflag\t%s %s\n", cli, f.Name, f.ValueType)
		}
	}
}
