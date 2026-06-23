# Tasks: D7 MUPS v4.3 Phase 4 — Verify 节点升格

**Change ID:** `devrix-d7-mups-v4-phase4-verify-promotion`
**Demand ID:** DM-20260623-002
**Status:** S2_Proposal → S3_Design → S4_Implemented → S7_Archived
**Created:** 2026-06-23

---

## Phase 0: Setup

- [ ] **P0.1** S2_Proposal review（user approved）
- [ ] **P0.2** 创建 `feat/devrix-d7-mups-v4-phase4-verify-promotion` 分支（from master）
- [ ] **P0.3** 同步 `openspec/specs/d7-orchestration/spec.md` v4.3.0 → v4.4.0 占位
- [ ] **P0.4** 同步 `openspec/specs/d7-orchestration/t-registry.md` v3.11.0 → v3.12.0 占位

---

## PR-D1: AggregateVerdicts (G3-1) + VerdictKind typed enum

**目标**：把分散在 UncertaintyCoord.FromVerifier 内联的 4 态 string 升级为 typed enum，新增 AggregateVerdicts 4 策略聚合函数。

### D1.1 VerdictKind typed enum（D7-S10-A32-T01）

- [ ] **D1.1.1** 新建 `internal/shared/types/verdict.go`
  - 定义 `type VerdictKind uint8`
  - 4 个常量：`VerdictPass (0)` / `VerdictPartial (1)` / `VerdictIndeterminate (2)` / `VerdictFail (3)`
  - `KindUnset = 0` zero value 默认 VerdictPass（与 ArtifactKind precedent 一致）
  - `String() string` 返回 wire format（`pass` / `partial` / `indeterminate` / `fail`）
  - `ParseVerdictKind(s string) (VerdictKind, error)` 反向解析（未知值 fail-fast）
  - `MarshalJSON` 输出字符串（omitempty）
  - `UnmarshalJSON` 解析字符串（空字符串 → VerdictPass 默认，零值兼容）
- [ ] **D1.1.2** 新建 `internal/shared/types/verdict_test.go`
  - `TestVerdictKind_String_4Kinds`：4 enum 值字符串正确
  - `TestVerdictKind_ParseVerdictKind_4Kinds`：4 字符串解析正确
  - `TestVerdictKind_ParseVerdictKind_UnknownFailFast`：未知值返回 error
  - `TestVerdictKind_MarshalJSON_WireFormat`：JSON 输出字符串
  - `TestVerdictKind_UnmarshalJSON_EmptyString_DefaultsToZeroValue`：空字符串零值兼容
- [ ] **D1.1.3** 修改 `internal/layers/orchestration/orchtypes/uncertainty_coord.go`
  - 添加 `type VerdictKind = types.VerdictKind` type alias（保持 PR-A1 FromVerifier 调用方零修改）
  - 保留 `case "pass"/"partial"/"indeterminate"/"fail"` 字符串 switch 行为（PR-D2 才统一替换）

### D1.2 AggregationStrategy + AggregateVerdicts（D7-S10-A32-T02）

- [ ] **D1.2.1** 新建 `internal/layers/orchestration/workmodel/aggregate_verdicts.go`
  - 定义 `type AggregationStrategy uint8`
  - 4 个常量：`WeakConjunction (0)` / `StrongConjunction (1)` / `Majority (2)` / `ThresholdByPass (3)`
  - 4 个常量分别含义：
    - `WeakConjunction` — 任一 PASS 即 PASS（OR 语义，最宽松）
    - `StrongConjunction` — 全 PASS 才 PASS（AND 语义，最严格）
    - `Majority` — PASS 数 > len/2 即 PASS（多数派）
    - `ThresholdByPass` — PASS 数 ≥ 阈值（默认 1）即 PASS
  - `String() string` 返回 wire format（`weak_conjunction` / `strong_conjunction` / `majority` / `threshold_by_pass`）
  - `ParseAggregationStrategy(s string) (AggregationStrategy, error)` 反向解析
  - `MarshalJSON` / `UnmarshalJSON` 同 VerdictKind
  - 定义 `Verdict` struct（Kind VerdictKind + Confidence float64 + Reason string + SourceID string）
  - 定义 `AggregateVerdicts(verdicts []Verdict, strategy AggregationStrategy) Verdict` 函数
