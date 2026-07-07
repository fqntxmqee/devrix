# Tasks: D7 multi-intent observation decompose + Plan DAG + parallel Execute

**Change ID:** `devrix-d7-multi-intent-observation-decompose`
**Total Tasks:** 59 (T01-T52 = 47 base + T60-T65 = 6 Learn 22-scenario + T66-T72 = 7 Plan 26-scenario,minus 1 deprecated = 60 - 1 = 59)
**Sprint:** devrix-d7-v7
**PRs:** 7 (PR-A1 grammar → PR-A2 AC contract + LLM IO → PR-B WaveScheduler+Decision+Sub-Worker → PR-C streaming+idempotency → PR-D config+e2e+verify-archive → PR-E Learn+22-scenario → PR-F Plan+26-scenario)

---

## P0-1 IntentSegment grammar + SpawnPolicy 扩展

**File:** `internal/layers/orchestration/orchtypes/intent_segment.go` + `internal/layers/orchestration/plan/spawn_policy.go`
**Effort:** 1.5 人天
**AC:** AC1-AC3 (新增)

| ID | Description | Status |
|---|---|---|
| T01 | 新增 `IntentSegment {ID, Text, IntentKind, Priority, Confidence}` 类型 + JSON schema | TODO |
| T02 | 新增 `IntentSegmentSet {Segments []IntentSegment, SourceDirective, DetectedAt}` 容器 | TODO |
| T03 | 扩展 `plan.SpawnPolicy` 加 `DecomposeByIntentSegments IntentSegmentSet` 变体 | TODO |
| T04 | `NewIntentSegment(orchtypes.IntentSegment)` + `Validate(...)` 含 (kind ∈ {Deterministic, Explore, Commit, Analyze}) + Priority ∈ [0, 100] | TODO |

## P0-2 IntentSegmenter (Observe 节点 LLM 切分)

**File:** `internal/layers/orchestration/sessionorchestrator/intent_segmenter.go`
**Effort:** 2.0 人天
**AC:** AC4-AC6

| ID | Description | Status |
|---|---|---|
| T05 | `IntentSegmenter` interface { `Segment(ctx, observeReq) (IntentSegmentSet, error)` } | TODO |
| T06 | `LLMIntentSegmenter` 用现有 D2 contextengine + D3 llmgateway 调 LLM,prompt appendix 含 6-shot example | TODO |
| T07 | `RuleBasedSegmenter` 兜底:regex 拆 "X + Y" / "X?另外 Y" / "X,Y" 等中文连接词 | TODO |
| T08 | `SegmenterDispatcher` 先 LLM(超时 800ms)→ fallback RuleBased | TODO |
| T09 | 单意图 directive(无连接词)切出 1 个 segment,confidence ≥ 0.95 走 fast-path | TODO |

## P0-3 PlanDAG grammar + DAG validator

**File:** `internal/layers/orchestration/plan/plan_dag.go` + `plan/dag_validator.go`
**Effort:** 1.5 人天
**AC:** AC7-AC10

| ID | Description | Status |
|---|---|---|
| T10 | `PlanNode {ID, SegmentID, WorkerHint, ExpectedArtifactTags}` 类型 | TODO |
| T11 | `DataEdge {From, To, DependsOnOutputs []string}` 类型(v1 字段保留,使用为空) | TODO |
| T12 | `PlanDAG {Nodes []PlanNode, Edges []DataEdge, Priorities map[string]int, MaxParallelism int}` | TODO |
| T13 | `validateDAG(plan PlanDAG, opts) error`:DFS 检测环 + 节点数 ≤ MaxFanOut + node-id 唯一性 + edge 端点存在 | TODO |
| T14 | `ValidateError` 含 Cycle / TooManyNodes / DuplicateNode / DanglingEdge 4 种子类型,触发 PlanParseReject | TODO |

## P0-4 Plan LLM proposer 扩展产 PlanDAG

