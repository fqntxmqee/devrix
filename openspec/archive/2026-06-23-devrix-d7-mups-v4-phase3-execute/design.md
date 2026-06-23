# Design: D7 MUPS v4.3 Phase 3 — Execute 节点

**Change ID:** `devrix-d7-mups-v4-phase3-execute`
**Status:** S3_Design (S3 review 修正：PR-C1 与 Phase 2 SideEffectStatus 类型去重 + 删除 PlanKindToArtifactKind 函数因 plan.PlanKind 未落地)
**Date:** 2026-06-23
**Author:** MUPS v4.3 落地梳理

---

## 0. S3-Gate 4 维度自检（2026-06-23）

> **方法论**：按 `openspec/specs/project/review-design.md §2` 4 维度（数据 / 逻辑 / 边界 / 调用 / 异常 — 5 维度）+ `feedback-design-doc-review-focus.md` 用户偏好聚焦（不纠结 typo/import/symbol）。

### 0.1 数据（Data Model）
- ✅ Artifact 4 类枚举（ArtifactKind uint8 + String 双向 + ParseArtifactKind 反向）
- ✅ SideEffectStatus 5 态（SideEffectNone/InFlight/Confirmed/Compensated/Unknown，**与 Phase 2 string alias 复用不重定义**）
- ✅ Artifact 字段升级 5 个：Kind / SourcePlanID / AnomaliesCount / SideEffectStatus / SideEffectDetail
- ✅ 全部 omitempty 向后兼容（v2 调用方零修改）
- ⚠ **数据耦合未解**：`SourcePlanID string` 反向追溯到 `orchtypes.Plan`（既有），但 `AnomaliesCount` 字段来源未明（需要 Phase 2 PR-B1 落地 `plan.Plan.AnomaliesCount`），PR-C1 落地时由调用方在创建 Artifact 时显式填入 0 即可

### 0.2 逻辑（Algorithm / State Machine）
- ✅ ArtifactKind 4 类 → 4 Channel 一一对应（PR-C1 静态映射，PR-C2 动态路由）
- ✅ SideEffectStatus 状态机：None / Unknown / InFlight → Committed / RolledBack 5 态
- ✅ `IsTerminal()` / `NeedsAttention()` 派生方法减少调用方重复判断
- ⚠ **状态机收口未到 7 PR**：`DetermineStatus(toolResult, timeout)` 函数在 PR-C1 不引入（依赖 ToolResult 抽象，PR-C2 才用到），仅 SideEffectStatus 5 态枚举

### 0.3 边界（Edge Cases）
- ✅ ArtifactKind 未知值 → `ParseArtifactKind` 返回 `ErrArtifactKindInvalid`（与 Phase 2 错误码模式一致）
- ✅ SideEffectStatus 既有 string alias 直接复用（无 unknown 状态在 wire format 上需要处理）
- ✅ `Artifact` 升级字段 omitempty（v2 调用方零修改）
- ⚠ **空 Artifact 边界**：`Artifact{Evidence: nil}` 是否合法？本 PR-C1 维持 `Evidence map[string]any`（与 v2 既有签名一致），PR-C5 升级为结构化时统一处理 nil

### 0.4 调用（API Contract）
- ✅ `Artifact` 升级字段对外暴露（导出）
- ✅ `ParseArtifactKind` / `IsTerminal` / `NeedsAttention` 派生方法
- ⚠ **跨域 wire format 兼容**：`SideEffectStatus` 复用 Phase 2 string alias 后，与 `UncertaintyCoord.SideEffectStatus` 字段类型一致（同一 package 同一定义），D5 dashboard `side_effect_status` 过滤可统一

### 0.5 异常（Error Handling）
- ✅ `ErrArtifactKindInvalid` sentinel error（待 S4 落地时加进 `errors.go`）
- ⚠ **`ErrSideEffectStatusInvalid` 不引入**：SideEffectStatus 是 string alias，未知值走空字符串（与 Phase 2 `uncertainty_coord.go` 一致）
- ✅ `side_effect_status_test.go` 6 个测试覆盖：5 态基本 + `IsTerminal` 派生 + `NeedsAttention` 派生 + 空字符串

### 0.6 S3-Gate 综合决议

| 维度 | 评分 | 备注 |
|------|------|------|
| 数据 | A | 类型去重 + 向后兼容，关键决策清晰 |
| 逻辑 | A- | 状态机 5 态清晰；状态机收口等 PR-C2 |
| 边界 | A- | wire format 兼容 + nil 边界有预案 |
| 调用 | A | 派生方法收敛 + 跨域类型统一 |
| 异常 | A | sentinel 错误齐 + wire format 友好 |
| **总评** | **A-** | **S3-Gate ✅ Approved，可进入 S4 实现 PR-C1** |

### 0.7 修正记录
1. **2026-06-23 R1** 删除 `PlanKindToArtifactKind()` 函数（plan.PlanKind 未落地）
2. **2026-06-23 R1** `SideEffectStatus` 复用 Phase 2 string alias 不重定义
3. **2026-06-23 R1** Artifact struct 升级字段全部 omitempty 向后兼容

---

## 1. 范围与架构位置

