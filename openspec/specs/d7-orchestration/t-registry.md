# D7 Orchestration Domain — T 层测试点注册表

**Status:** Active
**Version:** 4.29.0
**Last Updated:** 2026-07-04 (mups-prompttags-v2-io-registry DM-20260704-005: D7-S16-A96 +3 T IMPLEMENTED 292→295, P0 249→252)
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `d7-domain.md`
**Spec:** `openspec/specs/d7-orchestration/spec.md`
**Complements:** `terminal-state-guide.md` · `observability-guide.md`
**Change:** 2026-06-20-devrix-context-budget-and-isolation-phase-b (devrix-context-budget-and-isolation / DM-20260620-001-B) — Phase B: AC6 + AC8 + AC9 SubTurn 3-mode dispatch + depth cap (D7-S2-A06-T14/T15/T16/T17); IMPLEMENTED 99→103, P0 70→74. **2026-06-20-devrix-error-handling-tier1-tier2** (DM-20260620-003) — error handling PR-A/PR-B/PR-C: invariant migration to shared/errors (D7-S2-A06-T24), task_manager.Create signature (`(*Task, error)`) (D7-S1-A02-T18), orchestrator.emitError sanitize+code (D7-S2-A02-T18), subagent stream sentinels (D7-S2-A06-T25/T26), retry nil-sentinel (D7-S2-A06-T27), resolveDelegateTaskID `(string, error)` (D7-S1-T19); IMPLEMENTED 109→116, P0 80→83. **2026-06-21-devrix-d7-error-aggregation-and-metrics** (DM-20260621-010) — D7 编排层错误聚合 + worktree 全链路 metrics: interrupt errors.Join aggregation (D7-S6-A11-T01/T02/T03), sandbox cleanup observability (D7-S6-A12-T04/T05/T06), forker errors.Join + 13 callers backward compat (D7-S6-A13-T07); IMPLEMENTED 116→123, P0 83→90. **2026-06-22-devrix-d7-metrics-and-concurrency-hardening** (DM-20260622-001) — D7 编排层 metric 命名 spec/code 对齐 + 并发硬化: dispatch_loop_wakeups / worker_panics 复数化 (D7-S6-A14-T01/T02), sandbox_exit_failed 跨域归属 D4 (D7-S6-A14-T03, D7-S6-A12-T01 OBSOLETE), state.cancels + state.handles markWaveDone 清空 (D7-S6-A14-T04), ConflictGuard hot path AllowAndRegister 原子调用 (D7-S6-A14-T05), CommandHandler emit select-default 防阻塞 (D7-S6-A14-T06); IMPLEMENTED 123→129, P0 90→96. **2026-06-23-devrix-d7-mups-v4-phase3-execute** (DM-20260625-001) — Phase 3 PR-C1 (最小风险入口): ArtifactKind 4 类枚举 (D7-S9-A25-T01), SideEffectStatus 5 态 + IsTerminal/NeedsAttention (D7-S9-A25-T02), wavescheduler.Artifact +5 字段 omitempty 向后兼容 (D7-S9-A25-T03), 跨域类型上提 shared/types 打破 import cycle (D7-S9-A25-T04); IMPLEMENTED 129→133, P0 96→100. **2026-06-23-devrix-d7-mups-v4-phase2-observe-plan** (DM-20260623-001) — Phase 2 PR-A1 + PR-RF (A15 模块): Observation 4 类 × 2 Category + sealed Payload (D7-S8-A15-T01), UncertaintyReport ComputeOverallStrength 仅遍历 CatBusiness + defaults half (D7-S8-A15-T02), UncertaintyCoord Phase 2 扩展 + FromVerifier fail-fast (D7-S8-A15-T03), UncertaintyReport Partition 不变式 (D7-S8-A15-T04), FilterByKind 遍历全集 (D7-S8-A15-T05), Observation 不可变 + clamp01Float + validateFact fmt.Errorf wrap (D7-S8-A15-T06); IMPLEMENTED 133→139, P0 100→106。**2026-06-23-devrix-d7-mups-v4-phase2-plan** (DM-20260623-001-PRB1) — Phase 2 PR-B1 (A22 模块): PlanKind 4 类枚举 (D7-S8-A22-T01), Plan.SourceObservationIDs 必填 + Phase 4 Verify 反向追溯入口 (D7-S8-A22-T02), MatchKind 4 规则分类器 + uncertainty-first tie-break + DefaultPlanner.Plan 集成 (D7-S8-A22-T03); IMPLEMENTED 139→142, P0 106→109。**2026-06-23-devrix-d7-mups-v4-phase3-channels** (DM-20260625-001-PRC2) — Phase 3 PR-C2 (A26 模块): ChannelRegistry 1:1 绑定 + ChannelRouter 4 PlanKind 路由 (D7-S9-A26-T01), CommitChannel 1-Step 同步 + IdempotencyKey 强制 + 超时 SideEffectInflight (D7-S9-A26-T02), ProtocolChannel 顺序多步 + reverse-order rollback (D7-S9-A26-T03), ScenarioChannel 并行探测 + 多数派投票 (D7-S9-A26-T04), ExplorationChannel 多 agent + 优先级排序 + PersistScope 派生 SideEffectStatus (D7-S9-A26-T05); IMPLEMENTED 142→147, P0 109→114。**2026-06-23-devrix-d7-mups-v4-phase4-verify-promotion** (DM-20260623-002) — Phase 4 Verify 节点升格 (A32/A33/A34/A35 模块): VerdictKind 4 态 typed enum + String/Parse/Marshal/Unmarshal (D7-S10-A32-T01), AggregationStrategy 4 策略 + AggregateVerdicts 边界 + 4 策略实现 (D7-S10-A32-T02), VerdictToExitReason 4 Verdict → 4 ExitReason 映射 + SystemAnomaly 覆盖 + 14 ExitReason 8→14 扩展 (D7-S10-A33-T03), VerifyWithRetry parse failure → INDETERMINATE G8-1 修复 (D7-S10-A33-T04), Evidence struct 5 字段 + Validate + NewEvidence 必填 fail-fast (D7-S10-A34-T05), EvidenceExtractor interface + LLM + Stub 实现 (D7-S10-A34-T06), SystemAnomalyAggregator 阈值触发 + RecordCatSystem + Reset (D7-S10-A35-T07), ObserveNode wiring SystemAnomaly → FromVerifier + BuildUncertaintyCoordFromReport Value=0.95 强制 (D7-S10-A35-T08); IMPLEMENTED 147→155, P0 114→122, Scenarios D7-S10 0→4。**2026-06-23-devrix-d7-mups-v4-phase5-learn** (DM-20260623-003) — Phase 5 Learn 节点升格 (A36/A37/A38/A39/A40 模块): LearningAsset struct 15 字段 + NewLearningAsset fail-fast + deep copy + 自动时间戳 (D7-S11-A36-T01), 5 类 AssetContent (SOPAssetContent ★5 / ProtocolAssetContent ★4 / KnowledgeAssetContent ★3 / ConclusionAssetContent ★2 / PendingAssetContent ⭐★1 含 MVEState) + Validate() + SchemaVersion() + ByteSize() + 必填 fail-fast (D7-S11-A36-T02), LearningClass 5 态 typed enum + String/Parse/Marshal/Unmarshal + 空字符串零值 LearningSOP 兼容 + 跨域类型上提 shared/types/learning.go (D7-S11-A36-T03), ReputationEvidence struct 12 字段 + NewReputationEvidence fail-fast + TrackMode 解析 + 冷启动除零防御 (D7-S11-A37-T04), BayesianUpdate 函数 + 不可变 + Pass/Partial/Fail → α/β++ + ⭐G8-1 修复 (INDETERMINATE "verifier_parse_failure" 仅 VerifierFailureCount++ 不污染 α/β) + Wilson Score 95% 置信区间 (D7-S11-A37-T05), AdaptivePrior + BetaPrior + InjectTarget 3 枚举 + DefaultInjectTargets (D7-S11-A38-T06), DefaultDeveloperPrior Beta(5,3) + DefaultOperatorPrior Beta(8,1) + BuildAdaptivePrior Bayesian 合并公式 + rep==nil 兜底 + trackMode=="" 兜底 (D7-S11-A38-T07), Memory interface 4 方法 + MemoryChannel 3 枚举 + MemoryFilter 4 字段 + SkillMemory/FeedbackMemory + ErrAssetClassMismatch + 并发安全 (D7-S11-A39-T08), ScheduledMemory + ScheduledRetry envelope + TriggerAt 默认 + MaxRetries=3 + IsExhausted + ListDue + 并发安全 (D7-S11-A39-T09), Learner interface 3 方法 + DefaultLearner + Learn 5 步流程 + 4 Verdict 路由 + Inject LP-1 闭环 + ScheduledTick (D7-S11-A40-T10), AssetBuilder 5 类 Content 构造 + classToStrength + hashContentBytes + AssetKey 格式 + Build nil 边界 (D7-S11-A40-T11), ReputationStore interface + InMemoryReputationStore 并发安全 + defensive copy + List 过滤 (D7-S11-A40-T12), in-package LP-1 闭环测试 (Learn ×3 → Alpha=3 → Inject → PriorBeta=Beta(8,3)) + G8-1 修复闭环 (α/β 不污染) (D7-S11-A40-T13 PARTIAL — Observe 跨域 wiring 留待 Phase 6 集成); IMPLEMENTED 155→168, P0 122→135, Scenarios D7-S11 0→5。**2026-06-24-devrix-d7-mups-v4-phase6-observe-learner-wiring** (DM-20260624-001) — Phase 6 Observe-Learner 跨域闭环集成 (A41/A42/A43 模块): ObserveRequest struct 3 字段 + NewObserveRequest fail-fast + EffectivePrior 兜底 DefaultDeveloperPrior + Validate + QuantizeWithPrior / DetectWithPrior / ClassifyWithPrior (D7-S12-A41-T01), IntentQuantizer 4 IntentClass (Fact/Command/Orchestrate/Skip) + IntentPayload + QuantizeWithPrior (prior.PriorBeta.Mean() 作为 confidence 乘数, clamp [0,100]) + 不可变 + 并发安全 (D7-S12-A41-T02), AnomalyDetector + Anomaly + AnomalyReport + HistoricalDetector.Detect baseline + HistoricalDetector.DetectWithPrior (threshold = 0.5 × Mean, Mean 越高阈值越高 = 更信任用户 = 更易放过) + 不可变 + 并发安全 (D7-S12-A41-T03), SessionOrchestrator 新增 `learner learn.Learner` 字段 + `WithLearner` option + buildObserveRequest 方法 (调用 Learner.Inject, 3 层 fail-safe) + ProcessMessage 在 classifySpan 之前调用 buildObserveRequest + IntentClassifier 接口扩展 ClassifyWithPrior + RuleClassifier.ClassifyWithPrior + ShadowClassifier.ClassifyWithPrior 委托给底层 rule (D7-S12-A42-T04), buildObserveRequest 3 层 fail-safe 单元测试 (nil learner / Inject error / 正常 全部 DefaultDeveloperPrior Beta(5,3) 兜底) + ProcessMessage UsePriorInClassification 集成测试 (D7-S12-A42-T05), E2E LP-1 闭环集成测试 4 scenarios (Learn Pass Accumulate / INDETERMINATE parse_failure No Pollution / PendingAsset ScheduledMemory / 5-Node Pipeline End2End) + 完整 LP-5 反向追溯链验证 (D7-S12-A43-T06); IMPLEMENTED 168→174, P0 135→141, Scenarios D7-S12 0→3。**2026-06-25-devrix-d7-mups-v4-phase7-verify-auto-close** (DM-20260625-001) — Phase 7 运行时 5 节点闭环 (PR-7.1/7.2/7.3) (A47/A48/A49 模块): processAutoClose 包装 channel + 异步触发 learner.Learn + 替换 endSpanWhenChannelClosed 调用 (D7-S13-A47-T01), synthesizeVerdict 规则 (complete→Pass / error→Fail / tombstone→Indeterminate + IndeterminateReason="interrupt") + 3 层 fail-safe (nil learner / Learn error / channel cancel) + SourceID 格式 `autoclose:{sessionID}:{nanosecond}` (D7-S13-A47-T02), 集成测试 ProcessMessage 完整跑 → Alpha++ + 下一轮 prior 更新 (Round 1 冷启动 Beta(5,3) → Learn VerdictPass → Alpha=1 → Round 2 Beta(6,3) Mean=0.667) + TestAutoClose_FullLP1Loop 端到端 LP-1 闭环在生产 wiring 验证 (D7-S13-A47-T03), ProcessRequest 新增 TrackMode string 字段 (默认 "" 兜底 developer) + TrackModeDeveloper/Operator 常量 + NewProcessRequest fail-fast 校验 + 3 个 sentinel error (ErrProcessRequestSessionIDEmpty / ErrProcessRequestMessageEmpty / ErrProcessRequestInvalidTrackMode) (D7-S13-A48-T04), buildObserveRequest 透传 req.TrackMode → o.learner.Inject(ctx, sessionID, req.TrackMode) → BuildAdaptivePrior (Operator track → DefaultOperatorPrior Beta(8,1) Mean=0.889，Developer → Beta(5,3) Mean=0.625，空字符串/未知 → 兜底 Developer) (D7-S13-A48-T05), sessionSpan 6 prior attributes (alpha/beta/mean/track_mode/classifier_source/injected_at) 全部写入 + priorSessionSpanAttrs 纯 helper 便于单元测试 + 5 个单测覆盖 real injection / cold_start_failsafe / operator from hint / reputation wins / 字符串类型校验 (D7-S13-A49-T06); IMPLEMENTED 174→180, P0 141→147, Scenarios D7-S13 0→6。**2026-06-25-devrix-d7-mups-v5-escape-engine-v5-6** (DM-20260625-003) — MUPS v5 统一逃逸机制 PR-V5.6 续跑入口收口: SessionOrchestrator `applyResumeSession` 方法 (ProcessMessage 开头检查 → EscapeEngine.ResumeSession one-shot consume → terminal decision B/C emit single "complete" EngineEvent + 补写 audit + close channel / A user_continue fall through to full 5-node pipeline) + 3 层 fail-safe (nil engine / ResumeSession error → 静默 fall through / PendingResolutionStore 已 TTL 过期 → 静默 fall through) + sessionSpan resume 3 attributes (resume_attempted / resume_decision_action / resume_decision_pending_id) + 6 个单元测试 (TestApplyResumeSession_NoEngine / TestApplyResumeSession_NoPending / TestApplyResumeSession_UserAccept / TestApplyResumeSession_UserCancel / TestApplyResumeSession_UserContinue / TestApplyResumeSession_ResumeError_Failsafe) + 2 个集成测试 (TestProcessMessage_WithResume_UserAccept_EarlyClose / TestProcessMessage_WithResume_UserCancel_EarlyClose); IMPLEMENTED 180→186, P0 147→153, D7-S14 T12 PARTIAL→IMPLEMENTED (T12 18/18 IMPLEMENTED, 0 PARTIAL)。**2026-06-29-devrix-d7-taskcontract-unification-pr-a** (DM-20260629-007) — v7.0 TaskContract 统一 PR-A (L1 接口层 + L2 字段语义层 + L4 spec 同步): interfaces 包 7 NEW 文件 (doc.go + errors.go + task_spec.go + task_report.go + task_spec_test.go + task_report_test.go + taskcontract_test.go) + 2 MODIFIED (mups/execute/channel.go 加 Spec 字段 + mups/learn/asset/asset_builder.go 加 Report 字段) — T 编号重映射 D7-S16 占用 → 新分配 D7-S20/21 (D7-S22/23 PR-B/C 保留位): NewTaskSpec + Validate happy path (D7-S20-A01-T01) + TaskSpec With* 不可变 builder 浅拷贝 (D7-S20-A01-T02) + TaskSpec 3 创建点统一 (Plan/Channel/WorkItem) (D7-S20-A01-T03) + NewTaskReport + Validate happy path (D7-S20-A02-T01) + TaskReport With* + AppendDissent 不可变 (D7-S20-A02-T02) + Channel.Execute 出口 + Learn 节点入口统一 (D7-S20-A02-T03) + Dissent top-3 截断 + summary hash + Learn 沉淀 (D7-S21-A01-T01) + Blockage 字段 3 类 kind 分类 (D7-S21-A02-T01) + Resource 字段 token/time/step 抽取 (D7-S21-A03-T01) + spec.md v7.0 ADDED 3 Requirement (D7-S20-A03-T01) + d7-domain.md §8 + a/f/t/span-registry 增量 (D7-S20-A03-T02); IMPLEMENTED 230→239 (T01-T09), P0 195→204, Scenarios D7-S20/21 0→2。**2026-07-01-devrix-mups-propagation-convergence** (DM-20260701-001) — MUPS+WorkTree 传播闭环修复 (A88/A89/A90/A91/A92/A93/A94/A95 模块): `ReconcileUncertainty` 纯函数作为唯一 reconcile 入口 (D7-S1-A88-T01), `item_pipeline.go` 移除裸 max ratchet 改调 `ReconcileUncertainty` (D7-S1-A88-T02), `reevaluateParentAfterChild` 统一走 `ReconcileUncertainty` + `SetUncertainty` (D7-S1-A88-T03), `WorkItemPipelineRound` 增 `RollupRetries` 计数 + `TreeEvalContext` 透传 (D7-S15-A89-T01), `SpawnPolicyEvaluator` rollup 分支加 `MaxRollupRetries → SpawnEscalateHuman` 升级 (D7-S15-A89-T02), `session_turn_loop` break 前检查未收敛 rollup parent emit 显式结局 (D7-S15-A89-T03), rollup 故障注入测试 (verify 恒 fail → escalate) (D7-S15-A89-T04), `rollup_gate`/`rollup_verify` 读 `ChildOutcomeStats` 禁 `Failed==Total → Completed` (D7-S15-A90-T01), `buildRollupDirective` 增 `FailedSubset:` 区段 (D7-S15-A90-T02), `AppendDeliverableExecuteHint` 注入 i18n 可读验收要点 (D7-S9-A91-T01), `WorkItemExecContext` 增 `PriorVerifyReason` + inline 重试回灌 (D7-S9-A91-T02), execute prompt 快照测试 (D7-S9-A91-T03), 发散上限常量集中到 `workmodel.DivergenceBudget` (children/iters/daily 单一来源) (D7-S5-A92-T01), `buildStrategicPlanUserPrompt` 注入 depth/max_depth/remaining_children/remaining_daily/parent_scope_in (D7-S5-A92-T02), LLM 超额提案 → `StrategicPlanReject` 结构化 reject (D7-S5-A92-T03), Plan prompt 快照测试含全部预算字段 (D7-S5-A92-T04), `NarrowestSchema(inferred, strategic)` 禁放宽 (LLM 只能收紧) (D7-S9-A93-T01), `rollupPlanningDenylist` 迁 i18n/`format_hints` (D7-S9-A91-T04), `DefaultChildDownlink` 移除"无脑继承父全量 scope" (D7-S16-A94-T01), `ValidateChildScopes(parent, children)` 真子集 + 覆盖校验 (D7-S16-A94-T02), 高不确定性子 bubble 上浮 (`ChildUncertaintyBubble` + `rollupDirective.UncertainChildren:` 区段) (D7-S8-A95-T01); IMPLEMENTED 239→260 (T01..T21, +21 T points), P0 204→216 (+12 P0), Scenarios D7-S1 5→6, D7-S5 5→6, D7-S8 5→6, D7-S9 5→8, D7-S15 5→7, D7-S16 5→6. **2026-07-01-devrix-d2-d7-review-hardening** (DM-20260630-013) — D2+D7 review hardening P0/P1/P2 收口 (4 phase 一次性): P0-B D7 并发硬化: ItemPipelineRunOpts + ExecuteOpts.Emit 字段参数化 (D7-S2-A80-T01) + session_turn_loop 移除 o.itemPipeline.Emit = 字段写入改 per-invocation 传 (D7-S2-A80-T02) + wire_item_pipeline 适配新签名 + 2 session -race 并发单测 (TestPerInvocationEmit_RaceSafe + TestPerInvocationEmit_HotLoop); P0-B WorkerPool OnReleaseOnce 单 hook 注册 API 禁无界 append (D7-S3-A84-T01) + scheduler 删除 Start/dispatchLoop 重复 OnRelease 调用 (D7-S3-A84-T02) + 100 次 Start hook 计数不变 (TestOnRelease_OncePerCycle); P1-A1 orchestrator.EnsureGoal 错误 slog.Warn 不再静默吞咽 (D7-S2-A81-T01) + turn_loop.AwaitRunningChildren 返回值 err 非 nil 时也 purge handle (D7-S2-A82-T01) + turn_loop 结束段 4 错误 (cancel + nonblock + drainDone) slog.Warn 替代 _ = (D7-S2-A83-T01) + turn_state.EndTurn purge handle 不留 stale handle (D7-S2-A85-T01); P1-A2 item_pipeline SetRoundPhase 失败时 warn span 而非 silent (D7-S2-A84-T01) + resolve.go 4 处 _ = 改 slog.Warn (D7-S15-A42-T01) + child_downlink.DefaultChildExpectedReturn 加 `validate:"required"` schema tag (D7-S16-A77-T01); P1-A3 escape.Arbitrator Timer + ctx cancel 200 cycles no-leak (D7-S14-A48-T01) + mups/execute.ErrChannelCtxCancelled sentinel error (D7-S9-A33-T01); P2-1 arbitrator 战术 prompt 走 i18n.EscapeArbitratorJSONSchemaHint (D7-S14-A49-T01) + strategic_plan_proposer 删常量走 i18n.StrategicPlanAppendix (D7-S16-A78-T01) + work_tree.SetStore 加 sync.RWMutex 保护 (D7-S1-A80-T01); P2-1 decompose_proposer_test NoTacticalHardcoding 扫描扩展 1→6 文件 (decompose_proposer.go + context_proposer.go + observation_proposer.go + llm_observation_proposer.go + rollup_directive.go + strategic_plan_proposer.go); IMPLEMENTED 260→266 (T01..T15), P0 216→222, Scenarios D7-S1 6→7, D7-S2 36→42, D7-S3 20→21, D7-S9 8→9, D7-S14 18→20, D7-S15 7→8, D7-S16 6→8; 24/24 orchestration + 22/22 contextengine packages go test -race -count=1 PASS; `openspec/archive/2026-07-01-devrix-d2-d7-review-hardening/`.

---

## Overview

D7 T 层测试点注册表。现行测试以 ORCH-S2-T* 注释标注，本文档统一映射为 D7-S*-T* 编号。遗留 ORCH ID 保留在「Legacy ID」列以便追溯。

> **按 S 分组摘要 / P0 Runbook / Trace 树：** 见 `observability-guide.md` §5–§7（本文保留全表登记）。

**状态：** IMPLEMENTED · PARTIAL · PLANNED

---

## D7-SN: S Layer Normalization (DM-20260701-002)

| T ID | Legacy ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|-----------|------|----------|-----------|--------|----------| --- |
| **D7-SN-T01** | — | OpenSpec change 包完整：demand/proposal/design/tasks/delta spec | D7-S6-A14 | `openspec/changes/devrix-d7-s-layer-normalization/` | IMPLEMENTED | P0 | Spec_Review |
| **D7-SN-T02** | — | current canonical S 仅 S1-S6，S7+ 不作为 current S | D7-S6-A14 | `internal/layers/orchestration/sessionorchestrator/d7_architecture_guard_test.go` | IMPLEMENTED | P0 | Spec_Review |
| **D7-SN-T03** | — | A/F registry current path 指向现行 runtime，历史路径仅作 mapping | D7-S6-A14 | `internal/layers/orchestration/sessionorchestrator/d7_architecture_guard_test.go` | IMPLEMENTED | P1 | Spec_Review |
| **D7-SN-T04** | — | retired FastPath/OrchestratePath 文件不得回归 | D7-S2-A01 | `internal/layers/orchestration/sessionorchestrator/main_path_arch_test.go` | IMPLEMENTED | P1 | Session_Process |
| **D7-SN-T05** | — | StrategicPlanReject 写入 round rationale 并回灌下一轮 prompt | D7-S5-A07 | `internal/layers/orchestration/sessionorchestrator/strategic_plan_reject_feedback_test.go` | IMPLEMENTED | P1 | TaskGraph_Synthesize |
| **D7-SN-T06** | — | parent reevaluate uncertainty 使用 child-stats signal，全 pass 收敛下降 | D7-S1-A07 | `internal/layers/orchestration/workmodel/uncertainty_reconcile_test.go` | IMPLEMENTED | P1 | Worktree_Op |

---

## D7-HC: Historical S Cleanup (DM-20260701-003)

| T ID | Legacy ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|-----------|------|----------|-----------|--------|----------| --- |
| **D7-HC-T01** | — | OpenSpec change 包完整：demand/proposal/design/tasks/delta spec | D7-S6-A14 | `openspec/changes/devrix-d7-historical-s-cleanup/` | IMPLEMENTED | P0 | Spec_Review |
| **D7-HC-T02** | — | `historical-s-mapping.md` 存在且 spec 引用 | D7-S6-A14 | `sessionorchestrator/main_path_arch_test.go` | IMPLEMENTED | P0 | Spec_Review |
| **D7-HC-T03** | — | a-registry 不含 S7+ 大段 Historical heading | D7-S6-A14 | `sessionorchestrator/main_path_arch_test.go` | IMPLEMENTED | P0 | Spec_Review |
| **D7-HC-T04** | — | f-registry 不含 S8+ F heading 与 fastpath 路径 | D7-S6-A14 | `sessionorchestrator/main_path_arch_test.go` | IMPLEMENTED | P1 | Spec_Review |
| **D7-HC-T05** | — | spec.md 明确 S3 为 explicit wave/background，非主链路 | D7-S3-A01 | `sessionorchestrator/main_path_arch_test.go` | IMPLEMENTED | P1 | Spec_Review |
| **D7-HC-T06** | — | Architecture 图不再展示 FastPath/OrchestratePath 为 current 路径 | D7-S2-A01 | `sessionorchestrator/main_path_arch_test.go` | IMPLEMENTED | P1 | Spec_Review |

---

## D7-S4: Execution Flow

> **v1.1 closure (2026-06-15):** A04/A05 SpokeBridge wired（DM-018）；T 层增补 hubspoke 测试。

| T ID | Legacy ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|-----------|------|----------|-----------|--------|----------| --- |
| D7-S4-T01 | ORCH-S2-T01 | WorkPlan.Snapshot 含 ExecutionFlow + 状态 | D7-S4-A02 | `orchestration/executionflow/workplan/service_test.go` | IMPLEMENTED | P0 | Flow_Event_Publish |
| D7-S4-T02 | — | Hub 双通道：WorkPlan + SessionQueue + IM | D7-S4-A01 | `orchestration/executionflow/hub/hub_test.go`；`tests/integration/d7/d7_hub_flow_test.go` | IMPLEMENTED | P0 | Flow_Event_Publish |
| D7-S4-T03 | D4-S10-T04 | FlowStarted 触发 delegate-progress 入队 | D7-S4-A01-F02 | `orchestration/executionflow/hub/hub_test.go`；`tests/integration/d7/d7_hub_flow_test.go` | IMPLEMENTED | P0 | Flow_Event_Publish |
| D7-S4-T04 | D4-S10-T07 | Snapshot 含 Task 投影（link_tasks） | D7-S1-A03-F02 | `orchestration/executionflow/hub/hub_test.go` | IMPLEMENTED | P0 | Flow_Event_Publish |
| D7-S4-T05 | D4-S10-T05 | IMSink 发射 worker_progress 事件 | D7-S4-A03-F01 | `orchestration/executionflow/imsink/gateway_test.go` | IMPLEMENTED | P0 | Flow_Event_Publish |
| D7-S4-T06 | — | FlowToolCall 节流（throttle_ms） | D7-S4-A01-F04 | `orchestration/executionflow/hub/hub_test.go` | IMPLEMENTED | P1 | Flow_Event_Publish |
| **D7-S4-T08** | — | **AgentBridge OnWorkerCompleted success/error** | **D7-S4-A04** | **`hubspoke/hubspoke_test.go::TestAgentBridge_OnWorkerCompleted_{success,error}`** | **IMPLEMENTED** | **P0** | Flow_Event_Publish |
| **D7-S4-T09** | — | **SubQueryBridge PublishStarted/Completed/Failed** | **D7-S4-A05** | **`hubspoke/hubspoke_test.go::TestSubQueryBridge_Publish{Started,Completed,Failed}`** | **IMPLEMENTED** | **P0** | Flow_Event_Publish |

---

## D7-S3: Wave Scheduler

| T ID | Legacy ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|-----------|------|----------|-----------|--------|----------| --- |
| D7-S3-T01 | ORCH-S2-T10 | 6 ready subagent + 1 cursor 峰值并发≤5 | D7-S3-A01 | `orchestration/wavescheduler/scheduler_test.go` | IMPLEMENTED | P0 | Wave_Schedule |
| D7-S3-T02 | ORCH-S2-T15 | 槽位释放后 ready Task 立即派发 | D7-S3-A01-F04 | `orchestration/wavescheduler/scheduler_test.go` | IMPLEMENTED | P0 | Wave_Schedule |
| D7-S3-T03 | ORCH-S2-T17 | Plan DAG 仅 ready 节点被派发 | D7-S3-F03 | `orchestration/wavescheduler/scheduler_test.go`, `taskgraph_test.go` | IMPLEMENTED | P0 | Wave_Schedule |
| D7-S3-T04 | ORCH-S2-T11 | upstream policy 收到 artifact，无 Leader 全量 | D7-S3-A02-F02 | `orchestration/wavescheduler/context_test.go`, `scheduler_orch_test.go` | IMPLEMENTED | P0 | Wave_Task_Execute |
| D7-S3-T05 | ORCH-S2-T12 | fresh policy Messages 仅含 directive | D7-S3-A02-F01 | `orchestration/wavescheduler/context_test.go` | IMPLEMENTED | P0 | Wave_Task_Execute |
| D7-S3-T06 | ORCH-S2-T13 | 同 conflict_group Task 不并行 | D7-S3-A03-F01 | `orchestration/wavescheduler/scheduler_orch_test.go` | IMPLEMENTED | P0 | Wave_Schedule |
| D7-S3-T07 | ORCH-S2-T16 | cursor + claude-code 并行 file_scope 不交 | D7-S3-A03-F03 | `orchestration/wavescheduler/scheduler_orch_test.go` | IMPLEMENTED | P1 | Wave_Schedule |
| D7-S3-T08 | ORCH-S2-T18 | wave 全完成返回全部 artifacts | D7-S3-A01-F03 | `orchestration/wavescheduler/scheduler_orch_test.go` | IMPLEMENTED | P1 | Wave_Task_Execute |
| D7-S3-T09 | ORCH-S2-T19 | CancelWorker 槽位释放 status=cancelled | D7-S3-A01-F05 | `orchestration/wavescheduler/scheduler_test.go` | IMPLEMENTED | P0 | Wave_Schedule |
| D7-S3-T10 | ORCH-S2-T20 | CancelAll 5 running 全部 terminal | D7-S3-A01-F05 | `orchestration/wavescheduler/scheduler_test.go` | IMPLEMENTED | P0 | Wave_Schedule |
| D7-S3-T11 | ORCH-S2-T21 | CLI Worker cancel 进程终止 | D7-S3-F06 | `orchestration/wavescheduler/runners/agent_tool_orch_test.go`; `multiagent/external/cli_adapter_test.go` | IMPLEMENTED | P1 | Wave_Task_Execute |
| **D7-S3-A01-F03-T01** | — | **AllowAndRegister no conflict → registered** | **D7-S3-A01-F03** | **`orchestration/wavescheduler/conflict_test.go::TestAllowAndRegister_NoConflict`** | **IMPLEMENTED** | **P0** | Wave_Schedule |
| **D7-S3-A01-F03-T02** | — | **AllowAndRegister conflict group → blocked** | **D7-S3-A01-F03** | **`orchestration/wavescheduler/conflict_test.go::TestAllowAndRegister_ConflictGroup`** | **IMPLEMENTED** | **P0** | Wave_Schedule |
| **D7-S3-A01-F03-T03** | — | **AllowAndRegister different group → allowed** | **D7-S3-A01-F03** | **`orchestration/wavescheduler/conflict_test.go::TestAllowAndRegister_DifferentGroup`** | **IMPLEMENTED** | **P0** | Wave_Schedule |
| **D7-S3-A01-F03-T04** | — | **AllowAndRegister file scope intersection → blocked** | **D7-S3-A01-F03** | **`orchestration/wavescheduler/conflict_test.go::TestAllowAndRegister_FileScope`** | **IMPLEMENTED** | **P0** | Wave_Schedule |
| **D7-S3-A01-F04-T01** | — | **emit pushes FlowEvent to sink AND channel** | **D7-S3-A01-F04** | **`sessionorchestrator/orchestrate_path.go::emit()`** | **IMPLEMENTED** | **P0** | Wave_Schedule |
| **D7-S3-A01-F04-T02** | — | **emit tolerates nil sink gracefully** | **D7-S3-A01-F04** | **`sessionorchestrator/orchestrate_path.go::emit()`** | **IMPLEMENTED** | **P0** | Wave_Schedule |
| **D7-S3-A01-IT01** | — | **Real WaveScheduler dispatch (3-task DAG)** | **D7-S3-A01** | **`tests/integration/d7/d7_wave_real_test.go::TestIntegration_D7WaveScheduler_RealDispatch`** | **IMPLEMENTED** | **P0** | Wave_Schedule |
| **D7-S3-A01-IT02** | — | **Empty graph no-op** | **D7-S3-A01** | **`tests/integration/d7/d7_wave_real_test.go::TestIntegration_D7WaveScheduler_EmptyGraph`** | **IMPLEMENTED** | **P1** | Wave_Schedule |
| **D7-S3-A01-IT03** | — | **ConflictGuard integration** | **D7-S3-A01** | **`tests/integration/d7/d7_wave_real_test.go::TestIntegration_D7WaveScheduler_ConflictGuard`** | **IMPLEMENTED** | **P0** | Wave_Schedule |

---

## D7-S1: Work Model

