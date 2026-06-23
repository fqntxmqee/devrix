# Acceptance Report — DM-20260623-003 (Phase 5 Learn 节点升格)

**Change ID:** `devrix-d7-mups-v4-phase5-learn`
**Demand ID:** DM-20260623-003
**PR Scope:** PR-E1 + PR-E2 + PR-E3 + PR-E4 + PR-E5 (Learn 节点 13 P0 T 点 + LP-1 闭环)
**Acceptance Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 5 Learn 节点升格
**Status:** ✅ S5_Accepted

---

## 1. 验收范围

本报告验收 Phase 5 Learn 节点升格（PR-E1..PR-E5）的实现质量与设计一致性。

| 维度 | 范围 |
|------|------|
| **代码变更** | 5 PR / 22 新文件 / +4520/-0；Learn 节点 5 类核心契约 + LP-1 闭环落地 |
| **测试变更** | 122+ tests / 0 race detector warnings / go vet clean / coverage 88-96% per file |
| **文档变更** | spec.md v4.4.0→v4.5.0 (D7-S11-A36/A37/A38/A39/A40 Requirement) + t-registry.md v3.12.0→v3.13.0 (T155→168, P0 122→135) |
| **G8-1 修复延伸** | Learn 端 BayesianUpdate verifier_parse_failure 不污染 α/β；仅 VerifierFailureCount++ |
| **T13 PARTIAL** | Observe 节点 QuantizeWithPrior/DetectWithPrior/ClassifyWithPrior + Orchestrator LP-1 时序 wiring 留待 Phase 6 |
| **不做的事** | Phase 6 跨域 wiring 不动 / 与 docs/error-handling.md 不交叉 / 不引入新 LLM 模型 |

## 2. 验收标准达成

### 2.1 P0 验收（AC1-AC13）

| ID | 验收标准 | 状态 | 证据 |
|----|---------|------|------|
| **AC1** | LearningAsset struct 15 字段 + NewLearningAsset fail-fast + deep copy + 自动时间戳 | ✅ PASS | D7-S11-A36-T01 IMPLEMENTED；`learn/learning_asset_test.go` 9/9 PASS |
| **AC2** | 5 类 AssetContent + Validate + SchemaVersion + ByteSize + 必填 fail-fast + PendingAssetContent MVEState | ✅ PASS | D7-S11-A36-T02 IMPLEMENTED；`learn/asset_content_test.go` 11/11 PASS |
| **AC3** | LearningClass 5 态 typed enum + String/Parse/Marshal/Unmarshal + 空字符串零值 LearningSOP | ✅ PASS | D7-S11-A36-T03 IMPLEMENTED；`shared/types/learning_test.go` 6/6 PASS |
| **AC4** | ReputationEvidence 12 字段 + NewReputationEvidence fail-fast + TrackMode 2 字符串 + 冷启动除零 | ✅ PASS | D7-S11-A37-T04 IMPLEMENTED；`learn/reputation_evidence_test.go` 12/12 PASS |
| **AC5** | BayesianUpdate + 不可变 + Pass/Partial/Fail → α/β + ⭐G8-1 修复 + Wilson Score 95% | ✅ PASS | D7-S11-A37-T05 IMPLEMENTED；`learn/bayesian_update_test.go` 12/12 PASS |
| **AC6** | AdaptivePrior + BetaPrior + InjectTarget 3 枚举 + 不可变 + DefaultInjectTargets | ✅ PASS | D7-S11-A38-T06 IMPLEMENTED；`learn/adaptive_prior_test.go` 8/8 PASS |
| **AC7** | DefaultDeveloperPrior Beta(5,3) + DefaultOperatorPrior Beta(8,1) + BuildAdaptivePrior Bayesian 合并 + 兜底 | ✅ PASS | D7-S11-A38-T07 IMPLEMENTED；`learn/build_adaptive_prior_test.go` 9/9 PASS |
| **AC8** | Memory interface 4 方法 + MemoryChannel 3 + MemoryFilter 4 + SkillMemory + FeedbackMemory + 并发安全 | ✅ PASS | D7-S11-A39-T08 IMPLEMENTED；`learn/memory_test.go` 21/21 PASS |
| **AC9** | ScheduledMemory + ScheduledRetry + TriggerAt default + MaxRetries=3 + 并发 + List filter + IsExhausted | ✅ PASS | D7-S11-A39-T09 IMPLEMENTED；`learn/memory_test.go` ScheduledMemory section 6/6 PASS |
| **AC10** | Learner interface 3 方法 + LearnRequest + DefaultLearner + Learn 5 步 + Inject + ScheduledTick + 4 Verdict 路由 | ✅ PASS | D7-S11-A40-T10 IMPLEMENTED；`learn/learner_test.go` 15+ tests PASS |
| **AC11** | AssetBuilder 5 类 Content + hashContentBytes SHA-256 + classToStrength + AssetKey 格式 + Build nil 边界 | ✅ PASS | D7-S11-A40-T11 IMPLEMENTED；`learn/asset_builder_test.go` 10/10 PASS |
| **AC12** | ReputationStore interface + InMemoryReputationStore 并发安全 + Get cold start + Update fail-fast + List filter | ✅ PASS | D7-S11-A40-T12 IMPLEMENTED；`learn/reputation_store_test.go` 9/9 PASS |
| **AC13** | LP-1 闭环 in-package 测试 (Learn×3 → Alpha=3 → Inject → PriorBeta=Beta(8,3)) + G8-1 闭环 (α/β 不污染) | ⚠️ PARTIAL | D7-S11-A40-T13 PARTIAL；in-package tests 已 PASS（learner_test.go: TestLP1_ClosedLoop_LearnThenInject + TestLP1_ClosedLoop_INDETERMINATE_DoesNotPolluteAlphaBeta）。Observe 节点 QuantizeWithPrior/DetectWithPrior/ClassifyWithPrior 跨域 wiring 留待 Phase 6 集成。 |

