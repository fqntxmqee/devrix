---
demand-id: DM-20260625-002
title: D7 MUPS v4 整体 Review 修复 (3 Critical + 10 High + 1 doc)
priority: P0
status: S7_Archived
dsaft_domain: orchestration
created: 2026-06-25
archived: 2026-06-25
---

# D7 MUPS v4 整体 Review 修复

## 1. 背景

2026-06-24 完成 MUPS v4 5 节点管道全量深度 review（覆盖 Phase 1-7 全部 7 个 S7_Archived change 涉及的代码）。review 涵盖 `internal/layers/orchestration/{workmodel,plan,execute,learn,sessionorchestrator,decisionplanning,wavescheduler,orchtypes}/`，从代码质量、并发安全、错误处理、命名一致性、测试覆盖、S4-Gate 检查清单 5 个维度扫描。

## 2. 评审结论

**48 个问题需修复** —— 3 CRITICAL + 11 HIGH + 20 MEDIUM + 14 LOW。

本 demand 只解决 **P0 + P1 全部 14 个修复**（3 Critical + 11 High），**不**包含 Medium/Low 修复（这些留给后续 cleanup change）。

## 3. 14 个修复点

| Fix | 节点 | 严重度 | 文件 |
|-----|------|--------|------|
| 1. clamp01OrFallback NaN | Verify | Critical | workmodel/aggregate_verdicts.go |
| 2. aggregateMeta 溯源 | Verify | Critical | workmodel/aggregate_verdicts.go |
| 3. rollback context 隔离 | Execute | Critical | execute/channel_protocol.go |
| 4. PersistScope fail-fast | Plan | High | plan/plan_struct.go + plan_test.go |
| 5. NewPlanID UUID+SHA256 | Plan | High | plan/planner.go |
| 6. ErrChannelStepInvalid | Execute | High | execute/errors.go |
| 7. CommitChannel 用新错 | Execute | High | execute/channel_commit.go |
| 8. sync.WaitGroup 模式 | Execute | High | execute/channel_exploration.go |
| 9. mostInformativeError | Execute | High | execute/channel_exploration.go |
| 10. LP-3 Reputation 顺序 | Learn | High | learn/learner.go |
| 11. ScheduledMemory 深拷贝 | Learn | High | learn/memory.go |
| 12. Auto-Close 异步 Learn | Auto-Close | High | sessionorchestrator/autoclose.go |
| 13. Auto-Close test 500ms | test | High | orchestrator_autoclose_test.go |
| 14. pipeline-architecture.md | doc | High | pipeline-architecture.md (NEW) + spec.md |

## 4. Hotfix 路径

按你 `2026-06-17 反馈的 bugfix hotfix 路径`（feedback-devrix-bugfix-skip-openspec）执行：

- 跳过 S1-S3 完整立项流程
- 直接进入 S4 实现（13 个 commit 已落地）
- 走 S4-Gate 审查（PR #192）
- 走 S5 验收（CI checks + go test -race）
- 走 S6 归档（本 archive）

本 demand 文档就是后置的 S1 文档（hotfix 路径的"先 code 后 doc"原则）。

## 5. PR 拆分

按你 `2026-06-20 确认的 bugfix 聚合原则`：

- **1 个聚合 PR**：PR #192（13 commit，14 fix + 1 OpenSpec change docs）
- 避免一个 fix 一个 PR 的过度拆分

## 6. 验证结果

- `go vet ./...` → exit 0
- `go test -race -count=1 ./internal/layers/orchestration/...` → 22/22 packages PASS
- CI: unit tests + layer-lint

详见 `acceptance-report.md`。

## 7. Out of Scope

- M1-M20 Medium 修复（20 个）→ 后续 cleanup change
- L1-L14 Low 修复（14 个）→ 后续 cleanup change
- `coordinator/aliases.go` 130 行 shim → 单独 Change
- `hubspoke/aliases.go` 80 行 shim → 单独 Change
- 1 个 false positive（Wilson margin formula 已验证正确）

## 8. References

- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/proposal.md`
- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/design.md`
- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/tasks.md`
- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/specs/d7-orchestration/spec.md`
- `openspec/specs/d7-orchestration/pipeline-architecture.md`（fix 14 新增）
- 7 个 MUPS Phase archive（9 个 change-id）