**File:** `internal/layers/orchestration/plan/strategic_plan_proposer.go`
**Effort:** 1.5 人天
**AC:** AC11-AC12

| ID | Description | Status |
|---|---|---|
| T15 | StrategicPlanProposal 加 `DAG *PlanDAG` 字段(可选,与现有 `Children` 并存,DAG 为空时退回 Children) | TODO |
| T16 | Plan prompt appendix 加 DAG JSON schema + 3-shot example(2-segment / 3-segment / deterministic+explore) | TODO |
| T17 | 解析时调 validateDAG();失败 → PlanParseReject(CompactJSON 错误反馈) | TODO |

## P0-3.5 Plan ↔ Verify 验收契约(AcceptanceCriteria + PerCriterion)

**File:** `internal/layers/orchestration/plan/acceptance_criteria.go` (NEW) + `internal/layers/orchestration/executionflow/verify/per_criterion.go` (NEW)
**Effort:** 2.0 人天
**AC:** AC25, AC27-AC28, AC11.1-AC11.3

| ID | Description | Status |
|---|---|---|
| T31 | `AcceptanceCriterion` type(ID, Description, CheckKind, CheckArgs, Severity, Rationale) + Validate() | TODO |
| T32 | `PerCriterionVerdict` type(CriterionID, Outcome, Evidence, Error) + aggregate() 4 路径 | TODO |
| T33 | `PlanNode.AcceptanceCriteria []AcceptanceCriterion` 字段(扩 §3.5) | TODO |
| T34 | `PlanAcceptanceContractBuilder.Build(dag, ac)` 一致性校验(NodeCoverageMissing / DuplicateCriterionID / ∀ Node ≥ 1 Required) | TODO |
| T35 | Verify 节点接收 AC[] + Artifact → PerCriterionVerdict[];聚合 VerdictKind 落 round.Metadata["ac_verdicts"] | TODO |
| T36 | 机械 CheckKind(ContainsString/NotContains/NumericRange/LengthRange/MentionsAll/MentionsAny/JSONPath)本地执行,LLM 调用为 0 | TODO |
| T37 | CustomLLMJudge CheckKind 走 D3 llm 调用,judge_prompt 含 plan_rationale + criterion.Description + artifact | TODO |
| T38 | Verify short-circuit:0 CustomLLMJudge 时 Verify 不调 LLM,延迟 < 50ms | TODO |

## P0-4.5 PlanLLMIO + VerifyLLMIO 完整协议

**File:** `internal/layers/contextengine/i18n/format_hints_mups.go` (MODIFIED,Plan/Verify appendix) + `internal/layers/orchestration/plan/llm_io.go` (NEW) + `internal/layers/orchestration/executionflow/verify/llm_io.go` (NEW)
**Effort:** 2.0 人天
**AC:** AC9, AC12, AC26-AC27, AC10 (feedback loop)

| ID | Description | Status |
|---|---|---|
| T39 | `PlanLLMInput` JSON schema 在 i18n `StrategicPlanAppendix` 强制声明(版本 v1) | TODO |
| T40 | `PlanLLMOutput` JSON schema + LLM 调用模板(3-shot example:全 deterministic / 全 uncertain / 混合;每个 example 必须含 AC[]) | TODO |
| T41 | PlanParseReject feedback loop 3 个子类(RejectCycle / RejectTooManyNodes / RejectACDuplicateID)格式化模板 | TODO |
| T42 | `VerifyLLMInput` JSON schema + i18n `VerifyAcceptanceAppendix`(ZH + EN,2-shot example) | TODO |
| T43 | `VerifyLLMOutput` 校验器(per_criterion_verdicts 顺序对齐 criteria,overall_verdict aggregation 合法) | TODO |
| T44 | CustomLLMJudge budget 控制:Verify 输入中 CustomLLMJudge 数量 > 3 → 截断 + log warning | TODO |
| T45 | ErrPlanLLMIOBudgetExceeded / ErrVerifyLLMJudgeBudgetExceeded 错误码注册 | TODO |

