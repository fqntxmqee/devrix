# D7 Orchestration Domain Specification

**Capability:** d7-orchestration
**Domain:** D7
**DSAFT Type:** 核心域 (Core Domain)
**Version:** 4.17.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-29 (taskcontract-unification-pr-b DM-20260629-008: 新增 2 ADDED Requirement (D7-S18-A11 PessimisticCommitGuard.Evaluate + D7-S18-A12 Rule-based Fallback) + 11 Gherkin Scenario; L3 防御运行时层 PessimisticCommitGuard interface + 5 类触发 + 4 候选规则 Rule-based Fallback; interfaces 包 +3 NEW (contracts.go + fallback_policy.go + convergence_budget.go) + escape/fallback.go (DefaultPessimisticCommitGuard); Feature Flag D7_PESSIMISTIC_COMMIT_ENABLED 默认 disabled 0 行为变更; 6/7 P0 T IMPLEMENTED T05 Span/Metric 完整 wire PLANNED 留 PR-C)
**Domain SoT:** `d7-domain.md`
**Layering Spec:** `openspec/specs/architecture/layering.md`
**Change ID:** devrix-d7-orchestration-domain (DM-20260613-001)
**Demand:** `openspec/changes/devrix-d7-orchestration-domain/demand.md`
**Review R1:** `openspec/changes/devrix-d7-orchestration-domain/review-r1.md`
**Review R2:** `openspec/changes/devrix-d7-orchestration-domain/review-r2.md`

**Archived Changes:** devrix-queryloop-context (2026-06-10, ORCH v2 read model), devrix-wave-scheduler (WaveScheduler), devrix-d7-uncertainty-gaps (2026-06-16, DM-20260616-001, 5 gap fixes), devrix-d7-error-aggregation-and-metrics (2026-06-21, DM-20260621-010, D7-S6 错误聚合 + worktree 全链路 metrics), devrix-d7-mups-v4-phase3-execute (2026-06-23, DM-20260625-001, Phase 3 PR-C1: Execute Artifact 4 类 + SideEffect 5 态 + Artifact struct 5 字段升级), devrix-d7-mups-v4-phase2-observe-plan (2026-06-23, DM-20260623-001, Phase 2 PR-A1 + PR-RF: Observation 4 类 + UncertaintyReport + UncertaintyCoord), devrix-d7-mups-v4-phase2-plan (2026-06-23, DM-20260623-001-PRB1, Phase 2 PR-B1: Plan 4 类 + Planner interface + MatchKind 4 Rules), devrix-d7-mups-v4-phase3-channels (2026-06-23, DM-20260625-001-PRC2, Phase 3 PR-C2: Execute 4 Channel + ChannelRouter), devrix-d7-mups-v4-phase4-verify-promotion (2026-06-23, DM-20260623-002, Phase 4 Verify 节点升格: VerdictKind 4 态 + AggregateVerdicts + VerdictToExitReason + Evidence + SystemAnomaly + 14 ExitReason + G8-1 修复), devrix-d7-mups-v4-phase5-learn (2026-06-23, DM-20260623-003, Phase 5 Learn 节点升格: LearningAsset 5 类 + AssetContent + LearningClass 5 枚举 + ReputationEvidence Bayesian + AdaptivePrior + Memory 3 通道 + Learner + LP-1 闭环 + G8-1 修复延伸), devrix-d7-mups-v4-phase6-observe-learner-wiring (2026-06-24, DM-20260624-001, Phase 6 Observe-Learner 跨域闭环集成: ObserveRequest + IntentQuantizer + AnomalyDetector + RuleClassifier.ClassifyWithPrior + Orchestrator buildObserveRequest 3 层 fail-safe + WithLearner option + LP-1 闭环 E2E 集成测试), devrix-d7-mups-v4-phase7-verify-auto-close (2026-06-24, DM-20260625-001, Phase 7 Verify→Learn Auto-Close + Operator TrackMode + D5 可观测化增强: SessionOrchestrator.processAutoClose + synthesizeVerdict 4 规则 + 3 层 fail-safe + AssetBuilder Auto-Close fallback + ProcessRequest.TrackMode 字段 + DefaultLearner.Inject 3-tier 解析 + sessionSpan 6 prior attributes), devrix-d7-mups-v5-escape-engine (2026-06-25, DM-20260625-003, MUPS v5 统一逃逸机制: LoopDepthTracker v2 + PlanKindSwitchPolicy + EscapeAction 6 类 + ChainedArbitrator LLM/Rule/Human 3 层 + EscapeEngine 整合入口 + CircuitBreaker 5 层接线 + 5 节点 EscapeEngine 接线点), devrix-d7-taskcontract-unification-pr-a (2026-06-29, DM-20260629-007, v7.0 TaskContract PR-A: interfaces 包 7 NEW + 2 MODIFIED + ChannelRequest.Spec/LearnRequest.Report additive + 5 ORCH_* SentinelError 7100-7104 + 5 P0 TaskContract span ops), devrix-d7-taskcontract-unification-pr-b (2026-06-29, DM-20260629-008, v7.0 TaskContract PR-B L3 防御运行时层: PessimisticCommitGuard interface + 5 类触发 (resource_exhausted/cb_l1/indeterminate_3x/empty_evidence/manual_abort) + 4 候选规则 Rule-based Fallback + 4 ORCH_* SentinelError 7110-7113 + 1 Pessimistic_Commit_Emit span + interfaces 包 +3 NEW + escape/fallback.go DefaultPessimisticCommitGuard + escape/engine.go NotifyPessimistic + mups/execute/channel.go ApplyPessimisticCommit + bootstrap/pessimistic_guard_wire.go Feature Flag env-gated; 6/7 P0 T IMPLEMENTED T05 Span/Metric 完整 wire PLANNED 留 PR-C; Feature Flag D7_PESSIMISTIC_COMMIT_ENABLED 默认 disabled 0 行为变更)

---

## Overview

D7 编排域回答 **"做什么、按什么顺序做、谁来做、做得怎么样了"**。作为 **横向协调层** 编排 D2（LLM↔Tool 执行原语）与 D4（Agent 委托原语），并向 D1 发布进度事件（D1 仍拥有 ingress）。

**现行实现路径（2026-06-25，v4.3 post-cleanup）：** v2.0 Structure（DM-20260619-005）+ devrix-d7-dead-files-cleanup（DM-20260625-013..016）合流后物理路径与 S 层 1:1 对齐：S2 `sessionorchestrator/`、S3 `wavescheduler/`、S4 `executionflow/{hub,workplan,imsink,bridge}/`、S5 `plan/` + `decisionplanning/`（PlanAgent 仅 `/plan` 命令入口）；`coordinator/` `hubspoke/` type-alias shim 已并入源包，0 残留。D1 主入口 `sessionorchestrator.Entry.ProcessMessage`（`bootstrap/wire_coordinator.go::WireD7`）。Intent 四链正交分发不变（CommandHandler / FastPath / OrchestratePath / Skip）。详见 `pipeline-architecture.md` v1.1.0 §6.3 + §7。

### S 层博弈角色定义（切法 A — 按用户价值流）

> **基于 `devrix-d7-sa-refine` (DM-20260614-008) + DM-020 D7 Turn 编排上移 (DM-20260614-020)**

| S 层 | 博弈角色 | North Star |
|------|---------|------------|
| D7-S2 | **Screening Mechanism** + **Turn Leader (Stackelberg)** | 用户消息统一入口 + Turn 主循环；S2 = Meta-Orchestrator 跨 S3/S4/S5 |
| D7-S3 | **Mechanism Designer** | 多任务并行执行，冲突避免，上下文隔离 |
| D7-S4 | **Costly Signaler** | 执行进度透明，WorkPlan 可追溯 |
| D7-S5 | **Information Producer** | 把用户 goal 转化为可执行的任务结构 |
| D7-S1 | **State Authority**（非博弈角色） | **WorkItem** 持久化与状态机（v4.3 post-cleanup，Task flat-view 已删）；产"事实"而非"决策" |

| 版本里程碑 | 能力 |
|-----------|------|
| ORCH v1.0 (2026-06-10) | Hub-Spoke 读模型：WorkPlan + ExecutionFlowHub |
| ORCH v1.1 (2026-06-10) | WaveScheduler：DAG 调度、5-slot WorkerPool、ConflictGuard |
| D7 v0.5 (2026-06-13) | DSAFT 域定义、A/F/T 注册表、迁移设计（S3 规划） |
| D7 v1.0 (目标) | 入口上移、D2 瘦身、Task 模型归 D7-S1、S5-P2 分类 |
| D7 v2.1.0 (文档) | Review R1 澄清：三模型、路由矩阵、迁移契约 |
| D7 v2.2.0 (文档) | Review R2：D7-D1 权力分配、HandleInterrupt 顺序、T02c |
| D7 v2.3.0 (2026-06-15) | v1.0 + v1.1 closure：S2 Turn Leader 角色补登 + S1 State Authority 标注；DSAFT 结构 + Scenarios 表 IMPLEMENTED 状态刷新 |

---

## Review R1 澄清摘要（2026-06-14）

完整条文见 `d7-requirements-clarifications.md` §Requirements Clarifications 与 `demand.md`。

| 主题 | 决议 |
|------|------|
| Task 模型 | 三模型职责分离（PlanTask / WaveTaskNode / BackgroundRun），v1.0 不合并存储 |
| S2 vs S3 | 编排路由矩阵：并行 execute 归 S3 Wave，S2 不串行替代 |
| S5 | 分阶段：v1.0 仅 P1(PlanMode)+P2(Classify)；自动拆解 v1.1 |
| 迁移 | `d7.enabled` 默认 true；legacy D1→D2 已退役（DM-20260614-007） |
| 性能 | FastPath 拆 T02a(proxy≤2ms) + T02b(classify≤1ms) + T02c(端到端≤2ms) |
| 配置 | 单一 SoT：`context_engine.tasks.store_dir` |

---

## Review R2 结构层决议（2026-06-14）

完整条文见 `review-r2.md`。

| 主题 | 决议 |
|------|------|
| D7-D1 权力 | D1 ingress owner；D7 routing decision owner；`d7_enabled` 否决权 |
| HandleInterrupt | /stop：Wave→D4→Process→stopped→TaskCancel；正常 Process 结束不杀 Wave |
| OQ-1~4 | 全部闭合（见 review-r1.md §2） |
| D6 advisory | P1 补 `orchestration.d6.validation.*` metric |
| S5 shadow | P1 tail-only LLM classify（规则未命中），为 v1.1 兜底准备 |

---

## DSAFT 结构

| 层级 | ID | 名称 | 说明 | 实现状态 |
|------|-----|------|------|----------|
| D | D7 | Orchestration | 跨域编排协调层 | **IMPLEMENTED**（v1.0 + v1.1 闭环） |
| S | D7-S1 | Work Model | **WorkItem** 数据模型与生命周期（v4.3 post-cleanup, Task flat-view 已删除） | **IMPLEMENTED** → `workmodel/work_tree.go` + `workmodel/workitem.go` + `sessionorchestrator/workmodel.go` |
| S | D7-S2 | Session Orchestrator | 用户消息主入口、Turn 主循环、Dispatch | **IMPLEMENTED** → `sessionorchestrator/` + `turn/` |
| S | D7-S3 | Wave Scheduler | DAG 调度、WorkerPool、ConflictGuard | IMPLEMENTED → `wavescheduler/` |
| S | D7-S4 | Execution Flow | FlowEvent 聚合、WorkPlan 快照、IM 广播 | IMPLEMENTED → `executionflow/` |
| S | D7-S5 | Decision & Planning | 意图分类、任务拆解、执行器选择 | **IMPLEMENTED** → `decisionplanning/` |

---

## Scenarios

