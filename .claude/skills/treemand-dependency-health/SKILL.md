---
name: treemand-dependency-health
description: Measure the health of treemand's dependencies (known vulnerabilities, Go modules behind or stale, the Go toolchain, tool versions pinned in CI) and turn findings into upgrades. Use for dependency, vulnerability or upgrade questions, a Dependabot PR, or dependency-health findings.
---

# Dependency health

Follow **treemand-measure** for running, recording and filing; this skill is the dependency playbook.

## Measure

`task deps:health -- --findings` (the same as `task measure -- dependency-health --findings`). The probe's docstring (`scripts/metrics/dependency_health.py`) defines every metric; `scripts/metrics/rules.json` holds the rules. It runs weekly in `measure-weekly.yml`, whose setup-go installs go.mod's toolchain, so that run is the one that judges the standard library treemand ships with. A local run judges the local Go: `details` says so under `vulns_toolchain`.

The probe makes few network calls (go.dev once, the module proxy, PyPI once per pinned Python tool), each cached for the run. Offline, every metric that needs the network is reported **skipped**, never zero; `DEPENDENCY_HEALTH_OFFLINE=1` forces that. Record only a run with nothing skipped.

## Act

- **vulns_reachable** (budget 0): the code calls a vulnerable symbol. Upgrade the module (or the toolchain, for a `stdlib` finding) to the fixed version `govulncheck ./...` names, in its own PR, today. If no fix exists, say so on the issue and keep it open.
- **vulns_imported_unreachable** (ratchet): imported, never called. Upgrade with the next batch; a rise means a new advisory or a new import.
- **go_modules_behind** (ratchet): Dependabot (`.github/dependabot.yml`) groups Go module updates weekly. Merge its open PR (`gh pr list --author app/dependabot`) rather than filing; file only for a module Dependabot has not proposed, or when its PR fails CI (then the issue is the failure).
- **go_modules_stale** (ratchet): no release in two years. Not a bug by itself: check whether the module is finished or abandoned (open issues, archived repo). Replace an abandoned one only if it carries risk (terminal, parsing or I/O code).
- **go_toolchain_behind** (budget 0): bump `go` (and `toolchain`, if present) in `cli/treemand/go.mod` to the newest stable patch on its line, run `task ci`, and note it in the PR. Patch releases carry security fixes. Moving to a newer minor line is a separate, deliberate change.
- **tools_behind** (ratchet): bump the pin where `details` names it (`file:line`; every pin of a tool together) and run the gate that tool backs (`task lint`, `task py:lint`, `task py:types`, `task infra:lint`, `task site:check`). New lint findings belong in the same PR. Tools pinned to `latest` are listed as unpinned and not counted.

## File

Title and labels by treemand-measure: `[measure] dependency-health: <metric> = <value> (<limit or previous>)`, labels `area:quality`, `measure:finding`, `loop:ready`. One issue per metric. Before filing, search open issues and Dependabot PRs for the module or tool. The body lists the items from `details` and the upgrade to make. A vulnerability issue names the OSV ID and the fixed version, never how to exploit it.

## Prove it

The upgrade PR re-runs the probe, quotes the **improved** verdict, and records the new value with `--record`.