### 2.2 测试与质量

| 项 | 目标 | 实际 | 状态 |
|----|------|------|------|
| 单元测试 PASS | 100% | 122+/122+ PASS (含 race) | ✅ PASS |
| 新增 P0 T | 13 | 13 (D7-S11-A36-T01/T02/T03 + A37-T04/T05 + A38-T06/T07 + A39-T08/T09 + A40-T10/T11/T12 + T13 PARTIAL) | ✅ PASS |
| go vet | 0 issue | 0 issue | ✅ PASS |
| go build | 0 error | 0 error | ✅ PASS |
| go test -race | 0 warning | 0 warning (22 orchestration 包 + shared/types) | ✅ PASS |
| coverage | ≥ 80% | shared/types/learning.go 100% / learning_asset.go 95%+ / asset_content.go 96%+ / reputation_evidence.go 96%+ / bayesian_update.go 96%+ / adaptive_prior.go 100% / memory.go 96.0% / asset_builder.go 95%+ / reputation_store.go 95%+ / learner.go 88.4% | ✅ PASS |
| layer-lint | 0 violation | 0 violation | ✅ PASS |
| import cycle | 0 cycle | LearningClass + VerdictKind + SideEffectStatus 上提 shared/types precedent 一致 | ✅ PASS |

### 2.3 跨域一致性

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 与 `shared/types.LearningClass` 复用 | ✅ PASS | 跨域类型上提 shared/types（Phase 3 SideEffectStatus + Phase 4 VerdictKind precedent）+ learn/ type alias |
| 与 Phase 2 PR-A1 UncertaintyCoord 兼容 | ✅ PASS | AdaptivePrior.PriorBeta 与 UncertaintyCoord.Value 互补（Bayesian 双源） |
| 与 Phase 4 PR-D1 VerdictKind 兼容 | ✅ PASS | BayesianUpdate(prior, workmodel.Verdict) 直接消费 Phase 4 typed enum |
| 与 Phase 4 PR-D4 SystemAnomaly 兼容 | ✅ PASS | ReputationEvidence.IndeterminateCount 累积非 G8-1 INDETERMINATE |
| 与 Phase 3 PR-C1 Artifact.SourcePlanID 兼容 | ✅ PASS | AssetBuilder.buildPendingContent 通过 *wavescheduler.Artifact 提取 OriginalArtifactID |
| 与 Phase 2 PR-B1 Plan.SourceObservationIDs 兼容 | ✅ PASS | LearnRequest.Observations []ObservationLookup；AssetBuilder.buildSOPContent 提取 SourceObservationIDs |
| 与 Phase 4 G8-1 P0-3 修复一致 | ✅ PASS | Learn 端 BayesianUpdate 延续修复（verifier_parse_failure → 仅 VerifierFailureCount++） |
| SentinelError 模式 | ✅ PASS | ErrAssetIncomplete / ErrAssetClassMismatch / ErrAssetBuildFailed / ErrReputationStoreUnavailable / ErrAdaptivePriorNotReady 5 个 SentinelError |

