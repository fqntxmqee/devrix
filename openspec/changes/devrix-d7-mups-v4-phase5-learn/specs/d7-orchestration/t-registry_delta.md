# T-Registry Delta: D7 MUPS v4.3 Phase 5 — Learn 节点升格

**Change ID:** `devrix-d7-mups-v4-phase5-learn`
**Target:** `openspec/specs/d7-orchestration/t-registry.md` v3.12.0 → v3.13.0
**Created:** 2026-06-23

---

## ADDED Test Points（13 P0 T 点）

### D7-S11-A36: Learn 节点数据契约入口

| T ID | 描述 | 归属 A/F | Test 位置 | Priority |
|------|------|----------|-----------|----------|
| D7-S11-A36-T01 | LearningAsset struct 15 字段 + NewLearningAsset fail-fast + deep copy + 自动时间戳（CreatedAt/ExpiryAt/ContentHash） | D7-S11-A36-F01 | `internal/layers/orchestration/learn/learning_asset_test.go` | P0 |
| D7-S11-A36-T02 | 5 类 AssetContent（SOPAssetContent / ProtocolAssetContent / KnowledgeAssetContent / ConclusionAssetContent / PendingAssetContent 含 MVEState）+ Validate() + SchemaVersion() + ByteSize() + 必填字段 fail-fast + Confidence [0,1] clamp + RetryAttempts 0-3 边界 | D7-S11-A36-F02 | `internal/layers/orchestration/learn/asset_content_test.go` | P0 |
| D7-S11-A36-T03 | LearningClass 5 态 typed enum + String/Parse/Marshal/Unmarshal（空字符串零值 LearningSOP 兼容）+ LearningUnknown 禁用 + 跨域类型上提 shared/types/learning.go | D7-S11-A36-F03 | `internal/shared/types/learning_test.go` | P0 |

### D7-S11-A37: ReputationEvidence + Bayesian Update

| T ID | 描述 | 归属 A/F | Test 位置 | Priority |
|------|------|----------|-----------|----------|
| D7-S11-A37-T04 | ReputationEvidence struct 12 字段 + NewReputationEvidence fail-fast + TrackMode 2 字符串解析 + 冷启动默认值（Alpha=Beta=0 + Mean=0 + Variance=0 + ConfidenceLow=0 + ConfidenceHigh=1） | D7-S11-A37-F01 | `internal/layers/orchestration/learn/reputation_evidence_test.go` | P0 |
| D7-S11-A37-T05 | BayesianUpdate 函数 + 不可变性（prior 不变）+ Pass/Partial/Fail → α/β++ + ⭐G8-1 修复（INDETERMINATE "verifier_parse_failure" 仅 VerifierFailureCount++ 不污染 α/β）+ 其他 INDETERMINATE → IndeterminateCount++ + 冷启动除零防御（Mean=prior.Mean）+ Wilson Score 95% 置信区间 + Mean 50 次 PASS 后收敛 ≈ 1.0 | D7-S11-A37-F02 | `internal/layers/orchestration/learn/bayesian_update_test.go` | P0 |

### D7-S11-A38: AdaptivePrior + DefaultPriors

| T ID | 描述 | 归属 A/F | Test 位置 | Priority |
|------|------|----------|-----------|----------|
| D7-S11-A38-T06 | AdaptivePrior + BetaPrior（String "Beta(α,β)"）+ InjectTarget 3 枚举（IntentQuantizer/HistoricalDetector/RuleClassifier）+ 不可变（无 setter）+ DefaultInjectTargets | D7-S11-A38-F01 | `internal/layers/orchestration/learn/adaptive_prior_test.go` | P0 |
| D7-S11-A38-T07 | DefaultDeveloperPrior Beta(5,3) + DefaultOperatorPrior Beta(8,1) + BuildAdaptivePrior(rep, trackMode) Bayesian 合并公式 + rep==nil 兜底 + trackMode=="" 兜底 + DefaultDeveloperPrior + InjectTargets=DefaultInjectTargets | D7-S11-A38-F02 | `internal/layers/orchestration/learn/build_adaptive_prior_test.go` | P0 |

### D7-S11-A39: Memory 3 通道接口 + 3 实现

| T ID | 描述 | 归属 A/F | Test 位置 | Priority |
|------|------|----------|-----------|----------|
| D7-S11-A39-T08 | Memory interface 4 方法（Store/Retrieve/Delete/List）+ MemoryChannel 3 枚举 + MemoryFilter 4 字段（Class/SessionID/MinStrength/Expired）+ SkillMemory 实现（SOP/Protocol 路由 + ErrAssetClassMismatch）+ FeedbackMemory 实现（Knowledge/Conclusion 路由）+ 并发安全（sync.RWMutex）| D7-S11-A39-F01 | `internal/layers/orchestration/learn/memory_test.go` | P0 |
| D7-S11-A39-T09 | ScheduledMemory 实现（LearningPending 路由）+ ScheduledRetry（Asset/TriggerAt/RetryCount/MaxRetries/LastRetryAt）+ TriggerAt 默认 = asset.ExpiryAt + MaxRetries 默认 3 + 并发安全 + List filter | D7-S11-A39-F02 | `internal/layers/orchestration/learn/scheduled_memory_test.go` | P0 |