| ID | Scenario | Responsibility | Status | 代码位置 |
|----|----------|----------------|--------|----------|
| D7-S1 | Work Model | **WorkItem** CRUD、依赖 DAG、磁盘持久化（schema v2）、PlanMode 状态机 | **IMPLEMENTED**（v1.0+post-cleanup v1.1） | `workmodel/work_tree.go` + `workmodel/workitem.go` + `sessionorchestrator/workmodel.go` |
| D7-S2 | Session Orchestrator | ProcessMessage、FastPath、HandleInterrupt、TurnLoop、InvokeLLM、Dispatch | **IMPLEMENTED** | `sessionorchestrator/` + `turn/` |
| D7-S3 | Wave Scheduler | TaskGraph DAG、5-slot 池、ContextPolicy、ConflictGuard | IMPLEMENTED | `wavescheduler/` |
| D7-S4 | Execution Flow | Hub 双通道发布、WorkPlan 读模型、IM worker_progress、SpokeBridge | IMPLEMENTED | `executionflow/{hub,workplan,imsink,bridge}/` |
| D7-S5 | Decision & Planning | PlanAgent 只读探索（/plan CLI）、规则+LLM 分类、PlanKind 4 类 + DefaultPlanner、SynthesizeTaskGraph、SelectExecutor | **IMPLEMENTED**（Phase 2 PR-B1） | `plan/{plan,plan_struct,planner,blast_radius,errors}.go` + `decisionplanning/` + `workmodel/{plan_agent,plan_mode}.go` |
| **D7-S6** | **Error Aggregation & Metrics** | **HandleInterrupt errors.Join 聚合 + InterruptMetrics; sandbox cleanup observability (freefork + execute); WaveScheduler 4 新 metrics 字段; TaskManager panic counter** | **IMPLEMENTED** | `sessionorchestrator/{interrupt,metrics}.go` + `multiagent/provision/freefork/{forker,metrics}.go` + `multiagent/execute/{worker,metrics}.go` + `wavescheduler/scheduler.go` + `workmodel/task_manager{,_metrics}.go` |
| **D7-S9** | **Execute Artifact Data Contract (PR-C1)** | **ArtifactKind 4 类枚举（StateChangeCert/ResponseRecord/ProbeReport/ExperimentData）+ SideEffectStatus 5 态（None/Unknown/Inflight/Committed/RolledBack）+ wavescheduler.Artifact +5 字段 omitempty 向后兼容 + 跨域类型上提 shared/types 打破 import cycle** | **IMPLEMENTED (PR-C1)** | `internal/shared/types/execute.go` + `orchtypes/artifact_kind_alias.go` + `wavescheduler/types.go` |
| **D7-S8** | **Observation + UncertaintyReport (PR-A1 + PR-RF)** | **Observation 4 类（ObsFact/ObsSignal/ObsDeviation/ObsUncertainty）× 2 Category（CatBusiness/CatSystem）+ sealed Payload interface + UncertaintyReport Partition 不变式 + UncertaintyCoord Phase 2 扩展（FromVerifier/IsColdStart/Equal/With*）+ PR-RF 5 项 review fix（C1 IntentKind enum + C3 FromVerifier fail-fast + W2 fmt.Errorf wrap + W3 clamp01Float 合并 + W6/I8 Partition clamp 末尾）** | **IMPLEMENTED (PR-A1 + PR-RF, A15 模块)** | `internal/layers/orchestration/orchtypes/{observation,uncertainty_report,uncertainty_coord,errors}.go` |
| **D7-S11** | **Learn Node (PR-E1..E5)** | **LearningAsset 5 类（LearningSOP ★5 / LearningProtocol ★4 / LearningKnowledge ★3 / LearningConclusion ★2 / LearningPending ⭐★1）+ AssetContent interface + LearningClass 5 态 typed enum（shared/types/learning.go）+ ReputationEvidence struct 12 字段 + BayesianUpdate 函数（已并入 reputation_evidence.go）+ Wilson Score 95% 置信区间 + ⭐G8-1 修复（verifier_parse_failure 不污染 α/β，仅 VerifierFailureCount++）+ AdaptivePrior + BetaPrior + InjectTarget 3 枚举 + DefaultDeveloperPrior Beta(5,3) + DefaultOperatorPrior Beta(8,1) + BuildAdaptivePrior Bayesian 合并 + Memory interface + 3 通道（SkillMemory/FeedbackMemory/ScheduledMemory）+ ScheduledRetry envelope + LP-2 隔离 ErrAssetClassMismatch + Learner interface 3 方法 + DefaultLearner + AssetBuilder 5 类 Content 构造 + ReputationStore + InMemoryReputationStore + LP-1 闭环入口（Inject）+ 4 Verdict → 5 LearningClass 路由（T13 PARTIAL：Observe 跨域 wiring 留待 Phase 6 集成）** | **IMPLEMENTED (PR-E1..E5, 12 P0 + T13 PARTIAL)** | `internal/layers/orchestration/learn/{learning_asset,asset_content,reputation_evidence,adaptive_prior,memory,asset_builder,reputation_store,learner}.go` + `internal/shared/types/learning.go` |
| **D7-S12** | **Observe-Learner 跨域闭环集成 (PR-F1/F2/F3)** | **ObserveRequest struct + NewObserveRequest fail-fast + EffectivePrior DefaultDeveloperPrior 兜底 + IntentQuantizer 4 IntentClass (Fact/Command/Orchestrate/Skip) + Quantize baseline + QuantizeWithPrior (Mean 乘数, clamp [0,100]) + AnomalyDetector + HistoricalDetector.DetectWithPrior (threshold = 0.5 × Mean, Mean 越高阈值越高 = 更信任用户 = 更易放过) + RuleClassifier.ClassifyWithPrior + IntentClassifier 接口扩展 ClassifyWithPrior + ShadowClassifier.ClassifyWithPrior 委托给 rule + SessionOrchestrator.learner 字段 + WithLearner option + buildObserveRequest 3 层 fail-safe (nil learner / Inject error / 正常 → 全部 DefaultDeveloperPrior 兜底) + ProcessMessage 在 classifySpan 之前调用 buildObserveRequest + 4 E2E 集成测试 (Pass Accumulate / G8-1 parse_failure No Pollution / PendingAsset ScheduledMemory / 5-Node Pipeline End2End) + LP-1 闭环 (Learn × 3 Pass → Alpha=3 → Round 2 观察 Beta(8,3)) + LP-2 隔离 (PendingAsset 仅在 ScheduledMemory) + LP-5 反向追溯 (Plan.SourceObservationIDs / Verdict.SourceArtifactID / Asset.SourceSessionIDs)** | **IMPLEMENTED (PR-F1/F2/F3, 6 P0 T)** | `internal/layers/orchestration/orchtypes/{observe_request,intent_quantizer,anomaly_detector}.go` + `internal/layers/orchestration/decisionplanning/{classifier,shadow_classifier}.go` + `internal/layers/orchestration/sessionorchestrator/orchestrator.go` + `tests/integration/d7/learn_observe_closure_test.go` |
| **D7-S13** | **运行时 5 节点闭环 (PR-7.1/7.2/7.3)** | **SessionOrchestrator.processAutoClose 包装 channel + 异步触发 learner.Learn + 替换 endSpanWhenChannelClosed 调用 + synthesizeVerdict 4 规则 (complete→VerdictPass / error→VerdictFail Reason=Content / tombstone→VerdictIndeterminate IndeterminateReason="interrupt" / 其他 Type→nil) + SourceID `autoclose:{sessionID}:{nanosecond}` + 3 层 fail-safe (nil learner / Learn error / channel cancel → 全部 log + skip 不阻塞 caller) + IntentSkip 路径不调用 processAutoClose + AssetBuilder Auto-Close fallback (sop:autoclose:<SourceID> + ["autoclose-completion"] 合成步骤) + ProcessRequest.TrackMode string 字段 + TrackModeDeveloper/Operator 常量 + NewProcessRequest fail-fast 校验 + 3 sentinel error (ErrProcessRequestSessionIDEmpty / MessageEmpty / InvalidTrackMode) + DefaultLearner.Inject 3-tier 解析 (Reputation 持久状态 > req.TrackMode hint > Developer 兜底 + slog.Warn 未知值) + buildObserveRequest 透传 TrackMode → Operator track → DefaultOperatorPrior Beta(8,1) Mean=0.889 + priorSessionSpanAttrs 纯 helper 函数 + sessionSpan 6 prior attributes (learn.prior.alpha / beta / mean / track_mode / classifier_source / injected_at) + injected_at "phase6_lp1" (真实注入) vs "cold_start_failsafe" (兜底) + Jaeger UI 自然支持 + 30+ 单测/集成测试** | **IMPLEMENTED (PR-7.1/7.2/7.3, 6 P0 T)** | `internal/layers/orchestration/sessionorchestrator/{autoclose,tracing,orchestrator}.go` + `internal/layers/orchestration/learn/{learner,asset_builder}.go` + `internal/layers/orchestration/orchtypes/{process,errors}.go` + 4 NEW test files (`orchestrator_autoclose_test.go` + `orchestrator_trackmode_test.go` + `orchestrator_priorspan_test.go` + `process_test.go`) |
| **D7-S14** | **MUPS v5 统一逃逸机制 (PR-V5.1/V5.2/V5.3/V5.4/V5.5)** | **LoopDepthTracker v2 (按模式 hash 计数回路深度, MaxDepth=3, depth < MaxDepth → Continue, depth >= MaxDepth → ForceExit) + PlanKindSwitchPolicy 3 档 (Constrained ≤4 / Allowed / Forbidden) + EscapeAction 6 类 (Continue / EscalateToRule / EscalateToHuman / ForceExit / AbortWithAudit / EscapePendingHuman) + ChainedArbitrator LLM/Rule/Human 3 层 (5s + 10s timeout 兜底) + EscapeEngine 整合入口 (3 类深度限制: tracker + LoopBudget + CircuitBreaker) + CircuitBreaker 5 层接线 (L0 AnomalyDetector 5 nil / L1 DispatchLoop 100/min / L2 Verifier 3×2s / L3 Hook 5 fail / L4 WorkerPanic 1 / L5 SandboxExit 5 fail) + AuditLog AuditLevel 0/1/2 + 5 节点 EscapeEngine 接线点 (Observe 失败/Plan 失败/Plan 前/Execute 失败/Verify 失败 + 1a 短路不调 1b) + ResumeSession / applyResumeSession T2 续跑机制 (user_choice=A Continue / B ForceExit / C AbortWithAudit) + 13 类失败降级矩阵 (Evaluate panic/error + audit fail-open + LLM timeout + ctx cancel + CB metric timeout + ...)** | **PLANNED** | `internal/layers/orchestration/escape/{loop_depth_tracker,plan_kind_switch_policy,arbitrator,engine,circuit_breaker,audit_log,notifier}.go` |
| **D7-S15** | **WorkItem Rollup 闭环** | **Parent Rollup Gate (Path A) + Root Fallback (Path B) + Summary/Structured dual bubble Observe + Rollup MUPS R2+ synthesize + Session complete deliverable + ephemeral checklist gate; Phase 1 `RollupGatePolicy=best_effort` only** | **IMPLEMENTED (Phase 1)** | `workmodel/{rollup_gate,resolve,context_bubble_apply}.go` + `sessionorchestrator/{item_observe,item_pipeline,rollup_directive,rollup_verify,session_turn_loop}.go` |
| **D7-S16** | **Layer SubContext (Phase 1+2+3)** | **Per-Layer SubContext + ChildDownlink + cohort CG2′ + Upstream BlockedBy + PeerStatus + ScopeContract + SpawnPolicy gate + Observe R-OBS mapping (no WorkItem private) + LLM ObservationProposer (Phase 3) @ Observe + Wave ContextResolver→MaterializePolicy (Phase 3) + `depth≥1` 默认 Materialize (无 feature flag)** | **IMPLEMENTED (Phase 1+2+3, PR #269-#270 + #273-#275)** | `orchestration/wavescheduler/{context,context_materialize}.go` + `orchestration/observe/{scope_contract,r_obs_mapping,observation_proposer,llm_observation_proposer}.go` + `wire_wave.go` 等 |
| **D7-S18** | **Pessimistic Commit + Rule-based Fallback (L3 防御运行时层)** | **PessimisticCommitGuard interface（Evaluate / ResolveFallback / BuildMVPArtifact）+ 5 类触发条件（resource_exhausted / cb_l1 / indeterminate_3x / empty_evidence / manual_abort）+ MVPArtifact struct（Output/RiskWarnings/Trigger/ChainHash）+ 3 FallbackPolicy 路径（Pessimistic / RuleBased / Abort）+ 4 候选规则（most_tests_passed / compiled_clean / min_cost / min_uncertainty, default min_uncertainty）+ buildChainHash FNV-1a 16-char hex + Additive 嵌入 Engine.SetPessimisticGuard + ChannelRouter.SetPessimisticGuard + ApplyPessimisticCommit + 5 层 fail-safe（nil guard / nil report / Evaluate error→fall-open / blocked→MVPArtifact inject / Result.Kind force）+ 4 ORCH_* SentinelError 7110-7113 + 1 Pessimistic_Commit_Emit span + Feature Flag `D7_PESSIMISTIC_COMMIT_ENABLED` env-gated 默认 disabled, 0 行为变更** | **IMPLEMENTED (PR-B 6/7 P0 T)** | `internal/layers/orchestration/interfaces/{contracts,fallback_policy,convergence_budget,errors}.go` + `internal/layers/orchestration/escape/{fallback,engine}.go` + `internal/layers/orchestration/mups/execute/channel.go` + `internal/bootstrap/pessimistic_guard_wire.go` |
| **D7-S20** | **TaskContract 下行契约** | **TaskSpec struct（SessionID + Plan + Channel + WorkItem + TraceID + Blockage?）+ NewTaskSpec fail-fast（empty session_id 返 ORCH 7100 + channel unknown 返 ORCH 7101 + trace_id `ts_<8 hex>` 校验 返 ORCH 7104）+ Validate + WithPlan/WithChannel/WithWorkItem 不可变 builder 浅拷贝 + 3 处创建点统一（Plan 节点入口 / Channel.Execute 入口 / WorkItem 节点入口）+ Additive 嵌入 ChannelRequest.Spec（`mups/execute/channel.go`）+ 9/11 P0 T IMPLEMENTED (T01-T03 + T01-T03 + T01-T03); 2 spec 同步 T PENDING** | **IMPLEMENTED (PR-A 9/11 P0 T)** | `internal/layers/orchestration/interfaces/{doc,errors,task_spec,task_report,task_spec_test,task_report_test,taskcontract_test}.go` + `mups/execute/channel.go` (Spec 字段 additive) |
| **D7-S21** | **TaskReport 上行契约 + 字段语义层** | **TaskReport struct（SessionID + Channel + Verdict + TraceID + Dissents + Blockage? + Resource）+ NewTaskReport fail-fast（empty 返 ORCH 7102 + verdict unknown 返 ORCH 7103）+ Validate + WithVerdict/WithResource/WithBlockage 不可变 builder + AppendDissent top-3 silent truncate + HashDissentSummary fnv64a→8 hex + Learn 沉淀到 mups/learn/asset/ 现有 feedback 通道（老路径 0 变更）+ Resource 三件套 token/time/step + Blockage 3 类 kind (permission/resource/contract) + Additive 嵌入 LearnRequest.Report（`mups/learn/asset/asset_builder.go`）** | **IMPLEMENTED (PR-A 9/11 P0 T)** | `internal/layers/orchestration/interfaces/task_report.go` + `mups/learn/asset/asset_builder.go` (Report 字段 additive) |

---

## Architecture

> **5 节点管道端到端链路 + LP-1/2/5 闭环 + Auto-Close 异步触发的完整运行时序**，见 `pipeline-architecture.md`（MUPS v4.3 Phase 1-7 全部 S7_Archived 后的端到端总图）。

```
D1 Gateway.RouteInbound
    └── D7-S2 SessionOrchestrator.ProcessMessage    ← v1.0 主入口（wired by wire_coordinator.go::WireD7）
            ├── D7-S2-A02 ClassifyIntent (rule + LLM fallback)
            ├── switch intent.Kind (v1.1.0+ orthogonal dispatch):
            │     ├─ IntentSkip        → close channel
            │     ├─ IntentCommand     → CommandHandler.Handle
            │     │                       ├─ /plan → PlanCLICommands → PlanMode (D7-S5 PlanAgent)
            │     │                       ├─ /worktree → CLICommands → TaskManager.Tree() (D7-S1 WorkItem)
            │     │                       └─ /help, /stop → explicit handlers
            │     ├─ IntentFast        → FastPath.Run → TurnOrchestrator → D3 (LLM) + D2 (tools/persist)
            │     └─ IntentOrchestrate → OrchestratePath.Run
            │                            ├─ TaskDecomposer.SynthesizeTaskGraph (D7-S5-A02)
            │                            ├─ WaveScheduler.Start (D7-S3-A01)
            │                            └─ WaveScheduler.WaitForCompletion (D7-S3)
            ├── D7-S2-A06 RunTurnLoop → D7-S2-A07 InvokeLLM → D3 (LLM Gateway)
            │                            → D2 (ContextPreparer / ToolRoundExecutor / SessionPersister)
            ├── D7-S2-A04 DispatchWorker → hubspoke.Dispatcher → D4 Worker / D2 SubQuery
            └── flow.GlobalHub.Publish    ← D7-S4 读模型入口
                    ├── workplan.Service.Apply
                    ├── queue.SessionQueue (delegate-progress)
                    └── imsink.GatewaySink (worker_progress)

D4 Delegate.Service
    └── FlowBridge → flow.GlobalHub.Publish

WaveScheduler (独立调用路径，由 delegate_tools / Plan 触发)
    ├── TaskGraph.ReadyNodes
    ├── WorkerPool.Acquire (cursor=1, claude_code=1, subagent=3)
    ├── ConflictGuard.Allow
    ├── ContextResolver.Resolve (fresh|resume|upstream)
    └── WorkerRunner.Run → ArtifactStore
```

### 域边界

| D7 拥有 | D7 编排（不拥有） | D7 不拥有 |
|---------|------------------|----------|
| WorkPlan 读模型（D7-S4） | D7 RunTurn / D2 Prepare | 会话上下文（D2） |
| Wave DAG 调度（D7-S3） | D4 Delegate RunAgent | Agent 生命周期（D4） |
| FlowEvent 契约（contracts） | — | LLM 调用（D3） |
| Task/Plan 写模型（D7-S1） | | |

---

## ADDED Requirements

### Requirement: D7-S3 Wave Scheduler

`WaveScheduler` MUST provide DAG-based multi-agent scheduling with fixed WorkerPool capacity, ConflictGuard, and ContextPolicy resolution.

**Priority:** P0  
**Package:** `internal/layers/orchestration/wavescheduler/`  
**T:** D7-S3-T01 … D7-S3-T10

#### Scenario: DAG ready-node dispatch

- GIVEN a TaskGraph with dependency edges
- WHEN `ReadyNodes()` is evaluated
- THEN only nodes whose dependencies are `completed` and self state is `pending` are returned
- AND dispatch order is deterministic (sorted by id)

#### Scenario: Worker pool capacity

- GIVEN default `DefaultPoolCapacity`
- WHEN slots are acquired concurrently
- THEN peak running ≤ 5 (cursor=1, claude_code=1, subagent=3)
- AND slot release triggers immediate re-dispatch (D2 continuous loop)

#### Scenario: Conflict group mutual exclusion

- GIVEN two TaskNodes sharing the same `conflict_group`
- WHEN both are ready
- THEN at most one runs concurrently

#### Scenario: Context policy isolation

- GIVEN `context_policy=fresh`
- WHEN ContextResolver resolves
- THEN Messages contain only the directive (no Leader history)

- GIVEN `context_policy=upstream` and upstream artifact exists
- WHEN ContextResolver resolves
- THEN SystemPrompt includes upstream summary (no Leader history)

---

### Requirement: D7-S4 Execution Flow Hub

`ExecutionFlowHub` MUST aggregate `FlowEvent` from D2 SubQuery and D4 Delegate into WorkPlan snapshots, enqueue Leader delegate-progress, and optionally emit IM worker_progress.

**Priority:** P0  
**Package:** `internal/layers/orchestration/executionflow/hub/hub.go`  
**Contract:** `internal/shared/contracts/execution_flow.go`  
**T:** D7-S4-T01 … D7-S4-T04

#### Scenario: Dual publish on flow event

- GIVEN `execution_flow.enabled=true` with `link_tasks` and `im_progress`
- WHEN Hub.Publish receives FlowStarted
- THEN WorkPlan is updated via `workplan.Service.Apply`
- AND SessionQueue receives delegate-progress for Leader drain
- AND IM sink receives worker_progress when configured

#### Scenario: WorkPlan snapshot

- GIVEN FlowStarted and TaskManager updates for a session
- WHEN Hub.Snapshot is called
- THEN response includes ExecutionFlows with status and RecentEvents
- AND linked Task snapshots reflect owner and in_progress status

#### Scenario: Flow event lifecycle kinds

- GIVEN an active SubQuery or D4 worker
- WHEN FlowEvent is published
- THEN kinds include `started`, `completed`, `failed`, `tool_call`, `iterating`, `joined`
- AND each event is timestamped with actual occurrence time

#### Scenario: Tool call throttle

- GIVEN rapid FlowToolCall events for the same worker
- WHEN throttle window (`tool_summary_throttle_ms`, default 500ms) not elapsed
- THEN duplicate tool_call events are suppressed from publication

---

### Requirement: D7-S1 Work Model ✅ IMPLEMENTED

`TaskManager` MUST provide session-scoped Task CRUD with optional disk persistence and dependency tracking. PlanMode MUST support inactive → active → pending_approval lifecycle.

**Priority:** P0
**Package:** `internal/layers/orchestration/workmodel/` + `sessionorchestrator/workmodel.go`（v1.1 闭环，layer-delta Phase I/J）
**T:** D7-S1-T01 … D7-S1-T08（其中 T06 升 IMPLEMENTED via decomposer_test.go::validateGraph）

#### Scenario: Task create and persist

- GIVEN `tasks.mode=v2` and `store_dir` configured
- WHEN TaskManager.Create is called
- THEN a Task is created with unique ID and status `pending`
- AND the Task is persisted to disk when store is enabled

#### Scenario: Task-flow linkage

- GIVEN `execution_flow.link_tasks=true`
- WHEN Hub.Publish FlowStarted with TaskID
- THEN TaskManager sets owner and status `in_progress`
- AND FlowCompleted/FlowFailed transitions Task to terminal status

---

### Requirement: D7-S5 Plan Mode ✅ IMPLEMENTED

PlanMode MUST support `/plan` command workflow: enter → explore (read-only) → pending_approval → approve/reject.

**Priority:** P1
**Package:** `internal/layers/orchestration/workmodel/{plan_mode,plan_agent}.go`（v1.1 迁入；白名单测试在 `plan_agent_whitelist_test.go` 10 个 AC）
**T:** D7-S5-T01 … D7-S5-T05（含 T04 SynthesizeTaskGraph + T05 SelectExecutor，均已 IMPLEMENTED）
**Design:** `task-planning-design.md`

#### Scenario: Plan mode state machine

- GIVEN inactive PlanMode
- WHEN `/plan <goal>` is invoked
- THEN state transitions to `active`
- AND PlanAgent runs in read-only mode
- AND on completion state becomes `pending_approval`

---

### Requirement: D7-S8-A15 Observation + UncertaintyReport + UncertaintyCoord ✅ IMPLEMENTED

MUPS v4.3 Phase 2 Observe 节点的首批落地（PR-A1 + PR-RF）。Observe 节点必须有统一的数据契约：Observation struct + 4 类 `ObservationKind`（ObsFact/ObsSignal/ObsDeviation/ObsUncertainty）× 2 类 `Category`（CatBusiness/CatSystem）+ sealed `Payload` interface + `UncertaintyReport` 聚合 + `UncertaintyCoord` 增量扩展。本 Requirement 不引入 ProcessMessage wiring（PR-A4 范围），仅落地数据契约层。

**Priority:** P0
**Package:** `internal/layers/orchestration/orchtypes/{observation,uncertainty_report,uncertainty_coord,errors}.go`
**T:** D7-S8-A15-T01 … D7-S8-A15-T06
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase2-observe-plan/`

#### Scenario: Observation 4 类 × 2 Category + 不可变

- GIVEN 定义 `ObservationKind{ObsFact, ObsSignal, ObsDeviation, ObsUncertainty}` 与 `Category{CatBusiness, CatSystem}`
- WHEN 调用 `Observation.WithKind()` / `WithStrength()`
- THEN 返回新 Observation 实例，原实例未被修改
- AND 4 类 Kind 与 2 类 Category 正交（4×2 = 8 种组合，每种绑定 sealed Payload concrete type）
- AND `Strength ∈ [0, 1]` 边界保护（越界 panic via `clamp01Float(v, onNaN)`）
- AND `DetectedAt` 非零（零值 → `ErrObservationDetectedAtZero`）
- AND `MarshalJSON` wire format 为嵌套对象 `{"id": ..., "kind": ..., "category": ..., "strength": ..., "detected_at": ..., "payload": {...}}`

#### Scenario: UncertaintyReport Partition 不变式 + ComputeOverallStrength 仅遍历 CatBusiness

- GIVEN 10 个 Observation（6 CatBusiness + 4 CatSystem）
- WHEN 调用 `NewUncertaintyReport(observations)`
- THEN `report.BusinessObservations` 含 6 个，`report.SystemObservations` 含 4 个
- AND 6 + 4 == `len(report.Observations)` == 10
- AND 不变式被破坏时返回 `ErrUncertaintyReportPartitionInvariant`
- AND `report.ComputeOverallStrength()` 仅遍历 `BusinessObservations`，返回 avg(CatBusiness.Strength)
- AND CatBusiness 为空时 defaults 0.5（避免 NaN）

#### Scenario: UncertaintyCoord Phase 2 扩展（FromVerifier + Phase 1 兼容）

- GIVEN `VerdictKind=Pass, Confidence=0.9, Reason="ok"`
- WHEN 调用 `UncertaintyCoord.FromVerifier(verdict, confidence, reason)`
- THEN 返回 `UncertaintyCoord{Score: 0.9, Verdict: Pass, FromVerifier: true, Source: SourceVerifier, Reason: "ok"}`

- GIVEN 一个旧版本 JSON（仅有 `Value/UpdatedAt` 字段，Phase 1 wire format）
- WHEN `Unmarshal` 到新 `UncertaintyCoord`
- THEN `FromVerifier=false`（零值）+ `SideEffectStatus=""`（零值）
- AND `MarshalJSON` 使用 `omitempty` 不写零值字段，保持 Phase 1 调用方零修改

- GIVEN 一个未知 `VerdictKind` 值（不在白名单）
- WHEN 调用 `UncertaintyCoord.FromVerifier(unknown, ...)`
- THEN fail-fast 返回 `NewUncertaintyCoordInvalidVerdictKindError` + 错误码 `ORCH_COORD_VERDICT_7004`
- AND 不静默兜底（不返回零值 Coord）

#### Scenario: UncertaintyReport Anomalies subset + FilterByKind 跨 Category

- GIVEN 10 个 Observation 含 3 个 `ObsDeviation`（不论 Category）
- WHEN 调用 `report.Anomalies`
- THEN 返回 3 个 `ObsDeviation` Observation
- AND 调用 `report.FilterByKind(ObsDeviation)` 返回同样的 3 个（FilterByKind 故意遍历全集）

#### Scenario: Observation 字段校验 + 错误码

- GIVEN `FactPayload{Statement: ""}` 触发的 `validateFact` 失败
- WHEN 调用 `Observation.Validate()`
- THEN 返回 `fmt.Errorf("orchtypes: FactPayload.Statement empty: %w", ErrObservationPayloadInvalid)`（W2 包装）
- AND 错误可被 `errors.Is` 判定为 `ErrObservationPayloadInvalid`

#### Scenario: PR-RF 5 项 review fix 闭环

- **C1**: `QuantizedIntent.Kind` 从 `string` 改为 `IntentKind` enum（消除 PR-A2 translation shim）
- **C3**: `FromVerifier` 未知 verdict fail-fast + `ORCH_COORD_VERDICT_7004` 错误码
- **W2**: `validateFact` 改 `fmt.Errorf("orchtypes: FactPayload.Statement empty: %w", ErrObservationPayloadInvalid)` 包装
- **W3**: `clamp01` + `clamp01Coord` 合并为 `clamp01Float(v, onNaN)` 单函数
- **W6/I8**: `Partition` 末尾 `r.Overall = clamp01Float(r.ComputeOverallStrength(), 0.5)`（NaN safe）

注：C2/W8（`MatchKind` 签名收紧为 `(*UncertaintyReport)`）落点为后续 PR-B1，本 Requirement 不涉及。

---

### Requirement: D7-S9-A25 Execute Artifact Data Contract ✅ IMPLEMENTED

Phase 3 Execute 节点的最小风险入口（PR-C1）。提供跨域共享的 Artifact 数据契约：
ArtifactKind 4 类枚举 + SideEffectStatus 5 态 + SideEffectDetail 结构 + Artifact struct 5 字段升级。
本 Requirement 不引入执行链路变更（PR-C2..C7 范围），仅落地数据契约层。

**Priority:** P0
**Package:** `internal/shared/types/` + `internal/layers/orchestration/orchtypes/` + `internal/layers/orchestration/wavescheduler/`
**T:** D7-S9-A25-T01 … D7-S9-A25-T04
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase3-execute/`

#### Scenario: ArtifactKind 4 类枚举 + snake_case wire format

- GIVEN 定义 `shared/types.ArtifactKind{StateChangeCert, ResponseRecord, ProbeReport, ExperimentData}`
- WHEN 任意 Artifact 实例
- THEN `Kind` 字段为 4 枚举之一（uint8 0-3）
- AND `String()` 输出 snake_case wire format：`"state_change_cert"` / `"response_record"` / `"probe_report"` / `"experiment_data"`
- AND `MarshalJSON()` 输出字符串（不是数字），便于 D5 dashboard 字符串过滤
- AND `UnmarshalJSON()` 接收字符串，未知值 fail-fast 返回带 kind 名称的 error（不静默兜底）

#### Scenario: SideEffectStatus 5 态 + IsTerminal/NeedsAttention 派生

- GIVEN 定义 `shared/types.SideEffectStatus{None, Unknown, Inflight, Committed, RolledBack}`（string alias）
- WHEN 任意 Artifact / UncertaintyCoord 实例
- THEN `SideEffectStatus` 字段为 5 枚举之一（字符串值 `"none"` / `"unknown"` / `"inflight"` / `"committed"` / `"rolled_back"`）
- AND `IsTerminal()` 返回 true 当 status ∈ {None, Committed, RolledBack}
- AND `NeedsAttention()` 返回 true 当 status ∈ {Unknown, Inflight}
- AND `orchtypes.SideEffectStatus = types.SideEffectStatus` type alias，Phase 2 UncertaintyCoord 调用方零修改

#### Scenario: wavescheduler.Artifact struct 5 字段升级（v2 JSON 向后兼容）

- GIVEN `wavescheduler.Artifact` 扩展 5 字段：`Kind` / `SourcePlanID` / `AnomaliesCount` / `SideEffectStatus` / `SideEffectDetail`
- WHEN 任意 Artifact 实例序列化
- THEN 5 字段全部带 `omitempty` JSON tag
- AND zero value（Kind=0/SourcePlanID=""/AnomaliesCount=0/SideEffectStatus=""/*SideEffectDetail=nil）不出现在 JSON 输出
- AND v2 调用方（仅写 v2 字段）序列化结果与升级前**字节相同**
- AND Unmarshal 接收 v2 JSON（5 字段缺失）不报错，零值默认到 `Kind=0 (StateChangeCert)` / `SideEffectStatus="" (None)` 等

#### Scenario: 跨域类型上提 shared/types 打破 import cycle

- GIVEN orchtypes → workmodel → wavescheduler 单向依赖链
- WHEN Artifact.SideEffectStatus 需要与 UncertaintyCoord.SideEffectStatus 同类型（跨域 wire format 统一）
- AND 直接 wavescheduler → orchtypes 双向引用会破环
- THEN 把 `ArtifactKind` + `SideEffectStatus` + `SideEffectDetail` 上提到 `internal/shared/types/execute.go`（Phase 1 `MemoryEntry` precedent）
- AND `orchtypes` 提供 type alias + const re-export（`type SideEffectStatus = types.SideEffectStatus`）保持 Phase 2 调用方零修改
- AND `shared/types` → orchtypes 单向依赖，无 cycle

---

### Requirement: D7-S8-A22 Plan Data Contract + Planner (Phase 2 PR-B1) ✅ IMPLEMENTED

MUPS v4.3 Phase 2 Plan 节点的数据契约层（PR-B1）。提供 Plan struct + 4 类 `PlanKind` 枚举 + Planner interface + DefaultPlanner.MatchKind 4 规则分类器。本 Requirement 落地后：
- Phase 3 PR-C2 ChannelRouter 直接消费 `plan.Plan` 进行 PlanKind → Channel 路由；
- Phase 4 Verify 通过 `Plan.SourceObservationIDs` 反向追溯 `Observation`；
- Phase 5 Learn 通过 `Plan.AnomaliesCount` 调整 ReputationEvidence。

**Priority:** P0
**Package:** `internal/layers/orchestration/plan/`
**T:** D7-S8-A22-T01 … D7-S8-A22-T03
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase2-plan/`

#### Scenario: PlanKind 4 类枚举 + snake_case wire format

- GIVEN 定义 `plan.PlanKind{CommitmentPlan, ProtocolPlan, ScenarioPlan, ExplorationPlan}`（uint8 1-4）
- AND 零值 `KindUnset` 显式区分（避免与任意 4 命名值混淆）
- WHEN 任意 Plan 实例
- THEN `Kind` 字段为 4 命名枚举之一（`IsKnown()` 返回 true）
- AND `String()` 输出 snake_case wire format：`"commitment_plan"` / `"protocol_plan"` / `"scenario_plan"` / `"exploration_plan"`
- AND `MarshalJSON()` 输出字符串（KindUnset omitempty 返回 null，未知值返回 error），便于 D5 dashboard 字符串过滤
- AND `UnmarshalJSON()` 接收字符串，未知值 fail-fast 返回带 kind 名称的 error（不静默兜底到 KindUnset）
- AND `ParsePlanKind(s)` CLI 反向解析（case-insensitive + trim whitespace）

#### Scenario: Plan.SourceObservationIDs 必填 + Phase 4 Verify 反向追溯入口

- GIVEN Plan 是 Phase 2 Observe 节点产出的可调度结构
- WHEN 任意 Plan 进入 ChannelRouter（Phase 3 PR-C2）或 Verifier（Phase 4）
- THEN `Plan.SourceObservationIDs` 必填（空 → `Validate()` 失败，返回 `ErrPlanSourceObservationIDsRequired` + 错误码 `PLAN_LINEAGE_8002`）
- AND `NewPlan` 防御性拷贝外部 slice（外部 mutation 不影响 Plan 内字段）
- AND `Plan.ReverseLookupObservations([]ObservationLookup)` 按 ID 集合求交，返回 `ObservationLookup` 子集（Phase 4 Verify 反向追溯入口）
- AND 重复 ID 在 SourceObservationIDs 中不产生重复结果（defensive copy 保证）
- AND 空输入（nil / empty）返回 nil（边界保护）

#### Scenario: MatchKind 4 规则分类器 + uncertainty-first tie-break

- GIVEN `MatchKind(quantizedKind string, stepCount int, anomaliesCount int) PlanKind`
- WHEN 4 规则按优先级求值：
  - **Rule 1**: `quantizedKind == "intent_orchestrate"` OR `anomaliesCount >= 3` → `ExplorationPlan`
  - **Rule 2**: `stepCount == 1` → `CommitmentPlan`
  - **Rule 3**: `quantizedKind == "intent_command"` OR `stepCount <= 3` → `ProtocolPlan`
  - **Rule 4**: 默认 → `ScenarioPlan`
- THEN 不确定性优先 tie-break：Rule 1 始终优于 Rule 2/3（exploration 比 commitment 更可回滚）
- AND `DefaultPlanner.Plan(input PlanInput)` 走完整集成：
  - 空 `ObservationIDs` fast-fail 返回 `ErrPlanSourceObservationIDsRequired`
  - 强度公式 `strengthFloor(anomalies, observations) = 0.7 − 0.1·anomalies + min(observations·0.02, 0.2)`（cap [0, 1]）
  - `Validate()` 失败透传（`ErrPlanFailureCriteriaEmpty` + `PLAN_PP2_EMPTY_8020` 等）
  - `BlastRadius` 透传到输出 Plan
  - `AnomaliesCount` 透传（Phase 4 Verify 用以关联 Observation anomalies）

---

### Requirement: D7-S9-A26 Execute 4 Channel + ChannelRouter (Phase 3 PR-C2) ✅ IMPLEMENTED

MUPS v4.3 Phase 3 Execute 节点的 4 类执行通道 + ChannelRouter（PR-C2）。Channel interface 把 4 PlanKind 路由到对应的 Execute 策略：
- `CommitChannel` (CommitmentPlan) → 1-Step 同步 ToolRunner 调用 → `ArtifactStateChangeCert`
- `ProtocolChannel` (ProtocolPlan) → 顺序多步 + 失败 reverse-order rollback → `ArtifactResponseRecord`
- `ScenarioChannel` (ScenarioPlan) → 并行探测 (MaxParallel=5) + 多数派投票 → `ArtifactProbeReport`
- `ExplorationChannel` (ExplorationPlan) → 多 agent 并行 + 优先级排序 + PersistScope 派生 → `ArtifactExperimentData`

ChannelRouter 走 `Plan.Kind` → `ChannelRegistry.Get(kind)` → `Channel.Execute` 三段式分发；ToolRunner 是本地 interface（PR-C2 隔离 PR-C4 ToolSpec v3 + PR-C7 DefaultExecutor）。

**Priority:** P0
**Package:** `internal/layers/orchestration/execute/`
**T:** D7-S9-A26-T01 … D7-S9-A26-T05
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase3-channels/`

#### Scenario: ChannelRegistry 1:1 绑定 + ChannelRouter 4 PlanKind 路由

- GIVEN 4 Channel 实现（Commit/Protocol/Scenario/Exploration）+ ChannelRegistry
- WHEN Register 全部 4 Channel
- THEN `ChannelRegistry.Len() == 4` 且 0 冲突（PlanKind → Channel 1:1）
- AND `ChannelRegistry.Get(planKind)` 返回对应 Channel；未注册时返回 `ErrChannelNotFound` + 错误码 `EXEC_CHANNEL_9001`
- AND 重复 Register 同一 PlanKind 返回 `ErrChannelUnsupported` + 错误码 `EXEC_CHANNEL_9002`（wiring conflict）

- GIVEN ChannelRouter 装载 4 Channel
- WHEN `Route(ctx, plan, ChannelRequest{SessionID: ...})` 4 种 PlanKind 各跑一次
- THEN 返回 4 类 ArtifactKind 1:1 对应：
  - `CommitmentPlan` → `ArtifactStateChangeCert`（PR-C1 §1 wire format）
  - `ProtocolPlan` → `ArtifactResponseRecord`
  - `ScenarioPlan` → `ArtifactProbeReport`
  - `ExplorationPlan` → `ArtifactExperimentData`
- AND `Artifact.SourcePlanID == plan.ID`（反向追溯）
- AND `Artifact.SessionID == req.SessionID`（线程绑定）
- AND `Route(ctx, nil, _)` 返回 `ErrChannelPlanNil`；`Route(ctx, plan{Kind: KindUnset}, _)` 返回 `ErrChannelUnsupported`（defensive checks）

#### Scenario: CommitChannel 1-Step 同步 + IdempotencyKey 强制

- GIVEN CommitChannel 装载 ToolRunner
- WHEN 单 Step Plan（`len(Steps) == 1`）+ IdempotencyKey 非空
- AND ToolRunner 返回 `ToolResult{ExitCode: 0}`
- THEN Artifact 字段：
  - `Kind == ArtifactStateChangeCert`
  - `SideEffectStatus == SideEffectCommitted`
  - `SideEffectDetail.IdempotencyKey == step.IdempotencyKey`
  - `SideEffectDetail.ConfirmedAt > SentAt`（确认时间戳）

- WHEN ToolRunner 返回 `context.DeadlineExceeded`（内或外 ctx）
- THEN `SideEffectStatus == SideEffectInflight`（PR-C3 StrategyDecider 路由到 `StrategyAskNow`）

- WHEN 多 Step Plan 或 Step 缺 IdempotencyKey
- THEN 返回 `ErrChannelStepCountMismatch` + 错误码 `EXEC_CHANNEL_9003`（fail-fast，避免无幂等性的 side effect）

#### Scenario: ProtocolChannel 顺序多步 + reverse-order rollback

- GIVEN ProtocolChannel 装载 ToolRunner
- WHEN 3 步 Plan（login → fetch → parse）全部成功
- THEN Artifact 字段：
  - `Kind == ArtifactResponseRecord`
  - `SideEffectStatus == SideEffectCommitted`
  - `Summary` 含 `step_0` / `step_1` / `step_2` 三个 step 输出（顺序）

- WHEN 第 2 步（fetch）失败
- THEN 已执行的 step 1（login）被 reverse-order rollback（ToolRunner 收到 `__rollback: true` 标记）
- AND Artifact `SideEffectStatus == SideEffectRolledBack`
- AND Runner 调用次数 = 3（login + fetch-failed + login-rollback）

- WHEN `len(Steps) == 0`
- THEN 返回 `ErrChannelStepCountMismatch`（protocol 必须有 ≥1 步）

#### Scenario: ScenarioChannel 并行探测 + 多数派投票

- GIVEN ScenarioChannel 装载 ToolRunner + `MaxParallel=5`
- WHEN 5 个 probe Step 并行执行
- THEN 至少 2 个 probe 同时 in-flight（验证 parallelism，避免串行退化）
- AND Artifact `Kind == ArtifactProbeReport`
- AND `SideEffectStatus == SideEffectNone`（read-only 探测，无 side effect）

- GIVEN 5 个 probe 中 3 个成功 / 2 个失败
- WHEN 调用 `Execute`
- THEN `Summary` 含 `"3/5 probes succeeded"` + threshold `> len/2 = 2`
- AND `ExitCode == 0`（多数派 pass）

- WHEN 5 个 probe 中 2 个成功 / 3 个失败（多数派 reject）
- THEN `ExitCode == 1` + 错误返回（`ErrChannelStepCountMismatch` 复用）

#### Scenario: ExplorationChannel 多 agent + 优先级排序 + PersistScope 派生

- GIVEN ExplorationChannel 装载 ToolRunner + `MaxParallel=3`
- WHEN 3 个 Step 并行执行
- THEN Artifact `Kind == ArtifactExperimentData`
- AND `SideEffectStatus` 由 `Plan.BlastRadius.PersistScope` 派生：
  - `PersistTransient` → `SideEffectNone`（read-only 沙箱）
  - `PersistSession` / `PersistPermanent` → `SideEffectCommitted`
  - 未知 scope → `SideEffectUnknown`（Phase 4 必须 verify）
- AND 容忍部分失败（free-fork 语义）：1/2 success 不触发 error
- AND 优先级排序：(1) 无 error → (2) `duration` 短 → (3) `EstimatedTokens` 少
- AND `Summary` 含 `"top_result: <winning_tool>"` + `"<n>/<m> succeeded"`

---

### Requirement: D7-S11-A36 Learn 节点数据契约（LearningAsset 5 类 + AssetContent + LearningClass 5 枚举）✅ IMPLEMENTED

MUPS v4.3 Phase 5 Learn 节点的核心数据契约（PR-E1）。Learn 节点必须有统一的学习资产抽象：LearningAsset struct 15 字段不可变值对象 + 5 类 AssetContent（★1-5 不同强度）+ LearningClass 5 态 typed enum（含 LearningPending ⭐新增）。本 Requirement 不引入 Memory 通道（PR-E4 范围）、不引入 Learner 接口（PR-E5 范围），仅落地数据契约层。

**Priority:** P0
**Package:** `internal/layers/orchestration/learn/{learning_asset,asset_content}.go` + `internal/shared/types/learning.go`
**T:** D7-S11-A36-T01 … D7-S11-A36-T03
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/`

#### Scenario: LearningClass 5 态 typed enum + 空字符串零值兼容

- GIVEN 定义 `LearningClass{LearningSOP=1, LearningProtocol=2, LearningKnowledge=3, LearningConclusion=4, LearningPending=5}`（KindUnset=0 默认）
- WHEN 调用 `LearningClass.String()` 5 个值
- THEN 分别返回 `"sop"` / `"protocol"` / `"knowledge"` / `"conclusion"` / `"pending"`
- AND `ParseLearningClass("sop")` → `LearningSOP`；解析 `"unknown"` → fail-fast 错误（禁用 LearningUnknown）
- AND `MarshalJSON` 输出 wire 字符串；`UnmarshalJSON("")` → `LearningSOP`（空字符串零值兼容 Phase 4 VerdictKind precedent）

#### Scenario: LearningAsset 15 字段 + 不可变 + 自动时间戳

- GIVEN 调用 `NewLearningAsset(id, sessionID, class, content, assetKey)`
- WHEN 任一必填字段为空（SessionID="" / class=LearningUnknown / content=nil / assetKey=""）
- THEN 返回 `ErrAssetIncomplete`（fail-fast）
- AND `CreatedAt = now`, `ExpiryAt = now + 24h`（`DefaultAssetTTL` 默认），`ContentHash = hashContent(content)`（SHA-256 hex 截断 16）
- AND 内部存储 `Content` deep copy（外部 mutation 不污染）
- AND `With*` setter 方法返回新对象，原对象不变

#### Scenario: 5 类 AssetContent 必填字段校验 + SchemaVersion + ByteSize

- GIVEN `SOPAssetContent{Name: "test", Steps: []string{"step1"}}`（Steps ≥ 1）
- WHEN 调用 `Validate()`
- THEN 返回 nil；`SchemaVersion() == "1.0.0"`；`ByteSize() > 0`（字节估算）

- GIVEN `SOPAssetContent{Name: ""}` 缺 Name
- WHEN 调用 `Validate()`
- THEN 返回 `ErrAssetIncomplete`（Name 必填）

- GIVEN `ProtocolAssetContent{Trigger: ""}` 缺 Trigger
- WHEN 调用 `Validate()`
- THEN 返回 `ErrAssetIncomplete`

- GIVEN `KnowledgeAssetContent{Topic: "", Hypothesis: ""}`
- WHEN 调用 `Validate()`
- THEN 返回 `ErrAssetIncomplete`（Topic + Hypothesis 必填）

- GIVEN `ConclusionAssetContent{Statement: ""}` 缺 Statement
- WHEN 调用 `Validate()`
- THEN 返回 `ErrAssetIncomplete`

- GIVEN `PendingAssetContent{IndeterminateReason: ""}` 缺 IndeterminateReason
- WHEN 调用 `Validate()`
- THEN 返回 `ErrAssetIncomplete`

- GIVEN `PendingAssetContent{MVEState: nonNil, Question: ""}`（⭐MVE checkpoint）
- WHEN 调用 `Validate()`
- THEN 返回 `ErrAssetIncomplete`（Question 必填）

- GIVEN `PendingAssetContent{RetryAttempts: -1}` 越界
- WHEN 调用 `Validate()`
- THEN 返回 `ErrAssetIncomplete`（RetryAttempts ∈ [0, MaxRetries]）

#### Scenario: LearningClass 跨域类型上提 + type alias

- GIVEN `internal/shared/types/learning.go` 定义 `type LearningClass uint8` + 5 枚举
- AND `internal/layers/orchestration/learn/learning_asset.go` 添加 `type LearningClass = types.LearningClass` type alias
- WHEN 调用 `learn.LearningSOP` 或 `types.LearningSOP`
- THEN 两者相等（type alias 不引入新类型）；无 import cycle
- AND Phase 3 SideEffectStatus + Phase 4 VerdictKind precedent 一致

---

### Requirement: D7-S11-A37 ReputationEvidence + Bayesian Update + Wilson Score + G8-1 修复 ✅ IMPLEMENTED

MUPS v4.3 Phase 5 Learn 节点的信誉引擎（PR-E2）。Learn 节点必须有跨会话信誉累积机制：ReputationEvidence struct 12 字段不可变值对象 + BayesianUpdate 函数（不可变 prior → 新 posterior）+ Wilson Score 95% 置信区间 + ⭐G8-1 修复（Phase 4 VerifyWithRetry parse failure → INDETERMINATE 的 Learn 端延伸，避免 LLM 输出格式异常污染用户信誉）。

**Priority:** P0
**Package:** `internal/layers/orchestration/learn/reputation_evidence.go`（v4.3 post-cleanup: BayesianUpdate 已并入此文件,`bayesian_update.go` 删除）
**T:** D7-S11-A37-T04 … D7-S11-A37-T05
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/`

#### Scenario: ReputationEvidence 12 字段 + 冷启动除零防御 + Wilson Score

- GIVEN 调用 `NewReputationEvidence(sessionID, TrackModeDeveloper)`
- WHEN SessionID 为空 / TrackMode 无效
- THEN 返回 `ErrReputationEvidenceInvalid`（fail-fast）
- AND 冷启动默认：Alpha=0, Beta=0, Mean=0, Variance=0, ConfidenceLow=0, ConfidenceHigh=1, VerifierFailureCount=0, IndeterminateCount=0
- AND `LastUpdated = now`, `UpdateCount = 0`

- GIVEN Alpha=10, Beta=2（运行一段时间）
- WHEN 计算派生指标
- THEN `Mean = 10 / 12 ≈ 0.833`；`Variance = 10*2 / (12²*13) ≈ 0.0107`；Wilson Score 95% 置信区间下界 ≈ 0.55，上界 ≈ 0.96

#### Scenario: BayesianUpdate 不可变 + Pass/Partial/Fail → α/β++

- GIVEN `prior = NewReputationEvidence("sess_1", TrackModeDeveloper)` 初始 Alpha=0, Beta=0
- AND 调用 `BayesianUpdate(prior, Verdict{Kind: VerdictPass, Confidence: 0.9})`
- WHEN 返回 `next`
- THEN `prior.Alpha` 仍为 0（不可变）
- AND `next.Alpha = 1`, `next.Beta = 0`, `next.Mean ≈ 1.0`, `next.UpdateCount = 1`
- AND Partial 同 Pass（Alpha++）；Fail → `next.Beta = 1`, `next.Alpha = 0`

#### Scenario: G8-1 修复 — INDETERMINATE verifier_parse_failure 不污染 α/β

- GIVEN `prior` Alpha=5, Beta=3（Developer Beta(5,3) + 累积）
- AND 调用 `BayesianUpdate(prior, Verdict{Kind: VerdictIndeterminate, IndeterminateReason: "verifier_parse_failure"})`
- WHEN 返回 `next`
- THEN `next.Alpha == 5`（不变），`next.Beta == 3`（不变）— **绝不动 α/β**
- AND `next.VerifierFailureCount = 1`（⭐G8-1 修复：仅递增 VerifierFailureCount）
- AND `next.Mean == prior.Mean`（不变 ≈ 0.625）

- GIVEN `prior` Alpha=5, Beta=3
- AND 调用 `BayesianUpdate(prior, Verdict{Kind: VerdictIndeterminate, IndeterminateReason: "tool_timeout"})`（其他 INDETERMINATE）
- WHEN 返回 `next`
- THEN `next.Alpha == 5`，`next.Beta == 3`（不变）
- AND `next.IndeterminateCount = 1`（保持原行为不污染 α/β）

#### Scenario: 50 次 Pass 后 Mean 收敛 ≈ 1.0

- GIVEN `prior` Alpha=0, Beta=0（冷启动）
- WHEN 连续调用 `BayesianUpdate` 50 次 VerdictPass
- THEN `next.Alpha = 50`, `next.Beta = 0`, `Mean = 50/50 = 1.0`
- AND Wilson Score 95% ConfidenceLow ≈ 0.93（接近 1.0）

---

### Requirement: D7-S11-A38 AdaptivePrior + DefaultPriors + BuildAdaptivePrior ✅ IMPLEMENTED

MUPS v4.3 Phase 5 Learn 节点的先验工厂（PR-E3）。LP-1 闭环的"读"侧入口：Learn 节点 → 下一轮 Observe。AdaptivePrior 把 ReputationEvidence + DefaultPriors Bayesian 合并为统一的 Beta(α,β) 先验，注入到 3 个 Observer 子模块（IntentQuantizer / HistoricalDetector / RuleClassifier）。

**Priority:** P0
**Package:** `internal/layers/orchestration/learn/adaptive_prior.go`
**T:** D7-S11-A38-T06 … D7-S11-A38-T07
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/`

#### Scenario: AdaptivePrior + BetaPrior + InjectTarget 3 枚举

- GIVEN `AdaptivePrior{Reputation: nil, PriorBeta: DefaultDeveloperPrior, InjectTargets: DefaultInjectTargets}`
- WHEN 验证字段
- THEN `PriorBeta.String() == "Beta(5,3)"`（Developer trackMode）
- AND `InjectTargets` 长度 = 3：`InjectIntentQuantizer` / `InjectHistoricalDetector` / `InjectRuleClassifier`
- AND 无 `With*` setter（不可变）

#### Scenario: DefaultDeveloperPrior Beta(5,3) + DefaultOperatorPrior Beta(8,1)

- GIVEN `DefaultDeveloperPrior` 常量
- WHEN 验证
- THEN `Alpha = 5`, `Beta = 3`, `Mean ≈ 0.625`（doc 25 §四 developer 基线）

- GIVEN `DefaultOperatorPrior` 常量
- WHEN 验证
- THEN `Alpha = 8`, `Beta = 1`, `Mean ≈ 0.889`（operator 更倾向成功）

#### Scenario: BuildAdaptivePrior Bayesian 合并 + 双重兜底

- GIVEN `rep = nil`（cold start）
- AND 调用 `BuildAdaptivePrior(nil, TrackModeDeveloper)`
- WHEN 返回 `prior`
- THEN `prior.Reputation == nil`，`prior.PriorBeta == DefaultDeveloperPrior`（Beta(5,3)）
- AND `prior.InjectTargets == DefaultInjectTargets`（3 个目标）

- GIVEN `rep.Alpha = 3, rep.Beta = 0`（累积 3 次 Pass）
- AND 调用 `BuildAdaptivePrior(rep, TrackModeDeveloper)`
- WHEN 返回 `prior`
- THEN `prior.PriorBeta.Alpha = 5 + 3 = 8`（merged）
- AND `prior.PriorBeta.Beta = 3 + 0 = 3`（merged）
- AND `prior.PriorBeta.String() == "Beta(8,3)"`

- GIVEN `rep != nil, trackMode = ""`（空字符串兜底）
- WHEN 调用 `BuildAdaptivePrior(rep, "")`
- THEN 使用 `DefaultDeveloperPrior`（fail-safe，避免 panic）

---

### Requirement: D7-S11-A39 Memory 3 通道接口 + 3 实现（LP-2 隔离） ✅ IMPLEMENTED

MUPS v4.3 Phase 5 Learn 节点的记忆通道（PR-E4）。LP-2 隔离原则：5 类 LearningAsset ↔ 3 通道 Memory partition。Skill（确定性 ★4-5）/ Feedback（软知识 ★2-3）/ Scheduled（延迟重试 ★1）。通道错位（e.g. SOP 写入 FeedbackMemory）→ fail-fast `ErrAssetClassMismatch`。

**Priority:** P0
**Package:** `internal/layers/orchestration/learn/memory.go`
**T:** D7-S11-A39-T08 … D7-S11-A39-T09
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/`

#### Scenario: Memory interface 4 方法 + MemoryChannel 3 枚举 + MemoryFilter 4 字段

- GIVEN `Memory` interface（Store/Retrieve/Delete/List）
- WHEN 3 实现（SkillMemory / FeedbackMemory / ScheduledMemory）签名匹配
- THEN 编译通过；`MemoryChannel.String()` 返回 `"skill"` / `"feedback"` / `"scheduled"`

- GIVEN `MemoryFilter{Class: LearningSOP, SessionID: "sess_1", MinStrength: 3, Expired: false}`
- WHEN 调用 `SkillMemory.List(ctx, filter)`
- THEN 仅返回 Skill 通道中 Class=LearningSOP ∧ SessionID="sess_1" ∧ Strength≥3 ∧ 未过期的 asset

#### Scenario: SkillMemory SOP/Protocol 路由 + 错位 fail-fast

- GIVEN `SkillMemory` 装载 1 个 `LearningAsset{Class: LearningSOP}`
- AND 调用 `SkillMemory.Store(ctx, assetClassLearningSOP)`
- THEN 成功 store

- AND 调用 `SkillMemory.Store(ctx, assetClassLearningKnowledge)`
- THEN 返回 `ErrAssetClassMismatch`（fail-fast：Knowledge 路由到 Feedback）

#### Scenario: FeedbackMemory Knowledge/Conclusion 路由

- GIVEN `FeedbackMemory` 装载 1 个 `LearningAsset{Class: LearningKnowledge}`
- AND 调用 `FeedbackMemory.Store(ctx, assetKnowledge)`
- THEN 成功 store
- AND `FeedbackMemory.Store(ctx, assetSOP)` → `ErrAssetClassMismatch`

#### Scenario: ScheduledMemory Pending 路由 + ScheduledRetry envelope

- GIVEN `ScheduledMemory.Store(ctx, assetPending)` 装载 `LearningAsset{Class: LearningPending}`
- WHEN 验证 envelope
- THEN `TriggerAt = asset.ExpiryAt`（默认）
- AND `MaxRetries = 3`（DefaultScheduledMaxRetries，匹配 PendingAssetContent validator 上界）
- AND `RetryCount = 0`（初始）

- GIVEN Pending + `TriggerAt = now - 1h`（已到期）
- WHEN 调用 `ScheduledMemory.ListDue(now)`
- THEN 返回该 ScheduledRetry

- GIVEN ScheduledRetry{RetryCount: 3, MaxRetries: 3}
- WHEN 调用 `IsExhausted()`
- THEN 返回 true

#### Scenario: 并发安全 sync.RWMutex

- GIVEN 10 goroutines 并发 `Store` + 10 goroutines 并发 `List`（21 ops × 10 = 210 ops）
- WHEN `go test -race ./internal/layers/orchestration/learn/...`
- THEN 0 race detector warnings（LP-2 跨 channel 并发安全）

---

### Requirement: D7-S11-A40 Learner interface + DefaultLearner + LP-1 闭环 ✅ IMPLEMENTED (T13 PARTIAL)

MUPS v4.3 Phase 5 Learn 节点的节点级入口（PR-E5）。Learner 是 LP-1 闭环的核心：Learn（写）+ Inject（读）+ ScheduledTick（重试调度）。DefaultLearner 整合 AssetBuilder + 3 Memory + ReputationStore + BayesianUpdater，提供 5 节点管道 Observe → Plan → Execute → Verify → Learn → 下一轮 Observe 的关键回写入口。

**Priority:** P0
**Package:** `internal/layers/orchestration/learn/{learner,asset_builder,reputation_store}.go`
**T:** D7-S11-A40-T10 … D7-S11-A40-T13
**Design:** `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/`

#### Scenario: Learner interface 3 方法 + Learn 5 步流程

- GIVEN `DefaultLearner` 装载 SkillMem + FeedbackMem + ScheduledMem + ReputationStore + AssetBuilder
- AND 调用 `Learn(ctx, LearnRequest{SessionID: "sess_1", Verdict: {Kind: VerdictPass, ...}, Plan: planWithSteps, Artifact: waveschedulerArtifact})`
- WHEN 流程执行
- THEN 步骤：(1) `class = classFromVerdictKind(Pass) = LearningSOP`；(2) `asset = AssetBuilder.Build(...)`；(3) `SkillMem.Store(asset)`（LP-2 隔离）；(4) `Reputation.Get("sess_1") + BayesianUpdate + Reputation.Update`（LP-3）；(5) 返回 `[*LearningAsset{asset}]`

- AND VerdictKind → LearningClass 路由：
  - `VerdictPass` → `LearningSOP`（★5）
  - `VerdictPartial` → `LearningProtocol`（★4）
  - `VerdictFail` → `LearningKnowledge`（★3）
  - `VerdictIndeterminate` → `LearningPending`（★1，⭐新增）

#### Scenario: Inject LP-1 闭环入口（in-package 闭环验证）

- GIVEN `DefaultLearner` 装载 `InMemoryReputationStore`
- AND 连续 `Learn(ctx, VerdictPass)` × 3
- WHEN 调用 `Inject(ctx, "sess_1")`
- THEN `ReputationStore.Alpha = 3, Beta = 0`（LP-3 Bayesian 累积）
- AND `BuildAdaptivePrior` 合并：`PriorBeta.Alpha = 5 + 3 = 8, Beta = 3 + 0 = 3`
- AND 返回 `AdaptivePrior{PriorBeta: Beta(8,3), InjectTargets: 3 个目标}`

- GIVEN Cold start（`ReputationStore.Get("sess_1")` → nil）
- AND 调用 `Inject(ctx, "sess_1")`
- THEN 返回 `AdaptivePrior{PriorBeta: Beta(5,3) = DefaultDeveloperPrior, InjectTargets: 3 个}`

#### Scenario: ScheduledTick 流程 + MaxRetries 耗尽 → FeedbackMemory escalation

- GIVEN `ScheduledMemory` 装载 1 个 ScheduledRetry（`TriggerAt = now - 1h`，`RetryCount = 2, MaxRetries = 3`）
- AND 调用 `ScheduledTick(ctx)`
- WHEN 流程执行
- THEN `RetryCount++ → 3, LastRetryAt = now, TriggerAt = now + 5min`（下次重试延迟 5 分钟）

- AND 给定 ScheduledRetry `RetryCount = 3 == MaxRetries = 3`（已耗尽）
- AND 调用 `ScheduledTick(ctx)`
- WHEN 流程执行
- THEN 写入 `FeedbackMemory` 一个 `KnowledgeAssetContent{Topic: "scheduled_retry_exhausted"}`（警告资产）
- AND 从 `ScheduledMemory` 删除该 ScheduledRetry

#### Scenario: AssetBuilder 5 类 Content 构造 + AssetKey 幂等

- GIVEN `AssetBuilder.Build(ctx, req, LearningSOP)`
- WHEN 构造 SOPAssetContent
- THEN `AssetKey = "sop:{PlanKind}:{hash}"` 格式（如 `"sop:commitment:abc123def456..."`）
- AND `hash = hashContentBytes(content)` SHA-256 hex 截断 16
- AND `Strength = 5`（classToStrength SOP→5）

- AND 同 Content 两次 Build → 同 hash（幂等）

#### Scenario: ReputationStore interface + InMemoryReputationStore

- GIVEN `InMemoryReputationStore.Update(ctx, nil)` 或 `Update(ctx, &ReputationEvidence{SessionID: ""})`
- WHEN 调用
- THEN 返回 `ErrReputationStoreUnavailable`（fail-fast）

- AND `InMemoryReputationStore.Get(ctx, "absent_session")` → `(nil, nil)`（cold start）
- AND `InMemoryReputationStore.List(ctx, TrackModeDeveloper, 10)` 返回 ≤ 10 个 Developer trackMode 信誉

#### Scenario: T13 PARTIAL — LP-1 闭环 in-package 测试（Observe 跨域 wiring 留待 Phase 6）

- GIVEN in-package 测试 `learner_test.go`:
  - `TestLP1_ClosedLoop_LearnThenInject`：3 × Learn(VerdictPass) → Alpha=3 → Inject → `PriorBeta = Beta(8,3)` ✅ PASS
  - `TestLP1_ClosedLoop_INDETERMINATE_DoesNotPolluteAlphaBeta`：Learn(INDETERMINATE + verifier_parse_failure) → α/β 不变 + `PriorBeta` 不变 + `VerifierFailureCount = 1` ✅ PASS

- AND **Phase 6 集成（out of scope）**：
  - `Orchestrator.ProcessMessage` 在 `ObserveNode.All()` 之前调用 `Learner.Inject(ctx, sessionID)`
  - `IntentQuantizer.QuantizeWithPrior` / `AnomalyDetector.HistoricalDetector.DetectWithPrior` / `RuleClassifier.ClassifyWithPrior` 跨域 wiring
  - `tests/integration/d7/learn_observe_closure_test.go` 端到端 5 节点管道 LP-1 闭环集成测试

### Requirement: D7-S12-A41 ObserveRequest + IntentQuantizer + AnomalyDetector + ClassifyWithPrior (Observer 子模块 + WithPrior 变体) ✅ IMPLEMENTED

MUPS v4.3 Phase 6 Observe-Learner 跨域闭环集成（PR-F1）。闭环 Phase 5 PR-E5 E5.4 T13 PARTIAL 中"Observe 跨域 wiring"段。Observe 节点需要 WithPrior 变体才能消化 LP-1 注入的 AdaptivePrior：3 通道观察（intent / anomaly / classify）是 Phase 2 PR-A1 4×4 象限的具体 Observer 实现。

**Priority:** P0
**Package:** `internal/layers/orchestration/orchtypes/{observe_request,intent_quantizer,anomaly_detector}.go` + `internal/layers/orchestration/decisionplanning/classifier.go`
**T:** D7-S12-A41-T01 … D7-S12-A41-T03
**Design:** `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase6-observe-learner-wiring/`

#### Scenario: ObserveRequest + EffectivePrior 兜底 DefaultDeveloperPrior

- GIVEN `orchtypes.NewObserveRequest("sess_1", "hello", nil)`
- WHEN 构造调用
- THEN `ObserveRequest{SessionID: "sess_1", Message: "hello", Prior: nil}` 返回
- AND `ObserveRequest{EffectivePrior()}` 返回 `*learn.AdaptivePrior{PriorBeta: Beta(5,3)}`（DefaultDeveloperPrior fail-safe）
- AND `ObserveRequest{}.Validate()` 对空 SessionID / 空 Message 返回 `fmt.Errorf`

- GIVEN `prior != nil`（已注入）
- THEN `EffectivePrior()` 直接返回该 prior（不重新构造），保持 prior 不变（immutable pattern）

#### Scenario: IntentQuantizer.QuantizeWithPrior — Mean 作为 confidence 乘数

- GIVEN `IntentQuantizer` 装载 DefaultConfig
- AND 消息 `"hello"`（匹配 greeting fast pattern）→ baseline `IntentPayload{Class: IntentClassFact, Confidence: 95, Reason: "greeting"}`
- WHEN 调用 `QuantizeWithPrior(ctx, "hello", prior)` 且 `prior.PriorBeta.Mean() = 8/11 ≈ 0.727`
- THEN 返回 `IntentPayload{Class: IntentClassFact, Confidence: int(95 × 0.727) = 69, Reason: "..."}`
- AND baseline `Quantize()` 不被改动（immutable）
- AND `prior.PriorBeta.Mean() == 0`（冷启动）→ 返回 baseline confidence 不变

#### Scenario: HistoricalDetector.DetectWithPrior — threshold = 0.5 × Mean

- GIVEN `Anomaly{Severity: 0.4, Category: CatBusiness}`
- AND `prior.PriorBeta.Mean() = 8/11 ≈ 0.727`
- WHEN 调用 `DetectWithPrior(ctx, anomalies, prior)`
- THEN `threshold = 0.5 × 0.727 ≈ 0.364`
- AND `Severity (0.4) > threshold (0.364)` → `AnomalyReport{TriggeredSystemAnomaly: true, ...}`
- AND `prior == nil` → `threshold = 0.5` baseline

#### Scenario: RuleClassifier.ClassifyWithPrior — Mean 作为 confidence 乘数（接口扩展）

- GIVEN `IntentClassifier` 接口扩展 `ClassifyWithPrior(ctx, message, prior) (IntentClassification, error)` 方法
- AND `RuleClassifier.ClassifyWithPrior` 在 `Classify` baseline 上叠加 `prior.PriorBeta.Mean()` 乘数
- AND `ShadowClassifier.ClassifyWithPrior` 委托给底层 `rule.ClassifyWithPrior`（shadow 异步路径保持无 prior 路径以保证样本可比性）
- THEN ProcessMessage 调用 `classifier.ClassifyWithPrior(ctx, req.Message, observeReq.Prior)`（替换原 `Classify`）

### Requirement: D7-S12-A42 SessionOrchestrator.buildObserveRequest + WithLearner + 3 层 fail-safe ✅ IMPLEMENTED

MUPS v4.3 Phase 6 Observe-Learner 跨域闭环集成（PR-F2）。LP-1 节点级落地最后一步。Learner.Inject → AdaptivePrior → Orchestrator → 3 Observer 子模块形成 5 节点管道 Observe → Plan → Execute → Verify → Learn → 下一轮 Observe 的关键回写路径。

**Priority:** P0
**Package:** `internal/layers/orchestration/sessionorchestrator/orchestrator.go`
**T:** D7-S12-A42-T04 … D7-S12-A42-T05
**Design:** `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase6-observe-learner-wiring/`

#### Scenario: WithLearner option + 懒构造 default learner

- GIVEN `NewSessionOrchestrator(cfg, exec, WithLearner(learner))`
- WHEN 调用
- THEN `orch.learner = learner` 注入成功
- AND `orch.learner == nil` 时（无 WithLearner 调用）→ ProcessMessage 走 EffectivePrior() 兜底路径

#### Scenario: buildObserveRequest 3 层 fail-safe

- GIVEN `o.learner == nil`（无 WithLearner 调用）
- WHEN 调用 `buildObserveRequest(ctx, req)`
- THEN `prior == nil`，但 `ObserveRequest{EffectivePrior()}` 返回 DefaultDeveloperPrior Beta(5,3)

- GIVEN `o.learner != nil` 但 `o.learner.Inject(ctx, sessionID)` 返回 error
- WHEN 调用 `buildObserveRequest(ctx, req)`
- THEN 记录 `slog.Warn("orchestrator: learner.Inject failed, using DefaultDeveloperPrior")`
- AND `prior == nil`，但 `ObserveRequest{EffectivePrior()}` 返回 DefaultDeveloperPrior Beta(5,3)

- GIVEN `o.learner != nil` 且 `Inject(ctx, sessionID)` 成功返回 `*AdaptivePrior`
- WHEN 调用 `buildObserveRequest(ctx, req)`
- THEN `prior = injected` 传给 `ObserveRequest`
- AND 下一轮 ProcessMessage 看到 prior.PriorBeta 已包含 Reputation Bayesian 累积

#### Scenario: ProcessMessage 在 classifySpan 之前调用 buildObserveRequest

- GIVEN ProcessMessage 启动 `classifySpan := o.startSpan(...)`
- AND 调用 `o.buildObserveRequest(ctx, req)` 在 classifySpan 之前
- AND `observeReq.Prior = prior` 传给 `classifier.ClassifyWithPrior(ctx, req.Message, prior)`
- AND `sessionSpan.SetAttributes("learn.prior.alpha" + "learn.prior.beta")` 记录 prior 到 D5 span
- THEN classifier 收到的 prior 与 ReputationStore 当前状态一致（LP-1 闭环）

### Requirement: D7-S12-A43 LP-1 闭环 E2E 集成测试 ✅ IMPLEMENTED

MUPS v4.3 Phase 6 Observe-Learner 跨域闭环集成（PR-F3）。端到端验证 LP-1 + LP-5 跨域集成，闭环 Phase 5 PR-E5 T13 PARTIAL。

**Priority:** P0
**Package:** `tests/integration/d7/learn_observe_closure_test.go`
**T:** D7-S12-A43-T06
**Design:** `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase6-observe-learner-wiring/`

#### Scenario: LP-1 闭环 Pass × 3 累积 Alpha=3 → Round 2 观察 Beta(8,3)

- GIVEN `newLP1Fixture(t, "sess-lp1-pass")` 创建 SessionOrchestrator + WithLearner + InMemoryReputationStore
- WHEN Round 1 `processOnce("hello")`
- THEN `classifier.lastPrior.PriorBeta = Beta(5,3)`（冷启动 DefaultDeveloperPrior, Mean = 5/8 = 0.625）

- AND 调用 `learner.Learn(ctx, VerdictPass) × 3`
- THEN `rep.Get("sess-lp1-pass")` 返回 `{Alpha: 3, Beta: 0}`（LP-3 Bayesian 累积）

- WHEN Round 2 `processOnce("another msg")`
- THEN `classifier.lastPrior.PriorBeta = Beta(8,3)`（merged Beta(5+3, 3+0), Mean = 8/11 ≈ 0.727）

#### Scenario: LP-1 闭环 INDETERMINATE + verifier_parse_failure 不污染 α/β（⭐G8-1）

- GIVEN `newLP1Fixture(t, "sess-lp1-parsefail")`
- WHEN Round 1 `processOnce("hi")`（冷启动 prior Beta(5,3)）
- AND `learner.Learn(ctx, Verdict{IndeterminateReason: "verifier_parse_failure"})`
- THEN `rep.Get(...)` 返回 `{Alpha: 0, Beta: 0, VerifierFailureCount: 1}`（⭐G8-1 fix: 不污染 α/β）

- WHEN Round 2 `processOnce("hi again")`
- THEN `classifier.lastPrior.PriorBeta = Beta(5,3)`（prior 未变化，因为 α/β 没动）

#### Scenario: LP-1 闭环 PendingAsset 路由 ScheduledMemory（⭐LP-2）

- GIVEN `learner.Learn(ctx, Verdict{IndeterminateReason: "env_limited"})`
- WHEN Learn 调用
- THEN 返回 `asset.Class = LearningPending`（PendingAssetContent）
- AND `ScheduledMemory.Retrieve(assetKey)` 命中，`SkillMemory.Retrieve` 与 `FeedbackMemory.Retrieve` 返回 nil（LP-2 隔离）
- AND `ScheduledMemory.ListDue(now + 25h)` 包含该 asset（TriggerAt 默认 ExpiryAt = now + 24h）

#### Scenario: 5-Node Pipeline 端到端（⭐LP-5 反向追溯）

- GIVEN `newLP1Fixture(t, "sess-5node-e2e")` + Round 1 ProcessMessage
- AND Plan stub `Plan{ID: "plan_5node", SourceObservationIDs: ["obs_e2e_1"], ...}`
- AND Artifact stub `Artifact{TaskID: "art_5node", ...}`
- AND Verdict `{Kind: VerdictPass, SourceID: "art_5node"}`
- WHEN `learner.Learn(ctx, LearnRequest{Verdict, Plan, Artifact, Observations})`
- THEN Asset 的 `SourceSessionIDs` 包含 plan.SessionID（LP-5 跨域血缘）
- AND Verdict.SourceID = art_5node（Artifact 反向追溯）
- AND `rep.Get(...)` 返回 `{Alpha: 1, Beta: 0}`（VerdictPass → α++）
- AND `SkillMemory.Retrieve(assetKey)` 命中（Compliance verdict → SOP → SkillMemory）

---

### Requirement: D7-S13-A47 SessionOrchestrator.processAutoClose — Verify→Learn 运行时闭环 ✅ IMPLEMENTED

MUPS v4.3 Phase 7 运行时 5 节点闭环（PR-7.1）。闭环 Phase 6 PR-F3 E2E 测试中手工模拟的 Verify→Learn 步骤：把 SessionOrchestrator.ProcessMessage 末尾的 LP-1 闭环 wire 到生产运行时，让 ReputationStore 真正跨会话累积。生产环境调用 ProcessMessage 时，Verify 节点的 Verdict 自动触发 Learn 节点的 BayesianUpdate + ReputationStore 更新。

**Priority:** P0
**Package:** `internal/layers/orchestration/sessionorchestrator/{autoclose,orchestrator}.go`
**T:** D7-S13-A47-T01 … D7-S13-A47-T03
**Design:** `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase7-verify-auto-close/`

#### Scenario: processAutoClose synthesizes VerdictPass from complete event

- GIVEN orchestrator with `learner != nil`
- WHEN execution path emits events `[thinking, text, complete]` and closes channel
- THEN `processAutoClose` synthesizes `workmodel.Verdict{Kind: VerdictPass, SourceID: "autoclose:{sessionID}:{nanosecond}"}`
- AND calls `o.learner.Learn(ctx, LearnRequest{Verdict: VerdictPass, SessionID})` asynchronously
- AND the synchronous return value of `ProcessMessage` is the channel proxy (non-blocking)

#### Scenario: processAutoClose synthesizes VerdictFail from error event

- GIVEN orchestrator with `learner != nil`
- WHEN execution path emits events `[text, error]` with `error.Content = "OOM at line 42"` and closes channel
- THEN `processAutoClose` synthesizes `workmodel.Verdict{Kind: VerdictFail, Reason: "OOM at line 42"}`
- AND calls `o.learner.Learn(ctx, LearnRequest{Verdict: VerdictFail, SessionID})` asynchronously

#### Scenario: processAutoClose synthesizes VerdictIndeterminate from tombstone event

- GIVEN orchestrator with `learner != nil`
- WHEN execution path emits a `tombstone` event and closes channel
- THEN `processAutoClose` synthesizes `workmodel.Verdict{Kind: VerdictIndeterminate, IndeterminateReason: "interrupt"}`
- AND calls `o.learner.Learn(ctx, LearnRequest{Verdict: VerdictIndeterminate, SessionID})` asynchronously

#### Scenario: processAutoClose skips non-terminal event types

- GIVEN orchestrator with `learner != nil`
- WHEN execution path closes channel without emitting `complete` / `error` / `tombstone` (e.g. last event is `text` / `thinking` / `tool_call` / `tool_result` / `status` / `permission`)
- THEN `synthesizeVerdict` returns nil
- AND `processAutoClose` does NOT call `o.learner.Learn`

#### Scenario: processAutoClose nil learner is a no-op passthrough

- GIVEN orchestrator with `learner == nil` (no `WithLearner` option)
- WHEN `ProcessMessage` is called
- THEN `processAutoClose` is NOT called
- AND the path-returned channel is wrapped by `endSpanWhenChannelClosed` directly (no Learn attempt)

#### Scenario: processAutoClose IntentSkip path does not trigger Learn

- GIVEN intent classification returns `IntentSkip` (e.g. empty message)
- WHEN `ProcessMessage` routes to the skip branch (orchestrator.go:373-376)
- THEN `processAutoClose` is NOT invoked
- AND an empty closed channel is returned synchronously
- AND no `o.learner.Learn` call is made (skip path has no execution results to learn from)

#### Scenario: processAutoClose Learn error is logged and does not block caller

- GIVEN orchestrator with `learner != nil` and `learner.Learn` returns error
- WHEN execution path closes channel
- THEN `processAutoClose` logs `slog.Warn` with `(session_id, verdict_kind, err)`
- AND channel proxy returned to caller is unaffected (caller observes original events)
- AND caller (D1 gateway) does not see the Learn error

#### Scenario: processAutoClose context cancellation skips Learn

- GIVEN orchestrator with `learner != nil`
- WHEN execution path's context is cancelled mid-flight
- THEN channel closes prematurely
- AND `processAutoClose` detects the empty/partial channel and logs `slog.Warn`
- AND `o.learner.Learn` is NOT called (no terminal Verdict available)

#### Scenario: AssetBuilder Auto-Close fallback keeps LP-1 alive when Plan/Artifact nil

- GIVEN orchestrator with `learner != nil` calling `processAutoClose` on a complete event
- WHEN `LearnRequest.Plan == nil && Artifact == nil` (Phase 7 v1.0 不反向追溯)
- THEN `AssetBuilder.extractSOPName` falls back to `sop:autoclose:<SourceID>`
- AND `AssetBuilder.extractStepsFromPlan` returns `["autoclose-completion"]`
- AND `AssetBuilder.Build` returns a non-nil `LearningAsset` (no `ErrAssetBuildFailed`)
- AND LP-1 闭环在生产 wiring 真实可走通 (5 节点管道 Learn → BayesianUpdate → ReputationStore → 下一轮 Inject)

---

### Requirement: D7-S13-A48 ProcessRequest.TrackMode — Operator 角色支持 + buildObserveRequest 透传 ✅ IMPLEMENTED

MUPS v4.3 Phase 7 TrackMode 字段 + 3-tier 解析（PR-7.2）。闭环 Phase 5 PR-E5 E5.3 (D7-S11-A38-T07) 已定义的 `DefaultOperatorPrior Beta(8,1)`：把 Operator 角色能力从 Learn 内部暴露给 caller，让 D1 gateway 飞书适配器层未来版本可按用户角色注入 TrackMode。

**Priority:** P0
**Package:** `internal/layers/orchestration/orchtypes/{process,errors}.go` + `internal/layers/orchestration/{learn,sessionorchestrator}/`
**T:** D7-S13-A48-T04 … D7-S13-A48-T05
**Design:** `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase7-verify-auto-close/`

#### Scenario: ProcessRequest.TrackMode zero value defaults to developer

- GIVEN a `ProcessRequest{}` is constructed (zero value)
- THEN `TrackMode` is `""`
- AND the orchestrator treats empty string as `learn.TrackModeDeveloper`
- AND `buildObserveRequest` uses `DefaultDeveloperPrior` (Beta(5,3), Mean=0.625)

#### Scenario: ProcessRequest.TrackMode="operator" uses DefaultOperatorPrior

- GIVEN a `ProcessRequest{TrackMode: "operator"}` is passed to `ProcessMessage`
- WHEN `buildObserveRequest` calls `o.learner.Inject(ctx, req.SessionID, "operator")`
- THEN the prior's `PriorBeta` is `learn.DefaultOperatorPrior` (Beta(8,1), Mean=0.889)
- AND the classification confidence is adjusted by `Mean = 0.889`

#### Scenario: ProcessRequest.TrackMode="developer" uses DefaultDeveloperPrior

- GIVEN a `ProcessRequest{TrackMode: "developer"}` is passed to `ProcessMessage`
- WHEN `buildObserveRequest` calls `o.learner.Inject(ctx, req.SessionID, "developer")`
- THEN the prior's `PriorBeta` is `learn.DefaultDeveloperPrior` (Beta(5,3), Mean=0.625)

#### Scenario: ProcessRequest.TrackMode invalid value falls back to developer

- GIVEN a `ProcessRequest{TrackMode: "garbage"}` is passed to `ProcessMessage` (struct literal bypasses NewProcessRequest)
- WHEN `DefaultLearner.Inject` is called
- THEN the orchestrator forwards the invalid value to `Inject` where it's logged + falls back to `learn.TrackModeDeveloper` (Beta(5,3))

#### Scenario: NewProcessRequest fail-fast validation

- GIVEN `NewProcessRequest("", "hi", "")` is called
- THEN it returns `ErrProcessRequestSessionIDEmpty`
- AND `NewProcessRequest("sess_1", "", "")` returns `ErrProcessRequestMessageEmpty`
- AND `NewProcessRequest("sess_1", "hi", "garbage")` returns `ErrProcessRequestInvalidTrackMode`
- AND `NewProcessRequest("sess_1", "hi", "")` / `TrackModeDeveloper` / `TrackModeOperator` all return `ProcessRequest{...}` with no error

#### Scenario: Reputation row track-mode wins over hint (3-tier policy)

- GIVEN a Reputation row exists with `TrackMode="operator"` (persisted state)
- AND `ProcessRequest.TrackMode="developer"` (per-request hint)
- WHEN `DefaultLearner.Inject` resolves the track mode
- THEN the operator track is used (rep wins over hint)
- AND the prior's `PriorBeta` is `learn.DefaultOperatorPrior` (Beta(8,1))

#### Scenario: ProcessMessageContract sets TrackMode="" for backward compatibility

- GIVEN D1 gateway calls `ProcessMessageContract(ctx, sessionID, message)` (string-args variant)
- WHEN the contract method constructs a `ProcessRequest`
- THEN `TrackMode` is `""` (zero value)
- AND the prior defaults to `learn.TrackModeDeveloper`
- AND existing v1.0 D1 gateway callers see no behavior change

---

### Requirement: D7-S13-A49 sessionSpan 6 prior attributes — D5 可观测化增强 ✅ IMPLEMENTED

MUPS v4.3 Phase 7 D5 trace 增强（PR-7.3）。扩展 sessionSpan trace 信息到 6 个 attribute，让 Jaeger UI 排查时一眼看清 prior 是真实注入还是冷启动兜底、TrackMode 是 developer 还是 operator、classifier 用的是 rule 还是 shadow。priorSessionSpanAttrs 是纯函数，便于单元测试（无 tracer / 无 sessionSpan mock 需求）。

**Priority:** P0
**Package:** `internal/layers/orchestration/sessionorchestrator/{tracing,orchestrator}.go`
**T:** D7-S13-A49-T06
**Design:** `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase7-verify-auto-close/`

#### Scenario: sessionSpan records all 6 prior attributes when prior is injected

- GIVEN orchestrator with `learner != nil` and a `ReputationStore` row with `Alpha=8, Beta=1, TrackMode=operator`
- WHEN `ProcessMessage` is called
- THEN `sessionSpan` contains the following 6 attributes:
  - `learn.prior.alpha` = `"8"` (string)
  - `learn.prior.beta` = `"1"` (string)
  - `learn.prior.mean` = `"0.889"` (string, 8/(8+1) formatted to 3 decimal places)
  - `learn.prior.track_mode` = `"operator"`
  - `learn.prior.injected_at` = `"phase6_lp1"` (real injection, not failsafe)
  - `learn.classifier_source` = `"rule"` (or `"shadow"` if `WithShadowClassifier` is wired)

#### Scenario: sessionSpan marks cold_start_failsafe when learner is nil

- GIVEN orchestrator with `learner == nil` (no `WithLearner` option)
- WHEN `ProcessMessage` is called
- THEN `sessionSpan` contains:
  - `learn.prior.alpha` = `"5"`
  - `learn.prior.beta` = `"3"`
  - `learn.prior.mean` = `"0.625"` (5/(5+3) formatted to 3 decimal places)
  - `learn.prior.track_mode` = `"developer"`
  - `learn.prior.injected_at` = `"cold_start_failsafe"` (failsafe, not injection)
  - `learn.classifier_source` = `"rule"` (or `"shadow"`)

#### Scenario: sessionSpan classifier_source reflects shadow wiring

- GIVEN orchestrator with `WithShadowClassifier` wired
- WHEN `ProcessMessage` is called
- THEN `sessionSpan` contains `learn.classifier_source = "shadow"`
- AND all other 5 prior attributes are still recorded

#### Scenario: sessionSpan TrackMode resolves via 3-tier policy

- GIVEN Reputation row absent AND `ProcessRequest.TrackMode="operator"` (hint)
- WHEN `priorSessionSpanAttrs` is called
- THEN `learn.prior.track_mode = "operator"` (hint used when rep absent)
- AND `learn.prior.alpha = "8"` / `learn.prior.beta = "1"` (DefaultOperatorPrior)

- AND GIVEN Reputation row has `TrackMode="operator"` AND hint is `"developer"`
- THEN `learn.prior.track_mode = "operator"` (rep wins over hint)

---

### Requirement: D7-S20-A01 TaskSpec 下行契约（L1 接口层）

`NewTaskSpec` MUST construct a `TaskSpec` value with non-empty SessionID + `Channel.Kind` ∈ {sync, async, probe, explore} + `TraceID` matching the `ts_<8 hex>` format, and reject construction with `ErrORCHTaskSpecEmpty` (Code 7100), `ErrORCHTaskSpecChannelUnknown` (Code 7101), or `ErrORCHTaskContractTraceInvalid` (Code 7104) otherwise. `Validate` MUST be idempotent and pure. The three construction callers (`SynthesizeTaskGraph` Plan node, `Channel.Execute` execute entrypoint, `WorkItemExecutor.ExecuteWorkItem`) MUST converge to the same `NewTaskSpec` factory path. `ChannelRequest.Spec` is an additive optional pointer field on `mups/execute/channel.go` to support legacy callers unchanged.

**Priority:** P0  
**Package:** `internal/layers/orchestration/interfaces/`  
**Additive Field:** `mups/execute/channel.go::ChannelRequest.Spec *interfaces.TaskSpec`  
**SentinelError Codes:** 7100 / 7101 / 7104  
**T:** D7-S20-A01-T01 / T02 / T03

#### Scenario: Happy-path NewTaskSpec

- GIVEN sessionID="sess_1782", a Plan, a Channel{Kind="sync"}, a WorkItem, trace_id="ts_abc12345"
- WHEN `NewTaskSpec` is invoked
- THEN the returned `TaskSpec` contains all 4 inputs non-zero
- AND `Validate()` returns nil
- AND `Spec.SpecHash` is derived from the 4 inputs (fnv64a stable)

#### Scenario: Empty session_id rejected

- GIVEN sessionID=""
- WHEN `NewTaskSpec` is invoked
- THEN it returns an error wrapping `ErrORCHTaskSpecEmpty` (Code 7100)
- AND `errors.Is(err, sharederrors.ErrORCHTaskSpecEmpty)` is true

#### Scenario: Channel.Kind validation

- GIVEN a Channel with Kind="unknown" (not in {sync, async, probe, explore})
- WHEN `NewTaskSpec` is invoked
- THEN it returns an error wrapping `ErrORCHTaskSpecChannelUnknown` (Code 7101)

- GIVEN a Channel with Kind="sync"
- WHEN `NewTaskSpec` is invoked
- THEN no error is returned (regression baseline)

#### Scenario: TraceID format enforcement

- GIVEN trace_id=""
- WHEN `NewTaskSpec` is invoked
- THEN it returns `ErrORCHTaskContractTraceInvalid` (Code 7104)

- GIVEN trace_id="ts_12345678" (8 hex chars after `ts_`)
- WHEN `NewTaskSpec` is invoked
- THEN no error is returned
- AND a valid TaskSpec with `TraceID="ts_12345678"` is produced

#### Scenario: With* immutable builders

- GIVEN a TaskSpec A
- WHEN `A.WithPlan(B.Plan)` / `WithChannel(B.Channel)` / `WithWorkItem(B.WorkItem)` is invoked
- THEN each call returns a new TaskSpec with the corresponding field replaced
- AND the original A's fields are unchanged (immutable copy)
- AND the new copy is a shallow copy (`c := *s` pattern)

#### Scenario: Three construction points converge

- GIVEN the Plan node (SynthesizeTaskGraph), the Channel.Execute entrypoint, and the WorkItem node (ExecuteWorkItem)
- WHEN each constructs a TaskSpec
- THEN all three paths delegate to `interfaces.NewTaskSpec` (single factory)
- AND the returned structs are byte-identical for identical inputs

---

### Requirement: D7-S20-A02 TaskReport 上行契约 + L2 字段语义层

`NewTaskReport` MUST construct a `TaskReport` with non-empty SessionID + `Verdict.Kind` in the 4-state union (ComplianceVerdict / TimelinessVerdict / RootCauseVerdict / StatisticalVerdict) + valid TraceID, rejecting otherwise with `ErrORCHTaskReportEmpty` (Code 7102), `ErrORCHTaskReportVerdictEmpty` (Code 7103), or `ErrORCHTaskContractTraceInvalid` (Code 7104). `AppendDissent` MUST silently truncate to top-N (default 3) without log spam, and compute `SummaryHash` lazily via fnv64a→8-hex prefix. `WithVerdict` / `WithResource` / `WithBlockage` MUST be immutable builders returning shallow copies. `LearnRequest.Report` is an additive optional pointer field on `mups/learn/asset/asset_builder.go`.

**Priority:** P0  
**Package:** `internal/layers/orchestration/interfaces/`  
**Additive Field:** `mups/learn/asset/asset_builder.go::LearnRequest.Report *interfaces.TaskReport`  
**SentinelError Codes:** 7102 / 7103 / 7104  
**T:** D7-S20-A02-T01 / T02 / T03 + D7-S21-A01-T01 / A02-T01 / A03-T01

#### Scenario: WithVerdict immutable

- GIVEN a TaskReport R
- WHEN `R.WithVerdict(v)` is invoked
- THEN a new TaskReport R' is returned with `Verdict=v`
- AND R's Verdict is unchanged
- AND R' equals R except for the Verdict field

#### Scenario: AppendDissent top-3 silent truncate

- GIVEN a TaskReport with 3 Dissents already populated
- WHEN `TaskReport.AppendDissent(d4)` is invoked
- THEN the returned TaskReport still has 3 Dissents (top-3 cap)
- AND d4 is silently dropped (no log, no error returned)
- AND SummaryHash on the truncated list remains stable

- GIVEN a TaskReport with 0 Dissents
- WHEN `TaskReport.AppendDissent(d1).AppendDissent(d2).AppendDissent(d3).AppendDissent(d4)` is invoked
- THEN the final TaskReport has exactly 3 Dissents (d1, d2, d3)
- AND no error, no log message

#### Scenario: Blockage 3 kind classification

- GIVEN an error of `*PermissionError` (403, IAM deny)
- WHEN `WithBlockage(err, planCtx)` is invoked
- THEN the returned TaskReport has `Blockage.Kind = permission`, `Retryable = false`

- GIVEN an error matching `*errors.OOM` or `*disk.FullError` or `*quota.ExceededError`
- WHEN `WithBlockage(err, planCtx)` is invoked
- THEN `Blockage.Kind = resource`, `Retryable = true`

- GIVEN any other error
- WHEN `WithBlockage(err, planCtx)` is invoked
- THEN `Blockage.Kind = contract`, `Retryable = true`

#### Scenario: Resource three-piece extraction

- GIVEN an ExecutionResult with `TokenUsed=1523`, `ElapsedMs=4217`, `ReActIterCount=4`
- WHEN `WithResource(result)` is invoked
- THEN the returned TaskReport has `Resource.TokenUsed = 1523`, `Resource.ElapsedMs = 4217`, `Resource.StepCount = 4`

- GIVEN `ExecutionResult` lacking metadata (nil TokenAccounting, no StartTime)
- WHEN `WithResource(result)` is invoked
- THEN Resource.TokenUsed = 0, Resource.ElapsedMs = 0, Resource.StepCount = 0 (zero-default)

#### Scenario: Learn precipitation to existing feedback channel

- GIVEN a TaskReport with 2 Dissents
- WHEN `LearnRequest{Report: &report, AssetHint: ...}` flows through existing `mups/learn/asset/asset_builder.go`
- THEN the Dissents are emitted to the existing `feedback` channel
- AND legacy callers of `LearnRequest` (without `Report`) behave unchanged
- AND no new memory channel is added (additive embedding, not a new route)

#### Scenario: Round-trip TaskSpec→Channel.Execute→TaskReport invariant

- GIVEN a TaskSpec built by `NewTaskSpec` at Plan node
- WHEN `Channel.Execute(req{Spec: spec})` runs and produces a TaskReport via `NewTaskReport`
- THEN the TaskReport.SessionID equals the TaskSpec.SessionID
- AND both have the same TraceID prefix `ts_` + 8 hex
- AND the TaskReport.Verdict is one of the 4 VerdictKind values
- AND `CrossSpecReport(spec, report)` returns `nil`

---

### Requirement: D7-S20-A03 TaskContract 治理横切（spec 同步 + 工具链）

This Change synchronizes the new L1/L2 contract surfaces across **6 spec files** (`openspec/specs/d7-orchestration/spec.md` ADDED Requirements, `d7-domain.md` §DSAFT 资产 + new §8 Layer 架构 + new §9 interfaces 包, `a-registry.md` 6 new A entries, `f-registry.md` 11 new F + 2 PLANNED, `t-registry.md` 11 P0 T points, `span-registry.md` 5 new Span operations). The `interfaces` package MUST remain **pure types** (no import of any `internal/layers/orchestration/{s1-s6,hardening,escape}` subpackage), with the sole exception of `internal/shared/errors/` for SentinelError wrapping. This invariant MUST be enforced by the CI lint invariant `TestInterfacesZeroImportD7`.

**Priority:** P0  
**Package:** `internal/layers/orchestration/interfaces/` + `scripts/ci-lint-invariant/`  
**SoT Files:** 6 spec files (a/f/t/span/d7-domain/spec) at `openspec/specs/d7-orchestration/`  
**Invariant:** `IV-1 interfaces pure types` (D7 子包 0 import)  
**T:** D7-S20-A03-T01 / T02

#### Scenario: Six-file spec consistency

- GIVEN the merge of DM-20260629-007
- WHEN cross-checking the 6 spec files
- THEN `spec.md` v4.16.0 has 3 new ADDED Requirements (D7-S20-A01/A02/A03) with 12 Scenarios
- AND `d7-domain.md` v2.7.0 has §DSAFT 资产 tables updated (A 49→55, F 75→86, T 230→241, Span 26→31)
- AND `a-registry.md` v5.1.0 has 6 new A activities (D7-S20/S21-A01..A03)
- AND `f-registry.md` v5.1.0 has 11 new F functions + 2 PLANNED (D7-S22 PR-B/C)
- AND `t-registry.md` v4.14.0 has 11 new P0 T points (D7-S20/S21-A*-T*)
- AND `span-registry.md` v4.3.0 has 5 new TaskContract span ops

#### Scenario: interfaces pure types invariant

- GIVEN `internal/layers/orchestration/interfaces/` package's full import set
- WHEN `scripts/ci-lint-invariant` runs `TestInterfacesZeroImportD7`
- THEN no import path matches `internal/layers/orchestration/` excluding the package itself
- AND `internal/shared/errors/` import is the sole allowed external dep
- AND the test PASSES; otherwise exits non-zero

#### Scenario: Additive embedding backward compat regression

- GIVEN legacy callers (DM-20260625-018 / DM-20260625-019 / earlier) of `Channel.Execute(req)` / `AssetBuilder.Build(req)` without setting `Spec` / `Report`
- WHEN the additive field is added
- THEN all legacy callers compile unchanged (zero source diff in caller sites)
- AND `req.Spec == nil` and `req.Report == nil` produce the same observable behavior as before PR-A
- AND 24/24 orchestration packages `go test -race` PASS

---

### Requirement: D7-S18-A11 Pessimistic Commit（L3 防御运行时层）

`PessimisticCommitGuard` is a Go interface in `internal/layers/orchestration/interfaces/contracts.go` with three methods: `Evaluate(spec *TaskSpec, report *TaskReport, budget ConvergenceBudget) (ok bool, blockedReason string, err error)`, `ResolveFallback(spec *TaskSpec, report *TaskReport) (policy FallbackPolicy, ruleName string)`, `BuildMVPArtifact(spec *TaskSpec, report *TaskReport, reason string) (mvp MVPArtifact, err error)`. The default implementation `DefaultPessimisticCommitGuard` in `internal/layers/orchestration/escape/fallback.go` MUST trigger (return `ok=false`) on any of 5 conditions: **T1 resource_exhausted** (Resource.TokenUsed > Budget.MaxTokens OR Resource.ElapsedMs > Budget.MaxElapsedMs), **T2 cb_l1** (CircuitBreakerState L1 Open), **T3 indeterminate_3x** (3 consecutive VerdictIndeterminate in report history), **T4 empty_evidence** (Verdict=VerdictPass but Report.Evidence is empty), **T5 manual_abort** (spec.Blockage.Kind == BlockageContract and retryable=false). The default guard MUST be **nil-safe** (guard==nil returns `(true, "", nil)`) and **disabled-safe** (`!Enabled` returns `(true, "", nil)`). `BuildMVPArtifact` MUST produce a non-empty `MVPArtifact.Output` or return `ErrORCHPessimisticMVPEmpty` (Code 7111). When triggered, `Engine.NotifyPessimistic` MUST inject the MVPArtifact into the TaskReport and force `Result.Kind = ResultPessimistic` (5-layer fail-safe).

**Priority:** P0  
**Package:** `internal/layers/orchestration/interfaces/contracts.go` + `internal/layers/orchestration/escape/fallback.go` + `internal/layers/orchestration/escape/engine.go`  
**Additive Field:** `escape.Engine.pessimisticGuard interfaces.PessimisticCommitGuard` + `mups/execute.ChannelRouter.pessimisticGuard`  
**SentinelError Codes:** 7110 / 7111  
**Span:** `D7_Orchestration_Pessimistic_Commit_Emit` (7 attrs: session_id + trace_id + reason + policy + fallback_used + mvp.artifact_hash + mvp.trigger)  
**T:** D7-S18-A11-T01 / T02 / T03 / T04 / T05

#### Scenario: Disabled guard passes through

- GIVEN `guard = NewDefaultPessimisticCommitGuard()` (Enabled=false by default)
- AND a valid `TaskSpec` and `TaskReport`
- WHEN `guard.Evaluate(spec, report, budget)` is invoked
- THEN it returns `(true, "", nil)` (no blocking, fall-open)
- AND no MVPArtifact is built
- AND no fallback path is taken

#### Scenario: Resource exhausted trigger (T1)

- GIVEN a `TaskReport` with `Resource.TokenUsed = 200000` and `Budget.MaxTokens = 100000`
- WHEN `guard.Evaluate(spec, report, budget)` is invoked
- THEN it returns `(false, "resource_exhausted", nil)`
- AND `ResolveFallback` returns `(FallbackPolicyPessimistic, "")` (no rule, MVP-only)

#### Scenario: Circuit Breaker L1 trigger (T2)

- GIVEN `CircuitBreakerState[1] = Open` (DispatchLoop tripped)
- WHEN `guard.Evaluate(spec, report, budget)` is invoked
- THEN it returns `(false, "cb_l1", nil)`
- AND the blockedReason string contains `"l1"` substring (for routing hint)
- AND 60-second persistent window is respected (per circuit_breaker_test.go::TestL1StateOpen_PersistentForPessimisticWindow)

#### Scenario: 3x Indeterminate Verdict trigger (T3)

- GIVEN a TaskReport with 3 consecutive VerdictIndeterminate in Dissents/Verdict history
- WHEN `guard.Evaluate(spec, report, budget)` is invoked
- THEN it returns `(false, "indeterminate_3x", nil)`
- AND `ResolveFallback` returns `(FallbackPolicyRuleBased, "min_uncertainty")` (default rule)

#### Scenario: Empty evidence trigger (T4)

- GIVEN a TaskReport with `Verdict.Kind = VerdictPass` but `Evidence` is empty/nil
- WHEN `guard.Evaluate(spec, report, budget)` is invoked
- THEN it returns `(false, "empty_evidence", nil)`

#### Scenario: Manual abort trigger (T5)

- GIVEN a TaskSpec with `Blockage.Kind = BlockageContract` and `Blockage.Retryable = false`
- WHEN `guard.Evaluate(spec, report, budget)` is invoked
- THEN it returns `(false, "manual_abort", nil)`

#### Scenario: Happy path (all checks pass)

- GIVEN a fully valid TaskSpec + TaskReport (Resource under budget + CB healthy + no indeterminate streak + non-empty evidence + no manual abort)
- WHEN `guard.Evaluate(spec, report, budget)` is invoked
- THEN it returns `(true, "", nil)`
- AND no fallback path is taken
- AND TaskReport.FallbackUsed remains false
- AND no MVPArtifact is injected

#### Scenario: BuildMVPArtifact produces MVPArtifact (5 trigger → MVP)

- GIVEN `guard.Enabled = true`
- AND a TaskSpec + TaskReport that triggered Pessimistic Commit
- WHEN `guard.BuildMVPArtifact(spec, report, reason)` is invoked
- THEN it returns a `MVPArtifact` with `Output` non-empty
- AND `RiskWarnings` populated from report.Dissents[:min(3,len)]
- AND `Trigger = reason`
- AND `ChainHash` = FNV-1a 16-char hex of `Output + RiskWarnings + reason + time.Now().UnixNano()`
- AND the same inputs produce a stable ChainHash prefix (deterministic within a session)

#### Scenario: BuildMVPArtifact empty output rejection

- GIVEN a BuildMVPArtifact producer that returns `MVPArtifact{Output: ""}`
- WHEN the guard tries to inject it into a TaskReport
- THEN it returns `ErrORCHPessimisticMVPEmpty` (Code 7111)
- AND `errors.Is(err, sharederrors.ErrORCHPessimisticMVPEmpty)` is true
- AND the TaskReport is not mutated

#### Scenario: 5-layer CB L1 Pessimistic routing

- GIVEN the 5-layer CircuitBreakerSet with `L1DispatchLoop` config `MaxWakeupsPerMin=100`
- WHEN 100 wakeups are emitted within 60 seconds
- THEN `L1DispatchLoop` transitions to StateOpen
- AND a subsequent `guard.Evaluate` returns `(false, "cb_l1_<reason>", nil)` where `<reason>` describes the L1 sub-cause
- AND `Engine.NotifyPessimistic` injects MVPArtifact and forces `Result.Kind = ResultPessimistic`

#### Scenario: Engine.NotifyPessimistic 5-layer fail-safe

- GIVEN `Engine.pessimisticGuard = nil` (not set, legacy callers)
- WHEN `Engine.NotifyPessimistic(spec, report)` is invoked
- THEN it returns nil (no-op, fail-safe, no panic)
- AND `Result.Kind` is unchanged

- GIVEN `Engine.pessimisticGuard = guard` and `guard.Enabled = false`
- WHEN `Engine.NotifyPessimistic(spec, report)` is invoked
- THEN it returns nil and does NOT inject MVPArtifact

- GIVEN `Engine.pessimisticGuard = guard` and `guard.Evaluate` returns error
- WHEN `Engine.NotifyPessimistic(spec, report)` is invoked
- THEN it logs the error via slog.Warn and returns nil (fail-open, don't propagate)

- GIVEN `Engine.pessimisticGuard = guard` and `guard.Evaluate` returns `(false, reason, nil)`
- WHEN `Engine.NotifyPessimistic(spec, report)` is invoked
- THEN it calls `guard.BuildMVPArtifact(spec, report, reason)` and injects the result into report.MVPArtifact
- AND `report.FallbackUsed = true`
- AND `Result.Kind = ResultPessimistic` (forced)

- GIVEN `Engine.pessimisticGuard = guard` and `report == nil`
- WHEN `Engine.NotifyPessimistic(spec, nil)` is invoked
- THEN it returns nil without panic (nil guard for nil report)

#### Scenario: Feature Flag env-gated default disabled

- GIVEN env `D7_PESSIMISTIC_COMMIT_ENABLED` is unset (default)
- WHEN `bootstrap.PessimisticCommitEnabled()` is invoked
- THEN it returns false
- AND `NewPessimisticCommitGuardFromEnv()` returns a guard with `Enabled = false`
- AND no observable behavior change vs pre-PR-B baseline

- GIVEN env `D7_PESSIMISTIC_COMMIT_ENABLED=true`
- WHEN `bootstrap.PessimisticCommitEnabled()` is invoked
- THEN it returns true
- AND `NewPessimisticCommitGuardFromEnv()` returns a guard with `Enabled = true`

#### Scenario: Pessimistic_Commit_Emit span + metric emit (PLANNED, T05)

- GIVEN `Engine.NotifyPessimistic(spec, report)` triggers and returns `(false, "resource_exhausted", nil)`
- WHEN the span/metric emit hook fires
- THEN a `D7_Orchestration_Pessimistic_Commit_Emit` span is emitted with 7 attributes (session_id + trace_id + reason="resource_exhausted" + policy + fallback_used=true + mvp.artifact_hash + mvp.trigger)
- AND `pessimistic_commit_trigger_count` metric is incremented by 1
- AND `mvp_artifact_generated_total` metric is incremented by 1
- AND `pessimistic_commit_latency_us` metric observes the Evaluate duration

> **NOTE:** T05 PLANNED 留 PR-C. PR-B 阶段仅在 `engine.go::NotifyPessimistic` 内通过 `slog.Info("pessimistic_commit_emit", ...)` 占位（结构化字段已对齐），完整 Jaeger (OTel span 注册) + Prometheus wire 待 PR-C 落地.

---

### Requirement: D7-S18-A12 Rule-based Fallback（4 候选规则）

`FallbackRuleBased.Select(report *TaskReport) (ruleName string, ok bool)` MUST choose one of 4 closed-set rules: `most_tests_passed` (prefer Report with highest `Resource.StepCount` from passing tests), `compiled_clean` (prefer Report with `Blockage.Kind == BlockageResource` retryable=true + clean compile), `min_cost` (prefer Report with lowest `Resource.TokenUsed + ElapsedMs * weight`), or `min_uncertainty` (default; prefer Report with lowest `Verdict.Kind = VerdictIndeterminate` count). `DefaultFallbackRule = "min_uncertainty"`. `ParseFallbackRuleName(name string) (FallbackPolicy, string)` MUST return `(FallbackPolicyRuleBased, name)` for valid 4-rule names, and `(FallbackPolicyPessimistic, DefaultFallbackRule)` for invalid/legacy names (silent fall-back, not an error). Invalid rule name in env `D7_RULE_FALLBACK_STRATEGY` MUST NOT fail bootstrap, just log warning and use default.

**Priority:** P0  
**Package:** `internal/layers/orchestration/interfaces/fallback_policy.go` + `internal/layers/orchestration/escape/fallback.go`  
**SentinelError Codes:** 7112 / 7113  
**Span:** `D7_Orchestration_Pessimistic_Commit_Emit.fallback_used = true` (reuse A11 span)  
**Metric:** `fallback_rule_select_total{rule=most_tests_passed|compiled_clean|min_cost|min_uncertainty}` + `fallback_rule_apply_total`  
**T:** D7-S18-A12-T01 / T02

#### Scenario: 4 candidate rules enumeration

- GIVEN `FallbackPolicyRuleNames() = []string{"most_tests_passed", "compiled_clean", "min_cost", "min_uncertainty"}`
- WHEN each name is passed to `ParseFallbackRuleName`
- THEN all 4 return `(FallbackPolicyRuleBased, name)` (no error, no fallback)

#### Scenario: Default rule = min_uncertainty

- GIVEN env `D7_RULE_FALLBACK_STRATEGY` unset
- WHEN `bootstrap.PessimisticRuleStrategy()` is invoked
- THEN it returns `"min_uncertainty"`
- AND `ParseFallbackRuleName("min_uncertainty")` returns `(FallbackPolicyRuleBased, "min_uncertainty")`

#### Scenario: Invalid rule name falls back silently

- GIVEN env `D7_RULE_FALLBACK_STRATEGY="legacy_fallback_v1"` (not in 4-rule set)
- WHEN `bootstrap.PessimisticRuleStrategy()` is invoked
- THEN it returns `DefaultFallbackRule` = `"min_uncertainty"` (silent fall-back)
- AND `slog.Warn("unknown rule strategy, using default", "requested", "legacy_fallback_v1", "default", "min_uncertainty")` is logged
- AND no `ErrORCHFallbackRuleInvalid` is returned (only triggered if caller explicitly rejects)

- GIVEN `ParseFallbackRuleName("legacy_fallback_v1")` is invoked directly (not via env)
- THEN it returns `(FallbackPolicyPessimistic, DefaultFallbackRule)` (silent fallback to MVP-only)

#### Scenario: ResolveFallback returns (Pessimistic, "") on T1/T4/T5 trigger

- GIVEN a guard.Evaluate returning `(false, "resource_exhausted", nil)` (T1)
- WHEN `guard.ResolveFallback(spec, report)` is invoked
- THEN it returns `(FallbackPolicyPessimistic, "")` (MVP-only, no rule selection)

#### Scenario: ResolveFallback returns (RuleBased, "min_uncertainty") on T3 trigger

- GIVEN a guard.Evaluate returning `(false, "indeterminate_3x", nil)` (T3)
- WHEN `guard.ResolveFallback(spec, report)` is invoked
- THEN it returns `(FallbackPolicyRuleBased, "min_uncertainty")` (default rule selection)

#### Scenario: FallbackPolicyPolicy override via env

- GIVEN env `D7_RULE_FALLBACK_STRATEGY="min_cost"`
- WHEN `NewPessimisticCommitGuardFromEnv()` is invoked
- THEN it returns a guard with `RuleName = "min_cost"`
- AND a T3 trigger calls ResolveFallback returning `(FallbackPolicyRuleBased, "min_cost")` (overrides default)

#### Scenario: FallbackRuleBased.Select most_tests_passed

- GIVEN 3 candidate TaskReports with `Resource.StepCount = [10, 7, 3]`
- WHEN `FallbackRuleBased.Select(reports)` is invoked with rule="most_tests_passed"
- THEN it returns the report with StepCount=10

#### Scenario: FallbackRuleBased.Select min_cost

- GIVEN 3 candidate TaskReports with `Resource.TokenUsed = [1000, 500, 2000]`
- WHEN `FallbackRuleBased.Select(reports)` is invoked with rule="min_cost"
- THEN it returns the report with TokenUsed=500

#### Scenario: FallbackPolicyRuleNames closed set

- GIVEN `FallbackPolicyRuleNames()` is the closed set
- WHEN `len(FallbackPolicyRuleNames())` is checked
- THEN it equals 4 (no more, no less)
- AND the set is: `{"most_tests_passed", "compiled_clean", "min_cost", "min_uncertainty"}`

---

## PLANNED Requirements (D7 v1.0 迁移)

**Status: v1.0 + v1.1 全闭环（2026-06-15）。** 以下条目仅作历史追溯，新功能请遵循 v2.0+ 路线（DM-018 Hub-Spoke + DM-020 Turn Leader 已 wired）。

| Requirement | 目标 | v1.0 / v1.1 状态 |
|-------------|------|------------------|
| D7-S2 ProcessMessage | D1→D7 入口上移 | ✅ IMPLEMENTED |
| D7-S5-P2 ClassifyIntent | 规则 + command-first + LLM fallback | ✅ IMPLEMENTED |
| D7-S5-P3 SynthesizeTaskGraph | 目标拆解为 DAG（规则 + LLM） | ✅ IMPLEMENTED |
| D7-S5 SelectExecutor | D2/D4 执行器选择 | ✅ IMPLEMENTED |
| D2 Thin / QueryLoop removed | loop 已删；D2 无 D4 import | ✅ DM-20260618-010 |
| D7 package identity | `sessionorchestrator/` + `decisionplanning/` + `orchtypes/`；`coordinator/` shim（DM-20260619-005） | ✅ IMPLEMENTED |
| D7 Migration Coexistence | 4 组合回归 | ✅ IMPLEMENTED |
| D7-S2 Turn Leader (DM-020) | A06 RunTurnLoop + A07 InvokeLLM | ✅ IMPLEMENTED（wired by `wire_coordinator.go`） |
| D7-S2 Hub-Spoke (DM-018) | A04 DispatchWorker + A04/A05 SpokeBridge | ✅ IMPLEMENTED（wired by `delegate.go`） |
| D7-S1 WorkModel 迁入 | 写模型从 D2 迁入 D7 | ✅ IMPLEMENTED |

---

### Requirement: PlanAgent Read-Only Sandbox (devrix-d7-uncertainty-gaps)

`PlanAgent` MUST enforce tool call whitelist at runtime, not only via prompt. `ValidateToolCall()` checks against the read-only whitelist and forbidden list, failing closed on unknown tools.

**Priority:** P0
**Package:** `internal/layers/orchestration/workmodel/plan_agent.go`
**T:** D7-S5-A02-F01-T01 … D7-S5-A02-F01-T04

#### Scenario: Allowed tool passes validation

- GIVEN a PlanAgent with the default read-only whitelist
- WHEN `ValidateToolCall` is called with `"read"`
- THEN no error is returned

#### Scenario: Forbidden tool is rejected

- GIVEN a PlanAgent with the default read-only whitelist
- WHEN `ValidateToolCall` is called with `"write"`
- THEN an error is returned containing `"forbidden in plan mode"`

#### Scenario: Unknown tool is rejected

- GIVEN a PlanAgent with the default read-only whitelist
- WHEN `ValidateToolCall` is called with `"unknown_tool"`
- THEN an error is returned containing `"not in the plan mode read-only whitelist"`

#### Scenario: Nil PlanAgent passes through

- GIVEN a nil PlanAgent
- WHEN `ValidateToolCall` is called with `"write"`
- THEN no error is returned (passthrough: no sandbox without PlanAgent)

---

### Requirement: PlanMode LLM Guard (devrix-d7-uncertainty-gaps)

`PlanMode.Enter()` MUST validate LLM availability via `HasLLM()` before entering active state, returning `ErrLLMNotConfigured` immediately instead of failing later during execution.

**Priority:** P0
**Package:** `internal/layers/orchestration/workmodel/plan_mode.go`
**T:** D7-S5-A02-F02-T01 … D7-S5-A02-F02-T02

#### Scenario: Enter with nil LLM returns error

- GIVEN a PlanMode created with nil LLM
- WHEN `Enter` is called
- THEN `ErrLLMNotConfigured` is returned
- AND the PlanMode state remains Inactive

#### Scenario: Enter with valid LLM succeeds

- GIVEN a PlanMode created with a valid LLMCompleter
- WHEN `Enter` is called
- THEN no error is returned
- AND the PlanMode state is Active

---

### Requirement: ConflictGuard Atomic Allow+Register (devrix-d7-uncertainty-gaps)

`ConflictGuard.AllowAndRegister()` MUST atomically check conflict and register a task, eliminating the TOCTOU window between `Allow()` and `Register()`. Returns `true` if registered, `false` if conflict prevents registration.

**Priority:** P0
**Package:** `internal/layers/orchestration/wavescheduler/conflict.go`
**T:** D7-S3-A01-F03-T01 … D7-S3-A01-F03-T04

#### Scenario: AllowAndRegister succeeds when no conflict

- GIVEN an empty ConflictGuard
- WHEN `AllowAndRegister` is called with a TaskNode in group `"A"`
- THEN the call returns true
- AND the task is registered in the guard

#### Scenario: AllowAndRegister blocks on conflict group

- GIVEN a ConflictGuard with a running task in group `"A"`
- WHEN `AllowAndRegister` is called with another TaskNode in group `"A"`
- THEN the call returns false
- AND the second task is NOT registered

#### Scenario: AllowAndRegister allows different groups

- GIVEN a ConflictGuard with a running task in group `"A"`
- WHEN `AllowAndRegister` is called with a TaskNode in group `"B"`
- THEN the call returns true
- AND both tasks are registered

#### Scenario: AllowAndRegister blocks on file scope intersection

- GIVEN a ConflictGuard with a running write task scoped to `"src/auth/**"`
- WHEN `AllowAndRegister` is called with a write TaskNode scoped to `"src/auth/login.go"`
- THEN the call returns false

---

### Requirement: OrchestratePath FlowEvent Sink (devrix-d7-uncertainty-gaps)

`emit()` MUST push FlowEvent to the EventPublisher sink for IM/WebSocket notifications, while also writing to the caller channel. Both paths respect context cancellation; nil sink is gracefully tolerated.

**Priority:** P0
**Package:** `internal/layers/orchestration/sessionorchestrator/orchestrate_path.go`
**T:** D7-S3-A01-F04-T01 … D7-S3-A01-F04-T02

#### Scenario: emit pushes to sink when available

- GIVEN an OrchestratePath with a non-nil EventPublisher sink
- AND a WorkerEvent with Type `"text"` and Content `"task_1 done"`
- WHEN `emit` is called
- THEN `sink.Publish` is called with the corresponding EngineEvent
- AND the event is also written to the out channel

#### Scenario: emit tolerates nil sink

- GIVEN an OrchestratePath with a nil EventPublisher sink
- WHEN `emit` is called
- THEN no panic occurs
- AND the event is written to the out channel

---

### Requirement: Dead Code Markers (devrix-d7-uncertainty-gaps)

`LLMFallbackClassifier` and `ExecutorSelector` MUST carry `Deprecated:` comments documenting they are deferred to v1.1, so future readers understand they are intentionally dead code rather than bugs.

**Priority:** P1
**Package:** `internal/layers/orchestration/decisionplanning/classifier_fallback.go`, `internal/layers/orchestration/decisionplanning/executor.go`
**T:** D7-S2-A03-F06-T01 … D7-S2-A03-F06-T02

#### Scenario: LLMFallbackClassifier has Deprecated marker

- GIVEN the `classifier_fallback.go` file
- THEN the file contains a `Deprecated:` comment
- AND the existing tests still pass

#### Scenario: ExecutorSelector has Deprecated marker

- GIVEN the `executor.go` file
- THEN the file contains a `Deprecated:` comment
- AND the existing tests still pass

### Requirement: D2 QueryLoop Legacy Path Removal (devrix-d2-queryloop-dismantle)

D2 `query.Loop`, `QueryLLMCaller`, and `d2_query_loop_legacy_invocations_total` MUST NOT exist. All LLM↔Tool loops MUST run through D7 `RunTurn` / `SubTurn`. Supersedes DM-20260617-001 Z0 deprecation.

**Priority**: P0  
**T mapping**: D7-S2-A06-T09, `contextengine/queryloop_removed_test.go`

#### Scenario: Production default uses D7 only

- GIVEN default devrix configuration
- WHEN a session completes a full turn
- THEN D7 RunTurn handles all LLM↔Tool iterations
- AND `grep QueryLLMCaller internal/` returns zero production hits

<!-- T: D7-S2-A06-T09 -->

---

### Requirement: SubTurn 3-Mode Context Isolation (devrix-context-budget-phase-b)

`SubTurnRunner.RunSubTurn` (D7-S2-A06) MUST dispatch sub-agent context
inheritance by `req.Mode` and reject requests that exceed the configured
recursion depth. Three modes align with the clawcode 3-mode pattern:

- `brief` (default) — `PreloadedMessages=nil`; child starts with a fresh
  history. Saves tokens at the cost of parent context visibility.
- `fork` — `PreloadedMessages=BuildForkedMessages(parent, directive)`;
  byte-level stable prefix shared across sibling fork children (prompt
  cache friendly). Includes the synthesized `"Fork started — processing
  in background"` placeholder.
- `full` — `PreloadedMessages=messagesWithoutLastUser(parent)`; legacy
  behavior, opt-in only for callers that need full parent history (D5
  evaluation).

When `req.Mode` is empty, `SubTurnRunner` falls back to
`SubagentConfig.DefaultMode` (or `LegacyMode` when set). When
`req.Depth >= SubagentConfig.MaxDepth`, the runner MUST return
`ErrSubagentDepthExceeded` (code `AGT_DEPTH_5011`) WITHOUT invoking the
LLM. Supersedes Phase A `SubTurnRunner` legacy default.

**Priority**: P0  
**T mapping**: D7-S2-A06-T14 … T17, `internal/layers/orchestration/sessionorchestrator/subturn_test.go`

#### Scenario: brief mode drops parent history

- GIVEN a parent session with N messages and `req.Mode=brief`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN the LLM sees exactly 1 message (the last user / directive)
- AND `PreloadedMessages` is nil

<!-- T: D7-S2-A06-T14 -->

#### Scenario: fork mode produces cache-friendly prefix

- GIVEN a parent session with assistant + tool_result messages
- WHEN `req.Mode=fork` and 10 sibling children are spawned in parallel
- THEN each child sees the same byte-level identical prefix
- AND the prefix contains the literal `"Fork started — processing in background"`

<!-- T: D7-S2-A06-T15 -->

#### Scenario: full mode preserves legacy behavior

- GIVEN `req.Mode=full`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN `PreloadedMessages` equals `messagesWithoutLastUser(req.Messages)`
- AND the result is byte-equal to the pre-Phase-B legacy dispatch

<!-- T: D7-S2-A06-T15 (D2-S15-A08-T07 boundary) -->

#### Scenario: empty Mode falls back to DefaultMode

- GIVEN `req.Mode=""` and `Cfg.DefaultMode="brief"`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN the runner dispatches as if `Mode=brief`

<!-- T: D7-S2-A06-T17 -->

#### Scenario: LegacyMode override wins over DefaultMode

- GIVEN `Cfg.DefaultMode="brief"` and `Cfg.LegacyMode="full"`
- WHEN `req.Mode=""` is sent
- THEN the runner dispatches as `Mode=full` (legacy back-compat)

<!-- T: D7-S2-A06-T17 -->

#### Scenario: invalid Mode is rejected before LLM call

- GIVEN `req.Mode="unknown"`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN the runner returns `ErrSubagentInvalidMode` (code `AGT_DEPTH_5012`)
- AND the LLM is not invoked

#### Scenario: Depth at MaxDepth is rejected

- GIVEN `Cfg.MaxDepth=3` and `req.Depth=3`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN the runner returns `ErrSubagentDepthExceeded` (code `AGT_DEPTH_5011`)
- AND the error message hints to retry with `mode=brief`
- AND the LLM is not invoked

<!-- T: D7-S2-A06-T16 -->

#### Scenario: Depth at MaxDepth-1 is allowed

- GIVEN `Cfg.MaxDepth=3` and `req.Depth=2`
- WHEN `SubTurnRunner.RunSubTurn` is called
- THEN the LLM is invoked normally

<!-- T: D7-S2-A06-T16 -->

---

### Requirement: HandleInterrupt errors.Join Aggregation (devrix-d7-error-aggregation-and-metrics)

`InterruptHandler.Handle` MUST aggregate errors from the 3 cancel steps (Wave → D4 → Process) via `errors.Join` instead of the previous "warn all + return nil" anti-pattern. The "stopped" EngineEvent emission remains best-effort regardless of cancel outcomes.

**Priority:** P0
**Package:** `internal/layers/orchestration/sessionorchestrator/`
**T:** D7-S6-A11-T01, T02, T03

#### Scenario: All 3 cancel steps fail

- GIVEN WaveCanceler / DelegateCanceler / ProcessCanceler all return non-nil errors
- WHEN `InterruptHandler.Handle(ctx, sessionID)` is called
- THEN the returned error is `errors.Join(waveErr, d4Err, procErr)`
- AND `errors.Is(returned, waveErr)` is true for each underlying error
- AND `InterruptMetrics.Snapshot.WaveCancelFailed == 1`
- AND `InterruptMetrics.Snapshot.D4CancelFailed == 1`
- AND `InterruptMetrics.Snapshot.ProcessCancelFailed == 1`
- AND `InterruptMetrics.Snapshot.HandleErrored == 1`
- AND the "stopped" EngineEvent is still emitted best-effort

#### Scenario: Partial failure

- GIVEN 1 of 3 cancelers fails (others return nil)
- WHEN `Handle` is called
- THEN the returned error contains only the failed step's wrapped error
- AND `InterruptMetrics.HandleErrored == 1`

#### Scenario: Backward-compat with nil Metrics

- GIVEN `InterruptOptions{Metrics: nil}` (pre-PR-A callers)
- WHEN `Handle` is called
- THEN `errors.Join` is still returned (no panic, no metric increments)
- AND the "stopped" event is still emitted if Sink is non-nil

### Requirement: Sandbox Cleanup Observability (devrix-d7-error-aggregation-and-metrics)

All `Sandbox.Exit` call sites across `freefork.Forker`, `execute.Executor`, and `wavescheduler.WorkerRunner` MUST surface Exit failures via atomic counter + `slog.Warn`, replacing the previous `_ = Sandbox.Exit(...)` silent-swallow pattern. **Note (2026-06-22):** the `sandbox_exit_failed` counter is owned by D4 (`multiagent/execute/worker.go::recordSandboxExitFailed`); the D7 wavescheduler does NOT emit it. See D7-S6-A14-T03 for the cross-domain clarification.

**Priority:** P0
**Package:** `internal/layers/multiagent/provision/freefork/` + `internal/layers/multiagent/execute/` + `internal/layers/orchestration/wavescheduler/`
**T:** D7-S6-A12-T01 (OBSOLETE — see D7-S6-A14-T03), D7-S6-A12-T02, T03, T04, T05, T06

#### Scenario: Forker sandbox Exit failure

- GIVEN `DefaultForker.WithMetrics(&ForkerMetrics{})` is set
- AND a rollback path triggers `Sandbox.Exit` that returns an error
- WHEN the rollback completes
- THEN `ForkerMetrics.Snapshot.SandboxExitFailed == 1`
- AND `slog.Warn("freefork: sandbox exit failed", ...)` is emitted

#### Scenario: Executor sandbox Exit failure

- GIVEN `Executor.WithMetrics(&ExecutorMetrics{})` is set
- WHEN `ExecuteSync` / `ExecuteAsync` / `forkWorker` cleanup paths trigger `Sandbox.Exit` failures
- THEN `ExecutorMetrics.Snapshot.SandboxExitFailed` is incremented (3 potential sites)
- AND each failure emits `slog.Warn("execute: sandbox exit failed", where=..., sessionID=..., ...)` with `where` distinguishing the call site

#### Scenario: TaskManager.publishCompletion panic

- GIVEN `notify.GlobalBus().Publish` (or a panicking subscriber) panics during `publishCompletion`
- WHEN the deferred recover catches the panic
- THEN `TaskManagerMetrics.Snapshot.PublishCompletionPanics.Add(1)` is incremented
- AND `slog.Error("taskmanager: publishCompletion panic", session=..., item_id=..., panic=..., metric="publish_completion_panics")` is emitted
- AND the publishCompletion goroutine does not crash the process

### Requirement: Forker errors.Join Aggregation + Backward Compatibility (devrix-d7-error-aggregation-and-metrics)

`DefaultForker.Fork` MUST aggregate errors from N concurrent spawn failures via `errors.Join`, replacing the previous `return nil, errs[0]` pattern that dropped N-1 errors. The `WithMetrics` setter MUST be backward-compatible with the 13 existing `NewDefaultForker` callers that don't explicitly wire metrics.

**Priority:** P0
**Package:** `internal/layers/multiagent/provision/freefork/`
**T:** D7-S6-A13-T07

#### Scenario: All N forks fail

- GIVEN N ForkRequests with a stub factory that fails every Create
- WHEN `Fork(ctx, parent, reqs)` is called
- THEN the returned error is `errors.Join(err1, err2, ..., errN)`
- AND each fork's name appears in the joined error message
- AND `errors.Is(returned, factoryErr)` is reachable

#### Scenario: Backward-compat with nil metrics

- GIVEN `NewDefaultForker(deps)` without `WithMetrics` call
- WHEN `Fork` is called
- THEN the call does not panic
- AND errors.Join aggregation still works (zero overhead)

#### Scenario: Hard assertion on leftover sandbox

- GIVEN a failed fork batch where rollback cleans up sandbox dirs
- WHEN the test inspects `wt.base` for leftover subdirectories
- THEN `t.Errorf` fires on any leftover (was `t.Logf` pre-PR-A, masking latent cleanup failures)

---

### Requirement: Metrics Naming Alignment & Concurrency Hardening (devrix-d7-metrics-and-concurrency-hardening)

D7-S6-A14 closes the remaining P0/P1 follow-ups from DM-20260621-010 (PR-B worktree
metrics) and the DM-20260621-009 deep review. It (1) aligns metric names with the
spec text so D5 dashboards can filter by name, (2) hands `sandbox_exit_failed`
ownership to its real emitter in D4, (3) bounds `state.cancels` and
`state.handles` so long-lived sessions with repeated wave re-entries do not
leak context-cancel references, (4) routes the dispatchLoop hot path through
`ConflictGuard.AllowAndRegister` to eliminate the TOCTOU window, and (5) hardens
`CommandHandler` against consumer stalls via `select-default` emit.

**Priority:** P1
**Package:** `internal/layers/orchestration/wavescheduler/` + `internal/layers/orchestration/sessionorchestrator/`
**T:** D7-S6-A14-T01 … D7-S6-A14-T06

#### Scenario: dispatch_loop_wakeups spec-aligned plural

- GIVEN `WaveScheduler.dispatchLoop` is running
- WHEN either `<-state.wakeupCh` or `<-ticker.C` fires
- THEN `s.incMetric("dispatch_loop_wakeups")` is invoked (plural, matches spec)
- AND D5 dashboards keyed on `dispatch_loop_wakeups` observe the count

#### Scenario: worker_panics spec-aligned plural

- GIVEN a worker goroutine inside `dispatchOne`
- WHEN the deferred recover catches a panic
- THEN `s.incMetric("worker_panics")` is invoked (plural, matches spec)
- AND `slog.Error(..., "metric", "worker_panics")` carries the matching key

#### Scenario: sandbox_exit_failed owned by D4 (cross-domain reference)

- GIVEN `multiagent/execute/worker.go::recordSandboxExitFailed` is the only emitter
- WHEN `sandbox_exit_failed` appears on the D5 dashboard
- THEN the counter comes from `ExecutorMetrics.SandboxExitFailed` (D4-S6-*)
- AND the D7 `SchedulerMetrics` struct does NOT carry a `sandbox_exit_failed`
  field (D7-S6-A12-T01 is OBSOLETE)
- AND `openspec/specs/d7-orchestration/t-registry.md` references D4-S6-A12-Txx
  for the actual owner

#### Scenario: state.cancels released after wave completion

- GIVEN a wave with N dispatched tasks
- WHEN `markWaveDone` runs (any terminal path: all-complete, ctx-cancel,
  or zero ready nodes)
- THEN `state.cancels` is `nil` (length 0)
- AND `state.handles` is an empty map
- AND a follow-up wave re-entry on the same session starts with empty bookkeeping

#### Scenario: ConflictGuard hot path uses AllowAndRegister

- GIVEN `WaveScheduler.dispatchLoop` iterating over ready nodes
- WHEN `dispatchOne` is called for each candidate
- THEN the conflict check is the atomic `AllowAndRegister` (single mutex
  acquisition covering both the conflict scan and the slot registration)
- AND `dispatchLoop` no longer pre-checks `guard.Allow` (which would leave
  a TOCTOU window between Allow and Register)
- AND the `go test -race` run for `TestD7S6A14T05_DispatchLoop_HotPathUsesAllowAndRegister`
  completes without race-detector findings

#### Scenario: CommandHandler emit drops events when consumer is wedged

- GIVEN `CommandHandler.Handle` returning a buffered channel (cap=4)
- WHEN the consumer fails to drain within the buffer capacity
- THEN `emit` uses `select { case out <- ev: default: slog.Warn(...) }`
- AND the Handle goroutine returns within bounded latency
- AND a `slog.Warn("command_handler: out channel full, drop event", ...)` is emitted

---

## REMOVED Requirements

### Requirement: PlanModeApproveGate Config (devrix-d7-uncertainty-gaps)

The `PlanModeApproveGate` config field has been removed across all config layers — Approve/Reject is driven by explicit CLI commands, not an extra config switch.

**Priority:** P0
**Packages:** `internal/layers/orchestration/orchtypes/config.go`, `internal/shared/config/coordinator.go`, `internal/shared/config/loader.go`, `internal/bootstrap/wire_coordinator.go`
**T:** D7-S5-A02-F05-T01 … D7-S5-A02-F05-T02

#### Scenario: Config struct no longer contains the field

- GIVEN the Config struct definition
- THEN `PlanModeApproveGate` field does not exist

#### Scenario: Default config compiles without it

- GIVEN the `DefaultConfig` function
- THEN no reference to `PlanModeApproveGate` exists

---

## Configuration

```yaml
context_engine:
  execution_flow:
    enabled: false              # 默认关闭，bootstrap 显式启用
    link_tasks: true
    im_progress: true
    tool_summary_throttle_ms: 500
    event_buffer_size: 32
  tasks:
    mode: v2                  # v1=todo, v2=task
    store_dir: "~/.devrix/tasks/"
  plan:
    enabled: false              # 显式 /plan 启用
    auto_detect: false

# D7 v1.0 规划配置（未实现）
orchestration:
  d7_enabled: true              # false 时 WireD7 失败，进程不启动（DM-007）
  routing_mode: loop_first      # loop_first (default) | rule_orchestrate (legacy ingress)
  fast_path:
    confidence_threshold: 0.9
  plan:
    max_tasks_per_plan: 20
    max_depth: 5
```

### Loop-First Routing (DM-20260616-002)

When `coordinator.routing_mode` is `loop_first` (default):

- Ingress: Skip | Command | Turn — no ingress-level `OrchestratePath`
- Wave/Plan: tool-gated inside Turn via `delegate_wave` / `enter_plan_mode`
- ShadowClassifier: continues tail-only observation on legacy orchestrate tail without changing routing
- Metrics: `orchestration.tool.delegate_wave`, route span label `turn` for loop_first messages

When `routing_mode=rule_orchestrate`, DM-20260615-004 ingress behavior is preserved (FastPathThreshold downgrade).

---

## Guides（互补，非登记 SoT）

- **领域 SoT**: `d7-domain.md` — North Star、Out of Scope、文档索引
- **终态架构**: `terminal-state-guide.md` — IntentKind 四链、跨域时序、路由矩阵
- **可观测性**: `observability-guide.md` — Span↔T、Trace 树、FastPath SLA、P0 Runbook
- **澄清归档**: `d7-requirements-clarifications.md` — Review R1/R2 完整条文

---

## Cross-Domain Contracts

| 契约 | 方向 | 接口 | 状态 |
|------|------|------|------|
| ExecutionFlowHub | D2/D4 → D7-S4 | `contracts.ExecutionFlowHub` | IMPLEMENTED |
| FlowBridge | D4 → Hub | `multiagent/delegate` FlowBridge | IMPLEMENTED |
| delegate_tools | D2 → D4 + Hub | `contextengine/delegate_tools.go` | IMPLEMENTED（目标由 D7 编排） |
| WorkPlanSnapshot | D7 → D2 Leader | `Hub.Snapshot` → delegate_status tool | IMPLEMENTED |
| D1 entry | D1 → D7 | `ProcessMessage` | IMPLEMENTED |
| **DM-020 LLMCaller 拆面** | **D7 → D2** | `contracts.LLMCaller` ← `turn.QueryLLMCaller` | **IMPLEMENTED** |
| **DM-020 Summarizer 拆面** | **D7 → D2** | `contracts.Summarizer` ← `turn.CompressionSummarizer` | **IMPLEMENTED** |
| **DM-20260617-008 Tool 端到端链路** | **D7 → D2** | `turn_adapter.ExecuteRound` (派发闸口) | **IMPLEMENTED** — 完整链路 SoT 在 `openspec/specs/d2-context-engine/spec.md` §"Tool Call End-to-End Flow" (Chain A/B/C + 5 surface 派发表 + 跨域拓扑) |
| **DM-20260625-003 MUPS v5 EscapeEngine 5 节点接线** | **D7 内部** | `escape.EscapeEngine` (5 wiring points 1a/1b/2/3 + 5 CB layers) | **IMPLEMENTED** — 5 节点统一逃逸机制, Plan/Execute 失败短路, Verify 失败跟进 (后续 PR 接入) |

---

## MUPS v5 EscapeEngine (DM-20260625-003, PR-V5.1 ~ V5.5)

### 5 节点统一逃逸机制

MUPS v5 在 5 节点管道 (Observe → Plan → Execute → Verify → Learn) 上落地统一逃逸机制, 通过 4 类深度限制 + 6 类 EscapeAction + 5 层 CircuitBreaker 实现回路深度不可被 LLM 操纵的硬保证.

#### 4 类深度限制 (V5.1-V5.2)

| 类型 | 实现 | 触发 | 文档 |
|------|------|------|------|
| 回路深度 | `LoopDepthTracker v2` (mode-hash SHA-256) | depth=3 → ForceExit | doc 38 §21.2 |
| PlanKind 切换累计 | `PlanKindSwitchPolicy` (3 档: Allowed/Constrained ≤ 4/Forbidden) | 累计超限 → ForceExit | doc 38 §21.4.2 |
| 仲裁深度 | `ChainedArbitrator` (LLM 5s → Rule → Human 10s) | 链式 3 层降级 | doc 38 §21.3.4 |
| CircuitBreaker 5 层 | `CircuitBreakerSet` (L0-L5) | 阈值触发 → open | doc 38 §21.13 |

#### 6 类 EscapeAction

| Action | 语义 | 触发 |
|--------|------|------|
| `EscapeContinue` | 继续回路 (正常路径) | 0 信号 / chain 裁决通过 |
| `EscalateToRule` | LLM 升级到 Rule | LLM 决定需要规则检查 |
| `EscalateToHuman` | 升级到 Human | 不可恢复失败需要人类介入 |
| `EscapeForceExit` | 强制退出 | 硬信号 / chain 兜底 |
| `EscapeAbortWithAudit` | 终止 + 完整审计 | 不可恢复错误 |
| `EscapePendingHuman` | Human 异步入口 | T2 ResumeSession 续跑 |

#### 5 层 CircuitBreaker (V5.4)

| 层 | 名称 | 阈值 | 触发场景 |
|----|------|------|---------|
| L0 | AnomalyDetector | 5 次连续 nil | 异常检测失效 |
| L1 | DispatchLoopWakeups | 100 次/分钟 | 派发回路失控 |
| L2 | Verifier | 3 次 > 2s | LLM 调用延迟降级 |
| L3 | Hook | 5 次连续 fail | Hook 失败 |
| L4 | WorkerPanic | 1 次 panic | Worker 严重错误 |
| L5 | SandboxExit | 5 次连续 fail | 沙箱失败 |

#### 5 节点接线点 (V5.5)

`SessionOrchestrator` 通过 `WithEscapeEngine` option 接入 EscapeEngine, 4 个接线点:

| # | 触发点 | EscapeEngine.Evaluate 行为 | 失败降级 |
|---|--------|---------------------------|---------|
| 1a | Plan 失败 (classifier error) | 1 信号 → 直接 ForceExit | err 透传 |
| 1b | Plan 前 (before dispatch) | 1 信号 → 直接返回 | 短路 return |
| 2 | Execute 失败 (path error) | 1 信号 → 直接 ForceExit | err 透传 |
| 3 | Verify 失败 (verdict FAIL/INDET) | 1 信号 → 直接 ForceExit | err 透传 (待 processAutoClose 暴露 verdict) |

Engine 决策合并逻辑 (V5.4):
- 0 非 Continue 信号 → EscapeContinue (正常回路)
- 1 信号 → 直接返回 (硬信号优先级, 跳过 LLM)
- 2+ 信号 → ChainedArbitrator 仲裁 (多源冲突, LLM 介入)

#### 关键设计不变式 (V5.x)

1. **失败降级**: 任何内部 panic/error → slog.Warn + 继续 (不阻塞主链路)
2. **200ms 评估超时**: CircuitBreaker 拉 metric 阻塞 → slog.Warn + 不触发
3. **panic recovery**: Evaluate 内部 panic → EscapeContinue (不向上传)
4. **T2 续跑入口**: HumanArbitrator.ResumeSession 由 EscapeEngine.ResumeSession 委托
5. **向后兼容**: `WithEscapeEngine(nil)` → V5.4- 行为 (不评估, 不破坏现有)

#### 与 Phase 1-7 数据契约兼容

- 14 ExitReason (Phase 4): v5 决策不影响 ExitReason 字段
- 4 VerdictKind (Phase 4): v5 不修改 verdict 合成
- ProcessRequest.TrackMode (Phase 7): v5 不影响 TrackMode 处理
- buildObserveRequest 3 层 fail-safe (Phase 6): v5 不修改 AdaptivePrior 注入

---

## Scenario D7-S15: WorkItem Rollup 闭环 (DM-20260627-001)

> **Archived change:** `openspec/archive/2026-06-27-devrix-d7-workitem-rollup-pipeline/`  
> **Design SoT:** change package `design.md` §4–§8; **Phase 2** decompose/parallel 登记不编码。

### Requirement: D7-S15-A50 Parent Rollup Gate

When all direct non-checklist children of a decomposed parent reach terminal status, `ReevaluateParentAfterChild` shall set `NeedsRollup=true`, reopen the parent to `pending`, and **not** auto-complete the parent until rollup MUPS finishes.

#### Scenario: best_effort triggers rollup when all children terminal including failures

- **Given** parent `P` with `LastRound.SpawnPolicy=SpawnDecompose` and `RollupGatePolicy=best_effort` (Phase 1 default via `RollupGatePolicyFor`)
- **When** all implement children reach terminal status (pass or fail)
- **Then** `ShouldRollupAfterChildren` returns true and `P.NeedsRollup=true`

### Requirement: D7-S15-A51 Summary Bubble Materialize

Rollup Observe shall inject **both** `structured_child_bubble` and `summary_child_bubble` observations per terminal child with artifact summary (CB3 truncate).

### Requirement: D7-S15-A55 RollupGatePolicy

Phase 1 ships **`best_effort` only** (`RollupGatePolicyFor`); `all_pass` logic exists for tests; `min_coverage` deferred to Phase 2.

### Requirement: D7-S15-A60 Parent Rollup Round 2+ MUPS

When `NeedsRollup=true`, ItemPipelineRunner executes CommitmentPlan rollup round with `verifyRollupArtifact` heuristic (len≥500, P0/P1, planning denylist).

### Requirement: D7-S15-A61 Session complete Deliverable

TurnLoop exit shall populate `complete.Content` from root post-rollup `LastRound.ArtifactSummary` via `ExtractSessionDeliverable`.

### Requirement: D7-S15-A53 Ephemeral Checklist Gate

`GetPipelineFocus` skips ephemeral checklist WorkItems; checklist children do not run MUPS.

### Requirement: D7-S15-A54 Root Session Rollup Fallback (Path B)

When root goal Round 1 ends with `SpawnNone` or failed + ephemeral checklist children exist, `MaybeRootRollupFallback` sets `NeedsRollup=true` before session close.

> **本 change 合入 ≠ WorkTree v2 完成** — Phase 2 DecomposeProposer / ParallelExplore / min_coverage / ExpectedReturn text match 独立 PR。

---

## Scenario D7-S16: Layer SubContext (DM-20260627-003 + DM-20260628-002)

> **Archived changes:** `openspec/archive/2026-06-28-devrix-d7-layer-subcontext/` (Phase 1+2) · `openspec/archive/2026-06-28-devrix-d7-layer-subcontext-phase3/` (Phase 3)  
> **Design SoT:** CG2′ in `workitem-context-graph-design.md` v0.4.0

### Requirement: D7-S16-A60 ScopeContract — Goal 范围收敛

Goal WorkItem Plan/Execute shall produce a parseable **ScopeContract** (`goal_statement`, `in_scope`, `out_of_scope`, `open_questions`, …). Non-empty `open_questions` blocks `SpawnDecompose` (rule gate + R-OBS-1 ObsUncertainty).

### Requirement: D7-S16-A61 ChildDownlink

Decompose persists **ChildDownlink** with mandatory non-empty `ExpectedReturn`; Materialize injects Directive + ScopeIn/Out for depth ≥ 1 children.

### Requirement: D7-S16-A62 LayerCohort Partition

Siblings under one parent share **cohort domain** (ScopeContract meta) via `EnsureCohortScope`; WorkItemPrivate transcript remains per `wi:<sid>:<wi_id>`.

### Requirement: D7-S16-A63 UpstreamSignal (BlockedBy)

Materialize mode **Upstream** injects structured bubble + truncated artifact summary only; blocker private jsonl shall not appear in dependent payload.

### Requirement: D7-S16-A64 PeerStatusSignal

When cohort size ≥ 3, terminal siblings may record **PeerStatusSignal**; Materialize may inject `peer_status` lines (no live ReAct tail sharing).

### Requirement: D7-S16-A70 WorkItemExecutor Materialize 接线

WorkItems at **depth ≥ 1** shall obtain LLM context via D2 `ContextMaterializer.Materialize` (default on, no env flag). L0 Goal continues legacy `ContextPreparer.Prepare`. Jaeger span `D2_Context_Materialize` records `wi_id`, `policy`, `message_count`.

### Requirement: D7-S16-A72 Signal→Observation 边界

Execute ReAct transcript shall **not** require per-iteration ObsFact/ObsSignal/ObsDeviation/ObsUncertainty self-labels; Observe rules map ScopeContract / bubbles to Obs* (R-OBS-1..7).

### Requirement: D7-S16-A73 Execute Structured Delivery Template

Materialize may soft-guide `<conclusion>` / `<open_questions>`; LastRound is Signal carrier, not UncertaintyReport SoT.

### Requirement: D7-S16-A65 SubTurn → MaterializePolicy (DM-20260628-002)

`PolicyFromSubTurnMode` maps SubTurn `brief`/`fork`/`full` to Materialize `fresh`/`fork`/`resume`. When `SubTurnRunner.Materializer` is wired, delegate sub-agents assemble context via D2 Materialize (`PartitionAgent`); nil Materializer falls back to legacy `applyMode`.

### Requirement: D7-S16-A66 Wave ContextResolver → Materializer (DM-20260628-002)

Wave worker policies `fresh`/`resume`/`upstream` delegate to D2 Materialize via `PartitionWave`. `NewMaterializingContextResolver` is the bootstrap default when Materializer is available; legacy `ContextResolver` remains when Materializer is nil.

### Requirement: D7-S16-A74 LLM ObservationProposer @ Observe (DM-20260628-002)

Optional `ObservationProposer` may propose Obs* from structured signals (directive, ScopeContract, inbound signal lines, prior) **without** WorkItem private ReAct transcript. `ValidateObservationProposals` rule-gates proposals (ObsFact strength ≤ 0.85, mandatory evidence); LLM failures are fail-safe (rules-only Observe continues).

> **Layer SubContext Phase 1–3 闭环** — WorkTree v2 其他项见 `tech-debt/worktree-v2-deferred.md`。

---

## Scenario D7-S20 / S21: TaskContract 统一（v7.0 PR-A, DM-20260629-007）

> **Change:** `devrix-d7-taskcontract-unification-pr-a`（DM-20260629-007）  
> **Change SoT:** `openspec/changes/devrix-d7-taskcontract-unification-pr-a/`  
> **归档:** 待 S5-S6 完成后移 `openspec/archive/2026-06-29-devrix-d7-taskcontract-unification-pr-a/`
>
> **v7.0 演进起点：** D7 缺契约不缺机制 → P0=TaskReport 五元素（缺 Dissent/Blockage/Resource）+ P1=TaskSpec 四元组（Plan/Channel/WorkItem 分散）。
>
> **本次 PR 范围：** L1 接口层（D7-S20-A01 TaskSpec + D7-S20-A02 TaskReport）+ L2 字段语义层（D7-S21-A01 Dissent + D7-S21-A02 Blockage + D7-S21-A03 Resource）+ L4 spec 同步（D7-S20-A03）。
>
> **L3 防御运行时层（Pessimistic Commit / Hard Evidence / CoW VersionChain / Rule-based Fallback / Similarity Check）保留给 PR-B + PR-C**，登记见 `f-registry.md` §D7-S22 F01-F02 PLANNED。

### Phase 1 (PR-A) 实现登记

| Layer | 主题 | 物理位置 | 落地状态 |
|-------|------|----------|---------|
| **L1 接口层** | **TaskSpec struct**（SessionID/Plan/Channel/WorkItem/TraceID/Blockage?）+ **NewTaskSpec** fail-fast（3 SentinelError：7100/7101/7104）+ **3 创建点统一**（Plan/Channel/WorkItem）+ **ChannelRequest.Spec additive 嵌入** | `orchestration/interfaces/task_spec.go` + `mups/execute/channel.go` | ✅ PR-A 落地 |
| **L1 接口层** | **TaskReport struct**（SessionID/Channel/Verdict/TraceID/Dissents/Blockage?/Resource）+ **NewTaskReport** fail-fast（3 SentinelError：7102/7103/7104）+ **Channel.Execute 出口** + **Learn 入口**统一 | `orchestration/interfaces/task_report.go` + `mups/learn/asset/asset_builder.go` | ✅ PR-A 落地 |
| **L2 字段语义层** | **Dissent** top-3 silent truncate + summary hash fnv64a→8 hex + Learn 沉淀 | `orchestration/interfaces/task_report.go::AppendDissent + HashSummary` | ✅ PR-A 落地 |
| **L2 字段语义层** | **Blockage** 3 类 kind 分类（permission/resource/contract）+ retryable 推导 | `orchestration/interfaces/task_spec.go::WithBlockage` | ✅ PR-A 落地 |
| **L2 字段语义层** | **Resource** token/time/step 三件套抽取 | `orchestration/interfaces/task_report.go::WithResource` | ✅ PR-A 落地 |
| **L4 治理横切层** | **6 spec 文件同步**（spec.md + d7-domain + a/f/t/span-registry）+ **5 ORCH_* SentinelError**（7100-7104）+ **5 P0 TaskContract span** + **interfaces 0 import D7 子包 lint invariant** | 各 spec/registry + `scripts/ci-lint-invariant/` | ✅ PR-A 部分落地（spec sync + 5 sentinel + 5 span + lint 4/9 项） |
| **L3 防御运行时层** | Pessimistic Commit + Hard Evidence + CoW VersionChain + Rule-based Fallback + Similarity Check | `orchestration/interfaces/contracts.go` DESIGN ONLY | ⬜ PLANNED PR-B + PR-C（8 T） |

### PR-A 测试矩阵（11 P0 T）

| T ID | 描述 | Status | Test Location | Span Evidence |
|------|------|--------|---------------|---------------|
| **D7-S20-A01-T01** | NewTaskSpec + Validate happy path + SpecHash 稳定 | **IMPLEMENTED** | `orchestration/interfaces/task_spec_test.go::TestNewTaskSpec_HappyPath` + `TestValidate` | Task_Spec_Created |
| **D7-S20-A01-T02** | WithPlan/WithChannel/WithWorkItem 不可变 builder 浅拷贝 | **IMPLEMENTED** | `orchestration/interfaces/task_spec_test.go::TestWith*` | — |
| **D7-S20-A01-T03** | Plan 节点入口 + Channel.Execute 入口 + WorkItem 节点入口 3 处统一调 NewTaskSpec | **IMPLEMENTED** | `orchestration/interfaces/taskcontract_test.go::TestThreeEntryPointsConverge` | Task_Spec_Created |
| **D7-S20-A02-T01** | NewTaskReport + Validate happy path | **IMPLEMENTED** | `orchestration/interfaces/task_report_test.go::TestNewTaskReport_HappyPath + TestValidate` | Task_Report_Created |
| **D7-S20-A02-T02** | WithVerdict/WithResource/WithBlockage + AppendDissent 不可变 + top-3 silent truncate | **IMPLEMENTED** | `orchestration/interfaces/task_report_test.go::TestWith* + TestAppendDissent_Truncate` | Task_Report_Created |
| **D7-S20-A02-T03** | Channel.Execute 出口 + Learn 节点入口统一用 NewTaskReport | **IMPLEMENTED** | `orchestration/interfaces/taskcontract_test.go::TestChannelExecuteAndLearnEntryUnified` | Task_Report_Created |
| **D7-S21-A01-T01** | Dissent top-3 截断 + summary hash fnv64a 稳定 + Learn 沉淀到 feedback 通道 | **IMPLEMENTED** | `orchestration/interfaces/task_report_test.go::TestDissent_Top3 + TestHashSummary + TestLearnPrecipitation` | Dissent_Recorded |
| **D7-S21-A02-T01** | Blockage 3 类 kind（permission/resource/contract）+ retryable 推导 | **IMPLEMENTED** | `orchestration/interfaces/task_spec_test.go::TestBlockage_3Kinds` | Blockage_Recorded |
| **D7-S21-A03-T01** | Resource token/time/step 三件套抽取 + 零值兜底 | **IMPLEMENTED** | `orchestration/interfaces/task_report_test.go::TestResource_ThreePiece + TestResource_ZeroDefault` | Resource_Recorded |
| **D7-S20-A03-T01** | spec.md v4.16.0 ADDED 3 Requirement（D7-S20-A01/A02/A03）+ 12+ Gherkin Scenarios | **PENDING** | spec.md §ADDED Requirements（v7.0 ADDED 段已就绪） | — |
| **D7-S20-A03-T02** | d7-domain.md §DSAFT 资产 + 新 §8 Layer + 新 §9 interfaces 包 + a/f/t/span-registry 全增量 | **PENDING** | 6 spec 文件同步完成 | — |

**PR-A Total:** 9/11 IMPLEMENTED + 2 PENDING spec 同步（spec.md + d7-domain 已落地，本 T 由 PR 创建时合并 commit 完成 IMPLEMENTED 闭环）

### 不变式（PR-A 必保）

| Invariant | 物理约束 | 验证方式 |
|-----------|---------|---------|
| **IV-1:** interfaces 0 import D7 子包 | `go vet` + `scripts/ci-lint-invariant/TestInterfacesZeroImportD7` | `grep -r "internal/layers/orchestration/" internal/layers/orchestration/interfaces/` 0 命中（除自身包外）|
| **IV-2:** TaskSpec/TaskReport 不可变（无 setter）| 单元测试覆盖 With* 浅拷贝 | `TestTaskSpec_Immutable_*` + `TestTaskReport_Immutable_*` |
| **IV-3:** AppendDissent top-3 silent | 单元测试覆盖第 4 个不改变切片 | `TestAppendDissent_Truncate` |
| **IV-4:** TraceID `ts_<8 hex>` 格式 | NewTaskSpec/NewTaskReport fail-fast + 单测覆盖 6 个 case | `TestTraceIDFormat_*` |
| **IV-5:** Additive 嵌入零破坏 | ChannelRequest/LearnRequest 老调用方 0 变化 + 24/24 packages -race PASS | `go test -race ./internal/...` 全绿 |

### 实施统计

- **代码：** 9 文件（7 NEW + 2 MODIFIED）+1100 行（interfaces 包 7 .go + 2 .go 嵌入点）
- **测试：** 11 P0 T（9 单元/集成 + 2 spec 同步）+ 30+ 子测试用例
- **性能：** interfaces 包纯类型 → 零 P99 开销（< 1μs 构造 + Validate）
- **跨域：** 0 import D7 子包（pure types） + 1 import `internal/shared/errors`（SentinelError）
- **回归：** 24/24 orchestration packages `go test -race` PASS，interfaces 包 coverage 95%

### PR-A 与 PR-B + PR-C 的边界

| 主题 | PR-A 本次 | PR-B 下次 | PR-C 后续 |
|------|-----------|-----------|-----------|
| L1 接口层 | ✅ TaskSpec + TaskReport + 3 创建点 | — | — |
| L2 字段语义层 | ✅ Dissent + Blockage + Resource | — | — |
| L3 防御运行时层 | ⬜ | ✅ Pessimistic Commit + Hard Evidence + Rule-based Fallback | ✅ CoW VersionChain + Similarity Check |
| L4 治理横切层 | ✅ spec + 5 sentinel + 5 span + lint invariant | ✅ Coverage + Perf + Security | ✅ Cross-Domain Boundary + Feature Flag + AdaptiveThreshold + Layout Guard |

---

## Scenario D7-S18: Pessimistic Commit + Rule-based Fallback（v7.0 PR-B, DM-20260629-008）

> **Change:** `devrix-d7-taskcontract-unification-pr-b`（DM-20260629-008）  
> **Change SoT:** `openspec/changes/devrix-d7-taskcontract-unification-pr-b/`  
> **归档:** 待 S5-S6 完成后移 `openspec/archive/2026-06-29-devrix-d7-taskcontract-unification-pr-b/`
>
> **L3 防御运行时层 PR-B 范围：** PessimisticCommitGuard interface + 5 类触发（resource_exhausted / cb_l1 / indeterminate_3x / empty_evidence / manual_abort）+ MVPArtifact best-effort 输出 + 4 候选规则 Rule-based Fallback + 4 ORCH_* SentinelError 7110-7113 + 1 Pessimistic_Commit_Emit span + Feature Flag env-gated 默认 disabled.
>
> **PR-C 后续：** Hard Evidence（强制 evidence 完整）+ CoW VersionChain + Similarity Check + T05 Span/Metric 完整 wire.

### PR-B 实现登记

| Layer | 主题 | 物理位置 | 落地状态 |
|-------|------|----------|---------|
| **L3 防御运行时层** | **PessimisticCommitGuard interface**（Evaluate / ResolveFallback / BuildMVPArtifact）+ 5 类触发条件 | `orchestration/interfaces/contracts.go` + `orchestration/escape/fallback.go::DefaultPessimisticCommitGuard` | ✅ PR-B 落地 |
| **L3 防御运行时层** | **FallbackPolicy helpers**（4 候选规则 + Default + Valid + ParseFallbackRuleName） | `orchestration/interfaces/fallback_policy.go` | ✅ PR-B 落地 |
| **L3 防御运行时层** | **ConvergenceBudget helpers**（NewConvergenceBudget + With* + Validate + RemainingBelowReserve） | `orchestration/interfaces/convergence_budget.go` | ✅ PR-B 落地 |
| **L3 防御运行时层** | **Engine.NotifyPessimistic**（5 层 fail-safe: nil guard / nil report / Evaluate error→fall-open / blocked→MVPArtifact inject / Result.Kind force）+ **Engine.SetPessimisticGuard/PessimisticGuard** accessor | `orchestration/escape/engine.go` | ✅ PR-B 落地 |
| **L3 防御运行时层** | **ChannelRouter.SetPessimisticGuard** + **ApplyPessimisticCommit**（Additive 嵌入）+ **3 channel 入口统一调** | `orchestration/mups/execute/channel.go` | ✅ PR-B 落地 |
| **L3 防御运行时层** | **bootstrap.PessimisticCommitEnabled / PessimisticRuleStrategy / NewPessimisticCommitGuardFromEnv**（Feature Flag env-gated） | `internal/bootstrap/pessimistic_guard_wire.go` | ✅ PR-B 落地 |
| **L3 防御运行时层** | **5 层 CB L1 联动**：L1 trips StateOpen + 60s 持久窗口 + L1-only reason 含 "l1" 路由 hint | `orchestration/escape/circuit_breaker_test.go::TestL1*` (3 NEW tests) | ✅ PR-B 落地 |
| **L4 治理横切层** | **6 spec 文件同步** + **4 ORCH_* SentinelError**（7110-7113）+ **1 Pessimistic_Commit_Emit span**（7 attrs）+ **interfaces 0 import D7 invariant IV-1 扩展 IV-5 nil/disabled-safe** | 各 spec/registry + `internal/shared/errors/WithCode` | ✅ PR-B 落地 |

### PR-B 测试矩阵（7 P0 T）

| T ID | 描述 | Status | Test Location | Span Evidence |
|------|------|--------|---------------|---------------|
| **D7-S18-A11-T01** | PessimisticCommitGuard.Evaluate happy path（资源充足 + CB 健康 + 无 INDETERMINATE streak + 非空 evidence + 无 manual abort）→ `(true, "", nil)` | **IMPLEMENTED** | `internal/layers/orchestration/escape/fallback_test.go::TestDefaultPessimisticCommitGuard_Enabled_HappyPath` | Pessimistic_Commit_Emit |
| **D7-S18-A11-T02** | PessimisticCommitGuard.BuildMVPArtifact（5 类触发 → MVPArtifact）— Output 非空 + RiskWarnings + Trigger + ChainHash FNV-1a 16-hex 稳定 | **IMPLEMENTED** | `internal/layers/orchestration/escape/fallback_test.go::TestDefaultPessimisticCommitGuard_BuildMVPArtifact + TestBuildMVPArtifact_Traceback + TestBuildChainHash_Stable + TestBuildMVPArtifact_NilReport` | Pessimistic_Commit_Emit |
| **D7-S18-A11-T03** | 5 层 CB L1 → Pessimistic action（100 wakeups/min 触发 L1 Open + 60s 持久窗口 + L1-only reason 含 "l1" 路由 hint） | **IMPLEMENTED** | `internal/layers/orchestration/escape/circuit_breaker_test.go::TestL1DispatchLoop_PessimisticHint + TestL1StateOpen_PersistentForPessimisticWindow + TestCircuitBreakerSet_L1Only_PessimisticCompatible` | Pessimistic_Commit_Emit |
| **D7-S18-A11-T04** | Feature Flag env-gated 默认 disabled（`D7_PESSIMISTIC_COMMIT_ENABLED` unset → Enabled=false + 8 truthy + 5 falsy value 全覆盖） | **IMPLEMENTED** | `internal/bootstrap/pessimistic_guard_wire_test.go::TestPessimisticCommitEnabled_DefaultsOff + TestPessimisticCommitEnabled_Truthy + TestPessimisticCommitEnabled_Falsy + TestPessimisticRuleStrategy_Default + TestPessimisticRuleStrategy_AllValid + TestPessimisticRuleStrategy_InvalidFallsBack + TestNewPessimisticCommitGuardFromEnv_OffByDefault + TestNewPessimisticCommitGuardFromEnv_EnabledWithCustomRule` (8 tests) | Pessimistic_Commit_Emit |
| **D7-S18-A11-T05** | Span `d7.s18.pessimistic.commit.emit` + Metric `pessimistic_commit_trigger_count` + `mvp_artifact_generated_total` + `pessimistic_commit_latency_us` 完整 wire | **PLANNED** | `engine.go::NotifyPessimistic` 内 `slog.Info("pessimistic_commit_emit", ...)` 占位（结构化字段已对齐 7 attrs）；完整 Jaeger (OTel) + Prometheus wire 待 PR-C | — (PLANNED T05) |
| **D7-S18-A12-T01** | 4 候选规则实现（most_tests_passed / compiled_clean / min_cost / min_uncertainty，default min_uncertainty）— ClosedSet + Valid + ValidNonLegacy + Default 稳定 | **IMPLEMENTED** | `internal/layers/orchestration/interfaces/fallback_policy_test.go::TestValid + TestValidNonLegacy + TestParseFallbackRuleName (9 cases) + TestClosedSet + TestDefaultFallbackRule_Stable` (5 tests) + `internal/layers/orchestration/escape/fallback_test.go::TestResolveFallback_Default + TestResolveFallback_PolicyOverride` (2 tests) | Pessimistic_Commit_Emit (fallback_used=true) |
| **D7-S18-A12-T02** | env `D7_RULE_FALLBACK_STRATEGY` 切换（default min_uncertainty + 4 候选规则切换 + invalid rule silent fall-back） | **IMPLEMENTED** | `internal/bootstrap/pessimistic_guard_wire_test.go::TestPessimisticRuleStrategy_Default + TestPessimisticRuleStrategy_AllValid + TestPessimisticRuleStrategy_InvalidFallsBack + TestNewPessimisticCommitGuardFromEnv_EnabledWithCustomRule` (4 tests) | Pessimistic_Commit_Emit (fallback_used=true) |

**PR-B Total:** 6/7 IMPLEMENTED + 1 PLANNED T05 (Span/Metric 完整 wire 留 PR-C).

### PR-B 落地的 SentinelError (7110-7113)

| Err 常量 | Code | 触发条件 | 返回方式 | Span Attr |
|----------|------|---------|----------|-----------|
| `ErrORCHPessimisticTriggered` | 7110 | Evaluate 返回 blocked（5 类触发条件之一命中） | `sharederrors.WithCode` | `reason` = 5 trigger 之一 |
| `ErrORCHPessimisticMVPEmpty` | 7111 | `BuildMVPArtifact` 输出空（producer 须保证 Output 非空） | `sharederrors.WithCode` | `mvp.artifact_hash` = "" |
| `ErrORCHFallbackRuleInvalid` | 7112 | env `D7_RULE_FALLBACK_STRATEGY` 不在 4 候选规则内（仅 caller explicit reject 触发；env 路径 silent fall-back） | `sharederrors.WithCode` | — |
| `ErrORCHFallbackAbortTimeout` | 7113 | FallbackAbort 超时（producer 须 respect `time_budget_ms`） | `sharederrors.WithCode` | `policy` = "abort" |

### 不变式（PR-A 必保 + PR-B 新增 IV-5）

| Invariant | 物理约束 | 验证方式 |
|-----------|---------|---------|
| **IV-1:** interfaces 0 import D7 子包 | `go vet` + `scripts/ci-lint-invariant/TestInterfacesZeroImportD7` | `grep -r "internal/layers/orchestration/" internal/layers/orchestration/interfaces/` 0 命中（除自身包外）|
| **IV-2:** TaskSpec/TaskReport 不可变（无 setter）| 单元测试覆盖 With* 浅拷贝 | `TestTaskSpec_Immutable_*` + `TestTaskReport_Immutable_*` |
| **IV-3:** AppendDissent top-3 silent | 单元测试覆盖第 4 个不改变切片 | `TestAppendDissent_Truncate` |
| **IV-4:** TraceID `ts_<8 hex>` 格式 | NewTaskSpec/NewTaskReport fail-fast + 单测覆盖 6 个 case | `TestTraceIDFormat_*` |
| **IV-5（PR-B 新增）:** PessimisticCommitGuard nil-safe / disabled-safe | guard==nil 或 guard.Enabled=false → Evaluate return (true, "", nil) + 不注入 MVPArtifact | `TestDefaultPessimisticCommitGuard_NilReceiver + TestDisabled_NilReport + TestEnabled_HappyPath` (3 tests) |

### 实施统计（PR-B）

- **代码：** 7 NEW + 4 MODIFIED 文件（interfaces 3 NEW + escape/fallback.go NEW + engine.go MOD + mups/execute/channel.go MOD + bootstrap/pessimistic_guard_wire.go NEW + circuit_breaker_test.go MOD + engine_test.go MOD + escape/fallback_test.go NEW）
- **测试：** 7 P0 T（6 IMPLEMENTED + 1 PLANNED T05）+ 25+ 子测试用例（interfaces 16 + escape 14 + bootstrap 8 + circuit_breaker 3 + engine 4）
- **覆盖率：** interfaces **96.9%** + escape **85.0%** + bootstrap **90%+** （均 ≥ 80%）
- **性能：** PessimisticCommitGuard.Evaluate < 1ms（P99，fall-open path 接近 0μs）
- **跨域：** interfaces 0 import D7 子包（pure types IV-1 维持）+ 仅依赖 `internal/shared/errors`（SentinelError）
- **回归：** 22/22 orchestration packages `go test -race` PASS + LP-1/LP-2/LP-5 集成测试 100% 兼容（Feature Flag 默认 disabled 0 行为变更）

### PR-B 与 PR-C 的边界

| 主题 | PR-B 本次 | PR-C 后续 |
|------|-----------|-----------|
| L3 防御运行时层（5 类触发）| ✅ ResourceExhausted / CB_L1 / Indeterminate3x / EmptyEvidence / ManualAbort | ✅ Hard Evidence 强制完整 |
| L3 防御运行时层（Fallback）| ✅ 4 候选规则 Rule-based + 3 FallbackPolicy 路径 | ✅ CoW VersionChain (version_id) + Similarity Check (intra-Dissent 重复检测) |
| L3 防御运行时层（可观测）| ⬜ T05 Span/Metric 完整 wire（仅 slog.Info 占位） | ✅ T05 OTel span 注册 + Prometheus metric emit (5 metrics: pessimistic_commit_trigger_count / fallback_rule_select_total / mvp_artifact_generated_total / pessimistic_commit_latency_us / fallback_rule_apply_total) |
| L4 治理横切层 | ✅ Coverage 96.9%/85.0% + 4 SentinelError 7110-7113 + 1 Span + IV-5 nil/disabled-safe invariant | ✅ Cross-Domain Boundary + Feature Flag (D7_PESSIMISTIC_COMMIT_ENABLED 完整治理) + AdaptiveThreshold + Layout Guard |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| **4.17.0** | **2026-06-29** | **devrix-d7-taskcontract-unification-pr-b (DM-20260629-008) PR-B 落地 v7.0 TaskContract L3 防御运行时层（部分）**：(1) **ADDED 2 Requirement** D7-S18-A11 Pessimistic Commit (PessimisticCommitGuard interface + 5 类触发 resource_exhausted/cb_l1/indeterminate_3x/empty_evidence/manual_abort + MVPArtifact best-effort 输出 + 4 ORCH_* SentinelError 7110-7111 + Engine.NotifyPessimistic 5 层 fail-safe + Additive 嵌入 Engine.pessimisticGuard) + D7-S18-A12 Rule-based Fallback (4 候选规则 most_tests_passed/compiled_clean/min_cost/min_uncertainty default min_uncertainty + 2 ORCH_* SentinelError 7112-7113 + env D7_RULE_FALLBACK_STRATEGY 切换 + invalid rule silent fall-back)；(2) **ADDED 11 Gherkin Scenarios**（disabled pass-through + 5 类触发单测 + happy path + BuildMVPArtifact 5/empty + 5-layer fail-safe 4 case + Feature Flag 2 case + 4 candidate rules enumeration + default + invalid fall-back + ResolveFallback 2 paths + override + 2 Select + closed set）；(3) **ADDED Scenario D7-S18 Section**（7 P0 T 矩阵 6 IMPLEMENTED + 1 PLANNED T05 留 PR-C）；(4) **4 个 ORCH_* SentinelError**（Code 7110-7113：PessimisticTriggered/PessimisticMVPEmpty/FallbackRuleInvalid/FallbackAbortTimeout）通过 `internal/shared/errors/WithCode` 注册；(5) **1 不变式 IV-5 新增**（PessimisticCommitGuard nil-safe / disabled-safe invariant）；(6) **interfaces 包 +3 NEW 文件**（contracts.go + fallback_policy.go + convergence_budget.go 共 10 文件）0 import D7 子包，coverage **96.9%**；**escape/fallback.go** (~310 LOC) + **bootstrap/pessimistic_guard_wire.go** (~75 LOC) NEW；**escape/engine.go** +NotifyPessimistic (5 层 fail-safe) + **mups/execute/channel.go** +ChannelRouter.SetPessimisticGuard/ApplyPessimisticCommit；(7) 22/22 orchestration packages `go test -race` PASS + LP-1/LP-2/LP-5 集成测试 100% 兼容（**Feature Flag D7_PESSIMISTIC_COMMIT_ENABLED 默认 disabled 0 行为变更**）；detail 在 `Scenario D7-S18` 段 |
| **4.16.0** | **2026-06-29** | **devrix-d7-taskcontract-unification-pr-a (DM-20260629-007) PR-A 落地 v7.0 TaskContract L1+L2+L4（部分）**：(1) **ADDED 3 Requirement** D7-S20-A01 TaskSpec 下行契约 (NewTaskSpec fail-fast ORCH 7100/7101/7104 + Validate + With* 不可变 + 3 创建点统一 + ChannelRequest.Spec additive) + D7-S20-A02 TaskReport 上行契约 (NewTaskReport fail-fast ORCH 7102/7103/7104 + WithVerdict/WithResource/WithBlockage + AppendDissent top-3 + HashDissentSummary + LearnRequest.Report additive) + D7-S20-A03 TaskContract 治理横切 (6 spec 文件同步 + interfaces 0 import D7 lint invariant)；(2) **ADDED 15+ Gherkin Scenarios**（happy + error path × 3 Requirements 全覆盖）；(3) **ADDED Scenario D7-S20/S21 Section**（11 P0 T 矩阵 9 IMPLEMENTED + 2 spec 同步 PENDING）；(4) **5 个 ORCH_* SentinelError**（Code 7100-7104：TaskSpecEmpty/ChannelUnknown/ReportEmpty/VerdictEmpty/TraceInvalid）通过 `internal/shared/errors/WithCode` 注册；(5) **5 不变式 IV-1..IV-5**（pure types + 不可变 builder + top-3 silent + trace_id 格式 + additive 零破坏）；(6) **interfaces 包 7 NEW 文件**（doc.go + errors.go + task_spec.go + task_report.go + 3 _test.go）0 import D7 子包，coverage 95%；(7) 24/24 orchestration packages `go test -race` PASS；detail 在 `Scenario D7-S20 / S21` 段 |
| **4.15.0** | **2026-06-28** | **devrix-d7-layer-subcontext-phase3 (DM-20260628-002) Phase 3 闭环**：(1) ADDED D7-S16-A65 SubTurn→MaterializePolicy；(2) ADDED D7-S16-A66 Wave ContextResolver→Materializer；(3) ADDED D7-S16-A74 LLM ObservationProposer @ Observe（G3 提案+规则校验）；PR #273–#275 |
| **4.14.0** | **2026-06-28** | **devrix-d7-layer-subcontext (DM-20260627-003) Phase 1+2 Layer SubContext**：(1) ADDED Scenario D7-S16 + A60/A61/A62/A63/A64/A70/A72/A73；(2) D2 Materialize + WorkItemPrivate partition；(3) ScopeContract + spawn gate + Observe R-OBS mapping；(4) ChildDownlink + cohort CG2′；(5) Upstream BlockedBy + PeerStatus；(6) depth≥1 默认 Materialize（无 feature flag）；(7) `/task context show` + ResolveHint 扩展；(8) Phase 3 T33–T35 登记 |
| **4.13.0** | **2026-06-27** | **devrix-d7-workitem-rollup-pipeline (DM-20260627-001) Phase 1 Rollup 闭环**：(1) ADDED Scenario D7-S15 + A50/A51/A53/A54/A55/A60/A61；(2) Parent Rollup Gate + `NeedsRollup` + `ReopenForRollup`；(3) Summary + Structured dual bubble Observe (`observationsFromChildSummaryBubbles`)；(4) Rollup MUPS R2+ + `verifyRollupArtifact`；(5) Root Fallback + session deliverable；(6) Phase 1 `RollupGatePolicy=best_effort` only；(7) t-registry v4.7.0→v4.8.0 (+21 P0 T) |
| 1.0.0 | 2026-06-10 | ORCH v2 read model spec (DM-20260610-012) |
| 1.0.0 | 2026-06-13 | D7 domain spec draft (DM-20260613-001, S3 design) |
| 2.0.0 | 2026-06-14 | 对齐最新代码：实现状态标注、DSAFT 结构、T 层映射、配置同步 |
| 2.1.0 | 2026-06-14 | Review R1 澄清写入 spec 摘要，指向 demand.md / d7-domain.md |
| 2.2.0 | 2026-06-14 | Review R2：D7-D1 权力分配、HandleInterrupt 顺序、OQ 闭合 |
| 2.3.0 | 2026-06-15 | **v1.0 + v1.1 闭环**：(1) S2 Turn Leader (DM-020) + Meta-Orchestrator 标注；(2) S1 State Authority 标注；(3) DSAFT 结构 + Scenarios 表 5/5 S 层 IMPLEMENTED；(4) Architecture 图更新至 D7-S2 主入口；(5) D7-S1 WorkModel Requirement 状态刷新（Partial → IMPLEMENTED）；(6) PLANNED Requirements 表全 ✅ |
| **2.4.0** | **2026-06-15** | **DM-020 D2→D3 拆面闭合**：(1) `shared/contracts/llm_facade.go` 新增 `LLMCaller` + `Summarizer` 拆面契约；(2) `turn.QueryLLMCaller` + `turn.CompressionSummarizer` 实现并由 `bootstrap/context_engine.go` 单一注入点 wired 至 `EngineDeps.QueryLLMCaller` / `EngineDeps.Summarizer`；(3) D2 production wiring 零 D3 import；(4) Cross-Domain Contracts 表新增两行 DM-020 拆面 IMPLEMENTED |
| **2.5.0** | **2026-06-15** | **DM-20260615-004 D7 Intent 路径正交化**：(1) `coordinator.command_handler.go` 新增（IntentCommand 显式分发到 PlanCLI/CLICommands，零 LLM 成本）；(2) `coordinator.orchestrate_path.go` 新增（IntentOrchestrate 显式调 `TaskDecomposer.SynthesizeTaskGraph` + `WaveScheduler.Start` + `WaitForCompletion`）；(3) `coordinator.orchestrator.go::ProcessMessage` switch 4 case 改为 4 独立执行链，删除 v1.0 `handleCommand` / `orchestrate` 占位实现；(4) Architecture 图更新至 v1.1.0+ orthogonal 形态 |
| **2.7.0** | **2026-06-15** | **D7 Real-Closure Spec Sync**：(1) 实现状态表 4 cell 更新（D7-S1 WorkModel、D7-S5 PlanMode、D7-S2-A06 RunTurnLoop、D7-S2-A07 InvokeLLM 全部 IMPLEMENTED）；(2) 域边界移除 "Task 写模型（暂在 D2）"；(3) D2 Loop 最终状态 sync（loop.go ≤200 行，LoopHooks 已删除）|
| **2.9.0** | **2026-06-15** | **D2 Loop 瘦身闭环**：(1) `query/loop.go` 239 行→170 行（符合 ≤200 行目标）；(2) `LoopHooks` 结构体删除，4 个编排字段迁出（`PlanMode`/`TaskManager`/`Orchestration`/`Hub`）；(3) D7-D4-T01 / D7-THIN-T01/T02 T 点闭环 |
| **2.10.0** | **2026-06-15** | **D7-S5 LLM Decomposer 闭环**：(1) `coordinator/llm_decomposer.go` 新增（LLM 增强任务合成，JSON DAG → wave.TaskNode）；(2) `coordinator/llm_decomposer_test.go` 7 T sub-cases（happy/bad JSON/enum coercion/unknown deps/extractJSON/nil LLM/routing）；(3) `WithLLMDecomposer` option wired into SessionOrchestrator |
| **3.0.0** | **2026-06-16** | **v1.2 + v2.0-b/c/f 全部闭环**：(1) D7-S1-T08 Task 状态机守卫；(2) D7-S5-A01-T01 置信度阈值；(3) D7-S2-A06/A07 Turn Leader；(4) t-registry 66/66 IMPLEMENTED |
| **3.1.0** | **2026-06-16** | 薄 `d7-domain.md` + `terminal-state-guide.md`；澄清迁至 `d7-requirements-clarifications.md`；域边界 LLM 产权修正 |
| **3.4.0** | **2026-06-16** | **devrix-d7-loop-first-routing (DM-20260616-002)**：(1) `routing_mode=loop_first` 默认 ingress → Turn；(2) `delegate_wave` / `enter_plan_mode` tool 门控 Wave/Plan；(3) EngineEvent 单投递路径；(4) `rule_orchestrate` 回滚；(5) L5-01..06 登记 |
| **3.3.0** | **2026-06-16** | **devrix-d7-uncertainty-gaps (DM-20260616-001) 归档**：(1) PlanAgent 运行时门控 Gherkin scenarios（4 T 点）；(2) PlanMode LLM 守卫（2 T 点）；(3) ConflictGuard 原子 Allow+Register（4 T 点）；(4) OrchestratePath FlowEvent sink 恢复（2 T 点）；(5) PlanModeApproveGate 死配置移除（2 T 点）；(6) 死代码 Deprecated 标记（2 T 点） |
| **3.2.0** | **2026-06-16** | `observability-guide.md`；`dsaft-architecture.md` Stub；Guides 索引 |
| **3.5.0** | **2026-06-17** | **devrix-queryloop-legacy-decommission (DM-20260617-001)**：(1) ADDED Requirements：D2 QueryLoop Legacy Path Decommission（loopFirst=true 主路径护栏 + 拆面 adapter 零调用 + legacy metric 暴露 + CLI 警告 + D2-S10 spec.md LEGACY 标记）；(2) 6 个 Gherkin Scenario 覆盖 AC1-AC7；(3) T09/T10 + T04/T05 注册 |
| **3.6.0** | **2026-06-17** | **devrix-tool-surface-phase2-full (DM-20260617-008) 工具调用链路登记**：(1) Cross-Domain Contracts 表新增 DM-20260617-008 行（指 D2 spec §"Tool Call End-to-End Flow" 为完整链路 SoT）；(2) 端到端 Chain A/B/C 视图（3 链 7 surface 5 domain 拓扑）由 D2 spec 持有, D7 通过本表反查 |
| **3.7.0** | **2026-06-17** | **devrix-unified-work-tree (DM-20260617-009)**：(1) ADDED WorkItem + WorkTree 统一工作语义；(2) WorkTree ⊥ RunRegistry 分离（`run_ref` 外键）；(3) todo_write→checklist ephemeral 子节点 + sc.Todos 投影；(4) Wave OrchestratePath SyncWaveNodes；(5) legacy TaskManager 适配器；(6) 跨 session 只读查询 baseline |
| **3.8.0** | **2026-06-18** | **devrix-unified-work-tree v1.5–v2.0 闭环 (PR #85–#87)**：(1) 统一工具 alias task_write/spawn/await；(2) RunTurn decompose + ResolveHint + depth/daily limits；(3) RunTurn blocking await (`ResolveAwaiter`)；(4) v2.1+ defer → `openspec/tech-debt/worktree-v2-deferred.md` |
| **3.9.0** | **2026-06-19** | **devrix-d7-v2-structure (DM-20260619-005)**：(1) S 层物理路径对齐 `code-layout.md` §4.2；(2) coordinator→sessionorchestrator+decisionplanning+orchtypes；(3) wave→wavescheduler、S4→executionflow；(4) hubspoke dispatch/bridge 拆分；(5) WorkTree TD-WT-02/03 部分闭合 |
| **4.0.0** | **2026-06-21** | **devrix-d7-error-aggregation-and-metrics (DM-20260621-010)**：(1) D7-S6 新增 Scenario「Error Aggregation & Metrics」覆盖 HandleInterrupt errors.Join + sandbox cleanup observability + Forker errors.Join；(2) ADDED Requirements：D7-S6-A11 (interrupt errors.Join 3 步 cancel 聚合) + D7-S6-A12 (sandbox cleanup observability freefork+execute+taskmanager) + D7-S6-A13 (forker errors.Join + 13 调用方 backward compat)；(3) Scenarios 表新增 D7-S6 行；(4) Archived Changes 表新增 DM-20260621-010 引用 |
| **4.1.0** | **2026-06-23** | **devrix-d7-mups-v4-phase3-execute (DM-20260625-001) Phase 3 PR-C1**：(1) ADDED Requirement D7-S9-A25 (Execute Artifact Data Contract): ArtifactKind 4 类枚举 + SideEffectStatus 5 态 + SideEffectDetail + wavescheduler.Artifact +5 字段 (Kind/SourcePlanID/AnomaliesCount/SideEffectStatus/SideEffectDetail) omitempty 向后兼容；(2) 跨域类型上提 shared/types/execute.go 打破 orchtypes→workmodel→wavescheduler 潜在 import cycle（Phase 1 MemoryEntry precedent）；(3) 4 个新 P0 T 点 D7-S9-A25-T01..T04 IMPLEMENTED；(4) t-registry v3.8.0→v3.9.0 (T 129→133, P0 96→100)；(5) Archived Changes 新增 DM-20260625-001 引用 |
| **4.2.0** | **2026-06-23** | **devrix-d7-mups-v4-phase2-observe-plan (DM-20260623-001) Phase 2 PR-A1 + PR-RF**：(1) ADDED Requirement D7-S8-A15 (Observation + UncertaintyReport + UncertaintyCoord): Observation 4 类 ObservationKind × 2 Category + sealed Payload interface + UncertaintyReport Partition 不变式 + ComputeOverallStrength 仅遍历 CatBusiness + UncertaintyCoord Phase 2 扩展 (FromVerifier/IsColdStart/Equal/With*) + 11 SentinelError + 4 错误码 (7001-7004)；(2) PR-RF 5 项 review fix 闭环（C1 IntentKind enum + C3 FromVerifier fail-fast + W2 fmt.Errorf wrap + W3 clamp01Float 合并 + W6/I8 Partition clamp 末尾）；(3) 6 个新 P0 T 点 D7-S8-A15-T01..T06 IMPLEMENTED；(4) t-registry v3.9.0→v3.10.0 (T 133→139, P0 100→106)；(5) Scenarios 表新增 D7-S8 行（Observation + UncertaintyReport PR-A1 + PR-RF, A15 模块）；(6) Archived Changes 新增 DM-20260623-001 引用 |
| **4.5.0** | **2026-06-23** | **devrix-d7-mups-v4-phase5-learn (DM-20260623-003) Phase 5 Learn 节点升格**：(1) ADDED Requirement D7-S11-A36 (LearningAsset 5 类 + AssetContent + LearningClass 5 枚举): LearningClass typed enum 5 态 + String/Parse/Marshal/Unmarshal + 空字符串零值 LearningSOP 兼容 + LearningAsset struct 15 字段不可变 + 自动时间戳 + deep copy + 5 类 AssetContent (SOPAssetContent ★5 / ProtocolAssetContent ★4 / KnowledgeAssetContent ★3 / ConclusionAssetContent ★2 / PendingAssetContent ⭐★1 含 MVEState) + Validate() + SchemaVersion() + ByteSize()；(2) ADDED Requirement D7-S11-A37 (ReputationEvidence + Bayesian Update + Wilson Score + G8-1 修复): ReputationEvidence struct 12 字段 + BayesianUpdate 不可变 + Pass/Partial/Fail → α/β++ + INDETERMINATE "verifier_parse_failure" G8-1 修复仅 VerifierFailureCount++ 不污染 α/β + Wilson Score 95% 置信区间 + 冷启动除零防御；(3) ADDED Requirement D7-S11-A38 (AdaptivePrior + DefaultPriors + BuildAdaptivePrior): AdaptivePrior + BetaPrior (String "Beta(α,β)") + InjectTarget 3 枚举 + DefaultDeveloperPrior Beta(5,3) + DefaultOperatorPrior Beta(8,1) + BuildAdaptivePrior Bayesian 合并 + rep==nil 兜底；(4) ADDED Requirement D7-S11-A39 (Memory 3 通道接口 + 3 实现 LP-2 隔离): Memory interface 4 方法 + MemoryChannel 3 枚举 + MemoryFilter 4 字段 + SkillMemory (SOP/Protocol) + FeedbackMemory (Knowledge/Conclusion) + ScheduledMemory (Pending) + ScheduledRetry envelope + IsExhausted() + ListDue() + ErrAssetClassMismatch fail-fast + sync.RWMutex 并发安全；(5) ADDED Requirement D7-S11-A40 (Learner interface + DefaultLearner + LP-1 闭环 T13 PARTIAL): Learner 3 方法 (Learn/Inject/ScheduledTick) + LearnRequest 5 字段 + DefaultLearner 6 字段 + Learn 5 步流程 + Inject LP-1 闭环入口 + ScheduledTick 重试调度 + AssetBuilder 5 类 Content 构造 + ReputationStore interface + InMemoryReputationStore + 4 Verdict → 5 LearningClass 路由；(6) 12 个新 P0 T 点 D7-S11-A36-T01/T02/T03 + A37-T04/T05 + A38-T06/T07 + A39-T08/T09 + A40-T10/T11/T12 IMPLEMENTED + T13 PARTIAL (Observe 跨域 wiring 留待 Phase 6 集成)；(7) t-registry v3.12.0→v3.13.0 (T 155→168, P0 122→135)；(8) Scenarios 表新增 D7-S11 行（Learn 节点 5 类资产 + 3 通道 + LP-1 闭环）；(9) Archived Changes 新增 DM-20260623-003 引用 |
| **4.6.0** | **2026-06-24** | **devrix-d7-mups-v4-phase6-observe-learner-wiring (DM-20260624-001) Phase 6 Observe-Learner 跨域闭环集成**：(1) ADDED Requirement D7-S12-A41 (ObserveRequest + IntentQuantizer + AnomalyDetector + ClassifyWithPrior): ObserveRequest struct 3 字段 (SessionID/Message/Prior) + NewObserveRequest fail-fast + EffectivePrior 兜底 DefaultDeveloperPrior + Validate + IntentQuantizer 4 IntentClass (Fact/Command/Orchestrate/Skip) + IntentPayload + Quantize baseline + QuantizeWithPrior (prior.PriorBeta.Mean() 作为 confidence 乘数, clamp [0,100]) + AnomalyDetector + Anomaly + AnomalyReport + HistoricalDetector.Detect baseline + HistoricalDetector.DetectWithPrior (threshold = 0.5 × Mean, Mean 越高阈值越高 = 更信任用户 = 更易放过) + RuleClassifier.ClassifyWithPrior + IntentClassifier 接口扩展 ClassifyWithPrior + ShadowClassifier.ClassifyWithPrior 委托给 rule + AdaptivePrior.BetaPrior.Mean() 方法；(2) ADDED Requirement D7-S12-A42 (SessionOrchestrator.buildObserveRequest + WithLearner + 3 层 fail-safe): SessionOrchestrator 新增 `learner learn.Learner` 字段 + `WithLearner(l learn.Learner) OrchestratorOption` + `buildObserveRequest(ctx, req)` 方法调用 Learner.Inject 拿 AdaptivePrior + 3 层 fail-safe (nil learner / Inject error / 正常 → 全部 DefaultDeveloperPrior Beta(5,3) 兜底) + ProcessMessage 在 classifySpan 之前调用 buildObserveRequest + ClassifyWithPrior 替换 Classify + sessionSpan.SetAttributes("learn.prior.alpha" + "learn.prior.beta") 记录 prior 到 D5 span；(3) ADDED Requirement D7-S12-A43 (LP-1 闭环 E2E 集成测试): 4 E2E 测试 (TestE2E_LP1_ClosedLoop_LearnPassAccumulatePrior / TestE2E_LP1_ClosedLoop_IndeterminateParseFailure_NoAlphaPollution / TestE2E_LP1_ClosedLoop_PendingAssetScheduledMemory / TestE2E_5NodePipeline_End2End) + in-memory mocks (InMemoryReputationStore + 3 Memory 通道 + recordingExecutor + recordingClassifier) + LP-1 闭环 (Learn × 3 Pass → Alpha=3 → Round 2 观察 Beta(8,3)) + G8-1 修复闭环 (parse_failure 不污染 α/β) + LP-2 隔离 (PendingAsset 仅在 ScheduledMemory) + LP-5 反向追溯；(4) 6 个新 P0 T 点 D7-S12-A41-T01/T02/T03 + A42-T04/T05 + A43-T06 IMPLEMENTED；(5) t-registry v3.13.0→v3.14.0 (T 168→174, P0 135→141)；(6) Scenarios 表新增 D7-S12 行（Observe-Learner 跨域闭环集成）；(7) Archived Changes 新增 DM-20260624-001 引用；(8) 闭环 Phase 5 PR-E5 T13 PARTIAL（Observe 跨域 wiring 全部完成） |
| **4.7.0** | **2026-06-24** | **devrix-d7-mups-v4-phase7-verify-auto-close (DM-20260625-001) Phase 7 运行时 5 节点闭环**：(1) ADDED Requirement D7-S13-A47 (SessionOrchestrator.processAutoClose — Verify→Learn 运行时闭环): processAutoClose 包装 channel + 异步触发 learner.Learn + 替换 endSpanWhenChannelClosed 调用 + synthesizeVerdict 4 规则 (complete→VerdictPass / error→VerdictFail Reason=Content / tombstone→VerdictIndeterminate IndeterminateReason="interrupt" / 其他 Type→nil) + SourceID `autoclose:{sessionID}:{nanosecond}` + 3 层 fail-safe (nil learner / Learn error / channel cancel → 全部 log + skip 不阻塞 caller) + IntentSkip 路径不调用 processAutoClose + AssetBuilder Auto-Close fallback (sop:autoclose:<SourceID> + ["autoclose-completion"] 合成步骤) 保证 LP-1 闭环在生产 wiring 真实可走通；(2) ADDED Requirement D7-S13-A48 (ProcessRequest.TrackMode — Operator 角色支持 + buildObserveRequest 透传): ProcessRequest 新增 TrackMode string 字段 + TrackModeDeveloper/Operator 常量 + NewProcessRequest fail-fast 校验 + 3 sentinel error (ErrProcessRequestSessionIDEmpty / MessageEmpty / InvalidTrackMode) + DefaultLearner.Inject 3-tier 解析策略 (Reputation 持久状态 > req.TrackMode hint > Developer 兜底 + slog.Warn 未知值) + buildObserveRequest 透传 TrackMode → Operator track → DefaultOperatorPrior Beta(8,1) Mean=0.889；(3) ADDED Requirement D7-S13-A49 (sessionSpan 6 prior attributes — D5 可观测化增强): priorSessionSpanAttrs 纯 helper 函数 + sessionSpan 6 attributes (learn.prior.alpha / beta / mean / track_mode / classifier_source / injected_at) + injected_at "phase6_lp1" (真实注入) vs "cold_start_failsafe" (兜底) + learn.classifier_source mirror (rule/shadow) + 30+ 单测/集成测试 100% PASS；(4) 6 个新 P0 T 点 D7-S13-A47-T01/T02/T03 + A48-T04/T05 + A49-T06 IMPLEMENTED；(5) t-registry v3.14.0→v3.15.0 (T 174→180, P0 141→147)；(6) Scenarios 表新增 D7-S13 行（运行时 5 节点闭环 Verify→Learn Auto-Close + Operator TrackMode + D5 可观测化增强）；(7) Archived Changes 表新增 DM-20260625-001 引用；(8) 闭环 Phase 6 PR-F3 E2E 测试中手工模拟的 Verify→Learn 步骤, 生产 wiring 真实可走通 LP-1 闭环 |
| **4.8.0** | **2026-06-25** | **devrix-d7-mups-v5-escape-engine (DM-20260625-003) MUPS v5 统一逃逸机制**：(1) PLANNED Requirement D7-S14 (5 PR 拆分 PR-V5.1..V5.5): LoopDepthTracker v2 + PlanKindSwitchPolicy + EscapeAction 6 类 + ChainedArbitrator + EscapeEngine + CircuitBreaker 5 层 + AuditLog + 5 节点 EscapeEngine 接线点 + T2 ResumeSession 续跑 + 13 类失败降级矩阵；(2) Scenarios 表新增 D7-S14 行（MUPS v5 统一逃逸机制 PLANNED）；(3) Archived Changes 表新增 DM-20260625-003 引用；(4) t-registry v3.15.0→v3.16.0（PLANNED 18 P0 T 点：LoopDepthTracker 7 + PlanKindSwitchPolicy 4 + ChainedArbitrator 4 + EscapeEngine 2 + AuditLog 1）；(5) review-r3.md 6 ISSUE 已修复（ISSUE-1 MaxDepth 边界 / ISSUE-2 ChainedArbitrator 骨架 / ISSUE-3 applyResumeDecision 骨架 / ISSUE-4 Notifier 清理 / ISSUE-5 Observe 失败 Continue / ISSUE-6 L2-07 表驱动）；(6) 121 个测试用例设计（L4 4 + L3 7 + L2 7 + L1 103），覆盖率 85%→97% |
| **4.9.0** | **2026-06-25** | **devrix-d7-mups-v5-escape-engine IMPLEMENTED (DM-20260625-003, PR-V5.1..V5.5 全部落地)**：(1) IMPLEMENTED 5 个 PR 全部 squash merge: V5.1 LoopDepthTracker v2 (commit 0f7243a, 15 T) + V5.2 PlanKindSwitchPolicy (a862892, 18 T) + V5.3 ChainedArbitrator LLM/Rule/Human 3 层 + Notifier + PendingResolution (69844e3, 33 T) + V5.4 EscapeEngine + CircuitBreaker 5 层 (2382207, 19 T) + V5.5 5 节点接线 (Orchestrator 1a/1b/2 + unit + integration 8 T)；(2) Engine 决策合并逻辑: 0 信号 → Continue / 1 信号 → 直接返回 (硬信号优先级) / 2+ 信号 → ChainedArbitrator 仲裁；(3) ADDED Section "MUPS v5 EscapeEngine" 详细 5 节点 + 4 类深度限制 + 6 类 EscapeAction + 5 层 CircuitBreaker + 5 节点接线点；(4) SessionOrchestrator 新增 WithEscapeEngine option + escapeEngine 字段 + 4 接线点（1a Plan 失败 / 1b Plan 前 / 2 Execute 失败 / 3 Verify 失败 — 接线点 3 待 processAutoClose 暴露 verdict 后落地）；(5) integration_test.go 5 个集成测试 (4DepthLimits + 3LayerArbitration + 5EscapeActions + PlanKindSwitchLimit + 5NodePipeline_End2End) 100% PASS；(6) 22/22 orchestration 包 go test -race 100% PASS（pre-existing TestAutoClose_FullLP1Loop 1s timeout 不影响 V5.5）；(7) t-registry v3.16.0→v3.17.0 (D7-S14 18 PLANNED → 17 IMPLEMENTED + 1 PARTIAL：T12 ResumeSession T2 续跑 SessionOrchestrator 入口 PR-V5.6 单独推进，V5.5 仅完成 V5.3 HumanArbitrator.ResumeSession 落地)；(8) S4-Gate review 修复: C-1 processEscapeDecision 返回 augmented error 避免静默吞错 (signature: `bool` → `(bool, error)`);(9) Scenarios 表 D7-S14 行从 PLANNED → IMPLEMENTED 状态刷新（保留 T12 PARTIAL 子节点标记） |
| **4.10.0** | **2026-06-25** | **devrix-d7-mups-v5-escape-engine PR-V5.6 T2 ResumeSession 续跑 SessionOrchestrator 入口收口 (DM-20260625-003)**：(1) ADDED Section "ResumeSession T2 续跑入口" + Requirement D7-S14-A50-T12 落地 (T12 PARTIAL → IMPLEMENTED)：SessionOrchestrator.applyResumeSession(ctx, req, sessionSpan) 方法在 ProcessMessage 入口 (after buildObserveRequest, before classify) 检查 PendingResolutionStore → 调用 EscapeEngine.ResumeSession (one-shot consume via HumanArbitrator.LoadAndDelete) → 3 层 fail-safe (nil engine / ResumeSession error → 静默 fall through / TTL 过期 not found → 静默 fall through) → terminal decision (B=user_accept → EscapeForceExit, C=user_cancel → EscapeAbortWithAudit) emit single "complete" EngineEvent + 补写 audit + close channel early / A=user_continue fall through to full 5-node pipeline；(2) resumeContentForDecision helper: 将 6 类 EscapeAction 映射为用户可见的中文 text 消息（Continue/EscalateToRule/EscalateToHuman/ForceExit/AbortWithAudit/EscapePendingHuman）；(3) sessionSpan resume 3 attributes (escape.resume.attempted / escape.resume.decision_action / escape.resume.decision_pending_id) 用于 D5 可观测化追踪 resume 路径触发率；(4) 6 个单元测试 (TestApplyResumeSession_NoEngine / TestApplyResumeSession_NoPending / TestApplyResumeSession_UserAccept / TestApplyResumeSession_UserCancel / TestApplyResumeSession_UserContinue / TestApplyResumeSession_ResumeError_Failsafe) + 2 个集成测试 (TestProcessMessage_WithResume_UserAccept_EarlyClose / TestProcessMessage_WithResume_UserCancel_EarlyClose) 全 PASS；(5) t-registry v3.17.0→v3.18.0 (D7-S14 T12 PARTIAL→IMPLEMENTED，T 180→186，P0 147→153，D7-S14 18/18 IMPLEMENTED, 0 PARTIAL)；(6) Scenarios 表 D7-S14 行 T12 子节点从 PARTIAL → IMPLEMENTED 状态刷新 |
| **4.12.0** | **2026-06-26** | **devrix-d7-inner-spans-dedup-remove (DM-20260626-009) follow-up 内层 observability span 登记 (PR #253+#254 落地，follow-up PR #255 待开)**：(1) ADDED Requirement D7-S1-A52 (EmitWorktreeOp — ItemPipelineRunner 11 callsite 内层 span): 4 attribute (worktree.op / worktree.item_id / worktree.phase_or_status + session_id) + happy path + nil-bridge fail-safe + 2 P0 T 点 (D7-S1-A52-T11 happy + T12 nil-bridge)；(2) ADDED Requirement D7-S1-A53 (EmitSubWorktreeRun — RunParallelExplore parent→child 关系 span): 4 attribute (subworktree.parent_id / subworktree.child_id / subworktree.spawned_by + session_id) + happy + nil-bridge fail-safe + 2 P0 T 点 (T13+T14)；(3) ADDED Requirement D7-S5-A54 (EmitSubTurnIteration — WorkItemExecutor ReAct loop per-iter span): 5 attribute (subturn.item_id / subturn.iter / subturn.finish_reason / subturn.stop_reason + session_id) + finish_reason (LLM 真实) 与 stop_reason (executor 自定义) 正交 + cap-hit 多发 1 span (iter=max+1, finish_reason="tool_calls", stop_reason="max_iters") + stepOneIter 抽离让 span 包单次函数调用而不是 6 inline return path + 2 P0 T 点 (T15+T16)；(4) t-registry v4.6.0→v4.7.0 (T 222→228, P0 185→191, 6 P0 T 点 IMPLEMENTED)；(5) span-registry v4.0.0→v4.1.0 (新增 3 ops + WorkItem Inner Layer Trace 树)；(6) observability-guide v2.0.0→v2.1.0 (3 行 §1 绑定 + §2.6 Trace 树 + S1/S5 矩阵 + §7 3 行检查 + §8 关闭 2 项缺口)；(7) span-registry/observability-guide/t-registry/spec 跨文档同步 (4 spec doc 一致登记 3 新 span)；(8) Archived Changes 表新增 DM-20260626-009 引用 |
| **4.11.0** | **2026-06-25** | **devrix-d7-dead-files-cleanup (DM-20260625-013..016) PR #214 squash-merged 2026-06-24T22:37:02Z 理想态架构收口**：(1) **D7-S1 WorkItem 单一来源**：Task flat-view (`task_store.go`) 整层删除 + Task conversion helpers (`ToTask/FromTask`) 全删 + taskStoreAdapter 全删 → WorkItem 是唯一 canonical 模型, TaskManager 只是 `Tree()` facade；(2) **typed-enum 强一致**：`orchtypes/uncertainty_coord.go::FromVerifier` 字符串版删除, `FromVerifierTyped` typed enum 取代；ordinal ClassToStrength 公式 `(LearningClass) 6-class` 计算 ★ 等级；(3) **shim 全清零**：`coordinator/` `hubspoke/` type-alias shim 已并入 `sessionorchestrator/` `orchtypes/` 源包, type-alias 模式 0 残留；(4) **turn 子系统拆分**：`turn/exit_reason.go` (89 行) 从 `turn/orchestrator.go` 抽出, 14 ExitReason enum 独立成文件（8 deterministic + 6 verify-driven）+ `loop_legacy_test.go` 改名 `runturn_main_path_test.go`（去掉"legacy"误导）；(5) **DM 序列**: DM-20260625-007 (drop 3 orphan code paths) + DM-20260625-008 (drop milestone service + TaskController) + DM-20260625-009 (drop dead turn_adapter + unified_tools) + DM-20260625-010 (migrate coordinator.* + hubspoke.*) + DM-20260625-011 (retire hubspoke/ + 4 dead code paths) + DM-20260625-012 (split runLoop god function into 7 helpers D3) + DM-20260625-013 (DI-migrate process singletons + drop dead code) + DM-20260625-014 (typed enum + ordinal ClassToStrength + v1.0 history cleanup) + DM-20260625-015 (delete Task flat-view + TaskStore, collapse to WorkTree 19 files) + DM-20260625-016 (split ExitReason enum + rename loop_legacy_test)；(6) **3 commits squash-merged**: `5c09aef` (typed enum) + `3f29b5a` (Task→WorkItem 19 files) + `7d32f4b` (ExitReason 抽出 + test rename)；(7) **净行数变化**: 24 files, +498/-751 = **-253 行**；(8) **验证**: 22/22 orchestration packages `go test -race` PASS, `go vet ./...` clean, `scripts/verify-archive.sh` 12/12 PASS, **0 OPEN PR**；(9) **理想态蓝图**: 详见 `openspec/specs/d7-orchestration/pipeline-architecture.md` v1.1.0 §7 清理变更日志 + §8 验证清单；(10) **Scenarios 表 D7-S1 行更新**: 描述从"Task/Plan 数据模型与生命周期"改为"WorkItem 数据模型与生命周期（v4.3 post-cleanup, Task flat-view 已删除）", 代码位置从 `workmodel/` 改为 `workmodel/work_tree.go` + `workmodel/workitem.go` + `sessionorchestrator/workmodel.go` |

---

## Unified Work Tree (DM-20260617-009)

> **Archived:** `openspec/archive/2026-06-17-devrix-unified-work-tree/`

### Requirement: WorkItem Unified Work Unit Model

The orchestration layer SHALL represent all work semantics as `WorkItem` nodes in a per-session tree owned by D7 `WorkTree`.

Each WorkItem MUST have: `id`, optional `parent_id`, `kind`, `status`, `title`, `directive`, optional `uncertainty`, `policy`, dependency edges, optional `run_ref`, and `ephemeral` flag.

#### Scenario: Session root goal creation
- GIVEN a new session with no work items
- WHEN the first user message is processed
- THEN exactly one `kind=goal` root WorkItem exists with the user directive as `directive`

#### Scenario: Child work item under goal
- GIVEN an existing session goal WorkItem
- WHEN `delegate_implement` is invoked without an explicit work item id
- THEN a new `kind=implement` WorkItem is created with `parent_id` set to the goal id

### Requirement: WorkTree and RunRegistry Separation

Work semantics (What) and execution handles (How) MUST remain separate stores. `WorkItem.run_ref` links to RunRegistry entries; terminal callbacks update WorkItem status and bubble parent re-evaluation.

### Requirement: Legacy TaskManager Compatibility Adapter ✅ OBSOLETE（v4.3 post-cleanup 已删）

> **v4.3 post-cleanup（PR #214, 2026-06-24）**：Legacy TaskManager adapter + `task_create`/`task_get`/`task_list`/`/task` CLI flat `Task` API **全部删除**。TaskManager 现仅是 `WorkTree` 的 facade,所有调用走 `Tree()` → WorkItem 单一模型。如有外部依赖 flat `Task` API 的 caller,需迁移到 `WorkItem` 接口（`Create/Get/List/UpdateStatus`）。

`TaskManager` 历史上 delegates to `WorkTree` internally while preserving flat `Task` API for `task_create`, `task_get`, `task_list`, and `/task` CLI commands — 上述 contract 已废弃。

### Requirement: Wave Scheduler Reads WorkTree (v1.1)

`OrchestratePath` SHALL call `TaskManager.SyncWaveNodes` after `SynthesizeTaskGraph`, writing implement subtrees before Wave dispatch.

### Deprecated (v2.0 target → v2.1 tech-debt)

> 详见 `openspec/tech-debt/worktree-v2-deferred.md` (TD-WT-02, TD-WT-03)

- Session-scratch `sc.Todos` as authoritative checklist (demoted to read projection via WorkTree)
- Independent persistent wave task graph as work semantics SoT

### Requirement: RunTurn Resolve and Decompose (v1.5–v2.0)

`DefaultOrchestrator.RunTurn` SHALL inject focus context via `FocusHintProvider`, MAY block-await running focus children via `ResolveAwaiter`, and SHALL guide decompose via `ResolveHint` when uncertainty exceeds threshold.

#### Scenario: Blocking await before LLM loop
- GIVEN a focus WorkItem with in-progress children that have `run_ref`
- WHEN RunTurn starts
- THEN `ResolveAwaiter` blocks until children reach terminal or timeout
- AND a `resolve` engine event is emitted with await summary

#### Scenario: High uncertainty decompose guidance
- GIVEN a focus WorkItem with uncertainty ≥ threshold and decomposable kind
- WHEN RunTurn injects focus hint
- THEN ResolveHint advises `task_write mode=decompose`
- AND `DecomposeChildren` enforces max depth, max children, and daily limit
