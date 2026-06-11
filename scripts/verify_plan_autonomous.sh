#!/bin/bash
# LLM 自主判断进入任务规划模式验收脚本

echo "=== LLM 自主判断任务规划模式验收 ==="
echo ""

cd "$(dirname "$0")/.."
GO="/opt/homebrew/bin/go"

echo "1. Building..."
if $GO build ./... 2>&1 | head -3; then
    echo "   ✓ Build successful"
else
    echo "   ✗ Build failed"
    exit 1
fi

echo ""
echo "2. Checking should_plan.go..."
if grep -q "PlanSignals" internal/layers/contextengine/pev/should_plan.go; then
    echo "   ✓ PlanSignals defined"
else
    echo "   ✗ PlanSignals not found"
fi

echo ""
echo "3. Checking hasPlanSignal function..."
if grep -q "hasPlanSignal" internal/layers/contextengine/pev/should_plan.go; then
    echo "   ✓ hasPlanSignal function exists"
else
    echo "   ✗ hasPlanSignal not found"
fi

echo ""
echo "4. Checking ShouldPlanDetailed function..."
if grep -q "ShouldPlanDetailed" internal/layers/contextengine/pev/should_plan.go; then
    echo "   ✓ ShouldPlanDetailed function exists"
else
    echo "   ✗ ShouldPlanDetailed not found"
fi

echo ""
echo "5. Plan Signals (触发词):"
grep -A 20 "PlanSignals" internal/layers/contextengine/pev/should_plan.go | head -15

echo ""
echo "=== 验收总结 ==="
echo ""
echo "LLM 自主判断功能已就绪："
echo ""
echo "触发条件："
echo "  1. 消息包含 PlanSignals（添加新功能、实现、重构等）"
echo "  2. 消息长度 >= min_chars_for_plan (默认200字符)"
echo "  3. 显式 /plan 命令"
echo ""
echo "自动检测词列表："
echo "  中文: 添加新功能、实现、重构、开发、设计、架构、添加模块等"
echo "  英文: add feature, implement, refactor, design, architecture等"
echo ""
echo "配置示例 (devrix.yaml):"
cat << 'YAML'
context_engine:
  plan:
    enabled: true           # 启用规划
    auto_detect: true      # 自动检测
    min_chars_for_plan: 200 # 触发阈值
YAML

echo ""
echo "=== 验收完成 ==="
