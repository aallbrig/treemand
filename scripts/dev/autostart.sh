#!/bin/sh
# Start the treemand_dev session at login, on launchd (macOS) or systemd (Linux).
#   install    write the LaunchAgent or user unit and load or enable it
#   uninstall  unload or disable it and remove every file install wrote
#   status     report installed or not, loaded or not (exit 0 only if both)
#
# At login it runs `scripts/dev_session.sh start`, which only creates the
# detached tmux session (a no-op if one exists). The loops window then runs
# `task loop:resume`, so the loop comes back only if it was enabled.
#
# The generated file is written to .loop/autostart/ (gitignored) and copied
# into place. It holds the repo path, a PATH and at most TREEMAND_SESSION and
# TREEMAND_NO_BROWSER: never credentials or tokens.
set -eu

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SESSION_SCRIPT="$ROOT/scripts/dev_session.sh"
GEN_DIR="$ROOT/.loop/autostart"

LABEL="dev.treemand.session"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
LOG="$HOME/Library/Logs/treemand-session.log"
UNIT_NAME="treemand-session.service"
UNIT="$HOME/.config/systemd/user/$UNIT_NAME"

die() { echo "autostart: $*" >&2; exit 1; }

init_system() {
  if [ "$(uname -s)" = Darwin ]; then
    echo launchd
  elif command -v systemctl >/dev/null 2>&1; then
    echo systemd
  else
    die "no launchd or systemd found; start the session with 'task dev' instead"
  fi
}

# Login services get a minimal PATH; add Homebrew, Go and pipx tool locations.
login_path() {
  printf '%s' "$HOME/go/bin:$HOME/.local/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/home/linuxbrew/.linuxbrew/bin:/usr/bin:/bin:/usr/sbin:/sbin"
}

xml_escape() { printf '%s' "$1" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g'; }

# systemd: quote the value and double % so specifiers are not expanded.
unit_quote() { printf '"%s"' "$(printf '%s' "$1" | sed -e 's/%/%%/g' -e 's/\\/\\\\/g' -e 's/"/\\"/g')"; }

# The optional, non-secret settings carried into the login environment.
passthrough() {
  for name in TREEMAND_SESSION TREEMAND_NO_BROWSER; do
    eval "val=\${$name:-}"
    # shellcheck disable=SC2154 # val is set by the eval above
    [ -n "$val" ] && printf '%s\t%s\n' "$name" "$val"
  done
  return 0
}

write_plist() {
  {
    cat <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>$LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/sh</string>
    <string>$(xml_escape "$SESSION_SCRIPT")</string>
    <string>start</string>
  </array>
  <key>WorkingDirectory</key>
  <string>$(xml_escape "$ROOT")</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>$(xml_escape "$(login_path)")</string>
    <key>LANG</key>
    <string>en_US.UTF-8</string>
EOF
    passthrough | while IFS="$(printf '\t')" read -r k v; do
      printf '    <key>%s</key>\n    <string>%s</string>\n' "$k" "$(xml_escape "$v")"
    done
    cat <<EOF
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>AbandonProcessGroup</key>
  <true/>
  <key>StandardOutPath</key>
  <string>$(xml_escape "$LOG")</string>
  <key>StandardErrorPath</key>
  <string>$(xml_escape "$LOG")</string>
</dict>
</plist>
EOF
  } >"$1"
}

write_unit() {
  {
    cat <<EOF
[Unit]
Description=treemand dev session (tmux)

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=$(unit_quote "$ROOT")
Environment=$(unit_quote "PATH=$(login_path)")
EOF
    passthrough | while IFS="$(printf '\t')" read -r k v; do
      printf 'Environment=%s\n' "$(unit_quote "$k=$v")"
    done
    cat <<EOF
ExecStart=/bin/sh $(unit_quote "$SESSION_SCRIPT") start

[Install]
WantedBy=default.target
EOF
  } >"$1"
}

launchd_target() { echo "gui/$(id -u)"; }
launchd_loaded() { launchctl print "$(launchd_target)/$LABEL" >/dev/null 2>&1; }

install() {
  mkdir -p "$GEN_DIR"
  case "$(init_system)" in
    launchd)
      write_plist "$GEN_DIR/$LABEL.plist"
      mkdir -p "$(dirname "$PLIST")" "$(dirname "$LOG")"
      if launchd_loaded; then launchctl bootout "$(launchd_target)/$LABEL" || true; fi
      cp "$GEN_DIR/$LABEL.plist" "$PLIST"
      launchctl bootstrap "$(launchd_target)" "$PLIST"
      echo "installed $PLIST (log: $LOG)"
      ;;
    systemd)
      write_unit "$GEN_DIR/$UNIT_NAME"
      mkdir -p "$(dirname "$UNIT")"
      cp "$GEN_DIR/$UNIT_NAME" "$UNIT"
      systemctl --user daemon-reload
      systemctl --user enable "$UNIT_NAME"
      echo "installed $UNIT (log: journalctl --user -u $UNIT_NAME)"
      ;;
  esac
}

uninstall() {
  case "$(init_system)" in
    launchd)
      if launchd_loaded; then launchctl bootout "$(launchd_target)/$LABEL" || true; fi
      rm -f "$PLIST" "$LOG" "$GEN_DIR/$LABEL.plist"
      ;;
    systemd)
      if [ -f "$UNIT" ]; then systemctl --user disable "$UNIT_NAME" || true; fi
      rm -f "$UNIT" "$GEN_DIR/$UNIT_NAME"
      systemctl --user daemon-reload || true
      ;;
  esac
  rmdir "$GEN_DIR" 2>/dev/null || true
  echo "autostart removed"
}

status() {
  installed=no loaded=no
  case "$(init_system)" in
    launchd)
      file="$PLIST"
      [ -f "$PLIST" ] && installed=yes
      launchd_loaded && loaded=yes
      ;;
    systemd)
      file="$UNIT"
      [ -f "$UNIT" ] && installed=yes
      systemctl --user is-enabled "$UNIT_NAME" >/dev/null 2>&1 && loaded=yes
      ;;
  esac
  echo "file:      $file"
  echo "installed: $installed"
  echo "loaded:    $loaded"
  [ "$installed" = yes ] && [ "$loaded" = yes ]
}

case "${1:-}" in
  install) install ;;
  uninstall) uninstall ;;
  status) status ;;
  *)
    echo "usage: $0 {install|uninstall|status}" >&2
    exit 2
    ;;
esac
