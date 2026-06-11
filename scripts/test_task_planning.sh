#!/bin/bash
set -e

echo "=== Task Planning System Verification ==="
echo ""

cd "$(dirname "$0")/.."

echo "1. Building..."
if go build ./... 2>&1; then
    echo "   ✓ Build successful"
else
    echo "   ✗ Build failed"
    exit 1
fi

echo ""
echo "2. Checking task manager..."
go run -exec "echo" ./internal/layers/contextengine/tasks/ 2>&1 | head -1 || true

echo ""
echo "3. Verifying source files..."
FILES=(
    "internal/layers/contextengine/tasks/task_manager.go"
    "internal/layers/contextengine/tasks/plan_mode.go"
    "internal/layers/contextengine/tasks/plan_agent.go"
    "internal/layers/contextengine/tasks/verification_agent.go"
    "internal/layers/contextengine/tasks/cli_commands.go"
)

for f in "${FILES[@]}"; do
    if [ -f "$f" ]; then
        echo "   ✓ $f"
    else
        echo "   ✗ Missing: $f"
        exit 1
    fi
done

echo ""
echo "4. Checking command types..."
if grep -q "CommandPlan" internal/shared/types/command.go; then
    echo "   ✓ CommandPlan defined"
else
    echo "   ✗ CommandPlan not found"
fi

if grep -q "CommandTask" internal/shared/types/command.go; then
    echo "   ✓ CommandTask defined"
else
    echo "   ✗ CommandTask not found"
fi

echo ""
echo "5. Checking CLI integration..."
if grep -q "PlanCLICommands" internal/layers/communication/adapters/cli.go; then
    echo "   ✓ PlanCLICommands integrated"
else
    echo "   ✗ PlanCLICommands not found"
fi

echo ""
echo "=== Verification Complete ==="
echo ""
echo "To test manually, run:"
echo "  ./devrix"
echo ""
echo "Then try:"
echo "  /plan Add user authentication"
echo "  /plan show"
echo "  /plan approve"
echo "  /task list"
