---
demand-id: DM-20260705-004
title: "MUPS Go-struct-driven I/O contract — M2 Plan 节点独立化 + 复用 M1 kernel"
source: DM-20260705-003 (M1) section 2.4 重构总图 / RH-MUPS-07 T-P1-2 (Budget self-bound)
priority: P1
status: S1_Demand
l1-domain: orchestration
created: 2026-07-05
related:
  - DM-20260705-003
  - openspec/specs/shared/prompttags.md
  - openspec/specs/d7-orchestration/spec.md
  - openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md
  - internal/shared/prompttags/structbind.go
  - internal/shared/prompttags/linefield.go
  - internal/shared/prompttags/semantics.go
  - internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go
  - internal/layers/orchestration/sessionorchestrator/observation_proposer.go
parent_demands:
  - DM-20260705-003
---

# MUPS Go-struct-driven I/O contract — M2 Plan 节点

## 1. 原始描述

复用 DM-20260705-003 (M1) 的 prompttags/structbind.go 反射 kernel, 对 Plan 节点执行与 Observe 节点同模式的 go-struct 化: StrategicPlanInput 加 pt struct tag, buildStrategicPlanUserPrompt 35+ 行手工 map 折叠为 1 行 BuildLineFrameFromStruct 调用. kernel 零代码增量. 重点处理 Budget 嵌套 struct (workmodel.DivergenceBudget 含 9 字段) 平铺到 frame 视角的设计契约.

## 2. 问题陈述

### 2.1 Plan 节点现状

| 资产 | 状态 | 路径 |
|------|------|------|
| StrategicPlanInput struct (9 字段) | done | strategic_plan_proposer.go:17-32 |
| PlanUserFrame FrameSpec (16 TagName) | done | prompttags/linefield.go:55-74 |
| buildStrategicPlanUserPrompt 35+ 行手工 map | done | strategic_plan_proposer.go:133-171 |
| i18n plan.input.*.when_use 翻译 (5 条) | partial | contextengine/i18n/prompttags_semantics_{en,zh}.go |
| applyBudgetCap / applySingleModeUncertaintyGate | done | 同文件 line 348+ |

### 2.2 与 Observe 节点的对称缺口

M1 已消除 Observe 节点三处描述漂移. Plan 节点存在完全相同的漂移风险:

| 描述同一份 schema 的三处 | 现状 |
|--------------------------|------|
| (a) StrategicPlanInput struct 字段顺序 | 9 字段 (嵌套 Budget) |
| (b) PlanUserFrame.FrameSpec.Fields | 16 TagName (Budget 平铺为 9 字段) |
| (c) buildStrategicPlanUserPrompt 手工 map | 35+ 行, 6 个条件分支 |

### 2.3 M2 独有设计点: 嵌套 struct 平铺

Observe 节点的 ObserveSignalInput 是 9 字段平铺 struct. Plan 节点的 StrategicPlanInput 含嵌套 Budget workmodel.DivergenceBudget (9 字段). M2 决策:

| 方案 | 说明 | 取舍 |
|------|------|------|
| A: 转换 struct 模式 (采纳) | StrategicPlanInput 保持 domain 语义; 新增 StrategicPlanFrame 平铺 struct (16 字段, 与 PlanUserFrame 1:1); buildStrategicPlanFrame(in) 转换函数把 Budget 展平 | kernel 零代码增量; D7 域内可控; 与 M1 对称 |
| B: kernel 扩展 pt flatten flag | 反射时自动拍平嵌套 struct 字段 | 改 kernel 破坏 M1 0 行为变化承诺 |
| C: 保持手工 map | 不重构 | 漂移风险持续累积 |

选 A. 理由: 与 M1 对称, kernel 零代码增量, 嵌套平铺属于 D7 编排域内职责.

### 2.4 目标行为

1. 单一定义点: StrategicPlanFrame 是 Plan user frame 唯一权威 (16 字段).
2. 反射注册: MustRegisterFrame[StrategicPlanFrame]() 在 init() 调用一次, 校验 pt tag × 16, FrameSpec 长度 == 16, i18n >= 16.
3. 反射序列化: buildStrategicPlanUserPrompt 35+ 行 → 1 行 BuildLineFrameFromStruct.
4. 0 行为变化 (M2 阶段): golden snapshot + E2E 全 PASS; Budget 字段顺序, plane, conditional 跳过逻辑完全保留.
5. 嵌套平铺契约: buildStrategicPlanFrame(in) 是唯一允许 Budget 展平的地方; StrategicPlanInput 保持原 struct 不动.