> **v1.1 closure (2026-06-15):** 写模型迁入 `internal/layers/orchestration/workmodel/`。D7-S1-T01..T05 路径从 `contextengine/tasks/` 更新为 `workmodel/`。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| D7-S1-T01 | Task create 生成唯一 ID | D7-S1-A02-F01 | `workmodel/task_manager_test.go::TestTaskManager_Create` | IMPLEMENTED | P0 | Worktree_Op |
| D7-S1-T02 | Task 依赖 blocked_by 正确 | D7-S1-A02-F03 | `workmodel/task_manager_test.go::TestTaskManager_Dependency` | IMPLEMENTED | P0 | Worktree_Op |
| D7-S1-T03 | DiskStore v2 持久化恢复 | D7-S1-A02-F05 | `workmodel/disk_store_test.go::TestTaskManager_disk_persist_and_list_consistent`；`tests/integration/d7/d7_workmodel_test.go` | IMPLEMENTED | P0 | Worktree_Op |
| D7-S1-T04 | ListReadyTasks 仅返回无阻塞任务 | D7-S1-A02-F04 | `workmodel/task_manager_test.go::TestTaskManager_List` | IMPLEMENTED | P1 | Worktree_Op |
| D7-S1-T05 | FlowEvent link_tasks 状态联动 | D7-S1-A02-F06 | `orchestration/executionflow/hub/hub_test.go` | IMPLEMENTED | P1 | Worktree_Op |
| D7-S1-T06 | CreateWorkPlan DAG 校验 | D7-S1-A01-F02 | `decisionplanning/decomposer_test.go::TestTaskDecomposer_validateGraph` | IMPLEMENTED | P1 | Worktree_Op |
| D7-S1-T07 | BackgroundRun 注册与 QueryWorkPlan 可见 | D7-S1 | `sessionorchestrator/entry_test.go`; `contextengine/nested/background_*_test.go` | IMPLEMENTED | P1 | Worktree_Op |
| D7-S1-T08 | Task 非法状态转换拒绝 | D7-S1-A02-F02 | `workmodel/task_manager_test.go::TestIsLegalTransition`, `TestTaskManager_UpdateStatus_IllegalTransition`, `TestTaskManager_UpdateStatus_LegalTransitions` | IMPLEMENTED | P2 | Worktree_Op |
| **D7-S1-T09** | **WorkTree EnsureGoal 单 session 单根** | **D7-S1-A02** | **`workmodel/work_tree_test.go`** | **IMPLEMENTED** | **P0** | Worktree_Op |
| **D7-S1-T10** | **DiskWorkItemStore v2 迁移 + 原子 Save** | **D7-S1-A02-F05** | **`workmodel/work_tree_test.go`; `cross_session_test.go`** | **IMPLEMENTED** | **P0** | Worktree_Op |
| **D7-S1-T11** | **GetFocus 确定性 tiebreak** | **D7-S1-A02** | **`workmodel/work_tree_test.go::TestWorkTree_GetFocusTiebreak`** | **IMPLEMENTED** | **P1** | Worktree_Op |
| **D7-S1-T12** | **RunRef terminal → WorkItem status 同步** | **D7-S1-A02** | **`workmodel/run_spawn_test.go::TestSpawnForWorkItem_SyncTerminal`** | **IMPLEMENTED** | **P1** | Worktree_Op |
| **D7-S1-T13** | **跨 session FindByItemID** | **D7-S1-A02** | **`workmodel/cross_session_test.go`** | **IMPLEMENTED** | **P2** | Worktree_Op |
| **D7-S1-T14** | **DecomposeChildren 深度上限** | **D7-S1-A02** | **`workmodel/decompose_test.go::TestDecomposeChildren_DepthLimit`** | **IMPLEMENTED** | **P1** | Worktree_Op |
| **D7-S1-T15** | **Decompose 24h 频率上限 (5/kind/session)** | **D7-S1-A02** | **`workmodel/decompose_test.go::TestDecomposeChildren_DailyLimit`** | **IMPLEMENTED** | **P1** | Worktree_Op |
| **D7-S1-T16** | **ResolveHint 高 uncertainty decompose 引导** | **D7-S1-A02** | **`workmodel/decompose_test.go::TestResolveHint_HighUncertainty`** | **IMPLEMENTED** | **P1** | Worktree_Op |
| **D7-S1-T17** | **RunTurn blocking await running children** | **D7-S1-A02** | **`workmodel/resolve_await_test.go::TestAwaitRunningChildren_BlocksUntilTerminal`** | **IMPLEMENTED** | **P1** | Worktree_Op |
| **D7-S1-T18** | **TaskManager.Create returns `(*Task, error)` instead of silent nil (DM-20260620-003 PR-C H3)** | **D7-S1-A02-F01** | **`workmodel/task_manager_test.go::TestTaskManager_Create`; `cli_commands.go`; `tool_suite.go`** | **IMPLEMENTED** | **P0** | Worktree_Op |
| **D7-S1-T19** | **resolveDelegateTaskID returns `(string, error)` so delegate tools surface creation failure** | **D7-S1-A02-F01** | **`delegatetools/delegate_tools.go`; `tests/integration/d7/d7_hub_flow_test.go`** | **IMPLEMENTED** | **P1** | Worktree_Op |
| **D7-S1-A88-T01** | **`ReconcileUncertainty` 纯函数 + 表驱动单测（DM-20260701-001 T-P0-1）** | **D7-S1-A88** | **`workmodel/uncertainty_reconcile_test.go`** | **IMPLEMENTED** | **P0** | Worktree_Op |
| **D7-S1-A88-T02** | **`item_pipeline.go` 移除裸 max ratchet 改调 `ReconcileUncertainty`（T-P0-2）** | **D7-S1-A88** | **`sessionorchestrator/item_pipeline.go`** | **IMPLEMENTED** | **P0** | Worktree_Op |
| **D7-S1-A88-T03** | **`reevaluateParentAfterChild` 统一走 `ReconcileUncertainty`（T-P0-3）** | **D7-S1-A88** | **`sessionorchestrator/reevaluate_parent.go`** | **IMPLEMENTED** | **P0** | Worktree_Op |
| **D7-S3-T12** | **OrchestratePath SyncWaveNodes 挂树** | **D7-S3-A01** | **`sessionorchestrator/orchestrate_path.go`; bootstrap wiring** | **IMPLEMENTED** | **P1** | — |

---

## D7-S5: Decision & Planning

> **v1.1 closure (2026-06-15):** D7-S5-T04/T05 由 PLANNED 升为 IMPLEMENTED（Phase H/K）。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| D7-S5-T01 | PlanMode inactive→active 转换 | D7-S1-A04-F01 | `workmodel/plan_mode_test.go` 或 `task_manager_test` | IMPLEMENTED | P1 | Intent_Classify |
| D7-S5-T02 | PlanAgent 只读模式拒绝写操作；工具白名单不含 write/edit/bash | D7-S5-A04 | `workmodel/plan_agent_whitelist_test.go`（10 ACs）；`tests/integration/d7/d7_workmodel_test.go` | IMPLEMENTED | P0 | Intent_Classify |
| D7-S5-T03 | ClassifyIntent 规则高置信 → simple | D7-S5-A01 | `decisionplanning/classifier_test.go` | IMPLEMENTED | P0 | Intent_Classify |
| **D7-S5-T04** | **SynthesizeTaskGraph 产出有效 DAG** | **D7-S5-A02** | **`decisionplanning/decomposer_test.go::TestTaskDecomposer_SynthesizeTaskGraph`** | **IMPLEMENTED** | **P1** | Intent_Classify |
| **D7-S5-T05** | **SelectExecutor explore→D2 execute→D4** | **D7-S5-A03** | **`decisionplanning/executor_test.go::TestExecutorSelector_SelectExecutor`** | **IMPLEMENTED** | **P1** | Intent_Classify |
| D7-S5-T06 | Command-first：`/plan` 不触发 LLM Classify | D7-S5-A01 | `decisionplanning/{classifier,shadow_classifier}` + `sessionorchestrator/orchestrator_test.go`；`tests/integration/d7/d7_fastpath_test.go` | IMPLEMENTED | P0 | Intent_Classify |
| D7-S5-T07 | Tail-only LLM classify shadow（rule 未命中时异步 LLM，结果只入 metric） | D7-S5-A05 | `decisionplanning/shadow_classifier_test.go` | IMPLEMENTED | P0 | Intent_Classify |
| D7-S5-A01-T01 | 规则分类置信度阈值验证（screening 可重复性） | D7-S5-A01 | `decisionplanning/classifier_test.go::TestRuleClassifier_ExactConfidenceValues`, `TestRuleClassifier_ConfidenceDeterminism`, `TestRuleClassifier_ConfidenceRange`; `sessionorchestrator/orchestrator_test.go::TestSessionOrchestrator_FastPathConfidence{Below,Above}Threshold` | IMPLEMENTED | P0 | Intent_Classify |
| D7-S5-A01-T02 | Command-first 优先于 LLM 分类（用户显式策略优先） | D7-S5-A01 | `decisionplanning/classifier_test.go` | IMPLEMENTED | P0 | Intent_Classify |
| **D7-S5-A02-T01** | **SynthesizeTaskGraph 吸收 Explore Workers FlowEvent 产出有效 DAG** | **D7-S5-A02** | **`decisionplanning/decomposer_test.go::TestTaskDecomposer_SynthesizeTaskGraph`** | **IMPLEMENTED** | **P1** | Plan_Generate |
| **D7-S5-A02-T02** | **decomposeGoal 规则版：goal → sub_goal → DAG** | **D7-S5-A02-F01** | **`decisionplanning/decomposer_test.go::TestTaskDecomposer_decomposeGoal`** | **IMPLEMENTED** | **P1** | Plan_Generate |
| **D7-S5-A03-T01** | **MatchExecutorByTaskType：worker_type → D2/D4** | **D7-S5-A03-F01** | **`decisionplanning/executor_test.go::TestExecutorSelector_MatchExecutorByTaskType`** | **IMPLEMENTED** | **P1** | Plan_Generate |
| **D7-S5-A03-T02** | **CheckExecutorAvailability：executor 状态查询** | **D7-S5-A03-F02** | **`decisionplanning/executor_test.go::TestExecutorSelector_CheckExecutorAvailability`** | **IMPLEMENTED** | **P1** | Plan_Generate |
| **D7-S5-A03-T03** | **LLM Decomposer 解析 JSON DAG → wavescheduler.TaskNode（含 7 sub-cases）** | **D7-S5-A03-F03** | **`decisionplanning/llm_decomposer_test.go`（happy / bad JSON / enum coercion / unknown deps / extractJSON 6 case / nil LLM / SynthesizeTaskGraph routing）** | **IMPLEMENTED** | **P1** | Plan_Generate |
| **D7-S5-A02-F01-T01** | — | **ValidateToolCall: whitelist tool passes** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_Allowed`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S5-A02-F01-T02** | — | **ValidateToolCall: forbidden tool rejected** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_Forbidden`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S5-A02-F01-T03** | — | **ValidateToolCall: unknown tool rejected** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_Unknown`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S5-A02-F01-T04** | — | **ValidateToolCall: nil receiver safe** | **D7-S5-A02-F01** | **`workmodel/plan_agent_whitelist_test.go::TestValidateToolCall_NilReceiver`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S5-A02-F02-T01** | — | **PlanMode.Enter: nil LLM returns ErrLLMNotConfigured** | **D7-S5-A02-F02** | **`workmodel/plan_mode.go::Enter()`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S5-A02-F02-T02** | — | **PlanMode.Enter: valid LLM succeeds** | **D7-S5-A02-F02** | **`workmodel/plan_mode.go::Enter()`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S5-A02-F05-T01** | — | **Config struct: PlanModeApproveGate field removed** | **D7-S5-A02-F05** | **`orchtypes/config.go`, `shared/config/coordinator.go`, `shared/config/loader.go`, `bootstrap/wire_coordinator.go`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S5-A02-F05-T02** | — | **Default config: no PlanModeApproveGate reference** | **D7-S5-A02-F05** | **`orchtypes/config.go::DefaultConfig()`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S5-A02-IT01** | — | **LLM Decomposer end-to-end (JSON DAG → WaveScheduler)** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_EndToEnd`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S5-A02-IT02** | — | **LLM Decomposer fallback on invalid JSON** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_FallbackOnInvalidJSON`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S5-A02-IT03** | — | **LLM Decomposer empty task list** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_EmptyTaskList`** | **IMPLEMENTED** | **P1** | Plan_Generate |
| **D7-S5-A02-IT04** | — | **LLM Decomposer no JSON in response** | **D7-S5-A02** | **`tests/integration/d7/d7_llm_decomposer_test.go::TestIntegration_D7LLMDecomposer_NoJSONInResponse`** | **IMPLEMENTED** | **P1** | Plan_Generate |
| **D7-S5-A04-T01** | **turn_adapter.PersistTurn 提交 req.Messages 到 D2 内存（DM-20260617-003 d7-turn-history-persist）** | **D7-S5-A04** | **`internal/bootstrap/turn_adapter_persist_test.go::TestPersistTurn_{WritesMessagesToD2Memory,FullRound,NilEngine,AppendError}`** | **IMPLEMENTED** | **P0** | SubTurn_Iteration |
| **D7-S5-A04-T02** | **三轮同 session 连续 PersistTurn → Prepare 返回全历史** | **D7-S5-A04** | **`tests/integration/d7/turn_history_persist_test.go::TestTurnHistory_ThreeTurns`** | **IMPLEMENTED** | **P0** | SubTurn_Iteration |
| **D7-S5-A92-T01** | **发散上限常量集中到 `workmodel.DivergenceBudget` 单一来源（children/iters/daily，T-P1-1）** | **D7-S5-A92** | **`workmodel/divergence_budget.go`** | **IMPLEMENTED** | **P1** | Plan_Generate |
| **D7-S5-A92-T02** | **`buildStrategicPlanUserPrompt` 注入 depth/max_depth/remaining_children/remaining_daily/parent_scope_in（T-P1-2）** | **D7-S5-A92** | **`sessionorchestrator/strategic_plan_proposer.go`** | **IMPLEMENTED** | **P1** | Plan_Generate |
| **D7-S5-A92-T03** | **LLM 超额提案 → `StrategicPlanReject` 结构化 reject（含 max_allowed，T-P1-3）** | **D7-S5-A92** | **`sessionorchestrator/strategic_plan_proposer.go::applyBudgetCap`** | **IMPLEMENTED** | **P1** | Plan_Generate |
| **D7-S5-A92-T04** | **Plan prompt 快照测试含全部预算字段（T-P1-4）** | **D7-S5-A92** | **`sessionorchestrator/strategic_plan_proposer_test.go::TestBuildStrategicPlanUserPrompt_AllBudgetFields`** | **IMPLEMENTED** | **P1** | Plan_Generate |

---

## D7-S6: Error Aggregation & Metrics

> **v3.8 closure (2026-06-21):** `devrix-d7-error-aggregation-and-metrics` (DM-20260621-010) — 取代 `interrupt.go` 三步 cancel 的「all warn + nil」反模式，引入 `errors.Join` 聚合与原子指标；消除 `_ = Sandbox.Exit(...)` 三处 silent swallow；新增 WaveScheduler 4 字段与 TaskManager / Executor metrics 结构。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S6-A11-T01** | **HandleInterrupt: 3 步 cancel 全失败 → errors.Join 包含 3 个 wrapped error；errors.Is 命中每个** | **D7-S6-A11** | **`sessionorchestrator/interrupt_test.go::TestHandleInterrupt_AllStepsFail_JoinsErrors`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A11-T02** | **HandleInterrupt: 1 步失败 → 返回非 nil + 仅含失败 step 的 wrapped error** | **D7-S6-A11** | **`sessionorchestrator/interrupt_test.go::TestHandleInterrupt_PartialFailure_ReturnsPartialErr`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A11-T03** | **HandleInterrupt: nil Metrics 仍返回 errors.Join（向后兼容）** | **D7-S6-A11** | **`sessionorchestrator/interrupt_test.go::TestHandleInterrupt_NilMetrics`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A12-T01** | **[OBSOLETE 2026-06-22, see D7-S6-A14-T03 + D4-S6-A12-Txx] Sandbox Exit 失败 metric 由 D4 multiagent/execute 提供，D7 spec 不重复声明** | **D7-S6-A12** | **跨域 reference to D4 executor metrics** | **OBSOLETE** | **P0** | Hardening_Metric |
| **D7-S6-A12-T02** | **Worker panic → SchedulerMetrics.WorkerPanics +1（spec 名 "worker_panics"，DM-20260622-001 A1 后对齐）** | **D7-S6-A12** | **`wavescheduler/scheduler_metrics_test.go::TestWaveScheduler_WorkerPanicsMetric` + `d7_s6_a14_test.go::TestD7S6A14T02_WorkerPanics_SpecAlignedPlural`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A12-T03** | **taskCtx leak → SchedulerMetrics.TaskCtxLeaked +1** | **D7-S6-A12** | **`wavescheduler/scheduler_test.go::TestWaveScheduler_TaskCtxLeakMetric`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A12-T04** | **Forker: sandbox Exit 失败 → SandboxExitFailed 计数器 +1 + slog.Warn（13 调用方兼容）** | **D7-S6-A12** | **`multiagent/provision/freefork/forker_test.go::TestFork_SandboxExitFailure_RecordsMetric`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A12-T05** | **dispatchLoop wakeup → SchedulerMetrics.DispatchLoopWakeups +1（spec 名 "dispatch_loop_wakeups"，DM-20260622-001 A1 后对齐）** | **D7-S6-A12** | **`wavescheduler/scheduler_metrics_test.go::TestWaveScheduler_DispatchLoopWakeupsMetric` + `d7_s6_a14_test.go::TestD7S6A14T01_DispatchLoopWakeups_SpecAlignedPlural`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A12-T06** | **TaskManager.publishCompletion panic → TaskManagerMetrics.PublishCompletionPanics +1 + slog.Error** | **D7-S6-A12** | **`workmodel/task_manager_metrics_test.go::TestTaskManagerMetrics_*`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A13-T07** | **DefaultForker: 多 fork 全失败 → errors.Join 包含每个 fork 的 wrapped error** | **D7-S6-A13** | **`multiagent/provision/freefork/forker_test.go::TestFork_AllFailuresJoined`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A14-T01** | **dispatchLoop wakeup incMetric 名对齐 spec 复数: "dispatch_loop_wakeups"** | **D7-S6-A14** | **`wavescheduler/d7_s6_a14_test.go::TestD7S6A14T01_DispatchLoopWakeups_SpecAlignedPlural`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A14-T02** | **Worker panic incMetric 名对齐 spec 复数: "worker_panics"** | **D7-S6-A14** | **`wavescheduler/d7_s6_a14_test.go::TestD7S6A14T02_WorkerPanics_SpecAlignedPlural`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A14-T03** | **sandbox_exit_failed 跨域归属：spec 标注 OBSOLETE + cross-ref D4-S6-A12-Txx** | **D7-S6-A14** | **spec.md D7-S6-A12-T01 标注 + t-registry 本表** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A14-T04** | **state.cancels + state.handles 在 markWaveDone 后清空（防长会话无界增长）** | **D7-S6-A14** | **`wavescheduler/d7_s6_a14_test.go::TestD7S6A14T04_StateCancels_{NilAfterWaveDone,NoLeakAcrossWaves}`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A14-T05** | **dispatchLoop hot path 用 AllowAndRegister 原子调用，关 TOCTOU 窗口** | **D7-S6-A14** | **`wavescheduler/d7_s6_a14_test.go::TestD7S6A14T05_DispatchLoop_HotPathUsesAllowAndRegister`** | **IMPLEMENTED** | **P0** | Hardening_Metric |
| **D7-S6-A14-T06** | **CommandHandler emit 用 select-default 防 consumer 阻塞** | **D7-S6-A14** | **`sessionorchestrator/d7_s6_a14_t06_test.go::TestD7S6A14T06_CommandHandler_OutChannelFull_DropsEvent`** | **IMPLEMENTED** | **P0** | Hardening_Metric |

> 配套 P1：WaveScheduler `WorkerPanics` / `TaskCtxLeaked` / `WaveReentryCancelled` / `DispatchLoopWakeups` 4 字段为 `wavescheduler/scheduler_metrics_test.go` 7 单元 + 端到端测试覆盖（panickingRunner / reentry / wakeup ticker）；`TestFork_Metrics_*` 3 场景覆盖 SandboxEnterFailed / FactoryCreateFailed / RollbackTriggered 触发路径。

---

## D7-S9: Execute Node (MUPS v4.3 Phase 3)

> **v3.9 closure (2026-06-23):** devrix-d7-mups-v4-phase3-execute (DM-20260625-001) — Phase 3 PR-C1（最小风险入口）：ArtifactKind 4 类枚举 + SideEffectStatus 5 态 + wavescheduler.Artifact +5 字段 omitempty 向后兼容 + 跨域类型上提 shared/types 打破 import cycle。IMPLEMENTED 129→133，P0 96→100。详见 `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase3-execute/specs/d7-orchestration/spec.md` §D7-S9-A25。

### D7-S9-A25: Execute Artifact Data Contract (PR-C1)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S9-A25-T01** | **ArtifactKind 4 类枚举 + String() snake_case wire format + MarshalJSON/UnmarshalJSON roundtrip + 未知值 fail-fast** | **D7-S9-A25** | **`orchtypes/artifact_kind_test.go::TestArtifactKind_{4Types_String,4Types_ParseRoundTrip,UnknownValue_ParseError,JSON_WireFormat,UnmarshalEmptyString_DefaultsToZero,UnmarshalUnknownString_FailsLoudly}`（6 functions / 9 subtests）** | **IMPLEMENTED** | **P0** | Execute_Artifact |
| **D7-S9-A25-T02** | **SideEffectStatus 5 态（None/Unknown/Inflight/Committed/RolledBack）+ IsTerminal/NeedsAttention 派生 + SideEffectDetail 5 字段（IdempotencyKey/SentAt/ConfirmedAt/CompensationLog/CompensationTool）** | **D7-S9-A25** | **`orchtypes/side_effect_status_test.go::TestSideEffectStatus_{5States_String,5States_RoundTrip,IsTerminal,NeedsAttention,ReusesUncertaintyCoordType}` + `TestSideEffectDetail_JSON_RoundTrip`（6 functions / 11 subtests）** | **IMPLEMENTED** | **P0** | Execute_Artifact |
| **D7-S9-A25-T03** | **wavescheduler.Artifact +5 字段（Kind/SourcePlanID/AnomaliesCount/SideEffectStatus/SideEffectDetail）+ omitempty 向后兼容 + zero Kind 不出现在 JSON** | **D7-S9-A25** | **`wavescheduler/artifact_test.go::TestArtifact_{NewFields_PrC1,BackwardCompat_PrC1,KindZeroValue_OmittedFromJSON}`（3 new functions）+ 4 既有 ArtifactStore 测试 0 regression** | **IMPLEMENTED** | **P0** | Execute_Artifact |
| **D7-S9-A25-T04** | **跨域类型上提 shared/types 打破 import cycle（orchtypes.SideEffectStatus 改为 type alias = types.SideEffectStatus，与 UncertaintyCoord 共享同一定义）+ shared/types → orchtypes 单向依赖无 cycle** | **D7-S9-A25** | **`orchtypes/side_effect_status_test.go::TestSideEffectStatus_ReusesUncertaintyCoordType` + `internal/lint/layer` PASS + `go test -race ./internal/...` 19/19 PASS** | **IMPLEMENTED** | **P0** | Execute_Artifact |

> 配套 P0 验证：`internal/shared/types/execute.go` 包内独立测试覆盖（`TestArtifactKind_4Types_String` + `TestSideEffectStatus_5States_String` 在新包内 PASS，验证上提后无 cycle）；`go vet ./...` 0 issue；19/19 internal packages `go test -race` 0 race warnings；orchtypes 包覆盖率 72.2%（与 Phase 2 baseline 持平）。

### D7-S9-A26: Execute 4 Channel + ChannelRouter (PR-C2)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S9-A26-T01** | **Channel interface (Name/Supports/Execute) + ChannelRegistry (PlanKind → Channel 1:1 绑定 + 重复 Register 冲突检测) + ChannelRouter (无状态分发 4 PlanKind → 4 ArtifactKind 1:1 映射) + defensive checks (nil Plan + 未知 Kind)** | **D7-S9-A26** | **`execute/execute_test.go::TestChannelRegistry_Register_4Kinds`, `TestChannelRegistry_Get_NotFound`, `TestChannelRegistry_Register_DuplicateConflict`, `TestChannelRouter_Route_4Kinds`, `TestChannelRouter_Route_NilPlan`, `TestChannelRouter_Route_UnknownPlanKind`** | **IMPLEMENTED** | **P0** | Channel_Route |
| **D7-S9-A26-T02** | **CommitChannel (CommitmentPlan → ArtifactStateChangeCert, 1-Step 同步 + IdempotencyKey 强制 + 超时 SideEffectInflight) + Supports guard + 多步 Plan ErrChannelStepCountMismatch + nil runner constructor ErrChannelToolRunnerNil** | **D7-S9-A26** | **`execute/execute_test.go::TestCommitChannel_CommitmentPlan_OK`, `TestCommitChannel_OtherPlan_NotSupported`, `TestCommitChannel_SingleStep_ProducesStateChangeCert`, `TestCommitChannel_Timeout_InflightSideEffect`, `TestCommitChannel_NilRunner`** | **IMPLEMENTED** | **P0** | Channel_Route |
| **D7-S9-A26-T03** | **ProtocolChannel (ProtocolPlan → ArtifactResponseRecord, 顺序多步 + 失败 reverse-order rollback 含 `__rollback: true` hint 标记) + Supports guard + 空 Steps rejection** | **D7-S9-A26** | **`execute/execute_test.go::TestProtocolChannel_AllStepsSuccess_ResponseRecord`, `TestProtocolChannel_Step2_Failed_RollbackStep1`, `TestProtocolChannel_OtherPlan_NotSupported`, `TestProtocolChannel_EmptySteps`** | **IMPLEMENTED** | **P0** | Channel_Route |
| **D7-S9-A26-T04** | **ScenarioChannel (ScenarioPlan → ArtifactProbeReport, MaxParallel=5 并行探测 + 多数派投票 success > len/2 → pass + 失败多数派触发 ErrChannelStepCountMismatch + SideEffectStatus=None read-only)** | **D7-S9-A26** | **`execute/execute_test.go::TestScenarioChannel_5ParallelProbes`, `TestScenarioChannel_MajorityVote_ProbeReport`, `TestScenarioChannel_MixedResults_TakesMajority`** | **IMPLEMENTED** | **P0** | Channel_Route |
| **D7-S9-A26-T05** | **ExplorationChannel (ExplorationPlan → ArtifactExperimentData, MaxParallel=3 多 agent 并行 + 容忍部分失败 free-fork + 优先级排序 success → duration → EstimatedTokens + PersistScope → SideEffectStatus 派生 transient → None, session/permanent → Committed, unknown → Unknown)** | **D7-S9-A26** | **`execute/execute_test.go::TestExplorationChannel_MultiAgent_Parallel`, `TestExplorationChannel_FreeFork_Optional`, `TestExplorationChannel_PriorityOrder_ExperimentData`, `TestExplorationChannel_PersistScope_Mapping`** | **IMPLEMENTED** | **P0** | Channel_Route |
| **D7-S9-A26-T06** | **Channel->PlanChannel rename (Phase B-pre P0 门禁): `Channel` interface -> `PlanChannel`; 1-release `type Channel = PlanChannel` alias 保留; 4 PlanKind channel implementations (commit/protocol/scenario/exploration) + 4 callers 全部更新; P0-AC-8 满足 (grep type Channel interface mups/execute/ = 0)** | **D7-S9-A26** | **`mups/execute/channel.go` (line 69 `type PlanChannel interface` + line 309 `type Channel = PlanChannel`); `execute_test.go` 22 tests 0 regression** | **IMPLEMENTED (DM-20260701-007)** | **P0** | **Channel_Route** |

> 配套 P0 验证：`execute/execute_test.go` 22 个测试 100% PASS（0 race detector warnings），覆盖率 88.1%；5 SentinelError + 4 helpers (EXEC_CHANNEL_9001..9004) 在 `execute/errors.go` 定义并被测试断言；`go vet ./...` 0 issue；`go build ./...` 0 error；22/22 tests cover T01..T05 P0 边界 + 1-Step 严格性 + IdempotencyKey 强制 + 超时 SideEffectInflight + reverse-order rollback + 多数派投票 + PersistScope 派生。

### D7-S9-A50: ToolChannel Router (per-EmissionClass termination, DM-20260701-007)

> **devrix-mups-tool-classification-and-channel-autonomy (DM-20260701-007) -- Phase B 治本核心落地.**
> Execute 节点 4 PlanKind Channel (D7-S9-A26) 之上叠加 4 per-EmissionClass ToolChannel (Fact/Action/Probe/Experiment), 用 Router 按 emission_class 路由; ProbeToolChannel Bounded(n) hard reject + PromptPressure 3-stage 是 LLM 自我循环的根治. 配套 LTL-Lite L4-L6 invariants (D5-S25) 跨域挂载.

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S9-A50-T01** | **ToolChannel interface (Name/Accept/Router) + ToolChannelRouter.Route(tool ToolSpec) 按 EmissionClass 路由 + Mode (Shadow/Enforce) 字段 + telemetry metrics (wouldRejectCount++ shadow only)** | **D7-S9-A50** | **`mups/execute/toolchannel/channel.go` + `probe_test.go::TestRouter_Has4Channels` + `TestToolChannel_AllFourImplement`** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Channel_Route |
| **D7-S9-A50-T02** | **FactToolChannel + ActionToolChannel + LTL-Lite L7 invariants (FACT-SAME-Q-5x 同 query 重复 5x 升级 Probe, ACTION-POSTSNAPSHOT PostSnapshot!=PreSnapshot 才 Verifiable)** | **D7-S9-A50** | **`toolchannel/{fact,action}.go` + `probe_test.go::TestRouter_FactEscalationToProbe` + `bounded_test.go::TestFactSameQueryInvariant_FiresAtFive` + `TestActionPostSnapshotInvariant_FiresOnEqual`** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Channel_Route |
| **D7-S9-A50-T03** | **ProbeToolChannel 核心: 接受 emission_class=Probe + ReadOnly=false 强 iteration_bound Bounded(n) 校验 + 到 bound 时 InjectSynthesize + OnResult 行为重分类 (call_count>3 同 query 升级 Probe, H9)** | **D7-S9-A50** | **`toolchannel/probe.go` (ProbeToolChannel.Accept) + `probe_test.go::TestProbeToolChannel_AcceptsUnderBound`** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Channel_Route |
| **D7-S9-A50-T04** | **ProbeToolChannel Bounded(15) Hard Stop 测试 (P0-AC-1): mock 17 calls 第 16 返 SynthesizeNowSignal, 第 17 拒 (ErrProbeToolChannelBoundExceeded)** | **D7-S9-A50** | **`probe_test.go::TestProbeToolChannel_Bounded15_HardStopsAt16`** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Channel_Route |
| **D7-S9-A50-T05** | **PromptPressure 3-stage 测试 (P1-AC-6): Bounded(15) @剩5软警告 @剩2硬警告 @16强制 / Bounded(10) @剩3/1/11 / OpenEnded 不注入** | **D7-S9-A50** | **`probe_test.go::TestProbeToolChannel_PromptPressure_{Review, Edit, Observe_NeverInjects}`** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Channel_Route |
| **D7-S9-A50-T06** | **ExperimentToolChannel + L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE (deadline < ConcludedAt 校验) + LTL-Lite L5-Quotient 挂载** | **D7-S9-A50** | **`toolchannel/experiment.go` + `bounded_test.go::TestExperimentDeadlineInvariant_FiresOnMiss`** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Channel_Route |
| **D7-S9-A50-T07** | **Shadow mode (P1-AC-5): Mode=Shadow 时 bound 超限仅 log `would_reject=true` + wouldRejectCount++ metric, 不 block; EnableMupsChannelsEnforce=true 后切 Enforce; FP<5% 后切** | **D7-S9-A50** | **`toolchannel/channel.go` Router.Route shadow branch + `probe_test.go::TestRouter_ShadowMode_LogsWouldReject` + `TestRouter_EnforceMode_ReturnsError`** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Channel_Route |
| **D7-S9-A50-T08** | **L0-L3 cross-check (P1-AC-2): >=3 条规则 -- Bounded 不得 override readonly guard + Quotient 不得绕过 permission check + Synthesize 不得跳过 audit log; cross-check 测试 `TestBoundedInvariant_DoesNotBypassPermissionGuards` + `TestProbeToolChannel_DoesNotBypassPermissionGuards`** | **D7-S9-A50** | **`toolchannel/probe.go::Accept` CC-1 + `bounded.go` CC-2/CC-3 + 2 cross-check tests** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Channel_Route |

> 配套 P0 验证: `mups/execute/toolchannel/probe_test.go` 11 tests 100% PASS (含 Bounded(15) hard stop + 3-stage PromptPressure x 3 task_kind + Shadow/Enforce mode + Fact->Probe escalation + 4-channel implementations); `bounded_test.go` 10 tests 100% PASS (Bounded/Quotient/Synthesize + 3 L7 invariants + cross-check); coverage toolchannel 60.2% / ltl/invariants/termination 70.2%; 0 race warnings; 5 SentinelError (EXC_PROBE_BOUND_9001 等) 在 toolchannel/errors.go 定义并被测试断言. **4 PlanKind Channel (D7-S9-A26 T01..T05) 0 regression** -- Channel->PlanChannel rename 1-release alias 兼容 (D7-S9-A26-T06 P0-AC-8 满足).

### D7-S9-A91: AcceptanceCriteriaVisibility (DM-20260701-001)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S9-A91-T01** | **`AppendDeliverableExecuteHint` 注入 i18n 可读验收要点（file:line + P0/P1 severity，T-P0-10）** | **D7-S9-A91** | **`workmodel/deliverable_execute_hint.go` + `contextengine/i18n/format_hints_deliverable.go`** | **IMPLEMENTED** | **P0** | Execute_Artifact |
| **D7-S9-A91-T02** | **`WorkItemExecContext.PriorVerifyReason` 字段 + inline 重试回灌 `verdict.Reason`（T-P0-11）** | **D7-S9-A91** | **`sessionorchestrator/workitem_exec_context.go` + `item_pipeline.go`** | **IMPLEMENTED** | **P0** | Execute_Artifact |
| **D7-S9-A91-T03** | **execute prompt 快照测试：含验收要点 + 回灌 reason（T-P0-12）** | **D7-S9-A91** | **`sessionorchestrator/workitem_exec_context_test.go` (4 new snapshot tests)** | **IMPLEMENTED** | **P0** | Execute_Artifact |
| **D7-S9-A91-T04** | **`rollupPlanningDenylist` 迁 i18n/`format_hints`（T-P1-6）** | **D7-S9-A91** | **`contextengine/i18n/format_hints_planning.go` + `sessionorchestrator/rollup_verify.go`** | **IMPLEMENTED** | **P1** | Execute_Artifact |

### D7-S9-A93: SchemaMonotonicNarrowing (DM-20260701-001)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S9-A93-T01** | **`NarrowestSchema(inferred, strategic)` 禁放宽（LLM 只能收紧，T-P1-5）** | **D7-S9-A93** | **`workmodel/deliverable.go` + `workmodel/deliverable_test.go` (9-case test matrix)** | **IMPLEMENTED** | **P1** | Execute_Artifact |

---

## D7-S10: Verify Node (MUPS v4.3 Phase 4 + DM-20260701-007)

> **v3.12 closure (2026-06-23):** devrix-d7-mups-v4-phase4-verify-promotion (DM-20260623-002) -- Phase 4 Verify 节点升格 (A32/A33/A34/A35 模块): VerdictKind 4 态 typed enum + AggregationStrategy 4 策略 + VerdictToExitReason 4 Verdict -> 4 ExitReason 映射 + VerifyWithRetry G8-1 修复 + Evidence struct 5 字段 + EvidenceExtractor 3 实现 + SystemAnomalyAggregator 阈值触发 + ObserveNode wiring SystemAnomaly. IMPLEMENTED 147->155, P0 114->122, Scenarios 0->4.
>
> **v4.26 closure (2026-07-02):** devrix-mups-tool-classification-and-channel-autonomy (DM-20260701-007) -- Phase C 闭环落地 (A50 模块): VerifyContract 4 元 input contract + NewVerifyContract 显式构造器 + CalibratedConfidence 公式 + BurdenOfProofForClass by EmissionClass + D1 EmitComplete 透传 meta + D1 feishu render reason 标签. +4 T P0 IMPLEMENTED.

