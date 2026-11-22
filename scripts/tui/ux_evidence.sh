#!/bin/sh
# Capture treemand's interactive TUI in realistic task flows, as evidence for
# the treemand-tui-ux skill to judge. Each flow (scripts/tui/ux_flows.txt) runs
# in a private tmux server at every size; every step leaves a frame:
#   <out>/<size>/<flow>/<step>.ans   capture-pane -e (ANSI colours kept)
#   <out>/<size>/<flow>/<step>.txt   the same frame as plain text
#   <out>/<size>/<flow>/<step>.png   rendered by vhs, when vhs is installed
#   <out>/index.tsv                  size, flow, step, keys, frame, check
#   <out>/checks.txt                 deterministic failures (empty when clean)
# Output defaults to .loop/tui-ux/<UTC date>/ (gitignored).
#
#   sh scripts/tui/ux_evidence.sh [--out DIR] [--sizes "80x24 120x40"] [--flows "open help"] [--no-png]
#   sh scripts/tui/ux_evidence.sh --render-only --out DIR   (re-render DIR's frames to PNG)
#
# The TUI runs from an empty scratch directory with a throwaway HOME, config
# and cache (treemand-safe-real-cli-run), so it sees a first run. UX_BIN runs a
# prebuilt binary instead of building one; UX_FLOW_FILE swaps the flow file.
# Exits 1 when a deterministic check fails (crash, panic, empty frame).
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"

out=""
sizes="80x24 120x40 200x60"
flows=""
png=1
capture=1
while [ $# -gt 0 ]; do
  case "$1" in
    --out) out="$2"; shift 2 ;;
    --sizes) sizes="$2"; shift 2 ;;
    --flows) flows="$2"; shift 2 ;;
    --no-png) png=0; shift ;;
    --render-only) capture=0; shift ;;
    *) echo "ux_evidence: unknown argument $1" >&2; exit 2 ;;
  esac
done
[ -n "$out" ] || out="$root/.loop/tui-ux/$(date -u +%Y%m%d_%H%M%S_UTC)"
flow_file="${UX_FLOW_FILE:-$here/ux_flows.txt}"
command -v tmux >/dev/null 2>&1 || { echo "ux_evidence: needs tmux" >&2; exit 2; }

mkdir -p "$out"
out="$(cd "$out" && pwd)"
scratch="$(mktemp -d)"
sock="treemand-ux-$$"
tm() { tmux -L "$sock" "$@"; }
cleanup() { tm kill-server 2>/dev/null || true; rm -rf "$scratch"; }
trap cleanup EXIT INT TERM
mkdir -p "$scratch/work" "$scratch/home/.config" "$scratch/bin"

bin="${UX_BIN:-}"
if [ "$capture" = 1 ] && [ -z "$bin" ]; then
  bin="$scratch/treemand"
  (cd "$root/cli/treemand" && go build -o "$bin" .)
fi

# @broken: a CLI whose help always fails. Discovery error states come from it.
cat >"$scratch/bin/broken-cli" <<'EOF'
#!/bin/sh
echo "broken-cli: cannot load plugin registry (simulated failure)" >&2
exit 2
EOF
chmod +x "$scratch/bin/broken-cli"


# settle captures the pane until it has not changed for $2 polls 0.3 s apart
# (spinners and background discovery keep it changing), at most ~20 s, and
# leaves the frame in $1.
settle() {
  prev=""
  same=0
  i=0
  sleep 0.3
  while [ "$i" -lt 66 ]; do
    cur="$(tm capture-pane -p -e -t ux 2>/dev/null || true)"
    if [ -n "$cur" ] && [ "$cur" = "$prev" ]; then
      same=$((same + 1))
      [ "$same" -ge "$2" ] && break
    else
      same=0
    fi
    prev="$cur"
    i=$((i + 1))
    sleep 0.3
  done
  printf '%s\n' "$cur" >"$1"
}

send_keys() {
  for k in $1; do
    case "$k" in
      lit:*) tm send-keys -t ux -l "${k#lit:}" ;;
      *) tm send-keys -t ux "$k" ;;
    esac
    sleep 0.15
  done
}

start_tui() { # target cols rows cache
  case "$1" in
    @broken) t="broken-cli" ;;
    @missing) t="no-such-cli-for-ux" ;;
    *) t="$1" ;;
  esac
  tm kill-server 2>/dev/null || true
  # The trailing marker and sleep keep the pane, so an exit or crash is captured.
  env -u TMUX tmux -L "$sock" -f /dev/null new-session -d -x "$2" -y "$3" -s ux \
    "cd '$scratch/work' && env HOME='$scratch/home' XDG_CONFIG_HOME='$scratch/home/.config' TREEMAND_CACHE_DIR='$4' PATH='$scratch/bin:$PATH' TERM=xterm-256color COLORTERM=truecolor '$bin' -i '$t'; echo \"[treemand exited \$?]\"; sleep 3600"
}

