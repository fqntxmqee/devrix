# Tasks: MUPS Go-struct-driven I/O contract (M2 Plan)

**Change ID:** `mups-plan-structbind`  
**Demand:** DM-20260705-004
**Status:** S4_Implementation (planned)

## P0

| Task | L4/L5 | Status |
|------|-------|--------|
| T1 strategic_plan_proposer.go: StrategicPlanFrame struct 16 字段 + pt tag | D7-S5-A100 | [ ] |
| T2 strategic_plan_proposer.go: buildStrategicPlanFrame(in) 转换函数 (含 Budget 嵌套展平 + 7 个条件守卫) | D7-S5-A100 | [ ] |
| T3 strategic_plan_proposer.go: init() 调 MustRegisterFrame[StrategicPlanFrame](FramePlanUser) | D7-S5-A100 | [ ] |
| T4 strategic_plan_proposer.go: buildStrategicPlanUserPrompt 35+ 行 → 2 行 (BuildLineFrameFromStruct + RenderFrameFieldGuide) | D7-S5-A100 | [ ] |
| T5 i18n: 补 11 条 plan.input.*.when_use 翻译 (en + zh) | i18n | [ ] |
| T6 plan_structbind_test.go: 5 子测试 (register / build / doc / Budget 展平 / 0 行为变化 4 组合) | D7-S5-A100-T01..T05 | [ ] |
| T7 strategic_plan_proposer_test.go: 现有测试 0 行为变化验证 | D7-S5-A100 | [ ] |
| T8 item_plan_test.go: 现有 E2E 0 行为变化验证 | D7-S5-A100 | [ ] |
| T9 L5-MUPS-GSD-11: MustRegisterFrame[StrategicPlanFrame]() init 成功 | L5 | [ ] |
| T10 L5-MUPS-GSD-12: BuildLineFrameFromStruct 字节等价 buildStrategicPlanUserPrompt | L5 | [ ] |
| T11 L5-MUPS-GSD-13: buildStrategicPlanFrame 平铺 Budget 9 字段与现状手工展开一致 | L5 | [ ] |
| T12 L5-MUPS-GSD-14: 4 项 init panic 校验 (pt 缺 / plane 错 / i18n 缺 / 字段数 == FrameSpec) | L5 | [ ] |
| T13 L5-MUPS-GSD-15: 现有 Plan E2E 套件 0 行为变化 (strategic_plan_proposer + item_plan + parse_reject) | L5 | [ ] |

## P1

| Task | L4/L5 | Status |
|------|-------|--------|
| T14 L5-MUPS-GSD-16: golden snapshot testdata/plan_user_prompt.golden 4 组合 PASS | L5 | [ ] |
| T15 t-registry.md D7-S5-A100 注册 6 T 点 | d7 t-registry | [ ] |
| T16 CHANGELOG.md d7-orchestration 追加一行 | d7 CHANGELOG | [ ] |
| T17 Draft PR 创建 (depends on #403) | git-workflow | [ ] |

## Verification

```bash
# 单包验证
go vet ./internal/shared/prompttags/... ./internal/layers/orchestration/sessionorchestrator/... ./internal/layers/contextengine/i18n/...
go test ./internal/shared/prompttags/... -race -count=1
go test ./internal/layers/orchestration/sessionorchestrator/... -race -count=1
go test ./internal/layers/contextengine/i18n/... -race -count=1

# 全仓回归
go test ./... -race -count=1

# kernel 零代码增量验证
git diff master..HEAD -- internal/shared/prompttags/structbind.go internal/shared/prompttags/linefield.go internal/shared/prompttags/semantics.go
# 期望: 0 行改动 (除非 PR #403 已 merge)

# 行为不变性 (golden)
diff <(git show master:internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go | grep -A 50 'buildStrategicPlanUserPrompt') \
     <(cat internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go | grep -A 5 'buildStrategicPlanUserPrompt')
# 期望: 旧版 35+ 行 → 新版 2 行; user prompt 输出 token 等价
```

## Rollback Plan

- git revert <commit> 一行回滚 (pure refactor, 无数据迁移)
- 旧 BuildAnnotatedLineFrame API 保留 (向后兼容)
- LineFrameRegistry 仍是合法手写入口, MustRegisterFrame 是新增而非替换

## Out-of-scope (不实现)

- M3 Strategy 抽象
- M4 Verify 表驱动
- M5 SpawnDecision 代数化
- 任何 Execute / Verify / Learn 节点改造
- 修改 workmodel.DivergenceBudget 字段
- 修改 PlanUserFrame 字段顺序
