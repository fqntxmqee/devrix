---
demand-id: DM-20260613-001
title: D7 Orchestration Domain — 编排域升格与 D2 解耦
source: 架构审计（D2 职责溢出、ORCH 身份不足）
priority: P0
status: S2_Clarified
dsaft_domain: D7
created: 2026-06-13
last-updated: 2026-06-14
review-round: R1
---

# D7 Orchestration Domain — 编排域升格与 D2 解耦

## 1. 原始描述

Devrix 在 V5–V6 快速迭代后，**D2 Context Engine 承担了编排职责**（Task 持久化、BackgroundTask、Delegate hooks、SessionQueue），而 **ORCH 包**（WaveScheduler、ExecutionFlowHub）虽已实现核心能力，却无独立域身份。系统缺少统一层回答：

> 做什么、按什么顺序做、谁来做、做得怎么样了。

**目标：** 升格 ORCH 为 **D7 Orchestration Domain**，D1 入口上移至 D7，D2 瘦身为纯 LLM↔Tool 原语。

## 2. 澄清记录（Review R1 — 2026-06-14）

### Q1: D7 与 D1 的层级关系？

**A**: D7 是 **横向协调层**，编排 D2+D4，向 D1 发布进度事件；**D1 仍是 ingress owner**。文档表述改为「协调 D1–D6 跨域执行」，而非「位于 D1 之上」。

### Q2: 系统里有几套 Task 模型？如何统一？

**A**: **三模型共存、职责分离**（v1.0 不强制合并数据结构）：

| 模型 | ID 前缀 | 归属 | 职责 |
|------|---------|------|------|
| **PlanTask** | `task_` | D7-S1 | Plan 任务图：subject、blocked_by、持久化 |
| **WaveTaskNode** | Plan 内节点 ID | D7-S3 | DAG 调度单元：worker_type、context_policy |
| **BackgroundRun** | `bg_` | D7-S1（目标迁入） | SubQuery 异步运行句柄：output、cancel |

**映射规则：** Wave `TaskNode.ID` 可关联 PlanTask.ID；BackgroundRun 通过 FlowEvent.TaskID 与 PlanTask 联动（`link_tasks`）。统一 **查询入口** 为 `QueryWorkPlan`，不要求 v1.0 合并存储。

### Q3: Task 状态机用哪套词汇？

**A**: **v1.0 沿用现行代码词汇**，需求中的 `created/assigned/running` 视为目标别名，映射如下：

| 目标（文档别名） | 现行（代码 SoT） | 说明 |
|----------------|-----------------|------|
| created | pending | TaskManager.Create 初始态 |
| assigned | pending + owner set | FlowStarted link_tasks 时 SetOwner |
| running | in_progress | FlowStarted 或手动 UpdateStatus |
| completed | completed | FlowCompleted |
| failed | failed | FlowFailed |
| cancelled | （BackgroundRun） | BackgroundRegistry 独立状态 |

v1.0 **不实现**严格状态机校验；v1.1 可在 D7-S1 引入 `TransitionTaskState` 校验。

### Q4: D7-S2 串行编排 vs D7-S3 并行 Wave 如何分工？

**A**: **编排路由矩阵**（见 `d7-domain.md` §Orchestration Routing Matrix）：

- **FastPath** → 直连 D2 QueryLoop，不经 Plan/Wave
- **单步 explore/plan** → S2 串行调 D2（只读工具集）
- **多 Worker 并行 execute** → S2 创建 Plan 后调用 **S3 WaveScheduler.Start**
- **单 SubQuery background** → S1 BackgroundRun 注册，不经 Wave

S2 **不**替代 S3 做并行 DAG 调度。

### Q5: PlanMode（`/plan`）与 ClassifyIntent + SynthesizeTaskGraph 的关系？

**A**: **分阶段引入**（S5 Phased Roadmap）：

| 阶段 | 能力 | 触发 |
|------|------|------|
| **S5-P1**（已有） | PlanMode + PlanAgent 只读探索 | 用户 `/plan` |
| **S5-P2**（v1.0） | ClassifyIntent：fast / orchestrate 二分 | 每条用户消息 |
| **S5-P3**（v1.1） | SynthesizeTaskGraph 自动拆解 | orchestrate 路径且非 PlanMode active |
| **S5-P4**（v1.2） | auto_detect 可选进入 PlanMode | 配置开启 |

v1.0 **不**要求自动 SynthesizeTaskGraph；编排路径可先进入 PlanMode 或手动 Task 创建。

### Q6: 迁移期双入口如何共存？

**A**: **Migration Coexistence Contract**（见 d7-domain.md）：

- `orchestration.d7_enabled=false`（默认）：D1→D2.Process，行为与现网一致
- `d7_enabled=true`：D1→D7.ProcessMessage；D7 **必须**通过 contracts 调 D2/D4，禁止 D2.Process 内嵌编排
- 迁移窗口：最长 2 个 release；4 组合回归矩阵（d7 × plan）

### Q7: FastPath ≤2ms 如何验收？

**A**: 拆为两个 WHAT 指标：

| T ID | WHAT（非 HOW） | 测量 |
|------|---------------|------|
| D7-S2-T02a | FastPath proxy 在 Classify 完成后的额外开销 P99 ≤ 2ms | benchmark |
| D7-S2-T02b | 规则 ClassifyIntent P99 ≤ 1ms（无 LLM） | benchmark |

不在热路径上对简单消息调用 LLM Classify。

### Q8: HandleInterrupt 范围？

**A**: 拆分为子能力，v1.0 分步交付（Review R2 顺序修正）：

