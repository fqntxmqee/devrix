---
demand-id: DM-20260607-006
title: 上下文引擎 V3（PEV Plan + Milestone DAG + LongTerm Memory）
source: 架构/产品
priority: P0
status: S7_ARCHIVED
l1-domain: devrix
created: 2026-06-07
---

# 上下文引擎 V3

## 1. 原始描述

> V2 已落地 Autocompact、Verify commands、Token 统一与真实 LLM Gateway 接线，但 PEV 仍为 **Execute→Verify** 简化循环，缺少：
> 1. **Plan 阶段** — 无法将复杂任务分解为 Milestone DAG，通信层 `milestone_progress` 任务流无生产者；
> 2. **LongTerm Memory** — `LongTermMemory.Recall` 返回 `FeatureNotImplemented`，跨 Session 无项目知识沉淀。
>
> 通信层 V3 已具备 `milestone.Service`、TaskFlow、`milestone_progress` 四流消费端，需 Layer 2 补齐 Plan 与长期记忆能力。

## 2. 澄清记录

### Q1: V3 与 V2 边界？

**A**: V3 新增 PEV Plan 阶段、Milestone DAG 对接、LongTerm SQLite；**不修改** V2 压缩管道与 Verify commands 行为。`plan.enabled=false` 时保持 V2 Execute→Verify 路径。 — 2026-06-07

### Q2: Layer 2 如何访问 Milestone Service（Layer 1）？

**A**: 在 `internal/shared/contracts/` 定义 `IMilestonePlanner`（Create/UpdateProgress/Complete/GetExecutionOrder）；`cmd/devrix` 通过 `bridges/milestone` 注入 `milestone.Service` 适配器。**禁止** L2 直接 import `communication/milestone`。 — 2026-06-07

### Q3: Plan 是否每次 Process 都执行？

**A**: 仅当 `plan.enabled=true` 且用户消息被判定为「需分解任务」（启发式：无活跃 milestone 或用户显式 `/plan`）时执行 Plan；否则跳过直接进入 Execute。 — 2026-06-07

### Q4: Milestone 由谁创建？

**A**: Plan 阶段调用 LLM 生成结构化 JSON（milestones + dependencies），经校验后通过 `IMilestonePlanner` 批量创建；DAG 环检测失败则拒绝并降级为单步 Execute。 — 2026-06-07

### Q5: LongTerm Memory 存储位置与格式？

**A**: SQLite `~/.devrix/memory.db`（可配置 `longterm.db_path`）；表 `memory_entries(id, session_id, topic, content, embedding_blob, created_at)`；V3 **不做** 向量检索，Recall 用 `LIKE` + topic 标签匹配。 — 2026-06-07

### Q6: LongTerm 何时写入？

**A**: Process 完成且 `longterm.auto_store=true` 时，将本轮摘要（Autocompact 产出或最后 assistant 回复前 2k 字符）写入；用户可配置 `longterm.topics` 白名单过滤。 — 2026-06-07

### Q7: 快照版本是否升级？

**A**: 保持 `ctx-v1`；`ContextSnapshot` 新增可选字段 `milestones[]`、`longterm_refs[]`，旧快照向前兼容。 — 2026-06-07

### Q8: IPEVObserver 如何扩展？

**A**: 新增 `EmitPlanCompleted`、`EmitMilestoneProgress`；不合并回 `IObserver`，保持 V2 拆分决议。 — 2026-06-07

### Q9: 本变更不做什么？

**A**: 快照 AES 加密、异步 Autocompact、Multi-Agent Fork/Merge、向量 Embedding 检索 — 均归后续专项。 — 2026-06-07

### Q10: Plan 模型是否与 Execute 分离？（Grill 2026-06-07）

**A**: **是**。`plan.model` 独立配置，默认 `deepseek-v4`；Execute 沿用 session model。Plan 允许 15s 超时（高于 Autocompact 10s，因 DAG 生成更复杂）。 — 2026-06-07

### Q11: milestone_id 是否进入 metrics 标签？（Grill 2026-06-07）

**A**: **是**，但仅限单任务内 ≤10 个 milestone；`devrix_ctx_milestone_duration_seconds{milestone_id}` 在任务结束后标签集失效（无跨任务累积）。禁止 `task_id` + `session_id` 组合标签。 — 2026-06-07

### Q12: LongTerm at-rest 加密？（Grill 2026-06-07）

**A**: V3 **不做**，与 V2 快照明文决议一致；加密归安全专项。 — 2026-06-07

### Q13: Milestone Verify 失败后如何处理后续节点？（Grill 2026-06-07）

**A**: **fail-fast** — 当前 milestone `Fail` 后跳过后续 milestone，Process 以 `info` 事件说明原因并进入 `complete`（非 panic）。 — 2026-06-07

### Q14: Plan 默认开关？（Grill 2026-06-07）

**A**: `plan.enabled=false` 默认关闭，生产灰度需显式开启；`longterm.enabled=true` 但 `auto_store=false` 默认。 — 2026-06-07

## 3. 澄清范围

### 3.1 L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | devrix | 开发大脑 | 已有 |
| L2 | L2-DEVRIX-02 | 对话式开发助手 | 已有 |
| L3-BE | L3-BE-CTX-01 | 处理用户消息并维护上下文 | 增强（Plan 入口） |
| L3-BE | L3-BE-CTX-03 | 复杂任务分解与里程碑推进 | **新增** |
| L4-BE | L4-CTX-PEV | PEV 执行循环 | 增强（Plan 阶段） |
| L4-BE | L4-CTX-MEMORY | 分层记忆 | 增强（LongTerm SQLite） |
| L4-BE | L4-CTX-PLAN | 任务规划与 DAG 生成 | **新增** |
| L5 | L5-CTX-19 ~ L5-CTX-25 | 见 `l5-registry.md` | 草拟 |

### 3.2 范围

**In Scope（本变更）**:
- OpenSpec 四件套 + L5 登记
- PEV Plan 阶段（LLM 结构化输出 → Milestone DAG）
- `IMilestonePlanner` 契约 + Bridge 接线
- Milestone 顺序驱动 Execute/Verify + `milestone_progress` 事件
- LongTerm Memory SQLite（Recall + Store）
- `IPEVObserver` 扩展 Plan/Milestone 事件
- 配置 `plan.*`、`longterm.*`

**Out of Scope（本变更）**:
- 快照 AES 加密
- 异步 Autocompact
- 向量 Embedding / 语义检索
- Multi-Agent Peer-Review / Fork-Merge
- 钉钉 Milestone Card UI（通信层已有，本变更只产事件）

### 3.3 前置依赖

| 变更 | 最低版本 | 说明 |
|------|----------|------|
| `devrix-context-engine` | V1 已归档 | 基线 |
| `devrix-context-engine-v2` | V2 已归档 | Autocompact + Verify + Token |
| `devrix-llm-gateway` | V1 已归档 | Plan/Execute LLM 调用 |
| Communication V3 | milestone.Service 可用 | DAG 存储与四流消费 |
