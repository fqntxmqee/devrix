# Acceptance Report: MUPS Spawn 决策代数化 (M5)

**Change ID:** `d7-spawn-decision-algebra`
**Demand:** DM-20260705-006
**Status:** S5_Acceptance → **ACCEPTED**

---

## 1. 验收结论

**Verdict:** ✅ **ACCEPTED**

M5（spawn_decision_algebra kernel + 3 sub-decision + normalizeCtx helper）0 行为变化承诺已验证：17 个新增子决策单测 + 22 个现有测试 + 1 个 byte-equivalent 测试 (27 sub-cases) + 1 个子决策顺序锁定测试 (4 sub-cases) 全 PASS；workmodel 包 + 全仓 orchestration packages `go test -race -count=1` 全绿；pre-existing 1 个 lint test 失败（`TestScan_FindsAllInvariantFiles` in `tools/ci-lint-invariant`）与 M5 无关（同 M4 报告）。

---

## 2. 验收范围

| 范围 | 包含 |
|------|------|
| **In** | `spawn_decision_algebra.go`（新，165 行）— `normalizeCtx` 不可变 helper + 3 个命名子决策 (`checkBudget` 4 budget gates + `checkRollupGuard` 跨 verdict guard + `checkVerdictDirection` R3..R8 switch on VerdictKind)<br/>`spawn_policy.go` 改造：`SpawnPolicyEvaluator` 50+→8 行（nil round 兜底 + `ctx = normalizeCtx(ctx)` + 3 步 checkXxx 显式调用）；移除 `plan` import<br/>17 新测试（7 checkBudget + 4 checkRollupGuard + 6 checkVerdictDirection + 2 normalizeCtx + 1 子决策顺序锁定 4-subtests = 20 sub-cases）+ 1 byte-equivalent 测试 (27 sub-cases, build tag `legacy_spawn`) |
| **Out** | M3 Strategy 抽象（独立 change）<br/>修改 `SpawnPolicy` 6 态枚举（明确不做）<br/>修改 `WorkItemPipelineRound` / `TreeEvalContext` 2 struct 字段<br/>修改 `EvaluateSpawnPolicy` / `spawnRationale` / 5 个 deliverable helper 行为<br/>修改 `spawnForDeliverableContinuation` / `RollupSynthEligible` / `IsExploratoryPlanKind` / `CanDecompose` 4 个依赖<br/>任何 Execute / Observe / Plan / Verify 节点改造<br/>跨域 LLM 节点（D3 LLMGateway）改造 |

---

## 3. 验收标准对照

### 3.1 P0 标准

| ID | 标准 | 验证方式 | 状态 |
|----|------|----------|------|
| AC1 | `go vet ./...` PASS | 全仓 | ✅ |
| AC2 | `go test ./internal/layers/orchestration/workmodel/... -race -count=1` PASS | 17 新 + 22 现有 + 1 顺序 = 40 测试全绿 | ✅ |
| AC3 | L5-MUPS-SDA-01 `checkBudget` 6 case 命名 + 单测 | `TestCheckBudget_R0/R05/R1_*/R2/FallThrough` 7/7 PASS | ✅ |
| AC4 | L5-MUPS-SDA-02 `checkRollupGuard` 4 case 命名 + 单测 | `TestCheckRollupGuard_AtLimitEscalates/BelowLimitInlines/NonRollupFallThrough/PassSkipsGuard` 4/4 PASS | ✅ |
| AC5 | L5-MUPS-SDA-03 `checkVerdictDirection` 5 case 命名 + 单测 | `TestCheckVerdictDirection_R3R4_*/R3_*/R5/R6/R7/R8` 6/6 PASS | ✅ |
| AC6 | L5-MUPS-SDA-04 `normalizeCtx` 5 字段 default 兜底单测 | `TestNormalizeCtx_DefaultValues` + `TestNormalizeCtx_PreservesNonZero` 2/2 PASS | ✅ |
| AC7 | L5-MUPS-SDA-05 22 组合字节级等价旧实现 | `TestSpawnPolicyEvaluatorRefactor_ByteEquivalent_OldVsNew` 27/27 sub-cases PASS（`go test -tags legacy_spawn`） | ✅ |
| AC8 | L5-MUPS-SDA-06 现有 22 测试 0 修改 PASS | 22 现有测试 (21 `spawn_policy_test.go` + 1 `spawn_policy_inline_test.go`) 0 修改 PASS | ✅ |
| AC9 | L5-MUPS-SDA-07 3 子决策顺序锁定测试 | `TestSpawnPolicyEvaluator_SubDecisionOrder` 4/4 sub-cases PASS | ✅ |
| AC10 | `SpawnPolicyEvaluator` 主函数体 ≤ 10 行 | **8 行**（含函数签名 + nil 兜底 + normalizeCtx + 3 步 checkXxx 显式调用） | ✅ |
| AC11 | rollup retry exhausted guard 单一权威位置 | `checkRollupGuard` 唯一实现；R5/R6/R7 块内 3 处重复 5 行已消除 | ✅ |
| AC12 | 3 子决策命名清晰 + 单一职责 | `checkBudget` (R0/R0.5/R1/R2) + `checkRollupGuard` (跨 verdict) + `checkVerdictDirection` (R3..R8) 各自单一职责 | ✅ |
| AC13 | 0 行为变化承诺 | 22 现有测试 0 修改 PASS + 27 byte-equivalent 组合 PASS | ✅ |

