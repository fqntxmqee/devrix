# Tasks: MUPS Verify 节点决策表化 (M4)

**Change ID:** `mups-verify-table-driven`  
**Demand:** DM-20260705-005
**Status:** S4_Implementation (planned)

## P0

| Task | L4/L5 | Status |
|------|-------|--------|
| T1 `verify_decision_table.go`: `verifyContext` 不可变 struct（art/item/pl/contract/stats/id 字段） | D7-S10-A101 | [ ] |
| T2 `verify_decision_table.go`: `VerdictTemplate` struct（Kind/Confidence/Reason/ReasonFunc/IndeterminateReason） | D7-S10-A101 | [ ] |
| T3 `verify_decision_table.go`: `VerdictTrigger` struct（Name/Fire/Template） | D7-S10-A101 | [ ] |
| T4 `verify_decision_table.go`: `VerifyDecisionTable` type + `applyDecisionTable(table, art, ctx) Verdict` | D7-S10-A101 | [ ] |
| T5 `verify_decision_table.go`: 5 artifact detector（`detectNilArtifact` / `detectMaxItersPartial` / `detectExecuteFail` / `detectSideEffectRolledBack` / `detectSideEffectUncertain`） | D7-S10-A101 | [ ] |
| T6 `verify_decision_table.go`: 3 workItem overlay detector（`detectUserGate` / `detectScopeOnlyDeliverable` / `detectDeliverableIncomplete`） | D7-S10-A101 | [ ] |
| T7 `verify_decision_table.go`: 3 rollup detector（`detectRollupAllFailed` / `detectRollupMixedFailedRunning` / `detectRollupContractSatisfied`）+ 1 guard（`art == nil` 或 `Error/ExitCode` → verifyArtifact 委托） | D7-S10-A101 | [ ] |
| T8 `verify_decision_table.go`: `artifactDecisionTable` + `rollupDecisionTable` 包级 var 构造 | D7-S10-A101 | [ ] |
| T9 `item_verify.go`: `verifyArtifact` 重构为"buildCtx + applyDecisionTable" 2 步（49→15 行） | D7-S10-A101 | [ ] |
| T10 `item_verify.go`: `verifyArtifactForWorkItemWithContract` 重构为"applyDecisionTable + 3 overlay detector"（54→30 行） | D7-S10-A101 | [ ] |
| T11 `rollup_verify.go`: `verifyRollupArtifact` 重构为"nil/Error/ExitCode guard + applyDecisionTable"（47→10 行） | D7-S10-A101 | [ ] |
| T12 `verify_decision_table_test.go`: 12 detector 单元测试（每个 detector 1-2 case 含 fire true/false） | D7-S10-A101-T01 | [ ] |
| T13 `verify_decision_table_test.go`: `applyDecisionTable` 行为测试（顺序遍历 + 第一个 fired 返回 + default 兜底） | D7-S10-A101-T02 | [ ] |
| T14 `verify_legacy_test.go`: 旧 3 verify 函数保留为 `verifyArtifactLegacy` 等 + build tag `legacy_verify` | D7-S10-A101 | [ ] |
| T15 `verify_legacy_test.go`: `TestVerifyArtifactRefactor_ByteEquivalent_OldVsNew` 7 组合字节级对比 | D7-S10-A101-T03 | [ ] |
| T16 `verify_legacy_test.go`: `TestVerifyArtifactForWorkItemWithContractRefactor_ByteEquivalent_OldVsNew` 4 overlay 字节级 | D7-S10-A101-T04 | [ ] |
| T17 `verify_legacy_test.go`: `TestVerifyRollupArtifactRefactor_ByteEquivalent_OldVsNew` 6 rollup 组合字节级 | D7-S10-A101-T05 | [ ] |
| T18 `verify_decision_table_test.go`: `TestVerifyArtifact_DetectorOrder` 11 detector 顺序断言 | D7-S10-A101-T07 | [ ] |
| T19 现有 13 测试 0 修改 PASS | D7-S10-A101-T06 | [ ] |

## P1

| Task | L4/L5 | Status |
|------|-------|--------|
| T20 `t-registry.md` D7-S10-A101 注册 7 T 点 + shared 索引同步 | d7 t-registry | [ ] |
| T21 `a-registry.md` D7-S10-A101 活动登记（verify_decision_table kernel + 3 verify 函数走表） | d7 a-registry | [ ] |
| T22 `specs/d7-orchestration/spec.md` §D7-S10 delta 新增 verify_decision_table Requirement | d7 spec.md | [ ] |
| T23 `CHANGELOG.md` d7-orchestration 追加 M4 行 | d7 CHANGELOG | [ ] |
| T24 `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md` 标记 M4 完成 | d7 spec | [ ] |
| T25 Draft PR 创建 + 标 ready | git-workflow | [ ] |

## Verification

```bash
# 单包验证
go vet ./internal/layers/orchestration/sessionorchestrator/...
go test ./internal/layers/orchestration/sessionorchestrator/ -run "VerifyArtifact|VerifyRollupArtifact" -race -count=1 -v
go test ./internal/layers/orchestration/sessionorchestrator/ -race -count=1  # 全部

# 全仓回归
go test ./... -race -count=1

# 行为不变性（手工 — 13 现有测试 PASS 即证明）
# 不需要额外手工 diff；byte-equivalent 测试自动跑
```

## Rollback Plan

- `git revert <commit>` 一行回滚（pure refactor，无数据迁移）
- 决策表 12 detector + 2 var 是新增；3 verify 函数体可一键 revert 到 if/switch 链
- `_legacy_test.go` 删除可单独 revert（不影响生产代码）

## Out-of-scope（不实现）

- M3 Strategy 抽象
- M5 SpawnDecision 代数化
- 修改 `workmodel.Verdict` 4 字段 / `types.VerdictKind` 4 态枚举
- 真实 LLM Verifier 注入
- 任何 Execute / Observe / Plan 节点改造
- 跨域 LLM 节点改造