1. `WaveScheduler.CancelAll(sessionID)` — 显式取消；Wave ctx 脱离 Process，不可依赖传播
2. 取消 D4 活跃 delegate workers
3. 取消 D2 Process context（已有 `gateway.StopProcess`）
4. 发射 `stopped` EngineEvent
5. TaskCancel → WorkerCancel 反向链路

**/stop** 与**正常 Process 结束**区分：后者不杀 Wave（Plan Engine 设计预期）。

### Q9: BackgroundTask 是否纳入 D7-S1？

**A**: **v1.0 标记为 D7-S1 托管目标**，代码暂留 `query/background.go`；需求补充 Scenario 与 T 点（D7-S1-T07）。与 DM-20260612-011（Unified Task Registry）对齐：PlanTask 与 BackgroundRun 分离，Registry facade 归 D7-S1。

### Q10: 配置 `tasks.store_dir` 与 `orchestration.task.store_dir` 重复？

**A**: **v1.0 单一 SoT**：仅 `context_engine.tasks.store_dir`；`orchestration.task.*` 规划项标记 **DEPRECATED**，实现时不新增第二配置源。

### Q11: D6 校验如何不影响热路径？

**A**: D6 ValidateOrchestration 为 **advisory**；调用超时默认 50ms，超时视为 pass；失败仅 structured log，不阻断编排。

## 3. L1–L5 / DSAFT 映射

| 层级 | ID | 名称 | 状态 |
|------|-----|------|------|
| D | D7 | Orchestration Domain | PARTIAL |
| S | D7-S1 | Work Model | PARTIAL |
| S | D7-S2 | Session Orchestrator | PLANNED |
| S | D7-S3 | Wave Scheduler | IMPLEMENTED |
| S | D7-S4 | Execution Flow | IMPLEMENTED |
| S | D7-S5 | Decision & Planning | PARTIAL（S5-P1 only） |

**依赖需求：** DM-20260610-012（ORCH v2）、DM-20260611-007（Wave Scheduler）、DM-20260612-011（Unified Task Registry，对齐用）

## 4. 范围

### In Scope（v1.0）

- D7 包骨架 + `d7_enabled` feature flag
- D7-S2 ProcessMessage + FastPath + HandleInterrupt（子能力 1–5）
- D7-S5-P2 ClassifyIntent（规则优先，command-first）
- D7-S1 PlanTask 迁入 D7（路径迁移，行为不变）
- D2 loop 瘦身（移除 delegate hooks）
- D7-S3/S4 包路径迁移（re-export 桥接）
- 文档与 T 层：S2/S5-P2 P0 测试点

### Out of Scope（v1.0）

- S5-P3 SynthesizeTaskGraph 自动拆解
- S5-P4 auto_detect
- PlanTask 与 BackgroundRun 数据结构合并
- WorkPlan 磁盘重放（纯内存读模型可重建）
- D6 校验规则完善

### 已实现（不重复交付）

- D7-S3 Wave Scheduler（DM-007）
- D7-S4 Execution Flow Hub（DM-012）
- S5-P1 PlanMode（task-planning-design.md）

## 5. 验收标准（P0 摘要）

| ID | 标准 |
|----|------|
| AC1 | `d7_enabled=true` 时 D1 RouteInbound 调用 D7.ProcessMessage，不直连 D2.Process |
| AC2 | `d7_enabled=false` 时行为与迁移前 bit-identical |
| AC3 | 简单消息走 FastPath，不创建 Plan/Wave |
| AC4 | `/plan`、`/stop` 等命令优先于 LLM Classify |
| AC5 | 并行 execute 路径经 S3 Wave，峰值并发 ≤5 |
| AC6 | HandleInterrupt：Wave→D4→Process→stopped→TaskCancel（/stop 专用，与正常 Process 结束区分） |
| AC7 | D2 loop 无 multiagent/delegate import |
| AC8 | 22+ 既有 S3/S4 T 点迁移后仍全绿 |
| AC9 | QueryWorkPlan 聚合 PlanTask + ExecutionFlow（现行行为保持） |

## 6. 实施阶段（修订版）

| Phase | 内容 | 交付物 |
|-------|------|--------|
| A | 文档澄清 + demand/tasks（本需求） | demand.md, tasks.md, d7-domain R1 |
| B | D7 骨架 + contracts + re-export | `internal/layers/d7/`, feature flag |
| C | S5-P2 ClassifyIntent + S2 ProcessMessage | orchestrator, classifier |
| D | S2 HandleInterrupt + D1 入口切换 | gateway 灰度 |
| E | D7-S1 迁移 + D2 loop 瘦身 | workmodel, thin loop |
| F | 回归 + acceptance-report | P0 T 全绿 |

> **修订说明：** 原 Proposal Phase 4（入口）先于 Phase 6（瘦身）易导致双轨期过长；修订为 Phase C/D 与 E 尽量同 release 交付，缩短 `d7_enabled=true` 且 loop 仍含 hooks 的窗口。

## 7. 评审入口（供二次 Review）

| 文档 | 用途 |
|------|------|
| 本文档 `demand.md` | 需求澄清 SoT |
| `openspec/specs/d7-orchestration/d7-domain.md` | 完整 Requirement + Scenario |
| `openspec/specs/d7-orchestration/design.md` | 六段式架构设计 |
| `openspec/specs/d7-orchestration/layer-delta.md` | 现行 vs 目标差距 |
| `openspec/changes/.../tasks.md` | 任务分解（无代码） |
| `openspec/changes/.../review-r1.md` | Review R1 决议索引 |

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-13 | 初稿（proposal 摘录） |
| 1.0 | 2026-06-14 | Review R1 澄清写入，范围/阶段/验收修订 |