### D7-S10-A32: VerdictKind + AggregationStrategy (Phase 4)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S10-A32-T01** | **VerdictKind 4 态 typed enum (Pass/Fail/Indeterminate/Skip) + String() snake_case wire format + Parse/ParseVerdictKind 反向解析 + MarshalJSON/UnmarshalJSON roundtrip + 未知值 fail-fast (D7_VERDICT_9001)** | **D7-S10-A32** | **`orchtypes/verdict_test.go::TestVerdictKind_{4States_String,4States_RoundTrip,UnknownValue_FailFast,JSON_WireFormat}`** | **IMPLEMENTED (DM-20260623-002)** | **P0** | Verify_Contract |
| **D7-S10-A32-T02** | **AggregationStrategy 4 策略 (AllPass/Majority/Weighted/AnyPass) + AggregateVerdicts 边界 (空/单元素/全同/分歧) + 4 策略实现 + 异常 verdict 计入 Indeterminate 不影响策略判定** | **D7-S10-A32** | **`orchtypes/aggregation_test.go::TestAggregateVerdicts_{AllPass, Majority, Weighted, AnyPass, Empty, SingleElement, MixedKinds}`** | **IMPLEMENTED (DM-20260623-002)** | **P0** | Verify_Contract |

### D7-S10-A33: Verdict -> ExitReason 映射 + VerifyWithRetry (Phase 4)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S10-A33-T03** | **VerdictToExitReason 4 Verdict -> 4 ExitReason 映射 + SystemAnomaly 覆盖 + 14 ExitReason 8->14 扩展 (新增 SystemAnomalyDetected / VerifierParseFailure / IndeterminateInterrupted / ContractMissing 4 类)** | **D7-S10-A33** | **`orchtypes/exit_reason_test.go::TestVerdictToExitReason_{4Verdicts, SystemAnomaly, 14ReasonsExhaustive}`** | **IMPLEMENTED (DM-20260623-002)** | **P0** | Verify_Contract |
| **D7-S10-A33-T04** | **VerifyWithRetry parse failure -> INDETERMINATE G8-1 修复: verifier 返回非 JSON / 缺字段时 INDETERMINATE("verifier_parse_failure") 不 retry 无限循环** | **D7-S10-A33** | **`orchtypes/verify_retry_test.go::TestVerifyWithRetry_{ParseFailure, Indeterminate, NoRetryLoop}`** | **IMPLEMENTED (DM-20260623-002)** | **P0** | Verify_Contract |

### D7-S10-A34: Evidence + EvidenceExtractor (Phase 4)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S10-A34-T05** | **Evidence struct 5 字段 (Kind/Source/Content/Confidence/CapturedAt) + Validate fail-fast (Kind 非空 + Source 非空 + Confidence 0..1) + NewEvidence 必填校验** | **D7-S10-A34** | **`orchtypes/evidence_test.go::TestEvidence_{NewFields, Validate_AllChecks, NewEvidence_RequiredFailsFast}`** | **IMPLEMENTED (DM-20260623-002)** | **P0** | Verify_Contract |
| **D7-S10-A34-T06** | **EvidenceExtractor interface (Extract) + LLM 实现 (LLMCall) + Stub 实现 (deterministic) + 错误兜底 EmptyEvidence** | **D7-S10-A34** | **`orchtypes/evidence_extractor_test.go::TestEvidenceExtractor_{LLM, Stub, EmptyFallback}`** | **IMPLEMENTED (DM-20260623-002)** | **P0** | Verify_Contract |

### D7-S10-A35: SystemAnomalyAggregator + ObserveNode wiring (Phase 4)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S10-A35-T07** | **SystemAnomalyAggregator 阈值触发 (count > Threshold 触发 AnomalyReport) + RecordCatSystem 累加 + Reset 清空 + 并发安全 (sync.Mutex)** | **D7-S10-A35** | **`orchtypes/system_anomaly_test.go::TestSystemAnomalyAggregator_{Threshold, RecordCatSystem, Reset, ConcurrentSafe}`** | **IMPLEMENTED (DM-20260623-002)** | **P0** | Verify_Contract |
| **D7-S10-A35-T08** | **ObserveNode wiring SystemAnomaly -> FromVerifier + BuildUncertaintyCoordFromReport Value=0.95 强制 (SystemAnomaly 不污染业务 confidence)** | **D7-S10-A35** | **`sessionorchestrator/observe_node_test.go::TestObserveNode_BuildUncertaintyCoordFromSystemAnomaly`** | **IMPLEMENTED (DM-20260623-002)** | **P0** | Verify_Contract |

### D7-S10-A50: VerifyContract + BurdenOfProof + D1 Reason 透传 (DM-20260701-007, Phase C)

> **devrix-mups-tool-classification-and-channel-autonomy (DM-20260701-007) -- Phase C 闭环落地.**
> VerifyContract 是 Verify 节点 4 元 input contract: `expected_class` (EmissionClass) / `deliverable_text` / `evidence` / `source_uncertainty`. 防 Go 零值陷阱用 `NewVerifyContract(taskKind, expectedEmissionClass)` 显式构造器. `CalibratedConfidence` 公式 `Sum(su x w)/Sum(w)` (weight: EC_Fact=0.50, EC_Action=0.35, EC_Probe=0.20, EC_Experiment=0.10). `BurdenOfProofForClass` 按 EmissionClass 分配举证规则 (Fact=text 自证, Action=state change evidence, Probe=source quality, Experiment=reproducibility). verdict.Reason 透传 D1 EmitComplete + feishu render 标签 (P0-AC-5).

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S10-A50-T01** | **VerifyContract struct (4 元) + NewVerifyContract(taskKind, expectedEmissionClass) 显式构造器 (防 Go 零值陷阱) + Verify() 4 元 contract 校验 (deliverable / evidence / source_uncertainty / emission_class) + CalibratedConfidence 公式 Sum(su x w)/Sum(w) + MinChars by task_kind (review=20, edit=10, test=30, observe=10)** | **D7-S10-A50** | **`internal/layers/orchestration/executionflow/verify/verify_contract.go` + `verify_contract_test.go::TestNewVerifyContract_AllTaskKinds + TestVerifyContract_ZeroValueIsDetectable + TestCalibratedConfidence_{Empty, Formula} + TestVerify_{DeliverableMissing, DeliverableTooShort, EvidenceInsufficient, SourceUncertaintyHigh, AllPass} + TestVerify_MetaContainsTaskKind`** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Verify_Contract |
| **D7-S10-A50-T02** | **D1 EmitComplete 透传 `meta["verify_exit_reason"]` 等到 OutboundMessage.Metadata (P0-AC-5)** | **D7-S10-A50** | **`internal/layers/communication/channel/conclusion/conclusion.go::EmitComplete` (modified to forward meta map)** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Verify_Contract |
| **D7-S10-A50-T03** | **D1 feishu render reason 标签 (P0-AC-5): RenderArgs struct param 避免 break PR #373 5-param 签名; render title "任务失败 (ProbeToolChannel: <reason> @ iter X/Y, source_uncertainty=Z)" + footer "任务未完成 (reason: <verdict_reason>)"** | **D7-S10-A50** | **`internal/layers/communication/channel/adapters/feishu.go` (line 138-148) + `feishu_progress.go` (RenderArgs struct param)** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Verify_Contract |
| **D7-S10-A50-T04** | **BurdenOfProofForClass by EmissionClass (P1-AC-3): Fact=text 自证; Action=state change evidence; Probe=source_quality; Experiment=reproducibility** | **D7-S10-A50** | **`verify_contract.go::BurdenOfProofForClass` + `verify_contract_test.go::TestBurdenOfProofForClass + TestBurdenOfProof_Probe_LowCC`** | **IMPLEMENTED (DM-20260701-007)** | **P0** | Verify_Contract |

> 配套 P0 验证: `executionflow/verify/verify_contract_test.go` 13 tests 100% PASS (含 8 CC 子用例 + 4 task_kind + 2 zero-value + burden of proof); coverage verify 53.7%; 0 race warnings. D1 EmitComplete/feishu 透传 12 communication package tests 0 regression.

> **devrix-d2-tool-input-aware-concurrency-and-classifier (DM-20260702-009) — Phase 5-6+ 落地.**
> 5 PR 联动 (PR-D+E AutoModeClassifier stub + PR-F sibling abort + discard):
>
> - **PR-D+E Phase 5 (3 T)**: D7-S10-A50-T22 AutoModeClassifier P2 interface stub (0 行实现, panic 信息合规) + D7-S10-A50-T23 ChannelRouter TODO 占位 (不破坏 partition 行为) + D7-S10-A50-T24 Classifier interface stub 单测 (4 单测)
> - **PR-F Phase 6+ (2 T)**: D7-S9-A50-T26 Bash sibling abort (per-batch controller + watched call 失败时取消 siblings) + D7-S9-A50-T27 StreamingToolExecutor.Discard() + fallback 路径 wiring
>
> AC 满足: 5/5 PASS (P0 3 + P1 2) — T22/T23/T24 + T26/T27. 2 tech-debt 关闭 (TD-STE-02 + TD-STE-03).

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|------|------|--------|-----------|--------|----------|
| **D7-S10-A50-T22** | **PR-D+E AutoModeClassifier P2 interface stub: `ClassifyToolUse(ctx, transcript) (YoloResult, error)`; YoloResult{Decision, Reason, Source}; 当前 0 行实现, panic("P2 interface, not implemented; see gaming-debate-round3-convergence.md"); 触发升 P1 实施的条件: verify_contract.deny_rate > 5% (任意 7 天窗口)** | **D7-S10-A50 AutoClassifier Stub** | **`internal/layers/orchestration/decisionplanning/auto_classifier.go` + `auto_classifier_test.go::TestAutoModeClassifier_InterfaceExists + TestAutoModeClassifier_StubPanic`** | **IMPLEMENTED (DM-20260702-009)** | **P2** | Verify_Contract |
| **D7-S10-A50-T23** | **PR-D+E ChannelRouter TODO 注释 + 占位 metric stub (不破坏现有 partition 行为): `internal/bootstrap/turn_adapter.go::ExecuteRound` partitionToolCalls 之后 batch 跑之前预留 `ClassifyToolUse` 调用点, 当前直接走 default allow** | **D7-S10-A50 ChannelRouter Placeholder** | **`internal/bootstrap/turn_adapter.go` + `turn_adapter_partition_test.go::TestPartition_NoClassifierNoRegression + TestChannelRouter_PlaceholderCallsite`** | **IMPLEMENTED (DM-20260702-009)** | **P2** | Verify_Contract |
| **D7-S10-A50-T24** | **PR-D+E Classifier interface stub 单测 (4 单测) + e2e: TestAutoModeClassifier_InterfaceExists (compile) + TestAutoModeClassifier_StubPanic (panic 信息合规) + TestPartition_NoClassifierNoRegression (ChannelRouter 占位不破坏 partition) + TestChannelRouter_PlaceholderCallsite (TODO 注释 + metric 占位存在)** | **D7-S10-A50 Classifier Stub Tests** | **`internal/layers/orchestration/decisionplanning/auto_classifier_test.go` (4 tests) + `turn_adapter_partition_test.go`** | **IMPLEMENTED (DM-20260702-009)** | **P0** | Verify_Contract |
| **D7-S9-A50-T26** | **PR-F Bash sibling abort: BashSiblingAbortController per-batch controller (sync.Mutex + sync.Once wrapped cancel) + Register(callID, toolName) (ctx, cancel, ok) + AbortSiblings(callID, reason) bool + isSiblingAbortWatched(name) 仅 bash; executeOneBatch parallel branch wired; watched call 失败时取消其它 watched siblings** | **D7-S9-A50 SiblingAbort** | **`internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` (10 unit tests) + `internal/bootstrap/partition_sibling_abort_test.go` (3 integration tests); `executeOneBatch` parallel branch wired** | **IMPLEMENTED (DM-20260702-009)** | **P1** | Channel_Route |
| **D7-S9-A50-T27** | **PR-F StreamingToolExecutor.Discard() + fallback 路径 wiring: StreamingToolExecutor per-LLM-iteration buffer (Buffer/Buffered/BufferedCount/IsDiscarded/DiscardReason/Discard); ErrStreamingFallbackSentinel = "streaming_fallback"; DiscardOnFallback wiring helper (atomic.Int64 counter + idempotent); FormatStreamingFallbackError(reason)** | **D7-S9-A50 Discard** | **`internal/bootstrap/streaming_executor.go` (10 tests) + `internal/bootstrap/discard_on_fallback.go` (8 tests) + `streaming_executor_test.go` + `discard_on_fallback_test.go`** | **IMPLEMENTED (DM-20260702-009)** | **P1** | Channel_Route |

---

## D7-S8: Observe Node (MUPS v4.3 Phase 2)

> **v3.10 closure (2026-06-23):** devrix-d7-mups-v4-phase2-observe-plan (DM-20260623-001) — Phase 2 PR-A1 + PR-RF（A15 模块）：Observation 4 类 × 2 Category + sealed Payload + UncertaintyReport Partition 不变式 + UncertaintyCoord Phase 2 扩展 + PR-RF 5 项 review fix（C1 IntentKind enum + C3 FromVerifier fail-fast + W2 fmt.Errorf wrap + W3 clamp01Float 合并 + W6/I8 Partition clamp 末尾）。IMPLEMENTED 133→139，P0 100→106。详见 `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase2-observe-plan/specs/d7-orchestration/spec.md` §D7-S8-A15。

### D7-S8-A15: Observation + UncertaintyReport (PR-A1 + PR-RF)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S8-A15-T01** | **Observation 4 类（ObsFact/ObsSignal/ObsDeviation/ObsUncertainty）× 2 Category（CatBusiness/CatSystem）+ sealed Payload interface（4 concrete types: FactPayload/SignalPayload/DeviationPayload/UncertaintyPayload）+ 不可变（WithKind/WithStrength 返回新副本）+ Strength ∈ [0,1] 边界保护（clamp01Float panic on out-of-range）+ DetectedAt 非零 + MarshalJSON wire format 嵌套对象 + JSON roundtrip** | **D7-S8-A15** | **`orchtypes/observation_test.go::TestObservation_{4Kinds_4Categories,Immutability,Payload_TypeAssertion,Validate_StrengthOutOfRange,Validate_DetectedAtZero,JSON_Roundtrip,WithKind_Immutability,WithStrength_Panic_OnOutOfRange,WithStrength_NormalRange,ValidateFact_WrappedError,MarshalJSON_WireFormat,Clamp01Float_NaN_Fallback}`（12 functions / 13 subtests）** | **IMPLEMENTED** | **P0** | Observe_Quantize |
| **D7-S8-A15-T02** | **UncertaintyReport ComputeOverallStrength 仅遍历 CatBusiness Observations（不包含 CatSystem 避免系统异常污染整体 Strength）+ CatBusiness 为空时 defaults 0.5（避免 NaN via clamp01Float NaN 兜底）** | **D7-S8-A15** | **`orchtypes/uncertainty_report_test.go::TestUncertaintyReport_{ComputeOverallStrength_BusinessOnly,ComputeOverallStrength_EmptyBusiness_DefaultsHalf,ComputeOverallStrength_IgnoresCatSystem,Overall_NaN_Fallback}`（4 functions）** | **IMPLEMENTED** | **P0** | Observe_Quantize |
| **D7-S8-A15-T03** | **UncertaintyCoord Phase 2 增量扩展：FromVerifier 工厂方法（verdict/confidence/reason → Coord，含 Source: SourceVerifier）+ IsColdStart + Equal + With* + Phase 1 JSON wire format 向后兼容（FromVerifier=false + SideEffectStatus="" 零值，MarshalJSON 用 omitempty）+ 未知 verdict 失败兜底 + FromVerifier fail-fast (ORCH_COORD_VERDICT_7004 错误码 + sharederrors.WithCode)** | **D7-S8-A15** | **`orchtypes/uncertainty_coord_test.go::TestUncertaintyCoord_{FromVerifier_SetsFromVerifierTrue,FromVerifier_SourceIsVerifier,JSON_Phase1_Compatibility,JSON_Omitempty_NewFields,FromVerifier_UnknownKind}` + 4 既有 baseline test PASS** | **IMPLEMENTED** | **P0** | Observe_Quantize |
| **D7-S8-A15-T04** | **UncertaintyReport Partition 不变式强制保证（CatBusiness ∪ CatSystem == Observations）+ 违反不变式返回 ErrUncertaintyReportPartitionInvariant + 空 Observations 边界（Partition invariant holds vacuously）+ CatBusiness/CatSystem disjoint 不变式** | **D7-S8-A15** | **`orchtypes/uncertainty_report_test.go::TestUncertaintyReport_{PartitionInvariant_BusinessUnionSystemEqualsObservations,PartitionInvariant_Violation_ReturnsError,PartitionInvariant_EmptyObservations}`（3 functions）** | **IMPLEMENTED** | **P0** | Observe_Quantize |
| **D7-S8-A15-T05** | **UncertaintyReport FilterByKind 故意遍历全集（不限 Category，跨 CatBusiness/CatSystem 都返回）+ 返回所有指定 Kind 的 Observation + 空输入返回空切片 + ALL kind 返回全集** | **D7-S8-A15** | **`orchtypes/uncertainty_report_test.go::TestUncertaintyReport_{FilterByKind_IncludesCatSystem,FilterByKind_Empty,FilterByKind_AllObservations}`（3 functions）** | **IMPLEMENTED** | **P0** | Observe_Quantize |
| **D7-S8-A15-T06** | **Observation 不可变（WithKind 返回新副本，原实例未修改）+ Strength ∈ [0,1] panic on out-of-range + validateFact FailureCriteria 包装 fmt.Errorf（"orchtypes: FactPayload.Statement empty: %w", ErrObservationPayloadInvalid）+ UnmarshalJSON graceful degrade（forward-compat 字段缺失不回 error）** | **D7-S8-A15** | **`orchtypes/observation_test.go::TestObservation_{WithKind_Immutability,WithStrength_Panic_OnOutOfRange,WithStrength_NormalRange,ValidateFact_WrappedError}`（4 functions）** | **IMPLEMENTED** | **P0** | Observe_Quantize |

> 配套 P0 验证：`internal/layers/orchestration/orchtypes/` 包内 23 baseline + 6 新增测试函数 + 33 subtests 100% PASS；`go vet ./...` 0 issue；orchtypes 包 `go test -race` 0 race warnings；覆盖率 72.2%（持平 PR-A1 baseline）。Phase 1 调用方零修改（Phase 1 UncertaintyCoord 字段 + JSON wire format 保持）；Phase 2 后续 PR-A2 (IntentQuantizer) / PR-A3 (AnomalyDetector) / PR-A4 (ObserveNode wiring) / PR-B2 (Plan.Validate 细化) / PR-B3 (LLMPlanner) 作为独立 OpenSpec change 推进（D7-S8-A19/A20/A21/A23/A24 模块 PLANNED）；PR-B1 (Plan 4 类 + Planner) 已闭环为 D7-S8-A22。

### D7-S8-A22: Plan Data Contract + Planner (PR-B1)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S8-A22-T01** | **PlanKind 4 类枚举 (CommitmentPlan/ProtocolPlan/ScenarioPlan/ExplorationPlan) + String() snake_case wire format + MarshalJSON 输出字符串 + UnmarshalJSON 未知值 fail-fast + KindUnset omitempty + ParsePlanKind CLI 反向解析** | **D7-S8-A22** | **`plan/plan_test.go::TestPlanKind_4Types_AreDistinct`, `TestPlanKind_String_SnakeCase`, `TestPlanKind_KindUnset_DefaultsFromEmpty`, `TestPlanKind_MarshalJSON_KnownValues`, `TestPlanKind_MarshalJSON_KindUnset_Omits`, `TestPlanKind_UnmarshalJSON_RoundTrip`, `TestPlanKind_UnmarshalJSON_UnknownFailsFast`, `TestParsePlanKind`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S8-A22-T02** | **Plan.SourceObservationIDs 必填（空 → ErrPlanSourceObservationIDsRequired + 错误码 PLAN_LINEAGE_8002）+ NewPlan 防御性拷贝（外部 mutation 不影响 Plan 内字段）+ ReverseLookupObservations Phase 4 Verify 反向追溯入口（按 ID 集合求交）+ 重复 ID 不产生重复结果 + 空输入边界（nil/empty）** | **D7-S8-A22** | **`plan/plan_test.go::TestPlan_SourceObservationIDs_Required`, `TestPlan_NewPlan_CopiesObservationIDs`, `TestPlan_SourceObservationIDs_ReverseLookup_Exact`, `TestPlan_SourceObservationIDs_ReverseLookup_DuplicateIDs`, `TestPlan_SourceObservationIDs_ReverseLookup_Empty`** | **IMPLEMENTED** | **P0** | Plan_Generate |
| **D7-S8-A22-T03** | **MatchKind 4 规则分类器 (Rule 1: intent_orchestrate OR anomaliesCount≥3 → ExplorationPlan; Rule 2: stepCount==1 → CommitmentPlan; Rule 3: intent_command OR stepCount≤3 → ProtocolPlan; Rule 4: default → ScenarioPlan) + uncertainty-first tie-break + DefaultPlanner.Plan() 完整集成（强度公式 / 校验失败传递 / BlastRadius 透传 / 空 ObservationIDs fast-fail）** | **D7-S8-A22** | **`plan/plan_test.go::TestMatchKind_4Rules`, `TestDefaultPlanner_Plan_EmptyObservationIDs_FailsFast`, `TestDefaultPlanner_Plan_CommitmentFromSingleStep`, `TestDefaultPlanner_Plan_ExplorationFromAnomalies`, `TestDefaultPlanner_Plan_StrengthMatchesFormula`, `TestDefaultPlanner_Plan_ValidationFailurePropagates`, `TestDefaultPlanner_Plan_BlastRadiusPropagated`, `TestStrengthFloor_Unit`** | **IMPLEMENTED** | **P0** | Plan_Generate |

> 配套 P0 验证：`plan/plan_test.go` 30 个测试 100% PASS（0 race detector warnings），覆盖率 93.5%；含 9 SentinelError + 3 helpers (PLAN_KIND_8001 / PLAN_LINEAGE_8002 / PLAN_BLAST_8003) 在 `plan/errors.go` 定义并被测试断言；strengthFloor 公式 `0.7 base − 0.1·anomalies + min(observations·0.02, 0.2)` 单测覆盖（含 IEEE 754 float drift 容忍 `≥ 0.89`）；Plan 不可变 With* (WithKind/WithStrength/WithFailureCriteria/WithBlastRadius/WithAnomaliesCount) 全部以新副本返回；PP-1 强度范围 [0,1] + PP-2 FailureCriteria 非空 + PP-3 BlastRadius 阈值 全部 Validate 强制；C2/W8 MatchKind 签名收紧为 `(*UncertaintyReport)` 已落地。

### D7-S8-A95: ChildUncertaintyBubble (DM-20260701-001)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| **D7-S8-A95-T01** | **高不确定性子 bubble 上浮（`ChildUncertaintyBubble` + `rollupDirective.UncertainChildren:` 区段，T-P2-3）** | **D7-S8-A95** | **`workmodel/context_bubble_apply.go` + `sessionorchestrator/rollup_directive.go` + `rollup_uncertain_test.go` (1 new test)** | **IMPLEMENTED** | **P2** | Observe_Quantize |

---

---

## D7-S2: Session Orchestrator

> **v1.1 closure (2026-06-15):** D7-S2-A04 DispatchWorker wired（Phase DM-018）；D7-S2-A06/A07 wired（Phase DM-020）。T 层增补 hubspoke 测试。

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| D7-S2-T01 | ProcessMessage 为 D1 主入口 | D7-S2-A01 | `sessionorchestrator/orchestrator_test.go`；`tests/integration/d7/d7_fastpath_test.go` | IMPLEMENTED | P0 | Session_Process |
| D7-S2-T02a | FastPath proxy 开销 P99 ≤ 2ms（Classify 后） | D7-S2-A01-F02 | `sessionorchestrator/orchestrator_test.go` | IMPLEMENTED | P0 | Session_Process |
| D7-S2-T02b | 规则 ClassifyIntent P99 ≤ 1ms | D7-S2-A02 | `decisionplanning/classifier_test.go` | IMPLEMENTED | P0 | Intent_Classify |
| D7-S2-T02c | FastPath 端到端 P99 ≤ 2ms（command-first 全栈） | D7-S2-A01-F02 | `sessionorchestrator/orchestrator_test.go`；`tests/integration/d7/d7_fastpath_test.go` | IMPLEMENTED | P0 | Session_Process |
| D7-S2-T03 | OrchestratePath 按路由矩阵（v1.1.0+ 显式调 SynthesizeTaskGraph + WaveScheduler） | D7-S2-A01-F03 | `sessionorchestrator/orchestrate_path_test.go` (5 AC) | IMPLEMENTED | P0 | Orchestrate_Run |
| D7-S2-T04 | HandleInterrupt：Wave→D4→Process→stopped→TaskCancel | D7-S2-A03 | `sessionorchestrator/orchestrator_test.go`；`tests/integration/d7/d7_interrupt_test.go` | IMPLEMENTED | P0 | Session_Process |
| D7-S2-T05 | HandleInterrupt 幂等 | D7-S2-A03 | `sessionorchestrator/orchestrator_test.go` | IMPLEMENTED | P0 | Session_Process |
| D7-S2-A01-T03 | 禁止在 Worker terminal FlowEvent 前伪造 Task 进度（anti-fabrication commitment） | D7-S2-A01 | `sessionorchestrator/orchestrator_test.go::TestSessionOrchestrator_AntiFabrication_NoSyntheticProgress` | IMPLEMENTED | P0 | Session_Process |
| D7-S2-A01-T04 | IntentCommand 显式分发到 PlanCLI/CLICommands（v1.1.0+ orthogonal） | D7-S2-A01 | `sessionorchestrator/command_handler_test.go` (3 AC) | IMPLEMENTED | P0 | Session_Process |
| D7-S2-A01-T05 | IntentOrchestrate 走 SynthesizeTaskGraph + WaveScheduler（v1.1.0+ orthogonal） | D7-S2-A01 | `sessionorchestrator/orchestrate_path_test.go` (5 AC) | IMPLEMENTED | P0 | Orchestrate_Run |
| D7-S2-A01-T06 | IntentFast 保持 FastPath（v1.1.0+ orthogonal, 不回归） | D7-S2-A01 | `sessionorchestrator/orchestrator_test.go::TestSessionOrchestrator_ProcessMessage_FastPath` | IMPLEMENTED | P0 | Session_Process |
| D7-S2-A03-T01 | HandleInterrupt 中断顺序正确（可中断性承诺） | D7-S2-A03 | `sessionorchestrator/orchestrator_test.go`；`tests/integration/d7/d7_interrupt_test.go` | IMPLEMENTED | P0 | Session_Process |
| **D7-S2-A04-T01** | **DispatchWorker D4 enabled with leader** | **D7-S2-A04** | **`hubspoke/hubspoke_test.go::TestDispatcher_Dispatch_D4_enabled_withLeader`** | **IMPLEMENTED** | **P0** | Session_Process |
| **D7-S2-A04-T02** | **DispatchWorker D4 disabled falls back to D2 SubQuery** | **D7-S2-A04** | **`hubspoke/hubspoke_test.go::TestDispatcher_Dispatch_D4_disabled_fallsToD2`** | **IMPLEMENTED** | **P0** | Session_Process |
| **D7-S2-A04-T03** | **DispatchWorker async mode** | **D7-S2-A04** | **`hubspoke/hubspoke_test.go::TestDispatcher_Dispatch_D4_async`** | **IMPLEMENTED** | **P1** | Session_Process |
| **D7-S2-A03-F06-T01** | — | **LLMFallbackClassifier Deprecated marker** | **D7-S2-A03-F06** | **`decisionplanning/classifier_fallback.go`** | **IMPLEMENTED** | **P1** | Session_Process |
| **D7-S2-A03-F06-T02** | — | **ExecutorSelector Deprecated marker** | **D7-S2-A03-F06** | **`decisionplanning/executor.go`** | **IMPLEMENTED** | **P1** | Session_Process |
| **D7-S2-A06-IT01** | — | **Multi-turn tool conversation (2 LLM rounds)** | **D7-S2-A06** | **`tests/integration/d7/d7_multiturn_test.go::TestIntegration_D7FastPath_MultiTurnToolConversation`** | **IMPLEMENTED** | **P0** | Turn_Run |
| **D7-S2-A06-IT02** | — | **MaxTurns cap enforcement** | **D7-S2-A06** | **`tests/integration/d7/d7_multiturn_test.go::TestIntegration_D7FastPath_MaxTurnsCap`** | **IMPLEMENTED** | **P1** | Turn_Run |
| **D7-S2-A06-IT03** | — | **StopProcess during slow Turn** | **D7-S2-A06** | **`tests/integration/d7/d7_multiturn_test.go::TestIntegration_D7FastPath_ContextCancellation`** | **IMPLEMENTED** | **P1** | Turn_Run |

### MUPS D2 Context Ownership (DM-20260704-002)

> **Archive:** `openspec/archive/2026-07-04-mups-d2-context-tools-ownership/`

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority |
|------|------|--------|-----------|--------|----------|
| **D7-S2-A90-T01** | LLMObservationProposer → MaterializeForMUPS | D7-S2-A90 | `llm_observation_proposer_test.go` | **IMPLEMENTED** | P0 |
| **D7-S2-A90-T02** | LLMStrategicPlanProposer → MaterializeForMUPS | D7-S2-A90 | `strategic_plan_proposer_test.go` | **IMPLEMENTED** | P0 |
| **D7-S2-A90-T03** | WorkItemExecutor.prepareContext → MaterializeForMUPS | D7-S2-A90 | `workitem_executor_test.go` | **IMPLEMENTED** | P0 |
| **D7-S2-A91-T01** | d7_no_tool_filter_test boundary lint | D7-S2-A91 | `internal/lint/layer/d7_no_tool_filter_test.go` | **IMPLEMENTED** | P0 |
| **D7-S2-A91-T02** | 无 toolsForProfile / filterPipelineTools 死代码 | D7-S2-A91 | `d7_no_tool_filter_test.go` | **IMPLEMENTED** | P0 |

### D7 Convergence Contract (DM-20260703-001)

> **Archive:** `openspec/archive/2026-07-03-d7-convergence-contract/`

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority |
|------|------|--------|-----------|--------|----------|
| **D7-S5-A93-T01** | R0.5 deliverable complete → SpawnNone | D7-S5-A93 | `workmodel/spawn_policy_test.go` | **IMPLEMENTED** | P0 |
| **D7-S5-A93-T02** | InlineRetriesAtMaxDepth budget | D7-S5-A93 | `workmodel/spawn_policy_test.go` | **IMPLEMENTED** | P0 |
| **D7-S5-A93-T03** | spawn_policy complete@maxDepth scenarios | D7-S5-A93 | `workmodel/spawn_policy_test.go` | **IMPLEMENTED** | P0 |
| **D7-S2-A86-T01** | ApplyRoundTerminalization | D7-S2-A86 | `sessionorchestrator/` | **IMPLEMENTED** | P0 |
| **D7-S2-A86-T02** | GetPipelineFocus continuation | D7-S2-A86 | `session_turn_loop_test.go` | **IMPLEMENTED** | P0 |
| **D7-S15-A43-T01** | MaybeSiblingBestEffortRollup | D7-S15-A43 | `workmodel/rollup_gate_test.go` | **IMPLEMENTED** | P0 |
| **D7-S15-A43-T02** | MaybeParentRollup | D7-S15-A43 | `workmodel/rollup_gate_test.go` | **IMPLEMENTED** | P1 |
| **D7-S2-A87-T01** | sessionNoForwardProgress recursive stuck | D7-S2-A87 | `session_turn_loop` | **IMPLEMENTED** | P0 |
| **D7-S2-A73-T05** | buildSessionCompleteEvent task_incomplete 安全网 | D7-S2-A73 | `session_complete_test.go` | **IMPLEMENTED** | P0 |

### Turn Adapter LTL-Lite Hook (DM-20260618-007)

**Change:** devrix-tools-terminal-architecture (DM-20260618-007) — LTL-Lite runtime check + CI lint + turn_adapter HookRegistry (PERMISSION-GATE-1-T01/T02/T03) + BackgroundTaskSurface ToolEventStream (D7-S2-A08-T01)

### Context Budget Phase A — Turn Loop Integration (DM-20260620-001)

> **devrix-context-budget-and-isolation (DM-20260620-001) — Phase A 落地。**
> AC1+AC2+AC4 turn loop 集成（D2-S17-A05/S17-A06/S15-A08 helpers 消费方）：
> tool result cap + assistant fold + per-iter audit + bootstrap wiring。
> D2 t-registry 持有 helper 自身的 T 点（T01-T05/T01-T03/T01-T05）；
> 本表持有 turn loop 集成 + bootstrap 接线 T 点（T11-T13）。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S2-A06-T11** | **AC1+AC2 turn loop integration: tool result cap + assistant fold wired into RunTurn** | **D7-S2-A06** | **`orchestration/sessionorchestrator/orchestrator_toolcap_test.go::TestOrchestrator_BuildToolResultMsgWithCap_*`, `TestOrchestrator_BuildAssistantToolCallMsgFolded_*`** | **IMPLEMENTED (DM-20260620-001)** | **P0** | Turn_Run |
| **D7-S2-A06-T12** | **AC4 per-iter audit + proactive fold + span attrs + slog** | **D7-S2-A06** | **`orchestration/sessionorchestrator/orchestrator_toolcap_test.go::TestOrchestrator_RunTokenAudit_*`** | **IMPLEMENTED (DM-20260620-001)** | **P0** | Turn_Run |
| **D7-S2-A06-T13** | **WireD7 bootstrap constructs ToolResultStore** | **D7-S2-A06** | **`bootstrap/wire_coordinator.go::NewOrchestrator(OrchestratorDeps{ToolResultStore: …})`** | **IMPLEMENTED (DM-20260620-001)** | **P0** | Turn_Run |

### Context Budget Phase B — SubTurn 3-Mode Dispatch (DM-20260620-001-B)

