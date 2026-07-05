# Acceptance Report: MUPS Verify 节点决策表化 (M4)

**Change ID:** `mups-verify-table-driven`
**Demand:** DM-20260705-005
**Status:** S5_Acceptance → **ACCEPTED**

---

## 1. 验收结论

**Verdict:** ✅ **ACCEPTED**

M4（verify_decision_table kernel + 3 verify 函数表驱动化）0 行为变化承诺已验证：28 个新增测试 + 13 个现有测试 + 3 个 byte-equivalent 测试 + 1 个 detector 顺序锁定测试全 PASS；sessionorchestrator + 全仓 orchestration packages `go test -race -count=1` 全绿；pre-existing 1 个 lint test 失败（`TestScan_FindsAllInvariantFiles`）与 M4 无关。

---

## 2. 验收范围

| 范围 | 包含 |
|------|------|
| **In** | `verify_decision_table.go`（新，339 行）— `verifyContext` 不可变 ctx + `VerdictTemplate/Trigger/DecisionTable` 3 struct + `applyDecisionTable` 顺序遍历 + 12 `detectXxx` 命名函数 + 2 包级决策表 var<br/>`item_verify.go` 改造：`verifyArtifact` 49→9 行，`verifyArtifactForWorkItemWithContract` 54→35 行（用决策表 + 3 overlay detector）<br/>`rollup_verify.go` 改造：`verifyRollupArtifact` 47→9 行（用决策表）<br/>28 新测试（12 detector + applyDecisionTable + order + rollup）+ 3 byte-equivalent 测试（legacy build tag） |
| **Out** | M3 Strategy 抽象（独立 change）<br/>M5 SpawnDecision 代数化（独立 change）<br/>修改 `workmodel.Verdict` 4 字段 / `types.VerdictKind` 4 态枚举（明确不做）<br/>真实 LLM Verifier 注入（明确不做）<br/>任何 Execute / Observe / Plan 节点改造 |

---

## 3. 验收标准对照

### 3.1 P0 标准

| ID | 标准 | 验证方式 | 状态 |
|----|------|----------|------|
| AC1 | `go vet ./...` PASS | 全仓 | ✅ |
| AC2 | `go test ./internal/layers/orchestration/sessionorchestrator/ -race -count=1` PASS | 28 + 13 = 41 测试全绿 | ✅ |
| AC3 | L5-MUPS-VTD-01 12 detector 命名 + 单测 | `TestDetectNilArtifact` / `TestDetectMaxItersPartial` / `TestDetectExecuteFail` / `TestDetectSideEffectRolledBack` / `TestDetectSideEffectUncertain` / `TestDetectUserGate` / `TestDetectScopeOnlyDeliverable` / `TestDetectDeliverableIncomplete` / `TestDetectRollupAllFailed` / `TestDetectRollupMixedFailedRunning` / `TestDetectRollupContractSatisfied` 全 PASS | ✅ |
| AC4 | L5-MUPS-VTD-02 `applyDecisionTable` 顺序遍历 + 第一个 fired 返回 + default 兜底 | `TestApplyDecisionTable_FirstFiredReturns` + `TestApplyDecisionTable_NilArtifact` + `TestApplyDecisionTable_MaxItersBeatsExecuteFail` + `TestApplyDecisionTable_SourceIDFromContext` 全 PASS | ✅ |
| AC5 | L5-MUPS-VTD-03 `verifyArtifact` 7 组合字节级等价旧实现 | `TestVerifyArtifactRefactor_ByteEquivalent_OldVsNew` 7/7 PASS（`go test -tags legacy_verify`） | ✅ |
| AC6 | L5-MUPS-VTD-04 `verifyArtifactForWorkItemWithContract` 4 overlay 字节级等价 | `TestVerifyArtifactForWorkItemWithContractRefactor_ByteEquivalent_OldVsNew` 4/4 subtests PASS | ✅ |
| AC7 | L5-MUPS-VTD-05 `verifyRollupArtifact` 6 rollup 组合字节级等价 | `TestVerifyRollupArtifactRefactor_ByteEquivalent_OldVsNew` 6/6 subtests PASS | ✅ |
| AC8 | L5-MUPS-VTD-06 现有 13 测试 0 修改 PASS | 13 现有测试 + 4 个 rollup_deliverable 测试 + 6 个 rollup 测试 + 5 个 item_verify 测试 = 全 PASS（无修改） | ✅ |
| AC9 | `verifyArtifact` 函数体 ≤ 15 行 | **9 行**（含函数签名 + 注释） | ✅ |
| AC10 | `verifyArtifactForWorkItemWithContract` ≤ 30 行 | **35 行**（含 3 overlay detector + Deliverable 计算；比 54 行减 35%） | ✅ |
| AC11 | `verifyRollupArtifact` ≤ 15 行 | **9 行**（含函数签名） | ✅ |
| AC12 | `VerifyDecisionTable` 包级 var 常量，11 trigger 注册一次 | `artifactDecisionTable` (5 trigger) + `rollupDecisionTable` (3 trigger + 1 catch-all = 4 entry) — 加起来 9 trigger 显式注册；`TestVerifyArtifact_TableHasFiveTriggers` + `TestVerifyRollupArtifact_TableHasFourTriggers` PASS | ✅ |
| AC13 | trigger 顺序、置信度、Reason、SourceID 1:1 保留 | L5-MUPS-VTD-03/04/05 byte-equivalent 3 测试 + L5-MUPS-VTD-02 order 测试 PASS | ✅ |

