# Design: D7 MUPS v4.3 Phase 3 PR-C2 — Execute 4 Channel + ChannelRouter

**Change ID:** `devrix-d7-mups-v4-phase3-channels`
**Status:** S3_Design → S4_Implemented → S7_Archived
**Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 3 Execute 节点落地梳理

---

## 0. S3-Gate Review（inherited from Phase 3 execute design）

> PR-C2 复用 Phase 3 execute design §0.6 S3-Gate 5 维度自检 + §0.7 S4-Gate 4 维度自检。
> 本节仅列出本 PR 范围内的调整点。

### 0.1 架构决策审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 层归属正确 | ✅ PASS | `internal/layers/orchestration/execute/` 在 D7 核心域下 |
| 跨域类型上提 | ✅ PASS | Channel 产出 `*wavescheduler.Artifact`（PR-C1 类型）；消费 `plan.Plan`（PR-B1 类型） |
| Channel 抽象解耦 PR-C4 | ✅ PASS | 本地 `ToolRunner` interface + `ToolRequest` + `ToolResult`；PR-C4 可独立落地替换为完整 ToolSpec v3 |
| C2/W8 决议 1:1 映射 | ✅ PASS | PlanKind 4 类 → Channel 4 类 → ArtifactKind 4 类 1:1 映射；D5 dashboard 字符串过滤可统一 |

### 0.2 数据契约审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| ChannelRegistry 1:1 绑定 | ✅ PASS | 内部 `map[plan.PlanKind]Channel`；Register 重复 → ErrChannelUnsupported |
| ChannelRouter 无状态分发 | ✅ PASS | 不持有任何状态；仅消费 ChannelRegistry.Get + Channel.Execute |
| 4 Channel 各自语义 | ✅ PASS | CommitChannel 1-Step 同步 / ProtocolChannel 顺序多步 / ScenarioChannel 并行投票 / ExplorationChannel 多 agent |
| SideEffectStatus 派生正确 | ✅ PASS | Commit: 0/timeout/err → Committed/Inflight/Unknown；Protocol: 全成功/部分 → Committed/RolledBack；Scenario: None；Exploration: 派生 PersistScope |

### 0.3 接口契约审查

| 检查项 | 结论 | 说明 |
|--------|------|------|
| Channel interface 最小化 | ✅ PASS | 仅 3 方法：Name() / Supports(PlanKind) / Execute(ctx, *plan.Plan, ChannelRequest) |
| ChannelRequest 上下文 | ✅ PASS | SessionID + PriorVerdictKinds []string（typed as string 暂存，Phase 4 类型上提时收紧为 `[]types.VerdictKind`） |
| defensive checks | ✅ PASS | nil Plan → ErrChannelPlanNil；未知 Kind → ErrChannelNotFound；nil runner → ErrChannelToolRunnerNil |

## 1. Channel 抽象

### 1.1 Channel interface

```go
type Channel interface {
    Name() string
    Supports(plan.PlanKind) bool
    Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*wavescheduler.Artifact, error)
}
```

### 1.2 ChannelRegistry

```go
type ChannelRegistry struct {
    mu       sync.RWMutex
    bindings map[plan.PlanKind]Channel
}

func (r *ChannelRegistry) Register(ch Channel) error {
    // 遍历 ch.Supports(p) 找首个 true 的 PlanKind
    // 重复绑定同一 PlanKind → ErrChannelUnsupported
    // 注册后只读
}

func (r *ChannelRegistry) Get(pk plan.PlanKind) (Channel, error) {
    // 未注册 → ErrChannelNotFound
}
```

**1:1 绑定冲突检测**: 一个 Channel 可 Supports 多个 PlanKind（理论上），但每个 PlanKind 只能绑定一个 Channel。

### 1.3 ChannelRouter

```go
type ChannelRouter struct {
    registry *ChannelRegistry
}

func (r *ChannelRouter) Route(ctx context.Context, p *plan.Plan, req ChannelRequest) (*wavescheduler.Artifact, error) {
    if p == nil { return nil, fmt.Errorf("%w", ErrChannelPlanNil) }
    ch, err := r.registry.Get(p.Kind)
    if err != nil { return nil, err }  // ErrChannelNotFound
    
    if !ch.Supports(p.Kind) { return nil, NewChannelUnsupportedError(ch.Name(), p.Kind.String()) }
    return ch.Execute(ctx, p, req)
}
```

### 1.4 PlanKind ↔ ArtifactKind 1:1 映射 (C2/W8 决议)

| PlanKind | Channel | ArtifactKind |
|----------|---------|--------------|
| CommitmentPlan | CommitChannel | ArtifactStateChangeCert |
| ProtocolPlan | ProtocolChannel | ArtifactResponseRecord |
| ScenarioPlan | ScenarioChannel | ArtifactProbeReport |
| ExplorationPlan | ExplorationChannel | ArtifactExperimentData |

