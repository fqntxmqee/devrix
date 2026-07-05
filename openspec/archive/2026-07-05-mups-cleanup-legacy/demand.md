---
demand-id: DM-20260705-007
title: "mups-cleanup-legacy — 删除 M4 verify_legacy_test.go + M5 spawn_policy_legacy_test.go 死代码"
source: MUPS 5 节点重构路线图（cleanup follow-on; M4 §4.Q6 + M5 §4.Q7 明确记录）
priority: P2
status: S3_Design
l1-domain: orchestration
created: 2026-07-05
related:
  - openspec/specs/d7-orchestration/spec.md
  - openspec/specs/d7-orchestration/mups-spawn-data-objects.md
  - internal/layers/orchestration/workmodel/spawn_policy.go
  - internal/layers/orchestration/workmodel/spawn_policy_test.go
  - internal/layers/orchestration/workmodel/spawn_policy_legacy_test.go  # DELETE
  - internal/layers/orchestration/sessionorchestrator/verify_legacy_test.go  # DELETE
  - internal/layers/orchestration/sessionorchestrator/verify_decision_table.go
  - internal/layers/orchestration/sessionorchestrator/item_verify.go
  - internal/layers/orchestration/sessionorchestrator/item_pipeline_rollup.go
parent_demands:
  - DM-20260705-003  # M1 Observe go-struct-driven
  - DM-20260705-004  # M2 Plan go-struct-driven
  - DM-20260705-005  # M4 Verify decision-table-driven
  - DM-20260705-006  # M5 Spawn decision-algebra
---

# mups-cleanup-legacy — MUPS 重构死代码清理

## 1. 原始描述

> MUPS 5 节点重构（M1+M2+M4+M5）已 S7_archived。每节点都用 1 个 `_legacy_test.go`（build tag `legacy_verify` / `legacy_spawn`）保留旧实现，作为 0 行为变化的字节级验证基线（"tripwire"）。当 4 个新实现（Observe/Plan/Verify/Spawn）在生产稳定后，tripwire 完成了它的历史使命，应删除以减少维护成本（死代码 663 行 + 2 个 build tag + 4 个旧符号）。本 change 以"删除 = 0 行为变化"原则清理 2 个 _legacy_test.go 文件和 4 个旧符号（`verifyArtifactLegacy` / `verifyArtifactForWorkItemWithContractLegacy` / `verifyRollupArtifactLegacy` / `SpawnPolicyEvaluatorLegacy`），不修改生产代码任何一行。

## 2. 问题陈述（现状诊断）

### 2.1 已有能力（不重复建设）

