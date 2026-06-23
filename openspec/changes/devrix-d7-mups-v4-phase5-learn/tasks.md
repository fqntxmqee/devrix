# Tasks: D7 MUPS v4.3 Phase 5 — Learn 节点升格

**Change ID:** `devrix-d7-mups-v4-phase5-learn`
**Demand ID:** DM-20260623-003
**Status:** S2_Proposal → S3_Design → S4_Implemented → S7_Archived
**Created:** 2026-06-23

---

## Phase 0: Setup

- [ ] **P0.1** S2_Proposal review（user approved）
- [ ] **P0.2** 创建 `feat/devrix-d7-mups-v4-phase5-learn` 分支（from master）
- [ ] **P0.3** 同步 `openspec/specs/d7-orchestration/spec.md` v4.4.0 → v4.5.0 占位
- [ ] **P0.4** 同步 `openspec/specs/d7-orchestration/t-registry.md` v3.12.0 → v3.13.0 占位

---

## PR-E1: LearningAsset 5 类 + AssetContent + LearningClass 5 枚举（最小入口）

**目标**：建立 Learn 节点核心数据契约——LearningAsset 不可变 struct + 5 类 AssetContent + LearningClass 5 枚举（含 LearningPending ⭐新增）。

### E1.1 LearningAsset struct + LearningClass 5 枚举（D7-S11-A36-T01）

- [ ] **E1.1.1** 新建 `internal/layers/orchestration/learn/learning_asset.go`
  - 定义 `type LearningAsset struct`（15 字段：ID, SessionID, Class, Strength, SourceSessionIDs, SourceVerdictIDs, Content, AssetKey, ContentHash, FailureCriterion, ExpiryAt, CreatedAt, LastUsedAt, UseCount, TraceID）
  - 全部字段 `validate:"required"` 标注（除 UseCount/LastUsedAt 外必填）
  - 不可变：所有 setter 方法返回新对象（LP-1 衍生）
- [ ] **E1.1.2** 定义 `LearningClass int` 5 枚举（禁用 LearningUnknown=0，启用 LearningSOP/Protocol/Knowledge/Conclusion/Pending）
  - `String() string` 返回 wire format（`sop`/`protocol`/`knowledge`/`conclusion`/`pending`）
  - `ParseLearningClass(s string) (LearningClass, error)` 反向解析（未知值 fail-fast，含 LearningUnknown）
- [ ] **E1.1.3** 定义 `NewLearningAsset(id, sessionID, class, content, assetKey)` 工厂函数
  - fail-fast：SessionID/class/content/assetKey 任一为空 → `ErrAssetIncomplete`
  - 自动设置 CreatedAt=now, ExpiryAt=now+24h, ContentHash=hashContent(content)
  - 不可变 copy：内部存储 deep copy（防外部 mutation）
- [ ] **E1.1.4** 新建 `internal/layers/orchestration/learn/learning_asset_test.go`
  - `TestLearningClass_String_5Classes`：5 enum 值字符串正确
  - `TestLearningClass_ParseLearningClass_5Classes`：5 字符串解析正确
  - `TestLearningClass_ParseLearningClass_LearningUnknownRejected`：LearningUnknown 解析失败
  - `TestNewLearningAsset_RequiredFieldsFailFast`：5 必填字段 fail-fast
  - `TestNewLearningAsset_AutoTimestamps`：CreatedAt/ExpiryAt/ContentHash 自动设置
  - `TestNewLearningAsset_DeepCopy`：Content 不可被外部 mutation 影响

### E1.2 5 类 AssetContent 接口 + 实现（D7-S11-A36-T02）

- [ ] **E1.2.1** 新建 `internal/layers/orchestration/learn/asset_content.go`
  - 定义 `type AssetContent interface { Validate() error; SchemaVersion() string; ByteSize() int }`
  - 定义 `CurrentAssetSchemaVersion = "1.0.0"` 常量
- [ ] **E1.2.2** 定义 `SOPAssetContent`（★ 5，LearningSOP）
  - 字段：Name (必填), Description, Steps (≥1 必填), PreConditions, PostConditions, ApplicableTools, EstimatedMs
  - `Validate() error`：Name/Steps 校验失败返回 `ErrAssetIncomplete`
- [ ] **E1.2.3** 定义 `ProtocolAssetContent`（★ 4，LearningProtocol）
  - 字段：Name, Trigger (必填), Actions, SLA (TargetMs/MaxRetries/OpenTimeout), Fallback
  - `Validate() error`：Trigger 校验失败返回 `ErrAssetIncomplete`
- [ ] **E1.2.4** 定义 `KnowledgeAssetContent`（★ 3，LearningKnowledge）
  - 字段：Topic (必填), Hypothesis (必填), Evidence, CounterEvid, Confidence ([0,1] clamp), RelatedCases
  - `Validate() error`：Topic/Hypothesis 校验失败返回 `ErrAssetIncomplete`
- [ ] **E1.2.5** 定义 `ConclusionAssetContent`（★ 2，LearningConclusion）
  - 字段：Statement (必填), PValue ([0,1]), ConfidenceInterval [2]float64, SampleSize (≥0), Methodology, Limitations
  - `Validate() error`：Statement 校验失败返回 `ErrAssetIncomplete`
