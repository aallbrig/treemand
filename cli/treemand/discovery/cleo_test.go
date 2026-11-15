package discovery

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// cleo apps (poetry, issue #166): --help describes the "list" command; the
// command list comes from running it.
const cleoHelp = `Description:
  Lists commands.

Usage:
  list [options] [--] [<namespace>]

Arguments:
  namespace                  The namespace name

Options:
  -h, --help                 Display help for the given command.
  -q, --quiet                Do not output any message.
`

const cleoList = `Poetry (version 2.5.1)

Usage:
  command [options] [arguments]

Options:
  -h, --help                 Display help for the given command.

Available commands:
  about              Shows information about Poetry.
  add                Adds a new dependency to pyproject.toml and installs it.
  build              Builds a package, as a tarball and a wheel by default.
  install            Installs the project dependencies.
  publish            Publishes a package to a remote repository.

 cache
  cache clear        Clears a Poetry cache by name.
  cache list         List Poetry's caches.
`

func TestDiscover_cleoAppsListTheirCommandsThroughList(t *testing.T) {
	bin, logPath := formCLI(t, 0, `case "$1" in
list) cat <<'L'
`+cleoList+`L
;;
*) cat <<'H'
`+cleoHelp+`H
;;
esac
`)
	root, err := NewHelpDiscoverer(1).Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range root.Children {
		names = append(names, c.Name)
	}
	for _, w := range []string{"add", "build", "install", "publish", "cache"} {
		if !slices.Contains(names, w) {
			t.Errorf("missing %q; got %v", w, names)
		}
	}
	if !strings.Contains(root.HelpText, "Available commands:") {
		t.Errorf("the root's help text should be the command list")
	}
	_ = logPath
}

func TestDiscover_listIsNeverSentToACLIWhoseHelpIsNotTheListCommand(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "acted")
	bin, _ := formCLI(t, 0, `if [ "$1" = list ]; then touch '`+marker+`'; fi
printf 'Usage: plain [OPTIONS] FILE\n\nOptions:\n  -v, --verbose  verbose\n'
`)
	if _, err := NewHelpDiscoverer(1).Discover(context.Background(), bin, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("list was passed to a CLI whose help does not describe the list command")
	}
}
