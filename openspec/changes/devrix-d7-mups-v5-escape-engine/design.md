# Design — devrix-d7-mups-v5-escape-engine (DM-20260625-003)

**Change ID:** `devrix-d7-mups-v5-escape-engine`
**Demand ID:** DM-20260625-003
**Sprint:** mups-v5
**SoT:** `brain/.../core-concepts/38-mature-uncertainty-methodology.md §21`（行 3621-4025，400 行 v5 完整设计）

---

## 1. 设计原则

**复用 doc 38 §21 完整设计，本 design.md 只做 devrix 落地映射**——核心架构、Go 代码骨架、流程图、5 类兜底动作、3 层仲裁均直接引用 doc 38 §21。

**devrix 特有约束**：
- 使用 `internal/shared/errors/` SentinelError 模式
- D7 域 `internal/layers/orchestration/` 子包结构
- Phase 1-7 已实现的 5 节点数据契约（Plan/Artifact/Verdict/LearningAsset）保持不变
- 5 节点接线失败降级：Evaluate error → slog.Warn + 继续

## 2. 模块结构

```
internal/layers/orchestration/escape/
├── loop_depth_tracker.go     (PR-V5.1) LoopContext + hash + History
├── plan_kind_switch_policy.go (PR-V5.2) 3 档策略
├── arbitrator.go             (PR-V5.3) EscapeAction + 3 层仲裁
├── circuit_breaker.go        (PR-V5.4) 5 层接线
├── engine.go                 (PR-V5.4) EscapeEngine 整合入口
├── audit_log.go              (PR-V5.4) AuditLevel 0/1/2
└── *_test.go                 (随每个 PR)
```

## 3. SoT 引用映射

| doc 38 §21 内容 | devrix 落地位置 | 行数 |
|-----------------|-----------------|------|
| §21.1 4 类深度限制整合 | escape/loop_depth_tracker.go (Tracker+LoopBudget) + escape/circuit_breaker.go (CircuitBreaker 5 层) | 100 |
| §21.2 关键漏洞（Plan.Kind 切换绕过回路）| escape/plan_kind_switch_policy.go (3 档策略 + PlanKindSwitchCount) | 50 |
| §21.3 v5 完整设计 | escape/arbitrator.go (5 类 EscapeAction + 3 层仲裁) + escape/engine.go (整合) | 250 |
| §21.4 Plan.Kind 切换 vs DenialBudget 协同 | escape/plan_kind_switch_policy.go 集成到 planner.MatchKind 之后 | 30 |
| §21.5 完整流程图 | doc 38 §21.5 文字版 + devrix spec.md v4.8.0 §5.x 接线图 | 80 |
| §21.6 4 类深度限制整合 | escape/engine.go.Evaluate 整合 | 50 |
| §21.7 与现有沉淀兼容 | Phase 1-7 已实现数据契约保持不变 | — |
| §21.8 实施工作量 | proposal.md §4 5 PR 拆分 | — |

**Go 代码骨架直接复用 doc 38 §21.3.2 - §21.4.2**（已在 doc 38 行 3706-3904 给出完整实现）。

## 4. 数据结构（基于 doc 38 §21.3.2 + §21.3.3）

```go
// escape/types.go

// LoopContext 回路上下文（按"模式 hash"计数的输入）
type LoopContext struct {
    SessionID           string
    PlanKind            PlanKind         // 4 类：CommitmentPlan / ProtocolPlan / ScenarioPlan / ExplorationPlan
    PrevPlanKind        PlanKind         // 用于切换计数
    ObservationKind     ObservationKind  // 4 类：ObsFact/ObsSignal/ObsDeviation/ObsUncertainty
    FailureCriterion    string           // Plan 失败判据
    ArtifactType        ArtifactType     // 4 类：StateChangeCert/ResponseRecord/ProbeReport/ExperimentData
    PlanKindSwitchCount int              // 累计切换次数
}

// EscapeAction 5 类兜底动作（按严重程度递增）
type EscapeAction int

const (
    EscapeContinue      EscapeAction = iota // 继续（未到上限）
    EscalateToRule                           // 升级到规则强制
    EscalateToHuman                          // 升级到人工接管
    EscapeForceExit                          // 强制退出（带 ExitReason）
    EscapeAbortWithAudit                     // 强制终止 + 完整审计（最严重）
)

// EscapeDecision 逃逸决策
type EscapeDecision struct {
    Action     EscapeAction
    Reason     string
    AuditLevel int  // 0=无审计, 1=记录, 2=完整审计
    Depth      int  // 当前回路深度
}

// PlanKindSwitchPolicy 3 档策略
type PlanKindSwitchPolicy int

const (
    SwitchAllowed       PlanKindSwitchPolicy = iota // 自由切换
    SwitchConstrained                                // 限制切换次数（≤4）
    SwitchForbidden                                  // 禁止切换
)
```