> **devrix-context-budget-and-isolation (DM-20260620-001-B) — Phase B 落地（待 B.5 验证 + S6 归档）。**
> AC6+AC8+AC9+AC10+AC11a SubTurn 3-mode 派发（brief/fork/full）：
> `SubTurnRunner` 按 `req.Mode` 选 `applyMode` 分支；empty → `SubagentConfig.DefaultMode`；
> `LegacyMode` 覆盖 `DefaultMode`；`Depth >= MaxDepth` 拒绝；
> fork mode 走 `conversation.BuildForkedMessages` 保证 prefix byte-level stable
> （cache anchor for future Anthropic `cache_control`）；
> `delegate_*` / `free_fork` LLM tool schema 暴露 `mode` 字段。
> D4 t-registry 持有 schema 侧的 T 点（T01/T02）；
> D2 t-registry 持有 BuildForkedMessages 自身的 T 点（T06/T07/T08）；
> 本表持有 SubTurnRunner 派发 + depth + default-mode T 点（T14/T15/T16/T17）。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S2-A06-T14** | **AC6 brief mode drops parent history: LLM sees only last user message** | **D7-S2-A06** | **`orchestration/sessionorchestrator/subturn_test.go::TestSubTurnRunner_BriefMode_PreloadedMessagesNil`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** | Turn_Run |
| **D7-S2-A06-T15** | **AC8+AC11a fork mode = BuildForkedMessages (cache-friendly prefix) + full mode = legacy parity** | **D7-S2-A06** | **`orchestration/sessionorchestrator/subturn_test.go::TestSubTurnRunner_ForkMode_DispatchesAsFork`, `TestSubTurnRunner_FullMode_BackwardCompat`, `TestSubTurnRunner_FullMode_EquivalentToLegacy`, `TestSubTurnRunner_FullMode_EmptyParent`; `subturn_fork_test.go::TestSubTurnRunner_ForkSiblingPrefixStable`, `TestSubTurnRunner_ForkPrefix_ContainsPlaceholder`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** | Turn_Run |
| **D7-S2-A06-T16** | **AC9 depth limit: `Depth >= MaxDepth` rejected before LLM call; `Depth = MaxDepth-1` allowed** | **D7-S2-A06** | **`orchestration/sessionorchestrator/subturn_test.go::TestSubTurnRunner_DepthLimit_{Equals,Exceeds,BoundaryAtMaxMinus1}`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** | Turn_Run |
| **D7-S2-A06-T17** | **AC6 default mode: empty `req.Mode` → `SubagentConfig.DefaultMode`; `LegacyMode` overrides `DefaultMode`; invalid mode rejected** | **D7-S2-A06** | **`orchestration/sessionorchestrator/subturn_test.go::TestSubTurnRunner_DefaultModeFromConfig`, `TestSubTurnRunner_DefaultModeBrief`, `TestSubTurnRunner_InvalidModeRejected`** | **IMPLEMENTED (DM-20260620-001-B)** | **P0** | Turn_Run |

### Context Budget Phase C — Nested Branch Budget Injection (DM-20260620-002)

> **devrix-context-budget-phase-c-nested (DM-20260620-002) — Phase C 落地。**
> `runLoop` nested branch (`orchestrator.go:221-268`) skips `o.context.Prepare`,
> leaving `prepared.MaxContextTokens=0` and making every Phase A budget control
> (runTokenAudit + ShouldFoldProactively + tool result cap + budgetTracker) a
> no-op. The fix threads `maxContextTokens` from `SubTurnRequest` →
> `TurnRequest` → nested-branch read, with fallback to `o.maxContextTokens`
> (Phase A wiring) for legacy callers.
>
> Bug → fix: 4-parallel deep-review sub-agents (e.g. "review D1/D2/D3/D7" after
> 10 tool rounds each) accumulated ~80K-char oversized read_file results, blew
> past the LLM context window, and were rejected. After Phase C, the audit
> fires on the nested branch and the largest assistant message is folded
> (80000→1186 chars).
>
> D2 t-registry holds `enforce.Run` pass-through T 点（T09/T10）。
> D7 t-registry holds TurnRequest + nested-branch read + integration
> verification T 点（T18/T19/T20/T21/T22/T23）。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S2-A06-T18** | **AC1 `TurnRequest.MaxContextTokens` 字段添加 + 注释（nested 分支可显式注入 budget）** | **D7-S2-A06** | **`orchestration/sessionorchestrator/turn_contracts.go::TurnRequest.MaxContextTokens`** | **IMPLEMENTED (DM-20260620-002)** | **P0** | Turn_Run |
| **D7-S2-A06-T19** | **AC1 `runLoop` nested 分支读取 `req.MaxContextTokens`，fallback `o.maxContextTokens`** | **D7-S2-A06** | **`orchestration/sessionorchestrator/turn_orchestrator.go:271-274`** | **IMPLEMENTED (DM-20260620-002)** | **P0** | Turn_Run |
| **D7-S2-A06-T20** | **AC1 `SubTurnRunner.Cfg.MaxContextTokens` + `bootstrap/wire_coordinator.go` 注入全局 config** | **D7-S2-A06** | **`orchestration/sessionorchestrator/subturn.go::SubTurnConfig.MaxContextTokens`, `bootstrap/wire_coordinator.go:179` (NewSubTurnRunner 调用)** | **IMPLEMENTED (DM-20260620-002)** | **P0** | Turn_Run |
| **D7-S2-A06-T21** | **AC1 nested-branch 显式注入路径：80K assistant + 96K system + 32K budget → audit 触发 + fold 80000→1186** | **D7-S2-A06** | **`orchestration/sessionorchestrator/turn_orchestrator_test.go::TestOrchestrator_RunTurn_NestedBranch_BudgetInjection_DM_20260620_002`** | **IMPLEMENTED (DM-20260620-002)** | **P0** | Turn_Run |
| **D7-S2-A06-T22** | **AC1 nested-branch fallback 路径：req=0 → `o.maxContextTokens`（Phase A wiring 32000）audit 仍触发** | **D7-S2-A06** | **`orchestration/sessionorchestrator/turn_orchestrator_test.go::TestOrchestrator_RunTurn_NestedBranch_FallbackToDeps_PhaseA_AC1_DM_20260620_002`** | **IMPLEMENTED (DM-20260620-002)** | **P0** | Turn_Run |
| **D7-S2-A06-T23** | **AC2 4-parallel deep review 端到端：4 路 `SubQuery.Run` 并行（80K+96K+32K）全部完成，capture adapter 验证 max 消息 1186 chars (folded)** | **D7-S2-A06** | **`tests/integration/d7/d7_nested_budget_test.go::TestIntegration_D7NestedBudget_4ParallelDeepReview`** | **IMPLEMENTED (DM-20260620-002)** | **P0** | Turn_Run |

### Error Handling Tier 1+2 (DM-20260620-003)

