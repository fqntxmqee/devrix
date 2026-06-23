# Spec Delta: D7 MUPS v4.3 Phase 5 — Learn 节点升格

**Change ID:** `devrix-d7-mups-v4-phase5-learn`
**Target:** `openspec/specs/d7-orchestration/spec.md` v4.4.0 → v4.5.0
**Created:** 2026-06-23

---

## ADDED Requirements（14 ADDED）

### D7-S11-A36: LearningAsset 5 类 + AssetContent + LearningClass 5 枚举

#### Requirement: LearningClass 5 态 typed enum

The D7 Learn 节点 MUST expose a typed `LearningClass` enum in `internal/shared/types/learning.go` with 5 离散态（`LearningSOP` / `LearningProtocol` / `LearningKnowledge` / `LearningConclusion` / `LearningPending`），实现 `String()` / `ParseLearningClass()` / `MarshalJSON` / `UnmarshalJSON`，且 `UnmarshalJSON` 对空字符串保持零值（`LearningSOP`）向后兼容。

`learn/learning_asset.go` MUST 通过 type alias `type LearningClass = types.LearningClass` 重新导出，避免 import cycle。

**Rationale**: 与 Phase 3 `SideEffectStatus` precedent（DM-20260625-001 PR-C1 D7-S9-A25-T04）+ Phase 4 `VerdictKind` precedent（DM-20260623-002 PR-D1 D7-S10-A32-T01）一致；跨域 typed enum 上提 `shared/types/` 避免 import cycle。

#### Requirement: LearningAsset struct 15 字段 + 不可变

`internal/layers/orchestration/learn/learning_asset.go` MUST 定义 `LearningAsset` struct 含 15 字段：

- `ID string` — UUID v7（必填）
- `SessionID string` — 当前 SessionID（必填）
- `Class LearningClass` — 5 类枚举（必填）
- `Strength orchtypes.CertaintyStrength` — ★ 1-5（自动派生自 Class）
- `SourceSessionIDs []string` — LP-5 跨会话可追溯（必填，长度 ≥ 1）
- `SourceVerdictIDs []string` — LP-5（必填，长度 ≥ 1）
- `Content AssetContent` — 多态（必填，5 类各异）
- `AssetKey string` — 幂等去重 key（必填）
- `ContentHash string` — 内容 SHA-256 hash（自动派生）
- `FailureCriterion string` — LP-4 可证伪（必填，默认 "ExpiryAt < now() OR UseCount > MaxUseCount"）
- `ExpiryAt time.Time` — TTL 默认 now+24h（必填）
- `CreatedAt time.Time` — 自动设置（必填）
- `LastUsedAt time.Time` — 用于 LRU 淘汰
- `UseCount int` — 用于 LRU 淘汰
- `TraceID string` — D5 trace 关联

不可变：所有 setter 方法返回新对象。

#### Requirement: 5 类 AssetContent 接口 + Validate()

`internal/layers/orchestration/learn/asset_content.go` MUST 实现：

- `AssetContent` interface 3 方法：
  - `Validate() error` — 必填字段 fail-fast 返回 `ErrAssetIncomplete`
  - `SchemaVersion() string` — 返回 `"1.0.0"`
  - `ByteSize() int` — 字节估算
- `SOPAssetContent`（★ 5）字段：Name（必填）/ Description / Steps（≥1 必填）/ PreConditions / PostConditions / ApplicableTools / EstimatedMs
- `ProtocolAssetContent`（★ 4）字段：Name / Trigger（必填）/ Actions / SLA / Fallback
- `KnowledgeAssetContent`（★ 3）字段：Topic（必填）/ Hypothesis（必填）/ Evidence / CounterEvid / Confidence（[0,1] clamp）/ RelatedCases
- `ConclusionAssetContent`（★ 2）字段：Statement（必填）/ PValue / ConfidenceInterval / SampleSize / Methodology / Limitations
- `PendingAssetContent`（⭐★ 1）字段：IndeterminateReason（必填）/ OriginalArtifactID（必填）/ RetryAttempts（0-3）/ MaxRetries（默认 3）/ NextRetryAt / BlockedReason / **MVEState**（nullable *execute.MVEState）/ PlanID / SessionID / Question（MVEState 非空时必填）/ Options

### D7-S11-A37: ReputationEvidence + Bayesian Update

#### Requirement: ReputationEvidence struct 12 字段 + 不可变

