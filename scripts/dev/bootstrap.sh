#!/bin/sh
# Dev-session bootstrap: report which tools `task ci` and the dev session need,
# with install hints, then open the local and production sites in a browser.
# Never fails: it is informational and always exits 0.
#
#   TREEMAND_NO_BROWSER=1       do not open a browser
#   TREEMAND_SITE_URL           local site to wait for (default http://localhost:1313/)
#   TREEMAND_BOOTSTRAP_WAIT     seconds to wait for it (default 60)

site_url="${TREEMAND_SITE_URL:-http://localhost:1313/}"
prod_url="https://treemand.dev"
wait_s="${TREEMAND_BOOTSTRAP_WAIT:-60}"

# tool|brew formula|apt package or other install hint ("-" when brew has none)
tools='task|go-task|see https://taskfile.dev/installation
go|go|apt install golang-go
golangci-lint|golangci-lint|see https://golangci-lint.run/welcome/install/
govulncheck|-|go install golang.org/x/vuln/cmd/govulncheck@latest
git|git|apt install git
gh|gh|apt install gh
jq|jq|apt install jq
curl|curl|apt install curl
python3|python@3.12|apt install python3
shellcheck|shellcheck|apt install shellcheck
dash|dash|apt install dash
gitleaks|gitleaks|apt install gitleaks
cfn-lint|cfn-lint|pipx install cfn-lint
checkov|checkov|pipx install checkov
hugo|hugo|apt install hugo (extended edition)
node|-|nvm install (version pinned in .nvmrc)
tmux|tmux|apt install tmux
tmuxinator|tmuxinator|apt install tmuxinator
watchexec|watchexec|see https://github.com/watchexec/watchexec#install'

report() {
  echo "treemand dev host readiness (tools for task ci and the dev session)"
  total=0
  missing=0
  old_ifs="$IFS"
  IFS='
'
  for row in $tools; do
    IFS='|' read -r name brew other <<ROW
$row
ROW
    total=$((total + 1))
    if command -v "$name" >/dev/null 2>&1; then
      printf '  %-8s %s\n' ok "$name"
    else
      missing=$((missing + 1))
      if [ "$brew" = - ]; then hint="$other"; else hint="brew install $brew | $other"; fi
      printf '  %-8s %-14s %s\n' missing "$name" "$hint"
    fi
  done
  IFS="$old_ifs"
  echo "$missing of $total tools missing"
  if [ "$missing" -gt 0 ]; then echo "(task setup installs the git hooks once the tools are present)"; fi
}

site_up() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsS -o /dev/null --max-time 2 "$site_url" >/dev/null 2>&1
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O /dev/null -T 2 "$site_url" >/dev/null 2>&1
  else
    return 1
  fi
}

opener() {
  if [ "$(uname -s 2>/dev/null)" = Darwin ]; then
    command -v open >/dev/null 2>&1 && echo open
  else
    command -v xdg-open >/dev/null 2>&1 && echo xdg-open
  fi
}

browse() {
  [ -n "${TREEMAND_NO_BROWSER:-}" ] && return 0
  cmd="$(opener)"
  [ -n "$cmd" ] || return 0
  echo
  echo "waiting up to ${wait_s}s for $site_url"
  waited=0
  up=0
  while [ "$waited" -lt "$wait_s" ]; do
    if site_up; then up=1; break; fi
    sleep 1
    waited=$((waited + 1))
  done
  if [ "$up" = 1 ]; then
    "$cmd" "$site_url" >/dev/null 2>&1 || true
    echo "opened $site_url"
  else
    echo "local site not up after ${wait_s}s (see the site window); skipping it"
  fi
  "$cmd" "$prod_url" >/dev/null 2>&1 || true
  echo "opened $prod_url"
}

report
browse
exit 0
