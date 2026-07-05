# Proposal: mups-cleanup-legacy — MUPS 重构死代码清理

**Change ID:** mups-cleanup-legacy
**Demand ID:** DM-20260705-007
**Status:** Archived (s7_archived)

## 1. Background

MUPS 5 节点重构（M1+M2+M4+M5）已 S7_archived。每节点都用 1 个 `_legacy_test.go`（build tag `legacy_verify` / `legacy_spawn`）保留旧实现，作为 0 行为变化的字节级验证 tripwire。M4 (`mups-verify-table-driven`) §4.Q6 + M5 (`d7-spawn-decision-algebra`) §4.Q7 明确记录："S5 验收通过后，下个 change (`mups-cleanup-legacy`) 删除 `_legacy_test.go`"。

tripwire 的历史使命是"在重构 S4/S5 阶段 byte-equivalent 验证 0 行为变化"，但生产代码稳定后 tripwire 变成"长期维护的死代码 + 2 个 build tag + 4 个 Legacy 后缀符号"，增加了：
- 死代码占用（663 行 test + 177 行旧实现）
- CI 矩阵冗余（2 个 build tag 编译目标 + 2 个 test 套件 +20% CI 时间）
- reader 认知负担（4 个 Legacy 后缀符号 → "哪个在生产？"二义性）

## 2. Problem Statement

**核心问题**：tripwire 完成历史使命后，663 行死代码 + 2 个 build tag + 4 个旧符号 长期占用维护成本，但 0 行为变化验证已由 22+17 现有测试 + 18+13 sub-decision/detector + 2 顺序锁定测试 多重保险覆盖，tripwire 失去存在价值。

**次要问题**：
- 2 个 build tag (`legacy_verify` / `legacy_spawn`) 增加构建矩阵
- 4 个 Legacy 后缀符号 (`verifyArtifactLegacy` / `verifyArtifactForWorkItemWithContractLegacy` / `verifyRollupArtifactLegacy` / `SpawnPolicyEvaluatorLegacy`) 命名歧义
- CI 跑 byte-equivalent 测试 (~5-10s) 是冗余时间

**不解决**：
- 不修改生产代码任何一行
- 不修改现有 41 测试任何一行
- 不引入新 build tag / 新 helper
- 不复活 M3 行为增量（属独立 change）

## 3. Proposed Solution

**核心方案**：`git rm` 2 个 `_legacy_test.go` 文件 + 验证 4 个旧符号 0 hits + 验证 2 个 build tag 0 hits + 现有 22+17 测试 0 修改 PASS。

**步骤**：
1. S1 需求：demand.md（DM-20260705-007）
2. S2 提案：proposal.md + .openspec.yaml (status: s3_design)
3. S3 设计：design.md（六段式简版，因小型 change 每段 1-3 行概要 + 关键示例）
4. S3-Gate：自检 design 完整性
5. S4 实现：`git rm verify_legacy_test.go spawn_policy_legacy_test.go`
6. S4-Gate：自检代码
7. S5 验收：跑 workmodel + sessionorchestrator 全套测试 + `go vet ./...` + 全文 grep 0 hits
8. S6-交付：开 PR + auto-merge 合入 master
9. S6-归档：move to archive/ + 同步 5 个域规范文档

## 4. Success Metrics

| Metric | Before | After | 备注 |
|--------|--------|-------|------|
| `_legacy_test.go` 文件数 | 2 | 0 | 完全删除 |
| `legacy_*` build tag 引用 | 2 个文件 | 0 hits | 完全删除 |
| Legacy 后缀符号 | 4 个 | 0 hits | 完全删除 |
| workmodel 测试通过率 | 22/22 + 18/18 | 22/22 + 18/18 | 0 修改 PASS |
| sessionorchestrator verify+rollup 测试通过率 | 17/17 + 13/13 | 17/17 + 13/13 | 0 修改 PASS |
| `go test -race -count=1 ./...` 失败数 | 1 (pre-existing lint) | 1 (pre-existing lint) | 0 新增失败 |
| `go vet ./...` 警告数 | 0 | 0 | 0 新增警告 |

## 5. Implementation Plan

**S4 实施步骤**（4 步）：
1. `git rm internal/layers/orchestration/sessionorchestrator/verify_legacy_test.go`
2. `git rm internal/layers/orchestration/workmodel/spawn_policy_legacy_test.go`
3. `go test ./internal/layers/orchestration/workmodel/ -race -count=1` → 期望 22+18 = 40/40 PASS
4. `go test ./internal/layers/orchestration/sessionorchestrator/ -race -count=1` → 期望 17+13 = 30/30 PASS

**S4 验证步骤**（5 步）：
1. `grep -rn "legacy_verify\|legacy_spawn" --include="*.go" .` → 0 hits
2. `grep -rn "verifyArtifactLegacy\|verifyArtifactForWorkItemWithContractLegacy\|verifyRollupArtifactLegacy\|SpawnPolicyEvaluatorLegacy" --include="*.go" .` → 0 hits
3. `go test ./... -race -count=1` → 1 fail (pre-existing) 不变
4. `go vet ./...` → 0 warning
5. `go build -tags legacy_verify ./...` → build error (build tag 0 hits, 预期失败)

## 6. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| 删除后 byte-equivalent tripwire 失去 | 未来重构可能引入静默行为变化 | 现有 22+17 测试 + 18+13 sub-decision/detector + 2 顺序锁定测试 = 70 测试多重保险 |
| 删除后 `_legacy_test.go` 仍有引用 | build 失败 | grep 0 hits 验证 + `go build -tags legacy_verify ./...` build error 验证 |
| M3 启动后需要 tripwire | M3 行为增量可能想要旧实现作对比 | M3 是行为增量（新增功能），不需要"旧实现字节级对比"，需要"新增功能单测" |
| 0 行为变化承诺被打破 | 现有 41 测试 fail | workmodel + sessionorchestrator 全套 -race -count=1 PASS + pre-existing 1 lint fail 不变 |

## 7. Out of Scope

- 修改生产代码任何一行（`verify_decision_table.go` / `item_verify.go` / `item_pipeline_rollup.go` / `spawn_decision_algebra.go` / `spawn_policy.go`）
- 修改现有 41 测试任何一行
- 引入新的 build tag
- 引入新的 helper 函数
- 真实 PlanKind 路由（属 M3 d7-mups-strategy-injection）
- 修改 5 节点重构总图（M1+M2+M4+M5 已 S7_archived，cleanup 完成后写 mups-5node-refactor-roadmap.md 最终落地文档）

## 8. 关联

- **Parent Demands**: DM-20260705-003 (M1) / DM-20260705-004 (M2) / DM-20260705-005 (M4) / DM-20260705-006 (M5)
- **Predecessor**: M4 §4.Q6 + M5 §4.Q7 明确记录本 change
- **Successor**: M3 d7-mups-strategy-injection (行为增量，最后做)