`internal/layers/orchestration/learn/reputation_evidence.go` MUST 定义 `ReputationEvidence` struct 含 12 字段：

- `SessionID string` — 主体 SessionID（必填）
- `TrackMode TrackMode` — `"developer"` / `"operator"`（必填）
- `Alpha int` — Beta 分布成功数（≥ 0）
- `Beta int` — Beta 分布失败数（≥ 0）
- `Mean float64` — alpha/(alpha+beta)，∈ [0, 1]
- `Variance float64` — alpha*beta/((alpha+beta)²*(alpha+beta+1))
- `ConfidenceLow float64` — Wilson Score 95% 下界
- `ConfidenceHigh float64` — Wilson Score 95% 上界
- `LastUpdated time.Time`
- `UpdateCount int`
- `SourceVerdictIDs []string`
- `VerifierFailureCount int` — ⭐G8-1 修复：Verifier 失败计数（不污染 α/β）
- `IndeterminateCount int` — ⭐G5-3 修复：INDETERMINATE 计数

#### Requirement: BayesianUpdate 函数 + Wilson Score + G8-1 修复

`internal/layers/orchestration/learn/reputation_evidence.go` MUST 实现 `BayesianUpdate(prior *ReputationEvidence, verdict verify.Verdict) *ReputationEvidence` 函数：

- **不可变**：输入 prior 不变（copy 副本），返回新对象
- **Pass/Partial** → `Alpha++`
- **Fail** → `Beta++`
- **Indeterminate** → ⭐**G8-1 修复分支**：
  - `verdict.IndeterminateReason == "verifier_parse_failure"` → 仅 `VerifierFailureCount++`，**绝不更新 α/β**
  - 其他 INDETERMINATE 原因 → `IndeterminateCount++`（保持原行为不更新 α/β）
- **派生指标**：
  - 冷启动除零防御：`Alpha + Beta == 0` → `Mean = prior.Mean`（保持冷启动默认值 Developer Beta(5,3) → 0.625）+ Variance=0 + ConfidenceLow=0 + ConfidenceHigh=1
  - 正常：`Mean = Alpha/(Alpha+Beta)` + `Variance = Alpha*Beta/((Alpha+Beta)²*(Alpha+Beta+1))`
  - 置信区间：Wilson Score 95%（z=1.96）

**Rationale**: G8-1 P0-3 修复（DM-20260623-002 Phase 4 VerifyWithRetry parse failure → INDETERMINATE）的 Learn 端延伸：避免 LLM 输出格式问题污染用户信誉（用户实际行为可能成功，Verdict 失败仅因 Verifier LLM 输出格式问题）。

### D7-S11-A38: AdaptivePrior + DefaultPriors

#### Requirement: AdaptivePrior + BetaPrior + InjectTarget

`internal/layers/orchestration/learn/adaptive_prior.go` MUST 实现：

- `AdaptivePrior` struct 3 字段（不可变）：
  - `Reputation *ReputationEvidence` — 当前 Reputation（nullable）
  - `PriorBeta BetaPrior` — Bayesian 合并后的 Beta 先验
  - `InjectTargets []InjectTarget` — 注入目标列表
- `BetaPrior` struct 2 字段：`Alpha int` + `Beta int` + `String()` 返回 `"Beta(α,β)"` 格式
- `InjectTarget` 3 枚举：
  - `InjectIntentQuantizer (0)` — 注入 IntentQuantizer.Quantize
  - `InjectHistoricalDetector (1)` — 注入 AnomalyDetector.HistoricalDetector
  - `InjectRuleClassifier (2)` — 注入 RuleClassifier.Classify

#### Requirement: DefaultPriors + BuildAdaptivePrior

`internal/layers/orchestration/learn/adaptive_prior.go` MUST 实现：

- `DefaultDeveloperPrior = BetaPrior{Alpha: 5, Beta: 3}` 常量（doc 25 §四）
- `DefaultOperatorPrior = BetaPrior{Alpha: 8, Beta: 1}` 常量（doc 25 §四）
- `DefaultInjectTargets = []InjectTarget{InjectIntentQuantizer, InjectHistoricalDetector, InjectRuleClassifier}` 常量
- `func BuildAdaptivePrior(rep *ReputationEvidence, trackMode TrackMode) *AdaptivePrior` 函数
  - 边界：`rep == nil` → `AdaptivePrior(PriorBeta=DefaultPrior, InjectTargets=DefaultInjectTargets, Reputation=nil)`
  - 边界：`trackMode == ""` → 默认使用 `DefaultDeveloperPrior`（fail-safe）
  - Bayesian 合并：`mergedAlpha = prior.Alpha + rep.Alpha`；`mergedBeta = prior.Beta + rep.Beta`

