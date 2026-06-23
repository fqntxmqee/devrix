# Acceptance Report — DM-20260625-002 (MUPS v4 Review 修复)

**Change ID:** `devrix-d7-mups-v4-review-fixes`
**Demand ID:** DM-20260625-002
**PR Scope:** 3 Critical + 10 High + 1 doc = 14 fix（聚合 1 PR，13 commit）
**Acceptance Date:** 2026-06-25
**Status:** ✅ S5_Accepted

---

## 1. 验收范围

| 维度 | 范围 |
|------|------|
| **代码变更** | 13 文件 +429/-66（含 1 个新文件 pipeline-architecture.md） |
| **测试变更** | 4 plan_test 适配 + 1 orchestrator_autoclose_test 适配 + 22 packages 全部 PASS, 0 race |
| **文档变更** | 新增 `openspec/specs/d7-orchestration/pipeline-architecture.md` (589 行)；spec.md 加 1 行引用 |
| **不做的事** | M1-M20 Medium 修复 / L1-L14 Low 修复 / coordinator+hubspoke shim 移除 |

## 2. 14 个修复点验收

| # | Fix | 节点 | 严重度 | 验收证据 | 状态 |
|---|-----|------|--------|----------|------|
| 1 | clamp01OrFallback NaN | D7-S10 | Critical | `TestClamp01OrFallback_NaN` + `TestClamp01OrFallback_OutOfRange` | ✅ |
| 2 | aggregateMeta 溯源 | D7-S10 | Critical | `TestAggregateMeta_DedupSourceID` + `TestAggregateMeta_LongestIndeterminateReason` + `TestAggregateMeta_ORSystemAnomaly` | ✅ |
| 3 | rollback context 隔离 | D7-S9 | Critical | `TestProtocolChannel_Rollback_OuterCancelNoEffect` | ✅ |
| 4 | PersistScope fail-fast | D7-S5 | High | 4 个 plan_test 加 BlastRadius，PersisteScope="" 返回 PLAN_PERSIST_8012 | ✅ |
| 5 | NewPlanID UUID+SHA256 | D7-S5 | High | `TestNewPlanID_Unique` + `TestNewPlanID_Format` | ✅ |
| 6 | ErrChannelStepInvalid | D7-S9 | High | `TestNewChannelStepInvalidError` | ✅ |
| 7 | CommitChannel 用新错 | D7-S9 | High | `TestCommitChannel_EmptyToolName` + `TestCommitChannel_MissingIdempotencyKey` | ✅ |
| 8 | sync.WaitGroup 模式 | D7-S9 | High | `TestExplorationChannel_NoDeadlock_MaxParallelLessThanSteps` | ✅ |
| 9 | mostInformativeError | D7-S9 | High | `TestExplorationChannel_TopErrorIsLongest` | ✅ |
| 10 | LP-3 Reputation 顺序 | D7-S11 | High | `TestLearner_LP3_ReputationBeforeMemory` | ✅ |
| 11 | ScheduledMemory 深拷贝 | D7-S11 | High | `TestScheduledMemory_ListDue_DeepCopy` | ✅ |
| 12 | Auto-Close 异步 Learn | D7-S13 | High | `TestProcessAutoClose_AsyncLearn_NotBlock` | ✅ |
| 13 | Auto-Close test 500ms | D7-S13 test | High | `TestProcessMessage_Verify2Learn_AutoClose_PassAlpha` Round 2 wait 500ms | ✅ |
| 14 | pipeline-architecture.md | doc | High | 589 行，6 章节，13 S 场景，4 类对应表，3 项不变式 | ✅ |

## 3. P0 验收（Critical 3 个）

| ID | 验收标准 | 状态 | 证据 |
|----|---------|------|------|
| **AC1** | NaN input → fallback（不再污染 reputation） | ✅ PASS | `clamp01OrFallback` 加 `math.IsNaN(v)` 检查 |
| **AC2** | 聚合 Verdict 保留 SourceID dedup + IndeterminateReason 最长 + SystemAnomaly OR | ✅ PASS | `aggregateMeta` 重写 |
| **AC3** | rollback 用独立 ctx，不被 outer cancel 打断 | ✅ PASS | `rollback` 改用 `context.WithTimeout(context.Background(), cfg.Timeout)` + first non-nil error |

## 4. P1 验收（High 10 个）