### 1.1 D7 域内位置

```
D7 编排层（internal/layers/orchestration/）
├── execute/                ⭐ 本 change 新建：Executor + 4 Channel + StrategyDecider + RetryPolicy
├── orchtypes/              ⭐ 本 change 扩展：ArtifactKind + SideEffectStatus + SideEffectDetail
├── wavescheduler/          ⚠ 微调：Artifact 升级 4 类 + SideEffect 字段 + Evidence 结构化
├── sessionorchestrator/    ⚠ 微调：ProcessMessage wiring + VerifyTrigger
├── plan/                   不变（Phase 2 落地，本 change 消费 Plan）
├── observe/                不变（Phase 2 落地）
├── workmodel/              不变（Phase 1 复用 UncertaintyCoord）
├── toolrunner/surface/     ⭐ 本 change 扩展：ToolSpec v3 10 字段
└── turn/                   ⚠ 微调：ExitReason 扩到 12 枚举（含 ExitReasonStrategyLLMDecided）
```

### 1.2 跨域依赖

| 跨域 | 接口 | 本 change 用法 |
|---|---|---|
| D3 LLM 网关 | `LLMCompleter.CompleteWithOptions(...)` | StrategyDecider Layer 1 |
| D4 多智能体 | `FreeForkSurface` | ExplorationChannel 可选使用 |
| D5 可观测性 | `d7_execute_channel_p95_ms` histogram × 4 | 4 类 Channel 各 1 指标 |
| D7-S1 WorkItem | `WorkItem` | ChannelRegistry 路由参考 |
| D7-S3 WaveScheduler | `WaveScheduler.SubmitTasks` | ScenarioChannel 内部调用 |

### 1.3 落地顺序与 PR 解耦

> **S3 review 修正（2026-06-23）**：原 design 把 Phase 3 视为一个 7 PR 联动的整体 change。
> 实际 PR-C2..C7 强依赖 `plan.PlanKind`（Phase 2 PR-B1 落地），目前 workmodel 仅有
> `orchtypes.Plan{ID, SessionID, Tasks, TaskSpec}`，**无 PlanKind 枚举 / FailureCriteria / BlastRadius / SourceObservationIDs**。
> 因此实际落地顺序：

| PR | 依赖 | 可独立落地？ | 说明 |
|---|---|---|---|
| **PR-C1 Artifact 升级** | 仅 orchtypes/wavescheduler 内部 | **✅ 独立** | 本次 S1-S6 完整流程走 PR-C1 |
| PR-C2 4 Channel | `plan.PlanKind` | ❌ | 等 Phase 2 PR-B1 后再开 |
| PR-C3 StrategyDecider + RetryPolicy | `plan.AnomaliesCount` | ⚠ 半独立 | L0 硬规则可独立；L1 LLM 决策需 Plan |
| PR-C4 ToolSpec v3 | toolrunner/surface（既有） | ✅ 独立 | 可单 PR 推进 |
| PR-C5 ExecutionEvidence | orchtypes（既有） | ✅ 独立 | 可单 PR 推进 |
| PR-C6 VerifyTrigger wiring | PR-C1 + PR-C5 | ⚠ 串联 | 需 PR-C1 + PR-C5 先行 |
| PR-C7 Executor + DispatchWorker v2 | PR-C1..C6 | ❌ | 终点 PR，依赖前 6 个 |

> **本次 S4 仅落地 PR-C1**（最小风险入口），后续 PR-C2..C7 各自开独立 OpenSpec change。
> PR-C1 + Phase 2 PR-B1 落地后再串联 PR-C2/C7。

---

## 2. Artifact 升级（PR-C1）

**修改文件**：`internal/layers/orchestration/orchtypes/artifact_kind.go`（NEW）+ `side_effect_status.go`（NEW）+ `wavescheduler/artifact.go`（MODIFY）

> **S3 review 修正（2026-06-23）**：
> 1. `SideEffectStatus` 与 Phase 2 `uncertainty_coord.go:14` 既有 string 类型**重名**。本 PR-C1 在 `orchtypes/side_effect_status.go` 复用既有 string alias 定义（避免重复类型），并补齐 `SideEffectNone` 5th 状态（既有 4 态：Unknown/Inflight/Committed/RolledBack）。
> 2. `PlanKind → ArtifactKind` 路由依赖 `plan.PlanKind`（Phase 2 PR-B1 落地），**目前 workmodel 仅有 `orchtypes.Plan{ID, SessionID, Tasks, TaskSpec}`，无 PlanKind 枚举**。本 PR-C1 删除 `PlanKindToArtifactKind()` 函数（需要时由 PR-C2 在 `plan.Plan` 落地后补），Artifact 创建时调用方显式传 `ArtifactKind`。
> 3. `wavescheduler/artifact.go` 升级为新增字段 `SourcePlanID string` + `AnomaliesCount int` + `SideEffectStatus` + `SideEffectDetail`，向后兼容（v2 调用方零修改，omitempty 兜底）。

