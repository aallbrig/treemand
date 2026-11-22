---
title: "mcp"
description: "treemand mcp: an MCP server that lets AI agents discover, search and assemble any CLI's commands."
weight: 8
---

# `treemand mcp`

Run a [Model Context Protocol](https://modelcontextprotocol.io) server on stdin
and stdout. An AI agent connected to it can map any command-line tool on your
`PATH` without running `--help` and parsing the text itself.

## Connect a client

The server is a local stdio process, started by the client:

```bash
claude mcp add treemand -- treemand mcp
```

Clients configured with JSON take the same command:

```json
{
  "mcpServers": {
    "treemand": { "command": "treemand", "args": ["mcp"] }
  }
}
```

## Tools

### `discover` {#tool-discover}

The command tree as JSON, following the schema `treemand schema` prints.

- `cli`: a command on `PATH`, such as `git`
- `depth`: levels of subcommands, 1 to 5 (default 2)
- `include_help`: keep each command's raw help text (default false)

### `search` {#tool-search}

Commands, flags and positional arguments that match a query, best first, each
with its full command path.

- `cli`, `query`
- `limit`: most results (default 20)

### `assemble` {#tool-assemble}

A shell-quoted command line and its `argv`. The path must exist; a flag the
help does not list is kept, with a warning.

- `cli`, `path`: subcommand names, such as `["remote", "add"]`
- `flags`: `{name, value}` objects; leave out `value` for a switch
- `args`: positional arguments

For example, `assemble` with `{"cli": "git", "path": ["commit"], "flags": [{"name": "m", "value": "fix: it's done"}]}`
returns `git commit --message 'fix: it'\''s done'`.

## Safety

- **Nothing is executed** except the `--help` probes that `treemand <cli>`
  already makes. `assemble` returns a command; it never runs it.
- `cli` must be a bare command name found on `PATH`, such as `kubectl`. Paths
  and names starting with `-` or `.` are refused.
- Each tool call is limited by `--timeout` (default 30 seconds). A tree cut short
  by the limit is returned with a note that it is partial.
- Trees are cached like any other run, so a second call on the same CLI is
  instant. `--no-cache` turns that off.
- Stdout carries only protocol messages; logs (`--debug`) go to stderr.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--timeout` | `30` | Seconds each tool call may take (0 = no limit) |
| `--no-cache` | false | Discover every time instead of using the cache |
| `--jobs` | auto | Max concurrent help processes during discovery |
| `--debug` | false | Debug logging on stderr |
