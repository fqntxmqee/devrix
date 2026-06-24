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
// 7 字段：5 hash 输入 + 2 状态（采纳 codex review §3.3 建议，11→7 精简）
// 注：PrevPlanKind / PlanKindSwitchCount 不在 LoopContext，由 PlanKindSwitchPolicy 模块自管
type LoopContext struct {
    // 5 hash 输入字段（参与 hashLoopContext，模式识别用）
    SessionID        string
    PlanKind         PlanKind         // 4 类：CommitmentPlan / ProtocolPlan / ScenarioPlan / ExplorationPlan
    ObservationKind  ObservationKind  // 4 类：ObsFact/ObsSignal/ObsDeviation/ObsUncertainty
    FailureCriterion string           // Plan 失败判据
    ArtifactType     ArtifactType     // 4 类：StateChangeCert/ResponseRecord/ProbeReport/ExperimentData

    // 2 状态字段（不参与 hash，但作为 LoopContext 一部分传入 Evaluate）
    LoopBudgetState  LoopBudgetState  // DenialBudget 状态（consecutive=3, total=20）
    ExitReason       ExitReason       // 14 类现有 ExitReason 映射（接线点 2/3 注入）
}

// LoopBudgetState DenialBudget 累计状态
type LoopBudgetState struct {
    ConsecutiveFails int  // 连续失败次数（达到 3 触发 ForceExit）
    TotalFails       int  // 累计失败次数（达到 20 触发 AbortWithAudit）
}

// PlanKindSwitchPolicy 内部 state（不在 LoopContext）
// PrevPlanKind / PlanKindSwitchCount 由 policy 模块自管，避免污染 LoopContext
type planKindSwitchState struct {
    mu            sync.Mutex
    lastPlanKind  PlanKind  // 用于检测切换
    switchCount   int       // 累计切换次数（达到 4 触发 ForceExit / CommitmentPlan 0 触发）
}

// EscapeAction 6 类动作（5 类正式 + 1 个 dev 扩展中间态）
// 详见 §5.3.1 — 6 类原因：devrix HumanArbitrator 异步化新增 EscapePendingHuman 中间态
type EscapeAction int

const (
    EscapeContinue      EscapeAction = iota // 继续（未到上限）
    EscalateToRule                           // 升级到规则强制
    EscalateToHuman                          // 升级到人工接管
    EscapeForceExit                          // 强制退出（带 ExitReason）
    EscapeAbortWithAudit                     // 强制终止 + 完整审计（最严重）
    EscapePendingHuman                       // 中间态：等待用户响应（v5 dev 扩展）
)

