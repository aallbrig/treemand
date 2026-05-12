#!/bin/bash
# Install git hooks for treemand development
# Run with: bash scripts/setup-hooks.sh

set -e

HOOKS_DIR=".git/hooks"

# ── pre-commit ──────────────────────────────────────────────────────────────
echo "Installing git pre-commit hook..."

cat > "$HOOKS_DIR/pre-commit" << 'EOF'
#!/bin/bash
# Pre-commit hook: run full code hygiene suite before every commit.
# Install with: bash scripts/setup-hooks.sh

set -e

if ! command -v task &>/dev/null; then
  echo "task not found — skipping pre-commit checks (install go-task to enable)"
  exit 0
fi

echo "Running precommit checks (task precommit)..."
task precommit
echo "Precommit checks passed."
EOF

chmod +x "$HOOKS_DIR/pre-commit"
echo "  pre-commit hook installed at $HOOKS_DIR/pre-commit"

# ── pre-push ────────────────────────────────────────────────────────────────
echo "Installing git pre-push hook..."

cat > "$HOOKS_DIR/pre-push" << 'EOF'
#!/bin/bash
# Pre-push hook: run precommit checks before pushing to main/develop.
# Install with: bash scripts/setup-hooks.sh

set -e

remote="$1"
url="$2"

while read local_ref local_oid remote_ref remote_oid; do
  branch=$(echo "$remote_ref" | sed 's#refs/heads/##')

  if [[ "$branch" == "main" || "$branch" == "develop" ]]; then
    echo "Running precommit checks before push to $branch..."

    if ! command -v task &>/dev/null; then
      echo "task not found — skipping pre-push checks"
      exit 0
    fi

    task precommit
    echo "Precommit checks passed."
  fi
done

exit 0
EOF

chmod +x "$HOOKS_DIR/pre-push"
echo "  pre-push hook installed at $HOOKS_DIR/pre-push"

echo ""
echo "Git hooks installed. Run 'task precommit' at any time to check manually."
