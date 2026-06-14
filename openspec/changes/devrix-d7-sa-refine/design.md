# D7 Orchestration — S 层重构 Design

**Change ID:** devrix-d7-sa-refine
**Demand ID:** DM-20260614-008
**阶段:** S3 Design
**版本:** v1.0
**状态:** Draft — 待 S3-Gate Review
**基于:** gaming-analysis.md §8 三方共识

---

## 1. 概述

### 1.1 设计目标

| 目标 | 描述 |
|------|------|
| S 切法 | 按用户价值流（S2=会话入口、S5=决策规划），非按模块 |
| Legacy 双轨 | 旧编号冻结追溯，新 Canonical 按价值流语义 |
| 边界声明 | D7 = Orchestration Mediator，保证过程可验证，不保证结果正确 |

### 1.2 版本范围

| 版本 | 范围 |
|------|------|
| v1.0 | Registry 重构（S/A/F 重编号 + Legacy 双轨） |
| v1.1 | S2 T 锚定 + S5 routing in coordinator + T03 anti-fabrication |
| v2.0 | Task 模型归 D7 + Legacy 删除 |

---

## 2. Decision 记录

### Decision: S 切法

| 方案 | 选择 | 理由 |
|------|------|------|
| 切法 A：按用户价值流 | **采用** | S 表达用户可验证承诺，非技术模块 |
| 切法 B：按模块 | 拒绝 | S 被模块绑架，与 D1 切法 A 不一致 |

### Decision: S 编号方案

| 方案 | 选择 | 理由 |
|------|------|------|
| 新编号 S6–S9 | 拒绝 | 浪费号段 |
| 复用现有 S2/S5 | **采用** | 保持兼容性，Legacy 双轨可追溯 |
| 全部重编 S1–S4 | 拒绝 | BREAKING |

### Decision: Legacy 双轨

| 方案 | 选择 | 理由 |
|------|------|------|
| Legacy 冻结追溯 | **采用** | 旧语义不可覆盖，新 Canonical 独立演进 |
| 禁止 Legacy 新增 T | **强制** | 防止双轨均衡固化 |

### Decision: D7-S3/S4 角色

| Clawcode 概念 | 修订映射 | 博弈含义 |
|---------------|----------|---------|
| Worker = D7-S3+S4（错误） | **D7-S3/S4 = 调度机制** | Mechanism Designer |
| — | **D2/D4 = 执行 Agent** | Stackelberg Follower |

---

## 3. S 层定义（切法 A Canonical）

### D7-S2: 会话编排入口

| 属性 | 值 |
|------|---|
| North Star | 用户消息统一入口，决定走快速路径还是编排路径 |
| 触发条件 | D1 Gateway.RouteInbound |
| 用户目标 | 消息被正确路由到执行路径 |
| 涉及 A | ProcessMessage、EvaluateIntent、HandleInterrupt |
| 博弈角色 | Screening Mechanism（筛路径） |

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

Scenario: 禁止伪造进度信号
  Given d7_enabled=true and FastPath
  When coordinator.ProcessMessage returns
  Then no synthetic Task progress is sent before Worker terminal FlowEvent
