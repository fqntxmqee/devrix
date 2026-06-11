---
demand-id: DM-20260611-007
title: Wave Scheduler — DAG 并行 Worker 池与 IM 多卡双区块
source: 产品 / 架构讨论（飞书 IM 多 Agent 并行编排）
priority: P0
status: S3_Planning
l1-domain: orchestration
created: 2026-06-11
---

# Wave Scheduler — DAG 并行 Worker 池与 IM 多卡双区块

## 1. 原始描述

复杂任务经拆解形成 **Task DAG** 后，需要固定 Worker 池并行执行，并在飞书 IM 为每个 Worker 展示独立任务卡片（思考区 + Agent 流式输出区）。

**默认并行配额：**

| Worker 类型 | 并发槽 | 后端 |
|------------|--------|------|
| Cursor Agent | 1 | `call_cursor` |
| Claude Code Agent | 1 | `call_claude-code` |
| SubAgent | 3 | 进程内 SubQuery（LLM Gateway） |

**调度与上下文：**

- DAG 由 **Plan Engine 自动生成**（非 Leader 手工 `task_create` 为主路径）
- **有空槽即派活**（持续调度，非「整 wave 全完再下一批」）
- CLI Agent 与 SubAgent **默认同 WorkDir**；同目录写冲突由 **上层调度规避**（任务分配、文件范围、时序），不强制 worktree 隔离

**IM 呈现：**

- 最多 5 张并行 Worker 卡片
- 每卡两区块：思考信息（thinking）+ Agent 流式返回（output）

**上下文策略：**

- **新派任务**：不带 Leader 全量历史；Task directive + 补充 System Prompt 即可
- **续接 / 依赖上游**：按 Task 边携带上游摘要或 Sidechain Resume，非无脑继承全对话

## 2. 澄清记录

### Q1: DAG 谁生成？
**A**: **Plan Engine 自动生成**（1B） — 2026-06-11

### Q2: 调度粒度？
**A**: **持续调度** — Task 就绪且对应 Worker 槽位空闲即启动（2B） — 2026-06-11

### Q3: CLI Agent 是否 worktree 隔离？
**A**: **同目录运行**（3B）；并行写文件风险由 Wave Scheduler 在任务分配层规避（互斥文件集、串行化冲突 Task、或调度到不同 SubAgent 只读/写分工） — 2026-06-11

### Q4: 与 v2 Hub-Spoke「CLI 不参与委派」的关系？
**A**: 本需求 **扩展 v2 边界**：将 `call_cursor` / `call_claude-code` 纳入 **Wave Worker Pool 一等调度单元**，仍保持「用户只与 Leader 对话、Worker 无编排权」 — 2026-06-11

## 3. 澄清范围

### 3.1 L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | orchestration | 工作编排 | 已有（v2 读模型） |
| L2 | L2-ORCH-WAVE | DAG 并行 Wave 调度 | **新增** |
| L3-BE | L3-BE-ORCH-DISPATCH | Plan 产出 DAG 后触发 Wave 派活 | **新增** |
| L3-FE | L3-FE-IM-WORKER-CARD | 飞书多 Worker 双区块卡片 | **新增** |
| L4-BE | L4-BE-ORCH-WAVE-SCHEDULER | Wave 调度器 + Worker 池 | **新增** |
| L4-BE | L4-BE-ORCH-CONTEXT-POLICY | Task 上下文策略（fresh/resume/upstream） | **新增** |
| L4-BE | L4-BE-ORCH-CONFLICT-GUARD | 同目录写冲突规避（调度层） | **新增** |
| L4-FE | L4-FE-IM-WORKER-CARD-RENDER | Worker 双区块 Card Renderer | **新增** |
| L5 | L5-ORCH-10 ~ L5-ORCH-18 | 见 design.md §6 | **草拟** |

### 3.2 范围

**In Scope**:

- Plan Engine → Task DAG（含 owner/worker_type/dependencies/context_policy）
- WaveScheduler：cursor×1 + claude-code×1 + subagent×3 槽位持续填充
- ContextPolicy：fresh / resume_sidechain / upstream_artifact
- ConflictGuard：同 WorkDir 下互斥写路径调度（上层规避，非 worktree）
- ExecutionFlowHub 扩展：per-task worker_id 事件
- **Worker 生命周期：** CancelWorker / CancelAll / 槽位释放（参照 clawcode Swarm shutdown，依赖 DM-009）
- 飞书：每 Worker 独立双区块卡片（thinking + output）
- Leader 汇总 wave 结果后继续编排

**Out of Scope**:

- 跨 Session 持久化 Task 看板（v3 D7 升格）
- Agent 间 Mailbox / 用户与 Worker 直接对话
- 自动 merge 冲突解决（git merge bot）
- Worktree 默认隔离（用户明确选择 3B）

## 4. 验收标准（P0 摘要）

| ID | 标准 |
|----|------|
| AC1 | Plan Engine 产出带依赖的 Task DAG，Scheduler 只调度 `ready` 节点 |
| AC2 | 同时最多 5 路并行（1+1+3 配额），槽满时 ready Task 排队 |
| AC3 | 槽位释放后 **持续** 派发下一 ready Task，无需等整批完成 |
| AC4 | 飞书 IM 显示 N≤5 张 Worker 卡，每卡含 thinking + output 两区块且独立流式更新 |
| AC5 | ContextPolicy=fresh 的 Task 不携带 Leader 全量 Messages |
| AC6 | ContextPolicy=upstream 的 Task 携带依赖 Task 的 artifact/summary |
| AC7 | 同 WorkDir 下标注冲突的 Task 不会被并行分配到会写同一文件的 Worker |
| AC8 | `CancelWorker(task_id)` 30s 内释放槽位，Worker 状态=cancelled，IM 卡显示取消 |
| AC9 | Session `/new` 或结束时 `CancelAll` 终止全部 running Worker |
| AC10 | CLI Worker cancel 终止底层进程，不产生 zombie 槽占用 |

## 5. 关联需求

- DM-20260607-006（Milestone DAG / Plan）
- DM-20260610-012（QueryLoop / SubQuery）
- DM-20260608-012（Agent Tools call_*）
- DM-20260611-006（Feishu Cardkit 流式 — Worker 卡可复用 cardkit 能力）
- **DM-20260611-009（Background Cancel 协议 — task_stop 复用）**
