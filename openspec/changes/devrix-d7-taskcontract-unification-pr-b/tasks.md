# Tasks: D7 TaskContract 统一 PR-B — Pessimistic Commit + Rule-based Fallback (L3 防御运行时层)

**Change ID:** devrix-d7-taskcontract-unification-pr-b
**Demand ID:** DM-20260629-008
**Phase:** S4 Implementation (S3-Gate 通过, S5 验证待跑)
**Status:** S4_Implementation (PR-B S4 code + tests + spec 同步全部完成; S5/S6 待跑)
**Created:** 2026-06-29

---

## Phase 1: S4 Implementation (P0 T 矩阵) ✅ 6/7 IMPLEMENTED + 1 PLANNED T05

### 1.1 interfaces/ 纯类型层 (3 NEW + 0 MOD PR-B; TaskReport.WithMVPArtifact PR-A 落地)

- [x] **D7-S18-A11-T01**: PessimisticCommitGuard interface + 4 SentinelError
  - [x] `internal/layers/orchestration/interfaces/contracts.go` (NEW, ~110 LOC) - PessimisticCommitGuard interface + 5 Trigger* 常量 + 4 ORCH_* error helpers (7110-7113)
  - [x] `sharederrors.WithCode(7110..7113)` 4 错误码
  - [x] 验证：`grep -r 'orchestration/' interfaces/ | grep -v _test` → 0 行 (IV-1)

- [x] **D7-S18-A11-T02**: FallbackPolicy enum + ConvergenceBudget 值对象
  - [x] `internal/layers/orchestration/interfaces/fallback_policy.go` (NEW, ~80 LOC) - FallbackPolicyRuleNames 4 候选 + DefaultFallbackRule + Valid + ValidNonLegacy + ParseFallbackRuleName
  - [x] `internal/layers/orchestration/interfaces/convergence_budget.go` (NEW, ~100 LOC) - NewConvergenceBudget + With* + Validate + RemainingBelowReserve + ToFields
  - [x] 验证：4 候选 + 5 字段不可变

- [x] **D7-S18-A11-T03**: TaskReport.WithMVPArtifact immutable builder (PR-A 已落地, MVPArtifact struct 含 Output/RiskWarnings/Trigger/ChainHash 4 字段)

### 1.2 escape/ 防御运行时层 (1 NEW + 2 MOD + 2 TEST)

- [x] **D7-S18-A11-T04**: DefaultPessimisticCommitGuard (PessimisticCommitGuard 默认实现) + 3 FallbackPolicy 路径
  - [x] `internal/layers/orchestration/escape/fallback.go` (NEW, ~310 LOC) - 5 类触发 check* + 3 methods (Evaluate/ResolveFallback/BuildMVPArtifact) + buildChainHash FNV-1a 16-char hex
  - [x] FallbackPessimistic (default) + FallbackRuleBased + FallbackAbort
  - [x] 验证：5 类触发条件 → 决策树完整 (见 design §3.1)

- [x] **D7-S18-A12-T01**: Rule-based 4 候选规则
  - [x] `escape/fallback.go::DefaultPessimisticCommitGuard.ResolveFallback` (4 规则实现)
  - [x] most_tests_passed / compiled_clean / min_cost / min_uncertainty (default)
  - [x] 验证：env `D7_RULE_FALLBACK_STRATEGY` 切换生效

- [x] **D7-S18-A12-T02**: 5 层 CB L1 → Pessimistic 升级（不修改 circuit_breaker.go, 仅在测试层验证 L1 → Pessimistic 联动）
  - [x] `internal/layers/orchestration/escape/circuit_breaker_test.go` (MOD, +3 NEW L1 Pessimistic tests)
  - [x] L1 trips StateOpen + 60s 持久窗口 + L1-only reason 含 "l1" 路由 hint
  - [x] 验证：L1 单失败触发 Pessimistic hint，L2-L5 行为不变

- [x] **D7-S18-A11-T05** (Span + Metric + ErrorCode 链路): 6/7 子项 IMPLEMENTED，T05 PLANNED 留 PR-C
  - [x] `internal/layers/orchestration/escape/engine.go` (MOD, +NotifyPessimistic 5 层 fail-safe + SetPessimisticGuard accessor)
  - [x] `NotifyPessimistic` 通过 `slog.Info("pessimistic_commit_emit", ...)` 占位（7 attrs: session_id/trace_id/reason/policy/fallback_used/mvp.artifact_hash/mvp.trigger 结构化字段已对齐）
  - [x] ErrorCode 7110-7113 ORCH_* SentinelError 完整注册到 `internal/shared/errors`
  - [ ] **T05 PLANNED**: 完整 Jaeger (OTel span 注册) + Prometheus wire (5 metrics: pessimistic_commit_trigger_count / fallback_rule_select_total / mvp_artifact_generated_total / pessimistic_commit_latency_us / fallback_rule_apply_total) 留 PR-C

### 1.3 mups/execute + bootstrap 集成 (2 MOD)

