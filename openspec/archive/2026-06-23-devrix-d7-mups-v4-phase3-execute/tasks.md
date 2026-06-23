# Tasks: D7 MUPS v4.3 Phase 3 — Execute 节点落地

**Change ID:** `devrix-d7-mups-v4-phase3-execute`
**Demand ID:** DM-20260625-001
**Status:** S3_Design (S1 demand.md 2026-06-23 已落；S2 proposal.md 完整；S3 design.md 完整；待 S3-Gate + S4 PR-C1)
**Date:** 2026-06-23
**Author:** MUPS v4.3 落地梳理

---

## Phase 0: Setup

- [ ] 创建 change 目录 `openspec/changes/devrix-d7-mups-v4-phase3-execute/`
- [ ] `proposal.md`（S2，已完成）
- [ ] `tasks.md`（S4，本文）
- [ ] `design.md`（S3，详细 Go 代码 + 跨域契约）
- [ ] `specs/d7-orchestration/spec_delta.md`（Gherkin 验收，17 个 ADDED Requirement）
- [ ] `specs/d7-orchestration/t-registry_delta.md`（21 个 P0 T 点）
- [ ] `specs/tool-surface/spec_delta.md`（ToolSpec v3 扩展 Gherkin，4 个 ADDED Requirement）
- [ ] `specs/tool-surface/t-registry_delta.md`（新增 4 个 P0 T 点）
- [ ] 分支 `feat/devrix-d7-mups-v4-phase3-execute master`
- [ ] Phase 2 archive 已存在（前置依赖）

---

## PR-C1: Artifact 升级 4 类 + SideEffect 字段

> **S3 review 修正（2026-06-23）**：
> - SideEffectStatus 5 态**复用 Phase 2 既有 string alias**（uncertainty_coord.go），不重定义；新增 `SideEffectNone` 作为 5th 状态，命名对齐 Phase 2（SideEffectInflight / SideEffectCommitted / SideEffectRolledBack）
> - 删除 `DetermineStatus()` 函数（依赖 ToolResult 抽象，PR-C2 引入）
> - 删除 `TestArtifactKind_PlanKindMapping`（plan.PlanKind 未落地，ArtifactKind 由调用方显式传）

### C1.1 ArtifactKind 4 类枚举

- [ ] **C1.1.1** 新建文件 `internal/layers/orchestration/orchtypes/artifact_kind.go`
  - 定义 `ArtifactKind` uint8 enum（ArtifactStateChangeCert / ArtifactResponseRecord / ArtifactProbeReport / ArtifactExperimentData）
  - 实现 `String()` 双向 + `ParseArtifactKind(s string) (ArtifactKind, error)` 反向解析
  - 实现 `MarshalJSON` / `UnmarshalJSON`（使用 String wire format）
- [ ] **C1.1.2** 测试 `orchtypes/artifact_kind_test.go`
  - `TestArtifactKind_4Types_String` — 4 枚举值 String() 正确
  - `TestArtifactKind_4Types_ParseRoundTrip` — Marshal/Unmarshal 双向
  - `TestArtifactKind_UnknownValue_ParseError` — 未知字符串返回 ErrArtifactKindInvalid
  - `TestArtifactKind_JSON_WireFormat` — JSON wire format 是字符串（"state_change_cert" 等）而非数字

### C1.2 SideEffectStatus 5 态

- [ ] **C1.2.1** 新建文件 `internal/layers/orchestration/orchtypes/side_effect_status.go`
  - **复用 Phase 2 string alias** `type SideEffectStatus string`（与 uncertainty_coord.go:14 一致）
  - 5 态：`SideEffectNone = "none"`（新增）+ `SideEffectUnknown = "unknown"`（既有）+ `SideEffectInflight = "inflight"`（既有）+ `SideEffectCommitted = "committed"`（既有）+ `SideEffectRolledBack = "rolled_back"`（既有）
  - 定义 `SideEffectDetail` struct（IdempotencyKey/SentAt/ConfirmedAt/CompensationLog/CompensationTool），**time.Time 改为 int64 unix nano**（与 UncertaintyCoord 风格一致）
  - 实现派生方法 `IsTerminal() bool` / `NeedsAttention() bool`