### 2.5 重构总图位置

```
M1 done (kernel + Observe) → M2 (本 change) → (M4 par M5) → M3 (行为增量, 最后)
```

## 3. 非目标

- M3 Strategy 抽象
- M4 Verify 表驱动
- M5 SpawnDecision 代数化
- 修改 workmodel.DivergenceBudget 字段
- 修改 PlanUserFrame 字段顺序
- 修改 applyBudgetCap / applySingleModeUncertaintyGate 业务逻辑
- 复活 ChannelRouter 4 个 channel 文件

## 4. 澄清记录

### Q1: M2 是否需要修改 M1 kernel?

否. MustRegisterFrame / BuildLineFrameFromStruct / DocBlockFromStruct 已经是泛型 kernel; M2 只需 (1) 新增 StrategicPlanFrame struct + pt tag, (2) init() 注册, (3) 业务代码 1 行化. kernel 零代码增量.

### Q2: 嵌套 Budget struct 怎么平铺?

在 strategic_plan_proposer.go 新增 buildStrategicPlanFrame(in) 转换函数. Budget.MaxChildren > 0 守卫保留 (与现状一致). StrategicPlanInput 保持原 9 字段 domain 语义不变.

### Q3: i18n 翻译条目覆盖度?

现状 5 条 plan.input.*.when_use; PlanUserFrame 16 字段需要 >= 16 条翻译. M2 补 11 条缺失: work_item_id / observation_ids / depth / max_depth / existing_children / max_children / decompose_used_today / remaining_daily / max_daily / max_iters / parent_scope_in (en + zh 各 11 条).

### Q4: 0 行为变化验证?

golden snapshot (testdata/plan_user_prompt.golden) 覆盖 4 组合: (i) Budget = 0; (ii) Budget > 0 + 无 ObservationIDs/Summary; (iii) Budget > 0 + ParentScopeIn 非空; (iv) Budget > 0 + UncertaintyMean > 0 + PriorParseReject 非空. snapshot diff = 0 即通过.

## 5. L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | orchestration | MUPS 5 节点 | 已有 |
| L1 | shared | prompttags | 已有 |
| L3-BE | D7-S5 | Plan 战略提案 | 改造 |
| L4-BE | D7-S5-A100 | Plan 节点 go-struct 化 (复用 M1 kernel) | 新增 |
| L5 | L5-MUPS-GSD-11 | MustRegisterFrame[StrategicPlanFrame]() init 成功 | P0 |
| L5 | L5-MUPS-GSD-12 | BuildLineFrameFromStruct 字节等价 buildStrategicPlanUserPrompt | P0 |
| L5 | L5-MUPS-GSD-13 | buildStrategicPlanFrame 平铺 Budget 9 字段与现状手工展开一致 | P0 |
| L5 | L5-MUPS-GSD-14 | 4 项 init panic 校验 | P0 |
| L5 | L5-MUPS-GSD-15 | 现有 Plan E2E 0 行为变化 | P0 |
| L5 | L5-MUPS-GSD-16 | golden snapshot 4 组合 PASS | P1 |

## 6. 验收标准

- P0: go vet ./... + 3 目标包 go test -race -count=1 全 PASS
- P0: 6 L5 测试点 (L5-MUPS-GSD-11..15) 全 PASS
- P0: buildStrategicPlanUserPrompt 函数体 <= 5 行 (含签名)
- P0: StrategicPlanFrame 字段数 == PlanUserFrame.Fields 长度 == 16
- P0: i18n 翻译条目 >= 16 条 plan.input.*.when_use (en + zh 各 16)
- P1: golden snapshot 4 组合 PASS

## 7. 规划状态

- [x] S1 demand.md
- [ ] S2 proposal.md
- [ ] S3 design.md
- [ ] S3-Gate 设计审查通过
- [ ] S4 tasks.md + 实现
- [ ] S4-Gate go vet + go test -race
- [ ] S5 验收报告
- [ ] S6-交付 PR squash
- [ ] S6-归档 archive/2026-07-05-mups-plan-structbind/
