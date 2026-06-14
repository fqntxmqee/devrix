# D7 编排域 S 层重构 — S2 Proposal

**Change ID:** devrix-d7-sa-refine
**Demand ID:** DM-20260614-008
**阶段:** S2 Proposal
**版本:** v1.0

---

## 1. 现状快照

| 指标 | 数值 | 说明 |
|------|------|------|
| S 总数 | 5 | S1–S5 |
| IMPLEMENTED T | 39 | S3/S4 为主 |
| PLANNED S | 1 | S2（主入口） |
| 跨域边界漂移 | 1 | D7-S1 Task 模型在 D2 |

---

## 2. North Star 确认

**D7 = 横向编排层：决定做什么、按什么顺序做、谁来做、做得怎么样了。**

| 承诺 | 博弈含义 |
|------|----------|
| S2 入口可追溯 | D1 路由到 D7 有 span + event_id |
| S5 决策可验证 | ClassifyIntent 结果可观测 |
| S3 并行可预期 | Wave DAG 调度有确定性 |
| S4 进度可观察 | FlowEvent 全链路可达 |

---

## 2.1 博弈论分析

### 2.1.1 D7 的博弈角色

D7 在多方博弈中扮演 **协调者（Mediator）** 而非 **裁判（Judge）**：

```
用户（Principal）→ D7（Mediator）→ D2/D4/D3（Agent）

D7 不评判 Agent 好坏，只保证：
1. 决策过程可观测（ClassifyIntent 有 span）
2. 执行顺序可预期（Wave DAG 有确定性）
3. 结果可追溯（FlowEvent 有 event_id）
```

### 2.1.2 委托代理问题（Principal-Agent Problem）

| 问题 | D7 职责 | 非 D7 职责 |
|------|---------|-----------|
| 用户目标是否被正确理解 | S5 意图分类 | D6 Judge 评判质量 |
| Agent 是否在正确执行 | S3 并行调度 | D6 评判结果 |
| 任务是否完成 | S4 进度广播 | D6 评判完成度 |

**关键洞察：** D7 只保证「编排过程可信」，不保证「编排结果正确」。

### 2.1.3 S2 vs S5 的激励错配风险

**现状问题：** S2 入口 PLANNED（无 T），S5 决策已部分实现。

| 场景 | 开发者局部最优 | 用户全局最优 |
|------|--------------|-------------|
| 先做 S5 | ClassifyIntent 准确 | 但入口不可验证 |
| 先做 S2 | 入口稳定 | 但决策可能错误 |

**切法 A 的博弈价值：**

```
S2（入口确定性）→ S5（决策正确性）→ S3（执行并行性）→ S4（结果可观察性）

激励链：
- S2 有 T 锚点 → D1 路由可验证 → 入口确定性 ✅
- S5 决策可观测 → 分类错误可追溯 → 决策质量可改进
- S3 DAG 调度确定 → 并行冲突可避免 → 执行效率可预期
```

### 2.1.4 Commitment Device：S2 入口 T 锚点

**问题：** S2 无 T 锚点 = D7 可以「声称」处理了消息，但无法验证。

**解决方案：** S2-A01-T01（端到端延迟）+ span 链

```go
// D7.S2 的承诺机制
type ProcessMessageResult struct {
    event_id    string    // D1 可追溯
    elapsed_ms  int64     // D7 测量，Agent 无法伪造
    span_ctx    SpanContext // 全链路可观测
}
```

**博弈含义：** 即使 D7 内部决策错误，外部可通过 `event_id` + `elapsed_ms` 验证入口行为是否合规。

### 2.1.5 信息不对称：S5 决策 vs S3 执行

| 信号 | D7 知道 | 用户知道 |
|------|---------|---------|
| ClassifyIntent 结果 | ✅ | ❌（直到任务完成） |
| Wave 调度决策 | ✅ | ❌（直到进度广播） |
| FlowEvent 时序 | ✅ | ✅（S4 实时广播） |