### D7-S11-A39: Memory 3 通道接口 + 3 实现

#### Requirement: Memory interface + 3 通道实现（LP-2 隔离）

`internal/layers/orchestration/learn/memory.go` MUST 实现：

- `Memory` interface 4 方法：
  - `Store(ctx, asset) error` — 路由到对应通道（LP-2 隔离检测）
  - `Retrieve(ctx, key) (*LearningAsset, error)`
  - `Delete(ctx, key) error`
  - `List(ctx, filter MemoryFilter) ([]*LearningAsset, error)`
- `MemoryChannel` 3 枚举：`MemorySkill (0)` / `MemoryFeedback (1)` / `MemoryScheduled (2)`
- `MemoryFilter` struct 4 字段：`Class LearningClass` / `SessionID string` / `MinStrength orchtypes.CertaintyStrength` / `Expired bool`
- 3 实现（全部 `sync.RWMutex` 并发安全）：
  - `SkillMemory` — `LearningSOP` + `LearningProtocol` 路由，其他返回 `ErrAssetClassMismatch`
  - `FeedbackMemory` — `LearningKnowledge` + `LearningConclusion` 路由，其他返回 `ErrAssetClassMismatch`
  - `ScheduledMemory` — `LearningPending` 路由，其他返回 `ErrAssetClassMismatch` + `ScheduledRetry{Asset, TriggerAt, RetryCount, MaxRetries, LastRetryAt}`
- `ErrAssetClassMismatch = errors.New("learn: asset class does not match memory channel")` SentinelError

### D7-S11-A40: Learner interface + DefaultLearner + Observe 闭环（LP-1）

#### Requirement: Learner interface + DefaultLearner

`internal/layers/orchestration/learn/learner.go` MUST 实现：

- `Learner` interface 3 方法：
  - `Learn(ctx, req LearnRequest) ([]LearningAsset, error)`
  - `Inject(ctx, sessionID) (*AdaptivePrior, error)` — ⭐LP-1 闭环入口
  - `ScheduledTick(ctx) error` — ScheduledMemory 重试调度
- `LearnRequest` struct 5 字段：`Verdict verify.Verdict` / `Plan plan.Plan` / `Observations []orchtypes.Observation` / `Artifact execute.Artifact` / `SessionID string`
- `DefaultLearner` struct 6 字段：3 Memory + ReputationStore + AssetBuilder + BayesianUpdater（可注入）
- `Learn()` 流程：
  1. 选择 LearningClass（INDETERMINATE → LearningPending）
  2. `AssetBuilder.Build(ctx, req, class)`（nil → `ErrAssetBuildFailed`）
  3. 按 class 路由到对应 Memory（LP-2 隔离）
  4. INDETERMINATE 额外写入 ScheduledMemory
  5. `ReputationStore.Get` + `BayesianUpdater` + `ReputationStore.Update`
- `Inject()` 流程：
  1. `ReputationStore.Get(ctx, sessionID)` 读 ReputationEvidence
  2. `BuildAdaptivePrior(rep, rep.TrackMode)` 构造 AdaptivePrior
  3. 边界：`rep == nil` → `BuildAdaptivePrior(nil, TrackModeDeveloper)` 兜底
- `ScheduledTick()` 流程：
  1. 遍历 ScheduledMemory.Store → 检查 `TriggerAt ≤ now`
  2. RetryCount++ + LastRetryAt = now
  3. `RetryCount >= MaxRetries` → 删除 ScheduledRetry + 写入 FeedbackMemory 警告

#### Requirement: AssetBuilder 5 类 Content 构造

`internal/layers/orchestration/learn/asset_builder.go` MUST 实现：

- `AssetBuilder` struct + `Build(ctx, req, class) *LearningAsset` 方法
- switch class 路由到 5 类 Content 构造：
  - `LearningSOP` → `SOPAssetContent{Name, Steps, ApplicableTools, ...}`
  - `LearningProtocol` → `ProtocolAssetContent{Name, Trigger, Actions, ...}`
  - `LearningKnowledge` → `KnowledgeAssetContent{Topic, Hypothesis, Evidence, ...}`
  - `LearningConclusion` → `ConclusionAssetContent{Statement, PValue, ...}`
  - `LearningPending` → `PendingAssetContent{IndeterminateReason, OriginalArtifactID, MVEState, ...}`