## P0-5 WaveScheduler DAG executor + 4 worker pool

**File:** `internal/layers/orchestration/wavescheduler/dag_executor.go`
**Effort:** 3.0 人天
**AC:** AC13-AC16

| ID | Description | Status |
|---|---|---|
| T18 | `DAGExecutor` interface { `RunPlanDAG(ctx, plan PlanDAG) <-chan SegmentEmit, error` } | TODO |
| T19 | `RunPlanDAG` 内部用 goroutine pool:4 worker,channel queue,readySet 用 priority heap | TODO |
| T20 | initialReadySet = 节点集 - {有入边的节点};完成时新 ReadySet = (节点集 - 已完成) ∩ {所有前置已完成的节点} | TODO |
| T21 | error propagation:任一 child error → 取消未启动 sibling + Drain emit | TODO |

## P0-6 ItemPipelineRunner.Run() 接入 + Stream emit

**File:** `internal/layers/orchestration/sessionorchestrator/item_pipeline.go`
**Effort:** 2.0 人天
**AC:** AC17-AC19

| ID | Description | Status |
|---|---|---|
| T22 | Run() 检测 StrategicPlanProposal.DAG != nil → 转发 DAGExecutor.RunPlanDAG | TODO |
| T23 | 每个 child 完成 → 立刻调 opts.Emit 发 partial EngineEvent + IdempotencyKey=(session_id, segment_id) | TODO |
| T24 | parent rollup 完成 → emit 最终 EngineEvent,覆盖上一 partial 卡片(需 IM adapter 支持 update) | TODO |

## P0-7 Learn per-segment + ParentEvidence 聚合

**File:** `internal/layers/orchestration/mups/learn/learner.go` + `mups/learn/reputation/parent_evidence.go`
**Effort:** 1.5 人天
**AC:** AC20

| ID | Description | Status |
|---|---|---|
| T25 | LearnRequest 加 WorkItemID 字段(per-segment attribution) | TODO |
| T26 | ParentEvidence aggregator:从所有 child α/β 合并成 parent α/β (sum, fold into AdaptivePrior) | TODO |

## P0-8 D1 飞书流式 + idempotency

**File:** `internal/layers/communication/feishu/streaming.go`
**Effort:** 1.5 人天
**AC:** AC21-AC22

| ID | Description | Status |
|---|---|---|
| T27 | `EmitPartial(ctx, idempotencyKey, content)` + `EmitFinal(ctx, idempotencyKey, content)` 飞书 API | TODO |
| T28 | in-memory dedup table:(session_id, segment_id) → Emitted,重复调走 noop | TODO |

## P0-9 Config flag + e2e LP-3 + verify-archive

**File:** `internal/bootstrap/config.go` + `scripts/verify-archive.sh`
**Effort:** 1.5 人天
**AC:** AC23-AC24

| ID | Description | Status |
|---|---|---|
| T29 | `devrix.d7.dag_executor.enabled` (bool, default false) — routing flag | TODO |
| T30 | E2E LP-3 multi-intent test:发 "1+1=几?另外查 devrix 架构" → 验证 2 张卡片 + 完整 finalText + α/β 累计 | TODO |
| T31 | spec.md + t-registry.md delta + changelog 同步 | TODO |

## P0-10 Decision Node(D7 6 节点流水线新增独立第 5 stage)

**File:** `internal/layers/orchestration/executionflow/verify/decision_node.go` (NEW) + `internal/layers/orchestration/sessionorchestrator/child_workitem.go` (NEW) + `internal/layers/orchestration/workmodel/child.go` (MODIFIED)
**Effort:** 2.5 人天
**AC:** AC29-AC34 (新增)