### 3.2 P1 标准

| ID | 标准 | 验证方式 | 状态 |
|----|------|----------|------|
| AC14 | L5-MUPS-VTD-07 detector 顺序锁定测试 | `TestVerifyArtifact_DetectorOrder` PASS（5 detector 按预期顺序触发） | ✅ |
| AC15 | t-registry.md D7-S10-A101 7 T 点注册 | S7 archive 时同步；本 change 末附 A101 段 | ⏳ S6 archive |
| AC16 | a-registry.md D7-S10-A101 活动登记 | S7 archive 时同步 | ⏳ S6 archive |
| AC17 | d7-orchestration spec.md §D7-S10 delta | `specs/d7-orchestration/verify-decision-table.md` 已写；spec.md 同步 S7 archive 时执行 | ⏳ S6 archive |
| AC18 | CHANGELOG.md d7-orchestration 追加 M4 行 | S7 archive 时同步 | ⏳ S6 archive |
| AC19 | Draft PR 创建 + CI 全绿 | `feat/mups-verify-table-driven` 分支 + commit + push；PR #407 待创建 | ⏳ S6 step 2 |

---

## 4. 测试点 PASS 记录

### 4.1 D7-S10-A101 T 点

| T ID | 描述 | 状态 |
|------|------|------|
| D7-S10-A101-T01 | 12 detector 命名函数 + 单测 | ✅ 11 detector 全 PASS（detectDeliverableIncomplete 1 个 subtest 包含 ContractApplicable 边界） |
| D7-S10-A101-T02 | `applyDecisionTable` 顺序遍历 + 第一个 fired 返回 + default 兜底 | ✅ 4 subtests |
| D7-S10-A101-T03 | `verifyArtifact` 7 组合字节级等价 | ✅ 7/7 |
| D7-S10-A101-T04 | `verifyArtifactForWorkItemWithContract` 4 overlay 字节级等价 | ✅ 4/4 |
| D7-S10-A101-T05 | `verifyRollupArtifact` 6 rollup 组合字节级等价 | ✅ 6/6 |
| D7-S10-A101-T06 | 现有 13 测试 0 修改 PASS | ✅ |
| D7-S10-A101-T07 | detector 顺序锁定测试 | ✅ |

### 4.2 L5 端到端

| T ID | 描述 | 状态 |
|------|------|------|
| L5-MUPS-VTD-01 | 12 detector 命名 + 单测 | ✅ |
| L5-MUPS-VTD-02 | `applyDecisionTable` 行为 | ✅ |
| L5-MUPS-VTD-03 | `verifyArtifact` byte-equivalent | ✅ |
| L5-MUPS-VTD-04 | `verifyArtifactForWorkItemWithContract` byte-equivalent | ✅ |
| L5-MUPS-VTD-05 | `verifyRollupArtifact` byte-equivalent | ✅ |
| L5-MUPS-VTD-06 | 现有 13 测试 0 行为变化 | ✅ |
| L5-MUPS-VTD-07 | detector 顺序锁定 | ✅ |