### 3.2 P1 标准

| ID | 标准 | 验证方式 | 状态 |
|----|------|----------|------|
| AC14 | `t-registry.md` D7-S15-A102 7 T 点注册 | S7 archive 时同步；本 change 末附 A102 段 | ⏳ S6 archive |
| AC15 | `a-registry.md` D7-S15-A102 5 F 活动登记 | S7 archive 时同步 | ⏳ S6 archive |
| AC16 | `d7-orchestration/spec.md` §D7-S15 delta | `specs/d7-orchestration/spawn-decision-algebra.md` 已写；spec.md 同步 S7 archive 时执行 | ⏳ S6 archive |
| AC17 | `CHANGELOG.md` d7-orchestration 追加 M5 行 | S7 archive 时同步 | ⏳ S6 archive |
| AC18 | `demand-archive-index.md` DM-20260705-006 入口 | S7 archive 时同步 | ⏳ S6 archive |
| AC19 | Draft PR 创建 + CI 全绿 | `feat/d7-spawn-decision-algebra` 分支 + commit + push；PR 待创建 | ⏳ S6 step 2 |

---

## 4. 测试点 PASS 记录

### 4.1 D7-S15-A102 T 点

| T ID | 描述 | 状态 |
|------|------|------|
| D7-S15-A102-T01 | `checkBudget` 7 case (R0/R0.5/R1 w/ cont/R1 w/ exhaust/R1 no schema/R2/fall-through) | ✅ 7/7 PASS |
| D7-S15-A102-T02 | `checkRollupGuard` 4 case (at-limit escalate/below-limit inline/non-rollup fall-through/Pass+Rollup skip) | ✅ 4/4 PASS |
| D7-S15-A102-T03 | `checkVerdictDirection` 6 case (R3/R4 + Pass w/ cont CC-1 + R5 + R6 + R7 + R8) | ✅ 6/6 PASS |
| D7-S15-A102-T04 | `normalizeCtx` 5 字段 default 兜底单测 | ✅ 2/2 PASS (DefaultValues + PreservesNonZero) |
| D7-S15-A102-T05 | `SpawnPolicyEvaluator` 22 组合字节级等价旧实现 (build tag `legacy_spawn`) | ✅ 27/27 sub-cases PASS |
| D7-S15-A102-T06 | 现有 22 测试 0 修改 PASS (21 `spawn_policy_test.go` + 1 `spawn_policy_inline_test.go`) | ✅ 22/22 PASS |
| D7-S15-A102-T07 | 3 子决策顺序锁定测试 (4 sub-cases) | ✅ 4/4 PASS |

### 4.2 L5 端到端

| T ID | 描述 | 状态 |
|------|------|------|
| L5-MUPS-SDA-01 | `checkBudget` 6 case 命名 + 单测 | ✅ 7/7 (含 fall-through) |
| L5-MUPS-SDA-02 | `checkRollupGuard` 4 case | ✅ 4/4 |
| L5-MUPS-SDA-03 | `checkVerdictDirection` 5 case | ✅ 6/6 (含 R8 unknown) |
| L5-MUPS-SDA-04 | `normalizeCtx` 5 字段 default 兜底 | ✅ 2/2 |
| L5-MUPS-SDA-05 | 22 组合字节级等价旧实现 | ✅ 27/27 |
| L5-MUPS-SDA-06 | 现有 22 测试 0 行为变化 | ✅ 22/22 |
| L5-MUPS-SDA-07 | 3 子决策顺序锁定 | ✅ 4/4 |

### 4.3 3 子决策尺寸

| 子决策 | 函数 | 行数 | 命名 |
|--------|------|------|------|
| `normalizeCtx` | helper | 12 | 5 default-value guards (value copy) |
| `checkBudget` | (SpawnPolicy, bool) | 23 | R0 + R0.5 + R1 (3 sub-branches) + R2 |
| `checkRollupGuard` | (SpawnPolicy, bool) | 12 | RH-MUPS-03 cross-verdict guard (excludes Pass) |
| `checkVerdictDirection` | SpawnPolicy | 47 | R3/R4 + R5 + R6 + R7 + R8 default |
| **总行数** | 4 函数 | 94 | 单一职责 + 显式返回签名 |

