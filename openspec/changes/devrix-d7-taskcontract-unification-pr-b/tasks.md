# Tasks: D7 TaskContract 统一 PR-B — Pessimistic Commit + Rule-based Fallback (L3 防御运行时层)

**Change ID:** devrix-d7-taskcontract-unification-pr-b
**Demand ID:** DM-20260629-008
**Phase:** S3 Tasks (S3-Gate 前置)
**Status:** S3_Design (待 S3-Gate review)
**Created:** 2026-06-29

---

## Phase 1: S4 Implementation (P0 T 矩阵)

### 1.1 interfaces/ 纯类型层 (3 NEW + 1 MOD)

- [ ] **D7-S18-A11-T01**: PessimisticCommitGuard interface + 4 SentinelError
  - [ ] `internal/layers/orchestration/interfaces/contracts.go` (NEW, 80 LOC)
  - [ ] `sharederrors.WithCode(7110..7113)` 4 错误码
  - [ ] 验证：`grep -r 'orchestration/' interfaces/ | grep -v _test` → 0 行 (IV-1)

- [ ] **D7-S18-A11-T02**: FallbackPolicy enum + ConvergenceBudget 值对象
  - [ ] `internal/layers/orchestration/interfaces/fallback_policy.go` (NEW, 40 LOC)
  - [ ] `internal/layers/orchestration/interfaces/convergence_budget.go` (NEW, 60 LOC)
  - [ ] 验证：3 态 + 4 字段不可变 (IV-2, IV-3)

- [ ] **D7-S18-A11-T03**: TaskReport.WithMVPArtifact immutable builder
  - [ ] `internal/layers/orchestration/interfaces/task_report.go` (MOD, +20 LOC)
  - [ ] 验证：`c := *r` 浅拷贝不修改 receiver

### 1.2 escape/ 防御运行时层 (1 NEW + 2 MOD + 2 TEST)

- [ ] **D7-S18-A11-T04**: 3 FallbackPolicy 实现
  - [ ] `internal/layers/orchestration/escape/fallback.go` (NEW, 200 LOC)
  - [ ] FallbackPessimistic (default) + FallbackRuleBased + FallbackAbort
  - [ ] 验证：5 类触发条件 → 决策树完整 (见 design §3.1)

- [ ] **D7-S18-A12-T01**: Rule-based 4 候选规则
  - [ ] `escape/fallback.go::FallbackRuleBased.Select` (4 规则实现)
  - [ ] most_tests_passed / compiled_clean / min_cost / min_uncertainty (default)
  - [ ] 验证：env `D7_RULE_FALLBACK_STRATEGY` 切换生效

- [ ] **D7-S18-A12-T02**: 5 层 CB L1 → Pessimistic 升级
  - [ ] `internal/layers/orchestration/escape/circuit_breaker.go` (MOD, +20 LOC)
  - [ ] L1 触发 → `engine.NotifyPessimistic(report)` 调用
  - [ ] 验证：L1 单失败触发 Pessimistic，L2-L5 行为不变

- [ ] **D7-S18-A11-T05** (Span + Metric + ErrorCode 链路):
  - [ ] `internal/layers/orchestration/escape/engine.go` (MOD, +30 LOC)
  - [ ] `NotifyPessimistic` 触发 `d7.s18.pessimistic.commit.emit` Span
  - [ ] `pessimistic_commit_trigger_count++` Metric
  - [ ] 验证：Span 落 Jaeger + Metric 进 Prometheus

### 1.3 mups/execute + d7-bootstrap 集成 (2 MOD)

- [ ] **D7-S18-A11-T06**: Channel.Execute 出口集成 PessimisticCommitGuard
  - [ ] `internal/layers/orchestration/mups/execute/channel.go` (MOD, +15 LOC)
  - [ ] Execute 结束 → `guard.Evaluate(spec, report, budget)`
  - [ ] 验证：5 类触发条件 (含本节) 全部走 Evaluate

- [ ] **D7-S18 横切-T01 (AC16)**: Feature Flag env-gated
  - [ ] `internal/layers/orchestration/d7-bootstrap/wire.go` (MOD, +25 LOC)
  - [ ] `D7_PESSIMISTIC_COMMIT_ENABLED` env (default `false`)
  - [ ] 验证：default false 时 `Evaluate()` 永远返回 ok (0 行为变更)

### 1.4 单元测试 (3 NEW)

- [ ] **D7-S18-A11-T07**: `internal/layers/orchestration/interfaces/contracts_test.go` (NEW, 150 LOC)
  - [ ] 覆盖率 ≥ 80%
  - [ ] happy path + 5 类触发条件 (含 CB L1)

- [ ] **D7-S18-A12-T03**: `internal/layers/orchestration/escape/fallback_test.go` (NEW, 200 LOC)
  - [ ] 3 FallbackPolicy 实现各 1 套测试
  - [ ] 4 候选规则 env 切换测试

