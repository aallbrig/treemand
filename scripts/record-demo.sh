#!/usr/bin/env bash
# Record all VHS demo tapes and concatenate into a single MP4.
#
# Usage:
#   bash scripts/record-demo.sh          # record all segments + stitch
#   bash scripts/record-demo.sh --keep   # keep intermediate segment files
#
# Prerequisites: vhs, ffmpeg, treemand (on $PATH)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEMOS_DIR="$REPO_ROOT/demos"
SEGMENTS_DIR="$DEMOS_DIR/segments"
OUTPUT_DIR="$REPO_ROOT/dist"
OUTPUT="$OUTPUT_DIR/demo.mp4"
KEEP_SEGMENTS=false

if [[ "${1:-}" == "--keep" ]]; then
  KEEP_SEGMENTS=true
fi

# Preflight checks
for cmd in vhs ffmpeg treemand; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "Error: $cmd is not installed or not on \$PATH" >&2
    exit 1
  fi
done

mkdir -p "$SEGMENTS_DIR" "$OUTPUT_DIR"

# Step 1: Record each numbered tape
echo "==> Recording demo segments..."
for tape in "$DEMOS_DIR"/[0-9][0-9]_*.tape; do
  name=$(basename "$tape" .tape)
  echo "  Recording: $name"
  vhs "$tape"
done

# Step 2: Build ffmpeg concat list
echo "==> Concatenating segments into $OUTPUT..."
CONCAT_FILE=$(mktemp)
trap 'rm -f "$CONCAT_FILE"' EXIT

for seg in "$SEGMENTS_DIR"/[0-9][0-9]_*.mp4; do
  echo "file '$(realpath "$seg")'" >> "$CONCAT_FILE"
done

# Step 3: Concatenate all segments
ffmpeg -y -f concat -safe 0 -i "$CONCAT_FILE" -c copy "$OUTPUT" 2>/dev/null

# Step 4: Clean up segments (unless --keep)
if [[ "$KEEP_SEGMENTS" == "false" ]]; then
  rm -rf "$SEGMENTS_DIR"
fi

echo "==> Done: $OUTPUT"
ls -lh "$OUTPUT"
