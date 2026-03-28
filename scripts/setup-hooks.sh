#!/bin/bash
# Setup git hooks for treemand development
# Run with: bash scripts/setup-hooks.sh

set -e

HOOKS_DIR=".git/hooks"

echo "Installing git pre-push hook..."

cat > "$HOOKS_DIR/pre-push" << 'EOF'
#!/bin/bash
# Pre-push hook: run lint and tests before allowing push to main/develop
# This prevents committing code that fails linting or tests

set -e

remote="$1"
url="$2"

while read local_ref local_oid remote_ref remote_oid; do
  # Extract branch name (e.g. refs/heads/main → main)
  branch=$(echo "$remote_ref" | sed 's#refs/heads/##')
  
  # Enforce lint/test on pushes to main and develop
  if [[ "$branch" == "main" || "$branch" == "develop" ]]; then
    echo "🔍 Running lint and tests before push to $branch..."
    
    if ! make lint > /dev/null 2>&1; then
      echo "❌ Lint failed. Run: make lint"
      exit 1
    fi
    
    if ! make test > /dev/null 2>&1; then
      echo "❌ Tests failed. Run: make test"
      exit 1
    fi
    
    echo "✓ Lint and tests passed"
  fi
done

exit 0
EOF

chmod +x "$HOOKS_DIR/pre-push"
echo "✓ Pre-push hook installed at $HOOKS_DIR/pre-push"
echo ""
echo "The hook will automatically run before pushing to main or develop"
echo "to ensure all code passes linting and testing."
