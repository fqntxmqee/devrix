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
HumanArbitrator (用户接管 A/B/C)
  ├─ A. 继续 → EscapeContinue
  ├─ B. 接受 → ForceExit
  └─ C. 取消 → AbortWithAudit
```

**devrix 落地差异**：
- LLMArbitrator 5s timeout 兜底 ForceExit（不阻塞主链路）
- HumanArbitrator 异步化（不阻塞 ProcessMessage 同步返回）
- 失败降级：EscapeEngine.Evaluate error → slog.Warn + EscapeContinue

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