### D7-S11-A40: Learner interface + DefaultLearner + Observe 闭环（LP-1）

| T ID | 描述 | 归属 A/F | Test 位置 | Priority |
|------|------|----------|-----------|----------|
| D7-S11-A40-T10 | Learner interface 3 方法（Learn/Inject/ScheduledTick）+ LearnRequest struct 5 字段 + DefaultLearner struct 6 字段 + Learn 流程 5 步（class 选择 → AssetBuilder → Memory 路由 → ScheduledMemory 兜底 → BayesianUpdate）+ Inject 流程（ReputationStore.Get → BuildAdaptivePrior）+ ScheduledTick 流程（TriggerAt ≤ now → RetryCount++ → MaxRetries 耗尽 → FeedbackMemory 警告）+ 3 种 Verdict 路由（Pass→SOP/Partial→Protocol/Fail→Knowledge/INDETERMINATE→Pending）| D7-S11-A40-F01 | `internal/layers/orchestration/learn/learner_test.go` | P0 |
| D7-S11-A40-T11 | AssetBuilder 5 类 Content 构造 + hashContentBytes SHA-256 hex 截断 16 + classToStrength 5 等级映射（LearningSOP→5/Protocol→4/Knowledge→3/Conclusion→2/Pending→1）+ AssetKey 格式（sop:PlanKind:hash / pending:ArtifactID:hash 等）+ ContentHash 幂等（同 Content 产生同 hash）+ Build nil 边界 | D7-S11-A40-F02 | `internal/layers/orchestration/learn/asset_builder_test.go` | P0 |
| D7-S11-A40-T12 | ReputationStore interface 3 方法（Get/Update/List）+ InMemoryReputationStore 实现（sync.RWMutex 并发安全）+ Get cold start 返回 nil,nil + Update nil/empty SessionID 返回 ErrReputationStoreUnavailable + List trackMode 过滤 + limit 上限 | D7-S11-A40-F03 | `internal/layers/orchestration/learn/reputation_store_test.go` | P0 |
| D7-S11-A40-T13 | Observe 节点对接（3 子模块）+ IntentQuantizer.QuantizeWithPrior 使用 prior.PriorBeta + AnomalyDetector.HistoricalDetector.DetectWithPrior 使用 prior.PriorBeta + RuleClassifier.ClassifyWithPrior 使用 prior.PriorBeta + Orchestrator.ProcessMessage LP-1 时序约束（Inject 在 Observe.All 之前）+ LP-1 闭环集成测试（5 节点管道 Observe → Plan → Execute → Verify → Learn → 下一轮 Observe 验证 prior 注入）| D7-S11-A40-F04 | `tests/integration/d7/learn_observe_closure_test.go` + `orchtypes/intent_quantizer_test.go` + `orchtypes/anomaly_detector_test.go` + `decisionplanning/rule_classifier_test.go` + `sessionorchestrator/orchestrator_test.go` | P0 |

---

## Statistics Delta

| 指标 | v3.12.0 (Phase 4) | v3.13.0 (Phase 5) | 增量 |
|------|-------------------|-------------------|------|
| Total T | 155 | 168 | +13 |
| IMPLEMENTED | 155 | 168 | +13 |
| PLANNED | 0 | 0 | 0 |
| P0 | 122 | 135 | +13 |
| Scenarios D7-S11 | 0 | 5 | +5 |

---

## Revision History

| Version | Date | Change | PR |
|---------|------|--------|-----|
| v3.13.0 | 2026-06-23 | Phase 5 Learn 节点升格：13 P0 T 点 IMPLEMENTED (D7-S11-A36-T01/T02/T03 + A37-T04/T05 + A38-T06/T07 + A39-T08/T09 + A40-T10/T11/T12/T13) | #175/#176/#177/#178/#179 + #180 (S6 archive) |

---

## 关联

- 前置：Phase 1 Foundation + Phase 2 PR-A1/PR-B1 + Phase 3 PR-C1/PR-C2 + Phase 4 PR-D1..D4
- 后续：None（5 节点管道完整闭环；可选 Phase 6 持久化升级 + Phase 7 跨会话追踪）
- 设计稿：openspec/changes/devrix-d7-mups-v4-phase5-learn/{proposal,design,tasks}.md