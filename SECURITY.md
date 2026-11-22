# Security policy

## Supported versions

treemand is pre-1.0 and ships small, frequent releases. Security fixes land
in the next release; only the [latest release](https://github.com/aallbrig/treemand/releases/latest)
is supported. Upgrade before reporting, if you can.

## Reporting a vulnerability

Please report vulnerabilities privately, not in a public issue.

1. Open the repository's [Security tab](https://github.com/aallbrig/treemand/security)
   and choose **Report a vulnerability** (GitHub private vulnerability
   reporting). Only the maintainers see the report.
2. If that button is not available, open an issue titled
   "Security contact request" with no details of the problem, and a
   maintainer will arrange a private channel.

Include the treemand version (`treemand version`), your OS, the command you
ran, and what an attacker gains. A proof of concept helps.

## What to expect

- An acknowledgement within 7 days.
- An assessment and, for a confirmed issue, a fix plan within 30 days.
- Credit in the release notes and the advisory, unless you ask otherwise.

## Scope

treemand runs the help commands of CLIs that are already installed
(`<cli> --help`, `<cli> help <sub>`), caches the parsed tree under the user's
cache directory, reads its config file, and assembles commands for the user
to copy or run. The website's Try page runs the same code in the browser as
WebAssembly. Reports about any of these are in scope, for example: discovery
passing arguments a CLI would act on, a cache or config file that can inject
a command, or assembled output that is quoted unsafely. The behaviour of the
CLIs treemand inspects is out of scope.
