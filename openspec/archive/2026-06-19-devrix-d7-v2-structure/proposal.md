# Proposal: D7 v2.0 结构重构

**Change ID:** `devrix-d7-v2-structure`  
**Demand ID:** DM-20260619-005  
**Status:** Archived  
**Created:** 2026-06-19  
**Methodology:** `docs/methodology/dsaft-refactoring-playbook.md` §4 v2.0 Structure  
**Tech Debt:** `openspec/tech-debt/worktree-v2-deferred.md` TD-WT-02, TD-WT-03

---

## Problem Statement

D7 功能已闭环，但 **架构双锚点断裂**：

1. **规格锚点**（`openspec/specs/d7-orchestration/`）与 **物理锚点**（`internal/layers/orchestration/`）不一致  
2. `coordinator/` 单包承载 S2 + S5，违背价值流切分  
3. WorkTree 语义 SoT 与 Wave TaskNode 持久化并存（TD-WT-02）  
4. 多份规格文档状态标注过时（design.md 仍标 PLANNED）

## Proposed Solution

三阶段交付（Owner 确认范围 **C**）：

```
Phase A: 规格同步（零代码）
Phase B: 物理路径对齐 S 层（参考 D6 eval→evaluate 迁移模式）
Phase C: WorkTree Legacy 清债（TD-WT-02/03）
```

### 核心策略

**先文档闭合 → 分 PR 迁路径（B1→B4）→ 最后清数据模型债**

| Phase | 内容 | 风险 |
|-------|------|------|
| A | design / layer-delta / d7-boundary / code-layout 同步 | 低 |
| B1 | `wave/` → `wavescheduler/` | 低 |
| B2 | S4 → `executionflow/{hub,workplan,imsink}/` | 中 |
| B3 | coordinator 拆包 → sessionorchestrator + decisionplanning | 高 |
| B4 | hubspoke 按 S2/S4 拆分 | 中 |
| C | TaskNode 投影化 + sc.Todos 降级 | 中 |

### 编排权模型（Owner Q2=A）

```text
sessionorchestrator.Entry.ProcessMessage
    ├── decisionplanning.IntentClassifier.Classify
    ├── decisionplanning.TaskDecomposer.SynthesizeTaskGraph
    ├── sessionorchestrator.FastPath / OrchestratePath / CommandHandler
    ├── turn.TurnOrchestrator.RunTurn
    └── executionflow.Hub.Publish
```

S5 只产结构决策，不拥有 ingress。

## Scope

### In Scope

- Phase A：D7 规格文档漂移修复
- Phase B：§4.2 code-layout 登记的 4 个 S 层路径迁移
- Phase B：`hubspoke` dispatch/bridge 物理拆分
- Phase B：`coordinator` 1-release shim re-export
- Phase C：TD-WT-02（TaskNode 投影化）、TD-WT-03（sc.Todos 降级）
- 66 既有 T 全绿验证（T ID 不变）

### Out of Scope

- D2 engine 改造（Owner Q5=A）
- TD-WT-01/04/05/06
- 新增 T 点
- North Star / D-S 编号变更

## Impact Analysis

| 影响面 | 说明 |
|--------|------|
| bootstrap | `wire_coordinator.go` import 路径更新 |
| tests | integration/d7/* 随包迁移 |
| layer-lint | D7 boundary 规则可能需更新路径白名单 |
| 外部 API | `IOrchestrationEntry` 契约不变；仅内部包路径变 |
| 文档 | request-flow / code-atlas 需 follow-up（可本 change 或独立 docs change） |

## Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| 大 PR 爆炸 | 按 B1→B4 分 4 个 PR，每 PR 独立全绿 |
| import 遗漏 | coordinator shim 保留 1 release；CI grep 旧路径 |
| WorkTree 回归 | Phase C 在路径稳定后执行；沿用现有 T07/T 覆盖 |
| 规格再次漂移 | Phase A 先于 Phase B merge |

## Success Criteria

- `code-layout.md` §4.2 D7 表 5/5 ✅
- 66/66 T IMPLEMENTED 保持
- TD-WT-02/03 CLOSED
- `go test ./...` + layer-lint strict PASS

## Related Documents

| 文档 | 关系 |
|------|------|
| `d7-domain.md` | North Star SoT（不变） |
| `code-layout.md` §4.2 | 物理路径目标 |
| `worktree-v2-deferred.md` | Phase C 清债来源 |
| D6 `devrix-d5-d6-sa-refine-v2.0` | 物理迁移参考模式 |