- [ ] **E1.2.6** 定义 `PendingAssetContent`（⭐★ 1，LearningPending — 第 5 类）
  - 字段：IndeterminateReason (必填), OriginalArtifactID (必填), RetryAttempts (0-3), MaxRetries (默认 3), NextRetryAt, BlockedReason
  - **MVE checkpoint state 字段**：MVEState (*execute.MVEState, nullable), PlanID, SessionID, Question (MVEState 非空时必填), Options
  - `Validate() error`：IndeterminateReason/OriginalArtifactID 校验失败返回 `ErrAssetIncomplete`；MVEState 非空 + Question 为空返回 `ErrAssetIncomplete`
- [ ] **E1.2.7** 新建 `internal/layers/orchestration/learn/asset_content_test.go`
  - `TestSOPAssetContent_Validate_RequiredFields`：Name/Steps 校验
  - `TestProtocolAssetContent_Validate_TriggerRequired`：Trigger 校验
  - `TestKnowledgeAssetContent_Validate_TopicHypothesisRequired`：Topic/Hypothesis 校验
  - `TestConclusionAssetContent_Validate_StatementRequired`：Statement 校验
  - `TestPendingAssetContent_Validate_IndeterminateReasonRequired`：IndeterminateReason 校验
  - `TestPendingAssetContent_Validate_MVEStateQuestionRequired`：MVEState + Question 校验
  - `TestPendingAssetContent_Validate_RetryAttemptsRange`：RetryAttempts 0-3 边界
  - `TestAssetContent_InterfaceCompliance`：5 类 content 都实现 AssetContent
  - `TestAssetContent_SchemaVersion_Current`：5 类 Content SchemaVersion() 返回 "1.0.0"

### E1.3 跨域类型上提 + type alias（D7-S11-A36-T03）

- [ ] **E1.3.1** 新建 `internal/shared/types/learning.go`
  - 定义 `type LearningClass uint8`（5 枚举，遵循 Phase 3 SideEffectStatus / Phase 4 VerdictKind precedent）
  - String() / ParseLearningClass() / MarshalJSON / UnmarshalJSON
  - KindUnset = 0 默认 LearningSOP（与 ArtifactKind precedent 一致）
- [ ] **E1.3.2** 修改 `internal/layers/orchestration/learn/learning_asset.go`
  - 添加 `type LearningClass = types.LearningClass` type alias
  - 保留 `learn.LearningClass int` 私有定义（PR-E1 完成后由 PR-E2 切换）
- [ ] **E1.3.3** 新建 `internal/shared/types/learning_test.go`
  - `TestLearningClass_String_5Kinds`：5 enum 值字符串正确
  - `TestLearningClass_ParseLearningClass_5Kinds`：5 字符串解析正确
  - `TestLearningClass_MarshalJSON_WireFormat`：JSON 输出字符串
  - `TestLearningClass_UnmarshalJSON_EmptyString_DefaultsToLearningSOP`：空字符串零值兼容

### E1.4 PR-E1 收尾

- [ ] **E1.4.1** `go vet ./internal/shared/types/... ./internal/layers/orchestration/learn/...` — 0 issue
- [ ] **E1.4.2** `go test -race -count=1 ./internal/shared/types/... ./internal/layers/orchestration/learn/...` — 14 tests 100% PASS / 0 race
- [ ] **E1.4.3** `go test -cover ./internal/layers/orchestration/learn/...` — coverage ≥ 80%
- [ ] **E1.4.4** 提交：`feat(d7): MUPS v4 Phase 5 PR-E1 (LearningAsset 5 类 + AssetContent + LearningClass 5 枚举)`
- [ ] **E1.4.5** squash auto-merge 入 master

---

## PR-E2: ReputationEvidence + Bayesian Update（信誉引擎）

**目标**：建立 Learn 节点的信誉累积机制——ReputationEvidence struct + BayesianUpdate 函数 + Wilson Score 置信区间 + G8-1 修复（INDETERMINATE "verifier_parse_failure" 不污染 α/β）。

### E2.1 ReputationEvidence struct（D7-S11-A37-T04）

- [ ] **E2.1.1** 新建 `internal/layers/orchestration/learn/reputation_evidence.go`
  - 定义 `type ReputationEvidence struct`（12 字段：SessionID, TrackMode, Alpha, Beta, Mean, Variance, ConfidenceLow, ConfidenceHigh, LastUpdated, UpdateCount, SourceVerdictIDs, VerifierFailureCount, IndeterminateCount）
  - Alpha/Beta ≥ 0 不变式
  - Mean ∈ [0, 1] 不变式（= Alpha/(Alpha+Beta)）
  - 不可变：BayesianUpdate 返回新对象（防外部 mutation）
- [ ] **E2.1.2** 定义 `TrackMode` 2 字符串常量（`developer` / `operator`）
  - `ParseTrackMode(s string) (string, error)` 反向解析（未知值 fail-fast）
- [ ] **E2.1.3** 定义 `NewReputationEvidence(sessionID, trackMode)` 工厂函数
  - fail-fast：SessionID/trackMode 任一为空 → `ErrReputationStoreUnavailable`
  - 默认 Alpha=Beta=0 + Mean=0 + Variance=0 + ConfidenceLow=0 + ConfidenceHigh=1
