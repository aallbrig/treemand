package discovery

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aallbrig/treemand/models"
)

// A root whose children cover each way a child's help can come back: none
// (broken), the root's own help again (same: the CLI ignores its arguments),
// and help of its own (real).
const childCasesCLI = `case "$*" in *broken*) exit 1;; esac
case "$1" in
real) printf 'Run the real thing.\n\nUsage:\n  formcli real [flags]\n\nFlags:\n  --force  force it\n';;
*) printf 'A fake tool.\n\nUsage:\n  formcli <command>\n\nAvailable Commands:\n  broken  Is broken.\n  real    Has help.\n  same    Ignores its arguments.\n\nFlags:\n  --verbose  be loud\n';;
esac
`

// childLine is one line per child: every field discoverWithHelp sets.
func childLine(root, c *models.Node) string {
	var flags []string
	for _, f := range c.Flags {
		flags = append(flags, f.Name)
	}
	errPrefix := c.DiscoveryErr
	if i := strings.Index(errPrefix, ":"); i >= 0 {
		errPrefix = errPrefix[:i]
	}
	return fmt.Sprintf("%s path=%s discovered=%t stub=%t err=%q desc=%q flags=%v kids=%d rootHelp=%t",
		c.Name, strings.Join(c.FullPath[1:], "/"), c.Discovered, c.Stub, errPrefix, c.Description,
		flags, len(c.Children), c.HelpText == root.HelpText)
}

func TestDiscover_childHelpCases(t *testing.T) {
	bin, _ := formCLI(t, 0, childCasesCLI)
	root, err := NewHelpDiscoverer(2).Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range root.Children {
		got = append(got, childLine(root, c))
	}
	want := []string{
		`broken path=broken discovered=true stub=false err="could not get help" desc="Is broken." flags=[] kids=0 rootHelp=false`,
		`real path=real discovered=true stub=false err="" desc="Run the real thing." flags=[--force] kids=0 rootHelp=false`,
		`same path=same discovered=true stub=false err="" desc="A fake tool." flags=[--verbose] kids=0 rootHelp=true`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("children:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDiscover_wideCommandsGetStubChildren(t *testing.T) {
	bin, logPath := formCLI(t, 0, childCasesCLI)
	d := NewHelpDiscoverer(2)
	d.StubThreshold = 2
	root, err := d.Discover(context.Background(), bin, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range root.Children {
		got = append(got, childLine(root, c))
	}
	want := []string{
		`broken path=broken discovered=false stub=true err="" desc="Is broken." flags=[] kids=0 rootHelp=false`,
		`real path=real discovered=false stub=true err="" desc="Has help." flags=[] kids=0 rootHelp=false`,
		`same path=same discovered=false stub=true err="" desc="Ignores its arguments." flags=[] kids=0 rootHelp=false`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("children:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, call := range calls(t, logPath) {
		if strings.HasPrefix(call, "broken") || strings.HasPrefix(call, "real") || strings.HasPrefix(call, "same") {
			t.Errorf("stub children must not be run; got call %q", call)
		}
	}
}
