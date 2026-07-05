# Tasks: MUPS Spawn 决策代数化 (M5)

**Change ID:** `d7-spawn-decision-algebra`  
**Demand:** DM-20260705-006
**Status:** S4_Implementation (planned)

## P0

| Task | L4/L5 | Status |
|------|-------|--------|
| T1 `spawn_decision_algebra.go`: `normalizeCtx(ctx) TreeEvalContext` 5 行 default 兜底 helper | D7-S15-A102 | [ ] |
| T2 `spawn_decision_algebra.go`: `checkBudget(round, ctx) (SpawnPolicy, bool)` R0/R0.5/R1/R2 4 budget gates (~25 行) | D7-S15-A102 | [ ] |
| T3 `spawn_decision_algebra.go`: `checkRollupGuard(round, ctx) (SpawnPolicy, bool)` 跨 verdict rollup retry exhausted guard (~10 行) | D7-S15-A102 | [ ] |
| T4 `spawn_decision_algebra.go`: `checkVerdictDirection(round, ctx) SpawnPolicy` R3..R8 switch on VerdictKind (~35 行) | D7-S15-A102 | [ ] |
| T5 `spawn_decision_algebra.go`: 3 子决策 + normalizeCtx 总览注释 + 顺序契约注释 | D7-S15-A102 | [ ] |
| T6 `spawn_policy.go`: `SpawnPolicyEvaluator` 重构为 nil round 兜底 + `ctx = normalizeCtx(ctx)` + 3 步 `if p, fired := checkXxx(...); fired { return p }` + `return checkVerdictDirection(...)` (50+→8 行) | D7-S15-A102 | [ ] |
| T7 `spawn_decision_algebra_test.go`: `TestCheckBudget_*` 6 case (R0/R0.5/R1 w/ cont/R1 w/ exhaust/R1 no schema/R2 + fall-through) | D7-S15-A102-T01 | [ ] |
| T8 `spawn_decision_algebra_test.go`: `TestCheckRollupGuard_*` 4 case (at-limit escalate/below-limit inline/non-rollup fall-through/Pass+RollupRound inline) | D7-S15-A102-T02 | [ ] |
| T9 `spawn_decision_algebra_test.go`: `TestCheckVerdictDirection_*` 5 case (Pass+R3/R4/Pass w/ cont+CC-1/Partial+R5/Fail+R6/Indeterminate+R7) | D7-S15-A102-T03 | [ ] |
| T10 `spawn_decision_algebra_test.go`: `TestNormalizeCtx` 5 字段 default 兜底单测 | D7-S15-A102-T04 | [ ] |
| T11 `spawn_decision_algebra_test.go`: `TestSpawnPolicyEvaluator_SubDecisionOrder` 3 子决策按预期顺序被调（call counter 验证） | D7-S15-A102-T07 | [ ] |
| T12 `spawn_policy_legacy_test.go`: 旧 `SpawnPolicyEvaluatorLegacy` 50+ 行实现保留 + build tag `legacy_spawn` | D7-S15-A102 | [ ] |
| T13 `spawn_policy_legacy_test.go`: `TestSpawnPolicyEvaluatorRefactor_ByteEquivalent_OldVsNew` 22 组合字节级对比 | D7-S15-A102-T05 | [ ] |
| T14 现有 22 测试 0 修改 PASS (21 + 1 inline) | D7-S15-A102-T06 | [ ] |

## P1

| Task | L4/L5 | Status |
|------|-------|--------|
| T15 `t-registry.md` D7-S15-A102 注册 7 T 点 + shared 索引同步 | d7 t-registry | [ ] |
| T16 `a-registry.md` D7-S15-A102 活动登记（spawn_decision_algebra kernel + 3 sub-decision + normalizeCtx） | d7 a-registry | [ ] |
| T17 `specs/d7-orchestration/spawn-decision-algebra.md` delta 新增 spawn_decision_algebra Requirement + 3 sub-decision + normalizeCtx | d7 spec.md | [ ] |
| T18 `CHANGELOG.md` d7-orchestration 追加 M5 行 | d7 CHANGELOG | [ ] |
| T19 `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md` 标记 M5 完成 | d7 spec | [ ] |
| T20 `demand-archive-index.md` DM-20260705-006 入口 | d7 demand-archive-index | [ ] |
| T21 Draft PR 创建 + 标 ready | git-workflow | [ ] |

## Verification

```bash
# 单包验证
go vet ./internal/layers/orchestration/workmodel/...
go test ./internal/layers/orchestration/workmodel/ -run "SpawnPolicyEvaluator|EvaluateSpawnPolicy" -race -count=1 -v
go test ./internal/layers/orchestration/workmodel/ -race -count=1  # 全部

# 全仓回归
go test ./... -race -count=1

# 行为不变性（手工 — 22 现有测试 PASS 即证明）
# byte-equivalent 测试需要 build tag
go test -tags legacy_spawn ./internal/layers/orchestration/workmodel/ -run "TestSpawnPolicyEvaluatorRefactor" -race -count=1 -v
```

## Rollback Plan

- `git revert <commit>` 一行回滚（pure refactor，无数据迁移）
- `spawn_decision_algebra.go` 是新增；`spawn_policy.go` 主函数体可一键 revert 到 50+ 行 if/switch 链
- `spawn_policy_legacy_test.go` 删除可单独 revert（不影响生产代码）

## Out-of-scope（不实现）

- M3 Strategy 抽象
- 修改 `SpawnPolicy` 6 态枚举
- 修改 `WorkItemPipelineRound` / `TreeEvalContext` 2 struct 字段
- 修改 `EvaluateSpawnPolicy` / `spawnRationale` / 5 个 deliverable helper 行为
- 修改 `spawnForDeliverableContinuation` / `RollupSynthEligible` / `IsExploratoryPlanKind` / `CanDecompose` 4 个依赖
- 任何 Execute / Observe / Plan / Verify 节点改造
- 跨域 LLM 节点改造
- 复活 ChannelRouter 4 文件
