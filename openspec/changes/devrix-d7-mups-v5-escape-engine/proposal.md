# Proposal — devrix-d7-mups-v5-escape-engine (DM-20260625-003)

**Change ID:** `devrix-d7-mups-v5-escape-engine`
**Demand ID:** DM-20260625-003
**Priority:** P0
**Sprint:** mups-v5
**Estimated Effort:** 6.5 天
**PR Count:** 5
**SoT:** `brain/.../core-concepts/38-mature-uncertainty-methodology.md §21` (400 行 v5 完整设计)

---

## 1. 目标

实现 MUPS v5 回路深度统一逃逸机制（EscapeEngine），解决 doc 38 §21.2 关键漏洞：

> **当所有回路都失败、所有 budget 都耗尽、所有 circuit breaker 都 open 时，系统的兜底行为是什么？**
>
> 当前设计 4 类深度限制（回路深度=3 / DenialBudget / CircuitBreaker 5 层 / CompensationAction 5 类）分散在多个沉淀中未统一；**真正的漏洞是 LLM 可通过切换 Plan.Kind 绕过回路深度计数**。
>
> v5 设计 = **分层逃逸 + 5 类兜底 + LLM/Rule/Human 三层仲裁**。

## 2. 范围

### 2.1 包含

- LoopDepthTracker v2（按"模式 hash"计数）
- PlanKindSwitchPolicy 3 档（Constrained/Allowed/Forbidden）
- ChainedArbitrator (LLM/Rule/Human) 3 层仲裁
- EscapeEngine 统一逃逸入口
- EscapeAction 5 类（Continue/EscalateToRule/EscalateToHuman/ForceExit/AbortWithAudit）
- CircuitBreaker 5 层接线（基于现有 5 metrics）
- 5 节点 EscapeEngine 接线（每个补偿动作前调用）

### 2.2 不包含

- 5 节点管道本身（Phase 1-7 已 S7_Archived）
- Plan 嵌套 Plan（不在 doc 38 §21 范围）
- 跨 turn Learn 注入（Phase 6-7 已 S7_Archived）
- Doc 38 §18 P0 盲点（已落地 v4）
- Doc 38 §19 Clawcode 借鉴（ForkMode / Denial 追踪 / ToolSurface 等，V5+ 候选）

## 3. SoT vs devrix 实际实现差距分析

| # | SoT（doc 38 §21） | devrix 实际（Phase 1-7 + review-fixes） | 差距 | 优先级 |
|---|-------------------|------------------------------------------|------|--------|
| 1 | **回路深度 = 3**（按轮数） | ❌ 缺 | 计数器缺 | P0 |
| 2 | **回路深度 v2 按模式 hash** | ❌ 缺 | 计数器 v2 缺 | P0 |
| 3 | **Plan.Kind 切换计数 ≤ 4** | ❌ 缺 | 切换计数缺 | P0 |
| 4 | **PlanKindSwitchPolicy 3 档** | ❌ 缺 | 策略缺 | P0 |
| 5 | **DenialBudget (consecutive=3, total=20)** | ⚠️ 14 ExitReason（结果是 outcome 不是 budget） | budget 缺 | P0 |
| 6 | **CircuitBreaker 5 层**（L0 AnomalyDetector / L2 Verifier / L3 Hook） | ⚠️ 5 metrics 已有（dispatch_loop_wakeups / worker_panics / state.cancels / state.handles / sandbox_exit_failed）但**没作为 circuit breaker 触发** | 触发器缺 | P0 |
| 7 | **CompensationAction 5 类**（Retry/Reobserve/Replan/AcceptFailure/ExitWithReason） | ⚠️ Phase 6 Auto-Close + Phase 4 14 ExitReason 隐式存在 | 抽象缺 | P0 |
| 8 | **EscapeAction 5 类**（Continue/EscalateToRule/EscalateToHuman/ForceExit/AbortWithAudit） | ❌ 缺 | 缺 | P0 |
| 9 | **EscapeEngine 统一入口** | ❌ 缺（5 节点各自决策） | 缺 | P0 |
| 10 | **LLM/Rule/Human 3 层仲裁** | ❌ 缺（只 LLM 单层） | 缺 | P0 |
| 11 | **ChainedArbitrator** | ❌ 缺 | 缺 | P0 |
| 12 | **AuditLevel 0/1/2** | ❌ 缺 | 缺 | P1 |
| 13 | **5 节点数据契约** | ✅ 已有（Phase 1-3 + Phase 5） | — | — |
| 14 | **Verify 4 态** | ✅ 已有（Phase 4） | — | — |
| 15 | **3 Memory 通道** | ✅ 已有（Phase 5：Skill/Feedback/Scheduled） | — | — |
| 16 | **LP-1 闭环** | ✅ 已有（Phase 6） | — | — |
| 17 | **Auto-Close → Learn** | ✅ 已有（Phase 7） | — | — |