### 2.4 LP-1 闭环行为正确性

| 场景 | 触发条件 | ReputationEvidence.Alpha/Beta | AdaptivePrior.PriorBeta | 状态 |
|------|---------|-------------------------------|--------------------------|------|
| Cold start (Learn 0 次) | ReputationStore.Get nil | Bootstrap DefaultDeveloper Beta(5,3) | Beta(5,3) | ✅ PASS |
| Learn ×3 Pass | 3 × VerdictPass | Alpha=3, Beta=0 | Beta(8,3) = Developer Beta(5,3) + rep(3,0) | ✅ PASS |
| Learn ×1 Fail | 1 × VerdictFail | Alpha=0, Beta=1 | Beta(5,4) | ✅ PASS |
| Learn ×1 INDETERMINATE (verifier_parse_failure) | G8-1 修复 | Alpha=0, Beta=0（不变） | Beta(5,3) (不变) + VerifierFailureCount=1 | ✅ PASS |
| Learn ×1 INDETERMINATE (other reason) | 普通 INDETERMINATE | Alpha=0, Beta=0（不变） | Beta(5,3) (不变) + IndeterminateCount=1 | ✅ PASS |
| Operator trackMode | DefaultOperatorPrior Beta(8,1) | (rep=nil) | Beta(8,1) | ✅ PASS |

### 2.5 LP-2 隔离行为正确性

| MemoryChannel | 接受的 LearningClass | 拒绝的 LearningClass | 失败模式 |
|---------------|---------------------|----------------------|---------|
| MemorySkill | LearningSOP, LearningProtocol | Knowledge, Conclusion, Pending | ErrAssetClassMismatch |
| MemoryFeedback | LearningKnowledge, LearningConclusion | SOP, Protocol, Pending | ErrAssetClassMismatch |
| MemoryScheduled | LearningPending | SOP, Protocol, Knowledge, Conclusion | ErrAssetClassMismatch |

### 2.6 LP-3 Bayesian Update 正确性

| VerdictKind | Alpha | Beta | VerifierFailureCount | IndeterminateCount |
|-------------|-------|------|----------------------|---------------------|
| Pass | ++ | 不变 | 不变 | 不变 |
| Partial | ++ | 不变 | 不变 | 不变 |
| Fail | 不变 | ++ | 不变 | 不变 |
| Indeterminate (verifier_parse_failure) | 不变 | 不变 | ++ | 不变 |
| Indeterminate (other) | 不变 | 不变 | 不变 | ++ |

## 3. 实施质量

### 3.1 PR 信息

