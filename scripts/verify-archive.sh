#!/bin/bash
# verify-archive.sh — S6 归档前检查清单自动化验证
#
# 对应规范: openspec/specs/project/archiving.md §2.1–§2.4
#
# Usage:
#   ./scripts/verify-archive.sh <change-id>          # 验证指定 change（archive 目录）
#   ./scripts/verify-archive.sh                       # 从当前分支名自动检测
#   ./scripts/verify-archive.sh --changes <change-id> # 验证 changes/ 目录（合并前）
#
# Exit: 0 = 全部通过, 1 = 存在问题

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass_count=0
fail_count=0
warn_count=0

pass() { echo -e "  ${GREEN}✓${NC} $1"; pass_count=$((pass_count + 1)); }
fail() { echo -e "  ${RED}✗${NC} $1"; fail_count=$((fail_count + 1)); }
warn() { echo -e "  ${YELLOW}⚠${NC} $1"; warn_count=$((warn_count + 1)); }

cd "$(dirname "$0")/.."
REPO_ROOT="$(pwd)"

# --- resolve change-id and target directory ---
CHANGES_MODE=false
if [ "$1" = "--changes" ]; then
    CHANGES_MODE=true
    shift
fi

if [ -n "$1" ]; then
    CHANGE_ID="$1"
else
    # Auto-detect from git branch name
    BRANCH=$(git branch --show-current)
    if echo "$BRANCH" | grep -qE '^feat/(archive-)?'; then
        CHANGE_ID="${BRANCH#feat/}"
        CHANGE_ID="${CHANGE_ID#archive-}"
    else
        echo "ERROR: cannot auto-detect change-id from branch '$BRANCH'"
        echo "Usage: $0 [--changes] <change-id>"
        exit 1
    fi
fi

if $CHANGES_MODE; then
    TARGET_DIR="$REPO_ROOT/openspec/changes/$CHANGE_ID"
    LABEL="changes/$CHANGE_ID"
else
    # Look in archive first, then changes
    ARCHIVE_DIR=$(find "$REPO_ROOT/openspec/archive" -maxdepth 1 -name "*${CHANGE_ID}" -type d 2>/dev/null | head -1)
    if [ -n "$ARCHIVE_DIR" ]; then
        TARGET_DIR="$ARCHIVE_DIR"
        LABEL="archive/$(basename "$ARCHIVE_DIR")"
    elif [ -d "$REPO_ROOT/openspec/changes/$CHANGE_ID" ]; then
        TARGET_DIR="$REPO_ROOT/openspec/changes/$CHANGE_ID"
        LABEL="changes/$CHANGE_ID"
    else
        echo "ERROR: change '$CHANGE_ID' not found in openspec/changes/ or openspec/archive/"
        exit 1
    fi
fi

echo "=== S6 归档检查清单验证: $LABEL ==="
echo ""

# ============================================================
# §2.1 文件完整性
# ============================================================
echo "§2.1 文件完整性"

# .openspec.yaml
if [ -f "$TARGET_DIR/.openspec.yaml" ]; then
    if grep -qE '^status:[[:space:]]*s7_archived' "$TARGET_DIR/.openspec.yaml"; then
        pass ".openspec.yaml 存在且 status=s7_archived"
    else
        cur_status=$(grep -E '^status:' "$TARGET_DIR/.openspec.yaml" | head -1)
        fail ".openspec.yaml status 不是 s7_archived (当前: $cur_status)"
    fi
else
    fail ".openspec.yaml 不存在"
fi

# proposal.md
if [ -f "$TARGET_DIR/proposal.md" ]; then
    if head -20 "$TARGET_DIR/proposal.md" | grep -qiE '(Status.*(Archived|S6|s6|s7)|阶段.*(S6|S7|归档))'; then
        pass "proposal.md 存在且标记为 Archived"
    else
        warn "proposal.md 存在但未确认 Archived 状态标记"
    fi
else
    fail "proposal.md 不存在"
fi

# demand.md
if [ -f "$TARGET_DIR/demand.md" ]; then
    pass "demand.md 存在"
else
    warn "demand.md 不存在（S1 未创建则正常）"
fi

# design.md
if [ -f "$TARGET_DIR/design.md" ]; then
    pass "design.md 存在"
else
    fail "design.md 不存在"
fi

# tasks.md
if [ -f "$TARGET_DIR/tasks.md" ]; then
    pass "tasks.md 存在"
else
    fail "tasks.md 不存在"
fi

# specs/*/spec.md
SPEC_FILES=$(find "$TARGET_DIR/specs" -name "spec.md" -type f 2>/dev/null)
if [ -n "$SPEC_FILES" ]; then
    pass "specs/*/spec.md 存在 ($(echo "$SPEC_FILES" | wc -l | tr -d ' ') 个)"
else
    fail "specs/*/spec.md 不存在"
fi

# acceptance-report.md
if [ -f "$TARGET_DIR/acceptance-report.md" ]; then
    if grep -qiE '(ACCEPTED|PASS|通过|验收通过)' "$TARGET_DIR/acceptance-report.md"; then
        pass "acceptance-report.md 存在且结论为 ACCEPTED"
    else
        warn "acceptance-report.md 存在但未确认 ACCEPTED/PASS 结论"
    fi
else
    fail "acceptance-report.md 不存在"
fi

echo ""

# ============================================================
# §2.2 状态一致性
# ============================================================
echo "§2.2 状态一致性"