**结论**：P0 缺 11 项 + P1 缺 1 项 = **12 项落地**，5 PR 6.5 天。

### 3.4 不实施 v5 的备选方案对比（采纳 codex review §3.1 建议）

**问题**：6.5 天工作量是否值得？需要明确"没有 v5 会发生什么具体 bug"。

**7 个具体 bug 场景**（按发生频度排序）：

| # | Bug 场景 | 没有 v5 的后果 | 频度估计 |
|---|---------|--------------|---------|
| 1 | **LLM 切换 Plan.Kind 绕过回路深度计数**（doc 38 §21.2 关键漏洞）| 回路无限循环，单 ProcessMessage 消耗 token 100k+，飞书卡片超时 | 中（LLM 操纵真实存在）|
| 2 | **同一模式重复 4 次仍继续**（LoopDepthTracker 缺失）| 资源耗尽、计费爆炸、用户体验卡死 | 中（Verify 失败时常见）|
| 3 | **AnomalyDetector 连续 5 次 nil 不触发升级**（CB L0 缺失）| 异常被默默吞掉，下游节点持续异常输入 | 高（异常检测是常态）|
| 4 | **Verifier p95 > 2s 连续 3 次不触发降级**（CB L2 缺失）| 用户等待超时，飞书卡片无反馈 | 中（高并发时常见）|
| 5 | **HumanArbitrator 同步等待 10s 阻塞 ProcessMessage**（同步约束违反）| 飞书卡片体验崩溃（已显示但实际卡 10s）| 高（每次升级到人工都触发）|
| 6 | **PlanKindSwitchPolicy 缺失导致 LLM 反复切换 Plan 类型**（doc 38 §21.4.2 关键漏洞）| LLM "试探模式"无限循环 | 中（探索型 Plan 常见）|
| 7 | **CircuitBreaker 与现有 5 metrics 重叠无升级触发**（CB 缺失）| 已知 5 metrics 仅作 metric 展示，无 circuit breaker 保护 | 高（dispatch_loop_wakeups 等高频）|

**不实施 v5 的备选方案**（评估 4 个）：

| 备选方案 | 工作量 | 缺点 | 评估 |
|---------|--------|------|------|
| **A. 维持现状**（依赖 Phase 1-7 + Auto-Close）| 0 天 | 5 个 P0 bug 不解决（场景 1/2/5/6/7）| ❌ 不可接受 |
| **B. 仅实现 LoopDepthTracker v2 + 同步 HumanArbitrator** | 2 天 | 解决 3 个 bug（1/2/6），但飞书卡片体验差（5），CB 缺失（3/4/7）| ⚠️ 部分缓解 |
| **C. 实现 v5 但砍掉异步 Human + CB 5 层** | 3 天 | 解决 4 个 bug（1/2/5/6），但 CB 缺失（3/4/7）| ⚠️ 部分缓解 |
| **D. 完整 v5**（本方案）| 6.5 天 | 解决 7 个 bug | ✅ 推荐 |

**结论**：6.5 天投入可解决 7 个 P0 bug，平均每个 bug 0.93 天，远低于"单独修"的累计成本（每个 1-2 天 + 协调成本）。

### 3.5 v5 与 Phase 1-7 是叠加关系（采纳 codex review §3.1 澄清）

**关键澄清**：v5 **不取代** Phase 1-7 任何已落地能力，而是**叠加**统一逃逸层。

