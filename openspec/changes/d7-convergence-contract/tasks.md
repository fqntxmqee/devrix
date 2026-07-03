# Implementation Tasks: D7 收敛契约

**Change ID:** `d7-convergence-contract`

---

## Phase 1: Round Terminalization（收敛闭环）

- [ ] 1.1 `SpawnPolicyEvaluator`：R0.5 `!deliverableContinuationRequired → SpawnNone`（在 R1 之前）
- [ ] 1.2 `WorkItemPipelineRound` + `TreeEvalContext`：增加 `InlineRetriesAtMaxDepth` / `MaxInlineRetriesAtMaxDepth`（默认 3）
- [ ] 1.3 新 `terminalize.go`：`ApplyRoundTerminalization` 统一 Status 更新（替代仅 SpawnNone 分支）
- [ ] 1.4 `GetPipelineFocus`：跳过 `SpawnInline` 且 `!DeliverableContinuationRequired` 的 WI（双保险）
- [ ] 1.5 测试 T1、T2、T3（见 design.md §6）

**Quality Gate:**
- [ ] `go test ./internal/layers/orchestration/workmodel/... ./internal/layers/orchestration/sessionorchestrator/...`

---

## Phase 2: Downward Scope Validation

- [ ] 2.1 新 `scope_validator.go`：repo 存在性 + parent scope 单调收窄 + blocklist
- [ ] 2.2 挂接 `PrepareDecomposeSpecs` / strategic plan 落地前
- [ ] 2.3 扩展 `shouldDecomposeForDeliverable` 至 general registered schema（非仅 p0_p1）
- [ ] 2.4 测试 T5

**Quality Gate:**
- [ ] decompose 提案全 reject 时 fallback DefaultDecomposeProposer

---

## Phase 3: Upward Feedback Enhancement

- [ ] 3.1 `MaybeSiblingBestEffortRollup`：1 complete + 1 stuck → fail stuck + parent NeedsRollup
- [ ] 3.2 `MaybeDecomposeParentRollup` → `MaybeParentRollup`（所有 decompose 父节点）
- [ ] 3.3 可选 `MergeChildDeliverables`（rollup verify 前结构化合并）
- [ ] 3.4 `RollupGatePolicyFor` 读 Session/WorkItem 配置（默认 best_effort）
- [ ] 3.5 测试 T4、T6、T7

**Quality Gate:**
- [ ] rollup 后 root `ExtractSessionDeliverable` 非空

---

## Phase 4: Session Exit & Docs

- [ ] 4.1 统一 `EvaluateSessionExit`；重构 `sessionNoForwardProgress` 为 subtree stuck
- [ ] 4.2 可选 Session `MaxMUPSRounds` 软上限
- [ ] 4.3 更新 `openspec/specs/d7-orchestration/pipeline-architecture.md` 引用本 change
- [ ] 4.4 `/openspec-archive` 前 acceptance-report

**Quality Gate:**
- [ ] 真实飞书指令 `review d2 领域 kernel目录下代码` 回归

---

## Completion Checklist

- [ ] All phases complete
- [ ] T1–T7 集成测试绿
- [ ] design.md 决策树与代码一致
- [ ] Ready for `/openspec-apply d7-convergence-contract`