- [x] **D7-S18-A11-T06**: ChannelRouter SetPessimisticGuard + ApplyPessimisticCommit
  - [x] `internal/layers/orchestration/mups/execute/channel.go` (MOD, +2 NEW methods + 1 field)
  - [x] ChannelRouter.pessimisticGuard interfaces.PessimisticCommitGuard + SetPessimisticGuard + ApplyPessimisticCommit
  - [x] 验证：5 类触发条件 全部走 ApplyPessimisticCommit

- [x] **D7-S18 横切-T01 (AC16)**: Feature Flag env-gated
  - [x] `internal/bootstrap/pessimistic_guard_wire.go` (NEW, ~75 LOC) - 实际路径在 `internal/bootstrap/`（不是 `d7-bootstrap/`，因 wire_coordinator.go 也在那里）
  - [x] `D7_PESSIMISTIC_COMMIT_ENABLED` env (default `false`) + `D7_RULE_FALLBACK_STRATEGY` (default `min_uncertainty`)
  - [x] `PessimisticCommitEnabled()` + `PessimisticRuleStrategy()` + `NewPessimisticCommitGuardFromEnv()` + 7 env tests
  - [x] 验证：default false 时 `Evaluate()` 永远返回 ok (0 行为变更)

### 1.4 单元测试 (6 NEW + 2 MOD) ✅

- [x] **D7-S18-A11-T07**: interfaces/ 单元测试 3 个文件 (NEW, 共 ~385 LOC, 覆盖率 **96.9%** ≥ 80%)
  - [x] `internal/layers/orchestration/interfaces/contracts_test.go` (NEW, ~150 LOC, 6 tests)
  - [x] `internal/layers/orchestration/interfaces/fallback_policy_test.go` (NEW, ~120 LOC, 5 tests)
  - [x] `internal/layers/orchestration/interfaces/convergence_budget_test.go` (NEW, ~115 LOC, 5 tests)

- [x] **D7-S18-A12-T03**: escape/fallback_test.go (NEW, ~360 LOC, 14 tests, 覆盖率 **85.0%** ≥ 80%)
  - [x] 3 FallbackPolicy 实现各 1 套测试（disabled / 5 triggers / happy path / 2 resolve fallback / 4 build MVP / nil receiver / chain hash stable）
  - [x] 4 候选规则 env 切换测试

- [x] **D7-S18-A12-T04**: escape/circuit_breaker_test.go (MOD, +3 L1 Pessimistic tests, 100 LOC)
  - [x] L1 → Pessimistic hint（TestL1DispatchLoop_PessimisticHint）
  - [x] L2-L5 行为不变（TestL1StateOpen_PersistentForPessimisticWindow + TestCircuitBreakerSet_L1Only_PessimisticCompatible）

### 1.5 spec 文档同步 (6 MOD) ✅

- [x] **D7-S18-A11-T08 (AC18)**: spec.md v4.16.0 → v4.17.0
  - [x] 新增 2 ADDED Requirements（D7-S18-A11 Pessimistic Commit + D7-S18-A12 Rule-based Fallback）
  - [x] 新增 11 Gherkin Scenarios（disabled + 5 trigger 单测 + happy path + BuildMVPArtifact 5 + 5-layer fail-safe + Feature Flag + 4 candidate rules + invalid fall-back + ResolveFallback 2 paths + 2 Select + closed set）
  - [x] 新增 §Scenario D7-S18 Section（7 P0 T 矩阵 6 IMPLEMENTED + 1 PLANNED T05）
  - [x] 修订记录 4.17.0 entry

- [x] **D7-S18-A11-T09**: d7-domain.md v2.7.0 → v2.8.0
  - [x] §8 Layer 架构 L3 行更新（PR-B 6/7 IMPLEMENTED）
  - [x] §8 Phase 2 (PR-B) 行更新（4 AC + 7 T + 本 PR）
  - [x] §9 interfaces 包章节扩展（10 文件清单 + 4 ORCH_* SentinelError 7110-7113 + IV-5 invariant + Additive 嵌入 ChannelRouter.pessimisticGuard）
  - [x] §DSAFT 资产登记规模更新（A 55→57 / F 86→93 / T 241→246 / Span 31→32）
  - [x] 修订记录 2.8.0 entry

- [x] **D7-S18-A11-T10**: a-registry.md v5.1.0 → v5.2.0
  - [x] D7-S18-A11/A12 2 A entries
  - [x] 修订记录 5.2.0 entry

- [x] **D7-S18-A11-T11**: f-registry.md v5.1.0 → v5.2.0
  - [x] D7-S18-A11/F01-F05 (5 F Pessimistic Commit) + D7-S18-A12/F01-F02 (2 F Rule-based Fallback) = 7 F entries
  - [x] F 总数 86 → 93 IMPLEMENTED
  - [x] D7-S22-F01 PessimisticCommitGuard 状态 PLANNED → IMPLEMENTED（已实现并归入 D7-S18-A11）
  - [x] D7-S22-F02 CoWVersionChain 仍 PLANNED (PR-C)
  - [x] 修订记录 5.2.0 entry