```go
// artifact_kind.go (NEW)
package orchtypes

import "fmt"

type ArtifactKind uint8

const (
    ArtifactStateChangeCert ArtifactKind = iota  // commit channel 产出
    ArtifactResponseRecord                        // protocol channel 产出
    ArtifactProbeReport                           // scenario channel 产出
    ArtifactExperimentData                        // exploration channel 产出
)

func (k ArtifactKind) String() string {
    switch k {
    case ArtifactStateChangeCert:
        return "state_change_cert"
    case ArtifactResponseRecord:
        return "response_record"
    case ArtifactProbeReport:
        return "probe_report"
    case ArtifactExperimentData:
        return "experiment_data"
    default:
        return fmt.Sprintf("ArtifactKind(%d)", uint8(k))
    }
}

// ParseArtifactKind 反向解析（用于 JSON wire format）
func ParseArtifactKind(s string) (ArtifactKind, error) {
    switch s {
    case "state_change_cert":
        return ArtifactStateChangeCert, nil
    case "response_record":
        return ArtifactResponseRecord, nil
    case "probe_report":
        return ArtifactProbeReport, nil
    case "experiment_data":
        return ArtifactExperimentData, nil
    default:
        return 0, fmt.Errorf("orchtypes: unknown ArtifactKind %q: %w", s, ErrArtifactKindInvalid)
    }
}
```

```go
// side_effect_status.go (NEW)
// 复用 Phase 2 既有 string alias 风格，避免与 uncertainty_coord.go 重复定义。
// 既有 4 态: SideEffectUnknown / SideEffectInflight / SideEffectCommitted / SideEffectRolledBack
// 本 PR-C1 补齐 SideEffectNone 5th 状态（无副作用，与 Inflight 区别）。
package orchtypes

type SideEffectStatus string  // 复用 uncertainty_coord.go 同款 string alias

const (
    SideEffectNone        SideEffectStatus = "none"            // ⭐新增（无副作用）
    SideEffectUnknown     SideEffectStatus = "unknown"          // 既有
    SideEffectInflight    SideEffectStatus = "inflight"         // 既有
    SideEffectCommitted   SideEffectStatus = "committed"        // 既有
    SideEffectRolledBack  SideEffectStatus = "rolled_back"      // 既有
)

func (s SideEffectStatus) IsTerminal() bool {
    return s == SideEffectNone || s == SideEffectCommitted || s == SideEffectRolledBack
}

func (s SideEffectStatus) NeedsAttention() bool {
    return s == SideEffectUnknown || s == SideEffectInflight
}

type SideEffectDetail struct {
    IdempotencyKey   string `json:"idempotency_key"`
    SentAt           int64  `json:"sent_at"`                       // unix nano（精简为 int64）
    ConfirmedAt      int64  `json:"confirmed_at,omitempty"`
    CompensationLog  string `json:"compensation_log,omitempty"`
    CompensationTool string `json:"compensation_tool,omitempty"`
}
```

**Artifact struct 升级**（`wavescheduler/artifact.go` MODIFY）：
```go
type Artifact struct {
    ID               string                  `json:"id"`
    TaskID           string                  `json:"task_id"`
    Kind             orchtypes.ArtifactKind  `json:"kind,omitempty"`           // ⭐新增 omitempty
    SourcePlanID     string                  `json:"source_plan_id,omitempty"` // ⭐新增
    AnomaliesCount   int                     `json:"anomalies_count,omitempty"`// ⭐新增（透传 Plan）
    SideEffectStatus orchtypes.SideEffectStatus  `json:"side_effect_status,omitempty"` // ⭐新增
    SideEffectDetail *orchtypes.SideEffectDetail `json:"side_effect_detail,omitempty"`
    Evidence         map[string]any           `json:"evidence,omitempty"`      // 维持 any（PR-C5 升级为结构化）
    CreatedAt        time.Time               `json:"created_at"`
}
```

---

## 3. 4 类 Channel（PR-C2）

**新建目录**：`internal/layers/orchestration/execute/`

### 3.1 Channel interface

```go
package execute

import (
    "context"
    "devrix/internal/layers/orchestration/plan"
    "devrix/internal/layers/orchestration/orchtypes"
)

type Channel interface {
    Name() string
    Supports(planKind plan.PlanKind) bool
    Execute(ctx context.Context, plan *plan.Plan, req ChannelRequest) (*orchtypes.Artifact, error)
}

type ChannelRequest struct {
    SessionID      string
    PriorVerdicts  []orchtypes.VerdictKind  // 历史 Verdict（用于信誉决策）
}

type ChannelRegistry struct {
    channels map[plan.PlanKind]Channel
}

func NewChannelRegistry() *ChannelRegistry {
    return &ChannelRegistry{
        channels: make(map[plan.PlanKind]Channel),
    }
}

func (r *ChannelRegistry) Register(c Channel) {
    for _, k := range r.allKinds() {
        if c.Supports(k) {
            r.channels[k] = c
        }
    }
}

func (r *ChannelRegistry) Get(pk plan.PlanKind) (Channel, error) {
    if c, ok := r.channels[pk]; ok {
        return c, nil
    }
    return nil, fmt.Errorf("%w: planKind=%s", ErrChannelNotFound, pk)
}

func (r *ChannelRegistry) allKinds() []plan.PlanKind {
    return []plan.PlanKind{
        plan.CommitmentPlan,
        plan.ProtocolPlan,
        plan.ScenarioPlan,
        plan.ExplorationPlan,
    }
}
```

