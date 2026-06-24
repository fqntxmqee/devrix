# D7 Orchestration Spec Delta — MUPS v5 EscapeEngine

**Change ID:** devrix-d7-mups-v5-escape-engine
**Demand ID:** DM-20260625-003
**Delta Type:** MAJOR (v4.7.0 → v4.8.0)
**SOT:** `core-concepts/38-mature-uncertainty-methodology.md §21`

---

## 1. 修改总览

本 change 是 MUPS v4.3 7 个 Phase + 1 review-fixes 全部 S7_Archived 之后，对 5 节点管道**统一逃逸机制**的补全。

不改变 5 节点数据契约，只新增 escape/ 子包 + 接线点 + 文档同步。

| 内容 | 节点 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. LoopDepthTracker v2 | Escape | NEW | 按"模式 hash"计数回路深度，MaxDepth=3 |
| 2. PlanKindSwitchPolicy | Escape | NEW | 3 档策略（Constrained ≤4 / Allowed / Forbidden） |
| 3. EscapeAction 5 类 | Escape | NEW | Continue / EscalateToRule / EscalateToHuman / ForceExit / AbortWithAudit |
| 4. ChainedArbitrator | Escape | NEW | LLM/Rule/Human 3 层仲裁（5s + 10s timeout 兜底）|
| 5. EscapeEngine | Escape | NEW | 整合 3 类深度限制（回路深度 + DenialBudget + CircuitBreaker）|
| 6. CircuitBreaker 5 层 | Escape | NEW | L0 AnomalyDetector / L1 Dispatch / L2 Verifier / L3 Hook / L4 Panic / L5 Sandbox |
| 7. 5 节点接线 | Orchestrator | MODIFIED | Plan 前 + Execute 失败 + Verify 失败 3 个接线点 |
| 8. AuditLog | Escape | NEW | AuditLevel 0/1/2 三档审计 |

## 2. 行为变化详述

### 2.1 回路深度按"模式 hash"计数

**旧行为**：v4 没有显式回路深度计数器。
**新行为**：v5 按 `hash(SessionID:PlanKind:ObsKind:FailureCriterion:ArtifactType)` 计数，同模式重复 depth++，不同模式 reset。MaxDepth=3。

**关键修复**（doc 38 §21.2）：v4 中 LLM 可通过切换 Plan.Kind 绕过回路深度计数。v5 通过"模式 hash"使 Plan.Kind 切换算作新回路（depth=1），但同一模式重复仍会被累积计数。

### 2.2 PlanKindSwitchPolicy 3 档

**新行为**：

| PlanKind | Policy | 切换累计 |
|----------|--------|----------|
| PlanExploration | Constrained | ≤4 |
| PlanScenario | Allowed | 无限制 |
| PlanProtocol | Constrained | ≤4 |
| PlanCommitment | Forbidden | 0（禁止）|

**关键设计**（doc 38 §21.4.2）：
- **ExplorationPlan** 限制切换：避免 LLM 反复尝试不同"探索"角度
- **ScenarioPlan** 允许切换：并行多假设是合法的
- **CommitmentPlan** 禁止切换：一旦承诺就要执行到底

### 2.3 EscapeAction 6 类（5 类正式 + 1 个 dev 扩展中间态）

```
EscapeContinue     → 继续回路
EscalateToRule     → 升级到规则强制
EscalateToHuman    → 升级到人工接管
EscapeForceExit    → 强制退出（带 ExitReason）
EscapeAbortWithAudit → 强制终止 + 完整审计（最严重）
EscapePendingHuman → 中间态：等待用户响应（v5 dev 扩展，决策携带 PendingID）
```

**EscapePendingHuman 中间态**（dev 特有）：
- 由 `HumanArbitrator` 异步通知飞书卡片 + 10s timeout 兜底
- `EscapeDecision.PendingID` 字段填充 pending decision ID
- `ProcessMessage` 同步返回 nil（不阻塞），session 状态持久化
- 下次 `ProcessMessage` 入口检查 `ResumeSession` 命中 → 续跑（详见 design.md §6）

