# Proposal: D6 Evolution spec 补登

**Change ID:** devrix-spec-sync-d6-evolution-registration
**Demand ID:** DM-20260619-003
**Status:** S7_Archived (2026-06-19)
**Date:** 2026-06-19
**Author:** Devrix Team

> **docs-only change**：本 change 仅改 D6 spec 四份文档（spec.md / design.md / layer-delta.md + 新建 d6-domain.md），不动 D6 域代码、不改 D-S 编号。

---

## 1. 背景

D6 Evolution 在 2026-06-15 完成 v2.0 物理路径迁移（DM-20260615-003 `devrix-d5-d6-sa-refine-v2.0`），3 包重命名：
- `eval/` → `evaluate/`
- `exporter/` → `export/`
- `orchestration/` → `guard/`

并新增 `verify/` 子包（v1.0 已有逻辑但 v2.0 物理独立）。

但 D6 spec **未完全同步 v2.0 状态**，存在 4 处重大遗漏。

## 2. 问题陈述

### 2.1 `evaluate/` `guard/` `verify/` 三个子包在 spec 三份文档中**零记录**

实际代码：
- `evaluate/` — Self-Eval 引擎（v2.0 重命名自 eval/）
- `guard/` — Guard 韧性（v2.0 重命名自 orchestration/，曾因误删从 42bf1d7 恢复）
- `verify/` — Invariant + Plan 验证（v2.0 物理独立）

spec.md / design.md / layer-delta.md 三份文档**均未提到**这 3 个子包，仍使用 `eval/` 与 `orchestration/` 旧路径。

### 2.2 spec.md Package Map 仍列 `orchestration/`

spec.md 仍把 `RuntimeOrchestrationValidator | orchestration/validator.go | 决策入口：预过滤→Judge 校验→干预触发` 列为活模块，但代码中 `orchestration/` 子包仅剩 `bridge.go`（已废弃桥接），实际职责已迁至 `guard/`。

### 2.3 D6 是项目**唯一**缺 `d6-domain.md` 的支撑域

D1/D2/D3/D4/D5/D7 均有 `d{N}-domain.md`（域 SoT），D6 是唯一缺口。这导致 D6 域职责、价值流、跨域契约没有集中权威说明。

### 2.4 layer-delta.md 缺 v2.0 物理路径迁移记录

`devrix-d5-d6-sa-refine-v2.0`（DM-20260615-003）已于 2026-06-15 落地，但 D6 layer-delta.md 未追加以下条目：
- `eval/` → `evaluate/` 重命名
- `orchestration/` → `guard/` 重命名
- 新增 `verify/` 子包
- 删除 11 个 bridge.go 桥接

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | spec.md §Package Map 用 `evaluate/` 替换 `eval/`；新增 `guard/` + `verify/` 子包条目 | P0 |
| AC2 | spec.md 删除 `RuntimeOrchestrationValidator | orchestration/validator.go` 旧条目 | P0 |
| AC3 | 新建 `d6-domain.md` 对齐 D2/D4/D5/D7 结构（域描述 + 价值流 + 跨域契约） | P0 |
| AC4 | design.md v2.1.0 → v2.2.0：§Package Map 用新路径；§v2.0 状态从"实施中"改为"已完成（DM-015-003）" | P0 |
| AC5 | layer-delta.md 追加 v2.0 物理路径迁移章节（eval→evaluate / orchestration→guard / 新增 verify/ / 11 bridge 删除） | P0 |
| AC6 | spec.md / design.md `Last Updated` 同步至 2026-06-19 | P0 |
| AC7 | `go vet ./...` 0 错 | P1 |
| AC8 | `verify-archive.sh openspec/changes/devrix-spec-sync-d6-evolution-registration` 全部 PASS | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖（已闭环）| `devrix-d6-sa-refine` (DM-20260615-002) — S4→S12 GuardRuntime v1.0 registry |
| 依赖（已闭环）| `devrix-d5-d6-sa-refine-v2.0` (DM-20260615-003) — v2.0 物理路径迁移 |
| 风险点 | guard 子包历史（误删 → 42bf1d7 恢复），本 change 需在 d6-domain.md 留痕 |
| 约束 | docs-only，不动 .go 源码 |
| 约束 | D-S 编号体系（D6-S/A/F/T）不变 |
| 约束 | 沿用 Devrix GitHub Flow：`feat/spec-sync-d6-evolution-registration` 分支 + squash merge + auto-merge |

## 5. 变更范围

### 修改 3 个
- `openspec/specs/d6-evolution/spec.md` — Package Map 用新路径；Last Updated 刷新
- `openspec/specs/d6-evolution/design.md` — v2.1.0 → v2.2.0
- `openspec/specs/d6-evolution/layer-delta.md` — 追加 v2.0 物理路径迁移章节

### 新建 6 个
- `openspec/specs/d6-evolution/d6-domain.md` — 域 SoT（与 D2/D4 对齐）
- `openspec/changes/devrix-spec-sync-d6-evolution-registration/.openspec.yaml`
- `openspec/changes/devrix-spec-sync-d6-evolution-registration/proposal.md`（本文件）
- `openspec/changes/devrix-spec-sync-d6-evolution-registration/design.md`（S3）
- `openspec/changes/devrix-spec-sync-d6-evolution-registration/tasks.md`（S4）
- `openspec/changes/devrix-spec-sync-d6-evolution-registration/acceptance-report.md`（S5）

### 不变更
- `internal/layers/evolution/**` 全部代码
- D6 t-registry.md（行为不变）