### 3.2 CommitChannel

```go
type CommitChannel struct {
    ToolSurface toolrunner.Surface
    Executor    *DefaultExecutor
    Timeout     time.Duration  // 默认 200ms
}

func (c *CommitChannel) Name() string { return "commit" }

func (c *CommitChannel) Supports(pk plan.PlanKind) bool {
    return pk == plan.CommitmentPlan
}

func (c *CommitChannel) Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*orchtypes.Artifact, error) {
    ctx, cancel := context.WithTimeout(ctx, c.Timeout)
    defer cancel()

    if len(p.Steps) != 1 {
        return nil, fmt.Errorf("%w: commit requires 1 step, got %d", ErrPlanStepCountMismatch, len(p.Steps))
    }

    step := p.Steps[0]
    result, err := c.Executor.InvokeTool(ctx, step.ToolName, step.Parameters, "", &step)
    if err != nil {
        return nil, fmt.Errorf("commit_channel: invoke: %w", err)
    }

    evidence := orchtypes.ExecutionEvidence{}
    evidence.AddInvocation(orchtypes.ToolInvocation{
        ToolName:       step.ToolName,
        Args:           step.Parameters,
        IdempotencyKey: step.IdempotencyKey,
        ExitCode:       result.ExitCode,
        StartedAt:      result.StartedAt,
        CompletedAt:    result.CompletedAt,
        RetryCount:     result.RetryCount,
    })

    sideEffect := orchtypes.DetermineStatus(result, ctx.Err() == context.DeadlineExceeded)

    return &orchtypes.Artifact{
        ID:               uuid.New().String(),
        TaskID:           req.SessionID,
        Kind:             orchtypes.ArtifactStateChangeCert,
        SourcePlanID:     p.ID,
        AnomaliesCount:   p.AnomaliesCount,
        SideEffectStatus: sideEffect,
        SideEffectDetail: buildSideEffectDetail(result, step.IdempotencyKey),
        Evidence:         evidence,
        CreatedAt:        time.Now(),
    }, nil
}
```

### 3.3 ProtocolChannel

```go
type ProtocolChannel struct {
    ToolSurface toolrunner.Surface
    Executor    *DefaultExecutor
    Timeout     time.Duration  // 默认 5s
}

func (c *ProtocolChannel) Name() string { return "protocol" }

func (c *ProtocolChannel) Supports(pk plan.PlanKind) bool {
    return pk == plan.ProtocolPlan
}

func (c *ProtocolChannel) Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*orchtypes.Artifact, error) {
    ctx, cancel := context.WithTimeout(ctx, c.Timeout)
    defer cancel()

    evidence := orchtypes.ExecutionEvidence{}
    executedSteps := []plan.PlanStep{}

    for _, step := range p.Steps {
        result, err := c.Executor.InvokeTool(ctx, step.ToolName, step.Parameters, "", &step)
        evidence.AddInvocation(orchtypes.ToolInvocation{
            ToolName: step.ToolName, Args: step.Parameters,
            IdempotencyKey: step.IdempotencyKey, ExitCode: result.ExitCode,
            StartedAt: result.StartedAt, CompletedAt: result.CompletedAt,
        })
        if err != nil {
            // 失败回滚：reverse 已执行的 Step[]
            c.rollback(ctx, executedSteps)
            return nil, fmt.Errorf("protocol_channel: step %d failed: %w", step.Index, err)
        }
        executedSteps = append(executedSteps, step)
    }

    return &orchtypes.Artifact{
        ID:             uuid.New().String(),
        TaskID:         req.SessionID,
        Kind:           orchtypes.ArtifactResponseRecord,
        SourcePlanID:   p.ID,
        AnomaliesCount: p.AnomaliesCount,
        Evidence:       evidence,
        CreatedAt:      time.Now(),
    }, nil
}

func (c *ProtocolChannel) rollback(ctx context.Context, executed []plan.PlanStep) {
    // 倒序执行补偿
    for i := len(executed) - 1; i >= 0; i-- {
        step := executed[i]
        spec := c.ToolSurface.GetSpec(step.ToolName)
        if spec.IsCompensable && spec.CompensationTool != "" {
            compensationArgs := spec.GetCompensationArgs(step.Parameters)
            _, _ = c.Executor.InvokeTool(ctx, spec.CompensationTool, compensationArgs, "", &step)
        }
    }
}
```

### 3.4 ScenarioChannel

