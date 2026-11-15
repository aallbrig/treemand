# A cross-section of CLIs for testing treemand

Date: 2026-11-15. Question: which CLIs, taken together, exercise every way a command-line tool
presents its help, so that a corpus built from them proves treemand's parser is robust beyond the
handful of tools it was developed against?

## Method

treemand parses help text, so the deciding variable is **how a CLI's help is produced**, not what the
tool does. That is mostly decided by its argument-parsing framework, plus a long tail of
hand-written help. The cross-section below therefore samples by framework family. It takes 2–4 CLIs
per family where one is available (different authors use the same framework differently) and adds
CLIs whose help is unusual in ways no framework predicts.

Framework attributions were **checked against each tool's actual `--help` output on this host**
(header shape, section names, flag layout), not assumed. Several assumptions were wrong:
- pnpm 12 prints clap-style help.
- go-task is not urfave/cli.
- pre-commit is argparse, not click.
- `bundle --help` is a man page.

"Installed" means present on the development host (Ubuntu, 2026-11-15).

## Help-text families and the CLIs chosen

| Family | What its help looks like | Chosen CLIs | Installed here |
|---|---|---|---|
| Go, cobra | `Usage:` / `Available Commands:` / `Flags:` / `Global Flags:`; `help for X` | kubectl, gh, docker, helm, hugo, k9s, golangci-lint | all |
| Go, mitchellh/cli (HashiCorp) | `Usage: tool [-version] [-help] <command> [args]`, `Main commands:` / `All other commands:`, single-dash long flags | terraform, vault, packer | none (container) |
| Go, kingpin | `usage: tool [<flags>] <command>`, `Flags:` then `Commands:` with indented descriptions | promtool | none (container) |
| Go, jessevdk/go-flags | `Usage:` / `Application Options:` / `Help Options:` / `Available commands:` | (snap is the common one; needs snapd) | none |
| Go, stdlib `flag` | `-flag type` on one line, description on the next, tab-indented | gofmt, go (custom on top) | all |
| Go, hand-written | free-form `Usage:` and prose | task (go-task) | yes |
| Rust, clap | `Usage: tool [OPTIONS] <COMMAND>`, `Commands:` / `Options:`, `-h, --help  Print help` | uv, ruff, pnpm 12, rg, fd, bat, cargo | uv, ruff, pnpm (container: rg, fd, bat) |
| Python, argparse | `usage:` with wrapped `{a,b,c}` choices, `positional arguments:` / `options:` | pip, pipx, ansible, pre-commit | all |
| Python, click / typer | `Usage: tool [OPTIONS] COMMAND [ARGS]...`, `Options:` / `Commands:` | black, flask, httpie | none (container) |
| Python, cleo | `Usage:` / `Options:` / `Available commands:` grouped by namespace | poetry | none (container) |
| Python, custom | man-page style `aws help`; `aws --help` errors | aws | yes |
| Node, commander | indented `Usage:` / `Options:` / `Commands:` | yarn 1 | yes |
| Node, yargs | `Commands:` / `Options:` with `[boolean]` / `[string]` type tags | eslint, jest | none (container) |
| Node, oclif | `USAGE` / `TOPICS` / `COMMANDS` in capitals | heroku, sf | none (container) |
| Node, hand-written | npm's alias lists; node's flat option list | npm, node | all |
| Ruby, thor | `Commands:` lines like `tool cmd ARG  # description` | thor, rails | none (container) |
| Ruby, optparse / custom | `Usage:`, option blocks; gem's `gem help commands` | gem, ruby; bundle (man page) | all |
| Java, custom / picocli | `Usage: tool <options>`; keytool's `Commands:` with `-genkeypair` style | java, keytool, mvn | java, keytool (container: mvn) |
| .NET, System.CommandLine | `Usage:` / `Options:` / `Commands:`; first-run banner on a fresh home | dotnet | yes |
| GNU getopt_long (coreutils et al.) | `Usage:` plus long option table, `--help` / `--version` | tar, grep, sed, ls, find, rsync, wget, curl | all |
| Multi-call binaries | one binary, many "functions" | busybox | yes |
| Hand-written C, odd shapes | each its own | git (man-backed), openssl (command tables), ffmpeg (huge flat option list), ip (`OBJECT := { … }` grammar), tmux (`list-commands`), systemctl, journalctl, ssh (usage-only), jq, make, cmake, apt | all |

## What the cross-section adds over today's corpus

Today's corpus has 17 CLIs: apt, aws, curl, docker, gh, git, go, helm, jq, kubectl, npm, pip, ssh,
systemctl, tar and tmux, plus the stripe fixtures used by other tests. That is mostly cobra, GNU and
hand-written C. It has **no** clap, click, cleo, commander, yargs, oclif, thor, kingpin,
mitchellh/cli, go-flags, stdlib `flag`, .NET or Java help, and no multi-call binary.

## Side effects observed while sampling

Running a CLI's help is not always free of effects, which matters for both the corpus capture and
the safety rule fixed in #149:

- **Corepack downloaded** pnpm and yarn binaries on their first `--help`.
- **dotnet printed its first-run banner**, which means it writes first-run state into the home
  directory.
- `task` would read a `Taskfile.yml` from the current directory if one existed.

So captures run from an empty directory (already the case since #151). The broad set runs in a
disposable container (see the `cli-zoo` image), where nothing a CLI does can touch the developer's
machine.

## Recommendation

1. **Add the installed CLIs from the table to the corpus now**, capturing root help only
   (`--depth=0`): it is all the corpus stores, and it runs one command per CLI instead of one per
   subcommand.
2. **Strengthen the corpus check.** Counts alone let a regression that swaps one subcommand for a
   junk entry pass, so the baseline also records subcommand names, and a name that disappears
   fails.
3. **Build a `cli-zoo` container** that installs pinned versions of the families missing here
   (mitchellh/cli, kingpin, clap via rg/fd/bat, click, cleo, yargs, oclif, thor, Maven). Capture
   their fixtures inside it, and smoke-test full discovery of every zoo CLI from a scheduled CI
   workflow.
4. Out of scope for now: az and gcloud (multi-GB installs, own help engines) and snap (needs
   snapd). Revisit if users ask.