| 现有沉淀 | v5 关系 | 协同方式 |
|---------|---------|---------|
| **Phase 4 14 ExitReason** | 保留为底层事实，v5 的 EscapeAction 5 类是上层抽象 | EscapeDecision.ExitReason 字段映射 14 类 |
| **Phase 6 buildObserveRequest 3 层 fail-safe** | 保留，v5 的 Evaluate error 降级为同一模式 | Evaluate panic → recover + Continue（同 fail-safe 模式）|
| **Phase 7 Auto-Close** | 保留，v5 的 HumanArbitrator 异步化复用 Auto-Close 模式 | "同步返回 + 内部异步" 是 Auto-Close 已有模式，v5 复用 |
| **Phase 4 14 ExitReason 隐式存在** | 升级为 CompensationAction 5 类抽象（V5.5）| ExitReason → CompensationAction 映射表 |
| **CircuitBreaker 与 5 metrics** | 显式选择 5 个升级为 circuit breaker，其余保留为纯 metric | 5 metrics 中 L0/L1/L2/L3/L4/L5 升级，其余 state.cancels/handles 保留 |

**结论**：v5 复用 Phase 1-7 全部数据契约，**零破坏性变更**（design §8 已声明，本节强化"叠加"语义）。

## 4. PR 拆分

### 4.1 PR-V5.1: LoopDepthTracker v2（1 天）

**内容**：
- `internal/layers/orchestration/escape/loop_depth_tracker.go`（NEW）
- `LoopContext` struct（7 字段：5 hash 输入 + 2 状态；PrevPlanKind/PlanKindSwitchCount 移到 policy 模块）
- `LoopDepthTracker` struct（MaxDepth=3 + History map + LoopBudget + SessionID）
- `hashLoopContext(ctx) string` SHA-256 算模式 hash
- `ShouldContinue(ctx LoopContext) EscapeDecision`

**AC 范围**：AC1（按模式 hash 计数）+ 单元测试
**依赖**：doc 38 §19.2 DenialBudget 概念
**风险**：低，纯新增模块

### 4.2 PR-V5.2: PlanKindSwitchPolicy（0.5 天）

**内容**：
- `internal/layers/orchestration/escape/plan_kind_switch_policy.go`（NEW）
- `PlanKindSwitchPolicy` enum（SwitchAllowed / SwitchConstrained / SwitchForbidden）
- `determineSwitchPolicy(planKind PlanKind) PlanKindSwitchPolicy` 决策函数
- 集成到 `plan/planner.go` 的 `MatchKind` 之后
- 切换计数累加（max=4）

**AC 范围**：AC2（3 档策略 + 切换计数 ≤4）+ 单元测试
**依赖**：PR-V5.1
**风险**：低，策略函数

### 4.3 PR-V5.3: ChainedArbitrator（2 天）

**内容**：
- `internal/layers/orchestration/escape/arbitrator.go`（NEW）
- `EscapeAction` enum（5 类）
- `EscapeDecision` struct（Action + Reason + AuditLevel + Depth）
- `EscapeArbitrator` interface
- `LLMArbitrator`（self-decide，5s timeout 兜底 ForceExit）
- `RuleArbitrator`（规则强制，不可恢复失败 → AbortWithAudit）
- `HumanArbitrator`（用户接管，A/B/C 选项）
- `ChainedArbitrator`（LLM → Rule → Human 链式调用）

**AC 范围**：AC3（3 层仲裁）+ 单元测试
**依赖**：PR-V5.1
**风险**：中，LLM timeout 兜底 + 人工接管异步化

### 4.4 PR-V5.4: EscapeEngine 整合（1 天）

**内容**：
- `internal/layers/orchestration/escape/engine.go`（NEW）
- `EscapeEngine` struct（tracker + chain + auditLog + circuitBreaker）
- `Evaluate(ctx LoopContext) EscapeDecision` 整合入口
- `CircuitBreaker` 5 层接线（基于现有 5 metrics）：
  - L0 AnomalyDetector (5 nil) → open
  - L2 Verifier (3 > 2s) → open
  - L3 Hook (5 fail) → open
  - L4/5 见 `circuit_breaker.go`
- `EscapeAuditLog`（AuditLevel 0/1/2）

**AC 范围**：AC4（3 类深度限制整合）+ AC7（CircuitBreaker 5 层接线）+ 单元测试
**依赖**：PR-V5.3
**风险**：中，与现有 5 metrics 重叠需谨慎选择

### 4.5 PR-V5.5: 5 节点接线 + 集成测试（2 天）