| ID | Description | Status |
|---|---|---|
| T46 | `DecisionKind` 5 路径枚举(accept / retry / child_worker / parent_rollup / human_review)+ Validate() | TODO |
| T47 | `DecisionNode.Decide(ctx, verdict, acs, verdicts, meta) (Decision, error)` 接口 + 11 行静态映射表(10 Verdict-based + 1 plan_error) | TODO |
| T48 | `ChildWorkItemSpec` 类型(ParentWorkItemID / SubSegmentIDs / InheritACSubset / MaxBudget=2)+ 校验 | TODO |
| T49 | `SubWorkerSpawner.SpawnChild(ctx, spec) (*WorkItem, error)` + `OnChildComplete` 触发 parent decision 重算 | TODO |
| T50 | Decision 5 路径单元测试(11 行映射表全测覆盖)+ 边界(降级 / budget=0 / sibling 未齐) | TODO |
| T51 | Decision 持久化:round.Metadata["decision"] = JSON{kind, reason, next_spec} + reputation.metadata.decision_kind | TODO |
| T52 | E2E LP-4 decision-pending test:发 partial-fail-prone 指令 → 验证 Decision=child_worker 触发 + 子 AC[] 复用 + parent rollup 触发 D 决策 | TODO |

## P0-11 Learn Node(D7 6 节点流水线最末 stage,per-segment 升格 + 异步化 + 契约精简)

**File:** `internal/layers/orchestration/mups/learn/learner.go` (MODIFIED) + `mups/learn/reputation/row.go` (NEW) + `mups/learn/reputation/parent_evidence.go` (NEW) + `mups/learn/async.go` (NEW)
**Effort:** 3.0 人天
**AC:** AC35-AC42 (新增)

| ID | Description | Status |
|---|---|---|
| T53 | `LearnRequest` 类型精简(仅收 WorkItemID + Decision + Verdict + ArtifactHash 可选,不收 Plan/Observations/ParentContext) | TODO |
| T54 | `LearnResponse` 类型 + `BayesianAction` 4 枚举(alpha_bump / beta_bump / no_change / force_plan) | TODO |
| T55 | `ReputationRow` 类型 + DB schema **11 字段**(plan_rationale **deprecated** → 迁移到 metadata.rationale,保留列只为兼容旧 row)+ migration | TODO |
| T56 | `BayesianUpdate` 纯函数(α++/β++/retry 不累计/force_plan 触发)+ 单元测试覆盖 6 路径 | TODO |
| T57 | `ParentEvidence` aggregator(Learn 内部按 Decision.parent_rollup 从 DB 查 child rows → sum → parent row)+ 边界(child 缺失降级) | TODO |
| T58 | `AsyncLearner` 异步化(chan queue size=100, worker=2, Enqueue < 1ms 不阻塞, Drain 阻塞) | TODO |
| T59 | E2E LP-5 multi-round reputation test:发 S-D 指令 3 次 → 验证 child α/β 累加 + parent rollup sum + force_plan 触发 | TODO |

## P0-11.5 Learn 节点 22 场景全覆盖(Observe + 5 维度扩展)

**File:** `internal/layers/orchestration/mups/learn/scenarios.go` (NEW) + `mups/learn/feishu_user_action.go` (NEW) + `mups/learn/learner.go` (MODIFIED,策略分发)
**Effort:** 1.5 人天
**AC:** AC43-AC54 (新增)