- [ ] **D1.2.2** AggregateVerdicts 边界与策略实现
  - 边界：`len(verdicts) == 0` → 返回 `Verdict{Kind: VerdictIndeterminate, Confidence: 0, Reason: "empty_verdict_set"}`
  - 边界：`len(verdicts) == 1` → 直接返回该 Verdict（无需聚合）
  - 边界：所有 Verdict Kind 相同 → 直接返回该 Kind（无需按 strategy 聚合）
  - 策略实现：
    - `WeakConjunction` — 任一 PASS → PASS；任一 FAIL → FAIL；否则 INDETERMINATE
    - `StrongConjunction` — 全 PASS → PASS；任一 FAIL → FAIL；任一 INDETERMINATE → INDETERMINATE
    - `Majority` — PASS 数 > len/2 → PASS；FAIL 数 > len/2 → FAIL；否则 INDETERMINATE
    - `ThresholdByPass` — PASS 数 ≥ Threshold（默认 1，可配置）→ PASS；否则 INDETERMINATE
- [ ] **D1.2.3** 新建 `internal/layers/orchestration/workmodel/aggregate_verdicts_test.go`
  - `TestAggregationStrategy_String_4Strategies`：4 策略字符串正确
  - `TestAggregationStrategy_ParseAggregationStrategy_4Strategies`：4 字符串解析正确
  - `TestAggregateVerdicts_EmptySlice_ReturnsIndeterminate`：空切片边界
  - `TestAggregateVerdicts_SingleVerdict_ReturnsDirectly`：单元素边界
  - `TestAggregateVerdicts_AllSameKind_ReturnsThatKind`：同质边界
  - `TestAggregateVerdicts_WeakConjunction_AnyPassWins`：OR 语义
  - `TestAggregateVerdicts_WeakConjunction_AnyFailLoses`：OR + FAIL 边界
  - `TestAggregateVerdicts_StrongConjunction_AllPassRequired`：AND 语义
  - `TestAggregateVerdicts_StrongConjunction_OneFailLoses`：AND + FAIL 边界
  - `TestAggregateVerdicts_Majority_HalfStrict`：多数派（success > len/2）
  - `TestAggregateVerdicts_ThresholdByPass_DefaultOne`：阈值 1
  - `TestAggregateVerdicts_ConfidenceAvgAndMaxReason`：聚合 Confidence 取 avg，Reason 取最具体

### D1.3 PR-D1 收尾

