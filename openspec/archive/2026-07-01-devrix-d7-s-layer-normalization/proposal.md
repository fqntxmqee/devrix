# Proposal: D7 S 层归一化

**Change ID:** `devrix-d7-s-layer-normalization`  
**Demand ID:** DM-20260701-002  
**Status:** Implementation In Progress  
**Created:** 2026-07-01

---

## 1. Problem Statement

D7 已完成多轮 WorkTree、MUPS、TaskContract、Rollup、Layer SubContext 演进，但 OpenSpec registry 仍把历史演进编号继续展示为 canonical S：

- current spec 同时出现 S1-S6、S8/S9/S11/S12/S13/S14/S15/S16/S18/S20/S21。
- A registry 同时保留 `LEGACY` 段、Canonical S1-S6、S7-S14、S20/S21。
- code-layout 只登记 D7 S1-S5 的物理 scenario slug，与 registry 中 S7+ 不一致。
- 运行时代码主链路已收敛，但注释中仍有 retired `FastPath` / `OrchestratePath` 词汇。

这会削弱 DSAFT 的 S 层稳定性：S 层应表达稳定场景 / 价值流，而不是技术节点、contract 或历史 change 编号。

## 2. Proposed Solution

将 D7 current canonical S 固定为 S1-S6：

| S | Name | North Star |
|---|------|------------|
| D7-S1 | Work Model | WorkItem / WorkTree / Scope / Rollup 状态权威 |
| D7-S2 | Session Orchestrator | 用户消息入口、TurnLoop、session-level coordination |
| D7-S3 | Wave Scheduler | 并行任务 DAG、worker pool、conflict/context isolation |
| D7-S4 | Execution Flow | FlowEvent、WorkPlan read model、D1/IM progress |
| D7-S5 | Decision & Planning | intent / observe / strategic plan / divergence proposal |
| D7-S6 | MUPS Governance | Execute / Verify / Learn / Escape / convergence governance |

迁移规则：

- S7-S14：保留为 historical mapping，活动映射到 S4/S5/S6。
- S20/S21：降级为 TaskContract contract section，映射到 S1/S6 的 A/F。
- MUPS 5 节点：表达为 A/F 活动链，不再作为独立 S。
- WorkTree 下行/上行：作为 S1 状态 contract + S6 convergence governance 的跨 S 机制。

## 3. Scope

### In Scope

- OpenSpec change 包与 delta spec。
- D7 canonical spec / a-registry / f-registry / t-registry / CHANGELOG 更新。
- Architecture layering / code-layout 中 D7 scenario 口径更新。
- 注释与 guard 测试澄清 retired ingress / compat shim。
- StrategicPlanReject 回灌与 child-stats uncertainty reconcile。

### Out of Scope

- 大规模包路径迁移。
- 历史 T ID 全量重编号。
- D2/D3/D4 S 层方法论重构。

## 4. Success Criteria

- [ ] D7 current canonical S 在 all SoT 中一致为 S1-S6。
- [ ] S7-S14/S20/S21 只出现在 historical/contract mapping 中。
- [ ] A/F registry 的 current code location 与现有代码路径一致。
- [ ] 主路径 guard 测试防止 retired ingress 文件回归。
- [ ] StrategicPlanReject 回灌测试通过。
- [ ] child-stats uncertainty reconcile 测试通过。

## 5. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| 历史 T ID 断裂 | High | 不重编号历史 T；新增 mapping section |
| 文档改动过大 | Med | 保留 historical mapping，先归一 current section |
| 代码补强与 docs-only 混杂 | Med | 代码补强限定两个函数级闭环 + 单元测试 |
| S6 物理路径无单独 slug | Low | S6 定义为 governance overlay，不新增目录迁移 |

## 6. Phasing

```text
P0  Demand + delta spec
P1  Canonical spec + registry normalization
P2  Compat shim comments + architecture guard
P3  StrategicPlanReject + uncertainty reconcile
P4  Acceptance + archive
```
