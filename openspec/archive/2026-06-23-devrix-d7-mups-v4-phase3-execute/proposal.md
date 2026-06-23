# Proposal: D7 MUPS v4.3 Phase 3 — Execute 节点落地

**Change ID:** `devrix-d7-mups-v4-phase3-execute`
**Demand ID:** DM-20260625-001
**Status:** S7_Archived (S1 demand.md 2026-06-23 已落；S2 proposal.md 完整；S3 design.md 完整；S3-Gate review-design.md 4 维度自检 A- 评价；S4 PR-C1 实现完成 + auto-merge PR #164 2026-06-23；S5 acceptance-report.md ACCEPTED 4/4 P0 AC + 19/19 internal 包 -race PASS + layer-lint PASS；S6 archive 2026-06-23 完成)
**Priority:** P0
**Date:** 2026-06-23
**Author:** MUPS v4.3 落地梳理 (doc 44 §Execute + doc 47 Phase 3)

---

## 1. Background

Phase 1 已落地 UncertaintyCoord scaffold（DM-20260623-001）；Phase 2 已落地 Observe + Plan 节点（DM-20260624-001 候选）。**Phase 3 是 5 Phase 落地的第三步**，把 [[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/44-d7-execute-node-design|doc 44 Execute 节点]] 设计稿同步为 OpenSpec 五件套（含 d7-orchestration + tool-surface 两个 spec delta）。

### 1.1 Phase 2 已落地的契约基础（Phase 3 直接消费）

| Phase 2 资产 | Phase 3 用法 |
|------------|------------|
| `plan.Plan` 含 4 类 PlanKind + Steps + FailureCriteria + BlastRadius + SourceObservationIDs + AnomaliesCount | Executor.Execute(Plan, sessionID) 输入 |
| `plan.PlanStep` 含 ToolName + Parameters + IdempotencyKey | Execute 调用 ToolSurface 时的关键参数 |
| `plan.BlastRadius` | Channel 选择 + 并发数决策 |
| `AnomaliesCount` | Verdict 反向追溯 Phase 5 Learn 路径（Phase 3 只透传）|

### 1.2 与已有 change 的关系

| 已有 change | 关系 |
|------------|------|
| `devrix-d7-mups-v4-phase1-foundation` (DM-20260623-001) | 直接前置：UncertaintyCoord scaffold |
| `devrix-d7-mups-v4-phase2-observe-plan` (DM-20260624-001 候选) | **直接前置**：Plan 类型 + 3 项强制约束 |
| `devrix-tool-surface-contract` (DM-20260617-007) | **直接前置**：ToolSurface 4 方法接口 + ToolFilter 链 |
| `devrix-tool-surface-phase2-full` (DM-20260617-008) | 直接前置：12→0 global loop 闭环 |
| `devrix-tool-surface-3changes` (DM-20260618-001/002/003) | 直接前置：ToolSpec v2 + CheckPermission + DeferLoading |
| `devrix-d7-v2-structure` (DM-20260619-005) | 间接前置：v2.0 物理路径 |
| `devrix-d7-metrics-and-concurrency-hardening` (DM-20260622-001) | 间接前置：D5 Span 可复用 |
| `devrix-d7-mups-v4-phase4-verify-promotion` (DM-20260626-001 候选) | **被前置**：Phase 4 Verifier 消费 Artifact |
| `devrix-d7-mups-v4-phase5-learn` (DM-20260627-001 候选) | 间接被前置：Learn 消费 Verdict → SourceArtifactID → Plan |

### 1.3 与 tech-debt 的关系

- 本 change 闭合 Phase 1/2 留下的 tech-debt：`DecisionPlanning.Decomposer → []Task` 与 `plan.Plan.Steps` 共存期（Phase 3 仍保留，Phase 4 后退役 Decomposer）
- 本 change 不引入新 tech-debt

---

## 2. Problem Statement

### Problem 1 (HIGH): Execute 节点无统一抽象，4 类执行通道散落

**位置**：
- `internal/layers/orchestration/wavescheduler/scheduler.go` 有 WaveScheduler 但只支持单通道（并行 max=5）
- `internal/layers/orchestration/sessionorchestrator/dispatch.go` 是 DispatchWorker 现有实现，零散耦合
- **没有 PlanKind → 4 类通道的路由**：CommitPlan/ProtocolPlan/ScenarioPlan/ExplorationPlan 都走同一条 WaveScheduler 路径

**根因**：v2.0 结构重构时只切分物理路径，未抽象 Execute 节点 + 4 类通道。

**影响**：
- Execute 无法做 PlanKind 决策（commit/protocol/scenario/exploration 应走不同策略）
- Tool 调用契约无统一入口，分散在 wavescheduler + sessionorchestrator + delegatetools
- 失败重试与降级策略只能 hardcode 在 WaveScheduler，无法 per-Plan 配置

**修法**：新建 `execute/` 目录 + 4 类 Channel（commit/protocol/scenario/exploration）+ Channel interface；`Executor` interface 作为唯一入口；Channel 选择由 PlanKind 路由。

### Problem 2 (HIGH): Artifact 数据契约缺失，反向追溯链断裂

**位置**：
- `internal/layers/orchestration/wavescheduler/artifact.go:Artifact` 当前只有 1 类（`Kind` 字段缺失）
- 无 `SideEffectStatus`（5 态）+ `SideEffectDetail`（IdempotencyKey/SentAt/ConfirmedAt/CompensationLog）
- 无 `SourcePlanID`（Verify 阶段反向追溯 Plan 必需）

**根因**：Phase 1/2 落地时只把 Artifact 当成 WaveScheduler 内部数据结构，未对外暴露契约。

**影响**：
- Phase 4 Verifier 无法根据 Artifact 决策 Verdict（不知道是哪个 Plan 触发的）
- Phase 5 Learn 无法累积 Plan.AnomaliesCount 到 ReputationEvidence
- SideEffect 状态（in flight / confirmed / compensated）无法跨节点传播

**修法**：扩展 `Artifact` struct（ArtifactKind 4 类 enum + SideEffectStatus 5 态 + SideEffectDetail + SourcePlanID + AnomaliesCount），保持向后兼容旧 Artifact（Kind 字段 `omitempty`）。

### Problem 3 (HIGH): ToolSpec 字段太瘦（4 字段），Execute/Verify/Learn 决策缺元数据

**位置**：
- `internal/layers/orchestration/toolrunner/surface/spec.go:ToolSpec` 只有 4 字段（Name/Description/Parameters/RiskLevel）
- RetryPolicy 在 YAML 全局配置，无法 per-tool 覆盖
- 无 IsAsync/IsIdempotent/IsRetryable/IsCompensable 声明式查询，靠 `isSideEffectTool(toolName)` 字符串匹配
- 无 CompensationTool/CompensationArgs，补偿操作散落在代码 if-else
- 无 FallbackTool，降级路径不显式

**根因**：DM-20260617-007/008 + DM-20260618-001/002/003 落地 ToolSpec v2 时只关注"是否可调用"，未关注"如何安全调用"。

**影响**：
- Execute RetryPolicy 无法 per-tool 定制（HTTP 类需要长 backoff，文件类可以短重试）
- SideEffect 工具失败时无自动补偿（需手写 reverse 操作）
- Plan.Step 决策 IdempotencyKey 时无 Tool 元数据支撑（Phase 2 必填校验靠运行时检测）

**修法**：扩展 `ToolSpec` 从 4 字段到 10 字段（增 IsAsync/IsIdempotent/IsRetryable/IsCompensable + 补偿契约 + 重试元数据 + 降级路径）。

### Problem 4 (MEDIUM): StrategyDecider 缺失，LLM 介入决策没有框架

**位置**：
- 当前 WaveScheduler 失败重试 + CircuitBreaker 都是 hardcode 决策树
- 无 LLM 介入策略决策（"现在该继续 / 终止 / 询问用户 / 回滚"）
- LoopFirst=false 时已实现的 4 类策略（Continue/AskAtRoundEnd/AskNow/AskAndRollback）仅停留在 doc 35 §三.4，未代码化

**根因**：MVP 阶段只关注快速落地硬规则，LLM 介入策略未实现。

**影响**：
- 复杂场景（如 tool 调用失败 + 用户已离线 + 副作用未确认）无法做智能决策
- Phase 5 Learn 信誉累积无 StrategyDecider 决策信号

**修法**：新建 `execute/strategy_decider.go`，MVP 实现 L0 硬规则 + L1 LLM 决策（4 类策略），含 `ExitReasonStrategyLLMDecided` 第 12 个枚举。

### Problem 5 (MEDIUM): ExecutionEvidence 不可机器解析，Verify 校验困难

**位置**：
- `Artifact.Evidence` 字段当前是 string/bytes，无结构化
- ToolInvocation（toolName + args + IdempotencyKey + exitCode）散落在 logs
- Phase 4 Verifier 验证 FailureCriteria 时需手 parse

**根因**：Phase 1 Artifact 设计时未考虑跨节点机器可解析。

**影响**：
- Verify 校验 FailureCriteria（Field/Op/Value 三元组）需字符串匹配
- 不能从 Evidence 自动提取字段做 Condition 评估

**修法**：扩展 `ExecutionEvidence` struct（ToolInvocation + Log + Metric 机器可解析），作为 `Artifact.Evidence` 的强类型。

### Problem 6 (LOW): VerifyTrigger wiring 缺失，Artifact 完成时不自动校验

**位置**：
- `WaveTaskNode` 完成时只写日志，无 Verifier 调用
- Phase 4 Verifier 需主动 poll 或查表

**根因**：Phase 1/2 落地时未考虑 Verify 触发的紧耦合。

**影响**：
- Verify 时效性差（poll 间隔可能丢失）
- Artifact 完成到 Verdict 出炉有时间窗口

**修法**：在 WaveTaskNode 完成时调 Verifier.Verify(Artifact, Plan)（Phase 3 落 PR-C6 wiring，Phase 4 落 Verifier 升格）。

---

## 3. Solution

### 3.1 Artifact 升级 4 类 + SideEffect 字段（PR-C1）

**修改文件**：
- `internal/layers/orchestration/orchtypes/artifact_kind.go`（NEW，~80 行）
- `internal/layers/orchestration/orchtypes/side_effect_status.go`（NEW，~150 行）
- `internal/layers/orchestration/wavescheduler/artifact.go`（MODIFY，扩展 ~50 行）

**核心数据结构**：
```go
type ArtifactKind uint8
const (
    ArtifactStateChangeCert ArtifactKind = iota  // commit channel 产出（如 git commit）
    ArtifactResponseRecord                       // protocol channel 产出（API 响应）
    ArtifactProbeReport                          // scenario channel 产出（并行试探结果）
    ArtifactExperimentData                       // exploration channel 产出（多 agent 数据）
)

type SideEffectStatus uint8
const (
    SideEffectNone        SideEffectStatus = iota  // 无副作用（pure read-only tool）
    SideEffectInFlight                              // 已发起但未确认（如 HTTP 调用超时未确认响应）
    SideEffectConfirmed                             // 已确认（HTTP 200 + DB commit）
    SideEffectCompensated                           // 已补偿（HTTP DELETE + DB rollback）
    SideEffectUnknown                               // 状态不可判定（timeout 后无法确认）
)

type SideEffectDetail struct {
    IdempotencyKey   string    `json:"idempotency_key"`
    SentAt           time.Time `json:"sent_at"`
    ConfirmedAt      time.Time `json:"confirmed_at,omitempty"`
    CompensationLog  string    `json:"compensation_log,omitempty"`  // 补偿操作记录
    CompensationTool string    `json:"compensation_tool,omitempty"` // 使用的补偿 tool
}

type Artifact struct {
    // ... Phase 1 已有字段 ...
    Kind               ArtifactKind      `json:"kind,omitempty"`  // ⭐新增，omitempty 保持向后兼容
    SourcePlanID       string            `json:"source_plan_id,omitempty"`  // ⭐新增
    AnomaliesCount     int               `json:"anomalies_count"`  // 透传 Plan.AnomaliesCount
    SideEffectStatus   SideEffectStatus  `json:"side_effect_status"`
    SideEffectDetail   *SideEffectDetail `json:"side_effect_detail,omitempty"`
    Evidence           ExecutionEvidence `json:"evidence"`  // ⭐从 string 升级到结构化
}
```

### 3.2 4 类执行通道（PR-C2）

**新建文件**：
- `internal/layers/orchestration/execute/channel.go`（~100 行）
- `internal/layers/orchestration/execute/channel_commit.go`（~200 行）
- `internal/layers/orchestration/execute/channel_protocol.go`（~200 行）
- `internal/layers/orchestration/execute/channel_scenario.go`（~200 行）
- `internal/layers/orchestration/execute/channel_exploration.go`（~250 行）

**核心接口**：
```go
type Channel interface {
    Name() string
    Supports(planKind plan.PlanKind) bool
    Execute(ctx context.Context, plan *plan.Plan, req ChannelRequest) (*Artifact, error)
}

type CommitChannel struct {
    ToolSurface  toolrunner.Surface
    Executor     *DefaultExecutor
}

func (c *CommitChannel) Supports(pk plan.PlanKind) bool {
    return pk == plan.CommitmentPlan
}

func (c *CommitChannel) Execute(ctx, plan, req) (*Artifact, error) {
    // 1 Step 直接执行
    // ToolSurface.Invoke(step)
    // 构造 Artifact{ArtifactKind: StateChangeCert}
}
```

**PlanKind → Channel 路由**（在 `Executor.Execute()` 内）：
- `CommitmentPlan` → `CommitChannel`
- `ProtocolPlan` → `ProtocolChannel`
- `ScenarioPlan` → `ScenarioChannel`
- `ExplorationPlan` → `ExplorationChannel`

### 3.3 StrategyDecider (MVP L0+L1) + RetryPolicy（PR-C3）

**新建文件**：
- `internal/layers/orchestration/execute/strategy_decider.go`（~300 行）
- `internal/layers/orchestration/execute/retry_policy.go`（~200 行）

**核心接口**：
```go
type Strategy int
const (
    StrategyContinue       Strategy = iota  // 继续（即使失败）
    StrategyAskAtRoundEnd                   // 在当前 round 结束后询问用户
    StrategyAskNow                          // 立即询问用户
    StrategyAskAndRollback                  // 询问用户 + 准备 rollback
)

type StrategyDecider interface {
    Decide(ctx context.Context, req DecideRequest) (Strategy, error)
}

type DefaultStrategyDecider struct {
    LLMCompleter  decisionplanning.LLMCompleter  // L1 LLM 决策
    Layer0Rules   []HardRule                    // L0 硬规则
}

type DecideRequest struct {
    FailureCount  int
    ToolName      string
    LastError     error
    SideEffectStatus SideEffectStatus
    UserAvailable bool  // 用户是否在线（从 presence 服务读）
    RoundID       string
}

// MVP: Layer 0 硬规则（不动 LLM）
func (d *DefaultStrategyDecider) Layer0Decide(req DecideRequest) Strategy {
    if req.SideEffectStatus == SideEffectInFlight {
        return StrategyAskNow  // 副作用未确认必须立即询问
    }
    if req.FailureCount >= 3 {
        return StrategyAskAtRoundEnd
    }
    return StrategyContinue
}

// Layer 1 LLM（SystemPrompt + CriticalReminder）
func (d *DefaultStrategyDecider) Layer1Decide(ctx, req) (Strategy, error) {
    prompt := d.buildLLMPrompt(req)
    result, err := d.LLMCompleter.CompleteWithOptions(ctx, CompleteOptions{
        SystemPrompt:      "你是 StrategyDecider。基于 FailureCount/SideEffectStatus/UserAvailable 决策 Strategy。**不要调用任何 tool**。",
        UserPrompt:        prompt,
        AllowedTools:      nil,
        CriticalReminder:  "STRATEGY MUST BE ONE OF: continue | ask_at_round_end | ask_now | ask_and_rollback",
    })
    // parse result.Strategy
}
```

**RetryPolicy 升级**（`wavescheduler/retry_policy.go`）：
```go
type RetryPolicy struct {
    MaxRetries        int           `json:"max_retries"`
    BackoffStrategy   string        `json:"backoff_strategy"`  // "exponential" | "linear" | "fixed"
    InitialDelayMs    int           `json:"initial_delay_ms"`
    MaxDelayMs        int           `json:"max_delay_ms"`
    UseIdempotencyKey bool          `json:"use_idempotency_key"`  // ⭐配合 IdempotencyKey
}
```

### 3.4 ToolSpec v3 扩展（10 字段）（PR-C4）

**修改文件**：`internal/layers/orchestration/toolrunner/surface/spec.go`

```go
type ToolSpec struct {
    // ⭐ Phase 3 已有（4 字段）
    Name        string
    Description string
    Parameters  string  // JSON Schema
    RiskLevel   types.RiskLevel

    // ⭐ Phase 3 新增：ToolCapability profile（4 字段）
    IsAsync       bool  // 是否支持异步（BackgroundTaskSurface 必备）
    IsIdempotent  bool  // 是否天然幂等（GET 类）— 与 IdempotencyKey 配合
    IsRetryable   bool  // 失败时是否可安全重试（区分 Fatal vs Transient）
    IsCompensable bool  // 副作用是否可逆（HTTP POST 可 DELETE；DB 可 rollback）

    // ⭐ 新增：补偿契约（3 字段）
    CompensationTool      string  // reverse 操作名（如 "http_delete"）
    CompensationArgs      string  // JSON Schema（如何从原 args 派生）
    CompensationTimeoutMs int     // 补偿超时

    // ⭐ 新增：重试/资源元数据（3 字段，覆盖 §6.2 YAML）
    MaxRetries      int
    BackoffStrategy string  // "exponential" | "linear" | "fixed"
    TimeoutMs       int

    // ⭐ 新增：降级路径（2 字段）
    FallbackTool string
    FallbackArgs string
}
```

**YAML 配置覆盖**（`~/.devrix/config.yaml`）：
```yaml
d7:
  execute:
    retry:
      default_max_retries: 3
      default_backoff: exponential
      default_initial_delay_ms: 100
      default_max_delay_ms: 5000
    circuit_breaker:
      threshold: 5
      open_timeout_ms: 30000
      half_open_max: 3
    channels:
      commit: { timeout_ms: 200 }
      protocol: { timeout_ms: 5000 }
      scenario: { max_parallel: 5, timeout_ms: 10000 }
      exploration: { max_parallel: 3, free_fork_optional: true, timeout_ms: 30000 }
```

### 3.5 ExecutionEvidence 机器可解析（PR-C5）

**新建文件**：
- `internal/layers/orchestration/execute/execution_evidence.go`（~200 行）
- `internal/layers/orchestration/wavescheduler/artifact.go`（MODIFY）

**核心数据结构**：
```go
type ExecutionEvidence struct {
    ToolInvocations []ToolInvocation `json:"tool_invocations"`
    Logs            []LogEntry       `json:"logs"`
    Metrics         MetricSnapshot   `json:"metrics"`
}

type ToolInvocation struct {
    ToolName       string          `json:"tool_name"`
    Args           string          `json:"args"`             // JSON
    IdempotencyKey string          `json:"idempotency_key"`
    ExitCode       int             `json:"exit_code"`
    Stdout         string          `json:"stdout,omitempty"`
    Stderr         string          `json:"stderr,omitempty"`
    StartedAt      time.Time       `json:"started_at"`
    CompletedAt    time.Time       `json:"completed_at"`
    RetryCount     int             `json:"retry_count"`
}

type LogEntry struct {
    Level   string    `json:"level"`  // "debug"/"info"/"warn"/"error"
    Message string    `json:"message"`
    Time    time.Time `json:"time"`
    Fields  map[string]any `json:"fields,omitempty"`
}

type MetricSnapshot struct {
    DurationMs    int  `json:"duration_ms"`
    TokensUsed    int  `json:"tokens_used"`
    EstimatedCost int  `json:"estimated_cost_usd"`  // micro-USD
    MemoryPeakMB  int  `json:"memory_peak_mb"`
}
```

### 3.6 VerifyTrigger wiring（PR-C6）

**修改文件**：`internal/layers/orchestration/sessionorchestrator/orchestrator.go`

```go
// 在 WaveTaskNode 完成时调 Verifier
func (o *Orchestrator) onWaveTaskComplete(ctx context.Context, taskID string, artifact *Artifact) {
    plan, _ := o.planStore.Get(artifact.SourcePlanID)  // 反向追溯 Plan
    verdict, err := o.verifier.Verify(ctx, artifact, plan)
    o.emitVerdict(taskID, verdict)
}
```

### 3.7 Executor interface 重构 + DispatchWorker 升级（PR-C7）

**新建文件**：
- `internal/layers/orchestration/execute/executor.go`（~250 行）
- `internal/layers/orchestration/execute/dispatch_worker_v2.go`（~250 行）

**核心接口**：
```go
type Executor interface {
    Execute(ctx context.Context, plan *plan.Plan, sessionID string) (*Artifact, error)
    InvokeTool(ctx context.Context, toolName string, input string, workDir string, step *plan.PlanStep) (*ToolResult, error)
}

type DefaultExecutor struct {
    Channels       map[plan.PlanKind]Channel  // 4 类通道
    ToolSurface    toolrunner.Surface        // EP-2 唯一入口
    StrategyDecider StrategyDecider
    IdempotencyCache IdempotencyCache
    FilterChain    []toolrunner.ToolFilter
}

func (e *DefaultExecutor) Execute(ctx, plan, sessionID) (*Artifact, error) {
    // 1. 选 Channel（按 PlanKind）
    channel, ok := e.Channels[plan.Kind]
    if !ok {
        return nil, ErrChannelNotFound
    }
    // 2. Channel.Execute(plan)
    artifact, err := channel.Execute(ctx, plan, ChannelRequest{SessionID: sessionID})
    if err != nil {
        // 3. StrategyDecider 决策
        strategy, _ := e.StrategyDecider.Decide(ctx, DecideRequest{...})
        // 4. 按 strategy 处理（continue / ask / rollback）
    }
    return artifact, nil
}

func (e *DefaultExecutor) InvokeTool(ctx, toolName, input, workDir, step) (*ToolResult, error) {
    // EP-2 唯一入口：Filter 链 → Surface.Execute → IdempotencyCache.Set
    if step.IdempotencyKey == "" && isSideEffectTool(toolName) {
        return nil, ErrIdempotencyKeyRequired
    }
    if cached, ok := e.IdempotencyCache.Get(step.IdempotencyKey); ok {
        return cached, nil
    }
    surface := e.ToolSurface  // 默认 surface
    specs := surface.Tools(ctx, "", "")
    specs = e.applyFilterChain(specs, toolName)
    if !containsToolName(specs, toolName) {
        return nil, ErrToolPermissionDenied
    }
    result, err := surface.Execute(ctx, toolName, input, workDir)
    if err != nil {
        return nil, err
    }
    e.IdempotencyCache.Set(step.IdempotencyKey, result)
    return result, nil
}
```

---

## 4. Acceptance Criteria

| AC | 描述 | 验证 |
|---|------|------|
| **AC1** | ArtifactKind 4 类 enum + 互斥 + JSON 兼容旧 Artifact（Kind 缺省时按默认处理） | 单测 `orchtypes/artifact_kind_test.go` ≥ 4 用例 |
| **AC2** | SideEffectStatus 5 态（None/InFlight/Confirmed/Compensated/Unknown） + SideEffectDetail 字段 | 单测 `orchtypes/side_effect_status_test.go` ≥ 6 用例 |
| **AC3** | Artifact.SourcePlanID 必填（反向追溯 Plan）+ AnomaliesCount 透传 Plan.AnomaliesCount | 单测 + 集成测试 |
| **AC4** | 4 类 Channel：Commit/Protocol/Scenario/Exploration + Channel.Supports(planKind) 路由 | 单测 `execute/channel_test.go` ≥ 8 用例 |
| **AC5** | Executor.Execute(plan, sessionID) 走 Channel 路由 + 返回 Artifact | 单测 `execute/executor_test.go` ≥ 5 用例 |
| **AC6** | StrategyDecider Layer 0 硬规则 + Layer 1 LLM（含 CriticalReminder）| 单测 `execute/strategy_decider_test.go` ≥ 6 用例 |
| **AC7** | RetryPolicy 5 字段 + IdempotencyKey 复用 | 单测 `execute/retry_policy_test.go` ≥ 4 用例 |
| **AC8** | ToolSpec 扩展到 10 字段（IsAsync/IsIdempotent/IsRetryable/IsCompensable + 补偿契约 + 重试 + 降级）| 单测 `toolrunner/surface/spec_test.go` ≥ 10 用例 |
| **AC9** | Executor.InvokeTool 必经 ToolSurface + IdempotencyKey 必填当 SideEffect | 单测 + 集成测试 `tests/integration/d7/execute_invoke_tool_test.go` |
| **AC10** | ExecutionEvidence 含 ToolInvocation + Log + Metric 机器可解析 | 单测 `execute/execution_evidence_test.go` ≥ 6 用例 |
| **AC11** | VerifyTrigger wiring：WaveTaskNode 完成时调 Verifier.Verify（Phase 4 衔接）| 集成测试 `tests/integration/d7/verify_trigger_test.go` |
| **AC12** | ExecuteNode P95 性能约束：CommitChannel ≤ 200ms / Protocol ≤ 5s / Scenario ≤ 10s / Exploration ≤ 30s | 性能测试 + 4 个 d7_execute_channel_p95_ms 指标 |
| **AC13** | ExitReasonStrategyLLMDecided 第 12 枚举（Phase 4 也消费此枚举）| 单测 `turn/exit_reason_test.go` |
| **AC14** | Plan.Step 必填校验：IdempotencyKey 在 SideEffect 工具时必填 | 单测 `execute/invoke_tool_validation_test.go` |
| **AC15** | Plan.Steps → Artifact 完整链路（Plan.Step → ToolInvocation → Artifact.Evidence）| 集成测试 `tests/integration/d7/execute_pipeline_test.go` |
| **AC16** | 4 类 PlanKind → 4 类 ArtifactKind 一一对应（commit→StateChangeCert 等）| 单测 `execute/plan_to_artifact_mapping_test.go` |
| **AC17** | SideEffect 状态判定失败 → Artifact.SideEffectStatus=InFlight + 强制 VerdictIndeterminate（Phase 4 衔接）| 集成测试 `tests/integration/d7/side_effect_inflight_test.go` |
| **AC18** | 全链路：Plan.Execute → Artifact → Verifier.Verify → ExitReason（Phase 3 + Phase 4 衔接）| 集成测试 `tests/integration/d7/execute_to_verify_test.go` |
| **AC19** | StrategyDecider LLM 调用 AllowedTools=nil（避免决策器调 tool 递归）| 单测 `TestStrategyDecider_NoToolAccess` |
| **AC20** | ToolSpec.FallbackTool 降级路径生效 | 单测 `TestToolSpec_FallbackPath` |
| **AC21** | ToolSpec.CompensationTool 在 SideEffectStatus=Compensated 时执行 | 单测 `TestToolSpec_Compensation_Executes` |

---

## 5. Out of Scope

| 任务 | 落点 |
|------|------|
| Verify 节点升格（AggregateVerdicts + 12 枚举语义表）| Phase 4 |
| Learn 节点（LearningAsset 4 类 + ReputationEvidence + 3 通道记忆）| Phase 5 |
| decisionplanning.Decomposer 退役（Phase 3 共存）| Phase 4+ 完成后 |
| FreeForkSurface 实际多 agent 调度（Phase 3 仅引用接口）| Phase 6+ |
| ToolSurface 本身扩展（背景任务、Verify、Delegate 等 surface）| 已由 DM-20260617-007 闭环 |
| WorktreeMode / ForkMode / DenialBudget | Phase 6+ |
| StrategyDecider LLM 决策的 prompt 调优 | Phase 6+ |

---

## 6. Risk Assessment

| 风险 | 等级 | 缓解 |
|---|---|---|
| 4 类 Channel 与现有 WaveScheduler 重复实现 | 中 | Channel 内部可调用 WaveScheduler（如 ScenarioChannel）；保留 DispatchWorker 共存；Phase 5 后逐步迁移 |
| ToolSpec v3 字段破坏现有 ToolSpec JSON 兼容 | 低 | 新增字段 omitempty；UnmarshalJSON 容错 |
| StrategyDecider LLM 调用超时影响决策时延 | 中 | Layer 0 硬规则兜底 + PerRoundTimeoutMs=2000 |
| VerifyTrigger 在 Phase 3 落地但 Phase 4 未升格时调用 stub | 低 | Phase 3 用 `verifier.Verify` 占位，Phase 4 升格；调用失败不阻塞 Artifact 提交 |
| SideEffect 状态判定失败导致 Verdict 强制 Indeterminate | 中 | G5-1 设计意图：环境受限不扣业务信誉；Phase 4 写 VerdictToExitReason 区分 |
| ExecutionEvidence 升级破坏现有 Artifact JSON | 低 | ExecutionEvidence 作为 struct（不是 string），零值时 omitempty |
| DecisionPlanning.Decomposer 与 Plan.Steps 双轨存在 | 中 | Phase 3 保留双轨；Phase 4 完成后通过 S6 归档退役 |

---

## 7. Workload Estimation

| 子任务 | 工作量 |
|---|---|
| Artifact 4 类 + SideEffect 5 态（PR-C1） | 1.5 天 |
| 4 类 Channel（PR-C2）| 3.0 天 |
| StrategyDecider + RetryPolicy（PR-C3）| 2.0 天 |
| ToolSpec v3 10 字段扩展（PR-C4）| 1.5 天 |
| ExecutionEvidence 结构化（PR-C5）| 1.0 天 |
| VerifyTrigger wiring（PR-C6）| 0.5 天 |
| Executor interface + DispatchWorker v2（PR-C7）| 2.0 天 |
| Spec + T Registry delta 撰写（d7 + tool-surface） | 1.0 天 |
| 集成测试 + 灰度 rollout | 1.5 天 |
| **合计** | **~14 天（取整 12 天）** |

---

## 8. Cross-References

- 详细设计：[[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/44-d7-execute-node-design|doc 44 Execute 节点]]（六段式 + 依赖契约 A/B/C）
- 落地全景：[[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/47-d7-mups-v4-phase2-5-openspec-landing-plan|doc 47 Phase 2-5 落地方案]]（§4 Phase 3 详解）
- 数据契约：doc 37 §2.3 Artifact + §2.5 SideEffectStatus + §3.2 ToolSpec
- Phase 1：DM-20260623-001 UncertaintyCoord scaffold
- Phase 2：DM-20260624-001（候选）Observe + Plan — Plan 是本 Phase 输入
- Phase 4：DM-20260626-001（候选）Verify 升格 — Verifier 消费 Artifact
- Phase 5：DM-20260627-001（候选）Learn — 消费 Verdict → Artifact → Plan