**内容**：
- 5 节点完整接线（每个 Plan→Execute→Verify→[Compensation] 前调用 EscapeEngine.Evaluate）
- 单元测试（>20 test functions 100% PASS）
- 集成测试覆盖：
  - 4 类深度限制
  - 3 层仲裁（mock LLM/Rule/Human）
  - 5 类兜底动作（Continue/EscalateToRule/EscalateToHuman/ForceExit/AbortWithAudit）
  - 0 race
- 文档同步：`openspec/specs/d7-orchestration/spec.md` v4.7.0 → v4.8.0
- `openspec/specs/d7-orchestration/t-registry.md` v3.15.0 → v3.16.0

**AC 范围**：AC5（5 节点接线）+ AC6（PlanKind 切换 ≤4 强制）+ AC8（测试覆盖）+ 文档同步
**依赖**：PR-V5.1 ~ V5.4
**风险**：中，5 节点接线改动面大

## 5. 实施顺序

```
PR-V5.1 (LoopDepthTracker) ─┐
                              ├─→ PR-V5.3 (ChainedArbitrator) ─→ PR-V5.4 (EscapeEngine) ─→ PR-V5.5 (接线+测试)
PR-V5.2 (PlanKindSwitch) ────┘                                                              ↑
                                                                                          │
                                                              (可与 V5.3/V5.4 并行) ─────┘
```

## 6. 兼容性

| 维度 | 影响 | 缓解 |
|------|------|------|
| 5 节点接线 | 每个 Plan→Execute→Verify→Compensation 路径加 Evaluate 调用 | 失败降级：Evaluate error → slog.Warn + 继续（不阻塞主链路） |
| LLM 仲裁 | LLMArbitrator 5s timeout | timeout → ForceExit 兜底 |
| Human 仲裁 | 异步等待用户输入 | 不阻塞 ProcessMessage 同步返回（同 Phase 7 Auto-Close 模式） |
| CircuitBreaker 5 层 | 与现有 5 metrics 重叠 | 显式选择哪些 metric 升级为 circuit breaker，保留其余为纯 metric |
| 现有 14 ExitReason | 隐式存在但未结构化 | V5.5 改造为 CompensationAction 5 类 + 保留 14 ExitReason 映射 |

## 7. 测试策略

- **单元测试**：每个 PR 独立 100% PASS
- **集成测试**（V5.5）：
  - 4 类深度限制 4 scenarios
  - 3 层仲裁 3 scenarios（mock）
  - 5 类兜底动作 5 scenarios
  - LLM 切换 Plan.Kind 累计 ≤4（强制 ForceExit）
  - 0 race
- **覆盖率**：≥ 80%（与现有 baseline 持平）

## 8. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| **R1**：PlanKindSwitchPolicy 阈值（>4）估计值 | 高 | V5.5 加可配置常量 + 集成测试覆盖阈值边界 |
| **R2**：3 层仲裁增加响应延迟 | 中 | LLM 阶段超时 5s 兜底为 ForceExit |
| **R3**：CircuitBreaker 5 层与现有 5 metrics 重叠 | 中 | V5.4 显式选择哪些升级为 circuit breaker |
| **R4**：5 节点接线改动面大 | 中 | V5.5 失败降级：Evaluate error → slog.Warn + 继续 |
| **R5**：Human 仲裁等待用户输入 | 低 | 不阻塞 ProcessMessage 同步返回（同 Phase 7 模式） |

## 9. Out of Scope

- Plan 嵌套 Plan（不在 doc 38 §21 范围）
- Doc 38 §18 P0 盲点（已落地 v4）
- Doc 38 §19 Clawcode 借鉴（V5+ 候选）
- 5 类不确定性合并为 4 类（doc 38 §17 V5+ 候选）
- 工具协议 / 工具 surface（已 v4 落地）
- Compression（已 v4 落地）

## 10. References

- `openspec/changes/devrix-d7-mups-v5-escape-engine/demand.md`
- `openspec/changes/devrix-d7-mups-v5-escape-engine/design.md`
- `openspec/changes/devrix-d7-mups-v5-escape-engine/tasks.md`
- `brain/.../core-concepts/38-mature-uncertainty-methodology.md §21`（行 3621-4025，400 行 v5 完整设计）
- 9 个 MUPS v4 归档（Phase 1-7 + review-fixes）
- `openspec/specs/d7-orchestration/pipeline-architecture.md`
- `openspec/specs/d7-orchestration/spec.md` v4.7.0
- `openspec/specs/d7-orchestration/t-registry.md` v3.15.0
