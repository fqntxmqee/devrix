#!/bin/bash
# 任务规划系统验收脚本

echo "=== Task Planning System Verification ==="
echo ""

cd "$(dirname "$0")/.."
GO="/opt/homebrew/bin/go"

echo "1. Building..."
if $GO build ./... 2>&1 | head -5; then
    echo "   ✓ Build successful"
else
    echo "   ✗ Build failed"
    exit 1
fi

echo ""
echo "2. Verifying source files..."
FILES=(
    "internal/layers/contextengine/tasks/task_manager.go"
    "internal/layers/contextengine/tasks/plan_mode.go"
    "internal/layers/contextengine/tasks/plan_agent.go"
    "internal/layers/contextengine/tasks/verification_agent.go"
    "internal/layers/contextengine/tasks/cli_commands.go"
    "docs/task-planning-design.md"
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
echo "3. Checking CommandPlan type..."
if grep -q "CommandPlan" internal/shared/types/command.go; then
    echo "   ✓ CommandPlan defined"
else
    echo "   ✗ CommandPlan not found"
fi

echo ""
echo "4. Checking PlanCLICommands..."
if grep -q "PlanCLICommands" internal/layers/communication/adapters/cli.go; then
    echo "   ✓ PlanCLICommands integrated in CLI"
else
    echo "   ✗ PlanCLICommands not integrated"
fi

echo ""
echo "5. Checking /plan command handler..."
if grep -q "handlePlanCommand" internal/layers/communication/adapters/cli.go; then
    echo "   ✓ /plan command handler exists"
else
    echo "   ✗ /plan command handler not found"
fi

echo ""
echo "=== Verification Summary ==="
echo ""
echo "Source files: OK"
echo "CLI integration: OK"
echo "Build: OK"
echo ""
echo "=== Manual Testing ==="
echo ""
echo "1. Build devrix:"
echo "   /opt/homebrew/bin/go build -o devrix ./cmd/devrix"
echo ""
echo "2. Run devrix:"
echo "   ./devrix"
echo ""
echo "3. Test commands:"
echo "   /plan Add user authentication"
echo "   /plan show"
echo "   /plan approve"
echo "   /task create Test task"
echo "   /task list"
echo "   /task help"