**缓解：** S4 进度广播（FlowEvent）是 D7 向用户发送的 **成本高昂信号（Costly Signal）**：
- 实时广播 = 消耗资源 = 可信
- 延迟/丢失 = 潜在问题 = 可追溯

### 2.1.6 博弈论视角结论

| 原则 | 在 D7 重构中的映射 |
|------|-------------------|
| 分离均衡 | S5 决策路径与 S2 入口路径分离 |
| 成本信号 | S4 FlowEvent = costly signal（实时广播） |
| 承诺装置 | S2 T 锚点 = 入口行为可验证 |
| 信息不对称缓解 | S4 进度广播 = D7→用户信息透明 |

---

## 2.2 Clawcode 编排设计参考

参考 `clawcode/src/coordinator/coordinatorMode.ts` 与 `clawcode/src/utils/swarm/` 实现。

### 2.2.1 Coordinator 角色定义

```typescript
// clawcode: coordinatorMode.ts
You are a **coordinator**. Your job is to:
- Help the user achieve their goal
- Direct workers to research, implement and verify code changes
- Synthesize results and communicate with the user
- Answer questions directly when possible — don't delegate work that you can handle without tools
```

**关键洞察：Coordinator 不做执行，只协调。**

| 角色 | 职责 | 类比 D7 |
|------|------|---------|
| Coordinator | 理解用户目标、分工、综合结构、对用户说话 | **D7-S2（入口）+ D7-S5（结构决策）** |
| Worker | 执行具体任务（research/implementation/verification） | **D2 QueryLoop + D4 Agent**（执行 Follower） |
| 调度 / 进度 | Phase 并行策略、task-notification | **D7-S3（Wave/ConflictGuard）+ D7-S4（FlowEvent→D1 Task）** |

> **映射修订（Cursor §10.3）：** Worker ≠ D7-S3/S4；S3/S4 是 Mechanism + Costly Signal，执行者在 D2/D4。

### 2.2.2 委托代理分离

```typescript
// Phase 分离（clawcode 的工作流）
| Phase | Who | Purpose |
|-------|-----|---------|
| Research | Workers (并行) | 研究代码库、理解问题 |
| Synthesis | Coordinator | 阅读发现、理解问题、制定规格 |
| Implementation | Workers | 按规格实施变更 |
| Verification | Workers | 测试变更是否有效 |
```

**关键洞察：Synthesis（综合）必须由 Coordinator 做，不能委托给 Worker。**

### 2.2.3 承诺机制：Task 状态机

```typescript
// clawcode: Task.ts
export type TaskStatus = 'pending' | 'running' | 'completed' | 'failed' | 'killed'

export function isTerminalTaskStatus(status: TaskStatus): boolean {
  return status === 'completed' || status === 'failed' || status === 'killed'
}
```

**关键洞察：**
- Task 有明确的生命周期状态
- Terminal 状态不可逆
- Coordinator 可以继续（SendMessage）或停止（TaskStop）Worker

### 2.2.4 信息不对称缓解

```typescript
// clawcode: Coordinator 不能说 "based on your findings"
"Workers can't see your conversation. Every prompt must be self-contained with everything the worker needs."
```

**关键洞察：**
- Coordinator 必须自己综合理解 Worker 的发现
- 不能把"理解"委托出去
- Worker prompts 必须自包含（self-contained）

### 2.2.5 并行化激励

```typescript
// clawcode: Phase 并行化
"Parallelism is your superpower. Workers are async.
Launch independent workers concurrently whenever possible."

| Situation | 机制 | Why |
|-----------|------|-----|
| Read-only tasks (research) | 并行 | 独立 |
| Write-heavy tasks (implementation) | 串行 | 避免冲突 |
| Verification | 可并行 | 不同文件区域 |
```

**关键洞察：不同阶段有不同并行策略。**

### 2.2.6 对 D7 重构的启示