```

### D7-S3: Wave 调度

| 属性 | 值 |
|------|---|
| North Star | 多任务并行执行，冲突避免，上下文隔离 |
| 触发条件 | SynthesizeTaskGraph 产出 TaskGraph |
| 用户目标 | 任务按 DAG 顺序执行，峰值并发 ≤ 5 |
| 博弈角色 | Mechanism Designer（定执行规则） |

### D7-S4: 执行流

| 属性 | 值 |
|------|---|
| North Star | 执行进度透明，WorkPlan 可追溯 |
| 触发条件 | FlowEvent 产生 |
| 用户目标 | 实时看到任务进度 |
| 博弈角色 | Costly Signaler（向用户广播成本） |

### D7-S5: 决策规划

| 属性 | 值 |
|------|---|
| North Star | 把用户 goal 转化为可执行的任务结构 |
| 触发条件 | ProcessMessage 需要决策 |
| 用户目标 | 系统理解我的意图并正确路由 |
| 博弈角色 | Information Producer（产私有信息） |

**注：** D7-S5 决策的是**结构路径**（goal → TaskNode DAG），不是**内容质量**（Tool 选择、结论对错）。

---

## 4. A 层重编号（Canonical）

### D7-S2 A 层

| A ID | 名称 | 旧 Legacy ID | 说明 |
|------|------|-------------|------|
| D7-S2-A01 | ProcessMessage | D7-S2-A01-LEGACY | 主入口，路由分发 |
| D7-S2-A02 | EvaluateIntent | — | 新增：意图评估（FastPath 判断） |
| D7-S2-A03 | HandleInterrupt | D7-S2-A03-LEGACY | 中断处理 |

### D7-S3 A 层

| A ID | 名称 | 旧 Legacy ID | 说明 |
|------|------|-------------|------|
| D7-S3-A01 | ScheduleWave | D7-S3-A01-LEGACY | DAG 调度 |
| D7-S3-A02 | ResolveWorkerContext | D7-S3-A02-LEGACY | 上下文解析 |
| D7-S3-A03 | GuardConflict | D7-S3-A03-LEGACY | 冲突保护 |

### D7-S4 A 层

| A ID | 名称 | 旧 Legacy ID | 说明 |
|------|------|-------------|------|
| D7-S4-A01 | PublishFlowEvent | D7-S4-A01-LEGACY | 发布 FlowEvent |
| D7-S4-A02 | SnapshotWorkPlan | D7-S4-A02-LEGACY | WorkPlan 快照 |
| D7-S4-A03 | NotifyGateway | D7-S4-A03-LEGACY | 通知 D1 |

### D7-S5 A 层

| A ID | 名称 | 旧 Legacy ID | 说明 |
|------|------|-------------|------|
| D7-S5-A01 | ClassifyIntent | D7-S5-A01-LEGACY | 意图分类（规则 + Command-first） |
| D7-S5-A02 | SynthesizeTaskGraph | D7-S5-A02-LEGACY | 任务图合成 |
| D7-S5-A03 | SelectExecutor | — | 新增（v1.1）：执行器选择 |

---

## 5. F 层草案（Canonical）

### D7-S2 F 层

| F ID | 名称 | 说明 |
|------|------|------|
| D7-S2-A01-F01 | RouteByIntent | 按意图分类路由 |
| D7-S2-A01-F02 | ExecuteFastPath | 执行快速路径 |
| D7-S2-A01-F03 | EnterOrchestration | 进入编排路径 |
| D7-S2-A01-F04 | EmitSessionEvents | 发送会话事件 |

### D7-S5 F 层

| F ID | 名称 | 说明 |
|------|------|------|
| D7-S5-A01-F01 | ClassifyByRules | 规则分类（verifiable） |
| D7-S5-A01-F02 | ClassifyByLLM | LLM 分类（cheap talk 风险，仅作 shadow） |
| D7-S5-A01-F03 | MergeClassifications | 合并分类结果 |

---

## 6. T 层补充（PLANNED → v1.1 实现）

| T ID | 描述 | 归属 A | 优先级 | 博弈含义 |
|------|------|---------|---------|---------|
| D7-S2-A01-T01 | ProcessMessage 端到端延迟 ≤ 2ms | D7-S2-A01 | P0 | 入口响应承诺 |
| D7-S2-A01-T02 | FastPath 命中时无 Wave 创建 | D7-S2-A01 | P0 | screening 完整性 |
| D7-S2-A01-T03 | 禁止在 Worker terminal FlowEvent 前伪造 Task 进度 | D7-S2-A01 | P0 | anti-fabrication commitment |
| D7-S2-A03-T01 | HandleInterrupt 中断顺序正确 | D7-S2-A03 | P0 | 可中断性承诺 |
| D7-S5-A01-T01 | 规则分类置信度阈值验证 | D7-S5-A01 | P0 | screening 可重复性 |
| D7-S5-A01-T02 | Command-first 优先于 LLM 分类 | D7-S5-A01 | P0 | 用户显式策略优先 |
| D7-S5-A02-T01 | SynthesizeTaskGraph 产出有效 DAG | D7-S5-A02 | P1 | 结构决策验证 |

---

## 7. Legacy 双轨方案

### 7.1 追溯规则

```
Legacy ID（如 D7-S2-A01-LEGACY）→ 新 Canonical（D7-S2-A01）
Legacy T（如 D7-S2-T01-LEGACY）→ 新 T 映射
```

### 7.2 registry 标记格式

```markdown
## D7-S2: 会话编排入口 🔶

> **Canonical** — 按用户价值流（S2=会话入口）
> **Legacy** — 旧 coordinator 模块（冻结追溯）
```

### 7.3 禁止约束

- **禁止** 在 Legacy 语义上新增 T
- **禁止** 在 Legacy 路径下开发新功能
- **强制** 新功能走 Canonical S

---

## 8. Clawcode 实证映射

### 8.1 WorkerContext 自包含契约

来自 Clawcode `clawcode/src/coordinator/coordinatorMode.ts`：

```go
// D7-S3-A02 输出必须满足 — 博弈：Worker 无法从用户对话搭便车
type WorkerContextBundle struct {
    TaskID      string
    Goal        string   // 自包含 goal，禁止 "based on findings"
    FileHints   []string // 具体路径/行号（若已知）
    Constraints []string // 只读/禁止写/工具白名单
}
```

**T 候选：** D7-S3-A02-T0x — Worker prompt 不得含模糊指代（「the bug」「your findings」）

### 8.2 FlowEvent Metadata 客观锚点

来自 Clawcode `<task-notification><usage>`：

```go
// D7 wall clock 测量，Agent 不可填
type FlowEventUsage struct {
    DurationMs int64  // D7 wall clock
    ToolUses  int    // D7 统计
    WorkerType string // cursor / claude_code / subagent
}
```

### 8.3 D6 L3 保守路由接口

来自 gaming-analysis.md §10.5：

```
D7 路由矩阵 = 默认合同
L3 惩罚 = 合同重谈判
θ Tune = 条款调整
```

**D6→D7 policy injection point：** 只读 config + span 标记，不内嵌信誉逻辑。

---

## 9. 跨域契约

### D7 ↔ D1

| 契约 | 方向 | 接口 |
|------|------|------|
| ProcessMessage | D1→D7 | `coordinator.Entry.ProcessMessage` |
| FlowEvent | D7→D1 | `contracts.EngineEvent`（含 source_event_id） |
| Task 进度 | D7→D1 | `d7.signal.task` span |

### D7 ↔ D6

| 契约 | 方向 | 接口 |
|------|------|------|
| L3 路由收缩 | D6→D7 | `orchestration.d7_enabled=false` + span 标记 |
| Validation metric | D7→D6 | `orchestration.d6.validation.*` counter |

---

## 10. 下一步

- [ ] S3-Gate Review 通过
- [ ] v1.0 Registry merge（a-registry.md + f-registry.md 更新）
- [ ] v1.1 tasks 创建（S2 T01/T02/T03 + S5 routing）

---

**Design 完成，待 S3-Gate Review。**
