---
reviewer: codex
review-date: 2026-06-24
demand-id: DM-20260625-003
soT-ref: brain/.../core-concepts/38-mature-uncertainty-methodology.md §21 (doc 38 v9)
reviewed-files:
  - demand.md (~100 行)
  - proposal.md (~200 行)
  - design.md (691 行)
  - tasks.md (296 行)
---

# Codex Review — devrix-d7-mups-v5-escape-engine (DM-20260625-003)

> **本评审独立于 doc 38 §22**（那里的内容是同一份 review，但放在 SoT 文档里便于追溯；这里是 devrix 需求目录的本地副本，便于 S3-Gate 评审时直接查阅）。

## 1. 总体判断

**需求整体质量极高，可直接进入 S3-Gate**。但有 3 个可落地的具体建议（合计 2 天工作量）应在 PR-V5.1 review fixes 阶段合并实施。

## 2. 5 个赞同点

### 2.1 赞同 1：与 doc 38 §21 完整对应

需求的设计**直接引用** doc 38 §21 的 400 行 v5 完整设计。proposal §3 的 17 项差距分析显示：

| 现状 | v5 修补 |
|------|--------|
| ❌ 回路深度计数器 | ✅ LoopDepthTracker v2 |
| ❌ PlanKind 切换计数 | ✅ PlanKindSwitchPolicy 3 档 |
| ⚠️ 14 ExitReason 隐式存在 | ✅ CompensationAction 5 类抽象 |
| ❌ 4 类深度限制未统一 | ✅ EscapeEngine 整合入口 |
| ❌ LLM/Rule/Human 仲裁 | ✅ ChainedArbitrator |
| ❌ EscapeAction 5 类 | ✅ 新增 6 类（含中间态）|

### 2.2 赞同 2：HumanArbitrator 异步化（M2 关键创新）

doc 38 §21 只说"3 层仲裁"，但没解决 `ProcessMessage` 同步约束。devrix 落地时**创造性地**设计了：

```
ProcessMessage 同步返回（飞书卡片立即显示）
  ↓
HumanArbitrator 注册 pendingID + 异步通知 + 启动 goroutine
  ↓
立即返回 EscapePendingHuman（中间态）
  ↓
后台 goroutine 等待（10s timeout）
  ├─ user 点 A → 写 audit + session 状态
  ├─ timeout → ForceExit 兜底
  └─ ctx.Done() → ForceExit + 同步覆盖飞书卡片
  ↓
下次 ProcessMessage 进入 → ResumeSession 续跑
```

**关键设计决策**：
- D1：同步返回 + 内部异步
- D2：audit 持久化 + 下次 ProcessMessage 续跑
- D3：可插拔 Notifier（FeishuCard/CLI/Email）

### 2.3 赞同 3：depth 跨 ProcessMessage 续跑（M4 明确）

> `LoopDepthTracker` 内部按 `SessionID` 维度保留 History map（不随 ProcessMessage 结束清空）

**避免"重置 depth 让回路无限续命"的漏洞**——这是 doc 38 §21 没明确的关键设计。

### 2.4 赞同 4：7 类 UI/状态一致性兜底（M2 详细）

| 场景 | 兜底机制 |
|------|---------|
| ctx.Done() 后飞书卡片已发 | 同步覆盖卡片"已强制退出" |
| SubmitUserChoice 已过期 | 仍写 audit（保留 user 响应记录）|
| LLMArbitrator panic | recover + ForceExit |
| Notifier 失败 | 链式 fallback |
| ctx 取消 vs LLM timeout 同时 | 优先 ctx 取消语义 |

### 2.5 赞同 5：失败降级矩阵（§9 M1 完整覆盖）

13 类失败点 + 各自降级行为，比 doc 38 §21 更工程化。

## 3. 5 个担心点

### 3.1 担心 1：devrix 是否需要 §21 这么复杂的 EscapeEngine？

**问题**：doc 38 §21 设计时基于"未来 LLM 可能操纵"，但 devrix MUPS v4 已实现的 5 节点管道（Phase 1-7）里**LLM 操纵风险是真实存在的吗**？

**反问**：
- Phase 4 的 14 ExitReason 是否已足够？
- Phase 7 的 Auto-Close 是否已处理"回路兜底"？
- Phase 6 的 `buildObserveRequest` 3 层 fail-safe 是否已覆盖？

**proposal §3 的差距分析**把 14 ExitReason + 5 metrics + Auto-Close 都列为"⚠️ 隐式存在"，但没说明**冲突关系**——v5 是要取代这些还是叠加？

**建议**：proposal.md §3 应增加"不实施 v5 的备选方案对比"——为什么 6.5 天投入是值得的？需要明确列出"没有 v5 会导致哪些 bug 频发 / 哪些场景无法支持"。

### 3.2 担心 2：6 类 EscapeAction 设计冗余

doc 38 §21 是 **5 类**（Continue/EscalateToRule/EscalateToHuman/ForceExit/AbortWithAudit）。devrix 加了第 6 类 `EscapePendingHuman`——合理（devrix Human 异步）。