- [ ] **E2.1.4** 新建 `internal/layers/orchestration/learn/reputation_evidence_test.go`
  - `TestReputationEvidence_NewReputationEvidence_DefaultZero`：冷启动默认值
  - `TestReputationEvidence_NewReputationEvidence_RequiredFieldsFailFast`：SessionID/trackMode fail-fast
  - `TestReputationEvidence_TrackMode_2Values`：developer/operator 2 字符串
  - `TestReputationEvidence_ParseTrackMode_2Values`：2 字符串解析
  - `TestReputationEvidence_ParseTrackMode_UnknownFailFast`：未知值 fail-fast

### E2.2 BayesianUpdate + Wilson Score + G8-1 修复（D7-S11-A37-T05）

- [ ] **E2.2.1** 修改 `internal/layers/orchestration/learn/reputation_evidence.go`
  - 定义 `func BayesianUpdate(prior *ReputationEvidence, verdict verify.Verdict) *ReputationEvidence` 函数
  - 输入：prior 不可变（copy 副本）+ verdict（含 IndeterminateReason 字段）
  - 输出：新 ReputationEvidence（不动 prior）
- [ ] **E2.2.2** 实现 Bayesian Update 核心逻辑
  - `UpdateCount++` + `LastUpdated = now()` + append SourceVerdictIDs
  - switch verdict.Kind：
    - `VerdictPass` / `VerdictPartial` → `Alpha++`
    - `VerdictFail` → `Beta++`
    - `VerdictIndeterminate` → ⭐**G8-1 修复分支**：
      - `verdict.IndeterminateReason == "verifier_parse_failure"` → 仅 `VerifierFailureCount++`，**绝不更新 α/β**（防 LLM 输出格式问题污染用户信誉）
      - 其他 INDETERMINATE → `IndeterminateCount++`（保持原行为不更新 α/β）
- [ ] **E2.2.3** 实现派生指标计算
  - 边界：`Alpha + Beta == 0`（冷启动除零）→ Mean = prior.Mean（保持冷启动默认值 Developer Beta(5,3) → 0.625）+ Variance=0 + ConfidenceLow=0 + ConfidenceHigh=1
  - 正常：`Mean = Alpha / (Alpha+Beta)` + `Variance = Alpha*Beta / ((Alpha+Beta)^2 * (Alpha+Beta+1))`
  - 置信区间：Wilson Score 95% 区间（z=1.96）
- [ ] **E2.2.4** 实现 Wilson Score 函数
  - `func wilsonScoreInterval(alpha, beta int, confidence float64) (float64, float64)` 私有函数
  - 公式：`p̂ = alpha/(alpha+beta)` + `z = 1.96 (95% confidence)` + `center = p̂ + z²/(2n)` + `margin = z*sqrt(p̂(1-p̂)/n + z²/(4n²))` + `denominator = 1 + z²/n`
  - 返回 `(center-margin)/denominator` 和 `(center+margin)/denominator`
- [ ] **E2.2.5** 新建测试 `TestBayesianUpdate_VerdictPass_IncrementsAlpha`
- [ ] **E2.2.6** 新建测试 `TestBayesianUpdate_VerdictFail_IncrementsBeta`
- [ ] **E2.2.7** 新建测试 `TestBayesianUpdate_VerdictPartial_IncrementsAlpha`
- [ ] **E2.2.8** 新建测试 `TestBayesianUpdate_VerdictIndeterminate_OtherReason_NotPollutes`（不污染 α/β，仅 IndeterminateCount++）
- [ ] **E2.2.9** 新建测试 `TestBayesianUpdate_VerdictIndeterminate_VerifierParseFailure_OnlyIncrementsVerifierFailureCount`（⭐G8-1 修复）
- [ ] **E2.2.10** 新建测试 `TestBayesianUpdate_ColdStartZeroAlphaBeta_KeepsPriorMean`（冷启动除零防御）
- [ ] **E2.2.11** 新建测试 `TestBayesianUpdate_MeanConvergence50`：50 次 PASS 后 Mean ≈ 1.0（基于 Developer Beta(5,3) 收敛）
- [ ] **E2.2.12** 新建测试 `TestBayesianUpdate_DoesNotMutatePrior`：返回新对象，prior 不变
- [ ] **E2.2.13** 新建测试 `TestWilsonScoreInterval_95_ConfidenceBounds`：已知样本的置信区间

### E2.3 PR-E2 收尾

- [ ] **E2.3.1** `go vet ./internal/layers/orchestration/learn/...` — 0 issue
- [ ] **E2.3.2** `go test -race -count=1 ./internal/layers/orchestration/learn/...` — 10 tests 100% PASS / 0 race
- [ ] **E2.3.3** `go test -cover ./internal/layers/orchestration/learn/...` — coverage ≥ 80%
- [ ] **E2.3.4** 提交：`feat(d7): MUPS v4 Phase 5 PR-E2 (ReputationEvidence + Bayesian Update + G8-1)`
- [ ] **E2.3.5** squash auto-merge 入 master