```go
type ScenarioChannel struct {
    ToolSurface toolrunner.Surface
    Executor    *DefaultExecutor
    MaxParallel int           // 默认 5
    Timeout     time.Duration // 默认 10s
}

func (c *ScenarioChannel) Supports(pk plan.PlanKind) bool {
    return pk == plan.ScenarioPlan
}

func (c *ScenarioChannel) Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*orchtypes.Artifact, error) {
    ctx, cancel := context.WithTimeout(ctx, c.Timeout)
    defer cancel()

    sem := make(chan struct{}, c.MaxParallel)
    var wg sync.WaitGroup
    results := make([]*orchtypes.ToolResult, len(p.Steps))
    errors := make([]error, len(p.Steps))

    for i, step := range p.Steps {
        wg.Add(1)
        sem <- struct{}{}
        go func(idx int, s plan.PlanStep) {
            defer wg.Done()
            defer func() { <-sem }()
            r, err := c.Executor.InvokeTool(ctx, s.ToolName, s.Parameters, "", &s)
            results[idx] = r
            errors[idx] = err
        }(i, step)
    }
    wg.Wait()

    // Majority vote 收敛
    successCount := 0
    for _, err := range errors {
        if err == nil {
            successCount++
        }
    }
    if successCount < len(p.Steps)/2 {
        return nil, fmt.Errorf("scenario_channel: majority failed (%d/%d)", successCount, len(p.Steps))
    }

    evidence := orchtypes.ExecutionEvidence{}
    for i, step := range p.Steps {
        evidence.AddInvocation(orchtypes.ToolInvocation{
            ToolName: step.ToolName, ExitCode: results[i].ExitCode,
            StartedAt: results[i].StartedAt, CompletedAt: results[i].CompletedAt,
        })
    }

    return &orchtypes.Artifact{
        ID:             uuid.New().String(),
        Kind:           orchtypes.ArtifactProbeReport,
        SourcePlanID:   p.ID,
        AnomaliesCount: p.AnomaliesCount,
        Evidence:       evidence,
        CreatedAt:      time.Now(),
    }, nil
}
```

### 3.5 ExplorationChannel

```go
type ExplorationChannel struct {
    ToolSurface     toolrunner.Surface
    FreeForkSurface *toolrunner.FreeForkSurface  // 可选
    Executor        *DefaultExecutor
    MaxParallel     int           // 默认 3
    Timeout         time.Duration // 默认 30s
}

func (c *ExplorationChannel) Supports(pk plan.PlanKind) bool {
    return pk == plan.ExplorationPlan
}

func (c *ExplorationChannel) Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*orchtypes.Artifact, error) {
    // 多 agent 并行探索 + 优先级排序
    // 复用 ScenarioChannel 的并行模式 + 加 FreeFork 可选
    ...
}
```

---

## 4. StrategyDecider + RetryPolicy（PR-C3）

### 4.1 StrategyDecider

```go
package execute

type Strategy int

const (
    StrategyContinue       Strategy = iota
    StrategyAskAtRoundEnd
    StrategyAskNow
    StrategyAskAndRollback
)

func (s Strategy) String() string {
    switch s {
    case StrategyContinue:
        return "continue"
    case StrategyAskAtRoundEnd:
        return "ask_at_round_end"
    case StrategyAskNow:
        return "ask_now"
    case StrategyAskAndRollback:
        return "ask_and_rollback"
    default:
        return fmt.Sprintf("Strategy(%d)", int(s))
    }
}

type DecideRequest struct {
    FailureCount     int
    ToolName         string
    LastError        error
    SideEffectStatus orchtypes.SideEffectStatus
    UserAvailable    bool
    RoundID          string
}

type StrategyDecider interface {
    Decide(ctx context.Context, req DecideRequest) (Strategy, error)
}

type DefaultStrategyDecider struct {
    LLMCompleter decisionplanning.LLMCompleter
    TimeoutMs    int  // 默认 2000
}

func (d *DefaultStrategyDecider) Decide(ctx context.Context, req DecideRequest) (Strategy, error) {
    // 1. 优先 Layer 0 硬规则
    if strategy := d.Layer0Decide(req); strategy != StrategyContinue {
        return strategy, nil
    }
    // 2. Layer 1 LLM 决策
    if d.LLMCompleter == nil {
        return StrategyContinue, nil  // LLM 未注入兜底
    }
    return d.Layer1Decide(ctx, req)
}

func (d *DefaultStrategyDecider) Layer0Decide(req DecideRequest) Strategy {
    if req.SideEffectStatus == orchtypes.SideEffectInFlight {
        return StrategyAskNow  // ⭐ 副作用未确认必须立即询问
    }
    if req.FailureCount >= 3 {
        return StrategyAskAtRoundEnd
    }
    if req.FailureCount >= 5 {
        return StrategyAskAndRollback
    }
    return StrategyContinue
}

func (d *DefaultStrategyDecider) Layer1Decide(ctx context.Context, req DecideRequest) (Strategy, error) {
    ctx, cancel := context.WithTimeout(ctx, time.Duration(d.TimeoutMs)*time.Millisecond)
    defer cancel()

    prompt := d.buildLLMPrompt(req)
    result, err := d.LLMCompleter.CompleteWithOptions(ctx, decisionplanning.CompleteOptions{
        SystemPrompt:     "你是 StrategyDecider... **不要调用任何 tool**",
        UserPrompt:       prompt,
        AllowedTools:     nil,  // ⭐ 避免递归
        CriticalReminder: "STRATEGY MUST BE ONE OF: continue | ask_at_round_end | ask_now | ask_and_rollback",
    })
    if err != nil {
        return StrategyContinue, nil  // LLM 失败兜底 Layer 0 决策
    }
    strategy, parseErr := parseStrategy(result)
    if parseErr != nil {
        slog.Warn("strategy_decider.parse_failed", "error", parseErr, "fallback", "Layer0")
        return d.Layer0Decide(req), nil
    }
    return strategy, nil
}
```