- [ ] **D7-S18-A12-T04**: `internal/layers/orchestration/escape/circuit_breaker_test.go` (NEW, 100 LOC)
  - [ ] L1 → Pessimistic 通知
  - [ ] L2-L5 行为不变

### 1.5 spec 文档同步 (6 MOD)

- [ ] **D7-S18-A11-T08 (AC18)**: spec.md v4.16.0 → v4.17.0
  - [ ] 新增 3 ADDED Requirements (D7-S18-A11/A12)
  - [ ] 9 Gherkin Scenarios (含 5 类触发条件)

- [ ] **D7-S18-A11-T09**: d7-domain.md v2.7.0 → v2.8.0
  - [ ] §8 Layer 4-Layer × 3-Phase PR-B 落地章节
  - [ ] §10 escape/ 章节新增 FallbackPolicy

- [ ] **D7-S18-A11-T10**: a-registry.md v5.1.0 → v5.2.0
  - [ ] D7-S18-A11/A12 2 A entries

- [ ] **D7-S18-A11-T11**: f-registry.md v5.1.0 → v5.2.0
  - [ ] D7-S22-F01 PessimisticCommitGuard 状态 PLANNED → IMPLEMENTED
  - [ ] D7-S22-F02 CoWVersionChain 仍 PLANNED (PR-C)

- [ ] **D7-S18-A11-T12**: span-registry.md v4.3.0 → v4.4.0
  - [ ] 1 个新 P0 span op: `d7.s18.pessimistic.commit.emit` → `D7_Escape_Pessimistic_Commit_Emit`

- [ ] **D7-S18-A11-T13**: t-registry.md v?.? → v?.?
  - [ ] 7 个 P0 T 登记 (D7-S18-A11-T01..T13 + D7-S18-A12-T01..T04 中 7 个核心)

## Phase 2: S5 Verification

- [ ] `go test -race -count=1 ./internal/layers/orchestration/...` → 24+ packages PASS
- [ ] `go test -cover ./internal/layers/orchestration/interfaces/` → ≥ 80%
- [ ] `go test -cover ./internal/layers/orchestration/escape/` → ≥ 80%
- [ ] LP-1/LP-2/LP-5 集成测试 100% 兼容
- [ ] `grep -r 'orchestration/' internal/layers/orchestration/interfaces/ | grep -v _test` → 0 行
- [ ] `D7_PESSIMISTIC_COMMIT_ENABLED=false` smoke test (0 行为变更)
- [ ] `D7_PESSIMISTIC_COMMIT_ENABLED=true` staging 烟测 24h (后置，不在 S5)

## Phase 3: S6 Archive

- [ ] S6-1: 创建 `acceptance-report.md`
- [ ] S6-2: 更新 `.openspec.yaml` status `S1_Demand` → `s7_archived`
- [ ] S6-3: 11+ T 标记 `IMPLEMENTED`
- [ ] S6-4: `git mv changes/devrix-d7-taskcontract-unification-pr-b/ archive/2026-06-29-devrix-d7-taskcontract-unification-pr-b/`
- [ ] S6-5: `demand-archive-index.md` 添加 PR-B 索引行
- [ ] S6-6: `./scripts/verify-archive.sh devrix-d7-taskcontract-unification-pr-b` → 12/12 PASS
- [ ] S6-7: `git push -u origin feat/devrix-d7-taskcontract-unification-pr-b`
- [ ] S6-8: `gh pr create --title "feat(d7): v7.0 taskcontract pr-b pessimistic commit + rule-based fallback (DM-20260629-008)" --body "..."`
- [ ] S6-9: `gh pr merge --auto --squash --delete-branch`
- [ ] S6-10: build + restart devrix (per `feedback-devrix-restart-via-script.md`)

---

## 任务总览

| 阶段 | 任务数 | 预计工作量 | 完成日期 |
|------|--------|------------|----------|
| S4 Implementation | 13 + 7 (1.5) = 13 | 3-4 天 | 2026-07-02 |
| S5 Verification | 7 | 1-2 天 | 2026-07-03 |
| S6 Archive | 10 | 0.5 天 | 2026-07-03 |
| **总计** | **30** | **~4 天** | **2026-07-03 S7_Archived** |

## 关键风险与缓解

| 风险 | 缓解 |
|------|------|
| Feature Flag 灰度 false negative | 分桶 1% → 10% → 50% → 100% 渐进 |
| 4 候选规则复杂度 | 先 min_uncertainty，其他 3 个 stub + TODO v7.0.1 |
| CB L1 升级状态机 | 复用 PR-A Blockage 字段，L1 仅 action 之一 |
| 错误码区间划分 | 7100-7109 PR-A / 7110-7119 PR-B / 7120-7129 PR-C |