| 能力 | 状态 | 路径 |
|------|------|------|
| `verify_decision_table.go` kernel (12 detector + 2 决策表 + applyDecisionTable) | ✅ S7_archived (M4, PR #407) | `sessionorchestrator/verify_decision_table.go` 339 行 |
| `verifyArtifact` 49→9 行 (新) | ✅ S7_archived (M4) | `sessionorchestrator/item_verify.go:1-9` |
| `verifyArtifactForWorkItemWithContract` 54→35 行 (新) | ✅ S7_archived (M4) | `sessionorchestrator/item_verify.go:35-...` |
| `verifyRollupArtifact` 47→9 行 (新) | ✅ S7_archived (M4) | `sessionorchestrator/item_pipeline_rollup.go` |
| `spawn_decision_algebra.go` kernel (3 子决策 + normalizeCtx) | ✅ S7_archived (M5, PR #409) | `workmodel/spawn_decision_algebra.go` 165 行 |
| `SpawnPolicyEvaluator` 50+→8 行 (新) | ✅ S7_archived (M5) | `workmodel/spawn_policy.go:18-30` |
| 22 现有 workmodel 测试 + 17 现有 sessionorchestrator 测试 | ✅ 0 修改 PASS | `spawn_policy_test.go` 21 + `spawn_policy_inline_test.go` 1 + `item_verify_test.go` 4 + `deliverable_verify_test.go` 5 + `item_pipeline_rollup_test.go` 8 |
| 3 byte-equivalent 测试 (verify) + 1 byte-equivalent 测试 (spawn) 字节级 PASS | ✅ 17 + 27 组合 | `verify_legacy_test.go` (build tag `legacy_verify`) + `spawn_policy_legacy_test.go` (build tag `legacy_spawn`) |

### 2.2 缺口（tripwire 已完成历史使命）

| 关注点 | 现状 | 风险 |
|--------|------|------|
| **旧实现仅在 `_legacy_test.go` 保留** | `verifyArtifactLegacy` (44 行) + `verifyArtifactForWorkItemWithContractLegacy` (44 行) + `verifyRollupArtifactLegacy` (39 行) + `SpawnPolicyEvaluatorLegacy` (50+ 行) 共 4 个旧符号，177 行 | 死代码但带 build tag 编译占用，2 个 build tag (`legacy_verify` / `legacy_spawn`) 增加构建矩阵维护成本 |
| **byte-equivalent 测试 17+27 组合** | `verify_legacy_test.go` 3 测试 + `spawn_policy_legacy_test.go` 1 测试 (27 sub-cases) 都在 `_legacy_test.go` 文件中 | tripwire 完成历史使命后，0 行为变化已被现有 22+17 测试 + sub-decision test + 顺序锁定测试 1+1 多重保险 |
| **CI 矩阵冗余** | 任何 PR 需跑 `go test -tags legacy_verify` 和 `go test -tags legacy_spawn` 验证 byte-equivalent | 多 2 个 build tag 编译目标 + 多 2 个测试套件 = CI 时间 +20% |
| **新读者认知负担** | reader 看到 `verifyArtifactLegacy` / `SpawnPolicyEvaluatorLegacy` 容易混淆"哪个是生产实现" | 命名歧义 → 阅读摩擦 |

**风险（tripwire 保留导致的 3 类问题）**：
1. **死代码占用**：177 行旧实现 + 2 个 build tag 文件 = 663 行（test 文件）→ 长期维护成本
2. **CI 冗余**：byte-equivalent 测试的目标（"验证新实现字节级等于旧实现"）已被现有测试 0 修改 + sub-decision test + 顺序锁定测试 1+1 替代
3. **reader 认知负担**：4 个 Legacy 后缀符号 → "哪个在生产？" 二义性

### 2.3 目标行为（删除 tripwire = 0 行为变化）

1. **删除 2 个 `_legacy_test.go` 文件**：`verify_legacy_test.go` (307 行) + `spawn_policy_legacy_test.go` (356 行) = 663 行
2. **删除 4 个旧符号**：`verifyArtifactLegacy` / `verifyArtifactForWorkItemWithContractLegacy` / `verifyRollupArtifactLegacy` / `SpawnPolicyEvaluatorLegacy`（仅存在于 `_legacy_test.go`，无生产代码引用）
3. **删除 2 个 build tag**：`legacy_verify` / `legacy_spawn`（仅这 2 个文件使用，删除后 build tag 整体消失）
4. **0 行为变化**：生产代码（`item_verify.go` / `item_pipeline_rollup.go` / `spawn_policy.go`）0 修改；现有 22+17 测试 + 17 new sub-decision + 1 顺序锁定 + 1 sub-decision-order 共 41 测试全 PASS
5. **简化 reader 认知**："生产实现 = `verifyArtifact` / `SpawnPolicyEvaluator`" 单一权威，0 歧义

### 2.4 0 行为变化验证矩阵

| 验证维度 | 工具 | 期望 |
|----------|------|------|
| 现有 workmodel 22 测试 PASS | `go test ./internal/layers/orchestration/workmodel/ -race -count=1` | 22/22 PASS |
| 现有 sessionorchestrator 17 测试 PASS | `go test ./internal/layers/orchestration/sessionorchestrator/ -race -count=1` | 17/17 PASS |
| M5 新增 17 sub-decision + 1 sub-decision-order PASS | (workmodel 测试) | 18/18 PASS |
| M4 新增 12 detector + 1 顺序锁定 PASS | (sessionorchestrator 测试) | 13/13 PASS |
| 全文 -race -count=1 绿 | `go test ./... -race -count=1` | 0 fail (除 pre-existing 1 lint test) |
| `go vet ./...` 0 warning | `go vet ./...` | 0 warning |
| build tag `legacy_verify` 0 hits | `grep -rn "legacy_verify" --include="*.go" .` | 0 hits |
| build tag `legacy_spawn` 0 hits | `grep -rn "legacy_spawn" --include="*.go" .` | 0 hits |
| 旧符号 0 hits | `grep -rn "verifyArtifactLegacy\|verifyArtifactForWorkItemWithContractLegacy\|verifyRollupArtifactLegacy\|SpawnPolicyEvaluatorLegacy" --include="*.go" .` | 0 hits |

### 2.5 重构总图（5 节点 + cleanup follow-on）

| 阶段 | 范围 | 行为变化 | 本 change 落点 |
|------|------|----------|-----------------|
| M1 | Observe go-struct 化 | 0 行为变化 | ✅ S7_archived (DM-20260705-003) |
| M2 | Plan go-struct 化（kernel 复用） | 0 行为变化 | ✅ S7_archived (DM-20260705-004) |
| M4 | Verify 决策表化 | 0 行为变化 | ✅ S7_archived (DM-20260705-005) |
| M5 | SpawnDecision 3 子决策代数化 | 0 行为变化 | ✅ S7_archived (DM-20260705-006) |
| **cleanup** | **删除 2 个 `_legacy_test.go` 死代码 + 4 个旧符号 + 2 个 build tag** | **0 行为变化（仅 test 编译路径）** | **本 change** |
| M3 | Strategy 抽象注入 WorkItemExecContext | 行为增量（PlanKind 路由恢复） | 最后做 (`d7-mups-strategy-injection`) |

**为什么 cleanup 在 M3 之前做**：M3 是行为增量，删除 tripwire 不会影响 M3 的新代码（tripwire 仅是 reference 实现），且 M3 启动后读者认知负担更低（"生产实现 = `verifyArtifact` / `SpawnPolicyEvaluator`" 单一权威）。

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `verify_legacy_test.go` + `spawn_policy_legacy_test.go` 2 个文件已从 git history 移除 | P0 |
| AC2 | `legacy_verify` + `legacy_spawn` 2 个 build tag 0 hits in repo | P0 |
| AC3 | `verifyArtifactLegacy` / `verifyArtifactForWorkItemWithContractLegacy` / `verifyRollupArtifactLegacy` / `SpawnPolicyEvaluatorLegacy` 4 个旧符号 0 hits in repo | P0 |
| AC4 | 现有 workmodel 22 测试 0 修改 PASS（含 M5 新增 18 测试） | P0 |
| AC5 | 现有 sessionorchestrator 17 测试 0 修改 PASS（含 M4 新增 13 测试） | P0 |
| AC6 | 全文 `go test ./... -race -count=1` 0 fail（除 pre-existing 1 lint test） | P0 |
| AC7 | `go vet ./...` 0 warning | P0 |
| AC8 | PR CI `unit tests` 绿 + auto-merge 合入 master | P0 |
| AC9 | S7 归档后 `demand-archive-index.md` 入口 + `t-registry.md` 版本 bump + `a-registry.md` 版本 bump + `CHANGELOG.md` 行 | P1 |

## 4. 非目标

- 修改生产代码任何一行（`verify_decision_table.go` / `item_verify.go` / `item_pipeline_rollup.go` / `spawn_decision_algebra.go` / `spawn_policy.go`）
- 修改现有 41 测试任何一行
- 引入新的 build tag
- 引入新的 helper 函数
- 真实 PlanKind 路由（属 M3 d7-mups-strategy-injection）
- 修改 5 节点重构总图（M1+M2+M4+M5 已 S7_archived，cleanup 完成后写 mups-5node-refactor-roadmap.md 最终落地文档）

## 5. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | DM-20260705-005 (M4) PR #407 MERGED ✅<br/>DM-20260705-006 (M5) PR #409 MERGED ✅ |
| 约束 | "0 行为变化" 承诺：仅删除 test 文件 + 旧符号（仅 test 编译路径），生产代码 0 修改 |
| 约束 | 全文 `go test -race -count=1` 0 fail（除 pre-existing 1 lint test `TestScan_FindsAllInvariantFiles` 与本 change 无关） |
| 约束 | S6-归档时同步 5 个域规范文档（spec / t-registry / a-registry / CHANGELOG / demand-archive-index） |

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 删除后 byte-equivalent tripwire 失去 | 未来重构可能引入静默行为变化 | 现有 22+17 测试 + sub-decision test + 顺序锁定测试 1+1 + 12 detector + 4 verdict 路由 多重保险 |
| 删除后 _legacy_test.go 仍有引用 | build 失败 | `grep -rn "legacy_verify\|legacy_spawn\|verifyArtifactLegacy\|verifyArtifactForWorkItemWithContractLegacy\|verifyRollupArtifactLegacy\|SpawnPolicyEvaluatorLegacy" --include="*.go" .` 0 hits 验证 |
| M3 启动后需要 tripwire | 未来 M3 行为增量可能想要旧实现作对比 | M3 是行为增量（新增功能），不需要"旧实现字节级对比"，需要"新增功能单测"覆盖 |