**6 类原因**：devrix `HumanArbitrator` 不能同步等待 10s user 响应（破坏飞书卡片体验），必须异步注册 + 立即返回中间态。这是 v5 dev 相对 doc 38 §21.3.3 原版 5 类的扩展。

### 2.4 3 层仲裁（LLM/Rule/Human）

```
LLMArbitrator (5s timeout)
  ├─ LLM 选 Continue → EscapeContinue
  └─ LLM 选 Exit / timeout → 下层

RuleArbitrator
  ├─ 不可恢复失败 → AbortWithAudit
  └─ 可恢复失败 → EscalateToHuman

HumanArbitrator (10s timeout)
  ├─ A. 继续 → EscapeContinue
  ├─ B. 接受 → EscapeForceExit
  └─ C. 取消 → EscapeAbortWithAudit
```

**devrix 特有**：
- LLM 5s timeout 兜底 ForceExit（不阻塞主链路）
- Human 10s timeout 兜底 ForceExit（异步化，不阻塞 ProcessMessage 同步返回）

### 2.5 CircuitBreaker 5 层接线

| 层 | 来源 | 现有 metric | 触发条件 |
|----|------|------------|----------|
| L0 | AnomalyDetector | `anomaly_detector_nil_total` | 5 次连续 |
| L1 | DispatchLoop | `dispatch_loop_wakeups_total` | 100/min |
| L2 | Verifier | `verifier_latency_p95_seconds` | 3 次 > 2s |
| L3 | Hook | `hook_failure_total` | 5 次连续 |
| L4 | Worker Panic | `worker_panics_total` | 1 次 panic |
| L5 | Sandbox Exit | `sandbox_exit_failed_total` | 5 次连续 |

**保留为纯 metric**：`state.cancels` / `state.handles`（状态追踪不是故障指标）

### 2.6 5 节点接线（3 个接线点）

```go
// 1. Plan 前
loopCtx := buildLoopContext(ctx, plan, observe)
decision := o.escapeEngine.Evaluate(loopCtx)
if decision.Action == EscapeForceExit { return }

// 2. Execute 失败
loopCtx.PrevPlanKind = plan.Kind
decision := o.escapeEngine.Evaluate(loopCtx)

// 3. Verify 失败
loopCtx.FailureCriterion = verdict.Reason
decision := o.escapeEngine.Evaluate(loopCtx)
```

**失败降级**：Evaluate error → slog.Warn + EscapeContinue（不阻塞主链路）

## 3. 兼容性

| 维度 | 影响 | 缓解 |
|------|------|------|
| 5 节点接线 | 加 Evaluate 调用 | 失败降级 slog.Warn + Continue |
| LLM 5s timeout | 增加等待 | 兜底 ForceExit |
| Human 10s timeout | 异步等待 | 不阻塞 ProcessMessage 同步返回 |
| CircuitBreaker 5 层 | 与 5 metrics 重叠 | 显式选择 5 个升级 |
| 现有 14 ExitReason | 隐式存在 | 保留 + 映射到 EscapeAction 5 类 |

## 4. 不在范围内

- 5 节点管道本身（已 S7_Archived）
- Plan 嵌套 Plan（不在 doc 38 §21 范围）
- Doc 38 §18 P0 盲点（已落地 v4）
- Doc 38 §19 Clawcode 借鉴（V5+ 候选）
- 5 类不确定性合并为 4 类（V5+ 候选）

## 5. References

- `openspec/changes/devrix-d7-mups-v5-escape-engine/proposal.md`
- `openspec/changes/devrix-d7-mups-v5-escape-engine/design.md`
- `openspec/changes/devrix-d7-mups-v5-escape-engine/tasks.md`
- `brain/.../core-concepts/38-mature-uncertainty-methodology.md §21`
- 9 个 MUPS v4 归档
- `openspec/specs/d7-orchestration/pipeline-architecture.md`
