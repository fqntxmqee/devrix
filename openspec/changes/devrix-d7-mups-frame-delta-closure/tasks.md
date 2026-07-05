# Tasks: D7 MUPS 5 节点 frame delta 闭环

**Change ID:** `devrix-d7-mups-frame-delta-closure`
**Demand:** DM-20260705-010
**Status:** S2_Proposal (planned)

## P0

| Task | 描述 | 关联 AC | Status |
|------|------|---------|--------|
| **Phase 1 — Plan → Execute frame delta 注入** | | | |
| T1 `interfaces/mups_frame_delta.go`: `FrameDelta` struct 定义 (`PriorArtifactSummary string` + `KnownGaps []string` + `ExecutionMode string` + `ChildSpecs []ChildSpecRef` + `DeliverableContract string`) | 根底类型 | [ ] |
| T2 `strategic_plan_proposer.go`: `StrategicPlanFrame` 增加 5 字段（同时驱动 M1 frame 9 字段契约 0 修改，append-only） | AC3 | [ ] |
| T3 `sessionorchestrator/execute_plan_frame_inject.go` (new): `InjectPlanFrameDelta(ctx, plan *PlanOutput, baselineSystemPrompt string) string` — 摘要 ≤ 80 字符 + schema hash 双轨 | AC3 | [ ] |
| T4 `item_pipeline.go`: Plan→Execute 边注入点（`buildExecuteSystemPrompt` 调用 InjectPlanFrameDelta） | AC3 | [ ] |
| T5 `execute_plan_frame_inject_test.go`: 5 子测试（注入正确 / 摘要 ≤ 80 / schema hash 稳定 / 0 注入 baseline / 注入破坏 prompt 不破坏） | D7-S9-A112-T01..T05 | [ ] |
| T6 L5-MUPS-FD-1: trace 重放 Execute system_prompt 增量 ≤ 200 字符 + 含 plan_frame_delta schema hash | AC3 + AC5 | [ ] |
| **Phase 2 — Observe → Plan 闭环节流** | | | |
| T7 `sessionorchestrator/observe_frame_delta.go` (new): `BuildObservePriorDelta(prevExecCtx *WorkItemExecContext) FrameDelta` — 首轮返回零值 | AC1 + AC2 | [ ] |
| T8 `observation_proposer.go`: `ObservationFrame` 9 字段之外 append `prior_artifact_summary` + `known_gaps` 两个 `obs_fact` kind 字段（兼容 DM-20260705-009 封闭式分类器定位） | AC1 + AC2 | [ ] |
| T9 `llm_observation_proposer.go`: `FrameObserveUser` spec 扩展 + i18n `obs.input.prior_artifact_summary` / `obs.input.known_gaps` 翻译（en + zh） | AC1 + AC2 | [ ] |
| T10 `observe_frame_delta_test.go`: 6 子测试（首轮零值 / 非首轮含上一轮收敛度量 / Plan scope_in 映射 known_gaps / 封闭式 JSON 不破坏 / 9 字段契约 0 修改 / i18n 键完整） | D7-S5-A111-T01..T06 | [ ] |
| T11 L5-MUPS-FD-2: trace 重放 Observe→Plan 链路 LLM user prompt 含 `prior_artifact_summary` + `known_gaps` span tag | AC1 + AC2 + AC5 | [ ] |
| **Phase 3 — convergence_metric deterministic 回写** | | | |
| T12 `sessionorchestrator/convergence_metric.go` (new): `ConvergenceMetric` struct + `ComputeConvergenceMetric(subTurns []SubTurnRecord) ConvergenceMetric` 纯 deterministic（工具结果 diff + claim 数 + obs_uncertainty 残量） | AC4 | [x] |
| T13 `item_pipeline.go`: 每个 sub-turn 结束 emit `convergence_metric` span（含 `uncertainty_reduction_rate` + `observed_gaps_closed_count` + `frame_delta_consumed`） | AC4 | [x] |
| T14 `convergence_metric_test.go`: 5 子测试（首轮 0 / 工具 diff 计算 / claim 累加 / Jaeger span 完整 / 0 LLM 调用验证） | D7-S9-A113-T01..T05 | [x] |
| T15 L5-MUPS-FD-3: trace 重放 Execute 5 个 sub-turn 全有 convergence_metric span + 末轮 uncertainty_reduction_rate ≥ 0.5 | AC4 + AC7 | [ ] |
| **Phase 4 — 端到端收敛验证** | | | |
| T16 `e2e_frame_delta_test.go`: 端到端 trace 重放 — sess_1783255992426_6000 wi_d0_s0_goal 重跑 → Observe→Plan→Execute LLM frame delta span tag 全可见 + AC5 通过 | AC5 | [ ] |
| T17 L5-MUPS-FD-4: 跨链 LLM 帧 delta 单调不增 — Observe→Plan→Execute prompt size 在 trace 上满足 ±5% 噪声内不增 | AC7 | [ ] |
| T18 L5-MUPS-FD-5: 70+ 现有 LLM frame 测试 0 行为变化 PASS（M1-M5 契约 0 修改回归） | AC6 | [ ] |
| T19 S3-Gate 三方博弈论 review：codex + cursor 三方共识评论通过 | AC8 | [ ] |
| T20 d7 spec.md 5 节点管道 I/O 协议段新增 frame delta 描述 + CHANGELOG.md 顶部条目 | spec sync | [ ] |
| T21 t-registry.md D7-S5-A111 / D7-S9-A112 / D7-S9-A113 登记 PLANNED | t-registry | [ ] |