但 `EscalateToRule` 和 `EscalateToHuman` **实际上不会单独存在**——`ChainedArbitrator` 链式调用时，下层仲裁一定会把决策收敛（不可恢复 → AbortWithAudit，可恢复 → HumanArbitrator）。

**疑问**：`EscalateToRule` 和 `EscalateToHuman` 作为 EscapeAction 枚举值是否必要？还是只用 `Continue / PendingHuman / ForceExit / AbortWithAudit` 4 类更清晰？

### 3.3 担心 3：LoopContext 11 字段冗余

```go
type LoopContext struct {
    // 7 核心字段（参与 hashLoopContext）
    SessionID           string
    PlanKind            PlanKind
    PrevPlanKind        PlanKind         // ← 不参与 hash
    ObservationKind     ObservationKind
    FailureCriterion    string
    ArtifactType        ArtifactType
    PlanKindSwitchCount int              // ← 不参与 hash

    // 4 状态字段（不参与 hash）
    LoopBudgetState     LoopBudgetState
    ReputationEvidence  *ReputationEvidence  // ← v5 不调？
    ExitReason          ExitReason
    CircuitBreakerState map[CBLevel]bool
}
```

**问题**：
- `PrevPlanKind` + `PlanKindSwitchCount` 是为 PlanKindSwitchPolicy 服务的，应该在 policy 模块，不应放在 LoopContext
- `ReputationEvidence` 在 v5 中实际不被使用（Phase 5 已有 Bayesian 累积）
- `CircuitBreakerState` 是 EscapeEngine 内部状态，不应作为 LoopContext 输入

**建议**：精简 LoopContext 为 7 核心字段（hash 输入），其他状态由 EscapeEngine 内部维护。

### 3.4 担心 4：CircuitBreaker 5 层阈值不严谨

```go
| L1 | (新增) | dispatch_loop_wakeups_total | 100 次/分钟 |
| L4 | (新增) | worker_panics_total | 1 次 panic |
| L5 | (新增) | sandbox_exit_failed_total | 5 次连续 |
```

**问题**：
- `dispatch_loop_wakeups_total` 100 次/分钟是**任意值**——为什么不是 50 或 200？没有推导
- `worker_panics_total` 1 次 panic 就触发——是不是过于敏感？单次 panic 可能可恢复
- `state.cancels` / `state.handles` 被排除，但没说为什么

**建议**：每个 metric 的阈值需要有推导依据（基于历史数据 + SLO），不能是任意值。design §7 应增加阈值推导。

### 3.5 担心 5：5 个接线点的"重复 Evaluate"

design §6 列出 5 个接线点：

```
★ 接线点 0: Observe 失败
★ 接线点 1a: Plan 失败
★ 接线点 1b: Plan 前
★ 接线点 2: Execute 失败
★ 接线点 3: Verify 失败
```

**问题**：接线点 1a（Plan 失败）和 1b（Plan 前）可能连续触发——同一个 ProcessMessage 内调两次 Evaluate，且 LoopContext 几乎相同。

**建议**：评估是否需要"接线点合并"——例如把 1a/1b 合并为"Plan 阶段"单一 Evaluate，或在 1a 后短路（不再调 1b）。

## 4. 与 doc 38 §21 的对比

| doc 38 §21 内容 | design.md 落地 | 评估 |
|----------------|--------------|------|
| §21.1 4 类深度限制整合 | LoopDepthTracker + CircuitBreaker 5 层 | ✅ 完整 |
| §21.2 关键漏洞（PlanKind 切换绕过）| PlanKindSwitchPolicy 3 档 | ✅ 完整 |
| §21.3 v5 完整设计 | ChainedArbitrator + EscapeEngine | ✅ 完整 |
| §21.4 PlanKind 切换 vs DenialBudget 协同 | §5.2 表格 + 集成 | ⚠️ 部分 |
| §21.5 完整流程图 | §5.3.2 决策流程（含 Human 异步）| ✅ 完整（devrix 扩展）|
| §21.8 实施工作量 | proposal §4 5 PR 拆分 | ✅ 一致 |

**核心偏差**：LoopContext 11 字段在 doc 38 §21 是 **7 核心**（hash 输入），devrix 扩展到 11 字段（增加 4 个状态字段）——这个偏差是否必要？

## 5. 8 个 AC 验证清单

| AC | 内容 | 评估 |
|----|------|------|
| AC1 | 按模式 hash 计数 | ✅ doc 38 §21.3.2 直接对应 |
| AC2 | 3 档策略 + 切换计数 ≤4 | ✅ doc 38 §21.4.2 直接对应 |
| AC3 | 3 层仲裁 + 6 类 EscapeAction | ✅ 含新增中间态 EscapePendingHuman |
| AC4 | 3 类深度限制整合 | ✅ LoopDepthTracker + LoopBudget + CircuitBreaker |
| AC5 | 5 节点完整接线 | ✅ 5 接线点 + processEscapeDecision 统一处理 |
| AC6 | PlanKind 切换累计 ≤4 强制 ForceExit | ✅ §5.2 表格 + 集成测试 |
| AC7 | CircuitBreaker 5 层接线 | ⚠️ 阈值缺推导 |
| AC8 | 单元测试 100% PASS + 集成测试 + 0 race | ✅ 标准 devrix 验收流程 |