---

## PR-E3: AdaptivePrior + DefaultPriors（先验工厂）

**目标**：建立 Learn 节点的先验注入工厂——AdaptivePrior struct + BetaPrior + DefaultDeveloperPrior (Beta(5,3)) / DefaultOperatorPrior (Beta(8,1)) + BuildAdaptivePrior 函数。

### E3.1 AdaptivePrior + BetaPrior + InjectTarget（D7-S11-A38-T06）

- [ ] **E3.1.1** 新建 `internal/layers/orchestration/learn/adaptive_prior.go`
  - 定义 `type AdaptivePrior struct`（3 字段：Reputation *ReputationEvidence, PriorBeta BetaPrior, InjectTargets []InjectTarget）
  - 不可变：所有字段 final + 无 setter
- [ ] **E3.1.2** 定义 `type BetaPrior struct`（2 字段：Alpha, Beta）
  - `String() string` 返回 wire format（`Beta(α,β)`）
- [ ] **E3.1.3** 定义 `type InjectTarget int` 3 枚举
  - `InjectIntentQuantizer (0)` — 注入到 IntentQuantizer.Quantize
  - `InjectHistoricalDetector (1)` — 注入到 AnomalyDetector.HistoricalDetector
  - `InjectRuleClassifier (2)` — 注入到 RuleClassifier.Classify
  - `String() string` 返回 wire format（`intent_quantizer`/`historical_detector`/`rule_classifier`）
- [ ] **E3.1.4** 新建 `internal/layers/orchestration/learn/adaptive_prior_test.go`
  - `TestBetaPrior_String_BetaFormat`：Beta(α,β) 格式正确
  - `TestInjectTarget_String_3Targets`：3 枚举字符串正确
  - `TestInjectTarget_ParseInjectTarget_3Targets`：3 字符串解析正确
  - `TestAdaptivePrior_Immutable_NoSetters`：compile-time 验证（struct 无 setter 方法）

### E3.2 DefaultPriors + BuildAdaptivePrior（D7-S11-A38-T07）

- [ ] **E3.2.1** 修改 `internal/layers/orchestration/learn/adaptive_prior.go`
  - 定义 `DefaultDeveloperPrior = BetaPrior{Alpha: 5, Beta: 3}` 常量（来自 doc 25 §四）
  - 定义 `DefaultOperatorPrior = BetaPrior{Alpha: 8, Beta: 1}` 常量（来自 doc 25 §四）
  - 定义 `DefaultInjectTargets = []InjectTarget{InjectIntentQuantizer, InjectHistoricalDetector, InjectRuleClassifier}` 常量
- [ ] **E3.2.2** 定义 `func BuildAdaptivePrior(rep *ReputationEvidence, trackMode string) *AdaptivePrior` 函数
  - 选择 DefaultPrior：trackMode == "developer" → DefaultDeveloperPrior；否则 DefaultOperatorPrior
  - Bayesian 合并：mergedAlpha = prior.Alpha + rep.Alpha；mergedBeta = prior.Beta + rep.Beta
  - 返回 AdaptivePrior（含 merged BetaPrior + DefaultInjectTargets + rep）
- [ ] **E3.2.3** 实现边界处理
  - `rep == nil` → 返回 AdaptivePrior(PriorBeta=DefaultPrior, InjectTargets=DefaultInjectTargets, Reputation=nil)
  - `trackMode == ""` → 默认使用 DefaultDeveloperPrior（fail-safe）
- [ ] **E3.2.4** 新建测试 `TestBuildAdaptivePrior_DeveloperMode_DefaultDeveloperPrior`：Developer track → Beta(5,3) 合并
- [ ] **E3.2.5** 新建测试 `TestBuildAdaptivePrior_OperatorMode_DefaultOperatorPrior`：Operator track → Beta(8,1) 合并
- [ ] **E3.2.6** 新建测试 `TestBuildAdaptivePrior_NilReputation_UseDefaultPrior`：nil rep 边界
- [ ] **E3.2.7** 新建测试 `TestBuildAdaptivePrior_EmptyTrackMode_DefaultDeveloper`：空 trackMode 兜底
- [ ] **E3.2.8** 新建测试 `TestBuildAdaptivePrior_BayesianMerge`：合并公式验证（α_prior+rep）

### E3.3 PR-E3 收尾

- [ ] **E3.3.1** `go vet ./internal/layers/orchestration/learn/...` — 0 issue
- [ ] **E3.3.2** `go test -race -count=1 ./internal/layers/orchestration/learn/...` — 8 tests 100% PASS / 0 race
- [ ] **E3.3.3** `go test -cover ./internal/layers/orchestration/learn/...` — coverage ≥ 80%
- [ ] **E3.3.4** 提交：`feat(d7): MUPS v4 Phase 5 PR-E3 (AdaptivePrior + DefaultPriors + BuildAdaptivePrior)`
- [ ] **E3.3.5** squash auto-merge 入 master

---

## PR-E4: Memory 3 通道接口 + 3 实现（记忆通道）

**目标**：建立 Learn 节点的 3 通道记忆架构——Memory interface + 3 实现（SkillMemory + FeedbackMemory + ScheduledMemory）+ LP-2 隔离校验。