| ID | Description | Status |
|---|---|---|
| T60 | `LearnPolicy` 类型 + `ShouldEnqueue()` / `BayesianAction()` 接口 + `classifyScenario()` 22 场景 → 策略纯函数映射 | TODO |
| T61 | 上游错误维度(7 场景 E1-E6):Plan 错误 NO Learn + audit log(metric `plan_error_no_learn++`);Execute/Verify 错误正常 Learn(β++);E4-post 走 α++ (last good) | TODO |
| T62 | 用户主动维度(3 场景 U1-U3):`feishu_user_action.go` 监听 user-cancel/accept/modify 事件 → U1 β++ / U2 α++ / U3 不 Learn(本轮) | TODO |
| T63 | force_plan 链路(2 场景 F1-F2):F1 触发时写 `metadata.next_observation_force_plan=true`;F2 下次 directive 走 Plan 路径,enqueue α++ | TODO |
| T64 | Learn 自身失败 3 级降级(L1/L2/L3):L1 silent + retry 3 + 最终 warn;L2 队列满降级 Sync.Learn;L3 未 Drain 写 DeferToNextSession marker | TODO |
| T65 | 22 场景单元测试覆盖(`scenarios_test.go`,每场景 1 个 test case)+ 6 维度并发压测 | TODO |

## P0-12 Plan 节点 26 场景全覆盖(Plan 字段验证 + Parse Reject 重试 + Decision plan_error 新路径)

**File:** `internal/layers/orchestration/plan/plan_validator.go` (NEW) + `plan/reject_retry.go` (NEW) + `sessionorchestrator/plan_error_decision.go` (NEW) + `plan/plan_field_validator_test.go` (NEW)
**Effort:** 2.0 人天
**AC:** AC55-AC67 (新增)

| ID | Description | Status |
|---|---|---|
| T66 | `PlanFieldValidator` 类型 + `Validate(proposal)` + 字段级校验(空 Children / 空 DAG / DAG 0 nodes / AC CheckKind 越界 / AC Required=0 / rationale 缺失 / priorities 越界 / segments ID 缺失) + 8 子类拒绝 | TODO |
| T67 | `PlanParseReject` 扩展 6 子类(RejectEmptyChildren / RejectEmptyDAG / RejectInvalidCheckKind / RejectRequiredZero / RejectPrioritiesOutOfRange / RejectSegmentIDMissing)+ 已有 6 子类合并共 12 子类 | TODO |
| T68 | `RetryWithFeedback(ctx, priorProposal, reject)` 重试 Plan LLM ≤ 2 次 + CompactJSON 错误 feedback(同 §9 feedback_loop 3 子类模式)+ 超限 fallback DecomposeIntoChildren | TODO |
| T69 | `PlanErrorDecision` 类型 + `Decide()` 直接 emit abort + 飞书卡 "❌ Plan 阶段失败" + NO Learn(metric `plan_error_no_learn++`)+ 3 错误类型(PlanLLMCallTimeout / PlanLLMCallError 5xx / PlanLLMCallPartialResponse) | TODO |
| T70 | Decision 映射表扩展 10 行 → 11 行:新增第 11 行 `plan_error` → E human_review(正交于 Verdict-based 10 行 baseline) | TODO |
| T71 | force_plan Plan 路径:PlanFieldValidator 读 `metadata.next_observation_force_plan=true` → Plan prompt 注入 "强制 Required AC[] ≥ 1" + PriorityHint | TODO |
| T72 | 26 场景单元测试(`plan_field_validator_test.go`,8 字段级 + 6 ParseReject 子类 + 3 PlanErrorDecision + 2 force_plan + 1 S-E Decision 边界 = 20 核心 case + 6 维度并发压测) | TODO |

## 总结

- P0 T 总数: **59** (跨 15 个 P0 模块,新增 P0-11 Learn Node 7 T 点 + P0-11.5 Learn 22 场景 6 T 点 + P0-12 Plan 26 场景 7 T 点)
- DONE: 0
- TODO: 59
- 进度: 0%

预估总 effort: **28.0 人天**,跨 **7 PR**(PR-A1 / PR-A2 / PR-B / PR-C / PR-D / PR-E / PR-F)

