---
demand-id: DM-20260701-002
title: D7 S 层归一化 — DSAFT canonical S1-S6 与 WorkTree+MUPS 关系收敛
priority: P0
status: S7_Archived
dsaft_domain: [orchestration]
created: 2026-07-01
reporter: AI 架构审查（D7 WorkTree+MUPS + DSAFT S/A layer review）
related:
  - DM-20260701-001 (MUPS propagation / convergence semantics)
  - DM-20260630-013 (D2+D7 review hardening)
  - DM-20260626-001 (D7 six-S simplification)
---

# Demand: D7 S 层归一化

## 1. 原始诉求

用户要求再次审查 D7 领域设计，重点确认 WorkTree+MUPS 架构中“向下传播”和“向上反馈”的机制是否逻辑清晰，并从 DSAFT 方法论检查 S 层、A 层设计是否符合规范，是否存在冗余或旧链路残留。

## 2. 问题陈述

审查结论：D7 运行时代码主链路已基本收敛到 `RunSessionTurnLoop → ItemPipelineRunner → WorkTree`，但 OpenSpec 资产仍把多个历史演进层级同时作为 canonical S 展示，导致 DSAFT 分层语义漂移。

| ID | 现象 | 根因 | 严重度 |
|----|------|------|--------|
| D7-SN-01 | `spec.md` 同时列出 S1-S6、S8/S9/S11/S12/S13/S14/S15/S16/S18/S20/S21 | 技术节点、contract、历史 change 编号被升格为 S | P0 |
| D7-SN-02 | `a-registry.md` 同时存在 `LEGACY` 段、Canonical S1-S6、S7-S14、S20/S21 | current SoT 与 historical mapping 混杂 | P0 |
| D7-SN-03 | A/F registry 中部分代码路径指向已迁移或不存在的旧包 | registry 未随 WorkItem/MUPS 运行时收敛同步 | P1 |
| D7-SN-04 | `FastPath` / `OrchestratePath` 等 retired 词汇仍在注释中传播 | 兼容 shim 与主路径未明确区分 | P1 |
| D7-SN-05 | `StrategicPlanReject` 结构化错误未回灌下一轮 prompt | Plan 阶段 divergence feedback 闭环不完整 | P1 |
| D7-SN-06 | parent reevaluate uncertainty 使用历史 stored value 作为 round signal | 上行反馈的 child-stats 收敛语义不够直接 | P1 |

## 3. 业务目标

| ID | 目标 | 可验证承诺 |
|----|------|------------|
| D7-SN-G1 | D7 canonical S 只表达稳定业务场景 | current SoT 固定为 S1-S6，S7-S14/S20/S21 迁为 historical/contract mapping |
| D7-SN-G2 | WorkTree+MUPS 关系可解释 | 下行传播、向上反馈、MUPS 5 节点在 S/A/F 中定位明确 |
| D7-SN-G3 | Registry 与代码路径一致 | A/F registry 不再指向旧包或 retired ingress |
| D7-SN-G4 | 发散/收敛反馈闭环完整 | StrategicPlanReject 与 child-stats uncertainty 都有测试覆盖 |

## 4. L1-L5 映射

| 层级 | 映射 |
|------|------|
| L1 | D7 Orchestration |
| L2 | WorkTree+MUPS 编排治理 |
| L3-BE | D7 S 层 canonical 场景归一化 + WorkItem pipeline 反馈闭环 |
| L4 | SLayerNormalization、RegistryCanonicalMapping、CompatShimGuard、StrategicPlanRejectFeedback、ChildStatsUncertaintyReconcile |
| L5 | D7-SN-T01 ~ D7-SN-T06 |

## 5. In Scope / Out of Scope

### In Scope

- D7 canonical spec / A/F/T registry 归一化。
- `layering.md` / `code-layout.md` D7 scenario 口径同步。
- retired ingress / compat shim 注释澄清。
- `StrategicPlanReject` 回灌与 child-stats uncertainty reconcile 小步补强。
- architecture guard 测试，防止重新引入多套 canonical S。

### Out of Scope

- 大规模物理目录迁移。
- 历史 T ID 全量重编号。
- WaveScheduler / D4 delegate 的行为重构。
- D2/D3/D4 其他领域的 S 层重构。

## 6. Demand 级验收标准

- [ ] **P0** D7 canonical S 在 `spec.md` / `a-registry.md` / `code-layout.md` 中一致为 S1-S6。
- [ ] **P0** S7-S14/S20/S21 不再作为 current canonical S 展示，仅作为 historical/contract mapping 保留。
- [ ] **P1** A/F registry 中旧 `observe/execute/verify/learn` 路径修正到当前代码路径。
- [ ] **P1** retired ingress / compat shim 在代码注释和 guard 测试中明确，不再误导为主链路。
- [ ] **P1** StrategicPlanReject 被写入 round / next prompt，可由测试验证。
- [ ] **P1** parent uncertainty reevaluate 使用 child-stats-driven signal，可由测试验证全 pass 收敛。