## 2. 4 个具体 Channel

### 2.1 CommitChannel

```go
type CommitChannelConfig struct {
    Timeout time.Duration  // default 5s
}

type CommitChannel struct {
    runner ToolRunner
    cfg    CommitChannelConfig
}

func (c *CommitChannel) Execute(ctx, p, req) (*wavescheduler.Artifact, error) {
    if len(p.Steps) != 1 { return nil, NewChannelStepCountMismatchError("commit", len(p.Steps), 1, 1) }
    step := p.Steps[0]
    if step.IdempotencyKey == "" { return nil, fmt.Errorf("%w: commit step requires IdempotencyKey", ErrChannelStepCountMismatch) }
    
    ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
    defer cancel()
    
    result, err := c.runner.Invoke(ctx, ToolRequest{SessionID: req.SessionID, ToolName: step.ToolName, Args: step.ToolArgs, IdempotencyKey: step.IdempotencyKey, StepID: step.ID})
    art := &wavescheduler.Artifact{TaskID: step.ID, SessionID: req.SessionID, SourcePlanID: p.ID, AnomaliesCount: p.AnomaliesCount, Kind: types.ArtifactStateChangeCert, ...}
    
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded || errors.Is(err, context.DeadlineExceeded) {
            art.SideEffectStatus = types.SideEffectInflight
            art.SideEffectDetail = &types.SideEffectDetail{IdempotencyKey: step.IdempotencyKey, SentAt: result.StartedAt.UnixNano()}
            return art, fmt.Errorf("%w: commit tool call timed out", ErrChannelStepCountMismatch)
        }
        art.SideEffectStatus = types.SideEffectUnknown
        return art, err
    }
    
    art.Summary = result.Output
    art.ExitCode = result.ExitCode
    art.Duration = result.CompletedAt.Sub(result.StartedAt)
    art.SideEffectStatus = types.SideEffectCommitted
    art.SideEffectDetail = &types.SideEffectDetail{IdempotencyKey: step.IdempotencyKey, SentAt: result.StartedAt.UnixNano(), ConfirmedAt: result.CompletedAt.UnixNano()}
    return art, nil
}
```

### 2.2 ProtocolChannel

```go
type ProtocolChannel struct {
    runner ToolRunner
}

func (c *ProtocolChannel) Execute(ctx, p, req) (*wavescheduler.Artifact, error) {
    if len(p.Steps) == 0 { return nil, ErrChannelStepCountMismatch }
    
    var executedSteps []plan.Step
    art := &wavescheduler.Artifact{Kind: types.ArtifactResponseRecord, SourcePlanID: p.ID, ...}
    
    for i, step := range p.Steps {
        result, err := c.runner.Invoke(ctx, ToolRequest{...})
        if err != nil {
            // reverse-order rollback
            for j := len(executedSteps) - 1; j >= 0; j-- {
                prev := executedSteps[j]
                rollbackArgs := map[string]any{"__rollback": true}
                for k, v := range prev.ToolArgs { rollbackArgs[k] = v }
                _, _ = c.runner.Invoke(ctx, ToolRequest{ToolName: prev.ToolName, Args: rollbackArgs, IdempotencyKey: prev.IdempotencyKey + ":rollback", StepID: prev.ID})
            }
            art.SideEffectStatus = types.SideEffectRolledBack
            return art, fmt.Errorf("step %d failed: %w", i, err)
        }
        executedSteps = append(executedSteps, step)
        art.Summary = result.Output
    }
    
    art.SideEffectStatus = types.SideEffectCommitted
    return art, nil
}
```

### 2.3 ScenarioChannel

```go
const MaxParallelProbes = 5

func (c *ScenarioChannel) Execute(ctx, p, req) (*wavescheduler.Artifact, error) {
    if len(p.Steps) == 0 { return nil, ErrChannelStepCountMismatch }
    
    sem := make(chan struct{}, MaxParallelProbes)
    var wg sync.WaitGroup
    results := make([]bool, len(p.Steps))
    
    for i, step := range p.Steps {
        wg.Add(1)
        sem <- struct{}{}
        go func(i int, step plan.Step) {
            defer wg.Done()
            defer func() { <-sem }()
            _, err := c.runner.Invoke(ctx, ToolRequest{...})
            results[i] = err == nil
        }(i, step)
    }
    wg.Wait()
    
    successCount := 0
    for _, r := range results { if r { successCount++ } }
    
    art := &wavescheduler.Artifact{Kind: types.ArtifactProbeReport, SourcePlanID: p.ID, SideEffectStatus: types.SideEffectNone, ...}
    
    if successCount > len(p.Steps)/2 {
        art.ExitCode = 0
        return art, nil
    }
    return art, NewChannelStepCountMismatchError("scenario", successCount, len(p.Steps)/2+1, len(p.Steps))
}
```