- [x] **D7-S18-A11-T12**: span-registry.md v4.3.0 → v4.4.0
  - [x] 1 个新 P0 span op: `D7_Orchestration_Pessimistic_Commit_Emit`（7 attrs: session_id + trace_id + reason + policy + fallback_used + mvp.artifact_hash + mvp.trigger）
  - [x] ops 总数 31 → 32
  - [x] 修订记录 4.4.0 entry

- [x] **D7-S18-A11-T13**: t-registry.md v4.14.0 → v4.15.0
  - [x] D7-S18-A11-T01..T05 (5 P0 T) + D7-S18-A12-T01/T02 (2 P0 T) = 7 P0 T entries
  - [x] 6/7 IMPLEMENTED + 1 PLANNED T05
  - [x] T 总数 239 → 246
  - [x] 修订记录 4.14.0 + 4.15.0 entries

## Phase 2: S5 Verification ⏳ PENDING

- [ ] `go test -race -count=1 ./internal/layers/orchestration/...` → 22 packages PASS
- [ ] `go test -cover ./internal/layers/orchestration/interfaces/` → ≥ 80%（实测 **96.9%**）
- [ ] `go test -cover ./internal/layers/orchestration/escape/` → ≥ 80%（实测 **85.0%**）
- [ ] LP-1/LP-2/LP-5 集成测试 100% 兼容（Feature Flag 默认 disabled）
- [ ] `grep -r 'orchestration/' internal/layers/orchestration/interfaces/ | grep -v _test` → 0 行（IV-1 invariant）
- [ ] `D7_PESSIMISTIC_COMMIT_ENABLED=false` smoke test (0 行为变更)
- [ ] `D7_PESSIMISTIC_COMMIT_ENABLED=true` staging 烟测 24h (后置，不在 S5)

## Phase 3: S6 Archive ⏳ PENDING

- [ ] S6-1: 创建 `acceptance-report.md`
- [ ] S6-2: 更新 `.openspec.yaml` status `S1_Demand` → `s7_archived`
- [ ] S6-3: 7 P0 T 标记 `IMPLEMENTED`（T05 保留 `PLANNED` 留 PR-C）
- [ ] S6-4: `git mv changes/devrix-d7-taskcontract-unification-pr-b/ archive/2026-06-29-devrix-d7-taskcontract-unification-pr-b/`
- [ ] S6-5: `demand-archive-index.md` 添加 PR-B 索引行
- [ ] S6-6: `./scripts/verify-archive.sh devrix-d7-taskcontract-unification-pr-b` → 12/12 PASS
- [ ] S6-7: `git push -u origin feat/devrix-d7-taskcontract-unification-pr-b`
- [ ] S6-8: `gh pr create --title "feat(d7): v7.0 taskcontract pr-b pessimistic commit + rule-based fallback (DM-20260629-008)" --body "..."`
- [ ] S6-9: `gh pr merge --auto --squash --delete-branch`
- [ ] S6-10: build + restart devrix (per `feedback-devrix-restart-via-script.md`)

---

## 任务总览

| 阶段 | 任务数 | 预计工作量 | 完成日期 | 状态 |
|------|--------|------------|----------|------|
| S1-S3 Design | 7 files + 1 commit | 1 天 | 2026-06-29 | ✅ DONE (commit ff9e451c) |
| S4 Implementation | 7 P0 T + 6 spec 文件 + 1 .openspec.yaml | 3-4 天 | 2026-06-29 | ✅ DONE (6/7 IMPLEMENTED + 1 PLANNED T05) |
| S5 Verification | 7 | 1-2 天 | 2026-06-29 | ⏳ PENDING |
| S6 Archive | 10 | 0.5 天 | 2026-06-29 | ⏳ PENDING |
| **总计** | **30** | **~4 天** | **2026-06-29 S7_Archived（target）** | |

## 关键风险与缓解

| 风险 | 缓解 | 现状 |
|------|------|------|
| Feature Flag 灰度 false negative | 分桶 1% → 10% → 50% → 100% 渐进 | 默认 disabled，0 行为变更已通过 IV-5 验证 |
| 4 候选规则复杂度 | 先 min_uncertainty，其他 3 个 stub + TODO v7.0.1 | 4 候选全部 IMPLEMENTED，ClosedSet 已验证 |
| CB L1 升级状态机 | 复用 PR-A Blockage 字段，L1 仅 action 之一 | 3 L1 测试已加，0 circuit_breaker.go 改动 |
| 错误码区间划分 | 7100-7109 PR-A / 7110-7119 PR-B / 7120-7129 PR-C | 7110-7113 PR-B 已注册，无重叠风险 |
| Span/Metric 完整 wire (T05) | 留 PR-C，本 PR 仅 slog.Info 占位 | T05 PLANNED 已登记 spec.md Scenario 段 |