- [ ] **C1.2.2** 测试 `orchtypes/side_effect_status_test.go`
  - `TestSideEffectStatus_5States_String` — 5 态 String() 正确
  - `TestSideEffectStatus_5States_RoundTrip` — Marshal/Unmarshal 双向
  - `TestSideEffectStatus_IsTerminal` — None / Committed / RolledBack → true
  - `TestSideEffectStatus_NeedsAttention` — Unknown / Inflight → true
  - `TestSideEffectStatus_ReusesUncertaintyCoordType` — 同一 type alias（编译期保证）

### C1.3 Artifact struct 扩展

- [ ] **C1.3.1** 修改 `internal/layers/orchestration/wavescheduler/artifact.go:Artifact`
  - 增加 5 字段：`Kind` (omitempty) / `SourcePlanID` (omitempty) / `AnomaliesCount` / `SideEffectStatus` / `SideEffectDetail` (omitempty)
  - `Evidence` 字段从 string 升级到 `ExecutionEvidence`（PR-C5 落地）
- [ ] **C1.3.2** 测试 `wavescheduler/artifact_test.go`（扩展）
  - `TestArtifact_Kind_OmitEmpty_BackwardCompatible`
  - `TestArtifact_SourcePlanID_Required`
  - `TestArtifact_AnomaliesCount_FromPlan`
  - `TestArtifact_SideEffectStatus_Default_None`
  - `TestArtifact_SideEffectDetail_RequiredWhen_NotNone`

---

## PR-C2: 4 类执行通道

### C2.1 Channel interface

- [ ] **C2.1.1** 新建文件 `internal/layers/orchestration/execute/channel.go`
  - 定义 `Channel` interface（Name/Supports/Execute）
  - 定义 `ChannelRequest` struct（SessionID/PriorVerdicts）
  - 定义 `ChannelRegistry` struct（map[PlanKind]Channel）
  - 实现 `ChannelRegistry.Register(channel Channel)`
  - 实现 `ChannelRegistry.Get(planKind PlanKind) (Channel, error)`（找不到返回 `ErrChannelNotFound`）

### C2.2 CommitChannel

- [ ] **C2.2.1** 新建文件 `internal/layers/orchestration/execute/channel_commit.go`
  - 定义 `CommitChannel` struct（ToolSurface/Executor）
  - 实现 `Supports(planKind) bool` → 仅 CommitmentPlan
  - 实现 `Execute(ctx, plan, req)` → 1 Step 直接 ToolSurface.Invoke → Artifact{ArtifactKind: StateChangeCert}
- [ ] **C2.2.2** 测试 `execute/channel_commit_test.go`
  - `TestCommitChannel_CommitmentPlan_OK`
  - `TestCommitChannel_OtherPlan_NotSupported`
  - `TestCommitChannel_SingleStep_ProducesStateChangeCert`

### C2.3 ProtocolChannel

- [ ] **C2.3.1** 新建文件 `internal/layers/orchestration/execute/channel_protocol.go`
  - 实现多 Step 顺序执行 + 失败回滚（reverse Step[]）
- [ ] **C2.3.2** 测试
  - `TestProtocolChannel_SequentialSteps`
  - `TestProtocolChannel_Step2_Failed_RollbackStep1`
  - `TestProtocolChannel_AllStepsSuccess_ResponseRecord`

### C2.4 ScenarioChannel

- [ ] **C2.4.1** 新建文件 `internal/layers/orchestration/execute/channel_scenario.go`
  - 并行试探 + max_parallel=5 + majority vote 收敛
- [ ] **C2.4.2** 测试
  - `TestScenarioChannel_5ParallelProbes`
  - `TestScenarioChannel_MajorityVote_ProbeReport`
  - `TestScenarioChannel_MixedResults_TakesMajority`

### C2.5 ExplorationChannel

- [ ] **C2.5.1** 新建文件 `internal/layers/orchestration/execute/channel_exploration.go`
  - 多 agent 并行 + FreeForkSurface 可选 + 优先级排序
- [ ] **C2.5.2** 测试
  - `TestExplorationChannel_MultiAgent_Parallel`
  - `TestExplorationChannel_FreeFork_Optional`
  - `TestExplorationChannel_PriorityOrder_ExperimentData`