新增 Learn Node 节点对原 PR 拆分的影响:
- **PR-A1(grammar/SpawnPolicy,H4 拆分)**:T01-T04, T10-T14(7 T 点,IntentSegment grammar + PlanDAG grammar + DAG validator + SpawnPolicy)→ **拆 PR-A1(grammar only)**
- **PR-A2(AC contract + LLM IO,H4 拆分)**:T31-T45(15 T 点,AcceptanceCriterion + PerCriterion + PlanLLMIO + VerifyLLMIO + feedback loop)→ **拆 PR-A2(AC contract + LLM IO)**
- PR-B(WaveScheduler DAG executor):T18-T21, T46-T49(Decision + Sub-Worker Spawn)→ +2.5 人天
- PR-C(streaming + idempotency):T22-T28 → 不变
- PR-D(config flag + e2e + verify-archive):T29-T30, T50-T52 → +1.0 人天(decision 单元测试 + LP-4)
- **PR-E(Learn Node 升格 + 异步化 + 22 场景覆盖)**:T53-T65(13 T 点, 4.5 人天)→ **新 PR**

预估总 effort: **28.0 人天**,跨 **7 PR**(PR-A1 + PR-A2 + PR-B + PR-C + PR-D + PR-E + PR-F)

**H4 拆分理由**:原 PR-A 一锅端 17 T 点过重(2.5 周工作量),review diff 大 + rollback 风险高。
- PR-A1 = grammar-only(类型 + validator + SpawnPolicy),7 T 点,1.0 人天,快速建立基础;
- PR-A2 = AC contract + LLM IO 合并,15 T 点,3.0 人天,在 PR-A1 基础上叠加契约 + LLM 协议。
- 两 PR 解耦,任一失败不影响另一;PR-A2 可等 PR-A1 review 通过后再开。

**PR-F 工作量明细**:
- T66 PlanFieldValidator 字段级校验 8 子类:0.3 人天
- T67 PlanParseReject 扩展 6 子类合并:0.2 人天
- T68 RetryWithFeedback 重试 ≤ 2 次 + fallback DecomposeIntoChildren:0.4 人天
- T69 PlanErrorDecision emit abort + NO Learn:0.2 人天
- T70 Decision 10 行 → 11 行映射表扩展:0.2 人天
- T71 force_plan Plan 注入 Required AC[] + PriorityHint:0.3 人天
- T72 26 场景单元测试 + 6 维度并发压测:0.4 人天

PR-E 工作量明细:
- T53 LearnRequest 扩展:0.5 人天
- T54 LearnResponse + BayesianAction:0.3 人天
- T55 ReputationRow + DB migration:0.5 人天
- T56 BayesianUpdate 纯函数 + 6 路径单元测试:0.7 人天
- T57 ParentEvidence aggregator + 边界:0.4 人天
- T58 AsyncLearner + 异步不阻塞:0.4 人天
- T59 E2E LP-5 multi-round reputation test:0.2 人天
- **P0-11.5 Learn 22 场景覆盖(新增)**:
  - T60 LearnPolicy + classifyScenario 纯函数:0.3 人天
  - T61 上游错误 7 场景策略分发 + audit log:0.3 人天
  - T62 feishu_user_action.go(user-cancel/accept 监听):0.4 人天
  - T63 force_plan 链路 F1/F2 metadata 写入 + Plan 路径:0.2 人天
  - T64 Learn 失败 3 级降级(L1/L2/L3):0.2 人天
  - T65 22 场景单元测试 + 6 维度并发压测:0.1 人天
- **P0-12 Plan 26 场景全覆盖(新增)**:
  - T66 PlanFieldValidator 字段级校验 8 子类:0.3 人天
  - T67 PlanParseReject 扩展 6 子类合并:0.2 人天
  - T68 RetryWithFeedback 重试 ≤ 2 次 + fallback DecomposeIntoChildren:0.4 人天
  - T69 PlanErrorDecision emit abort + NO Learn:0.2 人天
  - T70 Decision 10 行 → 11 行映射表扩展:0.2 人天
  - T71 force_plan Plan 注入 Required AC[] + PriorityHint:0.3 人天
  - T72 26 场景单元测试 + 6 维度并发压测:0.4 人天