### 4.2 RetryPolicy

```go
package execute

type RetryPolicy struct {
    MaxRetries        int    `json:"max_retries"`         // 默认 3
    BackoffStrategy   string `json:"backoff_strategy"`    // "exponential" | "linear" | "fixed"
    InitialDelayMs    int    `json:"initial_delay_ms"`    // 默认 100
    MaxDelayMs        int    `json:"max_delay_ms"`        // 默认 5000
    UseIdempotencyKey bool   `json:"use_idempotency_key"` // ⭐
}

func (r *RetryPolicy) ComputeDelay(attempt int) time.Duration {
    if attempt <= 0 {
        return 0
    }
    switch r.BackoffStrategy {
    case "exponential":
        delay := r.InitialDelayMs * (1 << (attempt - 1))
        if delay > r.MaxDelayMs {
            delay = r.MaxDelayMs
        }
        return time.Duration(delay) * time.Millisecond
    case "linear":
        delay := r.InitialDelayMs * attempt
        if delay > r.MaxDelayMs {
            delay = r.MaxDelayMs
        }
        return time.Duration(delay) * time.Millisecond
    default:  // "fixed"
        return time.Duration(r.InitialDelayMs) * time.Millisecond
    }
}

func (r *RetryPolicy) ShouldRetry(attempt int, err error) bool {
    if attempt >= r.MaxRetries {
        return false
    }
    if err == nil {
        return false
    }
    // ⭐ 只有 Idempotent 或 IdempotencyKey 的 tool 才能重试
    return true  // Phase 3 简化：调用方负责 IsRetryable 检查
}
```

---

## 5. ToolSpec v3（PR-C4）

**修改文件**：`internal/layers/orchestration/toolrunner/surface/spec.go`

```go
package toolrunner

type ToolSpec struct {
    // 已有 4 字段（Phase 1）
    Name        string
    Description string
    Parameters  string  // JSON Schema
    RiskLevel   types.RiskLevel

    // ⭐ ToolCapability profile（新增 4 字段）
    IsAsync       bool
    IsIdempotent  bool
    IsRetryable   bool
    IsCompensable bool

    // ⭐ 补偿契约（新增 3 字段）
    CompensationTool      string
    CompensationArgs      string  // JSON Schema
    CompensationTimeoutMs int

    // ⭐ 重试/资源元数据（新增 3 字段）
    MaxRetries      int
    BackoffStrategy string
    TimeoutMs       int

    // ⭐ 降级路径（新增 2 字段）
    FallbackTool string
    FallbackArgs string
}

func (s *ToolSpec) IsSideEffect() bool {
    return s.IsCompensable  // ⭐ 副作用工具必须可补偿
}

func (s *ToolSpec) GetCompensationArgs(originalArgs string) string {
    // 解析 CompensationArgs Schema，从 originalArgs 派生补偿操作 args
    if s.CompensationArgs == "" {
        return originalArgs  // 兜底：原样使用
    }
    var schema, original map[string]any
    json.Unmarshal([]byte(s.CompensationArgs), &schema)
    json.Unmarshal([]byte(originalArgs), &original)
    // 简化版：直接 schema["args"] 作为补偿参数
    if compensation, ok := schema["args"].(map[string]any); ok {
        for k := range compensation {
            if v, ok := original[k]; ok {
                compensation[k] = v
            }
        }
        result, _ := json.Marshal(compensation)
        return string(result)
    }
    return originalArgs
}
```

**YAML 加载器**：YAML 中每个 tool 配置 10 字段，`spec_loader.go` 解析后填入 `ToolSpec`。

---

## 6. ExecutionEvidence（PR-C5）

**新建文件**：`internal/layers/orchestration/execute/execution_evidence.go`

```go
package execute

import "time"

type ExecutionEvidence struct {
    ToolInvocations []ToolInvocation `json:"tool_invocations"`
    Logs            []LogEntry       `json:"logs"`
    Metrics         MetricSnapshot   `json:"metrics"`
}

type ToolInvocation struct {
    ToolName       string    `json:"tool_name"`
    Args           string    `json:"args"`
    IdempotencyKey string    `json:"idempotency_key"`
    ExitCode       int       `json:"exit_code"`
    Stdout         string    `json:"stdout,omitempty"`
    Stderr         string    `json:"stderr,omitempty"`
    StartedAt      time.Time `json:"started_at"`
    CompletedAt    time.Time `json:"completed_at"`
    RetryCount     int       `json:"retry_count"`
}

type LogEntry struct {
    Level   string         `json:"level"`
    Message string         `json:"message"`
    Time    time.Time      `json:"time"`
    Fields  map[string]any `json:"fields,omitempty"`
}

type MetricSnapshot struct {
    DurationMs    int `json:"duration_ms"`
    TokensUsed    int `json:"tokens_used"`
    EstimatedCost int `json:"estimated_cost_usd"`
    MemoryPeakMB  int `json:"memory_peak_mb"`
}

func (e *ExecutionEvidence) AddInvocation(inv ToolInvocation) {
    e.ToolInvocations = append(e.ToolInvocations, inv)
}

func (e *ExecutionEvidence) GetExitCode(toolName string) (int, bool) {
    for _, inv := range e.ToolInvocations {
        if inv.ToolName == toolName {
            return inv.ExitCode, true
        }
    }
    return -1, false
}
```

