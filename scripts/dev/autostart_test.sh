#!/bin/sh
# shellcheck disable=SC2016,SC2034,SC2329 # checks run through eval
# Behaviour tests for scripts/dev/autostart.sh. Runs it under dash with a
# scratch HOME, a copy of the repo scripts and a PATH whose uname, id,
# launchctl and systemctl are stubs that log their arguments.
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM
fail=0
check() { if ! eval "$2"; then echo "autostart_test FAIL: $1" >&2; fail=1; fi; }

repo="$tmp/repo"
mkdir -p "$repo/scripts/dev"
cp "$here/autostart.sh" "$repo/scripts/dev/"
: >"$repo/scripts/dev_session.sh"

bin="$tmp/bin"
mkdir -p "$bin"
for t in sed mkdir cp rm rmdir dirname cat; do ln -s "$(command -v "$t")" "$bin/$t"; done
stub() { printf '#!/bin/sh\n%s\n' "$2" >"$bin/$1"; chmod +x "$bin/$1"; }
stub id 'echo 501'
# launchctl: "print" succeeds only while bootstrapped (tracked by a marker file).
stub launchctl "echo \"launchctl \$*\" >>\"$tmp/calls\"
case \"\$1\" in
  print) [ -f \"$tmp/loaded\" ] ;;
  bootstrap) : >\"$tmp/loaded\" ;;
  bootout) rm -f \"$tmp/loaded\" ;;