### C2.6 集成测试

- [ ] **C2.6.1** 新建 `tests/integration/d7/execute_channels_test.go`
  - E2E: Plan → Executor.Execute → Channel → Artifact
  - 4 类 PlanKind 各跑一次

---

## PR-C3: StrategyDecider (MVP L0+L1) + RetryPolicy

### C3.1 StrategyDecider

- [ ] **C3.1.1** 新建文件 `internal/layers/orchestration/execute/strategy_decider.go`
  - 定义 `Strategy` enum（Continue/AskAtRoundEnd/AskNow/AskAndRollback）
  - 定义 `StrategyDecider` interface（Decide）
  - 定义 `DefaultStrategyDecider` struct（LLMCompleter/Layer0Rules）
  - 实现 `Decide(ctx, req)` 优先 Layer 0 兜底，Layer 1 LLM 决策
  - 实现 `Layer0Decide(req) Strategy`（不动 LLM）
- [ ] **C3.1.2** system prompt 强约束："你是 StrategyDecider... **不要调用任何 tool**"
- [ ] **C3.1.3** 测试 `execute/strategy_decider_test.go`
  - `TestStrategyDecider_Layer0_InFlight_AskNow`
  - `TestStrategyDecider_Layer0_3Failures_AskAtRoundEnd`
  - `TestStrategyDecider_Layer1_LLM_CriticalReminderEnforced`
  - `TestStrategyDecider_Layer1_NoToolAccess`（⭐ AC19）
  - `TestStrategyDecider_Layer1_InvalidStrategy_FallbackLayer0`
  - `TestStrategyDecider_LLMCompleter_NilSafe`

### C3.2 RetryPolicy 升级

- [ ] **C3.2.1** 新建文件 `internal/layers/orchestration/execute/retry_policy.go`
  - 定义 `RetryPolicy` struct（MaxRetries/BackoffStrategy/InitialDelayMs/MaxDelayMs/UseIdempotencyKey）
  - 实现 `ComputeDelay(attempt int) time.Duration`（exponential/linear/fixed）
  - 实现 `ShouldRetry(attempt int, err error) bool`
- [ ] **C3.2.2** 测试
  - `TestRetryPolicy_Exponential_Backoff`
  - `TestRetryPolicy_Linear_Backoff`
  - `TestRetryPolicy_Fixed_Backoff`
  - `TestRetryPolicy_IdempotencyKey_RequiredForSideEffect`

### C3.3 ExitReason 扩到 12 枚举（与 Phase 4 衔接）

- [ ] **C3.3.1** 修改 `internal/layers/orchestration/turn/orchestrator.go:ExitReason`
  - 增加 `ExitReasonStrategyLLMDecided = "strategy_llm_decided"`（第 12 枚举）
- [ ] **C3.3.2** 测试 `turn/exit_reason_test.go`（扩展）
  - `TestExitReason_12Enums_AllDistinct`
  - `TestExitReason_StrategyLLMDecided_Phase4Handshake`（Phase 4 衔接点）

---

## PR-C4: ToolSpec v3 扩展（10 字段）

### C4.1 ToolSpec 字段扩展

- [ ] **C4.1.1** 修改 `internal/layers/orchestration/toolrunner/surface/spec.go:ToolSpec`
  - 增加 6 字段：IsAsync/IsIdempotent/IsRetryable/IsCompensable/CompensationTool/CompensationArgs/CompensationTimeoutMs
  - 增加 3 字段：MaxRetries/BackoffStrategy/TimeoutMs
  - 增加 2 字段：FallbackTool/FallbackArgs
- [ ] **C4.1.2** 实现 `ToolSpec.IsSideEffect() bool`（基于 IsCompensable 推导）
- [ ] **C4.1.3** 实现 `ToolSpec.GetCompensationArgs(originalArgs string) string`（解析 CompensationArgs Schema 派生）

### C4.2 ToolSpec YAML 加载

- [ ] **C4.2.1** 修改 `internal/layers/orchestration/toolrunner/surface/spec_loader.go`
  - YAML 解析支持新字段