## 5. 核心算法

### 5.1 hashLoopContext（doc 38 §21.3.2）

```go
func hashLoopContext(ctx LoopContext) string {
    h := sha256.New()
    h.Write([]byte(fmt.Sprintf("%s:%s:%s:%s:%s",
        ctx.SessionID,
        ctx.PlanKind,
        ctx.ObservationKind,
        ctx.FailureCriterion,
        ctx.ArtifactType)))
    return hex.EncodeToString(h.Sum(nil))
}
```

**关键设计**：**按"模式 hash"计数，而非"按轮数"计数**。这意味着 LLM 切换 Plan.Kind 算作新回路（depth=1），但同一模式重复会被累积计数。

### 5.2 PlanKindSwitchPolicy（doc 38 §21.4.2）

| PlanKind | Policy | 理由 |
|----------|--------|------|
| PlanExploration | Constrained (≤4) | 避免 LLM 反复尝试不同"探索"角度 |
| PlanScenario | Allowed | 并行多假设是合法的 |
| PlanProtocol | Constrained (≤4) | 防止规则层震荡 |
| PlanCommitment | Forbidden | 一旦承诺就要执行到底 |

### 5.3 ChainedArbitrator 3 层（doc 38 §21.3.4）

```
LLMArbitrator (5s timeout) → EscapeContinue
  ↓ LLM 选 Exit / timeout
RuleArbitrator (规则检查)
  ├─ 不可恢复失败 → AbortWithAudit
  └─ 可恢复失败 → EscalateToHuman
HumanArbitrator (用户接管 A/B/C, 10s timeout)
  ├─ A. 继续 → EscapeContinue
  ├─ B. 接受 → ForceExit
  └─ C. 取消 → AbortWithAudit
```

**devrix 落地差异**：
- LLMArbitrator 5s timeout 兜底 ForceExit（不阻塞主链路）
- HumanArbitrator 异步化（不阻塞 ProcessMessage 同步返回）— 详见 §5.3.1
- 失败降级：EscapeEngine.Evaluate error → slog.Warn + EscapeContinue

### 5.3.1 HumanArbitrator 详细设计（devrix 落地）

**核心约束**（与 Phase 7 Auto-Close 协同）：
- `ProcessMessage` 是**同步接口**（飞书卡片立即显示）
- HumanArbitrator **不能同步等待 user 响应**（10s 阻塞会破坏飞书卡片体验）
- 必须**异步注册 + 立即返回 + goroutine 兜底**

**关键设计决策**：

| 决策 | 选择 | 理由 |
|------|------|------|
| **D1**：Evaluate 同步 vs 异步？ | 同步返回 + 内部异步 | ProcessMessage 同步约束；用 `EscapePendingHuman` 中间态立即返回 |
| **D2**：user 响应后 decision 怎么应用？ | audit 持久化 + 下次 ProcessMessage 续跑 | 类似 Phase 6 buildObserveRequest 模式 |
| **D3**：notifyUser 通道？ | 可插拔 `Notifier` interface | dev 默认 `FeishuCardNotifier`（3 按钮 A/B/C），可扩展 `CLINotifier`/`EmailNotifier` |

**EscapeAction 扩展**：在 doc 38 §21.3.3 5 类基础上新增 1 个中间态：

```go
const (
    EscapeContinue      EscapeAction = iota  // 继续（未到上限）
    EscalateToRule                            // 升级到规则强制
    EscalateToHuman                           // 升级到人工接管
    EscapeForceExit                           // 强制退出（带 ExitReason）
    EscapeAbortWithAudit                      // 强制终止 + 完整审计
    EscapePendingHuman                        // 中间态：等待用户响应（v5 dev 扩展）
)
```

**EscapeDecision 扩展**：

```go
type EscapeDecision struct {
    Action     EscapeAction
    Reason     string
    AuditLevel int       // 0=无审计, 1=记录, 2=完整审计
    Depth      int       // 当前回路深度
    PendingID  string    // 仅 EscapePendingHuman 时填充
}
```