- [ ] **D1.3.1** `go vet ./internal/shared/types/... ./internal/layers/orchestration/workmodel/...` — 0 issue
- [ ] **D1.3.2** `go test -race -count=1 ./internal/shared/types/... ./internal/layers/orchestration/workmodel/...` — 12 tests 100% PASS / 0 race
- [ ] **D1.3.3** `go test -cover ./internal/layers/orchestration/workmodel/...` — coverage ≥ 80%
- [ ] **D1.3.4** 提交：`feat(d7): MUPS v4 Phase 4 PR-D1 (VerdictKind enum + AggregateVerdicts)` (PR #172)
- [ ] **D1.3.5** squash auto-merge 入 master

---

## PR-D2: VerdictToExitReason (G8-1 P0-3 修复) + 14 ExitReason

**目标**：建立 Verdict → ExitReason 的语义映射（避免 orchestrator.go 内联 string switch），修复 VerifyWithRetry parse failure 误判为 FAIL 的 bug（G8-1 P0-3），ExitReason 从 8 扩展到 14。

### D2.1 VerdictToExitReason 函数（D7-S10-A33-T03）

- [ ] **D2.1.1** 新建 `internal/layers/orchestration/turn/verdict_to_exit_reason.go`
  - 定义 `VerdictToExitReason(v Verdict, sessionID string) ExitReason` 函数
  - 4 VerdictKind → ExitReason 映射：
    - `VerdictPass` → `ExitReasonNatural`（health LLM finish）
    - `VerdictPartial` → `ExitReasonPartialVerified`（部分 PASS，需人工 review）— 新增
    - `VerdictIndeterminate` → `ExitReasonVerifierAbstain`（verifier abstain，需人工 review）— 新增
    - `VerdictFail` → `ExitReasonVerifierFail`（verifier 判定 FAIL）— 新增
  - `SystemAnomaly=true` 覆盖：`ExitReasonSystemAnomaly` — 新增
- [ ] **D2.1.2** 新建 `internal/layers/orchestration/turn/verdict_to_exit_reason_test.go`
  - `TestVerdictToExitReason_4Kinds`：4 Verdict → 4 ExitReason 映射
  - `TestVerdictToExitReason_SystemAnomalyOverrides`：SystemAnomaly 覆盖
  - `TestVerdictToExitReason_EmptyVerdict_DefaultsToIndeterminate`：空 Verdict → VerifierAbstain
  - `TestVerdictToExitReason_NilConfidence_DefaultsToZero`：nil confidence 边界

### D2.2 ExitReason 8 → 14 扩展（D7-S10-A33-T04）

- [ ] **D2.2.1** 修改 `internal/layers/orchestration/turn/orchestrator.go`
  - 在 ExitReason 常量块（line 73-97）追加 6 个新值：
    - `ExitReasonPartialVerified = "partial_verified"` — 部分验证通过
    - `ExitReasonVerifierAbstain = "verifier_abstain"` — verifier abstain
    - `ExitReasonVerifierFail = "verifier_fail"` — verifier 判定 FAIL
    - `ExitReasonSystemAnomaly = "system_anomaly"` — CatSystem 异常
    - `ExitReasonUnresolved = "unresolved"` — 失败但可重试
    - `ExitReasonAbstain = "abstain"` — verifier 主动 abstain
  - 既有 8 个 enum 值字符串保持不变（`natural` / `max_turns` / `aborted_user` / `aborted_llm` / `aborted_tool` / `repeated_tool` / `tool_failure` / `token_diminishing`）
  - 添加 ExitReason 14 值枚举的 wire format 反向解析函数 `ParseExitReason(s string) (ExitReason, error)`
- [ ] **D2.2.2** 修改 `internal/layers/orchestration/turn/orchestrator_test.go`
  - `TestExitReason_14Values_AllParseable`：14 值可正确反向解析
  - `TestExitReason_8LegacyValues_StringUnchanged`：8 既有 enum 字符串保持不变（向后兼容）

### D2.3 VerifyWithRetry parse failure → INDETERMINATE（G8-1 P0-3）

- [ ] **D2.3.1** 新建 `internal/layers/orchestration/workmodel/verify_with_retry.go`
  - 定义 `VerifierOutput` struct（Raw string + ParsedKind VerdictKind + Confidence float64）
  - 定义 `ParseVerifierOutput(raw string) (VerifierOutput, error)` 函数（单次 parse，失败返回 error）
  - 定义 `ParseVerifierOutputWithRetry(raw string, maxRetries int) VerifierOutput` 函数（重试 3 次后 parse failure → 返回 `VerifierOutput{ParsedKind: VerdictIndeterminate, Confidence: 0, Raw: raw}`，不返回 error）
- [ ] **D2.3.2** 修改 `internal/layers/orchestration/orchtypes/uncertainty_coord.go` FromVerifier 函数
  - 把字符串 switch 改为 typed enum switch（`case VerdictPass:` 替代 `case "pass":`）
  - 未知 VerdictKind 仍然 fail-fast（`NewUncertaintyCoordInvalidVerdictKindError(kind)`），但 kind 参数从 string 改为 VerdictKind
- [ ] **D2.3.3** 新建 `internal/layers/orchestration/workmodel/verify_with_retry_test.go`
  - `TestParseVerifierOutput_ValidJSON_ReturnsVerdict`：正常 parse
  - `TestParseVerifierOutput_InvalidJSON_ReturnsError`：单次失败 error
  - `TestParseVerifierOutputWithRetry_3Failures_ReturnsIndeterminate`：3 次 parse 失败 → INDETERMINATE（修复 G8-1）
  - `TestParseVerifierOutputWithRetry_2Failures1Success_ReturnsVerdict`：2 失败 + 1 成功 → 成功结果
  - `TestParseVerifierOutputWithRetry_AllSuccess_ReturnsFirst`：全成功 → 第一次结果

### D2.4 PR-D2 收尾

- [ ] **D2.4.1** `go vet ./internal/layers/orchestration/turn/... ./internal/layers/orchestration/workmodel/...` — 0 issue
- [ ] **D2.4.2** `go test -race -count=1 ./internal/layers/orchestration/turn/...` — 14 tests 100% PASS / 0 race
- [ ] **D2.4.3** `go test -cover ./internal/layers/orchestration/turn/...` — coverage ≥ 80%
- [ ] **D2.4.4** `go test -race ./internal/layers/orchestration/...` — 0 v2 regression
- [ ] **D2.4.5** 提交：`feat(d7): MUPS v4 Phase 4 PR-D2 (VerdictToExitReason + 14 ExitReason + G8-1 修复)` (PR #173)
- [ ] **D2.4.6** squash auto-merge 入 master

---

## PR-D3: EvidenceExtractor — 结构化 Evidence

**目标**：从 Verifier LLM 输出提取结构化 Evidence（Reason/Confidence/Counterexample），为 Phase 5 Learn 节点的 LearningAsset 生成准备数据。

### D3.1 Evidence struct（D7-S10-A34-T05）

- [ ] **D3.1.1** 新建 `internal/layers/orchestration/workmodel/evidence.go`
  - 定义 `Evidence` struct：
    - `Reason string`（判定依据，自然语言描述）
    - `Confidence float64`（置信度 ∈ [0,1]）
    - `Counterexample string`（反例，若有）
    - `SourceRef string`（来源 Verifier ID / Plan ID / Observation ID）
    - `ExtractedAt time.Time`（提取时间）
  - 定义 `NewEvidence(reason string, confidence float64, sourceRef string) (Evidence, error)` 工厂方法
  - `Validate() error` — 强制 Reason 非空 + Confidence ∈ [0,1] + SourceRef 非空
  - `MarshalJSON` / `UnmarshalJSON` 同 VerdictKind
- [ ] **D3.1.2** 新建 `internal/layers/orchestration/workmodel/evidence_test.go`
  - `TestEvidence_NewEvidence_ValidInputs`：正常构造
  - `TestEvidence_NewEvidence_EmptyReason_FailsValidation`：空 Reason 失败
  - `TestEvidence_NewEvidence_ConfidenceOutOfRange_Clamped`：Confidence > 1 / < 0 → clamp 到 [0,1]
  - `TestEvidence_Validate_AllFieldsRequired`：必填字段验证
  - `TestEvidence_MarshalJSON_WireFormat`：JSON 序列化正确

### D3.2 EvidenceExtractor interface（D7-S10-A34-T06）

- [ ] **D3.2.1** 新建 `internal/layers/orchestration/workmodel/evidence_extractor.go`
  - 定义 `EvidenceExtractor` interface（2 方法）：
    - `Extract(ctx context.Context, verifierOutput VerifierOutput) ([]Evidence, error)` — 从 Verifier LLM 输出提取 Evidence 列表
    - `Validate(evidence []Evidence) error` — 验证 Evidence 列表合法性
  - 实现 `LLMEvidenceExtractor`（基于现有 verifier prompt + 正则表达式提取 Reason/Confidence/Counterexample）
  - 实现 `StubEvidenceExtractor`（返回固定 Evidence，便于测试）
- [ ] **D3.2.2** 新建 `internal/layers/orchestration/workmodel/evidence_extractor_test.go`
  - `TestLLMEvidenceExtractor_ValidOutput_Extracts3Fields`：正常 LLM 输出提取 3 字段
  - `TestLLMEvidenceExtractor_MalformedOutput_ReturnsError`：畸形输出返回 error
  - `TestLLMEvidenceExtractor_EmptyOutput_ReturnsEmptySlice`：空输出空切片
  - `TestStubEvidenceExtractor_ReturnsFixedEvidence`：stub 返回固定值
  - `TestEvidenceExtractor_Validate_EmptyReason_Fails`：Validate 验证失败

### D3.3 PR-D3 收尾

- [ ] **D3.3.1** `go vet ./internal/layers/orchestration/workmodel/...` — 0 issue
- [ ] **D3.3.2** `go test -race -count=1 ./internal/layers/orchestration/workmodel/...` — 10 tests 100% PASS / 0 race
- [ ] **D3.3.3** `go test -cover ./internal/layers/orchestration/workmodel/...` — coverage ≥ 80%
- [ ] **D3.3.4** 提交：`feat(d7): MUPS v4 Phase 4 PR-D3 (Evidence struct + EvidenceExtractor)` (PR #174)
- [ ] **D3.3.5** squash auto-merge 入 master

---

## PR-D4: SystemAnomaly 异常聚合 — 节点级 wiring

**目标**：聚合 CatSystem 类 Observation 异常信号为 SystemAnomaly 决策，wire 到 UncertaintyCoord.FromVerifier 的 systemAnomaly 参数。

### D4.1 SystemAnomalyAggregator（D7-S10-A35-T07）

- [ ] **D4.1.1** 新建 `internal/layers/orchestration/workmodel/system_anomaly.go`
  - 定义 `SystemAnomalyConfig` struct：
    - `AnomalyThreshold int`（默认 3，AnomaliesCount ≥ Threshold 触发 SystemAnomaly）
    - `MinCatSystemRatio float64`（默认 0.5，CatSystem 数量 / 总 AnomaliesCount ≥ Ratio 触发 SystemAnomaly）
  - 定义 `SystemAnomalyAggregator` struct（内含 cfg + 统计 state）
  - `Evaluate(report UncertaintyReport) bool` — 返回是否触发 SystemAnomaly
    - 触发条件：`(AnomaliesCount ≥ AnomalyThreshold) && (CatSystem/AnomaliesCount ≥ MinCatSystemRatio)`
  - `RecordCatSystem(count int)` — 累加 CatSystem 异常
  - `Reset()` — 重置 state
- [ ] **D4.1.2** 新建 `internal/layers/orchestration/workmodel/system_anomaly_test.go`
  - `TestSystemAnomalyAggregator_BelowThreshold_NoTrigger`：AnomaliesCount < Threshold → false
  - `TestSystemAnomalyAggregator_AboveThreshold_AllCatSystem_Triggers`：CatSystem 全量 → true
  - `TestSystemAnomalyAggregator_AboveThreshold_MostlyCatBusiness_NoTrigger`：CatBusiness 主导 → false
  - `TestSystemAnomalyAggregator_AboveThreshold_HalfHalf_DefaultTriggers`：边界 50%
  - `TestSystemAnomalyAggregator_RecordCatSystem_Accumulates`：累加正确
  - `TestSystemAnomalyAggregator_Reset_ClearsState`：Reset 清空 state

### D4.2 ObserveNode wiring 集成（D7-S10-A35-T08）

- [ ] **D4.2.1** 修改 `internal/layers/orchestration/workmodel/uncertainty.go`
  - 添加 `EvaluateSystemAnomaly(report UncertaintyReport) bool` 函数（封装 SystemAnomalyAggregator.Evaluate）
  - 添加 `BuildUncertaintyCoordFromReport(report UncertaintyReport, verifier Verdict) (UncertaintyCoord, error)` 函数
    - 调用 `EvaluateSystemAnomaly(report)` 获取 systemAnomaly
    - 调用 `FromVerifier(verifier.Kind.String(), verifier.Confidence, verifier.Reason, systemAnomaly)` 构造 UncertaintyCoord
- [ ] **D4.2.2** 新建 `internal/layers/orchestration/workmodel/uncertainty_system_anomaly_test.go`
  - `TestEvaluateSystemAnomaly_HighCatSystem_Triggers`：CatSystem 高 → true
  - `TestEvaluateSystemAnomaly_LowCatSystem_NoTrigger`：CatSystem 低 → false
  - `TestBuildUncertaintyCoordFromReport_NormalCase`：正常构造
  - `TestBuildUncertaintyCoordFromReport_SystemAnomalyOverrides_Value`：SystemAnomaly → Value=0.95
  - `TestBuildUncertaintyCoordFromReport_InvalidVerkindKind_ReturnsError`：未知 VerdictKind → error

### D4.3 PR-D4 收尾

- [ ] **D4.3.1** `go vet ./internal/layers/orchestration/workmodel/...` — 0 issue
- [ ] **D4.3.2** `go test -race -count=1 ./internal/layers/orchestration/workmodel/...` — 12 tests 100% PASS / 0 race
- [ ] **D4.3.3** `go test -race ./internal/layers/orchestration/...` — 0 v2 regression
- [ ] **D4.3.4** 提交：`feat(d7): MUPS v4 Phase 4 PR-D4 (SystemAnomaly aggregation + ObserveNode wiring)` (PR #175)
- [ ] **D4.3.5** squash auto-merge 入 master

---

## Phase 5: 文档同步

- [ ] **P5.1** `openspec/specs/d7-orchestration/spec.md` v4.3.0 → v4.4.0，新增 D7-S10-A32/A33/A34/A35 Requirement（12 ADDED Requirements）
- [ ] **P5.2** `openspec/specs/d7-orchestration/t-registry.md` v3.11.0 → v3.12.0，新增 D7-S10-A32-T01/T02 + A33-T03/T04 + A34-T05/T06 + A35-T07/T08（8 T 点 IMPLEMENTED）
- [ ] **P5.3** `openspec/demand-archive-index.md` 添加 DM-20260623-002 行
- [ ] **P5.4** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase4-verify-promotion/` 完整归档（5 文件 + specs/ snapshot）

---

## Phase 6: S6 归档 + Memory

- [ ] **P6.1** `./scripts/verify-archive.sh devrix-d7-mups-v4-phase4-verify-promotion` — 12/12 PASS
- [ ] **P6.2** 创建 PR（chore 类型）：`chore(openspec): S6 archive devrix-d7-mups-v4-phase4-verify-promotion (DM-20260623-002)` (PR #176)
- [ ] **P6.3** squash auto-merge 入 master
- [ ] **P6.4** 保存 Phase 4 memory entry `~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-mups-v4-phase4-verify-promotion-archived.md`
- [ ] **P6.5** 更新 MEMORY.md index（追加 1 行）

---

## 工作量总览

| PR | 文件数 | LOC | 测试 | 风险 | 时间 |
|----|--------|------|------|------|------|
| PR-D1 | 3 + 1 test | +600/-0 | 12 | Low | 1.5 天 |
| PR-D2 | 4 + 1 test | +800/-50 | 14 | Medium | 2 天 |
| PR-D3 | 3 + 1 test | +500/-0 | 10 | Low | 1 天 |
| PR-D4 | 3 + 1 test | +700/-30 | 12 | Medium | 1.5 天 |
| **总计** | **13 + 4 test** | **+2600/-80** | **48** | — | **6 天** |