## Verification

```bash
# 单包验证（Phase 1）
go vet ./internal/layers/orchestration/sessionorchestrator/... ./internal/layers/orchestration/interfaces/...
go test ./internal/layers/orchestration/sessionorchestrator/... -race -count=1 -run 'TestExecute.*FrameDelta|TestPlanFrameInject'
go test ./internal/layers/orchestration/sessionorchestrator/... -race -count=1 -run 'TestPlanFrameInject'

# 单包验证（Phase 2）
go test ./internal/layers/orchestration/sessionorchestrator/... -race -count=1 -run 'TestObserveFrameDelta'
go test ./internal/layers/contextengine/i18n/... -race -count=1

# 单包验证（Phase 3）
go test ./internal/layers/orchestration/sessionorchestrator/... -race -count=1 -run 'TestConvergenceMetric'

# 全仓回归
go test ./... -race -count=1

# kernel 零代码增量验证（M1-M5 frame 契约 0 修改）
git diff master..HEAD -- internal/layers/orchestration/sessionorchestrator/observation_proposer.go internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go
# 期望: append-only 增量，9 字段契约 0 修改

# E2E trace 验证（Phase 4）
bash scripts/e2e_trace_replay.sh sess_1783255992426_6000 wi_d0_s0_goal \
  | grep -E '(prior_artifact_summary|known_gaps|convergence_metric|plan_frame_delta)'
# 期望: span tag 全可见

# spec sync 验证
diff <(git show master:openspec/specs/d7-orchestration/spec.md | grep -A 5 'I/O 协议') \
     <(cat openspec/specs/d7-orchestration/spec.md | grep -A 5 'I/O 协议')
# 期望: 新增 frame delta 描述段
```

## Rollback Plan

- git revert <commit> 一行回滚（pure append-only，无 schema migration）
- 旧 StrategicPlanFrame 9 字段契约保留（frame delta 在原 frame 之外增量注入，不进 frame）
- 旧 ObservationFrame 9 字段契约保留（同理）
- FrameDelta 注入点是可选中间件，未注入时走 baseline

## Out-of-scope (不实现)

- Verify / Learn / Decide 节点改造（已是 deterministic，0 LLM）
- 修改 M1-M5 已落地的 LLM frame 契约（append-only）
- 修改 DM-20260704-006 ResolutionContract 数据契约
- 修改 DM-20260705-008 Strategy 决策表
- 修改三层 fail-safe / Pessimistic Commit L3 防御
- 修改 workmodel.DivergenceBudget 字段
- PlanKind / VerdictKind 决策表改造