// EscapeDecision 逃逸决策
// 9 字段：5 核心 + 4 审计/续跑关键
type EscapeDecision struct {
    // 5 核心字段
    Action     EscapeAction
    Reason     string
    AuditLevel int  // 0=无审计, 1=记录, 2=完整审计
    Depth      int  // 当前回路深度
    PendingID  string  // 仅 EscapePendingHuman 时填充

    // 4 审计/续跑关键字段
    ExitReason         ExitReason  // 14 类现有 ExitReason 映射（保留兼容）
    SessionID          string      // audit 持久化 key
    CreatedAt          time.Time   // 审计时间戳
    SourceDecisionIDs  []string    // 上游决策链（用于 audit 追溯）
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

**MaxDepth 判定规则**（明确边界）：
- `depth < MaxDepth` → EscapeContinue（继续回路）
- `depth >= MaxDepth` → EscapeForceExit（强制退出）
- 例：MaxDepth=3 时，depth=1/2 → Continue，depth=3 → ForceExit

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

**ChainedArbitrator.Arbitrate 完整 Go 骨架**（采纳 review-r3 ISSUE-2 建议）：

```go
// ChainedArbitrator 链式调度 3 层仲裁
// 调度顺序：LLM → Rule → Human，每层根据 Action 决定下一步
type ChainedArbitrator struct {
    llm   *LLMArbitrator
    rule  *RuleArbitrator
    human *HumanArbitrator
}

// Arbitrate 链式调用 3 层仲裁，返回最终 EscapeDecision
// 关键不变式：
//   1. 返回值 Action ∈ {EscapeContinue, EscapePendingHuman, EscapeForceExit, EscapeAbortWithAudit}
//   2. EscalateToRule / EscalateToHuman 中间态绝不返回给 caller（由本函数消化）
//   3. 任何一层 panic / error → 降级到下一层（不阻塞主链路）
func (c *ChainedArbitrator) Arbitrate(ctx context.Context, loopCtx LoopContext, decisions []EscapeDecision) EscapeDecision {
    chain := append([]EscapeDecision{}, decisions...)

    // 第 1 层：LLMArbitrator
    llmDecision, err := c.llm.Arbitrate(ctx, loopCtx, chain)
    if err == nil {
        switch llmDecision.Action {
        case EscapeContinue:
            return llmDecision  // LLM 选 Continue 立即返回
        case EscapeForceExit, EscapeAbortWithAudit:
            return llmDecision  // LLM 选终止直接返回（timeout/panic 兜底）
        }
    } else {
        // LLM error → 降级为 EscalateToRule 走下一层
        llmDecision = EscapeDecision{
            Action:     EscapeForceExit,  // 兜底终止
            Reason:     "llm_error_" + err.Error(),
            AuditLevel: 1,
        }
        return llmDecision
    }

    // llmDecision.Action == EscalateToRule（继续向下）
    chain = append(chain, llmDecision)

    // 第 2 层：RuleArbitrator
    ruleDecision, err := c.rule.Arbitrate(ctx, loopCtx, chain)
    if err == nil {
        switch ruleDecision.Action {
        case EscapeAbortWithAudit:
            return ruleDecision  // Rule 不可恢复 → 终止
        case EscapeContinue, EscapeForceExit:
            return ruleDecision  // Rule 终止决策（少见但允许）
        }
    } else {
        // Rule error → 降级为 EscalateToHuman 走下一层
        ruleDecision = EscapeDecision{
            Action:     EscapeForceExit,  // 兜底终止
            Reason:     "rule_error_" + err.Error(),
            AuditLevel: 1,
        }
        return ruleDecision
    }

    // ruleDecision.Action == EscalateToHuman（继续向下）
    chain = append(chain, ruleDecision)

    // 第 3 层：HumanArbitrator（异步路径，立即返回中间态）
    return c.human.Arbitrate(ctx, loopCtx, chain)
}
```

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

**6 类对外/对内语义分层**（采纳 codex review §3.2 建议，澄清冗余疑问）：

| EscapeAction | 对外 API 暴露？ | 对内链式传递？ | 说明 |
|--------------|---------------|---------------|------|
| `EscapeContinue` | ✅ 4 类核心之一 | ✅ | LLM/Rule/Human 任一层选 Continue → 立即返回，继续回路 |
| `EscapePendingHuman` | ✅ 4 类核心之一 | ✅ | Human 异步路径立即返回，session 状态持久化 |
| `EscapeForceExit` | ✅ 4 类核心之一 | ✅ | 强制退出，最终决策（兜底/超时/panic）|
| `EscapeAbortWithAudit` | ✅ 4 类核心之一 | ✅ | 强制终止 + 完整审计，最严重（不可恢复失败）|
| `EscalateToRule` | ❌ 对外不暴露 | ✅ | ChainedArbitrator 内部链式传递："LLM 选 Exit → Rule 还没裁决"中间态 |
| `EscalateToHuman` | ❌ 对外不暴露 | ✅ | ChainedArbitrator 内部链式传递："Rule 选 Human → Human 还没裁决"中间态 |

**澄清 codex 担心**：
- `EscalateToRule` / `EscalateToHuman` 作为 enum 值**保留**，因为：
  - ChainedArbitrator 内部链式传递需要中间值（避免 Rule 还没跑就 return EscalateToHuman 给 caller）
  - audit log 追溯有语义价值（"LLM 拒了 → Rule 拒了 → Human 接了" vs 直接 "Human 接了"）
- 对外 API（EscapeEngine.Evaluate 返回值）实际只暴露 4 类核心，调用方不应见到 EscalateToRule/Human
- 实现约束：caller 在 `processEscapeDecision` switch 时**不应**有 `case EscalateToRule, EscalateToHuman` 分支（由 ChainedArbitrator 内部消化），若有则视为 bug（兜底为 ForceExit）

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

// ResumeSession：T2 续跑入口
// 由 EscapeEngine.ResumeSession 委托调用，ProcessMessage 开头检查
// 返回 (decision, found)：
//   found=true  → 上次 ProcessMessage 升级到 Human 已持久化，续跑
//   found=false → 走完整 5 节点流程
func (a *HumanArbitrator) ResumeSession(sessionID string) (EscapeDecision, bool) {
    decision, found, err := a.resume.Load(sessionID)
    if err != nil || !found {
        return EscapeDecision{}, false
    }
    // 消费后立即删除（防重复续跑）
    _ = a.resume.Delete(sessionID)
    return decision, true
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

// SubmitOverrideCard 覆盖已发卡片（ctx.Done() 兜底用）
func (n *FeishuCardNotifier) SubmitOverrideCard(pendingID string, msg string, buttons []FeishuButton) error {
    return n.cardClient.UpdateCard(n.userID, FeishuCard{
        Title: "⚠️ " + msg,
        Buttons: buttons,
    })
}

// ChainedNotifier 链式 fallback 装饰器（M2 实现）
// 顺序：FeishuCard → CLI → Email，任一成功立即返回，全部失败返回最后 error
type ChainedNotifier struct {
    notifiers []Notifier
}

func (c *ChainedNotifier) Notify(loopCtx LoopContext, pendingID string, decisions []EscapeDecision) error {
    var lastErr error
    for _, n := range c.notifiers {
        if err := n.Notify(loopCtx, pendingID, decisions); err == nil {
            return nil  // 任一成功立即返回
        } else {
            slog.Warn("notifier fallback", "pending_id", pendingID, "error", err)
            lastErr = err
        }
    }
    return lastErr  // 全部失败
}

func (c *ChainedNotifier) SubmitOverrideCard(pendingID string, msg string, buttons []FeishuButton) error {
    for _, n := range c.notifiers {
        override, ok := n.(OverrideCardNotifier)
        if !ok {
            // 采纳 review-r3 ISSUE-4 建议：类型断言失败时 slog.Warn 降级
            slog.Warn("notifier_does_not_support_override",
                "notifier_type", fmt.Sprintf("%T", n),
                "pending_id", pendingID)
            continue
        }
        if err := override.SubmitOverrideCard(pendingID, msg, buttons); err == nil {
            return nil
        }
    }
    return fmt.Errorf("all notifiers failed to override card")
}

// OverrideCardNotifier 可选接口（不是所有 Notifier 都支持覆盖）
// 采纳 review-r3 ISSUE-4 建议：明确为可选 interface，与 Notifier 完全独立
type OverrideCardNotifier interface {
    SubmitOverrideCard(pendingID string, msg string, buttons []FeishuButton) error
}

// Notifier interface 定义见上方 line 421-423（仅 Notify 方法，无重复定义）
// 完整 Notifier interface 已在首次定义处声明，避免重复声明导致的编译错误
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
  └─ EscapeEngine.ResumeSession(sessionID) → 委托给 HumanArbitrator.ResumeSession
       ├─ resume.Load(sessionID) → 未命中 → 走完整 5 节点流程
       └─ resume.Load(sessionID) → 命中（消费后 Delete，防重复续跑）
            ├─ user_choice=A → 继续回路（重跑 Plan → Execute → Verify，**depth 续 T1.5 状态**）
            │    具体：LoopDepthTracker.SessionID 维度保留 depth=3，下次回路 +1=4 → ForceExit
            │    （即 depth 跨 ProcessMessage 边界不重置，符合"模式 hash"语义）
            ├─ user_choice=B → ForceExit（已 ForceExit，无新动作）
            └─ user_choice=C → AbortWithAudit（已终止）

**depth 续 T1 状态机制**（M4 明确）：
- `LoopDepthTracker` 内部按 `SessionID` 维度保留 History map（不随 ProcessMessage 结束清空）
- T1.5 触发 EscapePendingHuman 时，depth=3 状态保留
- T2 续跑 user 选 A → 重跑回路 → depth=4 → ForceExit
- 避免"重置 depth 让回路无限续命"的漏洞
- 唯一清空时机：`tracker.Reset()`（仅在 session 彻底结束 / admin reset 调用）

**applyResumeDecision 完整 Go 骨架**（采纳 review-r3 ISSUE-3 建议）：

```go
// applyResumeSession T2 续跑入口
// 由 EscapeEngine.ResumeSession 委托调用，ProcessMessage 开头检查
// 返回值: error → caller 应 return（终止 ProcessMessage）
//
// 关键不变式：
//   1. decision.Action ∈ {EscapeContinue, EscapeForceExit, EscapeAbortWithAudit}
//      （EscalateTo* 中间态已由 ChainedArbitrator 消化，不会出现在此处）
//   2. user_choice=A → Continue：进入 runLoopWithResume 重跑回路（depth 续 T1 状态）
//   3. user_choice=B → ForceExit：等价于正常终止（user 已接受当前）
//   4. user_choice=C → AbortWithAudit：不可恢复终止 + 补写 audit
func (o *Orchestrator) applyResumeSession(ctx context.Context, decision EscapeDecision) error {
    switch decision.Action {
    case EscapeContinue:
        // user_choice=A：续跑回路
        // depth 由 LoopDepthTracker 按 SessionID 维度自动续 T1 状态
        return o.runLoopWithResume(ctx, decision)
    case EscapeForceExit:
        // user_choice=B：用户接受当前 → 等价于正常 ForceExit
        return newExitError(decision.Reason)
    case EscapeAbortWithAudit:
        // user_choice=C：用户取消 → 不可恢复终止
        o.escapeEngine.AuditLog().Record(decision)  // 补写 audit（保留 user_cancel 决策痕迹）
        return newExitError(decision.Reason)
    default:
        // 兜底：不应出现的 Action 值（EscalateTo* 中间态）
        slog.Error("invalid_resume_decision",
            "action", decision.Action,
            "reason", decision.Reason,
            "session_id", decision.SessionID)
        return newExitError("invalid_resume_decision_" + decision.Reason)
    }
}

// runLoopWithResume 续跑回路（复用 ProcessMessage 内层 for 循环逻辑）
// depth 续 T1 状态由 LoopDepthTracker 自动保证
func (o *Orchestrator) runLoopWithResume(ctx context.Context, resumeDecision EscapeDecision) error {
    for {
        plan, err := o.plan(ctx, nil)  // T2 入口重新规划
        if err != nil {
            // 接线点 1a：Plan 失败 → EscapeEngine 升级
            loopCtx := buildLoopContext(ctx, nil, nil)
            decision := o.escapeEngine.Evaluate(loopCtx)
            return o.processEscapeDecision(decision, err)
        }

        artifact, err := o.execute(ctx, plan)
        if err != nil {
            loopCtx := buildLoopContext(ctx, plan, nil)
            loopCtx.PrevPlanKind = plan.Kind
            decision := o.escapeEngine.Evaluate(loopCtx)
            if terminate, derr := o.processEscapeDecision(decision, err); terminate {
                return derr
            }
            continue
        }

        verdict := o.verify(ctx, artifact, plan)
        if verdict.Kind == VerdictFail || verdict.Kind == VerdictIndeterminate {
            loopCtx := buildLoopContext(ctx, plan, nil)
            loopCtx.FailureCriterion = verdict.Reason
            decision := o.escapeEngine.Evaluate(loopCtx)
            if terminate, derr := o.processEscapeDecision(decision, nil); terminate {
                return derr
            }
            continue
        }

        // Verify 通过 → 跳出回路
        return nil
    }
}
```

**与 Phase 7 Auto-Close 的差异**：
- Auto-Close：同步返回 + 内部异步（不续跑，仅本次 ProcessMessage 异步通知）
- T2 Resume：同步返回 + 下次 ProcessMessage 续跑（user 响应跨 ProcessMessage 边界应用）
```

### 5.3.3 7 类边界场景兜底

| 场景 | 行为 | 兜底机制 |
|------|------|---------|
| 10s 内 user 点 A/B/C | 应用 user choice | 正常路径 |
| 10s 内 user 不响应 | EscapeForceExit + AuditLevel=2 | 10s timer 兜底 |
| ProcessMessage 客户端断开 | EscapeForceExit + AuditLevel=2 | ctx.Done() 兜底 |
| user 响应但 pendingID 已 timeout/cleanup | SubmitUserChoice 丢弃（map 已 cleanup）| map cleanup 兜底 |
| LLMArbitrator 自身 panic | recover + ForceExit | 失败降级（同 Phase 7 模式）|
| Notifier 发送失败 | 降级为下一个 Notifier | Notifier 链式 fallback（见 ChainedNotifier）|
| **ctx.Done() 后飞书卡片已发**（UI/状态不一致）| ctx.Done() 兜底分支同步发"已强制退出"覆盖卡片 | submitOverrideCard(pendingID, "已强制退出") 同步调用；user 响应过期也写 audit("user_late_response") |

**M6 LLM 仲裁 4 类异常处理**（设计补全）：

```go
func (a *LLMArbitrator) Arbitrate(ctx context.Context, loopCtx LoopContext, decisions []EscapeDecision) (EscapeDecision, error) {
    defer func() {
        if r := recover(); r != nil {
            slog.Error("llm_arbitrator_panic", "panic", r, "session_id", loopCtx.SessionID)
            // 兜底：panic → ForceExit
        }
    }()

    // 1. 构造 prompt（注入 LoopContext + PlanKindSwitchCount）
    prompt := buildArbitratorPrompt(loopCtx, decisions)

    // 2. 5s timeout 调用 LLM
    llmCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    rawRespCh := make(chan string, 1)
    var rawResp string
    var llmErr error
    go func() {
        resp, err := a.llmClient.Generate(llmCtx, prompt)
        rawRespCh <- resp
        llmErr = err
    }()

    select {
    case rawResp = <-rawRespCh:
        // 路径 1: LLM 正常返回
    case <-llmCtx.Done():
        if ctx.Err() != nil {
            // M6 关键：ctx 取消语义优先于 LLM timeout
            return EscapeDecision{Action: EscapeForceExit, Reason: "ctx_cancelled", AuditLevel: 2}, ctx.Err()
        }
        return EscapeDecision{Action: EscapeForceExit, Reason: "llm_timeout_5s", AuditLevel: 2}, llmCtx.Err()
    case <-time.After(6 * time.Second):
        // 双保险：ctx 已 cancel 但 LLM 没退出，强制兜底
        return EscapeDecision{Action: EscapeForceExit, Reason: "llm_stuck_force_exit", AuditLevel: 2}, nil
    }

    if llmErr != nil {
        return EscapeDecision{Action: EscapeForceExit, Reason: "llm_error", AuditLevel: 1}, llmErr
    }

    // 3. 解析 LLM 响应（JSON 格式：{"action": "Continue|Exit", "reason": "..."}）
    action, reason, parseErr := parseArbitratorResponse(rawResp)
    if parseErr != nil {
        // M6: 1 次重试（注入格式示例）
        retryResp, retryErr := a.llmClient.Generate(llmCtx, prompt+"\n\n必须返回 JSON: {\"action\":\"Continue|Exit\",\"reason\":\"...\"}")
        if retryErr != nil {
            return EscapeDecision{Action: EscapeForceExit, Reason: "llm_non_json_after_retry", AuditLevel: 1}, retryErr
        }
        action, reason, parseErr = parseArbitratorResponse(retryResp)
        if parseErr != nil {
            return EscapeDecision{Action: EscapeForceExit, Reason: "llm_invalid_format", AuditLevel: 1}, parseErr
        }
    }

    // 4. 校验 action 合法性
    if action != "Continue" && action != "Exit" {
        // M6: 非法 action 直接 ForceExit（不传给 ChainedArbitrator 下层）
        return EscapeDecision{Action: EscapeForceExit, Reason: "llm_invalid_action_" + action, AuditLevel: 1}, nil
    }

    if action == "Continue" {
        return EscapeDecision{Action: EscapeContinue, Reason: reason, AuditLevel: 0}, nil
    }
    return EscapeDecision{Action: EscalateToRule, Reason: reason, AuditLevel: 1}, nil
}
```

**M6 关键设计**：
- **panic 优先 recover**：不阻塞主链路
- **ctx 取消语义优先**：ctx.Err() != nil 时 reason=ctx_cancelled（不是 llm_timeout）
- **LLM 解析失败 1 次重试**：注入格式示例，仍失败 → ForceExit
- **非法 action 拦截**：LLM 返回 "Continue|Exit" 之外的值 → ForceExit（不污染 ChainedArbitrator 下层）
- **6s 双保险 timer**：防 LLM 卡死（5s timeout + 1s grace）

**7 类 UI/状态一致性兜底**（H4 关键修复）：

```go
// ctx.Done() 路径不仅要写 audit，还要覆盖飞书卡片防 UI 误导
case <-ctx.Done():
    finalDecision = EscapeDecision{
        Action: EscapeForceExit, Reason: "ctx_cancelled", AuditLevel: 2,
        ExitReason: ExitReasonCtxCancelled, SessionID: loopCtx.SessionID, CreatedAt: time.Now(),
    }
    a.audit.Record(loopCtx, decisions, finalDecision)
    a.resume.Save(loopCtx.SessionID, finalDecision)
    // 关键：同步覆盖飞书卡片（防 user 看到"已升级"但实际已退出）
    a.notifier.SubmitOverrideCard(pendingID, "已强制退出（客户端断开）", []FeishuButton{})

// SubmitUserChoice 已过期时仍写 audit（保留 user 响应记录）
func (a *HumanArbitrator) SubmitUserChoice(pendingID string, choice UserChoice) {
    a.mu.Lock()
    ch, ok := a.pending[pendingID]
    a.mu.Unlock()
    if !ok {
        // 关键：已 expired 也要写 audit（user 响应记录不能丢）
        a.audit.Record(nil, nil, EscapeDecision{
            Action: EscapePendingHuman, Reason: "user_late_response",
            PendingID: pendingID, AuditLevel: 1, CreatedAt: time.Now(),
        })
        return
    }
    select {
    case ch <- choice:
    default:
    }
}
```

## 6. 5 节点接线

**5 个接线点**（Observe 失败 + Plan 失败 + Plan 前 + Execute 失败 + Verify 失败），每个调用 `EscapeEngine.Evaluate(ctx)`：

```go
// 5 节点伪代码（具体见 PR-V5.5）
func (o *Orchestrator) ProcessMessage(ctx context.Context, req ProcessRequest) error {
    // T2 续跑检查：上次 ProcessMessage 升级到 Human 且 session 状态已持久化
    if decision, ok := o.escapeEngine.ResumeSession(req.SessionID); ok {
        return o.applyResumeDecision(ctx, decision)
    }

    // ─── 节点 1: Observe ───
    observe, err := o.observe(ctx, req)
    if err != nil {
        // ★ 接线点 0: Observe 失败
        loopCtx := buildLoopContextFromObserve(req.SessionID, observe)
        decision := o.escapeEngine.Evaluate(loopCtx)
        if terminate, derr := o.processEscapeDecision(decision, err); terminate {
            return derr
        }
        // Observe 失败但未到上限 → continue 路径细分（采纳 review-r3 ISSUE-5 建议）：
        //   case 1: observe != nil 但 Observations == []（AnomalyDetector 全 nil 但函数未返 err）
        //           → 进 for 循环调 o.plan(ctx, observe)，Plan 走默认分支（无 ObservationKind）
        //   case 2: observe == nil（Observe 函数 panic/recover 后返回 nil）
        //           → 进 for 循环调 o.plan(ctx, nil)，Plan 立即失败 → 接线点 1a 触发
        // 两者都不破坏主链路（失败降级已覆盖）
    }

    for {
        // ─── 节点 2: Plan ───
        plan, err := o.plan(ctx, observe)
        if err != nil {
            // ★ 接线点 1a: Plan 失败
            // 关键短路：1a 失败后立即 return，**不再调 1b**（Plan 阶段已结束，1b 无 plan 输入，语义模糊）
            // 采纳 codex review §3.5 建议：1a/1b 不合并但 1a 短路不调 1b
            loopCtx := buildLoopContext(ctx, nil, observe)
            decision := o.escapeEngine.Evaluate(loopCtx)
            return o.processEscapeDecision(decision, err)  // 1a 短路出口
        }

        // ★ 接线点 1b: Plan 前（仅在 1a 成功后执行）
        loopCtx := buildLoopContext(ctx, plan, observe)
        decision := o.escapeEngine.Evaluate(loopCtx)
        if terminate, derr := o.processEscapeDecision(decision, nil); terminate {
            return derr
        }

        // ─── 节点 3: Execute ───
        artifact, err := o.execute(ctx, plan)
        if err != nil {
            // ★ 接线点 2: Execute 失败
            loopCtx.PrevPlanKind = plan.Kind
            decision := o.escapeEngine.Evaluate(loopCtx)
            if terminate, derr := o.processEscapeDecision(decision, err); terminate {
                return derr
            }
            continue  // 非终止决策 → 重新规划
        }

        // ─── 节点 4: Verify ───
        verdict := o.verify(ctx, artifact, plan)
        if verdict.Kind == VerdictFail || verdict.Kind == VerdictIndeterminate {
            // ★ 接线点 3: Verify 失败
            loopCtx.FailureCriterion = verdict.Reason
            decision := o.escapeEngine.Evaluate(loopCtx)
            if terminate, derr := o.processEscapeDecision(decision, nil); terminate {
                return derr
            }
            continue  // 非终止决策 → 重新规划
        }

        // Verify 通过 → 跳出回路（EscapeContinue 在 processEscapeDecision 内已处理）
        return nil
    }
}

// processEscapeDecision 统一处理 6 类 EscapeAction
// 返回 (terminate, err)：
//   terminate=true  → caller 应 return err
//   terminate=false → caller 应 continue 回路（仅 EscapeContinue）
func (o *Orchestrator) processEscapeDecision(decision EscapeDecision, baseErr error) (terminate bool, err error) {
    switch decision.Action {
    case EscapeContinue:
        return false, nil  // 继续回路（caller 重新跑 Plan）
    case EscalateToRule, EscalateToHuman:
        // 经 ChainedArbitrator 链式裁决后已收敛；若 Evaluate 直接返回，兜底为 ForceExit
        return true, errors.Join(baseErr, newExitError(decision.Reason+"_unhandled_escalation"))
    case EscapePendingHuman:
        // Human 异步路径：session 状态已持久化，等下次 ProcessMessage 续跑
        return true, nil
    case EscapeForceExit, EscapeAbortWithAudit:
        if baseErr != nil {
            return true, errors.Join(baseErr, newExitError(decision.Reason))
        }
        return true, newExitError(decision.Reason)
    default:
        return true, newExitError("unknown_escape_action")
    }
}
```

**接线点清单**（5 个）：

| # | 触发点 | LoopContext 关键字段 | 失败降级 |
|---|--------|----------------------|---------|
| 0 | Observe 失败 | SessionID + ObservationKind | slog.Warn + Continue（observe 仍可部分用） |
| 1a | Plan 失败 | SessionID + PrevPlanKind=nil | slog.Warn + 决策 |
| 1b | Plan 前 | SessionID + PlanKind + PrevPlanKind + ObservationKind | Evaluate error → Continue |
| 2 | Execute 失败 | PrevPlanKind=plan.Kind | Evaluate error → Continue |
| 3 | Verify 失败 | FailureCriterion=verdict.Reason | Evaluate error → Continue |

**关键修复（相对 v4 旧版本）**：
- **Plan 失败不再裸 return**：加 EscapeEngine 升级机会（关键漏洞 — Plan 失败可能正是 PlanKind 切换时机）
- **Observe 失败加接线点 0**：覆盖 AnomalyDetector 5 nil 等异常检测
- **break 改为 continue + return**：EscapeContinue = "继续回路"应回到 for 顶部重跑 Plan，不是 break
- **统一 processEscapeDecision 函数**：解决嵌套 scope 问题，全 6 类 Action 分支
- **Verify 通过显式 return**：避免"for 循环无 break 出口"歧义

**T2 续跑入口**（见 §5.3.2）：`ResumeSession` 在 ProcessMessage 开头检查；命中 → `applyResumeDecision` 恢复状态；未命中 → 走完整 5 节点流程。

## 7. CircuitBreaker 5 层接线（基于现有 5 metrics）

| 层 | doc 38 §21 SoT | devrix 现有 metric | 触发条件 | 阈值推导（采纳 codex review §3.4 建议，V5.1 阶段写占位推导）|
|----|----------------|-------------------|----------|------|
| L0 | AnomalyDetector 5 nil | `anomaly_detector_nil_total` | 5 次连续 | **占位**：连续 5 次 nil 表示异常检测彻底失效，必须升级（doc 38 §18.2.1 P0 盲点修补已用此阈值）；待 V5.5 集成测试后回填实际数据 |
| L1 | (新增) | `dispatch_loop_wakeups_total` | 100 次/分钟 | **占位**：基于 doc 38 §21.13 诚实声明"估计值，需根据实际场景调优"；100/min 是 P99 估算 × 1.5 安全系数；待 V5.5 查 Prometheus 历史 P99 后回填 |
| L2 | Verifier 3 > 2s | `verifier_latency_p95_seconds` | 3 次 > 2s | **占位**：3 次 > 2s 触发降级，对齐 doc 38 §18.2.2 LLM 调用延迟阈值；2s = devrix SLO P95 目标 |
| L3 | Hook 5 fail | `hook_failure_total` | 5 次连续 | **占位**：与 L0 AnomalyDetector 同模式（连续 5 次失败 = 系统性问题）；doc 38 §18.2.3 已用此阈值 |
| L4 | (新增) | `worker_panics_total` | 1 次 panic | **占位**：worker panic 是严重错误（devrix §2 §errors.go panic 协议规定 panic = 立即升级）；单次 panic 即可触发，避免 panic 累积掩盖根因 |
| L5 | (新增) | `sandbox_exit_failed_total` | 5 次连续 | **占位**：与 L3 Hook 阈值对齐（doc 38 §18.2.3 沙箱失败阈值） |

**devrix 现有 5 metrics 中保留**（采纳 codex review §3.4 澄清）：
- `state.cancels` / `state.handles` 是**状态追踪**不是故障指标（per devrix observability 命名规范），不升级为 circuit breaker
- 升级为 CB 的 5 metrics 选自"故障/异常"语义明确的指标

**阈值分两阶段校准**（采纳 codex review §3.4 建议）：
- **V5.1 阶段（短期）**：使用 doc 38 §21.13 占位推导（如上表"阈值推导"列），标注"占位"
- **V5.5 集成测试后（长期）**：查 devrix Prometheus 历史 P99 / P95，回填正式推导
  - 具体查询：`sum(rate(dispatch_loop_wakeups_total[5m]))` P99（建议查询脚本见 `scripts/cb-threshold-calibrate.sh`，V5.5 产出）

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

## 9. 失败降级策略（M1 完整覆盖）

| 失败点 | 降级行为 | 原因 |
|--------|---------|------|
| EscapeEngine.Evaluate panic | recover + slog.Warn + EscapeContinue | 不阻塞主链路 |
| EscapeEngine.Evaluate error | slog.Warn + EscapeContinue | 监控缺失但不影响流程 |
| buildLoopContext 失败（plan/observe 为 nil）| slog.Warn + default LoopContext + Continue | 降级但可继续 |
| **CircuitBreaker 拉 metric 阻塞** | 200ms timeout + slog.Warn + 不触发 | 避免主链路卡住 |
| **CircuitBreaker.Evaluate panic** | recover + slog.Warn + 不触发 | 监控缺失但不影响流程 |
| **audit.Record 失败** | slog.Warn + fail-open | 不阻塞主流程；audit 丢失但决策应用 |
| LLMArbitrator timeout (5s) | ForceExit 兜底 | 避免无限等待 LLM |
| **LLMArbitrator 返回非法 Action**（>5）| slog.Warn + ForceExit | 防意外枚举值污染回路 |
| **LLMArbitrator 返回非 JSON** | 1 次重试 + 仍失败 → ForceExit | LLM 偶发格式错 |
| **LLM 5s timeout + ctx 取消同时发生** | 优先 ctx 取消 → ForceExit(reason=ctx_cancelled) | ctx 取消语义优先级最高 |
| HumanArbitrator 不响应 | 10s 后 ForceExit 兜底 | 同 Phase 7 Auto-Close |
| **PendingResolutionStore.Save 失败** | slog.Warn + audit 仍写（仅 session 续跑丢失）| audit 优先，续跑可降级 |
| **ResumeSession.Load 失败** | slog.Warn + 走完整 5 节点 | 续跑失败不阻塞 |

## 10. References

- doc 38 §21（行 3621-4025，400 行 v5 完整设计）
- doc 38 §21.5 完整流程图
- doc 38 §21.3.2 LoopDepthTracker v2 Go 骨架
- doc 38 §21.3.3 EscapeAction 5 类 Go 骨架
- doc 38 §21.3.4 ChainedArbitrator 3 层 Go 骨架
- doc 38 §21.4.2 PlanKindSwitchPolicy Go 骨架
- 9 个 MUPS v4 归档
- `openspec/specs/d7-orchestration/pipeline-architecture.md`
