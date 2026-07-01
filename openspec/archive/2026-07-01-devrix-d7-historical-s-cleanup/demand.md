---
demand-id: DM-20260701-003
title: D7 Historical S Cleanup — S7+ 正文迁出与 S3 定位澄清
priority: P1
status: S7_Archived
dsaft_domain: [orchestration]
created: 2026-07-01
reporter: AI 架构审查（DM-20260701-002 后续）
related:
  - DM-20260701-002 (D7 S layer normalization)
---

# Demand: D7 Historical S Cleanup

## 1. 原始诉求

DM-20260701-002 将 D7 canonical S 收敛为 S1-S6 后，用户追问 S3 是否仍有价值、S7+ 场景能否进一步清理。需要在保留追溯能力的前提下，把 current registry 与 historical 正文分离。

## 2. 问题陈述

| ID | 现象 | 根因 | 严重度 |
|----|------|------|--------|
| D7-HC-01 | `a-registry.md` 仍含 S7-S14/S18/S20/S21 大段正文 | 002 仅改 heading 为 Historical，未物理迁出 | P1 |
| D7-HC-02 | `f-registry.md` 仍含 D7-S8–S14 F 段与 fastpath 路径 | 与 current runtime 主链路不一致 | P1 |
| D7-HC-03 | S3 WaveScheduler 定位在 spec 中易被误读为主链路一环 | Architecture 图仍展示 FastPath→WaveScheduler | P1 |
| D7-HC-04 | 缺少独立 historical 追溯文档 | mapping 表分散在 spec/registry 正文 | P2 |

## 3. 业务目标

| ID | 目标 | 可验证承诺 |
|----|------|------------|
| D7-HC-G1 | current registry 只承载 S1-S6 | a/f-registry 不含 S7+ 大段 heading 正文 |
| D7-HC-G2 | 历史 ID 仍可追溯 | `historical-s-mapping.md` 保留完整 mapping + 正文 |
| D7-HC-G3 | S3 定位清晰 | spec 明确 S3 为 explicit wave/background，非用户消息主链路 |
| D7-HC-G4 | 防回归 | guard 测试禁止 S7+ current heading / fastpath 路径回归 |

## 4. L1-L5 映射

| 层级 | 映射 |
|------|------|
| L1 | D7 Orchestration |
| L2 | DSAFT 文档治理 + Wave 调度边界澄清 |
| L3-BE | Registry 清理 + S3 定位 + architecture guard |
| L4 | HistoricalMappingDoc、RegistryTrim、S3BoundaryClarify、DocGuard |
| L5 | D7-HC-T01 ~ D7-HC-T06 |

## 5. In Scope / Out of Scope

### In Scope

- 新增 `historical-s-mapping.md`
- 精简 `a-registry.md` / `f-registry.md`
- 更新 `spec.md` S3 定位与 Architecture 图
- guard 测试扩展
- T 注册与 change 归档

### Out of Scope

- 历史 T ID 重编号
- WaveScheduler 代码行为变更
- 物理目录迁移

## 6. Demand 级验收标准

- [ ] **P0** change 包完整（demand/proposal/design/tasks/delta spec）
- [ ] **P0** `historical-s-mapping.md` 存在且含 summary mapping + 迁出的 A/F 正文
- [ ] **P1** a/f-registry 仅保留 S1-S6 current 段 + 指向 historical 文档
- [ ] **P1** spec.md 明确 S3 非用户消息主链路
- [ ] **P1** guard 测试全绿