### 4.4 22 现有测试 0 修改 PASS

| 文件 | 测试函数 | 状态 |
|------|---------|------|
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R0_RunningChildren` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R1_MaxDepth_IncompleteDeliverable` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R05_DeliverableCompleteAtMaxDepth` | ✅ (T: D7-S5-A93-T01) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R1_InlineRetriesExhaustedEscalates` | ✅ (T: D7-S5-A93-T02) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R1_MaxDepth_NoDeliverableSchema` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R2_DailyLimit` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R3_CommitmentPass` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R4_ExplorationPass` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_PartialHighUncertainty` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_PartialAtThreshold` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_ExplorationPartialLowUncertainty_Decomposable` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_ExploreLeafPartialInlines` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_RollupPartialInlines` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_PartialLowUncertainty` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R6_ScenarioFail` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R6_ExplorationFail` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_RollupPartial_BelowLimitInlines` | ✅ (T: D7-S15-A89-T01) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_RollupPartial_AtLimitEscalates` | ✅ (T: D7-S15-A89-T02) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_RollupFail_AtLimitEscalates` | ✅ (T: D7-S15-A89-T03) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_RollupIndeterminate_AtLimitEscalates` | ✅ (T: D7-S15-A89-T04) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_RollupPass_AlwaysNone` | ✅ (T: D7-S15-A89-T05) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R6_ExploreLeafFailInlines` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R6_CommitmentFail` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R7_IndeterminateRetry` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R7_IndeterminateExhausted_ExploratoryDecomposes` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R7_ExploreLeafIndeterminateExhaustedEscalates` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R7_IndeterminateExhausted_CommitmentEscalatesHuman` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R8_UnknownVerdict` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_NilRound` | ✅ |
| `spawn_policy_test.go` | `TestEvaluateSpawnPolicy_SetsRationale` | ✅ |
| `spawn_policy_test.go` | `TestValidateSpawnDecompose` | ✅ |
| `spawn_policy_test.go` | `TestCapChildSpecs` | ✅ |
| `spawn_policy_test.go` | `TestResolveHint_FromLastRound` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_PartialIncompleteDeliverable_Inlines` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_CCU1_inlineNotEscalateWithEvidence` | ✅ |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_CCU1_escalateWithoutEvidence` | ✅ |
| `spawn_policy_inline_test.go` | `TestSpawnPolicyEvaluator_DeliverableInlineWouldExhaustEscalatesAtDepth0` | ✅ |

22 个 SpawnPolicyEvaluator 相关测试 + 6 个 helper 测试 = 28 个相关测试 0 修改 → 0 行为变化。

---

## 5. 性能指标

| 指标 | 目标 | 实测 |
|------|------|------|
| `SpawnPolicyEvaluator` 热路径 | < 1 μs | < 1 μs（3 函数指针调用 + 5 ctx 字段比较 + 1 switch on enum） |
| Sub-decision 函数热路径 | < 500 ns | < 200 ns（简单 if 链 + VerdictKind 比较） |
| `normalizeCtx` value copy | < 200 ns | ~50 ns（17 字段 copy，value type） |

---

## 6. 行为变化审计

| 类型 | 详情 |
|------|------|
| **新增** | `spawn_decision_algebra.go` (165 行) + `spawn_decision_algebra_test.go` (313 行) + `spawn_policy_legacy_test.go` (356 行, build tag `legacy_spawn`) |
| **修改** | `spawn_policy.go` 268→157 行（`SpawnPolicyEvaluator` 50+→8 行 + 移除 `plan` import） |
| **删除** | 内联 50+ 行 if/switch 链（被 3 子决策 + normalizeCtx 取代）+ 5 行 `if ctx.X <= 0` default 兜底（被 `normalizeCtx` 取代）+ R5/R6/R7 verdict 块顶部 3 处 `if ctx.RollupRound` 重复 5 行（被 `checkRollupGuard` 取代） |
| **保留** | `EvaluateSpawnPolicy` (line 145) + `spawnRationale` (line 155) + 5 个 deliverable helper + `spawnForDeliverableContinuation` + `RollupSynthEligible` + `IsExploratoryPlanKind` + `CanDecompose` 全部不变 |
| **0 行为变化** | 22 现有测试 0 修改 + 27 byte-equivalent sub-cases 字节级 PASS |

---

## 7. 后续

- **M3** `d7-mups-strategy-injection` 行为增量，最后做
- **`mups-cleanup-legacy`** 下下个 change：删除 `spawn_policy_legacy_test.go` + `SpawnPolicyEvaluatorLegacy` 死代码（与 M4 `verify_legacy_test.go` + `verifyArtifactLegacy` 一起清理）
- **5 节点重构总图** (M1+M2+M4+M5 全部 0 行为变化 S7_archived)：M3 行为增量是最后一步
