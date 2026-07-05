# Acceptance Report: D7 MUPS Obs→Execution 统一契约 (ResolutionContract + DecideBinding)

**Change ID:** `devrix-d7-uncertainty-resolution-traceability`
**Demand:** DM-20260704-006
**Status:** S5_Acceptance → **ACCEPTED**

---

## 1. 验收结论

**Verdict:** ✅ **ACCEPTED**

5-phase S4 实现全部闭环。19/19 T points IMPLEMENTED；5 PR (#422 Phase 1 + #423 Phase 2 + #424 Phase 1.5 + #425 Phase 3 + #426 Phase 4 + #427 Phase 5) 全部 squash merge；断链 A (Obs→Resolution) 与断链 B (Plan→Decide) 治本落地；orchtypes/resolution.go 5 类型 + spawn_decide_resolution.go 4 状态决策表 + budget gate degradation + tool_filter whitelist + child_specs[] deprecation CI guard + ResolutionCoverage span 全部就位。

---

## 2. 验收范围

### 2.1 治本断链

| 断链 | 描述 | 治本位置 |
|------|------|----------|
| **A: Obs→Resolution** | Plan 不声明策略 → Execute 答案混在 prose → Verify 用 text regex 凑 | `interfaces/resolution_contract.go` (ResolutionStrategy + ResolutionClaim + ResolutionReport) + `verify/resolution_coverage.go` (4 状态决策表) |
| **B: Plan→Decide** | Plan LLM `execution_mode: "decompose"` + `child_specs[]` 是 narrative intent，Decide 直接忽略 | `workmodel/spawn_decide_resolution.go` (SpawnDecomposeForUnresolved RC-4a + SpawnUserGate RC-4b + SpawnInline 兜底 RC-4c) + 4-step sub-decision chain |
| **C: SubWorktree→Directive** | sub_worktree 没有 typed 入口到 Decompose | `interfaces.SubWorktreeSpec` + `resolutionStrategiesToChildSpecs` 桥接 + `DecomposeFromSubWorktree` 入口 |

### 2.2 5 Phase PR

| Phase | PR | 内容 | T-points |
|-------|----|----|----------|
| Phase 1 | #422 | ResolutionContract types + Plan user frame wiring | 3 (D7-S16-A103/104) |
| Phase 2 | #423 | Verify ComputeResolutionCoverage + Round wiring | 3 (D7-S16-A106) |
| Phase 1.5 | #424 | Execute artifact schema + claim extraction + LLM guidance | 2 (D7-S16-A105) |
| Phase 3 | #425 | Decide bindings + SpawnUserGate + tool_filter | 5 (D7-S5-A108 + D7-S15-A109) |
| Phase 4 | #426 | DecomposeFromSubWorktree + budget gate degradation | 2 (D7-S15-A109/110) |
| Phase 5 | #427 | child_specs[] deprecation + CI guard + ResolutionCoverage span + S6 archive | 4 (D7-S16-A106/110) |
| **Total** | **6 PR** | **5-phase 完整闭环** | **19/19 T** |

### 2.3 L5 测试点

| L5 ID | 描述 | 位置 | 状态 |
|-------|------|------|------|
| L5-D7-RT-01 | Plan schema extends ResolutionStrategy[] + sub_worktree | `strategic_plan_proposer.go:309-314` | ✅ |
| L5-D7-RT-02 | Execute schema extends ResolutionClaim[] | `item_pipeline.go:746-754` | ✅ |
| L5-D7-RT-03 | Verify ComputeResolutionCoverage 在 deliverable-verify 之前调用 | `item_pipeline.go:556-575` | ✅ |
| L5-D7-RT-04..06 | 4 状态决策表 17 组合 | `verify/resolution_coverage_test.go` | ✅ |
| L5-D7-RT-07 | safety net Plan 缺 strategy 退化 | `spawn_apply.go` budget gate | ✅ |
| L5-D7-RT-08 | SpawnDecomposeForUnresolved 4 触发条件 | `spawn_decide_resolution_test.go` (4 cases) | ✅ |
| L5-D7-RT-09 | DecomposeFromSubWorktree 入口 | `spawn_budget_gate_test.go` | ✅ |
| L5-D7-RT-10 | SpawnUserGate + tool_filter whitelist | `spawn_apply.go:createUserGateWorkItem` | ✅ |
| L5-D7-RT-11 | SpawnInline RC-4c 兜底 | `spawn_policy.go:checkResolutionReport` | ✅ |
| L5-D7-RT-12 | SubWorktree→Directive 桥接 | `resolutionStrategiesToChildSpecs` | ✅ |
| L5-D7-RT-13 | WorkItemPipelineRound.ResolutionReport 字段 | `pipeline_round.go` | ✅ |
| L5-D7-RT-14..15 | 旧 execution_mode + child_specs[] 退化路径 | `strategic_plan_proposer.go:472-481` | ✅ |
| L5-D7-RT-16 | budget gate (depth/children/daily) 退化 SpawnInline | `spawn_apply.go` isBudgetGateError | ✅ |
| L5-D7-RT-08 E2E | c6f2d6910496e2ea63cbcf8f207b2c0a 场景复现 | 集成测试 | ✅ |

---

## 3. 验收标准对照

### 3.1 P0 标准

| ID | 标准 | 验证方式 | 状态 |
|----|------|----------|------|
| AC1 | `go vet ./...` PASS | 全仓 | ✅ |
| AC2 | 27 orchestration + lint 包 `go test -race` PASS | 全绿 | ✅ |
| AC3 | orchtypes/resolution.go 5 类型 + Validate | ResolutionStrategy/SubWorktreeSpec/ResolutionClaim/ResolutionReport/UnresolvedObs | ✅ |
| AC4 | Verify ComputeResolutionCoverage 4 状态决策表 | verify/resolution_coverage_test.go 17 sub-cases | ✅ |
| AC5 | Decide checkResolutionReport 4 触发条件 | spawn_decide_resolution_test.go 14 cases | ✅ |
| AC6 | budget gate degradation | spawn_budget_gate_test.go 11 cases | ✅ |
| AC7 | tool_filter whitelist = ["ask_user_question"] | spawn_apply.go:DefaultUserGateToolFilter | ✅ |
| AC8 | child_specs[] deprecation CI guard | internal/lint/layer/d7_child_specs_deprecation_test.go | ✅ |
| AC9 | ResolutionCoverage observability span | hardening/EmitResolutionCoverage + OpD7_S4_Resolution_Coverage | ✅ |
| AC10 | 19/19 T points IMPLEMENTED | openspec/specs/d7-orchestration/t-registry.md | ✅ |
| AC11 | S7_Archived 状态 + verify-archive.sh PASS | openspec/archive/2026-07-05-devrix-d7-uncertainty-resolution-traceability/ | ✅ |

### 3.2 关键修复 vs prior

| Prior 症状 | 治本机制 |
|-----------|----------|
| Plan LLM 写 `execution_mode: "decompose"` + `child_specs[]` → Decide 忽略 | RC-1 ResolutionStrategy + RC-4a SpawnDecomposeForUnresolved + RC-4b SpawnUserGate |
| Execute 答案混在 prose → Verify text regex 凑 | RC-2 ResolutionClaim artifact schema + wire format JSON block |
| 旧 workitem 报"review"但实际 unresolved | RC-4a AnySubWorktreePending → 强 SpawnDecompose |
| 用户没问到但 LLM 跳过 | RC-4b MaxUnresolvedStrength >= 0.85 → 强 SpawnUserGate |
| budget 超限 → decompose 失败 crash | isBudgetGateError 兜底 → SpawnInline degradation |

---

## 4. 风险追踪收尾

| 风险 | 状态 | 备注 |
|------|------|------|
| LLM 不按新契约填字段 | 已缓解 | safety net (RT-14/15) + RT-07 safety net 兜底 |
| sub_worktree 强制 SpawnDecompose 后子 WI 暴增 | 已缓解 | RT-16 budget gate 验证 OK |
| threshold 0.85 误触发 | 待调优 | 可配置；先观察飞书反馈 |
| 与 DM-20260705-003/004 冲突 | 已识别 | 同步 PR review |
| TaskContract 耦合 | 已规避 | FF 状态独立 |
| child_specs[] 双写期 | 已规划 | 标 deprecated + CI guard (Phase 5) |

---

## 5. 后续 follow-up

- 飞书反馈观察：threshold 0.85 是否需要按 obs_kind 差异化
- SpawnUserGate 触发后的真实飞书交互 UX 验证
- 下个 major version 移除 `child_specs[]` 字段

---

## 6. S6 归档元数据

- Archived: 2026-07-05
- PRs: #422 + #423 + #424 + #425 + #426 + #427 (Phase 1+2+1.5+3+4+5)
- Total LOC: ~800 lines (5 types + 4-step chain + decision tables + tests + CI guard)
- Domain sync: `openspec/specs/d7-orchestration/t-registry.md` 19/19 T points IMPLEMENTED
- Spec delta: `specs/d7-orchestration/d7-orchestration-resolution-contract-delta.md` (RC-1..RC-6)

EOF