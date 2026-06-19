# Proposal: 链路图文档对齐 D7 v2.0

**Change ID:** devrix-docs-request-flow-v2
**Demand ID:** DM-20260619-001
**Status:** S7_Archived（2026-06-19，docs-only 快速通道；PR #89 merged）
**Date:** 2026-06-19
**Author:** Devrix Team

> **docs-only change**：本 change 仅改 3 个架构文档，不动 D7 域代码、不改 spec.md（v3.8.0 是 SoT，docs 单向对齐 spec）。

---

## 1. 背景

Devrix 编排层 D7 自 2026-06-15 完成 v1.0 + v1.1 closure（PR #30 + #35 + #36），2026-06-18 又完成 v2.0 unified task tools 闭环（PR #83-#87 + #88）。**主入口从 D2 迁移到 D7-S2 `coordinator.Entry.ProcessMessage`**，并落地了 4 IntentKind → 4 真实执行链的正交分发。

但 3 个对外暴露链路图的核心架构文档**停留在 2026-06-13 的 QueryLoop 时代**：

| 文档 | 版本 | 实际状态 | 与当前 D7 v3.8.0 错位程度 |
|------|------|---------|--------------------------|
| `docs/architecture/request-flow.md` | 2026-06-13（无版本号） | QueryLoop 时代 D1→D2.QueryLoop.Run→D3 | **严重**（D1→D2 整条主路径错；4 IntentKind 完全未体现） |
| `openspec/specs/architecture/code-atlas.md` | v1.1.0（2026-06-13） | QueryLoop v2 D-S Index，D7 v2.0 0 体现 | **严重**（D-S Index 全部是 contextengine/query/*，v2.0 unified 一行未提） |
| `docs/architecture/dsaft-overview.md` | v1.0.0（2026-06-13） | 域架构图 "ORCH 读模型，非 D7" | **部分**（D7 是核心域的升级未反映；切法 A 博弈角色未对齐 v1.0 实现） |

这 3 份文档是新人 oncall / 跨团队 onboarding / 架构 review 的**第一入口**，过期会直接误导。

## 2. 问题陈述

### 2.1 request-flow.md 与代码严重错位

旧链路图（已过期）：
```
D1 → D2.runProcess → D2.QueryLoop.Run → D3 LLM
```

当前实际链路（v3.8.0）：
```
D1 → D7-S2 coordinator.ProcessMessage
        ├─ ClassifyIntent (rule + LLM fallback, 置信度 ≥ 0.9)
        └─ switch IntentKind → 4 真实链 (Skip/Command/Fast/Orchestrate)
        └─ D7-S2-A06 turn.RunTurn (resolve/decompose loop, v2.0 新增)
             ├─ D7-S2-A07 InvokeLLM → D3 (D7 直调, import ban D2→D3)
             └─ D2 ContextPreparer / ToolRoundExecutor / SessionPersister
```

### 2.2 code-atlas.md 完全未体现 D7 v2.0

- D-S Index 仍列 `query_loop` / `subquery` / `sidechain_transcript` 等已**退役或降级**的模块
- **未列**：`coordinator.Entry` / `turn/orchestrator.RunTurn` / `workitem` / `RunRegistry` / `ResolveAwaiter` / `wire_coordinator.go::WireD7`
- Dependency Direction 图仍写 D2 QueryLoop → ORCH Hub → D2 SessionQueue，**主路径已迁移到 D7 内**

### 2.3 dsaft-overview.md 部分过期

- 域架构图：ORCH 标 "读模型，非 D7"，但 2026-06-15 起 D7 已是独立核心域（PR #30 升格 + PR #36 v1.0 闭环）
- 切法 A（按用户价值流）：D7-S2 "Screening + Turn Leader" 已实现，但未标 IMPLEMENTED 状态
- "D2 上下文域（现行重点）"整节是 QueryLoop 时代视角；D7 才是当前生产主入口

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `request-flow.md` 整体重写，含 4 IntentKind → 4 真实链 + turn.RunTurn + D7 直调 D3 | P0 |
| AC2 | `code-atlas.md` v1.1.0 → v1.2.0：D-S Index 替换为 D7 v2.0 unified；新增 wire_coordinator.go | P0 |
| AC3 | `dsaft-overview.md` v1.0.0 → v1.1.0：D7 域架构升级；D7-S 层 5 个标 IMPLEMENTED | P0 |
| AC4 | 3 文档中"已退役/已降级"模块明确标 DEPRECATED（`query_loop` / `subquery` / `harness/`）| P0 |
| AC5 | 3 文档中代码锚点（如 `coordinator/orchestrator.go:ProcessMessage`）与实际仓库 grep 一致 | P0 |
| AC6 | `verify-archive.sh openspec/changes/devrix-docs-request-flow-v2` 全部 PASS | P0 |
| AC7 | `go vet ./...` 0 错（docs-only 改动不应破 go 编译）| P1 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖（SoT 不动）| `openspec/specs/d7-orchestration/spec.md` v3.8.0（2026-06-18） |
| 依赖（已闭环）| PR #35（orthogonal-intent-paths v1.1.0）/ PR #83-#87（unified-work-tree v2.0）/ PR #30 + #36（D7 v1.0 closure）|
| 约束 | docs-only，不动 .go 源码 |
| 约束 | D7 spec v3.8.0 不动；docs 单向对齐 spec |
| 约束 | 沿用 Devrix GitHub Flow：`feat/docs-request-flow-v2` 分支 + squash merge + auto-merge |
| 约束 | 归档后 `demand-archive-index.md` 追加一行 |

## 5. 变更范围

### 修改 3 个
- `docs/architecture/request-flow.md` — 整体重写
- `openspec/specs/architecture/code-atlas.md` — v1.1.0 → v1.2.0
- `docs/architecture/dsaft-overview.md` — v1.0.0 → v1.1.0

### 新建 5 个（4 件套 + yaml）
- `openspec/changes/devrix-docs-request-flow-v2/.openspec.yaml`
- `openspec/changes/devrix-docs-request-flow-v2/proposal.md`
- `openspec/changes/devrix-docs-request-flow-v2/design.md`
- `openspec/changes/devrix-docs-request-flow-v2/tasks.md`
- `openspec/changes/devrix-docs-request-flow-v2/acceptance-report.md`

### 不变更
- `openspec/specs/d7-orchestration/spec.md` v3.8.0（SoT）
- `internal/layers/orchestration/**` 全部代码
- `internal/layers/communication/**` 全部代码
- `internal/layers/contextengine/**` 全部代码（D2 仍是 Follower，由 D7 编排）