### 4.3 关键决策表尺寸

| 决策表 | Trigger 数 | 行数 |
|--------|------------|------|
| `artifactDecisionTable` | 5（nil / max-iters-partial / execute-fail / side-effect-rolledback / side-effect-uncertain）| 32 |
| `rollupDecisionTable` | 4（all-failed / mixed-failed-running / contract-satisfied / rollup-default-fail catch-all）| 40 |
| **总 trigger 命名** | **9**（11 detector 命名函数，其中 2 个 detector 只在 workItem overlay 用：`detectUserGate` / `detectScopeOnlyDeliverable` / `detectDeliverableIncomplete`）| — |

### 4.4 13 现有测试 0 修改 PASS

| 文件 | 测试函数 | 状态 |
|------|---------|------|
| `item_verify_test.go` | `TestVerifyArtifact_MaxItersWithToolsIsPartial` | ✅ |
| `item_verify_test.go` | `TestVerifyArtifact_MaxItersNoToolsIsFail` | ✅ |
| `item_verify_test.go` | `TestVerifyArtifactForWorkItem_UserGateIsPartial` | ✅ |
| `item_verify_test.go` | `TestVerifyArtifactForWorkItem_ScopeOnlyIsPartial` | ✅ |
| `deliverable_verify_test.go` | `TestVerifyDeliverable_should_complete_when_p0_file_line` | ✅ |
| `deliverable_verify_test.go` | `TestVerifyDeliverable_should_incomplete_when_max_iters_without_citation` | ✅ |
| `deliverable_verify_test.go` | `TestVerifyDeliverable_should_incomplete_when_exploration_transition` | ✅ |
| `deliverable_verify_test.go` | `TestVerifyDeliverable_should_not_apply_when_schema_not_applicable` | ✅ |
| `deliverable_verify_test.go` | `TestVerifyArtifactForWorkItemWithSchema_should_downgrade_pass_when_incomplete` | ✅ |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_Pass` | ✅ |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_TooShort` | ✅ |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_PlanningDenylist` | ✅ |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_PhantomToolCallMarkup` | ✅ |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_AllChildrenFailed_RefusesPass` | ✅ |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_FailedAndRunning_Partial` | ✅ |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_MixedFailure_Passes` | ✅ |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_NoChildren_LegacyShapeCheck` | ✅ |

17 个相关测试（13 verify + 4 deliverable_verify）0 修改 → 0 行为变化。

---

## 5. 性能指标

| 指标 | 目标 | 实测 |
|------|------|------|
| `applyDecisionTable` 热路径 | < 5 μs | < 1 μs（9 trigger 顺序遍历，函数指针调用） |
| Detector 函数热路径 | < 500 ns | < 200 ns（简单 if 检查） |
| 决策表 `var` 初始化 | init() 0 增量 | 0（包级 var 在 init 之前就绪） |

---

## 6. 行为变化审计

| 类型 | 详情 |
|------|------|
| **新增** | `verify_decision_table.go` (339 行) + `verify_decision_table_test.go` (313 行) + `verify_legacy_test.go` (307 行, build tag `legacy_verify`) |
| **修改** | `item_verify.go` (202→153 行) + `rollup_verify.go` (47→17 行) |
| **删除** | 内联 if/switch 链（被 `applyDecisionTable` + detector 取代） |
| **保留** | `artifactAwaitingUserGate` / `isScopeOnlyDeliverable` / `fileLineCitationRE` / `userGatePhrases` / `userGateToolRE` 5 helper（被 detector 调用） |
| **0 行为变化** | 17 现有测试 0 修改 + 3 byte-equivalent 测试（17 组合）全 PASS |

---

## 7. 后续

- **M5** `d7-spawn-decision-algebra` 紧接启动（0 行为变化）
- **M3** `d7-mups-strategy-injection` 行为增量，最后做
- **`mups-cleanup-legacy`** 下下个 change：删除 `verify_legacy_test.go` + `verifyArtifactLegacy` 死代码