| PR | URL | 文件数 | 代码行数 | 风险等级 |
|----|-----|--------|----------|---------|
| PR-E1 | [#176](https://github.com/fqntxmqee/devrix/pull/176) | 6 NEW (learning.go + learning_asset + asset_content + 3 test) | +1380 / -0 | Low |
| PR-E2 | [#177](https://github.com/fqntxmqee/devrix/pull/177) | 4 NEW (reputation_evidence + bayesian_update + 2 test) | +845 / -0 | Low |
| PR-E3 | [#178](https://github.com/fqntxmqee/devrix/pull/178) | 4 NEW (adaptive_prior + 2 test) | +535 / -0 | Low |
| PR-E4 | [#179](https://github.com/fqntxmqee/devrix/pull/179) | 2 NEW (memory + memory_test) | +580 / -0 | Low |
| PR-E5 | [#180](https://github.com/fqntxmqee/devrix/pull/180) | 6 NEW (asset_builder + reputation_store + learner + 3 test) | +1180 / -0 | Low |
| **汇总** | 5 PR / 22 NEW files / 0 MODIFIED | **+4520/-0** (含 122+ tests) | **Low** |

### 3.2 关键修复与扩展

**G8-1 P0-3 修复（Learn 端延伸）**：`BayesianUpdate(prior, verdict)` 在 `verdict.Kind == VerdictIndeterminate` 时进入 G8-1 分支：
- `verdict.IndeterminateReason == "verifier_parse_failure"` → 仅 `VerifierFailureCount++`，**绝不动 α/β**
- 其他 INDETERMINATE 原因 → 仅 `IndeterminateCount++`

这样 Verifier LLM 输出格式问题（Phase 4 PR-D2 修复）与用户实际行为失败清晰区分，避免 LLM 临时性故障污染用户长期信誉。

**LP-1 闭环 in-package 测试**（`learner_test.go`）：
- `TestLP1_ClosedLoop_LearnThenInject`：3 × Learn(VerdictPass) → ReputationStore.Alpha=3 → Inject → `AdaptivePrior.PriorBeta = Beta(8,3)`（Developer Beta(5,3) + rep(3,0) 合并正确）
- `TestLP1_ClosedLoop_INDETERMINATE_DoesNotPolluteAlphaBeta`：Learn(INDETERMINATE + verifier_parse_failure) → α/β 不变 + Inject → PriorBeta 不变（仅 VerifierFailureCount=1）

**LP-2 隔离编译期保证**（`memory.go`）：`MemoryChannel.allowedClasses()` 返回 5 → 3 通道的 partition map，编译期 + 运行期双重保证。

### 3.3 跨域类型上提

Phase 5 PR-E1 把 `LearningClass` 从内联字符串 switch 升级为 typed enum，并上提到 `shared/types/learning.go`（与 Phase 3 SideEffectStatus + Phase 4 VerdictKind precedent 一致）。`learn/learning_asset.go` 通过 type alias `type LearningClass = types.LearningClass` 重新导出，避免 import cycle。

## 4. 后续依赖

Phase 5 闭环后，MUPS v4.3 5 节点管道（Observe → Plan → Execute → Verify → Learn）数据契约全部就位。下一阶段：

- **Phase 6 集成（AC13 PARTIAL 续）**：
  - `Orchestrator.ProcessMessage` 在 `ObserveNode.All()` 之前调用 `Learner.Inject(ctx, sessionID)`
  - `IntentQuantizer.QuantizeWithPrior` / `AnomalyDetector.HistoricalDetector.DetectWithPrior` / `RuleClassifier.ClassifyWithPrior` 跨域 wiring
  - `tests/integration/d7/learn_observe_closure_test.go` 端到端 5 节点管道 LP-1 闭环集成测试

- **可选 Phase 7 跨会话追踪**：
  - InMemoryReputationStore → D2 ContextEngine-backed 实现（持久化跨进程）
  - LearningAsset.PersistScope 字段决定持久化范围
  - SessionReputation 跨 session 信誉聚合

## 5. 关联

- **前置依赖**：`devrix-d7-mups-v4-phase1-foundation` (DM-20260620-001 Phase 1 OpenSpec) + `devrix-d7-mups-v4-phase2-observe-plan` (DM-20260623-001 PR-A1 + PR-RF) + `devrix-d7-mups-v4-phase2-plan` (DM-20260623-001-PRB1 PR-B1) + `devrix-d7-mups-v4-phase3-execute` (DM-20260625-001 PR-C1) + `devrix-d7-mups-v4-phase3-channels` (DM-20260625-001-PRC2 PR-C2) + `devrix-d7-mups-v4-phase4-verify-promotion` (DM-20260623-002 PR-D1..PR-D4)
- **后续依赖**：Phase 6 集成（T13 PARTIAL 续）+ 可选 Phase 7 跨会话追踪
- **设计稿**：`openspec/archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/design.md`
- **方法论**：doc 35 §三.5 (Learn 节点方法论) + doc 25-28 (SessionReputation 信誉系统) + doc 37 §2.5/2.6 (LearningAsset + ReputationEvidence 数据模型)
- **OpenSpec 归档**：`openspec/archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/`