**HumanArbitrator 实现**：

```go
type HumanArbitrator struct {
    timeout  time.Duration           // 默认 10s
    notifier Notifier                 // 可插拔（FeishuCard/CLI/Email）
    audit    *EscapeAuditLog
    mu       sync.Mutex
    pending  map[string]chan UserChoice  // pendingID → user input channel
    resume   PendingResolutionStore   // session 状态（DB / Memory / 内存）
}

type UserChoice struct {
    Value     string  // "A" | "B" | "C"
    PendingID string
    Timestamp time.Time
}

// Arbitrate 同步调用，立即返回
func (a *HumanArbitrator) Arbitrate(ctx context.Context, loopCtx LoopContext, decisions []EscapeDecision) EscapeDecision {
    // 1. 注册 pending decision
    pendingID := uuid.New().String()
    userInputCh := make(chan UserChoice, 1)  // buffered=1 防 goroutine 泄漏
    a.mu.Lock()
    a.pending[pendingID] = userInputCh
    a.mu.Unlock()
    
    // 2. 异步通知 user（不阻塞）
    go a.notifier.Notify(loopCtx, pendingID, decisions)
    
    // 3. 启动 goroutine 等待 user 响应或 timeout
    go a.waitForUserResponse(ctx, pendingID, userInputCh, loopCtx, decisions)
    
    // 4. 立即返回中间态（ProcessMessage 不阻塞）
    return EscapeDecision{
        Action:     EscapePendingHuman,
        Reason:     "human_review_required",
        PendingID:  pendingID,
        AuditLevel: 1,
    }
}

func (a *HumanArbitrator) waitForUserResponse(ctx context.Context, pendingID string, ch chan UserChoice, loopCtx LoopContext, decisions []EscapeDecision) {
    timer := time.NewTimer(a.timeout)
    defer timer.Stop()
    defer a.cleanupPending(pendingID)
    
    var finalDecision EscapeDecision
    select {
    case choice := <-ch:
        // 路径 1: user 提前响应
        finalDecision = mapToEscapeDecision(choice.Value, loopCtx)
    case <-timer.C:
        // 路径 2: 10s timeout 兜底
        finalDecision = EscapeDecision{
            Action:     EscapeForceExit,
            Reason:     "human_timeout_10s",
            AuditLevel: 2,
        }
    case <-ctx.Done():
        // 路径 3: ctx 取消（ProcessMessage 客户端断开）
        finalDecision = EscapeDecision{
            Action:     EscapeForceExit,
            Reason:     "ctx_cancelled",
            AuditLevel: 2,
        }
    }
    
    // 持久化 decision（下次 ProcessMessage 续跑）
    a.audit.Record(loopCtx, decisions, finalDecision)
    a.resume.Save(loopCtx.SessionID, finalDecision)
}

// SubmitUserChoice：user 响应入口（feishu 按钮 callback / CLI handler 调用）
func (a *HumanArbitrator) SubmitUserChoice(pendingID string, choice UserChoice) {
    a.mu.Lock()
    ch, ok := a.pending[pendingID]
    a.mu.Unlock()
    if !ok {
        return  // expired 或不存在，丢弃
    }
    select {
    case ch <- choice:  // non-blocking（buffered=1）
    default:
        // pending 已被 consume，丢弃
    }
}

func mapToEscapeDecision(value string, loopCtx LoopContext) EscapeDecision {
    switch value {
    case "A":
        return EscapeDecision{Action: EscapeContinue, Reason: "user_continue", AuditLevel: 1}
    case "B":
        return EscapeDecision{Action: EscapeForceExit, Reason: "user_accept", AuditLevel: 1}
    case "C":
        return EscapeDecision{Action: EscapeAbortWithAudit, Reason: "user_cancel", AuditLevel: 2}
    default:
        return EscapeDecision{Action: EscapeForceExit, Reason: "user_invalid_choice", AuditLevel: 2}
    }
}
```

**Notifier 接口**（可插拔）：