| ID | 验收标准 | 状态 | 证据 |
|----|---------|------|------|
| **AC4** | Plan.PersistScope == "" fail-fast (PLAN_PERSIST_8012) | ✅ PASS | `Plan.Validate` 改用 `Valid()` |
| **AC5** | NewPlanID 格式 `plan_<uuid[:8]>_<sha256[:8]>`，无冲突 | ✅ PASS | `TestNewPlanID_Unique` |
| **AC6** | 区分 StepInvalid vs ToolCallTimedOut 错误码 | ✅ PASS | 2 个新 sentinel + 2 个 helper |
| **AC7** | CommitChannel 改用新错误 | ✅ PASS | 改写 field validation |
| **AC8** | ExplorationChannel MaxParallel < len(Steps) 不死锁 | ✅ PASS | spawn-all + sync.WaitGroup |
| **AC9** | 全部失败时 Summary 显示最长 error | ✅ PASS | `mostInformativeError` helper |
| **AC10** | LP-3 顺序：Reputation 先于 Memory | ✅ PASS | `DefaultLearner.Learn` 调换 |
| **AC11** | ScheduledMemory.ListDue 返回深拷贝 | ✅ PASS | envelope copy |
| **AC12** | Auto-Close 异步 Learn + 10s timeout + 3 层 fail-safe | ✅ PASS | goroutine + sync.Once + log+skip |
| **AC13** | Auto-Close test Round 2 等 500ms | ✅ PASS | wait loop |

## 5. 文档验收（High 1 个）

| ID | 验收标准 | 状态 | 证据 |
|----|---------|------|------|
| **AC14** | pipeline-architecture.md ≥ 500 行 + 6 章节齐全 | ✅ PASS | 589 行，§1 总览 / §2 S 场景 / §3 入口 / §4 6 步时序 / §5 闭环 / §6 Cross-references |
| **AC15** | spec.md 在 §Architecture 加引用 | ✅ PASS | 顶部加 1 行指向 pipeline-architecture.md |

## 6. 测试与质量

| 项 | 目标 | 实际 | 状态 |
|----|------|------|------|
| 编译 | `go vet ./...` exit 0 | exit 0 | ✅ PASS |
| 单元测试 | 100% PASS | 22/22 packages PASS, 0 race | ✅ PASS |
| Lint | layer-lint (warn) SUCCESS | SUCCESS | ✅ PASS |
| 单元测试 CI | 必填 checks | SUCCESS | ✅ |
| Commit 数 | 13 | 13 | ✅ PASS |
| 文件变更 | 13 个修改 + 1 个新 | 13 + 1 | ✅ PASS |

## 7. 兼容性

| 维度 | 影响 | 缓解 |
|------|------|------|
| PersistScope fail-fast | 4 个 plan_test 失败 | 已加 `BlastRadius: BlastRadius{PersistScope: ...}` |
| Auto-Close 异步化 | 1 个 orchestrator_autoclose_test 失败 | 已加 500ms wait |
| NewPlanID 格式 | DB 里旧 ID 仍可读 | 旧 ID 不删除，只是不再生成 |
| LP-3 顺序 | Bayesian 信誉累积更快 | 用户不可见 |
| rollback ctx 隔离 | ProtocolPlan 失败时副作用更彻底清理 | 用户不可见 |
| sync.WaitGroup | ExplorationChannel 性能提升 | 用户不可见 |
| 异步 Learn | ReputationStore 写入延迟最高 +10s | 用户不可见 |
| pipeline-architecture.md | 文档新增 | spec.md 加 1 行引用 |

## 8. Out of Scope

- M1-M20 Medium 修复（20 个）→ 后续 cleanup change
- L1-L14 Low 修复（14 个）→ 后续 cleanup change
- `coordinator/aliases.go` 130 行 shim → 单独 Change
- `hubspoke/aliases.go` 80 行 shim → 单独 Change
- 1 个 false positive（Wilson margin formula 已验证正确）

## 9. Risk & Rollback

### Risk
- R1：14 个 commit 合并可能引入意外 behavior change（虽然都有 test 覆盖）
  - Mitigation：每个 commit 独立可 revert
- R2：plan_test 改了 4 个 test 适配新 PersistScope 行为，可能有遗漏
  - Mitigation：run `go test ./internal/layers/orchestration/plan/... -v` 100% 通过
- R3：Auto-Close 异步化后 telemetry 时间戳可能偏移
  - Mitigation：endSpanWithOnce 保留原 sessionSpan，attribute 不变

### Rollback
- 单 commit revert：`git revert <commit-sha>`
- 全量 revert：`git revert -n HEAD~13..HEAD && git commit -m "revert: devrix-d7-mups-v4-review-fixes"`
- 数据回滚：ReputationStore 写入不依赖 commit（已经是 idempotent）

## 10. References

- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/proposal.md`
- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/design.md`
- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/tasks.md`
- `openspec/archive/2026-06-25-devrix-d7-mups-v4-review-fixes/specs/d7-orchestration/spec.md`
- `openspec/specs/d7-orchestration/pipeline-architecture.md`（fix 14 新增）
- 7 个 MUPS Phase archive（9 个 change-id）
- PR #192: https://github.com/fqntxmqee/devrix/pull/192