| Clawcode 设计 | D7 对应 | 改进方向 |
|--------------|---------|---------|
| Coordinator synthesis 不可外包（禁 "based on findings"） | D7-S5 结构 + S3-A02 WorkerContext 自包含 | v1.1 WorkerContext 契约 + 模糊指代 lint |
| Task 状态机 + task-notification | D7-S1 + S4 FlowEvent（含 usage 客观锚点） | 加强 chain_consistency metric |
| Phase 分离 | D7-S2/S3/S4/S5 切法 A | 已对齐 |
| 并行 research，串行 write | D7-S3 ConflictGuard | 已实现 |
| Never fabricate agent results | D7-S2 anti-fabrication T（T03 候选） | v1.1 P0 |
| TaskStop / SendMessage | D7-S2-A03 HandleInterrupt + runner continue | 已实现 |
| matchSessionMode | PlanMode 会话一致性 span | v1.1 P1 |

**参考总结：** Clawcode Coordinator 验证了切法 A；Devrix 需将 prompt 层约束落实为 **span/T/契约**（见 `gaming-analysis.md` §10）。

> **完整博弈论 + 双边共识：** [`gaming-analysis.md`](gaming-analysis.md) §7（Claude）§8（共识）§10（Cursor 回应 Clawcode）。

---

## 3. S 切法对比

### 切法 A（推荐）：按用户价值流

| S | 价值流 | 核心 A | 状态 |
|---|--------|--------|------|
| D7-S2 | 会话编排入口 | ProcessMessage、HandleInterrupt | PLANNED |
| D7-S3 | Wave 调度 | ScheduleWave、GuardConflict | IMPLEMENTED |
| D7-S4 | 执行流 | PublishFlowEvent、NotifyGateway | IMPLEMENTED |
| D7-S5 | 决策规划 | ClassifyIntent、SynthesizeTaskGraph | PARTIAL |

**优点：**
- S 表达用户可验证承诺，非技术模块
- 与 D1 切法 A 原则一致
- A 层边界清晰（入口=会话、决策=规划）

**缺点：**
- S2/S5 需重新编号

### 切法 B（现状）：按模块

| S | 模块 | 问题 |
|---|------|------|
| S2 | coordinator | 含 S2+S5，边界不清 |
| S3 | wave | ok |
| S4 | flow/workplan/imsink | 4 个子模块合并 S4 |
| S5 | plan | 规划从 S2 拆分，但 Task 模型仍在 D2 |

**优点：**
- 贴近代码组织
- 编号无需大改

**缺点：**
- S 被模块绑架，非用户价值流
- S2 含 S5（边界不清）

---

## 4. Decision: S 编号方案

| 方案 | 选择 | 理由 |
|------|------|------|
| A: 新编号 S6–S9 | — | 浪费号段 |
| B: 复用现有 S2/S5 | **采用** | 保持兼容性 |
| C: 全部重编 S1–S4 | — | BREAKING |

**选择：方案 B（复用现有编号）**

**理由：**
- 已有 S2=会话编排、S5=决策规划的定义
- 不浪费号段
- Legacy 双轨可追溯

**约束：**
- 旧语义冻结为 Legacy Module Index
- 新 Canonical 按价值流语义

---

## 5. Legacy 双轨方案

### 旧 S 冻结为 Legacy

| 旧 S | 旧语义 | 新语义 | Legacy 路径 |
|------|--------|--------|-----------|
| D7-S2 | coordinator（含 S5） | 会话编排入口 | `d7-legacy/coordinator/` |
| D7-S5 | plan（部分在 D2） | 决策规划 | `d7-legacy/plan/` |

### 追溯规则

```
Legacy ID（如 D7-S2-A01-LEGACY）→ 新 Canonical（D7-S2-A01）
Legacy T（如 D7-S2-T01-LEGACY）→ 新 T 映射
```

### registry 标记

```markdown
## D7-S2: Session Orchestrator 🔶

> **Canonical** — 按用户价值流（S2=会话入口）
> **Legacy** — 旧 coordinator 模块（冻结追溯）
```