- [ ] **C4.2.2** 测试 `toolrunner/surface/spec_test.go`
  - `TestToolSpec_10Fields_AllDefaults` — 默认值正确
  - `TestToolSpec_IsAsync_True` — BackgroundTask 必备
  - `TestToolSpec_IsIdempotent_GET_HTTP`
  - `TestToolSpec_CompensationTool_HTTP_Delete`
  - `TestToolSpec_FallbackTool_Alternative`
  - `TestToolSpec_CompensationArgs_DeriveFromOriginal`

### C4.3 YAML 配置（4 类 Channel）

- [ ] **C4.3.1** 修改 `devrix.yaml` + `internal/layers/orchestration/orchtypes/config.go`
  - 增加 `ExecuteConfig` struct（Retry/CircuitBreaker/Channels 4 类）
- [ ] **C4.3.2** 测试 `orchtypes/config_test.go`（扩展）
  - `TestExecuteConfig_4Channels_YAMLParsed`

---

## PR-C5: ExecutionEvidence 机器可解析

### C5.1 ExecutionEvidence struct

- [ ] **C5.1.1** 新建文件 `internal/layers/orchestration/execute/execution_evidence.go`
  - 定义 `ExecutionEvidence` struct（ToolInvocations/Logs/Metrics）
  - 定义 `ToolInvocation` struct（含 toolName/args/IdempotencyKey/exitCode/stdout/stderr/startedAt/completedAt/retryCount）
  - 定义 `LogEntry` struct（level/message/time/fields）
  - 定义 `MetricSnapshot` struct（durationMs/tokensUsed/estimatedCost/memoryPeakMB）
- [ ] **C5.1.2** 实现 `ExecutionEvidence.AddInvocation(inv ToolInvocation)` 不可变
- [ ] **C5.1.3** 实现 `ExecutionEvidence.GetExitCode(toolName string) (int, bool)`（按 toolName 查 exit code）

### C5.2 Artifact.Evidence 升级

- [ ] **C5.2.1** 修改 `wavescheduler/artifact.go:Artifact.Evidence` 字段
  - 从 string 改为 ExecutionEvidence（保持 omitempty 向后兼容）
  - 增加 helper 方法 `func (a *Artifact) AddEvidence(ev ExecutionEvidence)`
- [ ] **C5.2.2** 测试
  - `TestArtifact_Evidence_String_BackwardCompatible` — 旧 string 仍可解析
  - `TestArtifact_Evidence_ExecutionEvidence_Struct`
  - `TestArtifact_Evidence_GetExitCode`

### C5.3 单元测试

- [ ] **C5.3.1** 新建 `execute/execution_evidence_test.go`
  - `TestExecutionEvidence_AddInvocation_Immutable`
  - `TestExecutionEvidence_GetExitCode_ByToolName`
  - `TestExecutionEvidence_Logs_Ordered`
  - `TestExecutionEvidence_Metrics_Aggregated`
  - `TestExecutionEvidence_ToolInvocation_Duration_Calculated`

---

## PR-C6: VerifyTrigger wiring（Phase 3 + Phase 4 衔接）

### C6.1 WaveTaskNode 完成 hook

- [ ] **C6.1.1** 修改 `internal/layers/orchestration/sessionorchestrator/orchestrator.go`
  - 增加 `onWaveTaskComplete(ctx, taskID, artifact)` 方法
  - 在 WaveTaskNode 完成时调 `verifier.Verify(artifact, plan)`（Phase 4 落地 Verifier 升格）
  - 捕获 error，emit `slog.Warn("verify_trigger.failed", ...)`，**不阻塞 Artifact 提交**
- [ ] **C6.1.2** 实现 Plan 反向追溯：`planStore.Get(artifact.SourcePlanID)`
- [ ] **C6.1.3** 测试 `sessionorchestrator/orchestrator_test.go`（扩展）
  - `TestOrchestrator_OnWaveTaskComplete_TriggersVerify`
  - `TestOrchestrator_OnWaveTaskComplete_VerifyError_NonBlocking`
  - `TestOrchestrator_OnWaveTaskComplete_PlanLookup`

### C6.2 Phase 3 + Phase 4 集成测试

- [ ] **C6.2.1** 新建 `tests/integration/d7/execute_to_verify_test.go`
  - E2E: Plan → Execute → Artifact → Verify（Phase 4 stub verifier 调用）→ ExitReason