---

## 7. VerifyTrigger wiring（PR-C6）

**修改文件**：`internal/layers/orchestration/sessionorchestrator/orchestrator.go`

```go
// 在 WaveTaskNode 完成时调 Verifier
func (o *Orchestrator) onWaveTaskComplete(ctx context.Context, taskID string, artifact *orchtypes.Artifact) {
    defer func() {
        if r := recover(); r != nil {
            slog.Error("verify_trigger.panic", "task_id", taskID, "panic", r)
        }
    }()

    plan, err := o.planStore.Get(artifact.SourcePlanID)
    if err != nil {
        slog.Warn("verify_trigger.plan_lookup_failed", "plan_id", artifact.SourcePlanID, "error", err)
        return
    }

    verdict, err := o.verifier.Verify(ctx, artifact, plan)
    if err != nil {
        slog.Warn("verify_trigger.failed", "task_id", taskID, "error", err)
        return  // ⭐ 不阻塞 Artifact 提交
    }
    o.emitVerdict(taskID, verdict)
}
```

---

## 8. Executor interface + DispatchWorker v2（PR-C7）

```go
package execute

type Executor interface {
    Execute(ctx context.Context, plan *plan.Plan, sessionID string) (*orchtypes.Artifact, error)
    InvokeTool(ctx context.Context, toolName string, input string, workDir string, step *plan.PlanStep) (*ToolResult, error)
}

type DefaultExecutor struct {
    Channels         *ChannelRegistry
    ToolSurface      toolrunner.Surface
    StrategyDecider  StrategyDecider
    IdempotencyCache IdempotencyCache
    FilterChain      []toolrunner.ToolFilter
    ArtifactBuilder  *ArtifactBuilder
}

func (e *DefaultExecutor) Execute(ctx context.Context, p *plan.Plan, sessionID string) (*orchtypes.Artifact, error) {
    channel, err := e.Channels.Get(p.Kind)
    if err != nil {
        return nil, err
    }

    artifact, err := channel.Execute(ctx, p, ChannelRequest{SessionID: sessionID})
    if err != nil {
        // StrategyDecider 决策失败处理
        strategy, _ := e.StrategyDecider.Decide(ctx, DecideRequest{
            FailureCount:     1,
            ToolName:         p.Steps[0].ToolName,
            LastError:        err,
            SideEffectStatus: artifact.SideEffectStatus,
        })
        slog.Warn("execute.strategy", "plan_id", p.ID, "strategy", strategy, "error", err)
        // MVP: 不自动 rollback（Phase 4 升格后处理）
    }
    return artifact, nil
}

func (e *DefaultExecutor) InvokeTool(ctx context.Context, toolName string, input string, workDir string, step *plan.PlanStep) (*ToolResult, error) {
    // ⭐ IdempotencyKey 必填当 SideEffect
    spec := e.ToolSurface.GetSpec(toolName)
    if spec == nil {
        return nil, fmt.Errorf("%w: %s", ErrToolNotFound, toolName)
    }
    if step.IdempotencyKey == "" && spec.IsSideEffect() {
        return nil, ErrIdempotencyKeyRequired
    }

    // 1. IdempotencyCache 命中检查
    if step.IdempotencyKey != "" {
        if cached, ok := e.IdempotencyCache.Get(step.IdempotencyKey); ok {
            cached.Cached = true
            return cached, nil
        }
    }

    // 2. Filter 链
    specs := e.ToolSurface.Tools(ctx, "", "")
    specs = e.applyFilterChain(specs, toolName)
    if !containsToolName(specs, toolName) {
        return nil, fmt.Errorf("%w: %s", ErrToolPermissionDenied, toolName)
    }

    // 3. 实际执行
    result, err := e.ToolSurface.Execute(ctx, toolName, input, workDir)
    if err != nil {
        return nil, err
    }

    // 4. 写 IdempotencyCache
    if step.IdempotencyKey != "" {
        e.IdempotencyCache.Set(step.IdempotencyKey, result)
    }
    return result, nil
}
```

**9 个 SentinelError**：
```go
var (
    ErrArtifactIncomplete     = errors.New("execute: artifact payload validation failed")
    ErrPlanStepCountMismatch  = errors.New("execute: plan step count mismatch channel")
    ErrBlastRadiusExceeded    = errors.New("execute: blast radius exceeded during execution")
    ErrRetryExhausted         = errors.New("execute: retry exhausted")
    ErrCircuitOpen            = errors.New("execute: circuit breaker open")
    ErrChannelNotFound        = errors.New("execute: no channel for plan kind")
    ErrToolNotFound           = errors.New("execute: tool not found in surface")
    ErrToolPermissionDenied   = errors.New("execute: tool permission denied")
    ErrIdempotencyKeyRequired = errors.New("execute: idempotency key required for side-effect tool (⭐EP-2 衍生)")
)
```