### E4.1 Memory interface + MemoryChannel + MemoryFilter（D7-S11-A39-T08）

- [ ] **E4.1.1** 新建 `internal/layers/orchestration/learn/memory.go`
  - 定义 `type Memory interface { Store(ctx, asset) error; Retrieve(ctx, key) (*LearningAsset, error); Delete(ctx, key) error; List(ctx, filter) ([]*LearningAsset, error) }`
  - 定义 `type MemoryChannel int` 3 枚举（`MemorySkill (0)`/`MemoryFeedback (1)`/`MemoryScheduled (2)`）
  - 定义 `type MemoryFilter struct`（4 字段：Class LearningClass, SessionID string, MinStrength orchtypes.CertaintyStrength, Expired bool）
- [ ] **E4.1.2** 定义 `ErrAssetClassMismatch = errors.New("learn: asset class does not match memory channel")` SentinelError
- [ ] **E4.1.3** 新建 `internal/layers/orchestration/learn/memory_test.go`
  - `TestMemoryChannel_String_3Channels`：3 枚举字符串正确
  - `TestMemoryFilter_ZeroValue`：zero value 验证
  - `TestMemoryFilter_Class_SessionID_Strength_Expired`：4 字段赋值

### E4.2 SkillMemory + FeedbackMemory 实现（D7-S11-A39-T09 续）

- [ ] **E4.2.1** 定义 `type SkillMemory struct { Store map[string]*LearningAsset; mu sync.RWMutex }`
  - `Store(ctx, asset)`：asset.Class ∈ {LearningSOP, LearningProtocol} 才接受，否则 ErrAssetClassMismatch；否则 Store[asset.AssetKey] = asset
  - `Retrieve(ctx, key)`：从 Store 读取 + 校验 Class
  - `Delete(ctx, key)`：从 Store 删除
  - `List(ctx, filter)`：遍历 Store 过滤（Class/SessionID/MinStrength/Expired）
  - 并发安全：所有方法加 mu 锁
- [ ] **E4.2.2** 定义 `type FeedbackMemory struct { Store map[string]*LearningAsset; mu sync.RWMutex }`
  - `Store(ctx, asset)`：asset.Class ∈ {LearningKnowledge, LearningConclusion} 才接受
  - 其余方法同 SkillMemory
- [ ] **E4.2.3** 新建测试 `TestSkillMemory_Store_AcceptsSOPAndProtocol`：LearningSOP/Protocol 接受
- [ ] **E4.2.4** 新建测试 `TestSkillMemory_Store_RejectsKnowledgeAndConclusion`：LearningKnowledge/Conclusion 返回 ErrAssetClassMismatch
- [ ] **E4.2.5** 新建测试 `TestSkillMemory_Retrieve_Delete_List`：3 方法正确
- [ ] **E4.2.6** 新建测试 `TestSkillMemory_Concurrent_StoreRetrieve`：race 检测
- [ ] **E4.2.7** 新建测试 `TestFeedbackMemory_Store_AcceptsKnowledgeAndConclusion`：Knowledge/Conclusion 接受
- [ ] **E4.2.8** 新建测试 `TestFeedbackMemory_Store_RejectsSOPAndProtocol`：SOP/Protocol 返回 ErrAssetClassMismatch
- [ ] **E4.2.9** 新建测试 `TestMemoryFilter_List_ByClassBySessionIDByStrength`：List 过滤验证

### E4.3 ScheduledMemory + ScheduledRetry（D7-S11-A39-T10）

- [ ] **E4.3.1** 定义 `type ScheduledRetry struct { Asset *LearningAsset; TriggerAt time.Time; RetryCount int; MaxRetries int; LastRetryAt time.Time }`
- [ ] **E4.3.2** 定义 `type ScheduledMemory struct { Store map[string]*ScheduledRetry; mu sync.RWMutex }`
  - `Store(ctx, asset)`：asset.Class == LearningPending 才接受
  - `Retrieve(ctx, key)`：从 Store 读取 ScheduledRetry
  - `Delete(ctx, key)`：删除
  - `List(ctx, filter)`：遍历过滤
  - 并发安全
- [ ] **E4.3.3** 新建测试 `TestScheduledMemory_Store_AcceptsOnlyPending`：LearningPending 接受，其他 ErrAssetClassMismatch
- [ ] **E4.3.4** 新建测试 `TestScheduledMemory_DefaultMaxRetries_3`：ScheduledRetry.MaxRetries 默认 3
- [ ] **E4.3.5** 新建测试 `TestScheduledMemory_TriggerAt_DefaultsToExpiryAt`：TriggerAt 默认 = asset.ExpiryAt
- [ ] **E4.3.6** 新建测试 `TestScheduledMemory_Concurrent_StoreRetrieve`：race 检测

### E4.4 PR-E4 收尾

- [ ] **E4.4.1** `go vet ./internal/layers/orchestration/learn/...` — 0 issue
- [ ] **E4.4.2** `go test -race -count=1 ./internal/layers/orchestration/learn/...` — 12 tests 100% PASS / 0 race
- [ ] **E4.4.3** `go test -cover ./internal/layers/orchestration/learn/...` — coverage ≥ 80%
- [ ] **E4.4.4** 提交：`feat(d7): MUPS v4 Phase 5 PR-E4 (Memory 3 通道接口 + 3 实现)`
- [ ] **E4.4.5** squash auto-merge 入 master

