# Proposal: Unified Work Tree

**Change ID:** `devrix-unified-work-tree`  
**Demand ID:** DM-20260617-009  
**Created:** 2026-06-17  
**Status:** Draft

---

## Problem Statement

Devrix 编排域把 **工作单元 (What)**、**执行句柄 (How)**、**编排图节点 (When)** 三套概念混称为 task，导致：

- LLM 需在 `task_id` / wave node / `bg_*` 之间切换语义
- `todo_write` 与 `task_*` 功能重叠、数据不同步
- Wave `TaskNode` 与持久 task board 割裂
- 子任务完成通知不完整（仅 workmodel 终态走 notify.Bus）
- 无法实现 clawcode 式 **主子任务递归 + uncertainty 驱动分解**

**受影响方**：D7 编排、D2 tool surface、LLM agent 工具调用、飞书 IM 任务进度展示。

## Proposed Solution

引入 **WorkItem 树** 作为唯一工作语义（What），与 **RunRegistry**（How，DM-011）分离挂接：

1. **WorkTree** — 父子 WorkItem、kind、依赖、持久化
2. **吸收现有入口** — todo_write→checklist、delegate→spawn、Wave→parallel implement 子树
3. **RunTurn 递归** — focus + uncertainty decompose（v2.0）
4. **Tool 面简化** — task_write / task_spawn / task_await / task_list（v2.0，含 alias）

**实施顺序**：先统一底层 WorkItem 模型（v1.0），再写路径挂树（v1.1），再 RunRegistry 挂接（v1.2），最后 tool rename + 递归（v2.0）。

## Scope

### In Scope

- WorkItem + WorkTree + DiskWorkItemStore
- TaskManager legacy adapter
- delegate / PlanMode / FlowHub / todo_write / Wave 写读 WorkTree
- WorkItem.RunRef ↔ RunRegistry（与 DM-011 联调）
- QueryWorkPlan 树形读模型
- 终态 tool 面（alias 期）
- Uncertainty + GetFocus（v2.0）

### Out of Scope

- RunRegistry 核心实现（见 DM-011 独立 change）
- 跨 session 任务可见性
- agent_tools 配置发现
- D2 QueryLoop 新能力

## Impact Analysis

| Component | Change Required | Details |
|-----------|-----------------|---------|
| workmodel | Yes | 新增 WorkItem/WorkTree；TaskManager 委托 |
| delegatetools | Yes | spawn 写 parent_id + kind |
| todo_tool | Yes | 底层改 UpsertChecklist |
| wave | Yes | decomposer 写树；scheduler 读树 |
| coordinator/workmodel | Yes | QueryWorkPlan 树形 |
| turn/orchestrator | Yes (v2.0) | focus + bubble notification |
| task_* tools | Yes (v2.0) | rename + alias |
| RunRegistry | Indirect | DM-011；本 change v1.2 挂接 |
| devrix.yaml | Minimal | 沿用 tasks.mode v2 |

## Architecture Considerations

- 对齐 DM-011：**WorkTree ≠ RunRegistry**，通过 RunRef 关联
- 对齐 DM-20260617-001：RunTurn 为 canonical 递归引擎
- 对齐 DM-20260617-008：TaskManager 显式注入，无 global singleton
- WaveScheduler 降为基础设施，不拥有任务类型
- D7 拥有 WorkTree；D2 仅 tool runner

## Success Criteria

- [ ] AC1–AC5：WorkItem 基础模型 + legacy 兼容（P0）
- [ ] AC6–AC10：各写入路径挂树（P1）
- [ ] AC11–AC13：RunRegistry 挂接（P2，依赖 DM-011）
- [ ] AC14–AC19：tool 简化 + 递归求解（P3）
- [ ] 全量 `go test ./internal/layers/orchestration/...` 绿

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| 大爆炸重写 | Med | High | 5 阶段 PR；legacy adapter 保留 |
| DM-011 阻塞 | Med | Med | v1.0–v1.1 RunRef 可空 |
| todo_write 回归 | Med | High | sc.Todos 投影 + 集成测试 |
| ID 前缀变更 | Low | Med | 文档 + alias |

## Next Steps

1. Review `demand.md` + `design.md`
2. `/openspec-apply devrix-unified-work-tree` 从 Phase 0（v1.0 模型）开始
3. 并行推进 DM-011 RunRegistry Phase 1