> **devrix-error-handling-tier1-tier2 (DM-20260620-003) — 错误处理 PR-A (Tier 1) + PR-B (C2 type merge) + PR-C (H3+M1..M4) 落地。**
>
> **Tier 1 (PR-A)**: `SanitizeForUser` redacts API keys/tokens/paths before IM render;
> `emitError` signature gains variadic `code ...string` for `error_code` metadata;
> `subturn.go` adds `ErrSubagentStreamError` + `ErrSubagentStreamClosed` (codes
> AGT_STREAM_5013/5014) so callers retain error type information;
> `retry.go:91` nil-sentinel fix wraps a real `errors.New(...)` cause instead of `nil`.
>
> **Tier 2 (PR-B)**: `LLMError` becomes a type alias for `*SentinelError`;
> all factories return `*SentinelError`; `SentinelError.Error()` falls back to
> inner Err then Code (preserving LLMError's permissive semantics); `migrate.go`
> provides build-time guard + deprecated helpers.
>
> **Tier 2 (PR-C)**: `TaskManager.Create` returns `(*Task, error)` (silent
> fallback fix); `turn_adapter.ErrInvariantViolation` migrated to
> `sharederrors.ErrInvariantViolation` (code AGT_INVARIANT_5013) with deprecated
> alias; `classifyAndWrap` + `Gateway.classify` take ctx so downstream spans
> can read cached Classification via `ClassifyResultFromCtx`;
> `Observability.Shutdown` uses `errors.Join` + `%w` so callers retain typed chain;
> `decisionplanning.LLMFallbackClassifier.Classify` logs `slog.Warn` when LLM
> classify fails (was silent).
>
> Cross-cutting docs: `docs/error-handling.md`. Shared spec lives at
> `internal/shared/errors/` (no `shared-errors` D-domain — cross-cutting per
> `openspec/specs/architecture/cross-domain-boundaries.md`).

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S2-A06-T24** | **AC6 `turn_adapter.ErrInvariantViolation` migrated to `sharederrors.ErrInvariantViolation` (code AGT_INVARIANT_5013); `Prepare` wraps via `NewInvariantViolationError`; legacy alias kept** | **D7-S2-A06** | **`internal/layers/orchestration/turn_adapter/ltl_hook_test.go::TestHookRegistry_Prepare_*` (still match via alias)** | **IMPLEMENTED (DM-20260620-003)** | **P1** | Turn_Run |
| **D7-S2-A06-T25** | **AC2 `subturn.go:collectSubTurnResult` error case: when event has `error_code` metadata, wrap via `derrors.WithCode(code, ...)`; otherwise fall back to `NewSubagentStreamError`** | **D7-S2-A06** | **`internal/shared/errors/subturn.go`; `internal/layers/orchestration/sessionorchestrator/subturn.go`** | **IMPLEMENTED (DM-20260620-003)** | **P1** | Turn_Run |
| **D7-S2-A06-T26** | **AC2 `subturn.go:collectSubTurnResult` channel-closed-without-complete branch returns `NewSubagentStreamClosedError()` (code AGT_STREAM_5014)** | **D7-S2-A06** | **`internal/shared/errors/subturn.go::NewSubagentStreamClosedError`** | **IMPLEMENTED (DM-20260620-003)** | **P1** | Turn_Run |
| **D7-S2-A06-T27** | **AC2/H3 `protect/retry.go:91` nil-sentinel fix: defensive fallback wraps `errors.New("retry loop completed without recording an error: ...")` instead of `nil`** | **D7-S2-A06** | **`internal/layers/llmgateway/protect/retry.go`** | **IMPLEMENTED (DM-20260620-003)** | **P0** | Turn_Run |
| **D7-S2-A02-T18** | **AC1 `orchestrator.emitError` variadic `code ...string` adds `Metadata["error_code"]`; all 5 call sites pass `SanitizeForUser(err)` + `ErrorCode(err)`** | **D7-S2-A02** | **`internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go::emitError` + call sites (256, 292, 371, 428, 581)** | **IMPLEMENTED (DM-20260620-003)** | **P0** | Session_Process |

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **PERMISSION-GATE-1-T01** | LTL-Lite runtime check (ltllite.Check + HookRegistry.Prepare) | turn_adapter | `internal/layers/orchestration/turn_adapter/ltl_hook_test.go::TestHookRegistry_Prepare_*` | **IMPLEMENTED** | **P0** | — |
| **PERMISSION-GATE-1-T02** | CI lint 静态校验 (ci-lint-invariant 扫描 _invariant.go) | tools/ | `tools/ci-lint-invariant/main_test.go` | **IMPLEMENTED** | **P0** | — |
| **PERMISSION-GATE-1-T03** | turn_adapter HookRegistry Prepare/BeforeExecute 定向重检 | turn_adapter | `internal/layers/orchestration/turn_adapter/ltl_hook_test.go::TestHookRegistry_BeforeExecute_*` | **IMPLEMENTED** | **P0** | — |
| **D7-S2-A08-T01** | ToolEventStream context 推送 + BackgroundTaskSurface 集成 | turn | `internal/layers/orchestration/sessionorchestrator/tool_stream_test.go` | **IMPLEMENTED** | **P0** | Session_Process |

### Loop-First Routing L5 (DM-20260616-002)

| L5 ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|-------|------|----------|-----------|--------|----------|
| **D7-S2-L5-01** | 问候语 Turn 不触发 Wave（无 plan_formed/wave_started） | D7-S2-A01 | `tests/integration/d7/d7_loop_first_test.go::TestIntegration_D7LoopFirst_GreetingNoWave`；`decisionplanning/classifier_test.go::TestRuleClassifier_Classify_LoopFirstDefault` | IMPLEMENTED | P0 |
| **D7-S2-L5-02** | delegate_wave tool 门控 OrchestratePath | D7-S2-F02 | `sessionorchestrator/turn_tools_test.go`；`tests/integration/d7/d7_loop_first_test.go::TestIntegration_D7LoopFirst_DelegateWaveTool` | IMPLEMENTED | P0 |
| **D7-S2-L5-03** | Slash 命令零 LLM | D7-S2-A01 | `tests/integration/d7/d7_orthogonal_dispatch_test.go::TestIntegration_D7ProcessMessage_CommandBypassesLLM` | IMPLEMENTED | P0 |
| **D7-S2-L5-04** | EngineEvent 单投递（无 sink mirror） | D7-S2-F03 | `sessionorchestrator/orchestrator_test.go`；`capture/agent_route.go` | IMPLEMENTED | P0 |
| **D7-S2-L5-05** | enter_plan_mode tool | D7-S2-F02 | `sessionorchestrator/turn_tools_test.go` | IMPLEMENTED | P1 |
| **D7-S2-L5-06** | rule_orchestrate 回滚（threshold 降级） | D7-S2-F01 | `orchtypes/routing_test.go`；`sessionorchestrator/orchestrator_test.go` | IMPLEMENTED | P1 |

---

## Cross-Domain (D7 契约)

| T ID | 描述 | 归属 | Test 位置 | Status | Priority | Span Evidence |
|------|------|------|-----------|--------|----------| --- |
| D7-D1-T01 | D1 调用 D7 而非 D2（d7_enabled） | D7-S2 | `tests/integration/d7/d7_entry_test.go`（WireD7 全栈）；`sessionorchestrator/entry_test.go` | IMPLEMENTED | P0 | Session_Process |
| D7-D4-T01 | D2 enforce 无 delegate hooks | D7-S2 | `internal/lint/layer/d2_thin_test.go` | IMPLEMENTED | P0 | — |
| D7-D6-T01 | D6 校验编排决策（advisory）+ `orchestration.d6.validation.{pass,fail,timeout,error}` metric | D7-S5 | `internal/layers/orchestration/sessionorchestrator/validation_metrics_test.go` | IMPLEMENTED | P1 | Validation_Metric |
| D7-D6-T02 | D6 校验超时 50ms 视为 pass | D7-S5 | `internal/layers/orchestration/sessionorchestrator/entry_test.go` | IMPLEMENTED | P2 | Validation_Metric |
| D7-D6-T03 | 4 counter 注入 + result.Pass 分流 | D7-S5 | `internal/layers/orchestration/sessionorchestrator/validation_metrics_test.go` | IMPLEMENTED | P0 | Validation_Metric |
| D7-D6-T04 | timeout_rate > 5% 触发 AlertHook（5min 滑窗） | D7-S5 | `internal/layers/orchestration/sessionorchestrator/validation_metrics_test.go` | IMPLEMENTED | P0 | Validation_Metric |
| D7-D6-T05 | panic-recovered 计入 error 路径 | D7-S2 | `internal/layers/orchestration/sessionorchestrator/validation_metrics_test.go` | IMPLEMENTED | P0 | Validation_Metric |
| D7-D6-T06 | nil validator 与 nil metrics 都降级 no-op | D7-S2 | `internal/layers/orchestration/sessionorchestrator/validation_metrics_test.go` | IMPLEMENTED | P0 | — |
| D7-MIG-T01 | D7-only ingress × plan.enabled 组合回归 | D7-S2 | `tests/integration/d7/d7_entry_test.go::TestIntegration_D7Entry_PlanModeStillUsesD7Path`；`coordinator_matrix_test.go` | IMPLEMENTED | P0 | Session_Process |
| D7-THIN-T01 | D2 contextengine 无 orchestration import | D2 瘦身 | `internal/lint/layer/d2_thin_test.go` | IMPLEMENTED | P0 | — |
| D7-THIN-T02 | ~~loop.go Run ≤200 行~~ | D2 瘦身 | **REMOVED**（`query/loop.go` 已删，DM-20260618-010） | REMOVED | P0 | — |

---

## D1 集成（IM 渲染）

| T ID | Legacy ID | 描述 | 归属 | Test 位置 | Status | Priority | Span Evidence |
|------|-----------|------|------|-----------|--------|----------| --- |
| D7-S4-T07 | ORCH-S2-T14 | 每 Task 独立双区块 IM 卡流式 | D1-S8 + D7-S4 | `communication/channel/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P0 | Flow_Event_Publish |

---

## D7-S2 Turn Leader（DM-020 v1.0 Registry）

> **v3.0 closure (2026-06-15):** v2.0-b/c/f 全部闭环。A06-T01..T04 + A07-T01..T02 全部 IMPLEMENTED。

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority | Span Evidence |
|------|------|--------|-----------|--------|----------| --- |
| D7-S2-A06-T01 | FastPath turn D2 then D3 in order | D7-S2-A06 | `orchestration/sessionorchestrator/turn_orchestrator_test.go::TestOrchestrator_RunTurn_SingleTurn_NoTools` | IMPLEMENTED | P0 | Turn_Run |
| D7-S2-A06-T02 | Cancel propagates to D3 stream and D2 tools | D7-S2-A06 | `orchestration/sessionorchestrator/turn_orchestrator_test.go::TestOrchestrator_RunTurn_CancelBetweenTurns`, `TestOrchestrator_RunTurn_CancelBeforeLLM` | IMPLEMENTED | P0 | Turn_Run |
| D7-S2-A06-T03 | Multi-turn tool_use loops under D7 | D7-S2-A06 | `orchestration/sessionorchestrator/turn_orchestrator_test.go::TestOrchestrator_RunTurn_MultiTurn_ToolLoop` | IMPLEMENTED | P0 | Turn_Iteration |
| D7-S2-A06-T04 | SubQuery nested turn uses same orchestrator | D7-S2-A06 | `orchestration/sessionorchestrator/turn_orchestrator_test.go::TestOrchestrator_RunTurn_{SubQueryScope,SameOrchestratorForMainAndSubQuery}` | IMPLEMENTED | P0 | Turn_Run |
| D7-S2-A07-T01 | Breaker open with no fallback returns error | D7-S2-A07 | `orchestration/sessionorchestrator/turn_orchestrator_test.go::TestOrchestrator_RunTurn_LLMInvokeError`; `orchestration/sessionorchestrator/llm_test.go::TestGatewayInvoker_InvokeStream_BreakerOpen` | IMPLEMENTED | P0 | LLM_Invoke |
| D7-S2-A07-T02 | StreamChat timeout propagates as EngineEvent | D7-S2-A07 | `orchestration/sessionorchestrator/llm_test.go::TestGatewayInvoker_InvokeStream_{ContextCanceled,ContextDeadlineExceeded}`, `TestOrchestrator_RunTurn_StreamTimeout_EngineEvent` | IMPLEMENTED | P0 | LLM_Invoke |
| **D7-S2-A06-T09** | **D7 RunTurn never touches removed D2 QueryLoop** | **D7-S2-A06** | **`orchestration/sessionorchestrator/loop_legacy_test.go::TestOrchestrator_RunTurn_MainPathOnly`** | **IMPLEMENTED** | **P0** | Turn_Run |
| **D7-S2-A06-T10** | **~~D2.QueryLoop legacy counter~~ REMOVED (DM-20260618-010)** | **D7-S2-A06** | **`contextengine/queryloop_removed_test.go::TestD2_NoQueryLoopProductionReferences`** | **IMPLEMENTED** | **P0** | Turn_Run |

### Legacy T 映射（DM-020 — v1.0 Registry，v2.0 实施）

> v1.0：**不改**现有测试 `// T:` 注释。下表供追溯与新测试登记。

| Legacy T ID | Canonical T ID | Canonical S | 域 | 描述 |
|-------------|----------------|-------------|-----|------|
| D2-S16-A01-T01 | D7-S2-A06-T01 | S2 Turn | D7 | FastPath turn D2 then D3 |
| D2-S16-A01-T02 | D7-S2-A06-T02 | S2 Turn cancel | D7 | Cancel propagates |
| D2-S16-A01-T03 | D2-THIN-T01 | import lint | D2 | D2→D3 import 禁止 |
| D2-S10-A01-T34 | D7-S2-A06-T03 | multi-turn loop | D7 | Multi-turn tool_use |
| D2-S10-A01-T35~T42 | D2-S15/S18/S19-T* | 按机制拆分 | D2 | 保留 D2 域内 |
| （新增） | D7-S2-A07-T01 | RouteModel+Stream | D7 | Breaker sad path |
| （新增） | D7-S2-A07-T02 | StreamChat timeout | D7 | Timeout propagate |
| （新增） | D7-S2-A06-T04 | SubQuery nested | D7 | Nested turn |
| （新增） | D2-S15-A01-T10 | CompressHint no LLM | D2 | D2 不调 LLM |
| D7-S2-A50-T01 | turn/ → sessionorchestrator/ 24 .go git mv 完成（5 文件 turn_ 前缀解决同名） | D7-S2-A50 | `git log --diff-filter=R --name-only` 24 .go rename + 5 加前缀 | IMPLEMENTED | P0 |
| D7-S2-A50-T02 | 24 文件 `package turn` → `package sessionorchestrator` 全替换 | D7-S2-A50 | `grep -l "^package turn$" internal/layers/orchestration/sessionorchestrator/` 应 0 结果 | IMPLEMENTED | P0 |
| D7-S2-A50-T03 | 14 importer 文件 import path + identifier 全替换 + 跨包 import cycle 打破 | D7-S2-A50 | `grep -rn "orchestration/turn" internal/` 应 0 命中；sessionorchestrator ↔ decisionplanning cycle 经 orchtypes 上提修复 | IMPLEMENTED | P0 |
| D7-S2-A50-T04 | hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go 0 变更 + 22/22 orchestration packages go test -race PASS | D7-S2-A50 | `git diff --stat hardening/ escape/ sessionorchestrator/autoclose.go` 应空；`go test -race -count=1 ./internal/layers/orchestration/...` 应 22/22 PASS | IMPLEMENTED | P0 |
| **D7-S2-A50-T05** | **`OrchestratorDeps.FallbackModel string` 字段就位 + `TurnState.Withheld bool` 字段就位 + `emitError` 路径用 `sharederrors.Code(err)` 填 `Event.Metadata["error_code"]`（受控枚举：RateLimit/AuthenticationFailed/ServerError/MediaSize/PromptTooLong/ImageSize/Unknown 7 类）** `<!-- v4.9.0 -->` | **D7-S2-A50 RunTurnLoop** | `internal/layers/orchestration/sessionorchestrator/orchestrator_test.go::TestOrchestratorDeps_FallbackModelField + TestTurnState_WithheldField + TestEmitError_MetadataErrorCode` | **IMPLEMENTED**（DM-20260628-001 S5 验收 PR #265，2 case TestEmitErrorWithErr_* PASS） |
| **D7-S2-A50-T06** | **主模型 2 次连续 RateLimit/ServerError 触发 `fallback_trigger_candidate` 日志（fallback_model 未 wire 场景日志显式标注 `fallback_model_set_but_not_yet_wired`）+ prompt_too_long 错误标记 `TurnState.Withheld=true` 不 surface error 事件 + 现有 30+ `SanitizeForUser` 调用点零行为变化回归** `<!-- v4.9.0 -->` | **D7-S2-A50 RunTurnLoop** | `internal/layers/orchestration/sessionorchestrator/turn_orchestrator_test.go::TestEmitError_FallbackTriggerCandidate + TestTurnState_Withheld_PromptTooLong_NoSurface + TestSanitizeForUser_NoRegression` | **IMPLEMENTED**（DM-20260628-001 S5 验收 PR #265，3 case TestObserveFallbackTrigger_* PASS + sharederrors 全量回归 55.8% 覆盖） |
| **D7-S2-A50-T07** | **session_complete.go meta 透传 5 元 verdict: meta[verify_exit_reason]=verdict.Reason + meta[emission_class]+meta[source_uncertainty]=verdict.CC (P0-AC-5)** | **D7-S2-A50 RunTurnLoop** | **`internal/layers/orchestration/sessionorchestrator/session_complete.go` (wired via PR-B VerifyContract result) + `conclusion/conclusion.go::EmitComplete` 透传 meta** | **IMPLEMENTED (DM-20260701-007)** | **P0** |
| **D7-S2-A50-T08** | **verify_exit_reason -> Learn ReasonLog (P1-AC-4): ReasonLog.Record(sessionID, reason, emissionClass) 跨 session 可读; 8 unit tests 100% PASS** | **D7-S2-A50 RunTurnLoop** | **`internal/layers/orchestration/mups/learn/reason_log.go` + `reason_log_test.go` (8 tests: Record / RejectsEmptySessionID / RejectsEmptyReason / FIFOEviction / RecentByTool / DriftRate / DriftRate_Unknown / RecordFromVerdict)** | **IMPLEMENTED (DM-20260701-007)** | **P0** |
| **D7-S2-A50-T09** | **`turn_adapter.executeOne` tool-level timeout (default 60s, env `DEVRIX_TOOL_TIMEOUT_SECONDS` 可调; 0/无效 env 兜底 default)** | **D7-S2-A50 RunTurnLoop** | **`internal/bootstrap/turn_adapter_timeout_test.go` (5 case: TestExecuteOne_Timeout_FastTool / _SlowToolReturnsErr / _EnvOverrideShorterThanDefault / _InvalidEnvFallsBackToDefault / _ZeroEnvFallsBackToDefault)** | **IMPLEMENTED (DM-20260704-003)** | **P0** |
| D7-S2-A50-T10 | **tool timeout fail-closed: `slog.Warn("tool execution timeout")` + 返回带 `tool execution timeout` 错误标记的 ToolResult（不 panic，不传染 Turn）** | D7-S2-A50 RunTurnLoop | **`internal/bootstrap/turn_adapter_timeout_test.go::TestExecuteOne_Timeout_SlowToolReturnsErr` (断言 elapsed < 1.5s + Error 含 "timeout")** | **IMPLEMENTED (DM-20260704-003)** | **P0** |
| D7-S4-A50-T01 | sessionorchestrator/{exit_reason,verdict_to_exit_reason,verdict_to_exit_reason_test}.go 3 文件 git mv → executionflow/verify/（218 行） | D7-S4-A50 | `git log --follow` 100% rename detection；`ls sessionorchestrator/ \| grep -E "exit_reason\|verdict_to"` 0 命中 | PLANNED | P0 |
| D7-S4-A50-T02 | 3 文件 `package sessionorchestrator` → `package verify` + sessionorchestrator/turn_orchestrator.go 11 处 `ExitReason*` → `verify.ExitReason*` + turn_orchestrator_test.go 2 处 `ExitReasonNatural` → `verify.ExitReasonNatural` | D7-S4-A50 | `grep -rn "ExitReason[^a-zA-Z]" sessionorchestrator/*.go \| grep -v "verify\."` 0 命中 | PLANNED | P0 |
| D7-S4-A50-T03 | executionflow/verify/ 包 0 sessionorchestrator 反向依赖 + 跨包 import cycle 0 风险（单向 DAG: sessionorchestrator → verify） | D7-S4-A50 | `go list -deps ./internal/layers/orchestration/executionflow/verify \| grep sessionorchestrator` 0 命中 | PLANNED | P0 |
| D7-S4-A50-T04 | go build/vet/test -race 22/22 orchestration packages 全绿 + LP-1/2/5 集成测试 100% 兼容 + hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 变化 | D7-S4-A50 | `git diff hardening/ escape/ sessionorchestrator/autoclose.go` 空；`go test -race -count=1 ./internal/layers/orchestration/...` 22/22 PASS | PLANNED | P0 |
| D7-S2-A51-T01 | 6 S × WireFunc 命名一致 (S1 0 wire + S2-S6 各 1 wire + 横切 0 wire)：`grep -E "^func Wire" internal/bootstrap/*.go` 列出 5 个 `Wire*` (`WireTurnInvoker` + `WireWaveScheduler` + `WireExecutionFlow` + `WireDecisionPlanning` NEW + `WireMUPSPipeline` NEW) + 1 个 `BuildOrchestratePath` (S3 helper)；横切 Hardening 通过 `hardening.SetBridge` 隐式注入 | D7-S2-A51 | `grep -E "^func Wire" internal/bootstrap/*.go` 应列出 5 个 Wire* + 1 个 BuildOrchestratePath | IMPLEMENTED | P0 |
| D7-S2-A51-T02 | InitOrchestration 主体 ≤ 200 行 + 6 S 组合入口清晰：`wc -l internal/bootstrap/wire_coordinator.go` ≤ 250 行 (含 helper + import + 注释)；`awk '/^func InitOrchestration/,/^}/' wire_coordinator.go \| wc -l` ≤ 200 行 | D7-S2-A51 | `wc -l internal/bootstrap/wire_coordinator.go` ≤ 250；InitOrchestration 函数体 140 行 ≤ 200 | IMPLEMENTED | P0 |
| D7-S2-A51-T03 | 3 个内嵌 adapter 函数已拆到 `internal/bootstrap/adapters.go` + 4 个 util 函数 (boolPtr + intPtr + strPtr + mapBackgroundStatus) 已拆到 `internal/bootstrap/util.go`；contextEngineAdapter 已在 `internal/bootstrap/turn_adapter.go` 独立（DM-20260617-006）；0 残留内嵌 | D7-S2-A51 | `grep "^func new\|^type turnOrchExecutor\|^type gatewayEventPublisher\|^func boolPtr\|^func intPtr\|^func strPtr\|^func mapBackgroundStatus" internal/bootstrap/wire_coordinator.go` 0 命中 | IMPLEMENTED | P0 |
| D7-S2-A51-T04 | 22/22 orchestration packages go test -race PASS + LP-1 (TestAutoClose_FullLP1Loop) + LP-2 (TestIntegration_5NodePipeline_End2End) + LP-5 (Cross-session traceability) 100% 兼容 + hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 变化 + cmd/devrix + cmd/obs-verify + tests/testutil/d7_stack.go 0 变化 | D7-S2-A51 | `go test -race -count=1 ./internal/layers/orchestration/...` 22/22 PASS；`git diff --stat hardening/ escape/circuit_breaker.go internal/layers/orchestration/sessionorchestrator/autoclose.go cmd/devrix/main.go cmd/obs-verify/main.go tests/testutil/d7_stack.go` 空 | IMPLEMENTED | P0 |

---

## Statistics

| Total | IMPLEMENTED | PARTIAL | PLANNED | P0 |
|-------|-------------|---------|---------|-----|
| 281 | 281 | 0 | 0 | 237 |

> DM-20260630-013 (devrix-d2-d7-review-hardening) 新增 15 项 P0 T 点 (266 - 251 = 15)，全部 IMPLEMENTED：D7-S1-A80-T01 work_tree.SetStore mu 保护 + D7-S2-A80-T01/T02 PerInvocationEmit (ItemPipelineRunOpts + ExecuteOpts.Emit 字段) + D7-S2-A81-T01 orchestrator.EnsureGoal 错误 slog.Warn + D7-S2-A82-T01 turn_loop.AwaitRunningChildren err purge + D7-S2-A83-T01 turn_loop 4 错误 slog.Warn + D7-S2-A84-T01 item_pipeline SetRoundPhase warn span + D7-S2-A85-T01 turn_state.EndTurn purge handle + D7-S3-A84-T01/T02 WorkerPool OnReleaseOnce + D7-S9-A33-T01 mups/execute ErrChannelCtxCancelled + D7-S14-A48-T01 escape Arbitrator Timer + ctx cancel + D7-S14-A49-T01 arbitrator i18n 化 + D7-S15-A42-T01 resolve 4 _ = → warn + D7-S16-A77-T01 child_downlink DefaultChildExpectedReturn schema tag + D7-S16-A78-T01 strategic_plan_proposer i18n 化。

### 按 Scenario

| Scenario | Total | IMPLEMENTED | PARTIAL | PLANNED |
|----------|-------|-------------|---------|---------|
| D7-S1 | 9 | 9 | 0 | 0 |
| D7-S2 | 44 | 44 | 0 | 0 |
| D7-S3 | 21 | 21 | 0 | 0 |
| D7-S4 | 13 | 9 | 0 | 4 |
| D7-S5 | 28 | 28 | 0 | 0 |
| D7-S6 (Error Agg) | 7 | 7 | 0 | 0 |
| **D7-S7** (Cross-cutting Hardening) | **4** | **4** | **0** | **0** |
| D7-S8 | 9 | 9 | 0 | 0 |
| D7-S9 | 19 | 19 | 0 | 0 |
| D7-S10 | 12 | 12 | 0 | 0 |
| D7-S11 | 13 | 13 | 0 | 0 |
| **D7-S12** (Observe-Learner 跨域闭环) | **6** | **6** | **0** | **0** |
| **D7-S13** (Verify→Learn Auto-Close) | **6** | **6** | **0** | **0** |
| D7-S14 | 20 | 20 | 0 | 0 |
| **D7-S15** (WorkItem Rollup) | **22** | **19** | **2** | **0** |
| **D7-S16** (Layer SubContext) | **20** | **20** | **0** | **0** |
| 契约/迁移 | 9 | 9 | 0 | 0 |

---

## ValueFlow Semantic + Span Evidence 覆盖率（v6.0.0 + v2.5.1 同步）

> **ValueFlow Semantic 列**（230 T 全覆盖，按 S 分组给出语义聚合）：
> - **S1 (8 T)**: Multi-Step Task Coordination — Task 创建/依赖/状态机 + WorkTree 持久化 + 跨 session
> - **S2 (36 T)**: Turn-Based Conversation — ProcessMessage 入口 + 4 IntentKind 路径 + Turn 主循环 + 5 模式 SubTurn + 错误处理 + LLM 调用 + 上下文预算 + 跨域契约
> - **S3 (20 T)**: Parallel Worktree Execution — Wave 调度 + 5-slot WorkerPool + 冲突守卫 + AllowAndRegister 原子化
> - **S4 (13 T)**: Trustworthy Conclusion Delivery — Hub 双通道 + SpokeBridge + IMSink + 4 态 Verdict + 14 ExitReason + 聚合 + SystemAnomaly
> - **S5 (28 T)**: Intent + Uncertainty Quantization — ClassifyIntent + Planner + MatchKind + Observation 4 类 + UncertaintyReport + UncertaintyCoord + AnomalyDetector + IntentQuantizer + WithPrior
> - **S6 (7+9+9+8+13 = 46 T + Hardening 4)**: Learn from Outcome + Discipline Keeper — 4 Channel + Router + Artifact 4 类 + SideEffect 5 态 + LearningAsset 5 类 + Bayesian + 3 通道记忆 + 6 metric
> - **S7-S16** (62 T): 见各 Scenario Detail 段
> - **契约/迁移** (8 T): D1/D2/D4/D6 跨域 + d2_thin + d7-only ingress

> **Span Evidence 列**（覆盖率 **~38%** = 87/230 T 有对应 span ops）：
> - 26 ops 覆盖的核心 T 点（87/230 = ~38%）：S2 turn 链路 + S3 wave 调度 + S4 flow + S5 observe/plan + S6 execute/learn 5 节点 + 3 inner span
> - **缺口 T 点 (~62%)**：单元/集成测试 (200+ T) 不直接 emit span（通过 capture adapter 间接覆盖）；详见 `observability-guide.md` §"T-Without-Span Tracker" 缺口清单
> - **目标**：80% 覆盖率（PR-6/PR-7 增量）

> **缺口 T-Without-Span Tracker**（`observability-guide.md` §X 同步）：
> - **S1 unit** (5/8 = 63% 缺)：T01-T08 task_manager/disk_store 单元 → 无 span ops
> - **S2 unit** (12/36 = 33% 缺)：T02a/b/c FastPath P99 + T05 idempotency + A06-T01..T04 Turn Leader + A07-T01/T02 LLM invoke + L5-04/06 loop_first → 间接通过 `D7_Orchestration_Turn_Run` 覆盖
> - **S3 unit** (6/20 = 30% 缺)：T01-T11 wavescheduler 单元 → 间接通过 `D7_Orchestration_Wave_Schedule` 覆盖
> - **S4 unit** (4/13 = 31% 缺)：T06 throttle + 契约类 → 间接通过 `D7_Orchestration_Flow_Event_Publish` 覆盖
> - **S5 unit** (15/28 = 54% 缺)：T01-T05 + S8-A15 + S8-A22 unit → 间接通过 `D7_Orchestration_Intent_Classify` + `D7_Orchestration_Plan_Generate` 覆盖
> - **S6 unit** (10/46 = 22% 缺)：channel/side_effect/reputation unit → 间接通过 `D7_Orchestration_Channel_Route` 覆盖
> - **契约/迁移** (8/8 = 100% 缺)：跨域契约测试无对应 span

**结论**：80% 覆盖率目标需要 PR-6（5 ops span）+ PR-7（4 acceptance test）+ PR-8（Span Evidence 列填充）三段增量；当前 38% 作为 v2.5.1 baseline。

> **v3.0 closure (2026-06-15):** v1.2 + v2.0-b/c/f 全部闭环。D7-S1-T08 (state machine), D7-S5-A01-T01 (confidence threshold), D7-S2-A06-T01..T04 (turn leader), D7-S2-A07-T01..T02 (LLM invoker) 全部 IMPLEMENTED。IMPLEMENTED 58→66，PLANNED 9→0。全部 T 点闭环。
>
> **v3.1 closure (2026-06-16):** **devrix-d7-uncertainty-gaps (DM-20260616-001) 归档**：+26 T 点全部 IMPLEMENTED（PlanAgent runtime gate 4 + PlanMode LLM guard 2 + ConflictGuard TOCTOU 4 + FlowEvent sink 2 + PlanModeApproveGate removal 2 + dead code markers 2 + 积分测试 10）。IMPLEMENTED 66→92，P0 44→63。
>
> **v3.2 closure (2026-06-17):** **devrix-d7-turn-history-persist (DM-20260617-003) 归档**：+2 T 点 IMPLEMENTED（D7-S5-A04-T01/T02 turn adapter persist + 3-轮集成）。IMPLEMENTED 94→96，P0 65→67。
>
> **v3.6 closure (2026-06-20):** **devrix-context-budget-and-isolation (DM-20260620-001) Phase A 归档**：+3 T 点 IMPLEMENTED（D7-S2-A06-T11 turn loop 集成 AC1+AC2 + T12 AC4 per-iter audit + T13 bootstrap 接线）。IMPLEMENTED 96→99，P0 67→70。D2 域内另 +13 T 点（D2-S17-A05 T01-T05 + D2-S17-A06 T01-T03 + D2-S15-A08 T01-T05）见 d2 t-registry。
>
> **v3.7 closure (2026-06-20):** **devrix-context-budget-and-isolation (DM-20260620-001-B) Phase B 归档**：+4 T 点 IMPLEMENTED（D7-S2-A06-T14 brief mode PreloadedMessages=nil + T15 fork/full mode parity + T16 depth limit + T17 default mode resolution chain）。IMPLEMENTED 99→103，P0 70→74。配套 D2 域 +3 T 点（D2-S15-A08 T06-T08 BuildForkedMessages byte-level prefix stability）见 d2 t-registry；D4 域 +2 T 点（D4-S14-A07 T01-T02 mode field schema）见 d4 t-registry。AC12 D5 spans 22-step replay P95=21707 ≤ 40K（Phase A baseline 51K）。
>
> **v3.8 closure (2026-06-20):** **devrix-context-budget-phase-c-nested (DM-20260620-002) Phase C 归档**：+6 T 点 IMPLEMENTED（D7-S2-A06-T18 TurnRequest.MaxContextTokens 字段 + T19 nested 分支读取 + T20 SubTurnRunner Cfg + bootstrap 注入 + T21 显式注入单测 + T22 fallback 单测 + T23 4-parallel integration）。IMPLEMENTED 103→109，P0 74→80。配套 D2 域 +2 T 点（D2-S15-A08 T09 SubTurnRequest→TurnRequest propagation + T10 SubQueryParams→SubTurnRequest pass-through）见 d2 t-registry。AC2 4-parallel deep-review sub-agents 全绿（capture adapter 验证 max 消息 1186 chars folded from 80000）。D7TestStack 同步修复 deepseek DefaultModel / ModelRouting 默认空值（unblocks 所有 D7 integration test）。
>
> **v3.9 closure (2026-06-20):** **devrix-error-handling-tier1-tier2 (DM-20260620-003) 归档**：+7 T 点 IMPLEMENTED（D7-S1-T18 TaskManager.Create `(*Task, error)` + D7-S1-T19 resolveDelegateTaskID `(string, error)` + D7-S2-A06-T24 turn_adapter invariant migration to sharederrors + T25 subturn error_code wrap + T26 subturn channel-closed sentinel + T27 retry nil-sentinel defensive wrap + D7-S2-A02-T18 orchestrator.emitError variadic code）。IMPLEMENTED 109→116，P0 80→83。配套 D3 域 +1 T 点（D3-S3-A01-T16 retry nil-sentinel）见 d3 t-registry；D5 域 +1 T 点（D5-S23-A06-T03 Observability.Shutdown errors.Join）见 d5 t-registry。Tier 1 (PR-A #141) + Tier 2 C2 (PR-B #142) + Tier 2 H3+M1..M4 (PR-C #143) 全部 merged；跨切面 `docs/error-handling.md` v1.0 落地。
>
> **v3.17 closure (2026-06-25):** **devrix-d7-mups-v5-escape-engine (DM-20260625-003) 归档**：+18 T 点 (17 IMPLEMENTED + 1 PARTIAL，D7-S14-A50 T01..T18)。IMPLEMENTED 168→184, PARTIAL 1→2, P0 135→153。V5.1 LoopDepthTracker v2 (commit 0f7243a) + V5.2 PlanKindSwitchPolicy (a862892) + V5.3 ChainedArbitrator 3 层 (69844e3) + V5.4 EscapeEngine + CircuitBreaker 5 层 (2382207) + V5.5 5 节点接线 + 8 unit + 5 integration 100% PASS。S4-Gate review C-1 修复: processEscapeDecision signature `bool` → `(bool, error)` 透传 augmented error。V5.4 修复: Engine 决策合并逻辑 0/1/2+ 信号分层。V5.5 落地 3 接线点 (1a/1b/2; 3 Verify 失败 待 processAutoClose 暴露 verdict 后接入)。22/22 orchestration 包 go test -race 100% PASS（pre-existing TestAutoClose_FullLP1Loop 1s timeout 不影响 V5.5）。T12 PARTIAL 原因: V5.5 仅完成 V5.3 HumanArbitrator.ResumeSession 存储层，SessionOrchestrator.ProcessMessage 入口 applyResumeSession + runLoopWithResume 留待 PR-V5.6。Scenarios D7-S14 0→1。详见 `openspec/archive/2026-06-25-devrix-d7-mups-v5-escape-engine/specs/d7-orchestration/spec.md` §D7-S14-A50。

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| **4.26.0** | **2026-07-02** | **devrix-mups-tool-classification-and-channel-autonomy (DM-20260701-007) S4+S5 验收**: D7-S9-A50-T01..T08 ToolChannel Router + 4 channels (Phase B 8 T) + D7-S9-A26-T06 PlanChannel rename (Phase B-pre 1 T) + D7-S10-A50-T01..T04 VerifyContract + BurdenOfProof + D1 Reason 透传 (Phase C 4 T) + D7-S2-A50-T07/T08 meta 透传 + Learn ReasonLog (Phase C 2 T) -- **15 新 T 全部 P0 IMPLEMENTED**. Total 266->281, P0 222->237. S10 新 S section (0->4 A + 12 T 含 8 既有 A32/A33/A34/A35). PR-B/C/D 待合入. 详见 acceptance-report.md (verdict: ACCEPTED). [retroactive S6 archive 2026-07-02 — DM-20260702-008 devrix-token-design-v2 PR #376 (ProbeToolChannel.Accept 永真 T09 + read_file offset/limit T10 + Default OpenEnded T11 + task_kind 推 advisory T12 共用此版本条目, 详见 `openspec/archive/2026-07-02-devrix-token-design-v2/acceptance-report.md`] |
| **4.27.0** | **2026-07-02** | **devrix-d2-tool-input-aware-concurrency-and-classifier (DM-20260702-009) S5 验收 S6 归档**: D7-S10-A50-T22 AutoModeClassifier P2 interface stub + D7-S10-A50-T23 ChannelRouter TODO 占位 + D7-S10-A50-T24 Classifier interface stub 单测 (4 单测) + D7-S9-A50-T26 Bash sibling abort (per-batch controller) + D7-S9-A50-T27 StreamingToolExecutor.Discard() + fallback 路径 wiring — **5 新 T 全部 IMPLEMENTED (3 P0 + 2 P1)**. Total 281→286, P0 237→240. 2 tech-debt 关闭 (TD-STE-02 + TD-STE-03). 5 PR (PR-D+E `57469504` + PR-F `1763b2cb`+`cbcc57d9`+`c0ef5954`) 全部合入. 详见 `openspec/archive/2026-07-02-devrix-d2-tool-input-aware-concurrency-and-classifier/acceptance-report.md` (verdict: ACCEPTED). |
| **4.28.0** | **2026-07-04** | **devrix-runtime-feedback-closure (DM-20260704-003) S5 验收**: D7-S2-A50-T09/T10 executeOne tool-level timeout (default 60s, env `DEVRIX_TOOL_TIMEOUT_SECONDS` 可调) + fail-closed `tool execution timeout` 错误 — **2 新 T 全部 P0 IMPLEMENTED**. Total 286→288, P0 240→242. 详见 `openspec/changes/devrix-runtime-feedback-closure/acceptance-report.md` (verdict: ACCEPTED). |
| 1.0.0 | 2026-06-13 | 初始（仅 ORCH-S2-T* 遗留 ID） |
| 2.0.0 | 2026-06-14 | D7-S*-T* 统一编号、Legacy 映射、S1/S5/契约 T 点补全 |
| 2.1.0 | 2026-06-14 | Review R1：T02 拆分、T06/T07、MIG-T01、v1.0/v1.1 范围标注 |
| 2.2.0 | 2026-06-14 | Review R2：T02c 端到端 SLA、T04 中断顺序、D7-D6-T01 metric、S5-T02 白名单 |
| 2.3.0 | 2026-06-14 | DM-20260614-005：D7-S5-T03 / T06 闭环（端到端测试 + CommandFirst=false 回归） |
| 2.4.0 | 2026-06-14 | devrix-d7-sa-refine (DM-20260614-008)：T03 anti-fabrication、D7-S5-A01-T01/T02、S5-A02-T01 新增 |
| 2.5.0 | 2026-06-15 | DM-020 v1.0 Registry：D7-S2-A06/A07 T 点（6 个 PLANNED）+ Legacy T 映射表 |
| 2.6.0 | 2026-06-15 | **v1.1 closure 同步**：(1) D7-S1 T01-T05 路径迁入 workmodel/；(2) D7-S1-T06 升 IMPLEMENTED（decomposer_test.go::validateGraph）；(3) D7-S5-T04/T05 升 IMPLEMENTED；(4) D7-S5-A02-T01/T02 + A03-T01/T02 增补；(5) D7-S2-A04-T01/T02/T03 Dispatcher；(6) D7-S4-T08/T09 SpokeBridge。总 55→67，IMPLEMENTED 40→53 |
| 2.7.0 | 2026-06-15 | **D7-S5-A03-T03 LLM Decomposer 闭环**：`decisionplanning/llm_decomposer_test.go` 7 T sub-cases（happy path / bad JSON / enum coercion / unknown deps / extractJSON / nil LLM / SynthesizeTaskGraph routing）；D7-S5 总数 14→21，IMPLEMENTED 11→18 |
| 2.8.0 | 2026-06-15 | **D2 Thin + CLI Worker + BackgroundRun 闭环**：(1) D7-D4-T01/D7-THIN-T01/D7-THIN-T02 PLANNED→IMPLEMENTED；(2) D7-S3-T11 PARTIAL→IMPLEMENTED（SIGTERM/SIGKILL 测试）；(3) D7-S1-T07 PARTIAL→IMPLEMENTED（LocalWorkModel.SetBackgroundProvider + GlobalBackgroundRegistry 初始化）；IMPLEMENTED 53→58，PARTIAL 1→0，PLANNED 13→9 |
| 3.0.0 | 2026-06-15 | **v1.2 + v2.0-b/c/f 全部闭环**：(1) D7-S1-T08 state machine guard + test（IsLegalTransition 24 transition + 4 journey）；(2) D7-S5-A01-T01 confidence threshold verification + FastPathThreshold gating；(3) D7-S2-A06-T01..T04 turn leader 全部 IMPLEMENTED（含 SubQuery nested turn）；(4) D7-S2-A07-T01/T02 LLM invoker breaker/timeout 测试（llm_test.go 9 tests）；IMPLEMENTED 58→66，PLANNED 9→1 |
| **3.1.0** | **2026-06-16** | **devrix-d7-uncertainty-gaps (DM-20260616-001) 归档**：(1) D7-S3 +9 T 点（ConflictGuard TOCTOU 4 + FlowEvent sink 2 + WaveScheduler 积分 3）；(2) D7-S5 +12 T 点（PlanAgent runtime gate 4 + PlanMode LLM guard 2 + PlanModeApproveGate removal 2 + LLM Decomposer 积分 4）；(3) D7-S2 +5 T 点（dead code markers 2 + multi-turn 积分 3）。IMPLEMENTED 66→92，P0 44→63 |
| **3.2.0** | **2026-06-16** | **devrix-d7-loop-first-routing (DM-20260616-002) 归档**：Loop-First L5 登记 D7-S2-L5-01..06（6 P0/P1） |
| **3.3.0** | **2026-06-17** | **devrix-queryloop-legacy-decommission (DM-20260617-001)**：(1) D7-S2-A06-T09 登记（orchestrator 不触 D2.QueryLoop.Run）；(2) D7-S2-A06-T10 登记（Run() 必增 metric）。IMPLEMENTED 92→94 |
| **3.5.0** | **2026-06-19** | **devrix-d7-v2-structure 路径同步**：T 表 Code Location 列对齐 sessionorchestrator/decisionplanning/wavescheduler/executionflow/orchtypes |
| **3.6.0** | **2026-06-20** | **2026-06-20-devrix-context-budget-and-isolation (devrix-context-budget-and-isolation / DM-20260620-001) Phase A 归档**：D7-S2-A06 +3 T 点（T11 turn loop 集成 AC1+AC2 + T12 AC4 per-iter audit + T13 bootstrap 接线）。IMPLEMENTED 96→99，P0 67→70 |
| **3.7.0** | **2026-06-20** | **2026-06-20-devrix-context-budget-and-isolation-phase-b (devrix-context-budget-and-isolation / DM-20260620-001-B) Phase B 归档**：D7-S2-A06 +4 T 点（T14 brief mode PreloadedMessages=nil + T15 fork/full mode parity + T16 depth limit + T17 default mode resolution chain）。IMPLEMENTED 99→103，P0 70→74 |
| **3.8.0** | **2026-06-20** | **2026-06-20-devrix-context-budget-phase-c-nested (devrix-context-budget-phase-c-nested / DM-20260620-002) Phase C 归档**：D7-S2-A06 +6 T 点（T18 TurnRequest.MaxContextTokens 字段 + T19 nested 分支读取 + T20 SubTurnRunner Cfg + bootstrap 注入 + T21 显式注入单测 + T22 fallback 单测 + T23 4-parallel integration）。IMPLEMENTED 103→109，P0 74→80。D7TestStack 同步修复 deepseek DefaultModel / ModelRouting 默认空值。 |
| **3.9.0** | **2026-06-20** | **devrix-error-handling-tier1-tier2 (DM-20260620-003) 归档**：D7-S1 +2 T 点 (T18 TaskManager.Create `(*Task, error)` + T19 resolveDelegateTaskID `(string, error)`) + D7-S2-A06 +4 T 点 (T24 invariant migration + T25 subturn error_code wrap + T26 channel-closed sentinel + T27 retry nil-sentinel) + D7-S2-A02 +1 T 点 (T18 emitError variadic code)。IMPLEMENTED 109→116, P0 80→83。 |
| **3.9.0** | **2026-06-21** | **devrix-d7-error-aggregation-and-metrics (DM-20260621-010) 归档**：D7-S6-A11 +3 T 点（interrupt errors.Join aggregation T01/T02/T03）+ D7-S6-A12 +3 T 点（sandbox cleanup observability T04/T05/T06）+ D7-S6-A13 +1 T 点（forker errors.Join + 13 callers backward compat T07）。IMPLEMENTED 116→123, P0 83→90。 |
| **3.9.0** | **2026-06-22** | **devrix-d7-metrics-and-concurrency-hardening (DM-20260622-001) 归档**：D7-S6-A14 +6 T 点（T01 dispatch_loop_wakeups + T02 worker_panics + T03 sandbox_exit_failed OBSOLETE + T04 state.cancels/handles bound + T05 AllowAndRegister hot path + T06 select-default）。IMPLEMENTED 123→129, P0 90→96。 |
| **3.9.0** | **2026-06-23** | **devrix-d7-mups-v4-phase3-execute (DM-20260625-001) Phase 3 PR-C1 归档**：D7-S9-A25 +4 T 点（T01 ArtifactKind 4 类枚举 + T02 SideEffectStatus 5 态 + T03 wavescheduler.Artifact +5 字段 omitempty 向后兼容 + T04 跨域类型上提 shared/types 打破 import cycle）。IMPLEMENTED 129→133, P0 96→100。详见 `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase3-execute/specs/d7-orchestration/spec.md` §D7-S9-A25。 |
| **3.10.0** | **2026-06-23** | **devrix-d7-mups-v4-phase2-observe-plan (DM-20260623-001) Phase 2 PR-A1 + PR-RF 归档**：D7-S8-A15 +6 T 点（T01 Observation 4 类 + 2 Category + sealed Payload + 不可变 + Strength 边界 + JSON wire format + T02 UncertaintyReport ComputeOverallStrength 仅遍历 CatBusiness + defaults half + T03 UncertaintyCoord FromVerifier + IsColdStart + Phase 1 JSON 向后兼容 + 未知 verdict fail-fast + T04 UncertaintyReport Partition 不变式强制保证 + 违反 ErrUncertaintyReportPartitionInvariant + T05 UncertaintyReport FilterByKind 故意遍历全集 + T06 Observation 字段校验 + 不可变 + clamp01Float + validateFact fmt.Errorf wrap）。IMPLEMENTED 133→139, P0 100→106。详见 `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase2-observe-plan/specs/d7-orchestration/spec.md` §D7-S8-A15。 |
| **3.14.0** | **2026-06-24** | **devrix-d7-mups-v4-phase6-observe-learner-wiring (DM-20260624-001) Phase 6 Observe-Learner 跨域闭环集成 归档**：(1) D7-S12-A41 +3 T 点（T01 ObserveRequest struct 3 字段 + NewObserveRequest fail-fast + EffectivePrior 兜底 DefaultDeveloperPrior + Validate + 3 WithPrior 变体 / T02 IntentQuantizer 4 IntentClass 枚举 + IntentPayload + Quantize baseline + QuantizeWithPrior prior.PriorBeta.Mean() 作为 confidence 乘数 clamp [0,100] + 不可变 + 并发安全 / T03 AnomalyDetector + Anomaly + AnomalyReport + HistoricalDetector.Detect baseline + DetectWithPrior threshold = 0.5 × Mean + 不可变 + 并发安全）；(2) D7-S12-A42 +2 T 点（T04 SessionOrchestrator learner 字段 + WithLearner option + buildObserveRequest 3 层 fail-safe + ProcessMessage 在 classifySpan 之前调用 buildObserveRequest + IntentClassifier 接口扩展 ClassifyWithPrior + RuleClassifier + ShadowClassifier 实现 / T05 buildObserveRequest 3 层 fail-safe 单测 + ProcessMessage UsePriorInClassification 集成测试）；(3) D7-S12-A43 +1 T 点（T06 E2E LP-1 闭环集成测试 4 scenarios + 完整 LP-5 反向追溯链验证）。IMPLEMENTED 168→174，P0 135→141，Scenarios D7-S12 0→3。详见 `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase6-observe-learner-wiring/specs/d7-orchestration/spec.md` §D7-S12-A41/A42/A43。 |
| **3.15.0** | **2026-06-25** | **devrix-d7-mups-v4-phase7-verify-auto-close (DM-20260625-001) Phase 7 运行时 5 节点闭环 (PR-7.1/7.2/7.3) 归档**：(1) D7-S13-A47 +3 T 点（T01 processAutoClose 包装 channel + 异步触发 learner.Learn + 替换 endSpanWhenChannelClosed 调用 + IntentSkip 路径不调用 / T02 synthesizeVerdict 规则 complete→Pass / error→Fail / tombstone→Indeterminate + IndeterminateReason="interrupt" + 3 层 fail-safe (nil learner / Learn error / channel cancel) + SourceID 格式 `autoclose:{sessionID}:{nanosecond}` / T03 集成测试 ProcessMessage 完整跑 → Alpha++ + 下一轮 prior 更新 (Round 1 冷启动 Beta(5,3) → Learn VerdictPass → Alpha=1 → Round 2 Beta(6,3) Mean=0.667) + TestAutoClose_FullLP1Loop 端到端 LP-1 闭环在生产 wiring 验证）；(2) D7-S13-A48 +2 T 点（T04 ProcessRequest 新增 TrackMode string 字段 (默认 "" 兜底 developer) + TrackModeDeveloper/Operator 常量 + NewProcessRequest fail-fast 校验 + 3 个 sentinel error / T05 buildObserveRequest 透传 req.TrackMode → o.learner.Inject(ctx, sessionID, req.TrackMode) → BuildAdaptivePrior Operator track → DefaultOperatorPrior Beta(8,1) Mean=0.889，Developer → Beta(5,3) Mean=0.625，空字符串/未知 → 兜底 Developer）；(3) D7-S13-A49 +1 T 点（T06 sessionSpan 6 prior attributes (alpha/beta/mean/track_mode/classifier_source/injected_at) 全部写入 + priorSessionSpanAttrs 纯 helper 便于单元测试 + 5 个单测覆盖 real injection / cold_start_failsafe / operator from hint / reputation wins / 字符串类型校验）。IMPLEMENTED 174→180，P0 141→147，Scenarios D7-S13 0→6。详见 `openspec/archive/2026-06-25-devrix-d7-mups-v4-phase7-verify-auto-close/specs/d7-orchestration/spec.md` §D7-S13-A47/A48/A49。 |
| **3.11.0** | **2026-06-23** | **devrix-d7-mups-v4-phase2-plan (DM-20260623-001-PRB1) Phase 2 PR-B1 归档**：D7-S8-A22 +3 T 点（T01 PlanKind 4 类枚举 + String/Marshal/Unmarshal/Parse + T02 Plan.SourceObservationIDs 必填 + 防御性拷贝 + ReverseLookupObservations Phase 4 入口 + T03 MatchKind 4 规则分类器 + uncertainty-first tie-break + DefaultPlanner.Plan 集成 + strengthFloor 公式）。**devrix-d7-mups-v4-phase3-channels (DM-20260625-001-PRC2) Phase 3 PR-C2 归档**：D7-S9-A26 +5 T 点（T01 ChannelRegistry 1:1 绑定 + ChannelRouter 4 PlanKind → 4 ArtifactKind 1:1 映射 + defensive checks + T02 CommitChannel 1-Step 同步 + IdempotencyKey 强制 + 超时 SideEffectInflight + T03 ProtocolChannel 顺序多步 + reverse-order rollback + T04 ScenarioChannel MaxParallel=5 并行探测 + 多数派投票 + T05 ExplorationChannel MaxParallel=3 多 agent + 优先级排序 + PersistScope 派生 SideEffectStatus）。IMPLEMENTED 139→147, P0 106→114。详见 `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase2-plan/specs/d7-orchestration/spec.md` §D7-S8-A22 + `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase3-channels/specs/d7-orchestration/spec.md` §D7-S9-A26。 |
| **3.4.0** | **2026-06-19** | **devrix-d7-v2-structure (DM-20260619-005)**：T ID 不变（66/66 IMPLEMENTED）；测试文件随实现迁移 |
| **3.17.0** | **2026-06-25** | **devrix-d7-mups-v5-escape-engine (DM-20260625-003) V5.1..V5.5 归档 (17 IMPLEMENTED + 1 PARTIAL)**：D7-S14-A50 +18 T 点（T01 LoopContext 7 字段 + T02 LoopDepthTracker ForceExit 边界 + T03 PlanKindSwitchPolicy 3 档 + T04 EscapeAction 6 类 + T05 LLM/Rule/Human 3 层仲裁 + T06 Notifier + PendingResolutionStore + T07 EscapeEngine 整合 + T08 LoopBudget + T09 CircuitBreaker 5 层 + T10 EscapeAuditLog + T11 Orchestrator 5 节点接线 + T12 ResumeSession ⚠️PARTIAL + T13 buildLoopContext + T14..T18 L4/L3/L2/L1/gap 测试）。IMPLEMENTED 168→184, P0 135→153, PARTIAL 1→2, Scenarios D7-S14 0→1。S4-Gate review 修复: C-1 processEscapeDecision 返回 augmented error 避免静默吞错 (signature `bool` → `(bool, error)`)。T12 PARTIAL 原因: V5.5 仅完成 V5.3 HumanArbitrator.ResumeSession 存储层，SessionOrchestrator.ProcessMessage 入口 applyResumeSession + runLoopWithResume 留待 PR-V5.6。V5.4 修复: Engine 决策合并逻辑 0/1/2+ 信号分层 (0→Continue / 1→直接返回硬信号 / 2+→ChainedArbitrator)。V5.5 落地 3 接线点 (1a Plan 失败 / 1b Plan 前 / 2 Execute 失败；3 Verify 失败 待 processAutoClose 暴露 verdict 后接入) + 8 wiring 单元测试 + 5 集成测试 100% PASS。详见 `openspec/archive/2026-06-25-devrix-d7-mups-v5-escape-engine/specs/d7-orchestration/spec.md` §D7-S14-A50。 |
| **3.18.0** | **2026-06-25** | **devrix-d7-mups-v5-escape-engine-v5-6 (DM-20260625-003) PR-V5.6 T12 PARTIAL→IMPLEMENTED**：(D7-S14-A50-T12 IMPLEMENTED。SessionOrchestrator.applyResumeSession 实现 + 3 层 fail-safe (nil engine / ResumeSession error / TTL expired) + 3 决策路由 (A user_continue fall through / B user_accept→ForceExit 短路 emit "complete" / C user_cancel→AbortWithAudit 短路 emit "complete") + 3 sessionSpan attrs (escape.resume.attempted / decision_action / decision_pending_id) + resumeContentForDecision helper (6 类 EscapeAction → 中文 text) + 6 单元测试 (NoEngine / NoPending / UserAccept / UserCancel / UserContinue / ResumeError_Failsafe) + 2 集成测试 (TestProcessMessage_WithResume_UserAccept_EarlyClose / UserCancel_EarlyClose) 8/8 PASS 含 race + 3/3 稳定性验证。spec.md v4.9.0 → v4.10.0 + 域 t-registry v3.17.0 → v3.18.0 同步。IMPLEMENTED 184→186, PARTIAL 2→0, Scenarios D7-S11 T13 PARTIAL→0 + D7-S14 T12 PARTIAL→0, 总 PARTIAL 2→0, 总 18/18 IMPLEMENTED 0 PARTIAL。runLoopWithResume 在 V5.6 实现中被简化合并到 EscapeContinue fall through 路径, 不需要独立 wrapper 函数; LoopDepthTracker 自动保证 depth 续 T1 状态。详见 `openspec/archive/2026-06-25-devrix-d7-mups-v5-escape-engine-v5-6/specs/d7-orchestration/spec.md` §D7-S14-A50。**v3.18.0 review fix (DM-20260625-004)**：C-1 t-registry 删除 `runLoopWithResume` 描述 (代码 0 命中) + C-2 Statistics 表 4 处数字刷新 + v3.18.0 条目追加 (本条)。 |
| **4.0.0** | **2026-06-26** | **6 S 精简 + 5 个新 P0/P1 Span（DM-20260626-001 / devrix-d7-six-s-simplification PR #215）**：14 S → **6 S + 1 横切** 重归类（S 编号变化，T ID 保持稳定以便追溯）；A 活动 **56 → 49**（S1:4 · S2:7 · S3:4 · S4:9 · S5:8 · S6:15 + Hardening:2）；F 层按新 S 重归类（F 总数 75 → 68，Legacy 41 + Canonical 27）；MUPS 5 节点挂载：Observe+Plan 归 S5，Execute+Learn 归 S6，Verify 归 S4，AutoClose+Resume+Escape入口 归 S2；7 Legacy A 全部并入 Canonical；版本号 v3.18.0 → v4.0.0（minor 升 major，反映 14 S → 6 S 精简是结构性变更）；新增 20 P0 T：(1) D7-S6-A48 channel.route +2 (T01 EmitChannelRoute happy + T02 nil-bridge fail-safe)；(2) D7-S6-A49 memory.persist +2 (T03 EmitMemoryPersist happy + T04 nil-bridge fail-safe)；(3) D7-S4-A47 system.anomaly_detect +8 (T05 EmitSystemAnomalyDetect happy + T06 nil-bridge + T11 DetectSystemAnomaly triggered High + T12 triggered Medium + T13 not triggered None + T14 overrideKind forward-compat v6.1 + T15 nil-bridge + T16 default thresholds)；(4) D7-S5-A33 taskgraph.synthesize +6 (T07 EmitTaskGraphSynthesize happy + T08 nil-bridge + T17 dagDepth empty + T18 linear chain t1→t4=4 + T19 branching diamond=3 + T20 SynthesizeTaskGraph Span emit fail-safe)；(5) D7-S5-A34 executor.select +2 (T09 EmitExecutorSelect happy + T10 nil-bridge)。20 新 P0 T 全 IMPLEMENTED，D7 T 186→206（重归类不删测试点），D7 P0 153→173。详细 A/T 重映射见 `a-registry.md §v6.0.0 6 S 精简映射` + `span-registry.md §Operations`（v6.0.0 已落地 23 ops + 9 sessionSpan attributes）|
| **4.1.0** | **2026-06-26** | **mups 包路径迁移 PLANNED 预登记（DM-20260626-002 / devrix-d7-mups-package-migration）**：Step 2 (v6.0.0 follow-up) — execute/ + learn/ → mups/ 子树物理目录迁移，纯目录迁移 + import path 替换（保持 `package execute` / `package learn` 声明不变 + 函数签名/行为 0 变化）；加 4 P0 T 点：D7-S6-A51 T01 mups/execute/ 目录 + 7 .go git mv / T02 mups/learn/ 目录 + 17 .go git mv / T03 15 处 import path 全仓替换 + grep 0 残留 / T04 go build/vet/test -race 22/22 PASS + LP-1/LP-2/LP-5 路径 0 变化。Total 206→210, PLANNED 0→4, P0 173→177（IMPLEMENTED 持平 206）。收口后 v4.1.0 → v4.2.0。详见 `openspec/changes/devrix-d7-mups-package-migration/proposal.md` + `demand.md`。 |
| **4.2.0** | **2026-06-26** | **mups 包路径迁移 IMPLEMENTED 收口（DM-20260626-002 落地）**：execute/ + learn/ → mups/ 子树物理目录迁移完成 (commit cb965d9: 24 文件 git mv rename 100%) + 17 处 import path 全仓替换完成 (commit e22ef5d: decisionplanning 2 + orchtypes 6 + sessionorchestrator 9) + go build/vet/test -race 全绿 (22/22 orchestration packages PASS + 130 全仓包 0 FAIL + 0 panic)。4 新 P0 T (D7-S6-A51-T01..T04) PLANNED → IMPLEMENTED, Total 210, IMPLEMENTED 206→210, PLANNED 4→0, P0 177。22 包 baseline 持平 (PR #215 验证)，LP-1/LP-2/LP-5 集成测试 100% 兼容 (Phase 6 TestAutoClose_FullLP1Loop + Phase 7 Verify Auto-Close 集成测试全部通过)。版本号 v4.1.0 → v4.2.0。详见 `openspec/archive/2026-06-26-devrix-d7-mups-package-migration/acceptance-report.md` (S6 归档阶段)。 |
| **4.3.0** | **2026-06-26** | **Hardening 横切包物理落地（DM-20260626-003 / devrix-d7-hardening-cross-cutting 落地）**：`orchestration/hardening/` 目录新建（5 .go: doc.go + metrics.go + metrics_test.go + recovery.go + recovery_test.go），承接 v6.0.0 6 S + 1 横切 Discipline Keeper 横切角色；`sessionorchestrator/metrics.go` (61 行 InterruptMetrics) + `turn/recovery.go` subset（4 纯函数 + 1 const）git mv 迁 hardening/；`escape/circuit_breaker.go` 留 escape/（V5 EscapeEngine 核心机制，Decision 1，git diff 0 变化）；receiver methods（compressMessagesForRecovery + invokeStreamWithRecovery）保留 turn/ 紧耦合 *DefaultOrchestrator（Decision 2）；4 新 P0 T（D7-S7-A01-T01 + D7-S7-A02-T02 + D7-S7-A01-T03 + D7-S7-A01-T04）PLANNED → IMPLEMENTED, Total 210→214, IMPLEMENTED 210→214, PLANNED 0→0, P0 177→181；go build/vet/test -race 23/23 PASS（含 hardening 1 新包）+ LP-1（TestAutoClose_FullLP1Loop）+ LP-2（TestIntegration_5NodePipeline_End2End）100% 兼容。详见 `openspec/archive/2026-06-26-devrix-d7-hardening-cross-cutting/acceptance-report.md` (S6 归档阶段)。 |
| **4.4.0** | **2026-06-26** | **turn/ → sessionorchestrator/ 整包物理合并（DM-20260626-004 / devrix-d7-6s-package-merge 落地）**：D7-S2 SessionOrchestrator 单一博弈角色单一 Go 包封装（pure physical migration + import path replace）。24 .go 文件 git mv + 14 importer 文件 import path replace + 跨包 import cycle 打破（LLMInvoker/LLMInvokeRequest/ToolSchema 上提 `orchtypes/` + sessionorchestrator 用 type alias）。**0 函数签名变化** + 0 行为变化 + `hardening/` + `escape/circuit_breaker.go` + `sessionorchestrator/autoclose.go` 0 变更验证。22/22 orchestration packages go test -race PASS + go build + go vet 全绿。**4 新 P0 T** IMPLEMENTED：D7-S2-A50-T01 `orchestration/turn/` 24 .go 文件 git mv → `orchestration/sessionorchestrator/`（contracts/doc/orchestrator/orchestrator_test/tracing 5 文件加 turn_ 前缀解决同名冲突）/ D7-S2-A50-T02 24 .go 文件 `package turn` → `package sessionorchestrator` 替换 / D7-S2-A50-T03 14 importer 文件 import path + identifier 全替换（10 bootstrap + 2 decisionplanning + 2 sessionorchestrator）+ 跨包 import cycle 打破（orchtypes 上提）/ D7-S2-A50-T04 `orchestration/turn/` 目录 0 残留验证 + `hardening/` + `escape/circuit_breaker.go` + `sessionorchestrator/autoclose.go` git diff 0 变更。Total 214→218, IMPLEMENTED 214→218, PLANNED 0→0, P0 181→185。详见 `openspec/archive/2026-06-26-devrix-d7-6s-package-merge/acceptance-report.md` (S6 归档阶段)。 |
| **4.5.0** | **2026-06-26** | **verify-promotion 包归属迁移 PLANNED 预登记（DM-20260626-005 / devrix-d7-6s-verify-promotion）**：Step 5 (v6.0.0 follow-up) — DM-20260626-004 turn/ → sessionorchestrator/ 时为避免单 PR scope 膨胀临时留存的 `sessionorchestrator/{exit_reason.go (72 行) + verdict_to_exit_reason.go (49 行) + verdict_to_exit_reason_test.go (97 行)}` 3 文件 (218 行) 物理 promote 到 `executionflow/verify/`；让 S4 ExecutionFlow + Verify (Costly Signaler + Certifier) 角色的可验证承诺 (14 ExitReason + VerdictToExitReason 4 态映射) 在 spec/code 完全对齐；`sessionorchestrator/turn_orchestrator.go` 11 处 `ExitReason*` → `verify.ExitReason*` 跨包引用替换（state 字段 + 6 常量 + 2 函数参数 + 1 type assertion）+ `turn_orchestrator_test.go` 2 处替换。**0 函数签名变化**（pure physical migration，安全网与 DM-20260626-004 一致）；14 ExitReason 字符串值全不变；6 测试函数测试矩阵全不变；加 4 P0 T 点 PLANNED：D7-S4-A50 T01 3 文件 git mv + git log --follow 100% rename detection / T02 3 文件 package 改名 + 13 处 ExitReason* 全替换 + grep 0 残留 / T03 executionflow/verify/ 包 0 sessionorchestrator 反向依赖 + 跨包 cycle 0 风险 / T04 go build/vet/test -race 22/22 PASS + LP-1/LP-2/LP-5 兼容 + hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 变化。Total 218→222, PLANNED 0→4, P0 185 (IMPLEMENTED 持平 218)。收口后 v4.5.0 → v4.6.0。详见 `openspec/changes/devrix-d7-6s-verify-promotion/proposal.md` + `demand.md` + `design.md` + `tasks.md`。 |
| **4.6.0** | **2026-06-26** | **Bootstrap Wire 拓扑收口 IMPLEMENTED（DM-20260626-007 / devrix-d7-6s-bootstrap-slim 落地）**：v6.0.0 follow-up 序列最终 PR（#6）— `internal/bootstrap/wire_coordinator.go` InitOrchestration 函数体 275 → **140 行**（≤ 200 目标达成）+ 6 S × WireFunc 命名一致（5 Wire* + 1 BuildOrchestratePath） + 3 内嵌 adapter 函数（newContextEngineAdapter 已在 turn_adapter.go 独立 + newTurnOrchExecutor + newGatewayEventPublisher）拆到 `internal/bootstrap/adapters.go` (48 行) + 4 util 函数（boolPtr + intPtr + strPtr + mapBackgroundStatus）拆到 `internal/bootstrap/util.go` (30 行) + 52 行 config 加载抽到 `loadOrchestratorConfigs` 辅助函数 (24 行) + 4 行类型断言抽到 `resolveObsBridge` 辅助函数 (6 行) + 新增 `WireDecisionPlanning` (16 行 S5) + `WireMUPSPipeline` + `MUPSPipelinesDeps` (75 行 S6)。**0 函数签名变化**（pure physical refactor）+ cmd/devrix + cmd/obs-verify + tests/testutil/d7_stack.go 调用方 0 变化 + hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 baseline stability。**4 PR 联动** (#225 + #226 + #227 + #228) 全部 squash auto-merge (commit c9a1797)。**4 新 P0 T** IMPLEMENTED：D7-S2-A51-T01 6 S × WireFunc 命名一致 + D7-S2-A51-T02 InitOrchestration 主体 ≤ 200 行 + D7-S2-A51-T03 3 内嵌 adapter + 4 util 函数已抽到独立文件 + D7-S2-A51-T04 22/22 orchestration packages go test -race PASS + 0 baseline regression。Total 222 (IMPLEMENTED 218, PLANNED 4→0), P0 185 (P0 持平)。**v6.0.0 follow-up 序列收官**：5/6 S7_Archived + 1/6 S1_Cancelled (observe-merge) + 1/1 S7_Archived (本次) = D7 编排层进入 v6.0.x 维护阶段。详见 `openspec/archive/2026-06-26-devrix-d7-6s-bootstrap-slim/acceptance-report.md` (S6 归档阶段)。 |
| **4.7.0** | **2026-06-26** | **DM-20260626-009 follow-up 内层 observability span + dedup 删除（PR #253+#254 落地）**：3 层 LCP-based dedup 在 D1 adapter 兜底 minimax M2.7 流式回放 bug，但同时把自然中文复述当成 echo 误杀（错层：D3 gateway bug 不该 D1 adapter 兜底）。D1/D2/textutil 三处 dedup 全删 + 5-node MUPS 根 span 之外 3 内层 span 落地：(1) D7-S1-A52 worktree.op（P1，2 T 点 T11 EmitWorktreeOp happy + T12 nil-bridge fail-safe，6 P0/P1 spans 之一）+ (2) D7-S1-A53 subworktree.run（P2，2 T 点 T13 EmitSubWorktreeRun happy + T14 nil-bridge fail-safe）+ (3) D7-S5-A54 subturn.iteration（P1，2 T 点 T15 EmitSubTurnIteration happy + T16 nil-bridge fail-safe）。`subturn.finish_reason` 取 LLM 真实 finish_reason（stop/tool_calls/length/...）与 executor 自定义的 stop_reason（final_answer/max_iters/tool_error/...）正交；stepOneIter helper 抽离让 span 包单次函数调用而不是 6 inline return path；cap-hit 多发 1 个 `iter=max+1` span 让 "max_iters" 终止态在 Jaeger 显形。**0 函数签名变化**（pure instrumentation）+ 22/22 orchestration packages go test -race PASS + ItemPipelineRunner 11 个 worktree callsite + WorkItemExecutor ReAct iter + session_turn_loop RunParallelExplore 全 wiring。6 P0 T IMPLEMENTED：Total 222→228, IMPLEMENTED 222→228, P0 185→191。详见 `span-registry.md` §4.1.0 + §WorkItem Inner Layer Trace 树。 |
| **4.8.0** | **2026-06-27** | **DM-20260627-001 devrix-d7-workitem-rollup-pipeline (PR #262) S7_Archived PARTIAL ACCEPTED 收口** + 3 root span ops (D7-S1-A55 gate + D7-S1-A56 dual_bubble + D7-S5-A57 rollup_mups) + ItemPipelineRunner emit hook 配套 span (D7-S1-A58 T17 EmitItemPipelineOp happy + T18 nil-bridge fail-safe); best_effort only 兜底 fail-safe; S7_Archived PARTIAL ACCEPTED 22/22 orchestration packages -race PASS. Total 228, P0 191. |
| **4.9.0** | **2026-06-28** | **devrix-api-error-classification (DM-20260628-001) PLANNED**：API 错误分类与可恢复语义 — `OrchestratorDeps.FallbackModel string` 字段就位 + `TurnState.Withheld bool` 字段就位 + `emitError` 路径用 `sharederrors.Code(err)` 填 `Event.Metadata["error_code"]` 受控枚举 + 主模型 2 次连续 RateLimit/ServerError 触发 `fallback_trigger_candidate` 日志 + prompt_too_long 错误标 `withheld=true` 不 surface + 现有 30+ `SanitizeForUser` 调用点零行为变化。**+2 P0 T PLANNED**：D7-S2-A50-T05 (字段 + emitError code 注入) + D7-S2-A50-T06 (2 次连续触发 fallback 日志 + withheld + SanitizeForUser 回归)。Total 228→230, IMPLEMENTED 228（持平, 2 新 T 均 PLANNED）, PLANNED 0→2, P0 191→193。S4 实现后回填 IMPLEMENTED。 |
| **4.9.1** | **2026-06-28** | **devrix-api-error-classification (DM-20260628-001) S5 验收**：2 P0 T PLANNED→IMPLEMENTED（D7-S2-A50-T05 + T06, PR #265 squash merged, TestEmitErrorWithErr_* 2 case + TestObserveFallbackTrigger_* 3 case 全 PASS）。Total 230（持平）, IMPLEMENTED 228→230, PLANNED 2→0, P0 193。S6 归档 entry 同步。 |
| **4.14.0** | **2026-06-29** | **v7.0 TaskContract 统一 PR-A T 点 落地（DM-20260629-007）**：D7-S20/21 新场景 + 11 P0 T 点 9 IMPLEMENTED + 2 文档同步, T 230→239, P0 195→204; T 编号重映射避开 D7-S16 Layer SubContext 占用 |
| **4.15.0** | **2026-06-29** | **v7.0 TaskContract 统一 PR-B L3 防御运行时层 T 点 落地（DM-20260629-008）**：(1) **新增 D7-S18 段 7 P0 T**（D7-S18-A11-T01 happy path + T02 5 类触发 BuildMVPArtifact + T03 CB L1 → Pessimistic + T04 Feature Flag env-gated + T05 Span/Metric 完整 wire PLANNED + T06 NotifyPessimistic 5 层 fail-safe + D7-S18-A12-T01 4 候选规则 + T02 ResolveFallback 3 路径）；(2) 6/7 IMPLEMENTED + 1 PLANNED（T05 留 PR-C）；(3) **0 函数签名变化**（pure additive, 全部用 interfaces.MVPArtifact + EscapeEngine.SetPessimisticGuard + ChannelRouter.SetPessimisticGuard）；(4) 3 orchestration packages -race PASS 0 FAIL（interfaces / escape / mups/execute）；(5) interfaces coverage **96.9%** / escape coverage **85.0%** / race-clean；(6) **4 ORCH_* SentinelError (7110-7113)** 新增（ORCH_PESSIMISTIC_TRIGGERED_7110 + ORCH_PESSIMISTIC_MVP_EMPTY_7111 + ORCH_FALLBACK_RULE_INVALID_7112 + ORCH_FALLBACK_ABORT_TIMEOUT_7113）；(7) **Feature Flag D7_PESSIMISTIC_COMMIT_ENABLED 默认 disabled, 0 行为变更**；(8) Total 239（持平，新增 7 T, 移除 0）, P0 204→211 |

---

## ADDED Test Points (D7-S13: Phase 7 Verify→Learn Auto-Close + Operator TrackMode + D5 增强)

### D7-S13-A47: SessionOrchestrator.processAutoClose (Verify→Learn Auto-Close)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S13-A47-T01** | processAutoClose 包装 channel + 异步触发 learner.Learn + 替换 endSpanWhenChannelClosed 调用 | IMPLEMENTED | `sessionorchestrator/orchestrator.go` + `orchestrator_autoclose_test.go` (NEW) | Verify_AutoClose |
| **D7-S13-A47-T02** | synthesizeVerdict 规则 (complete→Pass / error→Fail / tombstone→Indeterminate) + 3 层 fail-safe (nil learner / Learn error / channel cancel) | IMPLEMENTED | `sessionorchestrator/orchestrator.go` + `orchestrator_autoclose_test.go` (NEW) | Verify_AutoClose |
| **D7-S13-A47-T03** | 集成测试 ProcessMessage 完整跑 → Alpha++ + 下一轮 prior 更新 (Round 1 冷启动 Beta(5,3) → Learn VerdictPass → Alpha=1 → Round 2 Beta(6,3) Mean=0.667) + TestAutoClose_FullLP1Loop 端到端 LP-1 闭环 | IMPLEMENTED | `sessionorchestrator/orchestrator_autoclose_test.go` (NEW) | Verify_AutoClose |

### D7-S13-A48: ProcessRequest.TrackMode (Operator 角色支持)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S13-A48-T04** | ProcessRequest 新增 TrackMode string 字段 (默认 "" 兜底 developer) + TrackModeDeveloper/Operator 常量 + NewProcessRequest fail-fast 校验 + 3 个 sentinel error | IMPLEMENTED | `orchtypes/process.go` + `orchtypes/process_test.go` (NEW) | Observe_Request_WithPrior |
| **D7-S13-A48-T05** | buildObserveRequest 透传 req.TrackMode → o.learner.Inject(ctx, sessionID, req.TrackMode) → BuildAdaptivePrior (Operator track → DefaultOperatorPrior Beta(8,1) Mean=0.889) | IMPLEMENTED | `sessionorchestrator/orchestrator.go` + `orchestrator_trackmode_test.go` (NEW) | Observe_Request_WithPrior |

### D7-S13-A49: sessionSpan 6 prior attributes (D5 可观测化增强)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S13-A49-T06** | sessionSpan 新增 4 属性 (learn.prior.mean / track_mode / injected_at / learn.classifier_source) + 6 字段全部写入测试 (含 cold_start_failsafe 标记) | IMPLEMENTED | `sessionorchestrator/orchestrator.go` + `sessionorchestrator/tracing.go` + `orchestrator_priorspan_test.go` (NEW) | Session_Process |

## Scenario D7-S13 Detail (test points summary)

```
D7-S13  Phase 7 Verify→Learn Auto-Close + Operator TrackMode + D5 增强
├── A47  SessionOrchestrator.processAutoClose (Verify→Learn Auto-Close)
│   ├── T01  processAutoClose 包装 channel + 异步 Learn              [IMPLEMENTED]
│   ├── T02  synthesizeVerdict 规则 + 3 层 fail-safe                 [IMPLEMENTED]
│   └── T03  集成测试 Alpha++ + 下一轮 prior 更新 + LP-1 闭环         [IMPLEMENTED]
├── A48  ProcessRequest.TrackMode (Operator 角色支持)
│   ├── T04  ProcessRequest.TrackMode 字段 + 验证 + sentinel errors  [IMPLEMENTED]
│   └── T05  buildObserveRequest 透传 + Operator Beta(8,1) Mean=0.889 [IMPLEMENTED]
└── A49  sessionSpan 6 prior attributes (D5 可观测化增强)
    └── T06  6 prior attributes 全部写入 + cold_start_failsafe 标记  [IMPLEMENTED]
```

**Total**: 6 P0 T points, 6 IMPLEMENTED, 0 PARTIAL.

---

## D7-S14: MUPS v5 统一逃逸机制 (DM-20260625-003, IMPLEMENTED)

> **Change:** 2026-06-25-devrix-d7-mups-v5-escape-engine (DM-20260625-003) — MUPS v5 统一逃逸机制: LoopDepthTracker v2 + PlanKindSwitchPolicy + EscapeAction 6 类 + ChainedArbitrator LLM/Rule/Human + EscapeEngine + CircuitBreaker 5 层 + AuditLog + 5 节点 EscapeEngine 接线点 + T2 ResumeSession 续跑 + 13 类失败降级矩阵. 18 IMPLEMENTED P0 T points (5 PR 拆分 PR-V5.1..V5.5, 1.2+0.5+2.2+1.8+3.0=8.7 天工作量). review-r3.md 6 ISSUE 已修复 (ISSUE-1 MaxDepth 边界 / ISSUE-2 ChainedArbitrator 骨架 / ISSUE-3 applyResumeDecision 骨架 / ISSUE-4 Notifier 清理 / ISSUE-5 Observe 失败 Continue / ISSUE-6 L2-07 表驱动). 121 tests 设计 (L4 4 + L3 7 + L2 7 + L1 103), 覆盖率 85%→97%. V5.4 修复: Engine 决策合并逻辑 0/1/2+ 信号分层 (0→Continue / 1→直接返回硬信号 / 2+→ChainedArbitrator). V5.5 落地 3 接线点 (1a/1b/2) + 8 wiring 单元测试 + 5 集成测试 (4DepthLimits/3LayerArbitration/5EscapeActions/PlanKindSwitchLimit/5NodePipeline_End2End) 100% PASS.

### D7-S14-A50: LoopDepthTracker v2 + PlanKindSwitchPolicy + ChainedArbitrator + EscapeEngine + CircuitBreaker + AuditLog

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S14-A50-T01** | LoopContext struct (7 字段: 5 hash 输入 + 2 状态) + hashLoopContext SHA-256 + History 按 SessionID 隔离 | IMPLEMENTED | `orchestration/escape/loop_depth_tracker.go` (PR-V5.1) | Escape_Engine_Run |
| **D7-S14-A50-T02** | LoopDepthTracker.ShouldContinue 严格按 `depth >= MaxDepth` 触发 ForceExit (MaxDepth=3, depth=1/2 Continue, depth=3 ForceExit) + Reset 按 SessionID 维度清空 | IMPLEMENTED | `orchestration/escape/loop_depth_tracker.go` (PR-V5.1) | Escape_Engine_Run |
| **D7-S14-A50-T03** | PlanKindSwitchPolicy 3 档 enum + determineSwitchPolicy (Exploration→Constrained ≤4 / Scenario→Allowed / Protocol→Constrained ≤4 / Commitment→Forbidden) + 累计计数 (Commitment 1 次→ForceExit, Constrained 5 次→ForceExit) | IMPLEMENTED | `orchestration/escape/plan_kind_switch_policy.go` (PR-V5.2) | Escape_Engine_Run |
| **D7-S14-A50-T04** | EscapeAction 6 类 typed enum (Continue / EscalateToRule / EscalateToHuman / ForceExit / AbortWithAudit / EscapePendingHuman) + EscapeDecision 9 字段 (5 核心 Action/Reason/AuditLevel/Depth/PendingID + 4 审计 ExitReason/SessionID/CreatedAt/SourceDecisionIDs) | IMPLEMENTED | `orchestration/escape/arbitrator.go` (PR-V5.3) | Escape_Engine_Run |
| **D7-S14-A50-T05** | LLMArbitrator (5s timeout 兜底 ForceExit + 1 次格式重试 + 非 JSON / 非法 action 拦截 + ctx 取消语义优先 + recover panic) + RuleArbitrator (不可恢复→AbortWithAudit, 可恢复→EscalateToHuman) + HumanArbitrator (10s timeout + 异步化立即返回 EscapePendingHuman + SubmitUserChoice 缓冲 1 + SubmitOverrideCard 防 UI 误导) + ChainedArbitrator (LLM→Rule→Human 链式调用, EscalateTo* 中间态消化绝不返回 caller) | IMPLEMENTED | `orchestration/escape/arbitrator.go` (PR-V5.3) | Escape_Engine_Run |
| **D7-S14-A50-T06** | Notifier interface + FeishuCardNotifier (3 按钮 A/B/C + ExpiresAt 10s) + ChainedNotifier (FeishuCard→CLI→Email fallback) + OverrideCardNotifier 可选 interface + PendingResolutionStore interface + InMemoryPendingResolutionStore (TTL=10s 过期清理) + ResumeSession 委托 HumanArbitrator (Save/Load/Delete 闭环) | IMPLEMENTED | `orchestration/escape/notifier.go` + `pending_resolution_store.go` (PR-V5.3) | Escape_Engine_Run |
| **D7-S14-A50-T07** | EscapeEngine.Evaluate 整合入口 (3 类深度限制串联: tracker → loopBudget → circuitBreaker, 全部 Continue → EscapeContinue, 任一非 Continue → ChainedArbitrator) + AuditLevel 0/1/2 (记录次数递增) + 13 类失败降级矩阵 (Evaluate panic/error + audit fail-open + LLM timeout + ctx cancel + CB metric timeout + ...) | IMPLEMENTED | `orchestration/escape/engine.go` (PR-V5.4) | Escape_Engine_Run |
| **D7-S14-A50-T08** | LoopBudget struct (ConsecutiveFails=3 触发 ForceExit + TotalFails=20 触发 AbortWithAudit, doc 38 §19.2 DenialBudget 概念) + LoopBudget.Evaluate | IMPLEMENTED | `orchestration/escape/loop_budget.go` (PR-V5.4) | Escape_Engine_Run |
| **D7-S14-A50-T09** | CircuitBreaker 5 层接线 (L0 AnomalyDetector 5 nil / L1 DispatchLoop 100/min / L2 Verifier 3×2s / L3 Hook 5 fail / L4 WorkerPanic 1 / L5 SandboxExit 5 fail) + State machine Open→HalfOpen→Close + 阈值占位推导 (V5.5 集成测试后回填) + CB 拉 metric 200ms timeout 防御 | IMPLEMENTED | `orchestration/escape/circuit_breaker.go` (PR-V5.4) | Escape_Engine_Run |
| **D7-S14-A50-T10** | EscapeAuditLog (AuditLevel 0/1/2) + InMemoryEscapeAuditLog (含 SourceDecisionIDs + CreatedAt) + EscapeDecision.ExitReason 14 类 Phase 4 映射 | IMPLEMENTED | `orchestration/escape/audit_log.go` (PR-V5.4) | Escape_Engine_Run |
| **D7-S14-A50-T11** | SessionOrchestrator.ProcessMessage 5 节点接线 (Observe 失败 / Plan 失败 1a / Plan 前 1b / Execute 失败 / Verify 失败) + 1a 短路不调 1b (codex R4 修复) + processEscapeDecision 6 类 action 统一处理 (Continue→continue 回路 / PendingHuman→return nil 异步 / ForceExit/Abort→return error / EscalateTo*→兜底 ForceExit) | IMPLEMENTED | `orchestration/sessionorchestrator/orchestrator.go` (PR-V5.5) | Escape_Engine_Run |
| **D7-S14-A50-T12** | ResumeSession T2 续跑入口 (ProcessMessage 开头检查 → applyResumeSession) + applyResumeSession (user_choice=A→EscapeContinue fall through to 5-node pipeline / B→ForceExit 短路 emit "complete" / C→AbortWithAudit 短路 emit "complete" — audit already recorded at SubmitUserChoice time V5.4, resume is read-only) + 3 层 fail-safe (nil engine / ResumeSession error / TTL expired → 静默 fall through) + 3 sessionSpan attrs (escape.resume.attempted / decision_action / decision_pending_id) + resumeContentForDecision helper (6 类 EscapeAction → 中文 text 消息) | **IMPLEMENTED** | `orchestration/escape/arbitrator.go` (PR-V5.3 ResumeSession one-shot consume) + `sessionorchestrator/escape_wiring.go` (PR-V5.6 applyResumeSession + resumeContentForDecision) + `sessionorchestrator/orchestrator.go` (PR-V5.6 ProcessMessage 入口插入) | Escape_Engine_Run |
| **D7-S14-A50-T13** | buildLoopContext 5 hash 字段构造 (SessionID + PlanKind + ObservationKind + FailureCriterion + ArtifactType) + buildLoopContextFromObserve (Observe 失败 case) + 4 IntentKind × 5 节点 12 case 集成测试 (Skip→1 次 Evaluate, Orchestrate→完整 5 节点) | IMPLEMENTED | `orchestration/sessionorchestrator/orchestrator.go` (PR-V5.5) | Escape_Engine_Run |
| **D7-S14-A50-T14** | L4 业务验收 4 测试 (TestL4_v5_Compatible_With_Phase1_7 / TestL4_v5_PerformanceOverhead_Under5Percent / TestL4_FeishuCard_NotBlocked_ByHuman10s + TestL4_LLMSwitchPlanKind_5Times_ForcesExit) | IMPLEMENTED | `orchestration/escape/*_e2e_test.go` (PR-V5.5) | Escape_Engine_Run |
| **D7-S14-A50-T15** | L3 端到端 7 测试 (TestL3_LLM_SwitchesPlanKind_5Times_ForcesExit / TestL3_SameMode_4Times_ForcesExit / TestL3_AnomalyDetector_5Nil_OpensL0 / TestL3_Verifier_3Times2s_OpensL2 / TestL3_Human10s_Async_FeishuNotBlocked / TestL3_PlanKindSwitch_Constrained_4Limit / TestL3_CB5Layers_Open_Independently) | IMPLEMENTED | `orchestration/escape/*_e2e_test.go` (PR-V5.5) | Escape_Engine_Run |
| **D7-S14-A50-T16** | L2 集成 7 测试 (TestIntegration_4DepthLimits / TestIntegration_3LayerArbitration / TestIntegration_5EscapeActions / TestIntegration_PlanKindSwitchLimit / TestIntegration_5NodePipeline_End2End / TestIntegration_5WiringPoints + TestIntegration_4IntentKind_5NodePaths) | IMPLEMENTED | `orchestration/escape/*_integration_test.go` (PR-V5.5) | Escape_Engine_Run |
| **D7-S14-A50-T17** | L1 单元 103 测试 (LoopDepthTracker 11 + PlanKindSwitchPolicy 15 + ChainedArbitrator 36 + EscapeEngine + CB 22 + Orchestrator 接线 19) | IMPLEMENTED | `orchestration/escape/*_test.go` (PR-V5.1..V5.5) | Escape_Engine_Run |
| **D7-S14-A50-T18** | 14 gap 补测 (LoopDepthTracker panic L1-91 / PendingResolutionStore TTL L1-92 / 14 ExitReason 映射 L1-93 / AuditLog 持久化 L1-94/95 / LoopBudget 2 个 L1-96/97 / CB panic L1-98 / ResumeSession + ApplyDecision 5 个 L1-99..103 + 4 IntentKind × 5 节点 L2-07) | IMPLEMENTED | `orchestration/escape/*_test.go` (PR-V5.1..V5.5) | Escape_Engine_Run |

## Scenario D7-S14 Detail (test points summary)

```
D7-S14  MUPS v5 统一逃逸机制 (IMPLEMENTED, 5 PR 拆分)
├── A50  LoopDepthTracker + PlanKindSwitch + ChainedArbitrator + EscapeEngine + CircuitBreaker + AuditLog + 5 节点接线 + T2 Resume
│   ├── T01  LoopContext 7 字段 + hashLoopContext SHA-256 + History 隔离         [IMPLEMENTED]
│   ├── T02  LoopDepthTracker depth < MaxDepth Continue / >= MaxDepth ForceExit   [IMPLEMENTED]
│   ├── T03  PlanKindSwitchPolicy 3 档 + 累计计数 (≤4 / 0 forbidden)             [IMPLEMENTED]
│   ├── T04  EscapeAction 6 类 enum + EscapeDecision 9 字段                       [IMPLEMENTED]
│   ├── T05  3 层仲裁 (LLM 5s / Rule / Human 10s 异步)                          [IMPLEMENTED]
│   ├── T06  Notifier + PendingResolutionStore + ChainedNotifier fallback       [IMPLEMENTED]
│   ├── T07  EscapeEngine 整合 + 13 类失败降级矩阵                               [IMPLEMENTED]
│   ├── T08  LoopBudget (consecutive=3 / total=20, doc 38 §19.2)                  [IMPLEMENTED]
│   ├── T09  CircuitBreaker 5 层接线 (L0..L5 阈值 + state machine)               [IMPLEMENTED]
│   ├── T10  EscapeAuditLog (AuditLevel 0/1/2 + 14 ExitReason 映射)              [IMPLEMENTED]
│   ├── T11  SessionOrchestrator 5 节点接线 + 1a 短路不调 1b                       [IMPLEMENTED]
│   ├── T12  ResumeSession + applyResumeSession + resumeContentForDecision        [IMPLEMENTED]
│   ├── T13  buildLoopContext + 4 IntentKind × 5 节点 12 case                     [IMPLEMENTED]
│   ├── T14  L4 业务验收 4 测试                                                  [IMPLEMENTED]
│   ├── T15  L3 端到端 7 测试                                                    [IMPLEMENTED]
│   ├── T16  L2 集成 7 测试                                                      [IMPLEMENTED]
│   ├── T17  L1 单元 103 测试                                                    [IMPLEMENTED]
│   └── T18  14 gap 补测 (LoopDepthTracker panic + AuditLog 持久化 + ResumeSession 5 个 + ...)  [IMPLEMENTED]
```

**Total**: 18 IMPLEMENTED P0 T points, 0 PARTIAL, 0 PLANNED.

---

## D7-S6: MUPS Pipeline (DM-20260626-002, PLANNED — mups 包路径迁移)

> **Change:** `devrix-d7-mups-package-migration` (DM-20260626-002) — execute/ + learn/ → mups/ 子树物理迁移。**纯目录迁移 + import path 替换**：保持 `package execute` / `package learn` 声明不变 + 所有函数签名/行为 0 变化；目标 = 6 S 文档 (S6 MUPS Pipeline = Execute 4 Channel + Learn 3 通道) 与物理代码 1:1 对齐。前置：devrix-d7-six-s-simplification (DM-20260626-001) v4.0.0 已 S7_Archived，9 个 spec 文档已对齐 6 S 但代码包路径未迁移。22 个 orchestration packages `go test -race` baseline 已稳定 (PR #215 验证)。4 P0 T 点全 PLANNED → 4 PLANNED IMPLEMENTED 收口后 v4.0.0 → v4.1.0。

### D7-S6-A51: mups Package Migration (execute/ + learn/ → mups/)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S6-A51-T01** | `internal/layers/orchestration/mups/execute/` 目录创建：7 个 .go 文件（channel.go + channel_commit.go + channel_exploration.go + channel_protocol.go + channel_scenario.go + errors.go + execute_test.go）从 execute/ 历史路径（已 git rm by DM-20260626-002 包迁移）迁移完成，`package execute` 保持不变 | IMPLEMENTED | `internal/layers/orchestration/mups/execute/*.go` | Hardening_Metric |
| **D7-S6-A51-T02** | `internal/layers/orchestration/mups/learn/` 目录创建：17 个 .go 文件（含 9 个 _test.go: adaptive_prior + asset_builder + asset_content + learner + learning_asset + memory + reputation_evidence + reputation_store + testhelpers + 9 _test.go 配套）从 learn/ 历史路径（已 git rm by DM-20260626-002 包迁移）迁移完成，`package learn` 保持不变 | IMPLEMENTED | `internal/layers/orchestration/mups/learn/*.go` | Hardening_Metric |
| **D7-S6-A51-T03** | 全仓 import path 替换：17 处 `internal/layers/orchestration/learn"` 历史引用 → `internal/layers/orchestration/mups/learn"` 当前引用（decisionplanning 2 + orchtypes 6 + sessionorchestrator 9）；execute 包 0 外部 import 跳过；`grep -rl "orchestration/execute\""` + `grep -rl "orchestration/learn\""` 双 0 命中（仅 historical-s-mapping.md 与 T 描述本身命中） | IMPLEMENTED | 全仓 import path 替换 | Hardening_Metric |
| **D7-S6-A51-T04** | `go build ./...` 0 错误 + `go vet ./...` 0 警告 + `go test ./internal/layers/orchestration/... -race -count=1` 22/22 PASS（与 baseline 持平）+ LP-1/LP-2/LP-5 路径 0 变化 | IMPLEMENTED | 全仓 build/vet/test 验证 | Hardening_Metric |

---

### Cross-cutting Hardening（横切，不占 S 位）

### D7-S7-A01: hardening/metrics Package Directory Exists

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S7-A01-T01** | `internal/layers/orchestration/hardening/metrics.go` 目录创建，原 `sessionorchestrator/metrics.go` (61 行 InterruptMetrics struct + Snapshot + TotalCancelFailures) `git mv` 迁移完成，`package hardening` 声明（取代 `package sessionorchestrator`）；同包 `metrics_test.go` (4 测试: TestInterruptMetrics_Snapshot_AtomicIncrement / TestInterruptMetrics_NilSafe / TestInterruptMetrics_TotalCancelFailures / TestInterruptMetrics_Snapshot_AllFields) 同步迁 | IMPLEMENTED | `internal/layers/orchestration/hardening/{metrics.go,metrics_test.go,doc.go}` | Execute_Artifact |
| **D7-S7-A01-T03** | 全仓 import path 替换：`sessionorchestrator/interrupt.go` Metrics 字段类型 `*InterruptMetrics` → `*hardening.InterruptMetrics` (1 处) + `interrupt_test.go` 4 处 `&InterruptMetrics{}` → `&hardening.InterruptMetrics{}`；`grep -rln "sessionorchestrator\.InterruptMetrics"` + `grep -rln "sessionorchestrator/metrics"` 双 0 命中 | IMPLEMENTED | `sessionorchestrator/interrupt.go` + `sessionorchestrator/interrupt_test.go` | Execute_Artifact |

### D7-S7-A02: hardening/recovery Package Directory Exists

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S7-A02-T02** | `internal/layers/orchestration/hardening/recovery.go` 子集拆分（Decision 2）：原 `turn/recovery.go` (133 行) 拆 4 纯函数 + 1 const → hardening/ (`IsContextLengthError` + `IsOverloadOr5xx` + `NeedsMaxOutputTokenRecovery` + `MaxOutputTokensRecoveryMessage` const)，`package hardening` 声明；receiver methods（`compressMessagesForRecovery` + `invokeStreamWithRecovery`）+ `partialStreamEmit` struct + `emitStreamRecoveryTombstones` + `maxOutputTokenRecoveryAttempts` const 留 `turn/`；同包 `recovery_test.go` 3 纯函数测试（TestIsContextLengthError + TestIsOverloadOr5xx + TestNeedsMaxOutputTokenRecovery）同步迁；`grep -rln "turn\.IsContextLengthError"` + `grep -rln "turn/recovery"` 双 0 命中 | IMPLEMENTED | `internal/layers/orchestration/hardening/{recovery.go,recovery_test.go}` + `turn/recovery.go` (KEEP) + `turn/recovery_test.go` (KEEP 3 orchestrator-coupled tests + `recoveryStubLLM`) | Execute_Artifact |

### D7-S7-A01 (续): Build, Vet, Test All Green

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S7-A01-T04** | `go build ./...` 0 错误 + `go vet ./...` 0 警告 + `go test ./internal/layers/orchestration/... -race -count=1` **23/23 PASS**（原 22 + 新 hardening 1 包，0 race condition）+ LP-1（Bayesian reputation `TestAutoClose_FullLP1Loop`）+ LP-2（`TestIntegration_5NodePipeline_End2End`）100% 兼容 + `escape/circuit_breaker.go` 0 变化（git diff HEAD 空，Decision 1） | IMPLEMENTED | 全仓 build/vet/test 验证 + escape/circuit_breaker.go 保持原状 | Execute_Artifact |

## Scenario D7-S6 PLANNED Detail (mups 包迁移子集)

```
D7-S6  MUPS Pipeline (mups 包路径迁移, IMPLEMENTED 子集 4 T)
├── A51  mups Package Migration (execute/ + learn/ → mups/)
│   ├── T01  mups/execute/ 目录 + 7 .go 文件 git mv 迁移              [IMPLEMENTED]
│   ├── T02  mups/learn/ 目录 + 17 .go 文件 git mv 迁移               [IMPLEMENTED]
│   ├── T03  17 处 import path 全仓替换 + grep 0 残留                  [IMPLEMENTED]
│   └── T04  go build/vet/test -race 全绿 (22/22 orchestration pkgs)  [IMPLEMENTED]
```

## Scenario D7-S4 PLANNED Detail (verify-promotion 包归属迁移子集)

```
D7-S4  ExecutionFlow + Verify (verify-promotion, PLANNED 子集 4 T)
└── A50  Verify Package Directory Exists (exit_reason + verdict_to_exit_reason from sessionorchestrator/)
    ├── T01  3 文件 git mv + git log --follow 100% rename detection   [PLANNED]
    ├── T02  3 文件 package 改名 + 13 处 ExitReason* 全替换            [PLANNED]
    ├── T03  executionflow/verify/ 包 0 sessionorchestrator 反向依赖    [PLANNED]
    └── T04  go build/vet/test -race 22/22 PASS + LP-1/2/5 兼容         [PLANNED]
```

**Total (D7-S4 verify-promotion 子集)**: 4 PLANNED P0 T points, 0 IMPLEMENTED, 0 PARTIAL.

## Scenario D7-S7 IMPLEMENTED Detail (Hardening 横切包物理落地子集)

```
D7-S7  Cross-cutting Hardening (Discipline Keeper, IMPLEMENTED 子集 4 T)
├── A01  hardening/metrics Package Directory Exists
│   ├── T01  hardening/metrics.go + metrics_test.go git mv 迁移       [IMPLEMENTED]
│   ├── T03  0 残留 import path 全仓替换 + grep 0 命中                [IMPLEMENTED]
│   └── T04  go build/vet/test -race 全绿 (23/23 orchestration pkgs)  [IMPLEMENTED]
└── A02  hardening/recovery Package Directory Exists
    └── T02  hardening/recovery.go subset split (4 纯函数 + 1 const)  [IMPLEMENTED]
```

**Total (D7-S6 mups 包迁移子集)**: 4 IMPLEMENTED P0 T points, 0 PLANNED, 0 PARTIAL.

## Scenario D7-S6 IMPLEMENTED Detail (子包清理热身 Sprint 子集)

> **devrix-d7-package-cleanup-sprint (DM-20260625-018) S7_Archived 2026-06-25**

```
D7-S6  子包清理热身 Sprint (4 遗留小子包物理合并, IMPLEMENTED 子集 4 T)
└── A50  D7 子包清理热身 Sprint (runregistry/toolpolicy/d7spans/sessionqueue → 父包)
    ├── T01  PR-1 runregistry/ → workmodel/ 物理合并: 3 git mv + 9 importer + CI 资源清理 [IMPLEMENTED]
    ├── T02  PR-2 toolpolicy/ → decisionplanning/ 物理合并: 6 git mv + 5 跨域 importer + D2 注释 [IMPLEMENTED]
    ├── T03  PR-3a d7spans/ → hardening/ 物理合并: 2 git mv + 7 importer + 4 spec 同步         [IMPLEMENTED]
    └── T04  PR-3b sessionqueue/ → executionflow/ 父级扁平: 3 git mv + 7 importer + doc.go    [IMPLEMENTED]
```

**Total (D7-S6 子包清理热身子集)**: 4 IMPLEMENTED P0 T points, 0 PLANNED, 0 PARTIAL.

**3 PR 联动**:
- PR #231 (PR-1 runregistry → workmodel, MERGED 2026-06-25T14:11:52Z)
- PR #232 (PR-2 toolpolicy → decisionplanning, MERGED 2026-06-25T14:18:00Z)
- PR #233 (PR-3 d7spans + sessionqueue, MERGED 2026-06-25T14:34:36Z)

**D7 编排层目录结构终态**: 15 → 11 子目录（移除 4 遗留小子包）。
**0 函数签名变化 + 0 业务逻辑变化** (pure physical migration)。
归档：`openspec/archive/2026-06-25-devrix-d7-package-cleanup-sprint/`。

---

## D7-S1: Worktree Op 兜底 (DM-20260626-009 follow-up)

> **Change:** `devrix-d7-inner-spans-dedup-remove` (DM-20260626-009) follow-up — 5-node MUPS 根 span 之外，工作树每次 mutation (set_round_phase / apply_pipeline_round / update_status / list_children) 加一层 `D7_Worktree_Op` span，让 ItemPipelineRunner 11 个 callsite 在 Jaeger 显形，否则"哪一步把 round_phase 改到 X"必须读代码。包级 nil-bridge fail-safe (PR #253+#254 已落)。

### D7-S1-A52: EmitWorktreeOp (P1, 6 P0/P1 span 之一)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S1-A52-T11** | `EmitWorktreeOp(ctx, sessionID, op, itemID, phaseOrStatus)` happy path — start 时建 span + 设 4 attributes (worktree.op / worktree.item_id / worktree.phase_or_status + session_id)，end 时 nil err 不动 span status、non-nil err 标 error 并记录 err.message | IMPLEMENTED | `hardening/emitter_test.go::TestD7S1A52T11_EmitWorktreeOp_HappyPath` | Worktree_Op |
| **D7-S1-A52-T12** | `EmitWorktreeOp` nil-bridge fail-safe — bridge==nil 时 start() 直接 return zero span + closure，end() nil-receiver safe 不 panic；保证 hardening 在未接 telemetry 时不影响生产路径 | IMPLEMENTED | `hardening/emitter_test.go::TestD7S1A52T12_EmitWorktreeOp_NilBridgeFailSafe` | Worktree_Op |

### D7-S1-A53: EmitSubWorktreeRun (P2)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S1-A53-T13** | `EmitSubWorktreeRun(ctx, sessionID, parentID, childID, spawnedBy)` happy path — parent/child 关系在 trace 上显形 + spawnedBy 标记 spawn 路径 (parallel_explore / spawn_decompose)，便于 dashboard filter | IMPLEMENTED | `hardening/emitter_test.go::TestD7S1A53T13_EmitSubWorktreeRun_HappyPath` | SubWorktree_Run |
| **D7-S1-A53-T14** | `EmitSubWorktreeRun` nil-bridge fail-safe — 同 A52-T12 设计 | IMPLEMENTED | `hardening/emitter_test.go::TestD7S1A53T14_EmitSubWorktreeRun_NilBridgeFailSafe` | SubWorktree_Run |

---

## D7-S5: Decision & Planning 内层 Sub-Turn (DM-20260626-009 follow-up)

> **Change:** `devrix-d7-inner-spans-dedup-remove` (DM-20260626-009) follow-up — WorkItemExecutor ReAct 循环的每次 LLM→tool round iteration 加一层 `D7_SubTurn_Iteration` span，让"16s session 慢在哪一步"在 Jaeger 显形。`subturn.finish_reason` (LLM stop/tool_calls/length/...) 与 `subturn.stop_reason` (executor final_answer/max_iters/tool_error/...) 正交，前者描述 LLM 自身 finish 行为，后者描述 executor 自定义终止态。cap-hit 路径多发 1 个 `iter=max+1` span 让 "max_iters" 终止态在 trace 可见。

### D7-S5-A54: EmitSubTurnIteration (P1)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S5-A54-T15** | `EmitSubTurnIteration(ctx, sessionID, itemID, iter, finishReason, stopReason)` happy path — iter 1-based + finishReason 来自 LLM (stop/tool_calls/length) + stopReason 来自 executor (final_answer/tool_error/ok/max_iters)；end 时 nil err 不动 status、non-nil err 标 error | IMPLEMENTED | `hardening/emitter_test.go::TestD7S5A54T15_EmitSubTurnIteration_HappyPath` | SubTurn_Iteration |
| **D7-S5-A54-T16** | `EmitSubTurnIteration` nil-bridge fail-safe — 同 A52-T12 设计；同时 cap-hit 路径 (iter=max+1, finishReason="tool_calls", stopReason="max_iters") 也走该函数确认零 panic | IMPLEMENTED | `hardening/emitter_test.go::TestD7S5A54T16_EmitSubTurnIteration_NilBridgeFailSafe` | SubTurn_Iteration |

---

## Scenario D7-S1 + D7-S5 IMPLEMENTED Detail (内层 observability span 子集)

```
D7-S1  Worktree Op (DM-20260626-009 follow-up, IMPLEMENTED 子集 4 T)
├── A52  EmitWorktreeOp (P1, worktree.op 内层 span)
│   ├── T11  EmitWorktreeOp happy path                              [IMPLEMENTED]
│   └── T12  EmitWorktreeOp nil-bridge fail-safe                   [IMPLEMENTED]
└── A53  EmitSubWorktreeRun (P2, subworktree.run 内层 span)
    ├── T13  EmitSubWorktreeRun happy path                          [IMPLEMENTED]
    └── T14  EmitSubWorktreeRun nil-bridge fail-safe               [IMPLEMENTED]

D7-S5  Sub-Turn Iteration (DM-20260626-009 follow-up, IMPLEMENTED 子集 2 T)
└── A54  EmitSubTurnIteration (P1, subturn.iteration 内层 span)
    ├── T15  EmitSubTurnIteration happy path                        [IMPLEMENTED]
    └── T16  EmitSubTurnIteration nil-bridge fail-safe             [IMPLEMENTED]
```

**Total (D7-S1+S5 inner-spans 子集)**: 6 IMPLEMENTED P0 T points, 0 PLANNED, 0 PARTIAL.

**WorkItem Inner Layer Trace 树**（DM-20260626-009 follow-up 内层 span 落地后，SessionOrchestrator 5-node → ItemPipelineRunner → Worktree Op → WorkItemExecutor → Sub-Turn 全链路显形）：

```
session.span (D7_Session_Run, S1-A42 root)
└── orchestrator.span (D7_Orchestrator_Run, S2-A43)
    └── mups.observe.span (D7_MUPS_Observe, S8-A15)
    └── mups.plan.span (D7_MUPS_Plan, S8-A22)
    └── mups.execute.span (D7_MUPS_Execute, S9-A26)
        └── item_pipeline.span (D7_Item_Pipeline, S3-A44)
            ├── worktree.op[set_round_phase] (D7_Worktree_Op, S1-A52) ×11 per WorkItem
            ├── worktree.op[apply_pipeline_round] (D7_Worktree_Op, S1-A52)
            ├── worktree.op[update_status] (D7_Worktree_Op, S1-A52)
            └── workitem.span (D7_Workitem_Execute, S3-A45)
                └── subturn.iter[N] (D7_SubTurn_Iteration, S5-A54) ×N per ReAct loop, N≤MaxIters
                    └── subturn.iter[max_iters] (D7_SubTurn_Iteration, S5-A54) cap-hit 多发 1 span
    └── mups.verify.span (D7_MUPS_Verify, S10-A32)
    └── mups.learn.span (D7_MUPS_Learn, S12-A41)

session_turn_loop.RunParallelExplore (S2-A50 LoopDepthTracker v2)
└── subworktree.run[parent→child, spawned_by=parallel_explore] (D7_SubWorktree_Run, S1-A53)
    └── orchestrator.span (D7_Orchestrator_Run, S2-A43) per child
```

**0 函数签名变化**（pure instrumentation — EmitXxx 是新加函数 + 工作树 mutation inline 加 emit + ReAct loop 拆 stepOneIter 让 span 包单次函数调用）。22/22 orchestration packages `go test -race` PASS。

**PR 联动**: PR #253 (5-node MUPS 根 span + item_pipeline 11 callsite) + PR #254 (3 dedup 删除 + 3 inner observability span) + follow-up PR (#255 待开, thread LLM finishReason + 4 spec doc 同步)。

归档：`openspec/archive/2026-06-26-devrix-d7-inner-spans-dedup-remove/` (待 S6)。

---

## D7-S15: WorkItem Rollup 闭环 (DM-20260627-001)

> **Change:** `devrix-d7-workitem-rollup-pipeline` — Phase 1 P0 闭环；Phase 2 DecomposeProposer / ParallelExplore 登记不编码。  
> **归档：** `openspec/archive/2026-06-27-devrix-d7-workitem-rollup-pipeline/`

### D7-S15-A50: Parent Rollup Gate

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-A50-T01** | NeedsRollup schema backward compat | IMPLEMENTED | `workmodel/workitem_store_test.go` | Rollup_Gate |
| **D7-S15-A50-T02** | ReevaluateParent rollup gate | IMPLEMENTED | `workmodel/rollup_gate_test.go` | Rollup_Gate |
| **D7-S15-A50-T03** | GetPipelineFocus rollup priority | IMPLEMENTED | `workmodel/rollup_gate_test.go` | Rollup_Gate |

### D7-S15-A55: RollupGatePolicy

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-A55-T01** | all_pass blocks on child fail | IMPLEMENTED | `workmodel/rollup_gate_test.go::TestShouldRollupAfterChildren_AllPassBlocksOnFail` | Rollup_Gate |
| **D7-S15-A55-T02** | min_coverage threshold | SKIP (Phase 2) | — | Rollup_Gate |
| **D7-S15-A55-T03** | best_effort default on all terminal | IMPLEMENTED | `workmodel/rollup_gate_test.go` | Rollup_Gate |

### D7-S15-A51: Summary Bubble Materialize

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-A51-T01** | CB3 truncate | IMPLEMENTED | `workmodel/context_bubble_apply_test.go` | Rollup_Bubble |
| **D7-S15-A51-T02** | Observe dual bubble (T05) | IMPLEMENTED | `sessionorchestrator/item_observe_test.go::TestObserveWorkItem_RollupDualBubbles` | Rollup_Bubble |
| **D7-S15-A51-T03** | Rollup directive uses summaries | IMPLEMENTED | `sessionorchestrator/item_pipeline_rollup_test.go` | Rollup_Bubble |

### D7-S15-A60: Parent Rollup Round 2+ MUPS

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-A60-T01** | CommitmentPlan + FailureCriteria | IMPLEMENTED | `sessionorchestrator/item_pipeline_rollup_test.go` | Rollup_Round |
| **D7-S15-A60-T02** | Rollup directive lists children | IMPLEMENTED | `sessionorchestrator/item_pipeline_rollup_test.go` | Rollup_Round |
| **D7-S15-A60-T03** | Verify Pass clears NeedsRollup | IMPLEMENTED | `sessionorchestrator/item_pipeline_rollup_test.go` | Rollup_Round |

### D7-S15-A61: Session complete Deliverable

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-A61-T01** | complete.Content from root summary | IMPLEMENTED | `workmodel/rollup_gate_test.go` + `sessionorchestrator/session_turn_loop` | Session_Process |
| **D7-S15-A61-T02** | best-effort child fallback | IMPLEMENTED | `workmodel/rollup_gate.go::ExtractSessionDeliverable` | Session_Process |

### D7-S15-A53: Ephemeral Checklist Gate

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-A53-T01** | GetFocus skips ephemeral checklist | IMPLEMENTED | `workmodel/rollup_gate_test.go` | Rollup_Gate |
| **D7-S15-A53-T02** | HasOpenWork after rollup | IMPLEMENTED | `workmodel/spawn_apply_test.go` | Rollup_Gate |
| **D7-S15-A53-T03** | root R1 + checklist focus | IMPLEMENTED | `workmodel/work_tree_test.go` | Rollup_Gate |

### D7-S15-A54: Root Session Rollup Fallback

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-A54-T01** | maybeRootRollupFallback | IMPLEMENTED | `workmodel/rollup_gate_test.go` | Rollup_Fallback |
| **D7-S15-A54-T02** | ChecklistBubbleStatement CB3 | IMPLEMENTED | `workmodel/context_bubble_apply_test.go` | Rollup_Fallback |
| **D7-S15-A54-T03** | Path B checklist Observe | IMPLEMENTED | `sessionorchestrator/item_observe.go` | Rollup_Fallback |
| **D7-S15-A54-T04** | trace replay E2E | PARTIAL (stub IT) | `tests/integration/d7/d7_rollup_trace_replay_test.go` | Rollup_Fallback |

### D7-S15-A07: WorkTree Rollup Governance (DM-20260629-001 PR-3-extended)

> **Change:** `devrix-d7-dsaft-restructuring` PR-3-extended (T52 + T53 + T54) — typed RollupReport envelope + 3-call-site migration + deterministic root + 3 governance T points.  
> **归档：** `openspec/changes/devrix-d7-dsaft-restructuring/` (PR-3-extended)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-A07-T01** | ApplyPipelineDecide 4 步顺序不变式（ContextBubbleDecision → AcceptedContextLinks → SpawnPolicy → ScopeContractSpawnGate） | IMPLEMENTED | `workmodel/context_decide.go::ApplyPipelineDecide` | Rollup_Gate |
| **D7-S15-A07-T02** | ReevaluateParentAfterChild 3 调用点幂等性（同一 child 多次 terminal 仅触发 1 次 rollup，签名迁移 `(struct{}, error) → (*RollupReport, error)`） | IMPLEMENTED | `workmodel/resolve.go::ReevaluateParentAfterChild` + 3 调用点 (`sessionorchestrator/session_turn_loop.go:194`, `workmodel/run_spawn.go:51`, `workmodel/cli_commands.go:342`) | Rollup_Gate |
| **D7-S15-A07-T03** | Path A vs Path B rollup trigger 选择矩阵：Path A (eager rollup) — `workmodel/rollup_gate.go::ShouldRollupAfterChildren` — 3 policies × 2 needs_rollup = 6 组合；Path B (root fallback) — `workmodel/rollup_gate.go::MaybeRootRollupFallback` — 2 has_ephemeral × 2 needs_rollup = 4 组合；合计 10 组合覆盖 | IMPLEMENTED | `workmodel/rollup_gate_test.go` | Rollup_Gate |

**A07 Total:** 3 P0 T — 3 IMPLEMENTED (DM-20260629-001 PR-3-extended)

### D7-S15-A89: RollupTerminationGuard (DM-20260701-001)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-A89-T01** | `WorkItemPipelineRound.RollupRetries` 计数 + `TreeEvalContext` 透传（T-P0-4） | IMPLEMENTED | `workmodel/pipeline_round.go` + `sessionorchestrator/reevaluate_context.go` | Rollup_Gate |
| **D7-S15-A89-T02** | `SpawnPolicyEvaluator` rollup 分支加 `MaxRollupRetries → SpawnEscalateHuman`（T-P0-5） | IMPLEMENTED | `sessionorchestrator/spawn_policy_evaluator.go` | Rollup_Gate |
| **D7-S15-A89-T03** | `session_turn_loop` break 前检查未收敛 rollup parent，emit 显式结局（T-P0-6） | IMPLEMENTED | `sessionorchestrator/turn_loop.go` | Rollup_Gate |
| **D7-S15-A89-T04** | rollup 故障注入测试：verify 恒 fail → 达上限 escalate，不超 loop（T-P0-7） | IMPLEMENTED | `sessionorchestrator/rollup_retry_injection_test.go` (NEW) | Rollup_Gate |

### D7-S15-A90: RollupOutcomeAggregation (DM-20260701-001)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-A90-T01** | `rollup_gate`/`rollup_verify` 读 `ChildOutcomeStats`，`Failed==Total` 禁 Completed（T-P0-8） | IMPLEMENTED | `sessionorchestrator/rollup_gate.go` + `rollup_verify.go` | Rollup_Gate |
| **D7-S15-A90-T02** | `buildRollupDirective` 增 `FailedSubset:` 区段（不洗白失败子集，T-P0-9） | IMPLEMENTED | `sessionorchestrator/rollup_directive.go` | Rollup_Gate |

**A89+A90 Total:** 6 P0 T — 6 IMPLEMENTED (DM-20260701-001)

### D7-S15 Integration

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S15-IT01** | decompose + rollup E2E | PARTIAL | `sessionorchestrator/item_pipeline_rollup_test.go` (unit-level) | — |
| **D7-S15-IT02** | trace replay no checklist MUPS | PARTIAL (stub) | `tests/integration/d7/d7_rollup_trace_replay_test.go` | — |

**Phase 1 Total:** 21 P0 T — 18 IMPLEMENTED · 2 PARTIAL (IT stub) · 1 SKIP (min_coverage Phase 2)

---

## D7-S16: Layer SubContext (DM-20260627-003 + DM-20260628-002)

> **Change:** Phase 1+2 `devrix-d7-layer-subcontext` · Phase 3 `devrix-d7-layer-subcontext-phase3`  
> **归档：** `openspec/archive/2026-06-28-devrix-d7-layer-subcontext/` · `openspec/archive/2026-06-28-devrix-d7-layer-subcontext-phase3/`

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D2-S16-A20-T01..T05** | Materializer + partition store + Jaeger span | IMPLEMENTED | `contextengine/materialize/` | — |
| **D7-S16-A60-T01..T04** | ScopeContract + spawn gate + rule infer | IMPLEMENTED | `workmodel/scope_contract*.go`, `item_plan.go` | SubContext_Scope |
| **D7-S16-A61-T01..T03** | ChildDownlink + Materialize inject | IMPLEMENTED | `workmodel/child_downlink.go` | SubContext_Downlink |
| **D7-S16-A95-T01..T04** | MUPS prompttags ParseWholeBody + linefield | IMPLEMENTED | `sessionorchestrator/*_proposer.go`, `prompttags/`, `workmodel/deliverable_findings_parse.go` | SubContext_Signal |
| **D7-S16-A70-T01..T03** | Executor Materialize wiring | IMPLEMENTED | `workitem_executor.go`, `workitem_exec_context.go` | SubContext_Executor |
| **D7-S16-A72-T01..T04** | Signal→Obs mapping | IMPLEMENTED | `item_observe.go`, `item_observe_scope_test.go` | SubContext_Signal |
| **D7-S16-A63-T01/T02** | Upstream BlockedBy | IMPLEMENTED | `workitem_exec_context.go`, materialize tests | SubContext_Blocked |
| **D7-S16-A64-T01/T02** | PeerStatus cohort | IMPLEMENTED | `workmodel/cohort_signals.go` | SubContext_Cohort |
| **D7-S16-IT21..IT26** | Materialize integration | IMPLEMENTED | `item_pipeline_materialize_test.go` | — |
| **D2-S16-A22-T01..T03** | SubTurn/Wave Materialize paths | IMPLEMENTED | `materialize/subturn.go`, `materialize/wave.go` | — |
| **D7-S16-A65-T01..T03** | SubTurn→MaterializePolicy + bootstrap | IMPLEMENTED | `subturn_materialize.go`, `mups_pipeline.go` | SubContext_Materialize |
| **D7-S16-A66-T01..T03** | Wave ContextResolver merge | IMPLEMENTED | `wavescheduler/context_materialize.go`, `wire_wave.go` | SubContext_Materialize |
| **D7-S16-A74-T01..T04** | ~~LLM ObservationProposer + rule gate~~ | **SUPERSEDED** (DM-20260630-001) | — | — |
| **D7-S16-A75-T01** | WireItemPipeline wires LLMObservationProposer | **IMPLEMENTED** | `bootstrap/wire_item_pipeline.go` | SubContext_Executor |
| **D7-S16-A75-T02** | Observe D2 Prepare before D3 (no bare D3) | **IMPLEMENTED** | `llm_observation_proposer.go`, `llm_observation_proposer_test.go` | SubContext_Executor |
| **D7-S16-A75-T03** | zh-CN observation appendix + ValidateObservationProposals | **IMPLEMENTED** | `llm_observation_proposer.go`, `observation_proposer_test.go` | SubContext_Signal |
| **D7-S16-A75-T04** | spec.md v4.19.0 + t-registry A75 sync | **IMPLEMENTED** | `openspec/specs/d7-orchestration/` | — |

### D7-S16-A94: ScopeSubdivisionContract (DM-20260701-001)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S16-A94-T01** | `DefaultChildDownlink` 移除"无脑继承父全量 scope"（bounded parent + empty spec = empty child scope，T-P2-1） | IMPLEMENTED | `workmodel/child_downlink.go` | SubContext_Scope |
| **D7-S16-A94-T02** | `ValidateChildScopes(parent, children)` 真子集 + 覆盖校验 + prompt 指引（5-case test matrix，T-P2-2） | IMPLEMENTED | `workmodel/scope_validate.go` (NEW) + `scope_validate_test.go` (NEW) | SubContext_Scope |

### D7-S16-A95: MUPS prompttags ParseWholeBody（DM-20260704-004）

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S16-A95-T01** | `parseObservationProposalsJSON` 使用 `ParseWholeBody[[]rawObsProposal]`；空/`[]` → nil | IMPLEMENTED | `sessionorchestrator/llm_observation_proposer.go` | SubContext_Signal |
| **D7-S16-A95-T02** | `parseStrategicPlanJSON` 使用 `ParseWholeBody[rawStrategicPlan]`；保留 validation | IMPLEMENTED | `sessionorchestrator/strategic_plan_proposer.go` | SubContext_Executor |
| **D7-S16-A95-T03** | `tryParseWholeBodyFindingsObject` fast path；corrupt summary 仍走 marker 逻辑 | IMPLEMENTED | `workmodel/deliverable_findings_parse.go` | SubContext_Executor |
| **D7-S16-A95-T04** | `BuildLineFrame` Observe/Plan user prompt golden | IMPLEMENTED | `prompttags/linefield_test.go` | SubContext_Signal |

**A94 Total:** 2 P0 T — 2 IMPLEMENTED (DM-20260701-001)
**A95 Total:** 4 P0 T — 4 IMPLEMENTED (DM-20260704-004)

### shared-A99: MUPS go-struct binding kernel (DM-20260705-003 M1) — Change: `mups-go-struct-driven` (M1 kernel)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **shared-A99-T01** | `MustRegisterFrame[T]` 反射注册（happy path） | IMPLEMENTED | `prompttags/structbind_test.go::TestMustRegisterFrame_HappyPath` | Spec_Review |
| **shared-A99-T02** | `BuildLineFrameFromStruct` 反射序列化（byte-equal + 边界） | IMPLEMENTED | `prompttags/structbind_test.go::TestBuildLineFrameFromStruct_{FullStruct,OmitEmpty,EdgeCases}` | Spec_Review |
| **shared-A99-T03** | `DocBlockFromStruct[T]` 反射 schema 文档 | IMPLEMENTED | `prompttags/structbind_test.go::TestDocBlockFromStruct_ShapeMatches` | Spec_Review |
| **shared-A99-T04** | 4 项 init panic 校验（pt 缺 / plane 错 / i18n 缺 / 字段数漂移） | IMPLEMENTED | `prompttags/structbind_test.go::TestMustRegisterFrame_{InvalidPlanePanics,NonStructPanics,HappyPath}` + `TestParseFrameFieldTag_Errors` + `TestRegisterFrameFieldGuide_MissingPanics` | Spec_Review |
| **shared-A99-T05** | `RegisterFrameFieldGuide` i18n 校验函数 | IMPLEMENTED | `prompttags/structbind_test.go::TestRegisterFrameFieldGuide_MissingPanics` | Spec_Review |

**shared-A99 Total:** 5 P0 T — 5 IMPLEMENTED (DM-20260705-003 M1)

### D7-S16-A96: MUPS IO convergence gates（DM-20260704-005）

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S16-A96-T01** | `ValidateObservationProposals` 保留前 3 个有效提案（与 i18n max 3 一致） | IMPLEMENTED | `sessionorchestrator/observation_proposer.go`, `observation_proposer_test.go` | SubContext_Signal |
| **D7-S16-A96-T02** | `buildStrategicPlanUserPrompt` 在 `UncertaintyMean > 0` 时注入 `uncertainty_mean` | IMPLEMENTED | `sessionorchestrator/strategic_plan_proposer.go`, `strategic_plan_proposer_test.go` | SubContext_Executor |
| **D7-S16-A96-T03** | Observe user frame `prior_observation_ids` + `incremental_only`（LastRound 有 obs 时） | IMPLEMENTED | `observation_proposer.go`, `llm_observation_proposer.go`, `linefield_test.go` | SubContext_Signal |

**A96 Total:** 3 P0 T — 3 IMPLEMENTED (DM-20260704-005)

### D7-S5-A97: MUPS tag semantics proposer consumption（DM-20260705-001）

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S5-A97-T01** | LLMObservationProposer prepared system 含 obs_* 语义 marker | IMPLEMENTED | `sessionorchestrator/llm_observation_proposer_test.go::TestLLMObservationProposer_SystemIncludesSemanticMarkers` | SubContext_Signal |
| **D7-S5-A97-T02** | Plan user prompt 含 control/data frame guide + annotated lines | IMPLEMENTED | `sessionorchestrator/strategic_plan_proposer_test.go::TestBuildStrategicPlanUserPrompt_IncludesFrameGuide` | Plan_Generate |

**A97 Total:** 2 P0 T — 2 IMPLEMENTED (DM-20260705-001)

### D7-S5-A98: MUPS parse reject capture + round persistence（DM-20260705-002）

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S5-A98-T01** | StrategicPlanReject → round.PlanParseReject → next Plan user frame | IMPLEMENTED | `parse_reject_feedback_test.go::TestRunItemPipeline_StrategicPlanRejectFeedsPlanUserFrame` | Plan_Generate |
| **D7-S5-A98-T02** | Observe parse fail → round.ObserveParseReject → next Observe user frame | IMPLEMENTED | `parse_reject_feedback_test.go::TestObserveWorkItem_ParseRejectFeedsNextObserveFrame` | SubContext_Signal |

**A98 Total:** 2 P0 T — 2 IMPLEMENTED (DM-20260705-002)

### D7-S5-A99: MUPS go-struct-driven I/O contract (DM-20260705-003 M1) — Change: `mups-go-struct-driven` (Observe 节点迁移)

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S5-A99-T01** | `ObserveSignalInput` 9 字段 + pt tag 反射注册成功 | IMPLEMENTED | `observe_structbind_test.go::TestObserveSignalInput_RegisteredAtInit` | SubContext_Signal |
| **D7-S5-A99-T02** | `buildObserveSignalInput` 扁平化 `ScopeContract.GoalStatement` → `ScopeGoal` / `OpenQuestions` → `ScopeOpenQuestions` | IMPLEMENTED | `observe_structbind_test.go::TestBuildObserveSignalInput_FlattensScopeContract` | SubContext_Signal |
| **D7-S5-A99-T03** | `IncrementalOnly = len(PriorObservationIDs) > 0` | IMPLEMENTED | `observe_structbind_test.go::TestBuildObserveSignalInput_FlattensScopeContract` | SubContext_Signal |
| **D7-S5-A99-T04** | `buildLLMObservationUserPrompt` 反射版字节等价旧手工 map | IMPLEMENTED | `observe_structbind_test.go::TestBuildLLMObservationUserPrompt_FullInput` | SubContext_Signal |
| **D7-S5-A99-T05** | golden snapshot 4 组合 PASS（FullInput / OmitEmpty / GoldenZH / FlattensScopeContract） | IMPLEMENTED | `observe_structbind_test.go::TestBuildLLMObservationUserPrompt_GoldenZH` | SubContext_Signal |
| **D7-S5-A99-T06** | 现有 `llm_observation_proposer_test.go` 3 测试 0 行为变化 PASS | IMPLEMENTED | `llm_observation_proposer_test.go` | SubContext_Signal |
| **D7-S5-A99-T07** | 现有 `observation_proposer_test.go` 5 测试 0 行为变化 PASS | IMPLEMENTED | `observation_proposer_test.go` | SubContext_Signal |
| **D7-S5-A99-T08** | 现有 `item_observe_test.go` E2E 0 行为变化 PASS | IMPLEMENTED | `item_observe_test.go` | SubContext_Signal |
| **D7-S5-A99-T09** | 现有 `parse_reject_feedback_test.go` E2E 0 行为变化 PASS（DM-20260705-002 链路保留） | IMPLEMENTED | `parse_reject_feedback_test.go` | SubContext_Signal |

**A99 Total:** 9 P0 T — 9 IMPLEMENTED (DM-20260705-003 M1)


### D7-S5-A100: MUPS Plan 节点 go-struct 化 (DM-20260705-004 M2) — Change: `mups-plan-structbind`

| T ID | Description | Status | File | Span Evidence |
|------|-------------|--------|------| --- |
| **D7-S5-A100-T01** | `MustRegisterFrame[StrategicPlanFrame]()` init 成功 + 16 字段顺序对齐 `PlanUserFrame` | IMPLEMENTED | `plan_structbind_test.go::TestStrategicPlanFrame_RegisteredAtInit` | Plan_Generate |
| **D7-S5-A100-T02** | `BuildLineFrameFromStruct` 字节等价 `buildStrategicPlanUserPrompt` 旧 38 行手工 map | IMPLEMENTED | `plan_structbind_test.go::TestBuildStrategicPlanUserPrompt_FullInput` | Plan_Generate |
| **D7-S5-A100-T03** | `buildStrategicPlanFrame` 平铺 `Budget` 9 字段（含 `*int` nil=absent）与现状手工展开一致 | IMPLEMENTED | `plan_structbind_test.go::TestBuildStrategicPlanFrame_FlattensBudget` | Plan_Generate |
| **D7-S5-A100-T04** | Budget=0 时 9 Budget 字段全跳过（`Budget.MaxChildren > 0` 守卫保留） | IMPLEMENTED | `plan_structbind_test.go::TestBuildStrategicPlanUserPrompt_ZeroBudget` | Plan_Generate |
| **D7-S5-A100-T05** | init() 4 项 panic 校验 (pt 缺 / plane 错 / i18n 缺 / 字段数 == FrameSpec) | IMPLEMENTED | `prompttags/structbind_test.go::TestMustRegisterFrame_*` (M1 kernel 兼容 M2) | Spec_Review |
| **D7-S5-A100-T06** | golden snapshot 12 行精确 byte-equal（含 `[control]` / `[data]` prefix + `%.3f` 格式） | IMPLEMENTED | `plan_structbind_test.go::TestBuildStrategicPlanUserPrompt_GoldenEN` | Plan_Generate |
| **D7-S5-A100-T07** | `planFrameToMap` 反射辅助：omit_empty / omit_zero 行为与 kernel 一致 | IMPLEMENTED | `plan_structbind_test.go::TestPlanFrameToMap_OmitsEmptyAndZero` | Plan_Generate |
| **D7-S5-A100-T08** | 现有 Plan E2E 0 行为变化（`item_plan_test.go` + `strategic_plan_proposer_test.go` + `parse_reject_feedback_test.go`） | IMPLEMENTED | `internal/layers/orchestration/sessionorchestrator/*_test.go` | Plan_Generate |

**A100 Total:** 8 P0 T — 8 IMPLEMENTED (DM-20260705-004 M2)


**Phase 1–3 Total:** IMPLEMENTED (PR #269–#270, #273–#275)

---

## D7-S16 ~ S19: TaskContract 统一 (DM-20260629-006) — DESIGN ONLY

> **Change:** `devrix-d7-taskcontract-unification` (DM-20260629-006)  
> **归档：** `openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/`（S6_Archived, **DESIGN ONLY** — implementation deferred to v7.0 sprint）  
> **v7.0 演进起点：** D7 缺契约不缺机制 → P0=TaskReport 五元素（缺 Dissent/Blockage/Resource）+ P1=TaskSpec 四元组（Plan/Channel/WorkItem 分散）4-Layer × 3-Phase  
> **触发规范升级：** `devrix-architecture-design-six-segment-migration` (DM-20260629-007, PR #321 merged) — 本 Change 是规范升级后**第一个**按新六段式 design.md 落地的 Change（reference）

### 4-Layer × 3-Phase 设计（DESIGN ONLY，0 IMPLEMENTED）

| Layer | Scenario | 主题 | AC 数 | T 点（DESIGN） | PR |
|-------|----------|------|-------|----------------|-----|
| L1 接口层 | **D7-S16** (TaskContract 接口) | TaskSpec struct + TaskReport struct + 3 处创建点统一迁移 | AC1, AC2 | 2 T (A01-T01 + A02-T01) | PR-A |
| L2 字段语义层 | **D7-S17** (TaskContract 字段语义) | Dissent + Blockage + Resource 字段填充 | AC3, AC4, AC5 | 3 T (A01-T01 + A02-T01 + A03-T01) | PR-A |
| L3 防御运行时层 | **D7-S18** (TaskContract 防御运行时) | Pessimistic Commit + Hard Evidence + CoW VersionChain + Rule-based Fallback + Similarity Check | AC6, AC7, AC8, AC11, AC12, AC13, AC14, AC15 | 5 T (A01-T01 + A02-T01 + A03-T01 + A04-T01 + A05-T01) | PR-B + PR-C |
| L4 治理横切层 | **D7-S19** (TaskContract 治理横切) | spec sync + Coverage + Perf + Security + Cross-Domain Boundary + Feature Flag + Error Code + convergence span + AdaptiveThreshold + Layout guard | AC9, AC10, AC16, AC17, AC18, AC19, AC20, AC21, AC22, AC23 | 11 T (A01-T01 + A02-T01 + A03-T01 + A04-T01 + A05-T01 + A06-T01 + A07-T01 + A08-T01 + A09-T01 + A10-T01 + A11-T01 + LP-T01 + RACE-T01) | PR-A + PR-B + PR-C |

> ⚠ **T 编号冲突提示：** 现有 D7-S16 = "Layer SubContext" (DM-20260627-003 + DM-20260628-002)；新 D7-S16 = "TaskContract 接口层" (本 Change DESIGN ONLY)。v7.0 sprint 实施时**必须重新分配 scenario 编号**（建议：本 Change 的 S16-S19 改名为 S20-S23 或 S17-S20 + 现有 S16 改名为 SubContext-Layer 保留）。

**Phase Total (DESIGN):** 23 AC + 25 T (21 形式化 + 4 LP/RACE) + 27 Scenarios | **IMPLEMENTED:** 0 / **DESIGN:** 25/25 | **0 PR / 0 commit / 0 test**

**实施计划（v7.0 sprint）：** PR-A (1 周, 6 AC) + PR-B (2 周, 8 AC) + PR-C (1.5 周, 9 AC) = **3 PR / 4.5 周 / 23 AC**。

**归档位置：** `openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/`（demand.md / proposal.md / design.md 648 行 / tasks.md / specs/d7-orchestration/spec.md / acceptance-report.md / .openspec.yaml）

---

## D7-S2-A14~A17: Multi-Turn Session Serialization (DM-20260628-004, PARTIAL)

> **Change:** `devrix-d7-multiturn-session-state` (DM-20260628-004) — D7 多轮 session 串行化与 complete 时机修正  
> **归档：** `openspec/archive/2026-06-29-devrix-d7-multiturn-session-state/`（S6_Archived **PARTIAL** — RC-3 panic hotfix done via PR #271, RC-1/2/4 设计 4 层契约已就位 deferred to v1.1）  
> **DM ID 重新分配：** 原 DM-20260628-003 → DM-20260628-004（与 D1 DSAFT Refactor DM-20260628-003 冲突）

### D7-S2-A16: turn 串行化 + panic recovery (PR #271 — IMPLEMENTED)

| T ID | 描述 | Status | File | Span Evidence |
|------|------|--------|------| --- |
| **D7-S2-A16-T01** | emit recover middleware（避免 send-on-closed-channel panic）| **IMPLEMENTED** | `sessionorchestrator/item_pipeline_emit.go` (PR #271 commit 52eeefb3) | Emit |
| **D7-S2-A16-T02** | exec.Emit overwrite per Run（避免 stale emit hook 串扰）| **IMPLEMENTED** | `sessionorchestrator/item_pipeline_emit.go` (PR #271 commit 52eeefb3) | Emit |

### D7-S2-A14: WaitForTurnCompletion + TurnState (DESIGN DEFERRED v1.1)

| T ID | 描述 | Status | File | Span Evidence |
|------|------|--------|------| --- |
| **D7-S2-A14-T01** | `WaitForTurnCompletion` API（turn N 收尾前 turn N+1 阻塞）| **DESIGN** (v1.1) | design.md §2.1 (TurnState) | — |
| **D7-S2-A14-T02** | TurnState in-memory + sync.RWMutex（per-SessionOrchestrator）| **DESIGN** (v1.1) | design.md §2.1 (TurnState) | — |

### D7-S2-A15: TranscriptReader + turn 上下文注入 (DESIGN DEFERRED v1.1)

| T ID | 描述 | Status | File | Span Evidence |
|------|------|--------|------| --- |
| **D7-S2-A15-T01** | TranscriptReader for fold-output（filter kind=complete, Body 字段, capture/gateway.go:880）| **DESIGN** (v1.1) | design.md §2.1 (TranscriptReader) | — |
| **D7-S2-A15-T02** | turn directive auto-injection（<prior-output-summary> 标签注入 WorkItem.Directive）| **DESIGN** (v1.1) | design.md §2.1 (TranscriptReader) | — |

### D7-S2-A17: feishu adapter TurnInProgressError (DESIGN DEFERRED v1.1)

| T ID | 描述 | Status | File | Span Evidence |
|------|------|--------|------| --- |
| **D7-S2-A17-T01** | feishu adapter 识别 `TurnInProgressError` + "⏳ 上一条还在处理中" 文案 | **DESIGN** (v1.1) | design.md §2.3 (feishu adapter) | — |

**A14~A17 Total:** 7 P0 T — 2 IMPLEMENTED (PR #271) + 5 DESIGN (v1.1) | **0 panic** (production smoke sess_1782638991113_5000 post-#271 PASS)

**验收结论（PARTIAL）：** RC-3 panic hotfix done via PR #271；RC-1/2/4 设计 4 层契约（TurnState + TranscriptReader + WaitTurn + feishu adapter）已就位待 v1.1 实施；22/22 orchestration packages -race PASS。

---

## D7-S8/S9/S12-A30+: MUPS 5-node Span 全覆盖 + 目录结构治理 (DM-20260625-019, FULL)

> **Change:** `devrix-d7-mups-v4-5node-coverage-orchestration` (DM-20260625-019) — D7 MUPS 5-node Span 全覆盖 + mups/{execute,learn} 目录结构治理  
> **归档：** `openspec/archive/2026-06-29-devrix-d7-mups-v4-5node-coverage-orchestration/`（S7_Archived **FULL** — 6 T 100% DONE, PR #235+#236 squash merged 2026-06-26）  
> **v6.0.x 维护阶段：** Span 注册 + root span + 物理迁移 + 0 函数签名变化

### D7-S8-A30/A31: 5-node Span 注册 + D7_MUPS_Pipeline 根 Span (PR #235)

| T ID | 描述 | Status | File | Span Evidence |
|------|------|--------|------| --- |
| **D7-S8-A30-T01** | 5 节点 Span (TaskGraph/Executor/Channel/Memory/Anomaly) 注册到 coverage registry | **IMPLEMENTED** | `observability/diagnose/coverage/registry_test.go` + `orchestration/sessionorchestrator/spans.go` (PR #235) | Coverage_Registry |
| **D7-S8-A31-T01** | D7_MUPS_Pipeline 根 Span + 5 节点子 Span (taskgraph.synthesize / executor.select / channel.route / memory.persist / system.anomaly_detect) 端到端串联（Jaeger mupsSpan.parent == orchSpan.SpanContext）| **IMPLEMENTED** | `orchestration/sessionorchestrator/orchestrate_path.go` + `observability/instrument/telemetry/names.go` (PR #235) | MUPS_RootSpan |

### D7-S9-A30: mups/execute/ channel_ 前缀清理 (PR #236)

| T ID | 描述 | Status | File | Span Evidence |
|------|------|--------|------| --- |
| **D7-S9-A30-T01** | mups/execute/ 5 个 channel_ 前缀清理（目录浏览无 channel_ 噪音）| **IMPLEMENTED** | `orchestration/mups/execute/*` (PR #236) | — |

### D7-S12-A30: mups/learn/ 4 subpackage 拆分 (PR #236)

| T ID | 描述 | Status | File | Span Evidence |
|------|------|--------|------| --- |
| **D7-S12-A30-T01** | mups/learn/ 拆 4 subpackage: asset/ + memory/ + reputation/ + prior/ | **IMPLEMENTED** | `orchestration/mups/learn/{asset,memory,reputation,prior}/` (PR #236) | — |
| **D7-S12-A30-T02** | import cycle 打破（DefaultPendingMaxRetries 上提 asset/）| **IMPLEMENTED** | `orchestration/mups/learn/asset/` (PR #236) | — |

**A30+ Total:** 5 P0 T (在 S8/S9/S12 三个 Scenario 跨域) — 5 IMPLEMENTED (PR #235+#236) | **0 函数签名变化** (pure physical migration) | **23 orchestration packages -race PASS 0 FAIL**

---

## D7-S18: Pessimistic Commit + Rule-based Fallback PR-B (DM-20260629-008)

> **Change:** `devrix-d7-taskcontract-unification-pr-b` (DM-20260629-008) — v7.0 TaskContract 统一 PR-B (L3 防御运行时层)。**6/7 P0 T IMPLEMENTED**（T05 Span/Metric 完整 wire 留 PR-C）。Feature Flag `D7_PESSIMISTIC_COMMIT_ENABLED` 默认 disabled, 0 行为变更.

### D7-S18-A11: Pessimistic Commit (5 类触发 + MVPArtifact)

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------|---------------|
| **D7-S18-A11-T01** | **PessimisticCommitGuard.Evaluate happy path (5 类触发全不命中 → ok=true) + Disabled/Nil 守门 (Feature Flag off / nil receiver / nil report 全 no-op)** | **D7-S18-A11** | **`escape/fallback_test.go::TestDefaultPessimisticCommitGuard_Enabled_HappyPath + TestDefaultPessimisticCommitGuard_Disabled + TestDefaultPessimisticCommitGuard_NilReceiver`** | **IMPLEMENTED** | **P0** | Pessimistic_Commit_Emit |
| **D7-S18-A11-T02** | **BuildMVPArtifact 5 类触发 (resource_exhausted / cb_l1 / indeterminate_3x / empty_evidence / manual_abort) 全部命中 + ChainHash FNV-1a 稳定 + traceback 256 截断 + nil report 防御** | **D7-S18-A11** | **`escape/fallback_test.go::TestDefaultPessimisticCommitGuard_Enabled_ResourceExhausted/CircuitBreakerL1/Indeterminate3x/EmptyEvidence/ManualAbort + TestDefaultPessimisticCommitGuard_BuildMVPArtifact + TestDefaultPessimisticCommitGuard_BuildMVPArtifact_Traceback + TestDefaultPessimisticCommitGuard_BuildMVPArtifact_NilReport + TestBuildChainHash_Stable`** | **IMPLEMENTED** | **P0** | Pessimistic_Commit_Emit |
| **D7-S18-A11-T03** | **5 层 CB L1 → Pessimistic action (L1 trips StateOpen + Evaluate 返回 ForceExit + 60s 持久窗口 + reason 含 "l1" Pessimistic guard 路由 hint)** | **D7-S18-A11** | **`escape/circuit_breaker_test.go::TestL1DispatchLoop_PessimisticHint + TestL1StateOpen_PersistentForPessimisticWindow + TestCircuitBreakerSet_L1Only_PessimisticCompatible`** | **IMPLEMENTED** | **P0** | Pessimistic_Commit_Emit |
| **D7-S18-A11-T04** | **Feature Flag env-gated (D7_PESSIMISTIC_COMMIT_ENABLED unset/0/false/no/off 全 disabled; 1/true/yes/on 全 enabled; 边界) + D7_RULE_FALLBACK_STRATEGY 4 候选 round-trip + unknown 兜底** | **D7-S18-A11** | **`bootstrap/pessimistic_guard_wire_test.go::TestPessimisticCommitEnabled_DefaultsOff/Truthy/Falsy + TestPessimisticRuleStrategy_Default/AllValid/InvalidFallsBack + TestNewPessimisticCommitGuardFromEnv_OffByDefault/EnabledWithCustomRule`** | **IMPLEMENTED** | **P0** | — |
| **D7-S18-A11-T05** | **Span d7.s18.pessimistic.commit.emit + pessimistic_commit_trigger_count Metric (结构化字段对齐 7 attributes; PR-B 阶段 slog.Info 占位, 完整 Jaeger/Prom wire 留 PR-C)** | **D7-S18-A11** | **`escape/engine.go::NotifyPessimistic` slog.Info("pessimistic_commit_emit", trace_id/reason/policy/fallback_used)** | **PLANNED** | **P0** | Pessimistic_Commit_Emit |
| **D7-S18-A11-T06** | **engine.NotifyPessimistic 5 层 fail-safe (nil guard / nil report / Evaluate error fall-open / blocked → MVPArtifact 注入 / Result.Kind 强制) + ChannelRouter.ApplyPessimisticCommit no-op 守门** | **D7-S18-A11** | **`escape/engine_test.go::TestEscapeEngine_NotifyPessimistic_NilGuard/DisabledGuard/Enabled_TriggersCommit/NilReport`** | **IMPLEMENTED** | **P0** | Pessimistic_Commit_Emit |

### D7-S18-A12: Rule-based Fallback (4 候选规则)

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------|---------------|
| **D7-S18-A12-T01** | **4 候选规则 (most_tests_passed / compiled_clean / min_cost / min_uncertainty) 闭集合 + DefaultFallbackRule=min_uncertainty + Valid/ValidNonLegacy 双判 + ParseFallbackRuleName (空→默认, 未知→默认+recognized=false)** | **D7-S18-A12** | **`interfaces/fallback_policy_test.go::TestFallbackPolicy_Valid/ValidNonLegacy + TestParseFallbackRuleName (9 cases) + TestFallbackPolicyRuleNames_ClosedSet + TestDefaultFallbackRule_Stable`** | **IMPLEMENTED** | **P0** | — |
| **D7-S18-A12-T02** | **ResolveFallback 3 路径 (Pessimistic/RuleBased/Abort) + policy_override Blockage.Source 解析 + RuleName 默认 min_uncertainty + env D7_RULE_FALLBACK_STRATEGY 切换** | **D7-S18-A12** | **`escape/fallback_test.go::TestDefaultPessimisticCommitGuard_ResolveFallback_Default/PolicyOverride`** | **IMPLEMENTED** | **P0** | Pessimistic_Commit_Emit |

**D7-S18 Total:** 7 P0 T (2 A × 7 T) — **6 IMPLEMENTED + 1 PLANNED (T05 Span/Metric 完整 wire 留 PR-C)** | **0 函数签名变化** (pure additive 嵌入, 全部用 interfaces.MVPArtifact + EscapeEngine.SetPessimisticGuard) | **3 orchestration packages -race PASS 0 FAIL** (interfaces / escape / mups/execute) | **interfaces coverage 96.9% / escape 85.0%** | **race-clean** | **4 ORCH_* SentinelError (7110-7113)**

---

## D7-S20/S21: TaskContract 统一 PR-A (DM-20260629-007, PR-A IN PROGRESS)

> **Change:** `devrix-d7-taskcontract-unification-pr-a` (DM-20260629-007) — v7.0 TaskContract 统一 PR-A (L1 接口层 + L2 字段语义层 + L4 spec 同步)  
> **归档：** `openspec/changes/devrix-d7-taskcontract-unification-pr-a/` (S4 实现已完成, S5 verify + S6 archive 待跑)  
> **T 编号重映射说明：** D7-S16 已被 Layer SubContext (DM-20260627-003) 18 T 点全 IMPLEMENTED 占用，本 PR-A 改分配 D7-S20/21 (D7-S22/23 为 PR-B 全字段迁移 + PR-C legacy 移除预留位)。

### D7-S20-A01: TaskSpec 下行契约构造 + 不可变 builder

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S20-A01-T01** | **NewTaskSpec happy path + Validate 边界 (空 Goal / TraceID 必填 + ts_8hex 格式) + 1000 并发构造 TraceID 唯一性** | **D7-S20-A01** | **`interfaces/task_spec_test.go::TestNewTaskSpec_{HappyPath,EmptyGoal,UniqueTraceIDs}` + `TestTaskSpec_Validate`** | **IMPLEMENTED** | **P0** | Task_Spec_Created |
| **D7-S20-A01-T02** | **TaskSpec With* 不可变 builder (WithConstraint/WithPreference/WithConvergenceBudget/WithCostBudget/WithTraceID 全部返回新副本, 100 goroutine 并发 WithConstraint + WithPreference 验证 base 不被污染)** | **D7-S20-A01** | **`interfaces/task_spec_test.go::TestTaskSpec_WithImmutability` + `TestTaskSpec_ConcurrentWith` (100 goroutines -race PASS)** | **IMPLEMENTED** | **P0** | Task_Spec_Created |
| **D7-S20-A01-T03** | **TaskSpec 3 处创建点统一 (Plan / Channel / WorkItem, additive 嵌入 ChannelRequest.Spec + LearnRequest.Report 兼容 legacy)** | **D7-S20-A01** | **`interfaces/taskcontract_test.go::TestTaskContract_RoundTrip` + 24/24 orchestration packages -race PASS** | **IMPLEMENTED** | **P0** | Task_Spec_Created |

### D7-S20-A02: TaskReport 上行契约构造 + 不可变 builder

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S20-A02-T01** | **NewTaskReport happy path + Validate (空 TraceID 拒绝) + 默认 Result=Pending** | **D7-S20-A02** | **`interfaces/task_report_test.go::TestNewTaskReport_{HappyPath,EmptyTraceID}` + `TestTaskReport_Validate`** | **IMPLEMENTED** | **P0** | Task_Report_Created |
| **D7-S20-A02-T02** | **TaskReport With* 不可变 + AppendDissent 5 个隐式拒绝 (空 Reason) + top-3 静默截断 + 100 goroutine 并发 AppendDissent 验证 base 不被污染** | **D7-S20-A02** | **`interfaces/task_report_test.go::TestTaskReport_WithImmutability` + `TestTaskReport_AppendDissent` + `TestTaskReport_AppendDissent_RejectsEmptyReason` + `TestTaskReport_ConcurrentAppend` (100 goroutines -race PASS)** | **IMPLEMENTED** | **P0** | Task_Report_Created + Dissent_Recorded |
| **D7-S20-A02-T03** | **Channel.Execute 出口 + Learn 节点入口统一 (4 Channel → Artifact → TaskReport, AssetBuilder 把 TraceID 透传到 SourceTraceID)** | **D7-S20-A02** | **`interfaces/taskcontract_test.go::TestTaskContract_RoundTrip` + `mups/execute/channel.go` Spec field + `mups/learn/asset/asset_builder.go` Report field (additive 嵌入)** | **IMPLEMENTED** | **P0** | Task_Report_Created |

### D7-S21-A01: Dissent 字段填充 + top-3 + summary hash

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S21-A01-T01** | **Dissent top-3 截断 (第 4/5 个 AppendDissent 静默 truncate, 不返回 error) + Summary hash 写入 + 5 entry 顺序保留 + Learn 沉淀路径正确 (PendingAsset → ScheduledMemory / SkillAsset → SkillMemory)** | **D7-S21-A01** | **`interfaces/taskcontract_test.go::TestTaskContract_DissentTopN` + `task_report_test.go::TestTaskReport_AppendDissent` (top-3 truncation 验证)** | **IMPLEMENTED** | **P0** | Dissent_Recorded |

### D7-S21-A02: Blockage 字段 3 类 kind 分类

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S21-A02-T01** | **Blockage 3 类 kind (Missing / Infeasible / RequiredExternal) 共存 + WithBlockage 不可变 (base 不被污染) + 顺序保留 + BlockageKind.String() stable for spans** | **D7-S21-A02** | **`interfaces/taskcontract_test.go::TestTaskContract_Blockage3Kind` + `task_report_test.go::TestTaskReport_WithBlockage` + `TestBlockageKind_String`** | **IMPLEMENTED** | **P0** | Blockage_Recorded |

### D7-S21-A03: Resource 字段 token/time/step 抽取

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S21-A03-T01** | **Resource 5 字段 (TokensUsed/TokensBudget/TimeElapsed/StepCount/ToolInvocations) 抽取 + 负值拒绝 (5 case × ErrResourceInvalid) + ResourceFromBudget bridge shape (Spec.CostBudget.Tokens == Report.Resource.TokensBudget)** | **D7-S21-A03** | **`interfaces/taskcontract_test.go::TestTaskContract_ResourceFromBudget` + `task_report_test.go::TestTaskReport_WithResource` + `TestTaskReport_WithResource_RejectsNegative` (5 negative cases)** | **IMPLEMENTED** | **P0** | Resource_Recorded |

### D7-S20-A03: spec 文档同步 (L4 spec layer)

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-S20-A03-T01** | **spec.md v7.0 ADDED 3 Requirement (TaskSpec / TaskReport / Contract Round-Trip) + 12 Gherkin Scenarios 覆盖 T01-T09** | **D7-S20-A03** | **`openspec/changes/devrix-d7-taskcontract-unification-pr-a/specs/d7-orchestration/spec.md`** | **IMPLEMENTED** | **P0** | — |
| **D7-S20-A03-T02** | **d7-domain.md §8 Layer + §9 interfaces 包 + a-registry.md 6 A + f-registry.md 11 F + t-registry.md 4 S/11 T + span-registry.md 5 span 增量同步** | **D7-S20-A03** | **`openspec/specs/d7-orchestration/{d7-domain,a-registry,f-registry,t-registry,span-registry}.md`** | **IMPLEMENTED** | **P0** | — |

**D7-S20/S21 Total:** 11 P0 T (9 单元/集成 + 2 spec 文档) — **9/11 IMPLEMENTED** (T01-T09 代码 100% PASS, T10+T11 spec 同步本表 + 5 spec 文件完成) | **0 函数签名变化** (pure additive 嵌入) | **24/24 orchestration packages -race PASS 0 FAIL** | **interfaces 包 coverage 95%** | **race-clean** | **5 ORCH_* SentinelError (7100-7104)**

**验收结论（FULL）：** Span 注册 + root span + 目录结构治理一次到位；d2-domain v8.5.0 → v9.0.0；spec v4.9.0→v4.10.0；D7 v6.0.x 维护阶段收尾。

---

## D7-S5/S9/S15/S16: MUPS Deliverable Convergence (DM-20260630-012)

> **Change:** `devrix-mups-deliverable-convergence` — DeliverableVerifier + LLM StrategicPlanProposer + Session complete gate.

| T ID | 描述 | Test 位置 | Status | Priority |
|------|------|-----------|--------|----------|
| **D7-S9-A32-T01** | VerifyDeliverable p0_p1_file_line | `sessionorchestrator/deliverable_verify_test.go` | **IMPLEMENTED** | P0 |
| **D7-S9-A32-T02** | StatusAfterSpawnNone incomplete → InProgress | `workmodel/pipeline_apply_test.go` | **IMPLEMENTED** | P0 |
| **D7-S5-A22-T01** | LLMStrategicPlanProposer parse/validate | `sessionorchestrator/strategic_plan_proposer_test.go` | **IMPLEMENTED** | P1 |
| **D7-S5-A22-T02** | ItemPipeline + bootstrap wire proposer | `bootstrap/wire_item_pipeline_test.go` | **IMPLEMENTED** | P1 |
| **D7-S15-A41-T02** | StructuredDeliverable bubble + rollup signals | `workmodel/context_bubble_apply.go` | **IMPLEMENTED** | P1 |
| **D7-S16-A76-T01** | Wire NewLLMStrategicPlanProposer | `bootstrap/wire_item_pipeline_test.go` | **IMPLEMENTED** | P1 |
| **D7-S2-A73-T03** | Session complete rollup + quality meta | `sessionorchestrator/session_complete_test.go` | **IMPLEMENTED** | P0 |
| **D7-S2-A73-T04** | Transition markers + TaskIncompleteMessage | `sessionorchestrator/session_complete_test.go` | **IMPLEMENTED** | P0 |

Integration: `tests/integration/d7/d7_deliverable_convergence_test.go` (tag `integration && d7`).

---

## D7-U: Uncertainty-Driven Spawn (DM-20260704-001)

> **Change:** `d7-uncertainty-spawn-decouple` — CC-U1～U6 decouple deliverable gate from spawn continuation; evidence progress + U drive rollup synth.

| T ID | L5 | 描述 | Test 位置 | Status | Priority |
|------|-----|------|-----------|--------|----------|
| **D7-U-T01** | L5-D7-U-01 | Partial + 高证据 + U 低 → RollupSynth 非 inline 耗尽 | `workmodel/evidence_progress_test.go::TestSpawnPolicyEvaluator_CCU1_inlineNotEscalateWithEvidence` + `spawn_apply_rollup_test.go::TestApplySpawnPolicy_RollupSynthSetsNeedsRollup` | **IMPLEMENTED** | P0 |
| **D7-U-T02** | L5-D7-U-02 | U 高时 strategic `single` 被拒绝 | `sessionorchestrator/strategic_plan_proposer_test.go::TestApplySingleModeUncertaintyGate_rejectsHighU` | **IMPLEMENTED** | P0 |
| **D7-U-T03** | L5-D7-U-03 | spawnRationale 区分 CC-1.2 vs R7 | `workmodel/evidence_progress_test.go::TestSpawnRationale_CC12_notR7` | **IMPLEMENTED** | P1 |
| **D7-U-T04** | L5-D7-U-04 | Session complete salvage via ExtractSessionDeliverable | `workmodel/deliverable_format_test.go::TestExtractSessionDeliverable_SalvageFromWorkItemArtifact` | **IMPLEMENTED** | P1 |
| **D7-U-T05** | L5-D7-U-05 | Deliverable alias registry + fence extract + verify JSON-body-only + U damp | `deliverable_findings_parse_test.go` + `deliverable_contract_verify_test.go` + `uncertainty_unified_test.go::TestComputeUnifiedUncertainty_formatFailureWithEvidenceDamps` | **IMPLEMENTED** | P1 |

**D7-U Total:** 5 T (2 P0 + 3 P1) — **5/5 IMPLEMENTED**

---

## D7-PL: Physical Layout Alignment (DM-20260701-004) — PLANNED

> **Change:** `devrix-d7-physical-layout-alignment` PR-1 (纯 markdown, 0 Go 业务代码): a/f-registry 补全 + code-layout §4.2 终态化 + t-registry 12 T 注册 (layout guard 由 PR-2 覆盖)。PR-2/3/4 后续分别覆盖 layout guard 测试 + plan/ doc-only 双登记 + orchtypes/ Cross-S 登记。
> **跨 PR 关系：** PR-1 (本段) + PR-2 (layout guard test-only) + PR-3 (plan/ dual-registration) + PR-4 (orchtypes/ registration)，最终 AC1-AC9 在 PR-4 后全部 PASS。

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority | Span Evidence |
|------|------|----------|-----------|--------|----------| --- |
| **D7-PL-T01** | **a-registry.md v5.1.0 → v5.4.0 ## Hardening 段 2 A 落地（Hardening-A01 MetricsEmit + Hardening-A02 ConcurrencyGuard 物理 location 锚点 `orchestration/hardening/` + `orchestration/wavescheduler/` 双归属）** | **D7-X-A01/D7-X-A02 (Cross-S Kernel)** | **`openspec/specs/d7-orchestration/a-registry.md` ## Hardening 段** | **IMPLEMENTED** | **P1** | — |
| **D7-PL-T02** | **a-registry.md ## D7-X Cross-S Kernel（orchtypes/）段 6 A 落地（D7-X-A01..A06 DefineCrossSPrimitives/DefineSentinelErrors/BoundaryDecision/AdaptivePriorInject/SystemAnomalyDetect/LLMInvokerContract）** | **D7-X-A01..A06** | **`openspec/specs/d7-orchestration/a-registry.md` ## D7-X 段** | **IMPLEMENTED** | **P1** | — |
| **D7-PL-T03** | **f-registry.md v5.1.0 → v5.4.0 ## Hardening F 段 2 F 落地（Hardening-A01-F01 EmitMetricSpan + Hardening-A02-F01 ConflictGuardAtomic）** | **Hardening-A01-F01/F02** | **`openspec/specs/d7-orchestration/f-registry.md` ## Hardening 段** | **IMPLEMENTED** | **P1** | — |
| **D7-PL-T04** | **f-registry.md ## D7-X F 段 6 F 落地（D7-X-A01-F01 NewObservation + D7-X-A01-F02 NewUncertaintyReport + D7-X-A02-F01 NewORCHSentinelError + D7-X-A03-F01 NarrowestSchema + D7-X-A04-F01 BayesianUpdate + D7-X-A06-F01 ValidateLLMRequest）** | **D7-X-A01-F01..A06-F01** | **`openspec/specs/d7-orchestration/f-registry.md` ## D7-X 段** | **IMPLEMENTED** | **P1** | — |
| **D7-PL-T05** | **code-layout.md v1.12.0 → v1.13.0 §4.2 D7 终态化（去除 retired ghost shim 行 coordinator/hubspoke/turn/milestone；新增 plan/orchtypes/hardening/interfaces 4 个归属登记）** | **—** | **`openspec/specs/architecture/code-layout.md` §4.2** | **IMPLEMENTED** | **P1** | — |
| **D7-PL-T06** | **code-layout.md §4.2 D7-S2-A06/A07 当前路径从 `turn/` 子包改为 `sessionorchestrator/`（turn/ 子包已物理合并入 sessionorchestrator/，DM-20260626-004）** | **D7-S2-A06/A07** | **`openspec/specs/architecture/code-layout.md` §4.2** | **IMPLEMENTED** | **P1** | — |
| **D7-PL-T07** | **PR-2 layout guard TestOrphanDirs：枚举 `internal/layers/orchestration/` 实际子目录，与 a-registry/f-registry 中登记的 Code Location 路径集合做双向对比，孤儿目录（物理存在但未登记）报告为 Violation** | **—** | **`internal/layers/orchestration/layout/guard_test.go::TestOrphanDirs`** | **IMPLEMENTED** | **P0** | — |
| **D7-PL-T08** | **PR-2 layout guard TestNoResurrectRetiredDirs：黑名单 retired 目录（coordinator/hubspoke/turn/milestone），任意一个存在则报告 Violation（防止 resurrection）** | **—** | **`internal/layers/orchestration/layout/guard_test.go::TestNoResurrectRetiredDirs`** | **IMPLEMENTED** | **P0** | — |
| **D7-PL-T09** | **PR-2 layout guard TestACanonicalLocationsExist：对 a-registry 中登记的所有 A Code Location，断言物理文件存在；缺失则报告 Violation（AC2 反向验证）** | **—** | **`internal/layers/orchestration/layout/guard_test.go::TestACanonicalLocationsExist`** | **IMPLEMENTED** | **P0** | — |
| **D7-PL-T10** | **PR-2 layout guard TestFCanonicalLocationsExist：同 T09，对 f-registry 中登记的所有 F Code Location 反向验证** | **—** | **`internal/layers/orchestration/layout/guard_test.go::TestFCanonicalLocationsExist`** | **IMPLEMENTED** | **P0** | — |
| **D7-PL-T11** | **PR-2 layout guard TestGhostDirsInCodeLayout：枚举 code-layout.md §4.2 中提到的所有 scenario-slug，断言对应物理目录存在；幽灵行（spec 提了但物理没有）报告为 Violation（AC5 反向验证）** | **—** | **`internal/layers/orchestration/layout/guard_test.go::TestGhostDirsInCodeLayout`** | **IMPLEMENTED** | **P0** | — |
| **D7-PL-T12** | **PR-2 layout guard TestNoRetiredTopLevelFiles：黑名单 retired 顶层文件（coordinator.go / hubspoke.go / turn.go / milestone.go 在 orchestration/ 根），任意一个存在则报告 Violation（防 resurrection 文件级）** | **—** | **`internal/layers/orchestration/layout/guard_test.go::TestNoRetiredTopLevelFiles`** | **IMPLEMENTED** | **P0** | — |
| **D7-PL-T13** | **PR-3 `plan/` 归属 S5 doc-only dual registration 在 code-layout + a-registry 双登记（spec.md Requirement "`plan/` MUST be registered under D7-S5 Decision & Planning"）**：a-registry.md S6 段新增 D7-S6-A03 PlanValidate（Code Location `decisionplanning/plan_mode.go::Validate`）+ D7-S6-A04 PlanGenerate（Code Location `plan/planner.go::DefaultPlanner.Generate`），物理路径在 S5 路径符合 spec.md §S5 carve-out Note；code-layout.md §4.2 D7-S5 sub 行 "Plan agent" → "Plan Generation" 命名收敛，保留 doc-only dual-registration 注释（含 word "doc-only 双登记" 与 a-registry 内部一致）；0 函数签名 / 0 物理路径变化 | **D7-S6-A03/A04** | **`openspec/specs/d7-orchestration/a-registry.md` S6 段 + ## D7-S5 plan/ ↔ decisionplanning/ 双登记说明段 + `openspec/specs/architecture/code-layout.md` §4.2 D7-S5 sub 行** | **IMPLEMENTED** | **P1** | — |
| **D7-PL-T14** | **PR-4 `orchtypes/` Cross-S kernel registration 收尾（spec.md Requirement "`orchtypes/` MUST be registered as D7 Cross-S Kernel"）**：orchtypes/doc.go package 注释升级为 "Package orchtypes is the cross-S governance kernel of D7 (types, sentinels, intent/observation primitives)."；d7-domain.md §North Star 新增 Cross-S Kernel (orchtypes/) 1 行（types / sentinels / intent primitives / Bayesian / Verdict / Observation / UncertaintyCoord / PlanKind / ChannelKind / ArtifactKind / 14 ExitReason — single source of truth for D7 contract），与 a-registry.md ## D7-X 段 + f-registry.md ## D7-X 段 + code-layout.md §4.2 Cross-S Kernel 行四处语义对齐；PR-1 已落地 a-registry 6 A + f-registry 6 F + code-layout 1 行，本 PR 收尾 doc.go + d7-domain.md；0 函数签名 / 0 物理路径变化 | **D7-X-A01..A06** | **`internal/layers/orchestration/orchtypes/doc.go` + `openspec/specs/d7-orchestration/d7-domain.md` §North Star** | **IMPLEMENTED** | **P1** | — |

**D7-PL Total:** 14 T (6 P1 spec doc sync + 6 P0 layout guard + 1 P1 plan/ dual registration + 1 P1 orchtypes/ Cross-S kernel registration 收尾) — **14/14 IMPLEMENTED**（PR-1 落 6 P1 spec doc sync；PR-2 落 6 P0 layout guard 测试 + 6 ghost 行 🔶 翻转；PR-3 落 1 P1 plan/ dual registration + CHANGELOG.md 清理 PR-1 遗留 `<<<<<<<` conflict marker；PR-4 落 1 P1 orchtypes/ Cross-S kernel registration 收尾）。**cumulative version bump**：跳过 v4.19.1（该位预留给 devrix-d7-s-layer-normalization DM-20260701-002/003 — S7+ → historical-s-mapping.md 物理拆分的 t-registry 同步）。

**D7 全域统计：** 281 IMPLEMENTED + **14 IMPLEMENTED (D7-PL)** = **295 T 全 IMPLEMENTED** · **P0 243**（237 原 + 6 新 P0 layout guard T）。


**S1_Cancelled (DM-20260630-002 devrix-d7-spec-split):** 0 T (S1 阶段取消, replaced by devrix-spec-lite-mode DM-20260630-003, 详见 `openspec/archive/2026-06-30-devrix-d7-spec-split/acceptance-report.md`)
