# Tasks: devrix-context-engine-v3

**Change ID:** devrix-context-engine-v3
**Demand:** DM-20260607-006
**Status:** S7 Archived
**Based on:** design.md, specs/context-engine/spec.md

---

## Milestone 1: Contracts + Types + Config（M1）

### Definition of Done
- [x] `IMilestonePlanner` 契约定义
- [x] `PEVPhasePlan` 类型扩展
- [x] `plan` / `longterm` 配置解析与校验

### Tasks

- [x] **T1**: 新增 `shared/contracts/milestone.go` — `IMilestonePlanner`
  - L5: L5-CTX-19（契约）
  - Estimate: 2h

- [x] **T2**: 扩展 `shared/types/context.go` — `PEVPhasePlan`、`PEVState.ActiveMilestoneID`
  - L5: L5-CTX-20
  - Estimate: 1h

- [x] **T3**: 新增 `shared/config/contextengine_v3.go` — PlanConfig、LongTermConfig
  - 校验：max_milestones ≤10、db_path 展开
  - L5: —
  - Estimate: 2h
  - Dependencies: None

- [x] **T4**: 新增 `shared/errors/context.go` — CTX_PLAN_4020/4021、CTX_MEMORY_4022
  - Estimate: 1h

- [x] **T5**: 配置单元测试 `shared/config/contextengine_v3_test.go`
  - Estimate: 2h
  - Dependencies: T3

---

## Milestone 2: Plan Engine + Milestone Bridge（M2）

### Definition of Done
- [x] Plan LLM 输出解析与 DAG 校验
- [x] Bridge 适配 milestone.Service

### Tasks

- [x] **T6**: 新增 `pev/plan.go` — PlanEngine（LLM → JSON → validate）
  - 环检测 Kahn 算法
  - 降级路径：校验失败 → 无 DAG
  - L5: L5-CTX-19, L5-CTX-25
  - Estimate: 6h
  - Dependencies: T1, T2, T3

- [x] **T7**: 新增 `bridges/milestone/wire.go` — MilestonePlannerAdapter
  - 实现 `IMilestonePlanner`
  - L5: L5-CTX-19
  - Estimate: 3h
  - Dependencies: T1

- [x] **T8**: Plan 单元测试 `pev/plan_test.go`
  - 有效 DAG、环检测、超上限拒绝
  - L5: L5-CTX-25
  - Estimate: 4h
  - Dependencies: T6

---

## Milestone 3: Milestone-Driven PEV + Events（M3）

### Definition of Done
- [x] 拓扑序 Execute→Verify
- [x] milestone_progress 事件发射
- [x] plan.enabled=false 回退 V2

### Tasks

- [x] **T9**: 新增 `pev/milestone_runner.go` — 按 DAG 驱动循环
  - L5: L5-CTX-20
  - Estimate: 5h
  - Dependencies: T6, T7

- [x] **T10**: 修改 `pev_engine.go` — 集成 Plan + MilestoneRunner
  - shouldPlan 启发式
  - L5: L5-CTX-24
  - Estimate: 4h
  - Dependencies: T9

- [x] **T11**: 扩展 `contracts.go` — IPEVObserver.EmitPlanCompleted/EmitMilestoneProgress
  - Estimate: 2h

- [x] **T12**: 修改 `engine.go` — milestone_progress 事件通道
  - L5: L5-CTX-21
  - Estimate: 3h
  - Dependencies: T10

- [x] **T13**: 集成测试 `tests/integration/context_plan_milestone_test.go`
  - L5: L5-CTX-20, L5-CTX-21, L5-CTX-24
  - Estimate: 4h
  - Dependencies: T12

---

## Milestone 4: LongTerm SQLite Memory（M4）

### Definition of Done
- [x] SQLite CRUD 实现
- [x] Recall 注入 + auto_store

### Tasks

- [x] **T14**: 新增 `memory/longterm.go` — SQLite 实现 `ILongTermMemory`
  - migration 建表
  - L5: L5-CTX-22, L5-CTX-23
  - Estimate: 5h
  - Dependencies: T3

- [x] **T15**: 修改 `memory/manager.go` — Recall 注入 system context
  - token 预算裁剪
  - L5: L5-CTX-22
  - Estimate: 3h
  - Dependencies: T14

- [x] **T16**: 单元测试 `memory/longterm_test.go`（替换 stub 测试）
  - L5: L5-CTX-22, L5-CTX-23
  - Estimate: 3h
  - Dependencies: T14

- [x] **T17**: 更新 `memory/longterm_stub.go` — enabled=false 路径
  - L5: L5-CTX-10 行为保持
  - Estimate: 1h

---

## Milestone 5: Wiring + Acceptance + Docs（M5）

### Definition of Done
- [x] main 路径接线完成
- [x] P0 验收全绿
- [x] L5 注册表更新

### Tasks

- [x] **T18**: `cmd/devrix/main.go` + `devrix-feishu` — WireContextV3（Planner + LongTerm）
  - L5: L5-CTX-18 延伸
  - Estimate: 3h
  - Dependencies: T7, T14

- [x] **T19**: `devrix.yaml` 示例配置 plan/longterm 段
  - Estimate: 1h

- [x] **T20**: 验收测试 `tests/acceptance/p0/ctx_plan_longterm_test.go`
  - L5: L5-CTX-19, L5-CTX-21, L5-CTX-22
  - Estimate: 4h
  - Dependencies: T18

- [x] **T21**: 更新 `openspec/l5-registry.md` — L5-CTX-19~25 IMPLEMENTED
  - Estimate: 1h

- [x] **T22**: 更新 `docs/context-engine-design.md` 附录 B V3 行 + 决策记录
  - Estimate: 2h

---

## Summary

| Milestone | Tasks | Estimate |
|-----------|-------|----------|
| M1 Contracts+Config | T1–T5 | 8h |
| M2 Plan+Bridge | T6–T8 | 13h |
| M3 Milestone PEV | T9–T13 | 18h |
| M4 LongTerm | T14–T17 | 12h |
| M5 Wiring+验收 | T18–T22 | 11h |
| **合计** | **22** | **~62h** |

---

## V4 Backlog（本变更不实施）

- [ ] 快照 AES 加密
- [ ] 异步 Autocompact
- [ ] LongTerm 向量 Embedding 检索
- [ ] Multi-Agent Milestone 协作