## 6. 与现有沉淀的关系

| 沉淀 | 与 v5 关系 |
|------|----------|
| doc 38 §21（v5 完整设计）| ✅ v5 需求直接引用 §21 的 400 行设计 |
| doc 38 §19.2（DenialBudget）| ✅ LoopBudget 复用 §19.2 的 consecutive=3, total=20 |
| doc 38 §19.4（ToolSurface P1）| ❌ v5 没纳入（V5+ 候选）|
| doc 38 §18（v4 P0 盲点修补）| ✅ Phase 1-7 已落地 |
| Phase 1-7 5 节点数据契约 | ✅ 零破坏性变更 |

## 7. 3 个具体 Review 建议

### 建议 1：简化 LoopContext 字段（针对担心 3）

**位置**：design.md §4

**修改**：
```go
// 精简为 7 核心字段（hash 输入）
type LoopContext struct {
    SessionID        string
    PlanKind         PlanKind
    PrevPlanKind     PlanKind  // 仅用于 PlanKindSwitchPolicy
    ObservationKind  ObservationKind
    FailureCriterion string
    ArtifactType     ArtifactType
    PlanKindSwitchCount int   // 仅用于 PlanKindSwitchPolicy
}
// 其他状态（LoopBudgetState / ReputationEvidence / ExitReason / CircuitBreakerState）
// 由 EscapeEngine 内部维护，不作为 LoopContext 输入
```

**工作量**：0.5 天，design 修改 + 单元测试调整。

### 建议 2：增加"不实施 v5 的备选方案对比"（针对担心 1）

**位置**：proposal.md §3 末尾或新 §3.4

**内容**：
- 列出 5-7 个"没有 v5 会发生的具体 bug"
- 例如："LoopDepthTracker 缺失 → LLM 切换 Plan.Kind 绕过回路深度计数（doc 38 §21.2 关键漏洞）"
- 例如："CircuitBreaker 5 层缺失 → 已知 5 metrics 未作为 circuit breaker 触发，仅为纯 metric"
- 例如："HumanArbitrator 缺失 → ProcessMessage 同步返回被 10s 阻塞，飞书卡片体验崩溃"

**工作量**：0.5 天，proposal 加 1 节。

### 建议 3：CircuitBreaker 阈值推导（针对担心 4）

**位置**：design.md §7 增加"阈值推导"

**内容**：
- `dispatch_loop_wakeups_total` 100 次/分钟 ← 基于历史 P99 × 1.5 安全系数
- `worker_panics_total` 1 次 panic ← 单次 panic 已严重（worker 是无状态可恢复的）
- `sandbox_exit_failed_total` 5 次连续 ← 与 Hook 失败阈值对齐（doc 38 §18.2.3）

**工作量**：1 天，需要查 devrix 实际历史数据。

## 8. 实施路径建议

3 个建议合并为 PR-V5.1 review fixes（在 V5.1 提交前先修）：

| 子任务 | 工作量 | 来源 |
|------|--------|------|
| 简化 LoopContext | 0.5 天 | 建议 1 |
| proposal §3.4 备选对比 | 0.5 天 | 建议 2 |
| CircuitBreaker 阈值推导 | 1 天 | 建议 3 |
| **合计** | **2 天** | V5.1 之前完成 |

不影响 V5.1-V5.5 主流程（6.5 天）。

## 9. 5 句话总结

1. **需求整体质量极高**——5 赞同（与 doc 38 §21 完整对应 + HumanArbitrator 异步化 + depth 续跑 + UI 兜底 + 失败降级）
2. **HumanArbitrator 异步化是 devrix 真正的工程创新**——doc 38 §21 没解决同步约束，devrix 创造性地用 EscapePendingHuman 中间态 + audit 持久化 + ResumeSession 续跑
3. **5 个担心聚焦"落地特定问题"**——LoopContext 11 字段冗余 / 6 类 EscapeAction 设计冗余 / CircuitBreaker 阈值缺推导 / 5 接线点重复 Evaluate / 不实施 v5 备选对比缺失
4. **3 个具体建议**都是工程可落地的——简化 LoopContext（0.5 天）/ 加不实施对比（0.5 天）/ 加阈值推导（1 天）
5. **整体建议**：需求可直接进入 S3-Gate，3 个建议合并为 PR-V5.1 review fixes（2 天工作量，不影响 5 PR 主流程）

---

## References

- `demand.md`
- `proposal.md`
- `design.md`
- `tasks.md`
- `brain/.../core-concepts/38-mature-uncertainty-methodology.md §21`（400 行 v5 完整设计）
- `brain/.../core-concepts/38-mature-uncertainty-methodology.md §22`（codex review 完整版，独立于此文件）