---

## PR-E5: Learner interface + DefaultLearner + Observe 闭环（节点级 + LP-1 闭环）

**目标**：完成 Learn 节点的节点级抽象——Learner interface + DefaultLearner 实现 + AssetBuilder + ScheduledTick 调度器 + Observe 节点对接（IntentQuantizer / HistoricalDetector / RuleClassifier 接收 AdaptivePrior）+ LP-1 闭环集成测试。

### E5.1 Learner interface + LearnRequest + DefaultLearner（D7-S11-A40-T10）

- [ ] **E5.1.1** 新建 `internal/layers/orchestration/learn/learner.go`
  - 定义 `type Learner interface { Learn(ctx, req) ([]LearningAsset, error); Inject(ctx, sessionID) (*AdaptivePrior, error); ScheduledTick(ctx) error }`
  - 定义 `type LearnRequest struct`（5 字段：Verdict verify.Verdict, Plan plan.Plan, Observations []orchtypes.Observation, Artifact execute.Artifact, SessionID string）
- [ ] **E5.1.2** 定义 `type DefaultLearner struct`（6 字段：SkillMemory Memory, FeedbackMemory Memory, ScheduledMemory Memory, ReputationStore ReputationStore, AssetBuilder *AssetBuilder, BayesianUpdater func(prior *ReputationEvidence, verdict verify.Verdict) *ReputationEvidence）
- [ ] **E5.1.3** 实现 `Learn(ctx, req)` 方法
  - 步骤 1：`classFromVerdictClass(req.Verdict.Class)` 选择 LearningClass（INDETERMINATE → LearningPending）
  - 步骤 2：`l.AssetBuilder.Build(ctx, req, class)` 构造 LearningAsset（nil → `ErrAssetBuildFailed`）
  - 步骤 3：按 class 路由到对应 Memory（LP-2 隔离）
  - 步骤 4：INDETERMINATE 额外写入 ScheduledMemory（LearningPending）
  - 步骤 5：`ReputationStore.Get` + `BayesianUpdater` + `ReputationStore.Update`
- [ ] **E5.1.4** 实现 `Inject(ctx, sessionID)` 方法
  - `ReputationStore.Get(ctx, sessionID)` 读 ReputationEvidence
  - `determineTrackMode(rep)` 决定 trackMode（rep.TrackMode → developer/operator）
  - `BuildAdaptivePrior(rep, trackMode)` 构造 AdaptivePrior
- [ ] **E5.1.5** 实现 `ScheduledTick(ctx)` 方法
  - 遍历 ScheduledMemory.Store → 检查 TriggerAt ≤ now → 重试 Verifier（Phase 4 doc 45 §4.6）
  - RetryCount++ + LastRetryAt = now
  - RetryCount >= MaxRetries → 删除 ScheduledRetry + 写入 FeedbackMemory 警告
- [ ] **E5.1.6** 新建 `internal/layers/orchestration/learn/learner_test.go`
  - `TestLearner_Learn_VerdictPass_BuildsSOPAsset`：Pass → SOPAsset
  - `TestLearner_Learn_VerdictPartial_BuildsProtocolAsset`：Partial → ProtocolAsset
  - `TestLearner_Learn_VerdictFail_BuildsKnowledgeAsset`：Fail → KnowledgeAsset
  - `TestLearner_Learn_VerdictIndeterminate_BuildsPendingAsset`：INDETERMINATE → PendingAsset
  - `TestLearner_Learn_ClassifyByVerdictClass`：ComplianceVerdict→SOP, TimelinessVerdict→Protocol, RootCauseVerdict→Knowledge, StatisticalVerdict→Conclusion
  - `TestLearner_Learn_AssetBuildFailed_ReturnsError`：AssetBuilder 返回 nil → ErrAssetBuildFailed
  - `TestLearner_Learn_AssetClassMismatch_RoutesCorrectly`：3 通道 LP-2 隔离验证
  - `TestLearner_Inject_BuildsAdaptivePrior`：Inject 正确构造 AdaptivePrior
  - `TestLearner_Inject_NilReputation_UseDefaultPrior`：nil rep 兜底
  - `TestLearner_ScheduledTick_RetryPendingAsset`：ScheduledTick 重试 PendingAsset
  - `TestLearner_ScheduledTick_ExhaustedMaxRetries_MoveToFeedbackMemory`：MaxRetries 耗尽 → FeedbackMemory
  - `TestDefaultLearner_InterfaceCompliance`：DefaultLearner 实现 Learner interface

### E5.2 AssetBuilder（D7-S11-A40-T11）

- [ ] **E5.2.1** 新建 `internal/layers/orchestration/learn/asset_builder.go`
  - 定义 `type AssetBuilder struct{}`
  - 定义 `func (b *AssetBuilder) Build(ctx, req, class) *LearningAsset` 方法
  - switch class 路由到 5 类 Content 构造
  - 调用 `NewLearningAsset` 工厂函数（fail-fast ErrAssetIncomplete）
