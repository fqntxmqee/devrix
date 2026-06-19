# Proposal: D2 spec 退役标记完整性

**Change ID:** devrix-spec-sync-d2-layer-delta-soften
**Demand ID:** DM-20260619-004
**Status:** S7_Archived (2026-06-19)
**Date:** 2026-06-19
**Author:** Devrix Team

> **docs-only change**：本 change 仅改 D2 spec 两份文档（layer-delta.md / d7-boundary.md），不动 D2 域代码、不改 D-S 编号、不删 D2 Scenarios（保留回滚兼容）。

---

## 1. 背景

D2 QueryLoop 退役是 D2 → D7 编排上移的伴生过程，2026-06-15 DM-020（`devrix-d7-turn-orchestration`）完成后 D7-S2-A06 RunTurnLoop 成为 canonical 主路径；2026-06-17 DM-20260617-001（`devrix-queryloop-legacy-decommission`）给 D2-S10 QueryLoop 加 3 信号 deprecation 契约（metric + warn + spec）。

D2 spec.md §18 已有 ⚠️ LEGACY 标记（2026-06-17，DM-20260617-001）说明：
- canonical 主路径是 D7-S2-A06 RunTurnLoop
- `loopFirst=true` 是默认
- Loop.Run 函数体逻辑保留（紧急回滚兜底）
- 本 spec 章节（D2-S10）所有 Requirement 与 Scenario **保留** 用于回滚兼容

但 D2 spec **三层退役标记**（spec.md LEGACY / layer-delta.md ADDED / d7-boundary.md 契约）未保持一致。

## 2. 问题陈述

### 2.1 layer-delta.md ADDED — QueryLoop Primary Runtime 措辞过强

`openspec/specs/d2-context-engine/layer-delta.md` line 12-14：

```markdown
### Requirement: QueryLoop Primary Runtime

When `context_engine.query_loop.enabled=true` (default since DM-20260611-004),
`ContextEngine.Process` MUST route all LLM↔Tool rounds through `query.Loop.Run`
instead of the retired PEV engine.
```

"Primary" + "MUST route" 措辞与 spec.md §18 LEGACY 标记矛盾——按 DM-20260617-001，QueryLoop 在 `loopFirst=false` 路径下已 Deprecated，canonical 主路径是 D7-S2-A06 RunTurnLoop。本 Requirement 需要加 Deprecated 注脚。

### 2.2 d7-boundary.md §79 LoopHooks 引用存在但无 Loop.Run Deprecated 状态

`openspec/specs/d2-context-engine/d7-boundary.md` line 79：

```markdown
| `LoopHooks` | `query/loop.go` | D7 注入 | D2 Loop |
```

引用存在但无 DEPRECATED 状态标注。按 DM-20260617-001，Loop.Run 在 `loopFirst=false` 路径下应显式标 DEPRECATED。

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | layer-delta.md "QueryLoop Primary Runtime" Requirement 加 **DEPRECATED (loopFirst=false path; DM-20260617-001)** 注脚 | P0 |
| AC2 | layer-delta.md "QueryLoop Primary Runtime" 措辞调整：移除 "MUST route all"，改为 "（Default 但已被 D7 RunTurnLoop 取代，loopFirst=false 路径 DEPRECATED）" | P0 |
| AC3 | d7-boundary.md §79 LoopHooks 行加 DEPRECATED 状态列：`DEPRECATED (loopFirst=false; canonical=D7-S2-A06 RunTurnLoop)` | P0 |
| AC4 | d7-boundary.md §4 契约表（Loop.Run 契约）补 DEPRECATED 状态标注 | P0 |
| AC5 | layer-delta.md / d7-boundary.md `Last Updated` 同步至 2026-06-19 | P0 |
| AC6 | `go vet ./...` 0 错 | P1 |
| AC7 | `verify-archive.sh openspec/changes/devrix-spec-sync-d2-layer-delta-soften` 全部 PASS | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖（已闭环）| `devrix-d7-turn-orchestration` (DM-20260614-020) — D7 Turn 编排上移 |
| 依赖（已闭环）| `devrix-queryloop-legacy-decommission` (DM-20260617-001) — 3 信号 deprecation 契约 |
| 依赖（已存在）| spec.md §18 LEGACY 标记（2026-06-17）— 本 change 同步其他两层 |
| 约束 | docs-only，不动 .go 源码 |
| 约束 | D2 Scenarios 不删除（保留回滚兼容，per spec.md §18） |
| 约束 | 沿用 Devrix GitHub Flow：`feat/spec-sync-d2-layer-delta-soften` 分支 + squash merge + auto-merge |

## 5. 变更范围

### 修改 2 个
- `openspec/specs/d2-context-engine/layer-delta.md` — QueryLoop Primary Runtime Requirement 措辞软化 + DEPRECATED 注脚
- `openspec/specs/d2-context-engine/d7-boundary.md` — §79 LoopHooks + §4 契约表补 DEPRECATED 状态

### 新建 5 个
- `openspec/changes/devrix-spec-sync-d2-layer-delta-soften/.openspec.yaml`
- `openspec/changes/devrix-spec-sync-d2-layer-delta-soften/proposal.md`（本文件）
- `openspec/changes/devrix-spec-sync-d2-layer-delta-soften/design.md`（S3）
- `openspec/changes/devrix-spec-sync-d2-layer-delta-soften/tasks.md`（S4）
- `openspec/changes/devrix-spec-sync-d2-layer-delta-soften/acceptance-report.md`（S5）

### 不变更
- `internal/layers/contextengine/**` 全部代码
- `openspec/specs/d2-context-engine/spec.md` §18 LEGACY 标记（已存在，保持）
- D2 Scenarios（保留回滚兼容）