```go
type Notifier interface {
    Notify(loopCtx LoopContext, pendingID string, decisions []EscapeDecision) error
}

// dev 默认实现：飞书卡片 + 3 按钮 A/B/C
type FeishuCardNotifier struct {
    cardClient FeishuCardClient
    userID     string
}

func (n *FeishuCardNotifier) Notify(loopCtx LoopContext, pendingID string, decisions []EscapeDecision) error {
    card := FeishuCard{
        Title: "🔧 回路需要人工接管",
        Body:  buildHumanReviewBody(loopCtx, decisions),
        Buttons: []FeishuButton{
            {Label: "A. 继续尝试", Value: "A", PendingID: pendingID},
            {Label: "B. 接受当前", Value: "B", PendingID: pendingID},
            {Label: "C. 取消",     Value: "C", PendingID: pendingID},
        },
        ExpiresAt: time.Now().Add(10 * time.Second),
    }
    return n.cardClient.UpdateCard(n.userID, card)
}
```

**PendingResolutionStore 接口**（续跑）：

```go
type PendingResolutionStore interface {
    Save(sessionID string, decision EscapeDecision) error
    Load(sessionID string) (EscapeDecision, bool, error)  // found
    Delete(sessionID string) error
}

// dev 默认：内存实现（可替换为 DB / Redis）
type InMemoryPendingResolutionStore struct {
    mu   sync.RWMutex
    data map[string]EscapeDecision
}
```

### 5.3.2 完整决策流程（含 Human 异步）

```
[T1] ProcessMessage 触发
  └─ Observe → Plan → Execute → Verify → FAIL
       └─ EscapeEngine.Evaluate(ctx)
            ├─ tracker.ShouldContinue → Continue
            ├─ loopBudget.Evaluate → Continue
            ├─ circuitBreaker.Evaluate → Continue
            └─ 全部 Continue → EscapeContinue → 继续回路

[T1.5] ProcessMessage 触发
  └─ Observe → Plan → Execute → Verify → FAIL (4 回路已尽)
       └─ EscapeEngine.Evaluate(ctx)
            ├─ tracker.ShouldContinue → ForceExit (loop_depth_exceeded)
            ├─ 决策非空 → ChainedArbitrator.Arbitrate
            │    ├─ LLMArbitrator (5s)
            │    │    ├─ LLM 选 Continue → EscapeContinue
            │    │    └─ LLM 选 Exit / timeout → 下层
            │    ├─ RuleArbitrator
            │    │    ├─ 不可恢复 → AbortWithAudit (AuditLevel=2)
            │    │    └─ 可恢复 → 下层
            │    └─ HumanArbitrator
            │         ├─ 注册 pendingID + 异步通知 feishu 卡片
            │         ├─ 启动 10s timer goroutine
            │         └─ 立即返回 EscapePendingHuman (ProcessMessage 同步返回)
            │
            └─ ProcessMessage 立即返回（飞书卡片显示"已升级到人工"）

[后台] goroutine 等待：
  ├─ 路径 1: feishu 按钮 callback → SubmitUserChoice → 收到 choice
  │    └─ mapToEscapeDecision → 写 audit + session 状态
  ├─ 路径 2: 10s timeout
  │    └─ EscapeForceExit + AuditLevel=2 + 写 session 状态
  └─ 路径 3: ProcessMessage 客户端断开
       └─ EscapeForceExit + AuditLevel=2 + 写 session 状态

[T2] user 下次 ProcessMessage 进入
  └─ resume.Load(sessionID) → 命中
       ├─ user_choice=A → 继续回路（重跑 Plan → Execute → Verify）
       ├─ user_choice=B → ForceExit（已 ForceExit，无新动作）
       └─ user_choice=C → AbortWithAudit（已终止）
```

### 5.3.3 4 类边界场景兜底

| 场景 | 行为 | 兜底机制 |
|------|------|---------|
| 10s 内 user 点 A/B/C | 应用 user choice | 正常路径 |
| 10s 内 user 不响应 | EscapeForceExit + AuditLevel=2 | 10s timer 兜底 |
| ProcessMessage 客户端断开 | EscapeForceExit + AuditLevel=2 | ctx.Done() 兜底 |
| user 响应但 pendingID 已 timeout/cleanup | SubmitUserChoice 丢弃（map 已 cleanup）| map cleanup 兜底 |
| LLMArbitrator 自身 panic | recover + ForceExit | 失败降级（同 Phase 7 模式）|
| Notifier 发送失败 | 降级为 CLI prompt（terminal 用户）| Notifier 链式 fallback |

## 6. 5 节点接线

每个 Plan→Execute→Verify→[Compensation] 路径前调用 `EscapeEngine.Evaluate(ctx)`：