---

## 6. S 层定义（切法 A）

### D7-S2: 会话编排入口

| 属性 | 值 |
|------|---|
| North Star | 用户消息统一入口，决定走快速路径还是编排路径 |
| 触发条件 | D1 Gateway.RouteInbound |
| 用户目标 | 消息被正确路由到执行路径 |
| 涉及 A | ProcessMessage、EvaluateIntent、HandleInterrupt |

**Gherkin Scenario：**

```gherkin
Scenario: 消息路由到快速路径
  Given d7_enabled=true
  When D1 RouteInbound receives "hello"
  Then coordinator.ProcessMessage returns within 2ms
  And no Wave is scheduled

Scenario: 消息路由到编排路径
  Given d7_enabled=true
  When D1 RouteInbound receives "/plan build a web app"
  Then coordinator.ProcessMessage triggers SynthesizeTaskGraph
  And WaveScheduler receives TaskGraph
```

### D7-S3: Wave 调度

| 属性 | 值 |
|------|---|
| North Star | 多任务并行执行，冲突避免，上下文隔离 |
| 触发条件 | SynthesizeTaskGraph 产出 TaskGraph |
| 用户目标 | 任务按 DAG 顺序执行，峰值并发 ≤ 5 |

### D7-S4: 执行流

| 属性 | 值 |
|------|---|
| North Star | 执行进度透明，WorkPlan 可追溯 |
| 触发条件 | FlowEvent 产生 |
| 用户目标 | 实时看到任务进度 |

### D7-S5: 决策规划

| 属性 | 值 |
|------|---|
| North Star | 意图分类正确，任务拆解合理 |
| 触发条件 | ProcessMessage 需要决策 |
| 用户目标 | 系统理解我的意图并正确执行 |

---

## 7. A/F 层重编号草案

### D7-S2 A 层

| A ID | 名称 | 旧 Legacy ID | 备注 |
|------|------|-------------|------|
| D7-S2-A01 | ProcessMessage | D7-S2-A01-LEGACY | 主入口 |
| D7-S2-A02 | EvaluateIntent | — | 新增 |
| D7-S2-A03 | HandleInterrupt | D7-S2-A03-LEGACY | 复用 |

### D7-S5 A 层

| A ID | 名称 | 旧 Legacy ID | 备注 |
|------|------|-------------|------|
| D7-S5-A01 | ClassifyIntent | D7-S5-A01-LEGACY | 从 coordinator 迁入 |
| D7-S5-A02 | SynthesizeTaskGraph | D7-S5-A02-LEGACY | 复用 |
| D7-S5-A03 | SelectExecutor | — | 新增（v1.1） |

---

## 8. T 层补充（PLANNED）

| T ID | 描述 | 归属 A | 状态 |
|------|------|--------|------|
| D7-S2-A01-T01 | ProcessMessage 端到端延迟 ≤ 2ms | D7-S2-A01 | PLANNED |
| D7-S2-A01-T02 | FastPath 命中时无 Wave 创建 | D7-S2-A01 | PLANNED |
| D7-S2-A03-T01 | HandleInterrupt 中断顺序正确 | D7-S2-A03 | PLANNED |
| D7-S5-A01-T01 | ClassifyIntent 规则置信度 | D7-S5-A01 | PLANNED |
| D7-S5-A01-T02 | Command-first 优先于 LLM | D7-S5-A01 | PLANNED |

---

## 9. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | D7 v2.2.0 注册表 |
| 依赖 | DM-20260614-007（D1→D7 入口） |
| 约束 | 复用现有 S2/S5 编号 |
| 约束 | v1.0 registry-only |
| 约束 | 已 IMPLEMENTED T 不改 |

---

## 10. 下一步

- [ ] S3 Design：A/F 完整重编号 + Legacy 追溯表
- [ ] S3-Gate：Review 通过
- [ ] v1.0 Registry merge

---

**S2 提案完成，待 S3 Design。**