### 2.4 ExplorationChannel

```go
const MaxParallelAgents = 3

func (c *ExplorationChannel) Execute(ctx, p, req) (*wavescheduler.Artifact, error) {
    sem := make(chan struct{}, MaxParallelAgents)
    var wg sync.WaitGroup
    type result struct{ idx int; step plan.Step; output string; err error; dur time.Duration }
    results := make([]result, len(p.Steps))
    
    for i, step := range p.Steps {
        wg.Add(1)
        sem <- struct{}{}
        go func(i int, step plan.Step) {
            defer wg.Done()
            defer func() { <-sem }()
            r, err := c.runner.Invoke(ctx, ToolRequest{...})
            results[i] = result{i, step, r.Output, err, r.CompletedAt.Sub(r.StartedAt)}
        }(i, step)
    }
    wg.Wait()
    
    // 过滤成功 + 优先级排序: success → duration → EstimatedTokens
    var successResults []result
    for _, r := range results { if r.err == nil { successResults = append(successResults, r) } }
    sort.Slice(successResults, func(i, j int) bool {
        if successResults[i].dur != successResults[j].dur { return successResults[i].dur < successResults[j].dur }
        return successResults[i].step.EstimatedTokens < successResults[j].step.EstimatedTokens
    })
    
    // PersistScope → SideEffectStatus 派生
    var sideEffect types.SideEffectStatus
    switch p.BlastRadius.PersistScope {
    case plan.PersistTransient: sideEffect = types.SideEffectNone
    case plan.PersistSession, plan.PersistPermanent: sideEffect = types.SideEffectCommitted
    default: sideEffect = types.SideEffectUnknown
    }
    
    return &wavescheduler.Artifact{Kind: types.ArtifactExperimentData, SourcePlanID: p.ID, SideEffectStatus: sideEffect, ...}, nil
}
```

## 3. ToolRunner 本地抽象（PR-C4 解耦）

```go
type ToolRunner interface {
    Invoke(ctx context.Context, req ToolRequest) (ToolResult, error)
}

type ToolRequest struct {
    SessionID      string
    ToolName       string
    Args           map[string]any
    IdempotencyKey string
    StepID         string
}

type ToolResult struct {
    ToolName    string
    Output      string
    ExitCode    int
    Error       error
    StartedAt   time.Time
    CompletedAt time.Time
}
```

PR-C4 落地时只需实现 ToolRunner interface，无需修改 Channel 代码。

## 4. 错误层（5 sentinels + 4 helpers）

| Sentinel | Code | 触发条件 |
|----------|------|---------|
| `ErrChannelNotFound` | EXEC_CHANNEL_9001 | PlanKind 未注册或未知 |
| `ErrChannelUnsupported` | EXEC_CHANNEL_9002 | Channel 不 Supports 该 PlanKind / 重复 Register |
| `ErrChannelStepCountMismatch` | EXEC_CHANNEL_9003 | Step 数与 Channel 期望不匹配 |
| `ErrChannelPlanNil` | — | Plan 参数为 nil |
| `ErrChannelToolRunnerNil` | EXEC_CHANNEL_9004 | ToolRunner 参数为 nil |

4 helpers：
- `NewChannelNotFoundError(kind string) *SentinelError`
- `NewChannelUnsupportedError(channelName, kind string) *SentinelError`
- `NewChannelStepCountMismatchError(channelName string, actual, expected, max int) *SentinelError`
- `NewChannelToolRunnerNilError(channelName string) *SentinelError`

## 5. 跨节点依赖

### 5.1 上游契约（从 Plan 接收）

- `Plan.Kind` → ChannelRouter.Route 1:1 路由
- `Plan.Steps` → Channel.Execute 顺序 / 并行执行
- `Plan.SourceObservationIDs` → Artifact.SourcePlanID 上游可追溯
- `Plan.BlastRadius.PersistScope` → ExplorationChannel 派生 `SideEffectStatus`

### 5.2 下游契约（向 Verify / Learn 交付）

- `Artifact.Kind` → Phase 4 Verify 4 态 Verdict 决策
- `Artifact.SourcePlanID` → 反向追溯 `Plan.SourceObservationIDs`
- `Artifact.SideEffectStatus` → 决定是否需要补偿 / 重试
- `Artifact.SideEffectDetail.IdempotencyKey` → 幂等性去重
- `Artifact.Summary` (ExplorationChannel ExperimentData) → Phase 5 Learn 消费