```go
// 5 节点伪代码（具体见 PR-V5.5）
func (o *Orchestrator) ProcessMessage(ctx context.Context, req ProcessRequest) error {
    observe := o.observe(ctx, req)
    
    for {
        plan, err := o.plan(ctx, observe)
        if err != nil {
            return err
        }
        
        // ★ EscapeEngine 接线点 1: Plan 前
        loopCtx := buildLoopContext(ctx, plan, observe)
        decision := o.escapeEngine.Evaluate(loopCtx)
        if decision.Action == EscapeForceExit {
            return newExitError(decision.Reason)
        }
        
        artifact, err := o.execute(ctx, plan)
        if err != nil {
            // ★ EscapeEngine 接线点 2: Execute 失败
            loopCtx.PrevPlanKind = plan.Kind
            decision := o.escapeEngine.Evaluate(loopCtx)
            ...
        }
        
        verdict := o.verify(ctx, artifact, plan)
        if verdict.Kind == VerdictFail || verdict.Kind == VerdictIndeterminate {
            // ★ EscapeEngine 接线点 3: Verify 失败
            ...
        }
        
        if decision.Action == EscapeContinue {
            break
        }
    }
    
    return nil
}
```

## 7. CircuitBreaker 5 层接线（基于现有 5 metrics）

| 层 | doc 38 §21 SoT | devrix 现有 metric | 触发条件 |
|----|----------------|-------------------|----------|
| L0 | AnomalyDetector 5 nil | `anomaly_detector_nil_total` | 5 次连续 |
| L1 | (新增) | `dispatch_loop_wakeups_total` | 100 次/分钟 |
| L2 | Verifier 3 > 2s | `verifier_latency_p95_seconds` | 3 次 > 2s |
| L3 | Hook 5 fail | `hook_failure_total` | 5 次连续 |
| L4 | (新增) | `worker_panics_total` | 1 次 panic |
| L5 | (新增) | `sandbox_exit_failed_total` | 5 次连续 |

**devrix 现有 5 metrics 中保留**：`state.cancels` / `state.handles` 是状态追踪不是故障指标，不升级为 circuit breaker。

## 8. 与 Phase 1-7 数据契约的兼容

| Phase | 已有 | v5 复用 |
|-------|------|--------|
| Phase 2 | Plan.PlanKind 4 类 | ✅ 直接使用 |
| Phase 2 | Observation.ObservationKind 4 类 | ✅ 直接使用 |
| Phase 3 | Artifact.Kind 4 类 | ✅ 直接使用 |
| Phase 3 | Plan.FailureCriteria | ✅ 提升为 LoopContext.FailureCriterion |
| Phase 4 | Verdict 4 态 | ✅ EscapeEngine 接线点 3 用 |
| Phase 4 | ExitReason 14 类 | ✅ 保留 + 映射到 EscapeAction 5 类 |
| Phase 5 | LearningAsset 5 类 | ✅ AbortWithAudit 时持久化 |
| Phase 5 | ReputationEvidence | ✅ 回路深度计数输入 |
| Phase 6 | buildObserveRequest 3 层 fail-safe | ✅ 复用为 EscapeEngine 失败降级 |
| Phase 7 | Auto-Close | ✅ 复用为 EscapeEngine 异步路径 |

**结论**：v5 复用 Phase 1-7 全部数据契约，**零破坏性变更**。

## 9. 失败降级策略

| 失败点 | 降级行为 | 原因 |
|--------|---------|------|
| EscapeEngine.Evaluate panic | recover + slog.Warn + EscapeContinue | 不阻塞主链路 |
| EscapeEngine.Evaluate error | slog.Warn + EscapeContinue | 监控缺失但不影响流程 |
| LLMArbitrator timeout (5s) | ForceExit 兜底 | 避免无限等待 LLM |
| HumanArbitrator 不响应 | 10s 后 ForceExit 兜底 | 同 Phase 7 Auto-Close |
| CircuitBreaker.Evaluate error | slog.Warn + 不触发 | 监控缺失但不影响流程 |

## 10. References

- doc 38 §21（行 3621-4025，400 行 v5 完整设计）
- doc 38 §21.5 完整流程图
- doc 38 §21.3.2 LoopDepthTracker v2 Go 骨架
- doc 38 §21.3.3 EscapeAction 5 类 Go 骨架
- doc 38 §21.3.4 ChainedArbitrator 3 层 Go 骨架
- doc 38 §21.4.2 PlanKindSwitchPolicy Go 骨架
- 9 个 MUPS v4 归档
- `openspec/specs/d7-orchestration/pipeline-architecture.md`