if [ -f "$TARGET_DIR/.openspec.yaml" ] && [ -f "$TARGET_DIR/proposal.md" ]; then
    YAML_STATUS=$(grep -E '^status:' "$TARGET_DIR/.openspec.yaml" | sed 's/^status:[[:space:]]*//' | head -1 | xargs)
    PROP_STATUS=$(head -20 "$TARGET_DIR/proposal.md" | grep -iE '(Status:|阶段:)' | sed 's/.*Status:\s*//;s/.*阶段:\s*//' | head -1 | xargs)
    if echo "$PROP_STATUS" | grep -qiE "archived|s6|s7|归档"; then
        pass ".openspec.yaml status ($YAML_STATUS) 与 proposal.md ($PROP_STATUS) 一致（均为归档状态）"
    else
        fail ".openspec.yaml status=$YAML_STATUS 但 proposal.md 阶段=$PROP_STATUS（应均为归档状态）"
    fi
fi

if [ -f "$TARGET_DIR/.openspec.yaml" ]; then
    DEMAND_ID=$(grep -E '^demand_id:' "$TARGET_DIR/.openspec.yaml" | sed 's/^demand_id:[[:space:]]*//' | head -1 | xargs)
    if [ -n "$DEMAND_ID" ] && [ "$DEMAND_ID" != "null" ]; then
        if grep -qF "$DEMAND_ID" "$REPO_ROOT/openspec/demand-archive-index.md" 2>/dev/null; then
            pass "demand-id ($DEMAND_ID) 在 demand-archive-index.md 中有记录"
        else
            fail "demand-id ($DEMAND_ID) 在 demand-archive-index.md 中未找到"
        fi
    else
        warn "无法从 .openspec.yaml 提取 demand_id"
    fi
fi

echo ""

# ============================================================
# §2.3 索引更新
# ============================================================
echo "§2.3 索引更新"

# Check demand-archive-index.md has the change
if [ -f "$REPO_ROOT/openspec/demand-archive-index.md" ]; then
    if grep -q "$CHANGE_ID" "$REPO_ROOT/openspec/demand-archive-index.md"; then
        pass "demand-archive-index.md 包含 $CHANGE_ID"
    else
        fail "demand-archive-index.md 未包含 $CHANGE_ID"
    fi
fi

# Check T point registration
if [ -f "$TARGET_DIR/.openspec.yaml" ]; then
    T_POINTS=$(grep -cE '^  - D' "$TARGET_DIR/.openspec.yaml" 2>/dev/null || echo 0)
    if [ "$T_POINTS" -gt 0 ]; then
        # Check that at least one domain t-registry has been updated
        DOMAINS=$(grep -E '^domains:' "$TARGET_DIR/.openspec.yaml" | sed 's/^domains:[[:space:]]*\[//' | sed 's/\]//' | sed 's/,/ /g' | head -1)
        T_REG_FOUND=false
        for d in $DOMAINS; do
            d_lower=$(echo "$d" | tr '[:upper:]' '[:lower:]')
            T_REG_CANDIDATES=$(find "$REPO_ROOT/openspec/specs" -path "*/${d_lower}*/t-registry.md" -type f 2>/dev/null)
            for tr in $T_REG_CANDIDATES; do
                if grep -q "$CHANGE_ID" "$tr" 2>/dev/null; then
                    pass "T 点已在 $(echo $tr | sed "s|$REPO_ROOT/||") 注册（$T_POINTS 个 T 点）"
                    T_REG_FOUND=true
                    break 2
                fi
            done
        done
        if ! $T_REG_FOUND; then
            fail "域 t-registry.md 中未找到 $CHANGE_ID 的 T 点更新"
        fi
    else
        warn "no T points defined in .openspec.yaml"
    fi
fi

# Evaluate domain doc sync need (§2.4)
echo ""
echo "§2.4 域文档同步评估"

CHANGE_TYPE=""
if grep -qiE '(bug.?fix|修复|gap|漏洞|缺陷)' "$TARGET_DIR/proposal.md" 2>/dev/null; then
    CHANGE_TYPE="bugfix"
elif grep -qiE '(新增|new feature|add|feat)' "$TARGET_DIR/proposal.md" 2>/dev/null; then
    CHANGE_TYPE="feature"
else
    CHANGE_TYPE="unknown"
fi

case "$CHANGE_TYPE" in
    bugfix)
        warn "检测为 Bug 修复类型 — 按 §2.4 通常不需要域文档同步（请人工确认）"
        ;;
    feature)
        echo "  检测为新功能/架构变更 — 必须检查 §2.4 是否需要域文档同步"
        warn "请人工确认: spec.md / design.md / a-registry.md / f-registry.md 是否已更新"
        ;;
    *)
        warn "无法自动判断变更类型，请人工检查 §2.4 域文档同步需求"
        ;;
esac

echo ""

# ============================================================
# §2.1 补充: changes/ 目录清理确认
# ============================================================
if [ -d "$REPO_ROOT/openspec/changes/$CHANGE_ID" ] && ! $CHANGES_MODE; then
    fail "changes/$CHANGE_ID 仍然存在（归档后应移除）"
else
    pass "changes/$CHANGE_ID 已移除"
fi

echo ""

# ============================================================
# Summary
# ============================================================
echo "========================================"
echo "  结果: $pass_count 通过, $fail_count 失败, $warn_count 警告"
echo "========================================"

if [ "$fail_count" -gt 0 ]; then
    echo ""
    echo "请修复以上 ✗ 项后重新运行。"
    exit 1
elif [ "$warn_count" -gt 0 ]; then
    echo ""
    echo "所有关键项通过，但存在 ⚠ 警告 — 建议人工确认后继续。"
    exit 0
else
    echo ""
    echo "S6 归档检查清单全部通过。"
    exit 0
fi