- [ ] **E5.2.2** 实现 5 类 Content 构造逻辑
  - `LearningSOP`：`extractSOPName(req)` + `extractStepsFromPlan(req.Plan)` + `extractTools(req.Artifact)`
  - `LearningProtocol`：`extractProtocolName(req)` + `extractTriggerFromVerdict(req.Verdict)` + `extractActions(req.Artifact)`
  - `LearningKnowledge`：`extractTopic(req.Verdict)` + `extractHypothesis(req.Verdict)` + `extractEvidence(req.Verdict)`
  - `LearningConclusion`：`extractConclusion(req.Verdict)` + `extractPValue(req.Verdict)`
  - `LearningPending`：`req.Verdict.IndeterminateReason` + `req.Artifact.ID` + 默认 RetryAttempts=0/MaxRetries=3
- [ ] **E5.2.3** 实现 `hashContent(content) string` 私有函数（SHA-256 hex）
- [ ] **E5.2.4** 实现 `classToStrength(class) orchtypes.CertaintyStrength` 映射（LearningSOP→5, Protocol→4, Knowledge→3, Conclusion→2, Pending→1）
- [ ] **E5.2.5** 新建 `internal/layers/orchestration/learn/asset_builder_test.go`
  - `TestAssetBuilder_BuildSOPAsset`：5 类 Content 构造正确
  - `TestAssetBuilder_BuildProtocolAsset`：Protocol Content
  - `TestAssetBuilder_BuildKnowledgeAsset`：Knowledge Content
  - `TestAssetBuilder_BuildConclusionAsset`：Conclusion Content
  - `TestAssetBuilder_BuildPendingAsset`：Pending Content + MVEState 字段透传
  - `TestAssetBuilder_AssetKeyFormat`：sop:PlanKind:hash 格式
  - `TestAssetBuilder_ContentHash_Stable`：相同 Content 产生相同 hash（幂等）
  - `TestAssetBuilder_ClassToStrength_5Levels`：5 类 → 5 等级映射

### E5.3 ReputationStore 接口 + 默认实现（D7-S11-A40-T12）

- [ ] **E5.3.1** 新建 `internal/layers/orchestration/learn/reputation_store.go`
  - 定义 `type ReputationStore interface { Get(ctx, sessionID) (*ReputationEvidence, error); Update(ctx, evidence) error; List(ctx, trackMode, limit) ([]*ReputationEvidence, error) }`
  - 定义 `ErrReputationStoreUnavailable = errors.New("learn: reputation store unavailable")` SentinelError
- [ ] **E5.3.2** 定义 `type InMemoryReputationStore struct { Store map[string]*ReputationEvidence; mu sync.RWMutex }`
  - `Get/Update/List` 实现 + 并发安全 + trackMode 过滤
  - `Update` 失败（如 nil evidence）返回 ErrReputationStoreUnavailable
- [ ] **E5.3.3** 新建测试 `TestInMemoryReputationStore_Get_Update_List`：3 方法正确
- [ ] **E5.3.4** 新建测试 `TestInMemoryReputationStore_List_FilterByTrackMode`：List trackMode 过滤
- [ ] **E5.3.5** 新建测试 `TestInMemoryReputationStore_Concurrent_GetUpdate`：race 检测

### E5.4 Observe 节点对接 + LP-1 闭环（D7-S11-A40-T13）

- [ ] **E5.4.1** 修改 `internal/layers/orchestration/orchtypes/intent_quantizer.go`
  - 新增 `QuantizeWithPrior(ctx, intent, prior *learn.AdaptivePrior) (IntentPayload, error)` 方法
  - `prior != nil` → 使用 prior.PriorBeta 作为 Beta 先验
  - `prior == nil` → 使用 DefaultDeveloperPrior 兜底
- [ ] **E5.4.2** 修改 `internal/layers/orchestration/orchtypes/anomaly_detector.go`
  - 新增 `HistoricalDetector.DetectWithPrior(ctx, anomalies, prior *learn.AdaptivePrior) (AnomalyReport, error)` 方法
  - `prior != nil` → 使用 prior.PriorBeta 作为历史基线
- [ ] **E5.4.3** 修改 `internal/layers/orchestration/decisionplanning/rule_classifier.go`
  - 新增 `ClassifyWithPrior(ctx, observations, prior *learn.AdaptivePrior) (PlanKind, error)` 方法
  - `prior != nil` → 使用 prior.PriorBeta 影响 classification 决策
- [ ] **E5.4.4** 修改 `internal/layers/orchestration/sessionorchestrator/orchestrator.go` ProcessMessage 入口
  - 在 `ObserveNode.All()` 调用之前调用 `Learner.Inject(ctx, sessionID)` 拿到 AdaptivePrior
  - 把 AdaptivePrior 注入到 ObserveRequest.Prior 字段
  - 各 Observer 子模块（IntentQuantizer/HistoricalDetector/RuleClassifier）从 ObserveRequest.Prior 读取
