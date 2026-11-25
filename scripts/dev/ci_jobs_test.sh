#!/bin/sh
# CI runs each check once (#478): task ci is exactly ci:go, ci:gates and
# measure:gate, each CI job runs one of them, and no step lists a check of
# its own that one of them already runs.
set -u
cd "$(dirname "$0")/../.." || exit 1
fail=0
ok() { echo "  ok   $1"; }
bad() { echo "  FAIL $1"; fail=1; }

# block <file> <key>: the lines of a two-space-indented YAML block
block() { sed -n "/^  $2:\$/,/^  [a-z:&-]*:\$/p" "$1" | sed '$d'; }

ci="$(block Taskfile.yml ci | grep -- '- task: ')"
want='      - task: ci:go
      - task: ci:gates
      - task: measure:gate'
[ "$ci" = "$want" ] && ok "task ci is ci:go, ci:gates and measure:gate" || bad "task ci runs: $ci"

go_steps="$(block Taskfile.yml ci:go | sed -n 's/^ *- task: //p')"
gate_steps="$(block Taskfile.yml ci:gates | sed -n 's/^ *- task: //p')"
dupes="$(printf '%s\n%s\n' "$go_steps" "$gate_steps" | sort | uniq -d)"
[ -z "$dupes" ] && ok "no step is in both ci:go and ci:gates" || bad "in both: $dupes"
for s in $go_steps $gate_steps; do
  [ "$s" = measure:gate ] && bad "measure:gate is in ci:go or ci:gates; the measure job runs it"
done
for s in $gate_steps; do
  [ "$s" = corpus:test ] && bad "corpus:test reruns TestCorpus, which test-race already runs"
done

for pair in test:ci:go gates:ci:gates measure:measure:gate; do
  job="${pair%%:*}" task="${pair#*:}"
  case "$(block .github/workflows/ci.yml "$job")" in
    *"run: task $task"*) ok "the $job job runs task $task" ;;
    *) bad "the $job job does not run task $task" ;;
  esac
done
grep -qE 'run: task ci *$' .github/workflows/ci.yml && bad "a CI job runs all of task ci"
if grep -qE 'golangci-lint@latest|govulncheck@latest|version: latest' .github/workflows/ci.yml; then
  bad "ci.yml installs a lint or vuln tool at latest"
else ok "golangci-lint and govulncheck are pinned"; fi
linux_go="$(sed '/^  test-macos:$/,/^  gates:$/d' .github/workflows/ci.yml | grep -E 'run: (go (test|vet)|gofmt|govulncheck)')"
[ -z "$linux_go" ] && ok "no Linux job runs a Go check outside the tasks" || bad "Go checks outside the tasks: $linux_go"

[ "$fail" = 0 ] && echo "ci_jobs_test: ok"
exit "$fail"