---

## 9. 配置

**修改文件**：`internal/layers/orchestration/orchtypes/config.go`

```go
type Config struct {
    // ... 已有配置 ...
    Execute ExecuteConfig `yaml:"execute"`
}

type ExecuteConfig struct {
    Retry RetryConfig `yaml:"retry"`
    CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
    Channels ChannelConfigs `yaml:"channels"`
}

type RetryConfig struct {
    DefaultMaxRetries   int    `yaml:"default_max_retries"`
    DefaultBackoff      string `yaml:"default_backoff"`
    DefaultInitialDelayMs int  `yaml:"default_initial_delay_ms"`
    DefaultMaxDelayMs   int    `yaml:"default_max_delay_ms"`
}

type CircuitBreakerConfig struct {
    Threshold     int `yaml:"threshold"`
    OpenTimeoutMs int `yaml:"open_timeout_ms"`
    HalfOpenMax   int `yaml:"half_open_max"`
}

type ChannelConfigs struct {
    Commit      ChannelConfig `yaml:"commit"`
    Protocol    ChannelConfig `yaml:"protocol"`
    Scenario    ScenarioConfig `yaml:"scenario"`
    Exploration ExplorationConfig `yaml:"exploration"`
}

type ChannelConfig struct {
    TimeoutMs int `yaml:"timeout_ms"`
}

type ScenarioConfig struct {
    MaxParallel int `yaml:"max_parallel"`
    TimeoutMs   int `yaml:"timeout_ms"`
}

type ExplorationConfig struct {
    MaxParallel      int  `yaml:"max_parallel"`
    FreeForkOptional bool `yaml:"free_fork_optional"`
    TimeoutMs        int  `yaml:"timeout_ms"`
}
```

---

## 10. 度量指标

新增 Prometheus 指标：

| 指标 | 类型 | Labels |
|------|------|--------|
| `d7_execute_channel_p95_ms` | Histogram | `channel`, `plan_kind` |
| `d7_execute_strategy_decisions_total` | Counter | `strategy`, `layer` |
| `d7_execute_idempotency_cache_hits_total` | Counter | `tool_name` |
| `d7_execute_idempotency_cache_misses_total` | Counter | `tool_name` |
| `d7_execute_idempotency_key_required_errors_total` | Counter | `tool_name` |
| `d7_execute_tool_side_effect_status_total` | Counter | `status` |
| `d7_execute_retry_total` | Counter | `tool_name`, `outcome` |
| `d7_execute_circuit_breaker_state` | Gauge | `state` |
| `d7_execute_compensation_executions_total` | Counter | `tool_name`, `outcome` |

---

## 11. 测试矩阵

| 文件 | 测试数 | 覆盖 |
|------|-------|------|
| `orchtypes/artifact_kind_test.go` | 4 | AC1 |
| `orchtypes/side_effect_status_test.go` | 6 | AC2 |
| `wavescheduler/artifact_test.go`（扩展） | 5 | AC3 |
| `execute/channel_test.go` | 8 | AC4 |
| `execute/channel_commit_test.go` | 3 | AC4 |
| `execute/channel_protocol_test.go` | 3 | AC4 |
| `execute/channel_scenario_test.go` | 3 | AC4 |
| `execute/channel_exploration_test.go` | 3 | AC4 |
| `execute/executor_test.go` | 5 | AC5, AC9 |
| `execute/strategy_decider_test.go` | 6 | AC6, AC19 |
| `execute/retry_policy_test.go` | 4 | AC7 |
| `toolrunner/surface/spec_test.go`（扩展） | 10 | AC8, AC20, AC21 |
| `execute/execution_evidence_test.go` | 5 | AC10 |
| `sessionorchestrator/orchestrator_test.go`（扩展） | 3 | AC11 |
| `turn/exit_reason_test.go`（扩展） | 2 | AC13 |
| 集成测试 5 套 | — | AC15, AC17, AC18 |
| 性能测试 | — | AC12 |
| **合计** | **~75** | — |

---

## 12. 风险与缓解（详见 proposal §6）

| 风险 | 缓解 |
|---|---|
| 4 类 Channel 与 WaveScheduler 重复 | Channel 内部调用 WaveScheduler（如 ScenarioChannel） |
| StrategyDecider LLM 超时 | Layer 0 硬规则兜底 + PerRoundTimeoutMs=2000 |
| VerifyTrigger 失败不阻塞 | slog.Warn + 不 return error |

---

## 13. Cross-references

- 设计稿：[[../../../brain/01知识探索/项目/20260620-certain-architecture/project-application/44-d7-execute-node-design|doc 44 Execute 节点]]
- Phase 1：DM-20260623-001 UncertaintyCoord scaffold
- Phase 2：DM-20260624-001（候选）Observe + Plan — Plan 是本 Phase 输入
- Phase 4：DM-20260626-001（候选）Verify 升格 — Verifier 消费 Artifact
- Phase 5：DM-20260627-001（候选）Learn — 消费 Verdict → Artifact → Plan