- [ ] **E5.4.5** 新建集成测试 `tests/integration/d7/learn_observe_closure_test.go`
  - **LP-1 闭环 E2E**：VerdictPass → Learner.Learn → ReputationStore 更新 → 下一轮 ProcessMessage → Learner.Inject → Observe.QuantizeWithPrior 使用 prior.PriorBeta
  - 验证：5 类 Asset + ReputationEvidence + AdaptivePrior 跨 session 累积
  - 验证：INDETERMINATE → PendingAsset + ScheduledMemory + VerifierFailureCount++
  - 验证：5 节点管道（Observe → Plan → Execute → Verify → Learn）完整跑通
- [ ] **E5.4.6** 新建单元测试 `TestIntentQuantizer_QuantizeWithPrior_UsePriorBeta`：使用 prior.PriorBeta
- [ ] **E5.4.7** 新建单元测试 `TestIntentQuantizer_QuantizeWithPrior_NilPrior_UseDefault`：nil prior 兜底
- [ ] **E5.4.8** 新建单元测试 `TestAnomalyDetector_HistoricalDetector_DetectWithPrior`：使用 prior.PriorBeta
- [ ] **E5.4.9** 新建单元测试 `TestRuleClassifier_ClassifyWithPrior_UsePriorBeta`：使用 prior.PriorBeta
- [ ] **E5.4.10** 新建单元测试 `TestOrchestrator_ProcessMessage_InjectPriorBeforeObserve`：LP-1 时序约束（Inject 在 Observe.All 之前）

### E5.5 PR-E5 收尾

- [ ] **E5.5.1** `go vet ./...` — 0 issue
- [ ] **E5.5.2** `go test -race -count=1 ./internal/layers/orchestration/learn/... ./internal/layers/orchestration/sessionorchestrator/...` — 16 tests 100% PASS / 0 race
- [ ] **E5.5.3** `go test -race -count=1 ./tests/integration/d7/...` — LP-1 闭环 E2E 100% PASS
- [ ] **E5.5.4** `go test -cover ./internal/layers/orchestration/learn/...` — coverage ≥ 80%
- [ ] **E5.5.5** layer-lint check — 无新增违规
- [ ] **E5.5.6** 提交：`feat(d7): MUPS v4 Phase 5 PR-E5 (Learner + DefaultLearner + Observe 闭环)`
- [ ] **E5.5.7** squash auto-merge 入 master

---

## Phase 6: S6 Archive

- [ ] **P6.1** 创建 S6 archive PR（chore(openspec): S6 archive devrix-d7-mups-v4-phase5-learn）
  - 移动 5 文件 → `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/`
  - 创建 `.openspec.yaml`（含 demand_id + change_id + status）
  - 创建 `acceptance-report.md`（含 14 AC PASS 清单）
- [ ] **P6.2** 同步 `openspec/specs/d7-orchestration/spec.md` v4.4.0 → v4.5.0
  - 更新版本号 + Last Updated
  - 在 Archived Changes 列表追加 Phase 5 entry
- [ ] **P6.3** 同步 `openspec/specs/d7-orchestration/t-registry.md` v3.12.0 → v3.13.0
  - 更新版本号 + Last Updated
  - 在 Change 段追加 Phase 5 entry（13 T 点 IMPLEMENTED，T 155→168, P0 122→135, Scenarios D7-S11 0→5）
- [ ] **P6.4** 同步 `openspec/demand-archive-index.md` 添加 Phase 5 entry
- [ ] **P6.5** 运行 `scripts/verify-archive.sh` — 13/13 PASS, 0 failure
- [ ] **P6.6** 提交 S6 archive PR + squash auto-merge
- [ ] **P6.7** 更新 memory：phase5-s7-archived + phase5-6-status（与 phase 4 precedent 一致）

---

## 执行顺序（避免循环依赖）

1. **第 1 步**：PR-E1（LearningAsset 5 类 + AssetContent）— 无依赖
2. **第 2 步**：PR-E2（ReputationEvidence + Bayesian Update）— 依赖 PR-E1
3. **第 3 步**：PR-E3（AdaptivePrior + DefaultPriors）— 依赖 PR-E2
4. **第 4 步**：PR-E4（Memory 3 通道）— 依赖 PR-E1
5. **第 5 步**：PR-E5（Learner + Observe 闭环）— 依赖全部前 4 PR
6. **第 6 步**：S6 Archive — 依赖全部 5 PR

**并行机会**：PR-E2 和 PR-E4 可并行启动（都依赖 PR-E1）

---

## Cross-references

- **设计稿**：doc 46 (D7 Learn 节点详细技术方案 1124 行)
- **方法论**：doc 35 §三 (5 节点管道方法论) + doc 37 §2.5-6 (LearningAsset + ReputationEvidence 实体)
- **先验来源**：doc 25 §四 (Developer Beta(5,3) / Operator Beta(8,1))
- **3 通道记忆**：doc 27 §三 (Skill/Feedback/Scheduled 分类)
- **跨会话累积策略**：doc 27 §四
- **前置 OpenSpec**：Phase 1 Foundation + Phase 2 PR-A1/PR-B1 + Phase 3 PR-C1/PR-C2 + Phase 4 PR-D1..D4
- **后续 OpenSpec**：None（5 节点管道完整闭环；可选 Phase 6 持久化升级）