esac"
stub systemctl "echo \"systemctl \$*\" >>\"$tmp/calls\"
case \"\$2\" in
  enable) : >\"$tmp/loaded\" ;;
  disable) rm -f \"$tmp/loaded\" ;;
  is-enabled) [ -f \"$tmp/loaded\" ] ;;
esac"

sh_bin="$(command -v dash || command -v sh)"
home="$tmp/home"
run() {
  mkdir -p "$home"
  env -i HOME="$home" PATH="$bin" GH_TOKEN=sekrit-token "$sh_bin" "$repo/scripts/dev/autostart.sh" "$@" >"$tmp/out" 2>&1
  echo $? >"$tmp/rc"
}
rc() { cat "$tmp/rc"; }
leftovers() { [ -z "$(cd "$home" && find . -type f)" ] && [ ! -e "$repo/.loop/autostart" ]; }

# ── macOS / launchd ──────────────────────────────────────────────────────────
stub uname 'echo Darwin'
plist="$home/Library/LaunchAgents/dev.treemand.session.plist"

run status
check "launchd: status before install exits non-zero" '[ "$(rc)" != 0 ]'
check "launchd: status says not installed" 'grep -q "installed: no" "$tmp/out"'

rm -f "$tmp/calls"
run install
check "launchd: install exits 0" '[ "$(rc)" = 0 ]'
check "launchd: plist written" '[ -f "$plist" ]'
check "launchd: generated copy kept under .loop/autostart" 'cmp -s "$plist" "$repo/.loop/autostart/dev.treemand.session.plist"'
check "launchd: label" 'grep -q "<string>dev.treemand.session</string>" "$plist"'
check "launchd: RunAtLoad" 'grep -A1 "<key>RunAtLoad</key>" "$plist" | grep -q "<true/>"'
check "launchd: runs dev_session.sh start with /bin/sh" 'grep -A4 "<key>ProgramArguments</key>" "$plist" | tr -d " \n" | grep -q "<string>/bin/sh</string><string>$repo/scripts/dev_session.sh</string><string>start</string>"'
check "launchd: PATH has Homebrew" 'grep -q "/opt/homebrew/bin:.*/usr/local/bin" "$plist"'
check "launchd: PATH has ~/go/bin" 'grep -q "$home/go/bin" "$plist"'
check "launchd: log path" 'grep -q "$home/Library/Logs/treemand-session.log" "$plist"'
check "launchd: no token leaks" '! grep -q sekrit "$plist"'
check "launchd: bootstrap into gui domain" 'grep -qx "launchctl bootstrap gui/501 $plist" "$tmp/calls"'
check "launchd: no bootout on fresh install" '! grep -q bootout "$tmp/calls"'

run status
check "launchd: status after install exits 0" '[ "$(rc)" = 0 ]'
check "launchd: status installed" 'grep -q "installed: yes" "$tmp/out"'
check "launchd: status loaded" 'grep -q "loaded:    yes" "$tmp/out"'

rm -f "$tmp/calls"
run install
check "launchd: reinstall boots out first" 'grep -qx "launchctl bootout gui/501/dev.treemand.session" "$tmp/calls"'
check "launchd: reinstall bootstraps again" 'grep -q "launchctl bootstrap" "$tmp/calls"'

: >"$home/Library/Logs/treemand-session.log"
rm -f "$tmp/calls"
run uninstall
check "launchd: uninstall exits 0" '[ "$(rc)" = 0 ]'
check "launchd: uninstall boots out" 'grep -qx "launchctl bootout gui/501/dev.treemand.session" "$tmp/calls"'
check "launchd: uninstall leaves nothing behind" 'leftovers'
run status
check "launchd: status after uninstall says not loaded" 'grep -q "loaded:    no" "$tmp/out"'
run uninstall
check "launchd: uninstall twice is fine" '[ "$(rc)" = 0 ]'

# ── Linux / systemd ──────────────────────────────────────────────────────────
rm -rf "$home" "$tmp/loaded"
stub uname 'echo Linux'
unit="$home/.config/systemd/user/treemand-session.service"

rm -f "$tmp/calls"
run install
check "systemd: install exits 0" '[ "$(rc)" = 0 ]'
check "systemd: unit written" '[ -f "$unit" ]'
check "systemd: generated copy kept" 'cmp -s "$unit" "$repo/.loop/autostart/treemand-session.service"'
check "systemd: oneshot" 'grep -qx "Type=oneshot" "$unit"'
check "systemd: RemainAfterExit" 'grep -qx "RemainAfterExit=yes" "$unit"'
check "systemd: ExecStart" 'grep -qx "ExecStart=/bin/sh \"$repo/scripts/dev_session.sh\" start" "$unit"'
check "systemd: WantedBy" 'grep -qx "WantedBy=default.target" "$unit"'
check "systemd: PATH has ~/go/bin" 'grep -q "^Environment=\"PATH=$home/go/bin:" "$unit"'
check "systemd: no token leaks" '! grep -q sekrit "$unit"'
check "systemd: daemon-reload" 'grep -qx "systemctl --user daemon-reload" "$tmp/calls"'
check "systemd: enable" 'grep -qx "systemctl --user enable treemand-session.service" "$tmp/calls"'

run status
check "systemd: status exits 0" '[ "$(rc)" = 0 ]'
check "systemd: status loaded" 'grep -q "loaded:    yes" "$tmp/out"'

rm -f "$tmp/calls"
run uninstall
check "systemd: disable" 'grep -qx "systemctl --user disable treemand-session.service" "$tmp/calls"'
check "systemd: uninstall leaves nothing behind" 'leftovers'
run status
check "systemd: status after uninstall exits non-zero" '[ "$(rc)" != 0 ]'

# Settings that are passed through, quoted for systemd.
env -i HOME="$home" PATH="$bin" TREEMAND_SESSION="x%y" TREEMAND_NO_BROWSER=1 "$sh_bin" "$repo/scripts/dev/autostart.sh" install >/dev/null 2>&1
check "systemd: TREEMAND_SESSION passed, % doubled" 'grep -qx "Environment=\"TREEMAND_SESSION=x%%y\"" "$unit"'
check "systemd: TREEMAND_NO_BROWSER passed" 'grep -qx "Environment=\"TREEMAND_NO_BROWSER=1\"" "$unit"'

# ── usage ────────────────────────────────────────────────────────────────────
run bogus
check "unknown command exits 2" '[ "$(rc)" = 2 ]'

[ "$fail" = 0 ] && echo "autostart_test: ok"
exit "$fail"