- `hashContentBytes(content) string` 私有函数（SHA-256 hex 截断 16 字符）
- `classToStrength(class) orchtypes.CertaintyStrength` 映射（LearningSOP→5, Protocol→4, Knowledge→3, Conclusion→2, Pending→1）
- AssetKey 格式：`sop:PlanKind:hash` / `protocol:PlanKind:hash` / `knowledge:PlanKind:hash` / `conclusion:PlanKind:hash` / `pending:ArtifactID:hash`

#### Requirement: ReputationStore 接口 + InMemoryReputationStore

`internal/layers/orchestration/learn/reputation_store.go` MUST 实现：

- `ReputationStore` interface 3 方法：
  - `Get(ctx, sessionID) (*ReputationEvidence, error)` — cold start 返回 nil, nil
  - `Update(ctx, evidence) error` — nil/empty SessionID 返回 `ErrReputationStoreUnavailable`
  - `List(ctx, trackMode, limit) ([]*ReputationEvidence, error)` — trackMode 过滤 + limit 上限
- `InMemoryReputationStore` 默认实现（`sync.RWMutex` 并发安全）

#### Requirement: Observe 节点对接（LP-1 闭环）

3 个 Observer 子模块 MUST 接收 `*learn.AdaptivePrior` 作为先验：

- `orchtypes/intent_quantizer.go`：
  - 新增 `QuantizeWithPrior(ctx, intent, prior) (IntentPayload, error)` 方法
  - `prior != nil` → 使用 `prior.PriorBeta.Alpha/Beta` 作为分类 Beta 先验
  - `prior == nil` → 使用 `DefaultDeveloperPrior` 兜底
- `orchtypes/anomaly_detector.go` HistoricalDetector：
  - 新增 `DetectWithPrior(ctx, anomalies, prior) (AnomalyReport, error)` 方法
  - `prior != nil` → 使用 `prior.PriorBeta` 作为历史基线
- `decisionplanning/rule_classifier.go`：
  - 新增 `ClassifyWithPrior(ctx, observations, prior) (PlanKind, error)` 方法
  - `prior != nil` → 使用 `prior.PriorBeta` 影响 classification 决策

`sessionorchestrator/orchestrator.go` ProcessMessage 入口 MUST 在 `ObserveNode.All()` 调用之前调用 `Learner.Inject(ctx, sessionID)` 拿到 AdaptivePrior 并注入到 `ObserveRequest.Prior` 字段（LP-1 时序约束）。

---

## Statistics Delta

| 指标 | v4.4.0 (Phase 4) | v4.5.0 (Phase 5) | 增量 |
|------|------------------|------------------|------|
| 节点数 | 5 节点（前 4 + Verify 升格） | 5 节点（前 4 + Learn 升格） | — |
| LP 节点级原则 | 4（Verify） | 5（Learn：LP-1..LP-5） | +1 |
| 资产类 | 0（无 LearningAsset） | 5（SOP/Protocol/Knowledge/Conclusion/Pending） | +5 |
| 记忆通道 | 0（散落） | 3（Skill/Feedback/Scheduled） | +3 |
| 信誉模型 | 简单计数 | Bayesian Beta + Wilson Score | — |
| 先验注入 | 无 | AdaptivePrior + 3 InjectTarget | — |

---

## Revision History

| Version | Date | Change | PR |
|---------|------|--------|-----|
| v4.5.0 | 2026-06-23 | Phase 5 Learn 节点升格：14 ADDED Requirement（LearningAsset 5 类 + ReputationEvidence Bayesian + AdaptivePrior + 3 通道 Memory + Learner + LP-1 闭环） | #175/#176/#177/#178/#179 + #180 (S6 archive) |

---

## 关联

- 前置：Phase 1 Foundation + Phase 2 PR-A1/PR-B1 + Phase 3 PR-C1/PR-C2 + Phase 4 PR-D1..D4
- 后续：None（5 节点管道完整闭环；可选 Phase 6 持久化升级 + Phase 7 跨会话追踪）
- 设计稿：openspec/changes/devrix-d7-mups-v4-phase5-learn/{proposal,design,tasks}.md