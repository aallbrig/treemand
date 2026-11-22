# Agent dev tooling: LSP, dlv and the GoLand debugger

`sh scripts/dev/setup.sh --install` (`task setup -- --install`) provisions all
of this: `gopls`, `dlv` (at least the Go toolchain's minor version) and the
Claude Code plugin `gopls-lsp@claude-plugins-official` at user scope. A dry
run (`sh scripts/dev/setup.sh`) shows what is missing.

## LSP (gopls) for navigation

Use it to find a symbol's definition, every reference to it, its callers and
its implementations, instead of grepping. It understands Go: a grep for
`ParseHelp` also hits comments, strings and unrelated identifiers, while
references from gopls are exact and cross the package boundary.

- **Claude Code**: with the `gopls-lsp` plugin enabled, the `LSP` tool answers
  `goToDefinition`, `findReferences`, `hover`, `documentSymbol`,
  `goToImplementation` and call-hierarchy queries on `.go` files. It needs
  `gopls` on PATH. Enable it by hand with
  `claude plugin install gopls-lsp@claude-plugins-official --scope user`, then
  start a new session.
- **Without the plugin**, gopls answers on the command line from
  `cli/treemand` (positions are `file:line:column`):

  ```sh
  cd cli/treemand
  gopls definition discovery/cmdlist_test.go:22:23
  gopls references discovery/cmdlist.go:28:6
  ```

## dlv for real breakpoints

Use it when a test fails and reading the code has not explained why: stop at
the line, then look at the stack and the values instead of adding prints.
Run it from `cli/treemand`. It never needs a terminal when its commands come
from an `--init` file that ends with `exit`:

```sh
cd cli/treemand
printf '%s\n' 'break discovery.ParseCommandList' continue 'stack 3' 'print len(out)' exit >/tmp/dlv.init
dlv test ./discovery --init /tmp/dlv.init --allow-non-terminal-interactive=true \
  -- -test.run '^TestParseCommandList_tmuxStyle$' </dev/null
```

A breakpoint can also be a line (`break discovery/cmdlist.go:31`) and take a
condition (`condition 1 line == "kill-server"`). Useful commands: `locals`,
`args`, `print <expr>`, `next`, `step`, `stepout`, `goroutines`.

For a longer session, run dlv headless and connect to it as often as needed:

```sh
cd cli/treemand
dlv test ./discovery --headless --listen=127.0.0.1:2345 --api-version=2 --accept-multiclient \
  -- -test.run '^TestParseCommandList_tmuxStyle$' &
dlv connect 127.0.0.1:2345 --init /tmp/dlv.init --allow-non-terminal-interactive=true </dev/null
```

With stdin at `/dev/null`, `exit` in the connect script answers dlv's
prompts with EOF, which leaves the server running for the next `connect`.
Stop it with `kill %1` when done. The same flags debug the binary itself:
`dlv debug . -- --output=json git`.

dlv must be at least as new as the Go toolchain (an older dlv fails to read
newer binaries). `setup.sh` reports an old dlv as `unsuitable`, and `--install` upgrades
it with `go install github.com/go-delve/delve/cmd/dlv@latest`.

## GoLand MCP debugger (optional)

When GoLand is installed, its MCP server lets a Claude Code session drive
GoLand's debugger (set breakpoints, run to a line, read frames). Enable it in
GoLand at **Settings → Tools → MCP Server** ("Enable MCP Server"), let it
configure the detected Claude Code client, and start a new session. It uses
the same delve engine as above, so plain `dlv` is enough when GoLand is not
running.