---

## PR-C7: Executor interface + DispatchWorker v2

### C7.1 Executor interface

- [ ] **C7.1.1** 新建文件 `internal/layers/orchestration/execute/executor.go`
  - 定义 `Executor` interface（Execute/InvokeTool）
  - 定义 `DefaultExecutor` struct（Channels/ToolSurface/StrategyDecider/IdempotencyCache/FilterChain）
  - 实现 `Execute(ctx, plan, sessionID)` 走 ChannelRegistry 路由
  - 实现 `InvokeTool(ctx, toolName, input, workDir, step)` 必经 ToolSurface + IdempotencyKey 校验
- [ ] **C7.1.2** 实现 9 个 SentinelError
- [ ] **C7.1.3** 测试 `execute/executor_test.go`
  - `TestExecutor_Execute_PlanKind_Routing`
  - `TestExecutor_InvokeTool_EP2_SingleEntryPoint`
  - `TestExecutor_InvokeTool_IdempotencyKey_Required_ForSideEffect`
  - `TestExecutor_InvokeTool_IdempotencyCache_Hit`
  - `TestExecutor_InvokeTool_FilterChain_Blocks`

### C7.2 DispatchWorker v2

- [ ] **C7.2.1** 新建文件 `internal/layers/orchestration/execute/dispatch_worker_v2.go`
  - 定义 `DispatchWorkerV2` struct（Executor/StrategyDecider）
  - 实现 `Run(ctx, plan)` 调 Executor.Execute
  - 实现 `HandleFailure(ctx, artifact, err)` 调 StrategyDecider
- [ ] **C7.2.2** 测试 `execute/dispatch_worker_v2_test.go`
  - `TestDispatchWorkerV2_Run_Success`
  - `TestDispatchWorkerV2_HandleFailure_StrategyContinue`
  - `TestDispatchWorkerV2_HandleFailure_StrategyAskNow_SideEffectInFlight`

### C7.3 9 个 SentinelError

- [ ] **C7.3.1** 在 `execute/errors.go` 定义
  - `ErrArtifactIncomplete`
  - `ErrPlanStepCountMismatch`
  - `ErrBlastRadiusExceeded`
  - `ErrRetryExhausted`
  - `ErrCircuitOpen`
  - `ErrChannelNotFound`
  - `ErrToolNotFound`
  - `ErrToolPermissionDenied`
  - `ErrIdempotencyKeyRequired`（⭐ EP-2 衍生）
- [ ] **C7.3.2** 测试每个 SentinelError 触发路径

---

## PR 全量集成测试

- [ ] **INT-1** `tests/integration/d7/execute_pipeline_test.go`
  - E2E: Plan → Executor.Execute → Channel → Artifact → ExecutionEvidence
- [ ] **INT-2** `tests/integration/d7/execute_invoke_tool_test.go`
  - E2E: Plan.Step → Executor.InvokeTool → ToolSurface → IdempotencyCache
- [ ] **INT-3** `tests/integration/d7/side_effect_inflight_test.go`
  - SideEffect 状态判定失败 → Artifact.SideEffectStatus=InFlight → Phase 4 强制 VerdictIndeterminate
- [ ] **INT-4** `tests/integration/d7/verify_trigger_test.go`
  - WaveTaskNode 完成 → Verifier.Verify stub 调用
- [ ] **INT-5** `tests/integration/d7/plan_to_artifact_test.go`
  - 4 类 PlanKind → 4 类 ArtifactKind 一一对应

---

## S6 Archive

- [ ] PR-C1 → C2 → C3 → C4 → C5 → C6 → C7 顺序合入 master（squash auto-merge）
- [ ] 归档到 `openspec/archive/2026-06-26-devrix-d7-mups-v4-phase3-execute/`
- [ ] `scripts/verify-archive.sh` 全部通过
- [ ] 更新 `openspec/specs/d7-orchestration/spec.md` + `t-registry.md` + `openspec/specs/tool-surface/spec.md` + `t-registry.md`（PLANNED → IMPLEMENTED）
- [ ] 关闭 Change（DM-20260625-001 状态：✅ S7_Archived）