check_frame() { # txt flow step -> echoes a check verdict
  if grep -qiE 'panic:|fatal error:|goroutine [0-9]+ \[' "$1"; then echo "panic"; return; fi
  if [ "$2" != "error_missing" ] && grep -q '^\[treemand exited' "$1"; then echo "exited"; return; fi
  if ! grep -q '[^[:space:]]' "$1"; then echo "empty"; return; fi
  echo "ok"
}

capture_all() {
  printf 'size\tflow\tstep\tkeys\tframe\tcheck\n' >"$out/index.tsv"
  : >"$out/checks.txt"
  esc="$(printf '\033')"
  set -f
  for size in $sizes; do
    cols="${size%x*}"
    rows="${size#*x}"
    cur_flow=""
    skip=0
    while IFS='|' read -r flow target step keys; do
      case "$flow" in ''|'#'*) continue ;; esac
      if [ -n "$flows" ]; then
        case " $flows " in *" $flow "*) ;; *) continue ;; esac
      fi
      dir="$out/$size/$flow"
      if [ "$flow" != "$cur_flow" ]; then
        cur_flow="$flow"
        skip=0
        mkdir -p "$dir"
        # open shows a cold first run; the other flows share a warm cache.
        cache="$scratch/cache-warm"
        [ "$flow" = open ] && cache="$scratch/cache-cold-$size"
        start_tui "$target" "$cols" "$rows" "$cache"
        if [ "$step" = 01_loading ]; then
          sleep 0.4
          tm capture-pane -p -e -t ux >"$dir/$step.ans"
        else
          settle "$dir/$step.ans" 4
        fi
      elif [ "$skip" = 1 ]; then
        continue
      else
        send_keys "$keys"
        settle "$dir/$step.ans" 2
      fi
      sed "s/${esc}\[[0-9;:?]*[A-Za-z]//g; s/${esc}[()][A-Z0-9]//g" "$dir/$step.ans" >"$dir/$step.txt"
      verdict="$(check_frame "$dir/$step.txt" "$flow" "$step")"
      rel="$size/$flow/$step"
      printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$size" "$flow" "$step" "${keys:--}" "$rel" "$verdict" >>"$out/index.tsv"
      if [ "$verdict" != ok ]; then
        echo "$rel: $verdict after keys '${keys:--}'" >>"$out/checks.txt"
        skip=1 # the TUI is gone; later steps of this flow would only repeat it
      fi
    done <"$flow_file"
  done
  set +f
  tm kill-server 2>/dev/null || true
}
[ "$capture" = 0 ] || capture_all

# Render every frame to PNG with vhs: one tape per size prints each .ans file
# in a terminal a little larger than the frame and screenshots it.
if [ "$png" = 1 ] && command -v vhs >/dev/null 2>&1; then
  for size in $sizes; do
    [ -d "$out/$size" ] || continue
    cols="${size%x*}"
    rows="${size#*x}"
    tape="$scratch/render-$size.tape"
    {
      echo "Output \"$scratch/render-$size.gif\""
      echo 'Set Shell "bash"'
      echo 'Set FontSize 14'
      echo 'Set Padding 12'
      echo 'Set TypingSpeed 0'
      echo "Set Width $((cols * 9 + 60))"
      echo "Set Height $(((rows + 2) * 18 + 60))"
      echo 'Hide'
      echo "Type \"export PS1=''; stty -echo\""
      echo 'Enter'
      echo 'Show'
      # An empty prompt and stty -echo keep the typed commands off screen.
      # Each frame ends with a numbered caption; waiting for it on screen is
      # what makes the screenshot hold this frame and not the one before
      # (Screenshot saves the next recorded frame, hence the pause after it).
      n=0
      for ans in "$out/$size"/*/*.ans; do
        n=$((n + 1))
        rel="${ans#"$out"/}"
        echo "Type \"clear; cat '$ans'; printf '[frame $n: ${rel%.ans}]'\""
        echo 'Enter'
        echo "Wait+Screen@10s /\\[frame $n:/"
        echo 'Sleep 500ms'
        echo "Screenshot \"${ans%.ans}.png\""
        echo 'Sleep 500ms'
      done
    } >"$tape"
    vhs "$tape" >"$scratch/vhs-$size.log" 2>&1 || { echo "ux_evidence: vhs failed at $size (see below)" >&2; tail -5 "$scratch/vhs-$size.log" >&2; }
  done
elif [ "$png" = 1 ]; then
  echo "ux_evidence: vhs not found; frames are ANSI and text only" >&2
fi

frames="$(($(wc -l <"$out/index.tsv") - 1))"
echo "ux_evidence: $frames frames in $out"
if [ -s "$out/checks.txt" ]; then
  echo "ux_evidence: deterministic checks failed:" >&2
  cat "$out/checks.txt" >&2
  exit 1
fi
