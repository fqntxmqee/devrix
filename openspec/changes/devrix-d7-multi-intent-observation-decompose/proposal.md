# Proposal: D7 multi-intent observation decompose + Plan DAG + parallel Execute

**Change ID:** `devrix-d7-multi-intent-observation-decompose`
**Demand ID:** DM-20260707-001
**Priority:** P0
**PR Count:** 7 (PR-A1 grammar/Plan fields · PR-A2 AC contract + LLM IO · PR-B DAG executor · PR-C streaming emit · PR-D gating+e2e · PR-E Learn+22-scenario · PR-F Plan+26-scenario)
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S6_Delivered → S7_Archived

---

## 1. Background

D7 MUPS v4 流水线当前把每个 directive 当作**单意图原子单元**:Observe 节点产 1 个 ObservationReport,Plan 节点产 1 个 Plan,Execute 节点串行跑 1 个 Worker。但用户实际发出的很多指令是**多意图混合**(deterministic Q&A + multiple explore/commit/analyze tasks),今天只能走以下两条路之一:

1. **串行整跑**:Plan 节点把所有 segments 揉成 1 个 Plan,Execute 节点单 Worker 串行跑,确定性部分延迟 3-5s,跨 segment 数据流断裂。
2. **早返回但遗漏**:fast-path(DM-20260706-011)在检测到 ObsUncertainty 时直接降级,确定性 segment 也被拖累。

DM-20260706-011 只解决了**单意图确定性 Q&A** 的 fast-return,**多意图混合** 这个 P0 缺口仍在。本 Change 解决该缺口。

**关联**:
- 上游:DM-20260706-011 observational_answer fast-path(S1 阶段产物,本次 PR-A 的语义前置)
- 上游:devrix-d7-mups-v4-phase6-observe-learner-wiring(IntentClassifier 接口扩展)
- 下游:devrix-d7-taskcontract-unification(TaskReport 五元素扩展预留位)

---

## 2. Problem Statement

**当一个 directive 包含 ≥1 个确定性 fragment + ≥1 个不确定性 fragment,Observe 节点应当识别出 fragment 边界,在 Observe 阶段就把 directive 拆成 N 个 IntentSegment(每个 segment 一个独立 WorkItem);Plan 节点接收这些 segment 后,构建一个 DAG(节点=WorkItem,边=依赖关系,priority=critical path),由 Execute 节点按 DAG 拓扑序执行,无依赖 segment 并行(硬上限 4 worker),流式 emit 各 child 的 partial finalText,parent rollup 节点汇总最终答案。**

影响范围:

- D7 ItemPipelineRunner.Run() 入参语义变化(从单 directive 演进为 SegmentSet)
- plan.Plan 类型扩展(从单 Plan 演进为 PlanDAG)
- wavescheduler 从单 Worker 演进为 DAG executor + 4 worker pool
- IM adapter 从单卡 emit 演进为 streaming partial + final emit
- Reputation attribution 从 per-Item 演进为 per-Segment(Learn 不能再全局 Verdict)

---

## 3. Goals

| Goal | Metric | Target |
|------|--------|--------|
| 多意图混合指令自动分解 | IntentSegment 数 == directive 真实意图数(LLM judge) | ≥ 90% |
| 并行 segment 真正并发执行 | 无依赖 segment 并发跑 (而非串行假并发) | 100% (有依赖除外) |
| hard limit 防 LLM Gateway 压垮 | WaveScheduler 同跑 worker 数 | ≤ 4 |
| 流式 partial emit | fast-path segment 平均 emit latency | ≤ 0.5s (从 Observe 完到 user-visible) |
| 端到端延迟 | trivial 2-segment 指令 ("1+1=几 + 巴黎时区") | ≤ 1.5s (从用户输入到完整 finalText) |
| Reputation 持续可用 | Learn per-segment α/β 累计 | 100% 兼容 DM-20260706-011 |
| CI 通过 | 27 包 orchestration `go test -race` + go vet | PASS |

---

## 4. Non-Goals

明确**不**做:

- ❌ **跨 segment 显式 DataDep 字段**(v1 不引入 `depends_on_outputs`;跨 segment 数据由 parent rollup 单点聚合,代价是 sub-second 上下文丢失)
- ❌ **流式 streaming card append**(v1 走"先 emit 快的 + 后补最终版"2 卡模式;不引入 IM streaming API)
- ❌ **自适应 MaxFanOut**(v1 硬上限 4;v2 用 metrics-driven 自适应)
- ❌ **critical path 自动推导**(v1 priority 由 LLM 显式给;v2 加 critical-path 静态分析器)
- ❌ **跨 session segment 复用**(v1 每个 session 独立 segment;复用留 v2)
- ❌ **LLM Gateway bypass / 私有模型并发**(复用现有 LLM Gateway 限流,不绕过)

---

## 5. Solution

### 方案 A(推荐):Observe IntentSegmenter + Plan DAG executor + WaveScheduler DAG runner + Learn per-segment

**核心思路**:

1. **Observe 节点新增 IntentSegmenter**:在 strategic_observation_proposer 输出 ObservationReport 时,附带 IntentSegment[](`{segment_id, text, intent_kind, priority, confidence}`)。IntentSegment 与 ObservationReport 解耦:报告是"看到了什么",segment 是"该被拆成几个 WorkItem"。
2. **plan.Plan 扩展为 plan.PlanDAG**:`{nodes: []PlanNode, edges: []DataEdge, priorities: map[string]int}`。每个 PlanNode 是一个 child WorkItem spec;DataEdge 暂留字段但 v1 不解析(`depends_on_outputs` 留空)。Plan LLM 显式返回 DAG,validateDAG() 校验无环 + 节点数 ≤ MaxFanOut(= 8,留 buffer 给 fallback re-plan)。
3. **WaveScheduler 新增 DAG executor**:`RunPlanDAG(ctx, dag)` 返回 Sync.WaitGroup-style wait,内部维护 4-worker pool + ready queue(初始 = 无依赖 node,完成时 = 新 ready node)。priority 高的 ready node 优先抢 worker slot。
4. **Execute 节点接收 PlanDAG**:ItemPipelineRunner.Run() 检测到 Plan 字段携带 DAG 时,转发给 WaveScheduler.RunPlanDAG。每个 child 复用现有 ItemPipelineRunner 跑独立 round(via 同一 SessionWorkItem,但 WorkItemID 不同)。
5. **Stream emit**:"先 emit 快的 + 后补最终版"。ItemPipelineRunner 在每个 child 完成时立刻 emit 一个 partial EngineEvent 给 IM adapter(IM 立即发卡),parent rollup 完成后 emit 最终 EngineEvent 覆盖卡片。idempotency key = `(session_id, segment_id)`,重复 emit 通过去重避免重复发卡。
6. **Learn per-segment**:每个 child Learn 单独跑,`learnRequest.WorkItemID = child.ID` 而不是 parent.ID,reputation 按 segment 维度累计。ParentEvidence 由 rollup 时聚合各 child α/β → Mean + Variance。

**影响范围**:

| 文件 | 改动 | 行数估计 |
|------|------|----------|
| `orchtypes/intent_segment.go` | NEW IntentSegment + IntentSegmentSet | +180 |
| `sessionorchestrator/intent_segmenter.go` | NEW LLM segmenter + rule validator | +260 |
| `plan/plan_dag.go` | NEW PlanNode + DataEdge + PlanDAG | +200 |
| `plan/strategic_plan_proposer.go` | 扩展 proposers 返回 PlanDAG | +120 |
| `plan/dag_validator.go` | NEW validateDAG(无环 + node ≤ MaxFanOut) | +150 |
| `wavescheduler/dag_executor.go` | NEW RunPlanDAG + 4 worker pool + ready queue | +450 |
| `wavescheduler/runners/dag_runner.go` | NEW DAG-aware ExecuteWorkItem 入口 | +200 |
| `sessionorchestrator/item_pipeline.go` | Run() 检测 PlanDAG → 转发 + 流式 emit | +180 |
| `mups/learn/learner.go` | LearnRequest 加 WorkItemID + ParentEvidence 聚合 | +120 |
| `d1/feishu/streaming.go` | NEW idempotency key + partial emit | +220 |
| `contextengine/i18n/format_hints_mups.go` | IntentSegment appendix (ZH + EN) | +80 |
| 测试:各包 + e2e LP-3 multi-intent | | +800 |
| **总计** | | **~+2960** |

**风险**:

| 风险 | 严重度 |
|------|--------|
| LLM 提议的 DAG 含环/超 fan-out | Medium;validateDAG() + PlanParseReject 重试解决 |
| 并发 4 worker 压垮 LLM Gateway | Medium;v1 硬上限已够,生产 ramp-up 慢启动 |
| 流式 emit 重复发卡 | Medium;idempotency key (session_id, segment_id) 去重 |
| Per-segment reputation 噪声 | Low;v2 加 per-segment smoothing |
| Plan LLM 不知道 DAG schema | High;需要新 prompt template + 详细 example + 测试 |

**回滚成本**:Medium。PlanDAG 是 plan.Plan 的 superset,旧 Plan 路径保留为 fallback;`cfg d7.dag_executor.enabled = true/false` 是 feature flag,可以黑/白切换;数据无迁移。

### 方案 B(已弃用,S3-Gate 后校准):Plan 节点"现有 SpawnPolicy.DecomposeIntoChildren" + Execute 节点 hardcode 并行

> **2026-07-07 事实校准**:本方案假定的"现有 SpawnPolicy.DecomposeIntoChildren"**不存在**。`SpawnPolicy` 实际是 `workmodel/pipeline_round.go:27-34` 的 3 值字符串枚举,`DecomposeIntoChildren` 不是其值。S3-Gate review 时此方案实际已被证伪,新方案 β(Plan 加 2 可选字段)取代。保留方案 B 仅作历史对照,无须实施。

**核心思路(已废)**:不动 Plan 类型,Plan 节点继续产出 Plan。Execute 节点扫描 Plan.SpawnPolicy,如果 Decompose 就 hardcode 串行触发 children(不是真并行,本质是顺序 await)。

**影响范围**:小,~600 行。

**风险**:不能并行执行 = 多 segment 延迟没改善。

**回滚成本**:Low,但能力上限低。

### 推荐 A

A 才是真正的"Plan 出 DAG + Execute 并行执行"。B 是 hack,只解决了"能拆"但解决不了"并行"。

### 5.1 场景分流摘要(详细方案见 `decision-tree.md`)

下面给出方案 A 在 5 个典型场景下的执行模式映射表。**每个场景的完整流程图、Observe gate 触发条件、PlanDAG / RunPlanDAG 调度细节、降级路径**——见同目录 `decision-tree.md`(320 行,本文档配套详细方案)。

#### 决策矩阵

| 场景 | segment 数 | 确定性 | 不确定性 | Plan 字段 | Execute 调用 | stream emit | 总延迟 |
|---|---|---|---|---|---|---|---|
| **S-A 单确定性** | 1 | 1 | 0 | ❌ 跳过(IntentSegmentSet/DAG 全 nil) | ❌ 跳过 → fast-path emit | 1 final | ~1s |
| **S-B 单不确定性** | 1 | 0 | 1 | ✅ Plan 4-channel(IntentSegmentSet/DAG 全 nil) | ✅ 单 Worker 串行 | 1 final | ~4s |
| **S-C 多确定性** | N≥2 | N | 0 | ❌ 跳过(maybeObservationalAnswerMulti) | ❌ 跳过 → fast-path emit 合并 | 1 merged final | ~1s |
| **S-D 多不确定性** | N≥2 | 0 | N | ✅ Plan(IntentSegmentSet + DAG) | ✅ RunPlanDAG 4 worker 并行 | 1 rollup final | ~5s |
| **S-E 混合** | N≥2 | ≥1 | ≥1 | ✅ Plan(IntentSegmentSet + DAG) | ✅ RunPlanDAG 混合(fast-path segment 子段 + verified 子段) | N partial + 1 final | ~0.5s 看到快答,~5s 最终 |

> **说明**:Plan 字段 = `Plan.IntentSegmentSet` + `Plan.DAG`(方案 β,2026-07-07 拍板)。SpawnPolicy 保持现有 3 值(`SpawnNone/SpawnDecompose/SpawnInline`)不变。

#### 5 场景一句话摘要

- **S-A** `1+1=几?` → Observe 识别 1 个 deterministic segment → **Observe 直接 return**(DM-20260706-011 fast-path)
- **S-B** `查 devrix 项目结构` → Observe 识别 1 个 explore segment → **走旧 Plan + 单 Worker 路径**(不变)
- **S-C** `1+1=? 2×3=? 巴黎时区?` → Observe 识别 3 个 deterministic segment → **Observe 直接 return**(扩展 maybeObservationalAnswerMulti)
- **S-D** `查 devrix 架构 + 分析 d2 风险 + 评估 v7 演进` → Observe 识别 3 个 explore/analyze/commit segment → **Plan 出 PlanDAG + RunPlanDAG 4 worker 并行 + parent rollup**
- **S-E** `1+1=? + 查 devrix 架构` → Observe 识别 1 deterministic + 1 explore → **Plan 出 PlanDAG + RunPlanDAG 混合模式**(seg_a fast-path partial 即时发,seg_b Worker 跑完发 partial,parent rollup 发 final)

#### Observe 直接 return 的硬规则(全部满足才放行)

```
✅ len(Segments) >= 1
✅ ∀ segment: Confidence >= 0.85 AND IntentKind ∈ {deterministic}
✅ ∀ o ∈ report: if o.Kind == ObsUncertainty AND o.Source ∉ {item_pipeline, verify_signal} → FAIL
✅ !isRollup && !isDeliverableSynth && !isParentRollup
✅ r.Learner != nil
```

否则降级 → 整个 directive 进 Plan 节点;Plan 节点根据 segment 数决定 **Plan 字段**(方案 β,2026-07-07 拍板):
- `len(Segments) == 1 && all deterministic` 触发 fast-path(S-A/S-C) → **Plan.IntentSegmentSet/DAG 都 nil,走 4-channel 路径**
- `len(Segments) >= 2` 触发 multi-intent 路径 → **Plan 写入 IntentSegmentSet + DAG**(S-D/S-E)
- `len(Segments) == 1 && uncertain` 触发旧 4-channel 路径(S-B 后向兼容,**SpawnPolicy 不变,Plan 字段全 nil**)

> **事实校准(S3-Gate 后 2026-07-07 拍板)**:`SpawnPolicy` 是 `workmodel/pipeline_round.go:27-34` 的 3 值字符串枚举,由 D7 Convergence Contract CC-1.1~CC-1.5 锚定,**不可改**。multi-intent 语义全部由 `Plan.IntentSegmentSet` + `Plan.DAG` 承载,SpawnPolicy 完全不动。

#### 关键设计决策

- **S-C 合并成 1 final 卡**(不切 N partial):简洁优先;v2 可切换 streaming partial
- **S-E 走 PlanDAG 而非 Observe fast-path**:虽然有确定性部分,但 hasObsUncertainty 全局挡 → PlanDAG 内 fast-path 部分子 segment(per-segment 评估)
- **多确定性场景(S-C)不走 RunPlanDAG**:节省 4 worker slot;只需合并 ObsFact → emit 1 final
- **parent rollup 等所有 child 完成才 emit final**:无 staleness;支持 idempotency key 去重

详见 `decision-tree.md` §2-§8 的 ASCII trace + gate condition + 边界 case 降级路径。

### 5.2 Plan ↔ Verify 验收契约(AcceptanceCriteria)

当前 v1 设计核心缺口:**Plan LLM 知道"应该验什么",Verify 不知道**。PlanDAG 节点结构缺少 `AcceptanceCriteria[]` 字段。

**核心思路**:

- 每个 `PlanNode` 携带一组 `AcceptanceCriterion`(声明 artifact 应满足什么条件)
- Plan LLM **与 PlanDAG 同步输出** acceptance_criteria[](`PlanLLMOutput`)
- `PlanAcceptanceContractBuilder.Build(dag, ac)` 校验一致性:每个 node ≥ 1 Required criterion、AC.ID 全局唯一、无引用 missing node
- Verify 节点拿 Artifact + AC[] → 逐条判定 → 聚合 VerdictKind
- 机械 CheckKind(`ContainsString` / `MentionsAll` / `NumericRange` / `JSONPath` 等)**本地执行,0 LLM 调用**(延迟 < 50ms)
- `CustomLLMJudge` CheckKind 委托 D3 LLM,带 plan_rationale,数量上限 3(budget 控制)

**判定聚合规则**(PerCriterionVerdict aggregation_rule):

```
∃ Required criterion Fail  → VerdictKind = VerdictFail
全部 Required Pass + ∃ Preferred Fail → VerdictPartial
全部 Pass                          → VerdictPass
任一 Error 且无 Fail              → VerdictIndeterminate
```

**Per-Criterion Evidence** 持久化到 `round.Metadata["ac_verdicts"]` JSON,飞书 final card 可读,audit 可查。

**完整类型契约 + IO 字段 + feedback loop 模板** 见 `spec_delta.md §3.5/§3.6/§11/§12`。

### 5.3 大模型前后协议(Plan LLM / Verify LLM IO)

当前 v1 设计缺口:Plan LLM 输入输出协议不完整、PlanParseReject feedback 格式未细化、Verify LLM IO 全无 schema 描述。S4 实现阶段 LLM 调用的 schema 必须先确定。

**PlanLLMIO 协议**(节选,完整见 `spec_delta.md §9`):

```
PlanLLMInput:
  - directive: string
  - segments: IntentSegmentSet
  - prior_parse_reject: PlanParseReject | null
PlanLLMOutput:
  - dag: PlanDAG                         # validateDAG 必须通过
  - acceptance_criteria: []AcceptanceCriterion  # 每个 node ≥ 1 Required
  - rationale: string (可选)
Schema: plan_llm_input_v1.json / plan_llm_output_v1.json  # i18n 强制
```

**PlanParseReject feedback loop 3 子类**:

```
RejectCycle → "上轮 DAG 含环: {cycle_path}。保持 segment 覆盖,重切 DAG。"
RejectTooManyNodes → "上轮 {n} 节点超 MaxFanOut={cap}。合并相邻 segment。"
RejectACDuplicateID → "上轮 AC.ID 重复: {ids}。确保全局唯一。"
```

**VerifyLLMIO 协议**(节选,完整见 `spec_delta.md §10`):

```
VerifyLLMInput:
  - artifact_summary, artifact_metadata, criteria: []AC, plan_rationale
  - CustomLLMJudge 数量 ≤ 3 (budget)
VerifyLLMOutput:
  - per_criterion_verdicts: []PerCriterionVerdict  # 顺序对齐 criteria
  - overall_verdict: enum(Pass/Partial/Fail/Indeterminate)
  - evidence: string
ShortCircuit: 0 CustomLLMJudge → Verify 不调 LLM (延迟 < 50ms)
```

**3-shot example 必备**:全 deterministic / 全 uncertain / 混合 3 种 scenario,每个 example 必须含 AC[]。

**错误码注册**:

- `ErrPlanLLMOutputInvalidJSON` / `ErrPlanLLMOutputMissingDAG` / `ErrPlanLLMOutputMissingAC`
- `ErrPlanLLMIOBudgetExceeded`(累计 > 4s)
- `ErrVerifyLLMOutputMismatchCount`(verdicts.len ≠ criteria.len)
- `ErrVerifyLLMJudgeBudgetExceeded`

### 5.4 各场景的 Plan LLM 输出 + Verify 节点处理(基于 5 场景)

| 场景 | Plan LLM 输出 | Verify 节点处理 |
|---|---|---|
| **S-A 单确定性** | 不调用(S-A 跳过 Plan) | 跳过(走 fast-path) |
| **S-B 单不确定性** | 单 child Plan + 1 Required AC(LLM judge) | CustomLLMJudge → 1 outcome |
| **S-C 多确定性** | 不调用(maybeObservationalAnswerMulti) | 跳过 |
| **S-D 多不确定性** | PlanDAG(3 nodes) + 每个 node 多条 AC | 每个 child 独立 verify,parent rollup 聚合 |
| **S-E 混合** | PlanDAG(2 nodes) + 每个 node 各自 AC | fast-path segment 不进 verify,Worker segment 进 verify |

详细 trace 见 `decision-tree.md §5-§6 + §9-§10`。

### 5.5 Decision Node(D7 6 节点流水线新增独立第 5 stage)

**核心思路**:Verify 节点产出 PerCriterionVerdict[] + VerdictKind 后,Decision 节点(独立 stage,5 节点 → 6 节点升级)立即跑映射表,产出 5 路径决策(接受/重试/子 Worker/父 rollup/人工)。**D7 升为 6 节点流水线**(Observe/Plan/Execute/Verify/Decision/Learn,都是独立 stage)。

**3 步决策**:

```
Step 1: 输入
  - PlanLLMOutput{DAG, AcceptanceCriteria, Rationale}
  - Artifact{Summary, Metadata, Evidence}
  - PerCriterionVerdict[] (顺序对齐 AC[])
  - VerdictKind {Pass | Partial | Fail | Indeterminate}
  - RoundMeta{AttemptNo, ChildBudgetRemaining, RiskLevel}

Step 2: 决策映射表(纯规则引擎,0 LLM 调用)
  ┌─────────────────────────────────────────────────────────────────┐
  │ Verdict       │ Other Conditions                       │ Decision │
  ├─────────────────────────────────────────────────────────────────┤
  │ Pass          │ (default)                              │ A 接受  │
  │ Partial       │ Tolerance=high OR ChildBudget=0        │ A 接受  │
  │ Partial       │ Partial 部分 AC 可独立分解 + Budget>0  │ C 子W   │
  │ Partial       │ 其它                                   │ A 接受  │
  │ Fail          │ AttemptNo < MaxRetry=1                 │ B 重试  │
  │ Fail          │ AttemptNo >= MaxRetry                  │ E 人工  │
  │ Indeterminate │ RiskLevel=high                         │ E 人工  │
  │ Indeterminate │ RiskLevel=normal/low                   │ B 重试  │
  │ Error(全Err)  │ Network/Timeout 类                     │ B 重试  │
  │ (任意)        │ current is child + all sibling decided │ D 父R   │
  └─────────────────────────────────────────────────────────────────┘

Step 3: 输出 Decision{D, Reason, NextWorkItemSpec?}
  - A 接受 → 调 emit final + Learn
  - B 重试 → RoundMeta.AttemptNo++,复用 AC[] 重跑 ItemPipelineRunner
  - C 子 Worker → 创建子 WorkItem,parent.AC[] 中 Required Fail 的子集作为子 AC[]
  - D 父 rollup → 触发 parent rollup 节点聚合
  - E 人工 → 飞书卡标"❓ 需人工确认" + emit abort 事件
```

**"下一层 WorkTree" 语义(关键设计决策)**:

- ✅ **当前 segment 内 spawn 子 Worker**(v1 选):复用 parent AC[] 中 Required Fail 的子集,Worker 跑子任务模板。粒度细,适合"部分答案对"场景。
- ❌ 重新分 segment 再 Plan 一次(留 v2):粒度粗,适合"整个方向错了"场景,反馈路径 +1-2s。
- ❌ 两条路径并存(留 v2):灵活但实现复杂。

**关键边界**:

- **B 重试上限**:`MaxRetry = 1`(从 Verify 失败重试 1 次),AttemptNo 在 round metadata 持久化。
- **C 子 Worker 上限**:`ChildBudgetRemaining = 2`(每个 segment 最多 spawn 2 个子 Worker),用完降级 A 接受。
- **D 父 rollup 触发**:仅当"当前是 child segment + 所有 sibling 已决策(任意 decision)"才触发,避免 sibling 还在跑时提前 rollup。
- **E 人工触发**:仅当 RiskLevel=high 才人工;normal/low 都走 B 重试。
- **决策延迟**:`< 5ms`(纯静态映射表查表,无 I/O)。

**5 路径决策依据**:**纯规则引擎映射表**(v1),LLM judge 留 v2 metrics 驱动。0 LLM 调用,可测试性极强。

**对原 5 节点流水线影响(D7 升为 6 节点)**:

| 节点 | v1 (MUPS v4.3 5 节点) | v1.1 (DM-20260707-001 6 节点) |
|------|----------------------|-------------------------------|
| 1. Observe | IntentSegment + ObservationReport | 不变 |
| 2. Plan | PlanLLMOutput(DAG + AC[])+ 3-shot | 不变 |
| 3. Execute | RunPlanDAG(4 worker) | 不变 |
| 4. **Verify** | PerCriterion 判定 → 4 态 Verdict | **不变**(PerCriterion 判定 → 4 态 Verdict) |
| 5. **Decision** | — (旧路径无独立 Decision 节点) | **新增独立 stage**:11 行静态映射表(10 Verdict-based + 1 plan_error)→ 5 路径决策 |
| 6. **Learn** | 收 Verdict 写 reputation(全局) | **收 Decision + Verdict**(per-segment)+ ParentEvidence aggregator + **异步不阻塞** |

**6 节点契约新拓扑**:

```
Observe → Plan → Execute → Verify → Decision → Learn
                                  ↓            ↓
                              Verdict      BayesianUpdate
                              ↓            ↓
                          Decision       Reputation DB
                          ↓              (async, 不阻塞)
                        5 路径(A/B/C/D/E)
                          ↓
                  (A/B/C 走回到 1-4 节点循环)
                  (D 触发 parent rollup 节点)
                  (E 走飞书 abort + 仍进 Learn)
```

**关键设计决策**:
- D7 升为 **6 节点流水线**(Observe / Plan / Execute / Verify / Decision / Learn)
- Decision 节点是**独立 stage**(不是我之前写的 Verify+Decision 合并)
- Learn 节点是**独立 stage**,**异步不阻塞 emit final**
- 5 节点契约破坏,但 6 节点契约更清晰,语义可读性 ↑

**5 场景的 Decision 路径分布**:

| 场景 | 期望 Decision | 触发 |
|------|---------------|------|
| S-A 单确定性 | — (不经过 Verify) | fast-path 直接 return |
| S-B 单不确定性 | A 接受 | 简单任务,LLM judge 1 次易通过 |
| S-C 多确定性 | — (不经过 Verify) | multi-fast-path 直接 return |
| S-D 多不确定性 | n_a: A / n_b: A / n_c: A 或 C (若 LLM judge 给 Partial) | 复杂任务,部分子节点可能触发 C |
| S-E 混合 | seg_a: — (fast-path) / seg_b: A | 1 子段 fast-path,1 子段正常 verify |

**完整 Decision 路径 trace + 边界 case 降级**见 `decision-tree.md §8.6 + §9-§10`。

### 5.6 Learn Node(6 节点流水线最末一环,per-segment 升格)

**核心思路**:Learn 节点是 D7 6 节点流水线最末 stage(独立 stage,不是 Verify 内部)。接收 **Decision 节点 + Verify 节点** 的输出,做两件事:(1) 每个 child segment 独立 BayesianUpdate,α/β 跨 session 累积;(2) parent rollup 时聚合 child α/β,落 parent reputation row。**Learn 异步执行,不阻塞 emit final**(emit final 完成后立即返回,Learn 后台 enqueue)。

**Learn 节点的直接依赖范围**(精简契约):

```
   Decision (5)  ──┐
                   ├─→ Learn (6)  ──→ Reputation DB
   Verify (4)    ──┘
```

| 节点 | Learn 依赖 | 用途 |
|------|-----------|------|
| **Decision (5)** | `Decision{Kind, Reason}` | retry 不累计 / human_review β++ / parent_rollup 走 aggregator |
| **Verify (4)** | `VerdictKind` + `PerCriterionVerdict[]` | BayesianUpdate:Pass→α++ / Fail→β++ |
| **Execute (3)** | `Artifact.MetadataHash`(可选项) | 防重(同一 artifact 重复 learn → no_change) |

**Learn 不直接依赖**:Plan / Observe / User 输入(LearnRequest 不收这些字段,精简契约)。

**3 步 Learn 流程**:

```
Step 1: 输入(精简)
  - LearnRequest{
      WorkItemID:    string,         // per-segment attribution
      Decision:      Decision,       // 来自节点 5
      Verdict:       Verdict,        // 来自节点 4
      ArtifactHash:  string,         // 可选,防重
    }

Step 2: 内部 BayesianUpdate(纯函数,无 I/O)
  prior α₀, β₀  (BuildAdaptivePrior cold start = Beta(5, 3))
  on Verdict.Pass           → α += 1
  on Verdict.Fail           → β += 1
  on Decision.Kind=accept   → 累计(正常 signal)
  on Decision.Kind=retry    → 不累计(retry 不算 reputation signal)
  on Decision.Kind=child_worker → child 完成后单独累计;parent 自身不累计
  on Decision.Kind=human_review → β += 1(强 negative)
  on Decision.Kind=parent_rollup → sum child α/β(ParentEvidence aggregator)

Step 3: 输出 + 持久化
  - LearnResponse{UpdatedAlpha, UpdatedBeta, ReputationRowID, BayesianAction}
  - reputation_row 持久化到 DB(异步):
      id / segment_id / parent_id / alpha / beta / last_updated
      decision_kind_history / source_id_history / artifact_metadata_hash
      metadata(JSON,含 rationale,user_action,force_plan)
      deprecated_plan_rationale(冻结,只为兼容旧 row,新 row 写 metadata.rationale)
  - BayesianAction 枚举:alpha_bump | beta_bump | no_change | force_plan
```

Step 2: 内部 BayesianUpdate(纯函数,无 I/O)
  prior α₀, β₀  (BuildAdaptivePrior cold start = Beta(5, 3))
  on Verdict.Pass           → α += 1
  on Verdict.Fail           → β += 1
  on Decision.Kind=accept   → 累计(正常 signal)
  on Decision.Kind=retry    → 不累计(retry 不算 reputation signal)
  on Decision.Kind=child_worker → child 完成后单独累计;parent 自身不累计
  on Decision.Kind=human_review → β += 1(强 negative)
  on Decision.Kind=parent_rollup → sum child α/β(ParentEvidence aggregator)

Step 3: 输出 + 持久化
  - LearnResponse{UpdatedAlpha, UpdatedBeta, ReputationRowID, BayesianAction}
  - reputation_row 持久化到 DB(11 字段,见 §3.11 + decision-tree §8.7.4):
      id / segment_id / parent_id / alpha / beta / last_updated
      decision_kind_history / source_id_history / artifact_metadata_hash
      metadata(JSON,含 rationale/user_action/force_plan)
      deprecated_plan_rationale(冻结)
  - BayesianAction 枚举:alpha_bump | beta_bump | no_change | force_plan
```

**关键设计决策(与 MUPS v4.3 已有能力的关系)**:

| 能力 | MUPS v4.3 已有 | PR-A (DM-20260707-001) |
|------|---------------|------------------------|
| per-Item Learn | ✅ 全局 Verdict 累计 | ⚠️ 废弃,改为 per-segment |
| per-segment Learn | ❌ | ✅ WorkItemID 字段 |
| ParentEvidence aggregator | ❌ | ✅ sum child α/β |
| decision_kind 入 reputation | ❌ | ✅ metadata.decision_kind |
| cold start BuildAdaptivePrior | ✅ Beta(5, 3) | ✅ 复用,不变 |
| 跨 session 累积 | ✅ | ✅ |
| retry 不累计 | ❌ (隐式) | ✅ 显式 (避免 noise) |
| Learn 失败不阻塞 | ❌ (主流程) | ✅ 降级 silent log |

**5 场景的 Learn 行为分布**:

| 场景 | Learn 次数 | 类型 | 落 reputation row |
|------|-----------|------|------------------|
| **S-A 单确定性** | 1 | fast-path VerdictPass (SourceID="obs_fact:seg_a_id") | child row seg_a: α++ |
| **S-B 单不确定性** | 1 | Verify VerdictPass + Decision=accept | child row seg_a: α++ |
| **S-C 多确定性** | N=3 | fast-path × 3(per-segment) | child row × 3: α++ each |
| **S-D 多不确定性** | N=3 + 1 parent | per-segment Learn × 3 + ParentEvidence aggregator | child row × 3 (α++) + parent rollup row (sum) |
| **S-E 混合** | 2 + 1 parent | seg_a fast-path Pass + seg_b verified Pass + parent rollup | seg_a α++ + seg_b α++ + parent rollup sum |

**Learn 节点 22 场景全覆盖**(扩展自 §8.7.5,完整表见 `decision-tree.md §8.7.10`):

之前的 5 场景仅覆盖"happy path",对**上游错误**(Plan 超时/失败、Worker panic、ctx cancel、LLM judge 超时、Decision map miss)、**用户主动行为**(user-cancel/accept/modify)、**force_plan 链路**、**Learn 自身失败**(DB 写挂/队列满/未 Drain)、**跨 session + 特殊语义**(segment 复用/emit 失败)均无明确定义。本 Change 扩展至 **22 场景 × 6 维度**:

| 维度 | 场景数 | Learn 触发数 | 关键策略 |
|------|--------|-------------|---------|
| 基础 5 场景 (S-A~S-E) | 5 | 5 | happy path,正常 BayesianUpdate |
| 上游错误 (E1-E6) | 7 | 4 (E3/E4-post/E5/E6) | **Plan 错误不 Learn**(无 segment row)+ Execute/Verify 错误正常 Learn β++ |
| 用户主动 (U1-U3) | 3 | 2 (U1/U2) | user-cancel β++ / user-accept α++ / user-modify 本轮不 Learn |
| force_plan (F1-F2) | 2 | 2 | F1 写 metadata 触发降级;F2 下次 directive 走 Plan 路径 |
| Learn 失败 (L1-L3) | 3 | 0 (走降级) | L1 silent+retry / L2 sync fallback / L3 defer next session |
| 跨+特殊 (C1+X1) | 2 | 2 | 跨 session 复用 reputation row;emit 失败不影响 Learn |
| **总计** | **22** | **15** | |

**3 个推荐决策**(用户 2026-07-07 拍板,见 §5.6.1 详细):

| 决策点 | 推荐策略 | 理由 |
|--------|---------|------|
| **上游错误 Learn 触发** | Plan 错误不 Learn / Execute 错误 Learn | Plan 阶段无 segment row,仅 audit log;Execute 阶段 segment 已存在,β++ 累计 |
| **用户主动 Learn** | user-cancel β++ / user-accept α++ / user-modify 不影响 | user 信号是真实 feedback,计入 reputation;modify 是新 directive 走新 round |
| **Learn 自身失败** | silent + 内部 retry + 最终 warn | 不阻塞 emit final;3 级降级覆盖 99% 边界 |

**Learn 节点核心边界**:

| 边界 | 说明 |
|------|------|
| **不阻塞主流程** | Learn 失败不阻塞 emit final + parent rollup,降级 silent log |
| **retry 不累计** | Decision=retry 时 α/β 不动,避免 noise |
| **child_worker 独立 row** | spawn 的子 Worker 独立 child row,parent 自身不累计 |
| **parent_rollup sum 聚合** | 父 row α/β = sum(child α/β),不重新贝叶斯 |
| **去重** | `artifact_metadata_hash` 防同一 artifact 重复 learn |
| **跨 session 持久** | reputation 跨 session 累积(同一 segment_id 长期 memory) |
| **force_plan 触发** | Beta/(Alpha+Beta) > 0.7 → BayesianAction=force_plan,下次 Observe 阶段降级到 Plan 路径(防低质量直接 fast-path) |

**Learn 节点在链路中的位置**(链路最末,但**异步**):

```
Verify (节点 4) + Decision (节点 5) 两个独立 stage
  ├─ emit final EngineEvent(主路径)   ← 用户先看到
  ├─ parent rollup 聚合(若有)
  └─ 异步触发 Learn
        ├─ per-segment Learn × N
        ├─ ParentEvidence aggregator(若有)
        └─ 落 reputation row
```

**关键设计决策**:Learn **不阻塞 emit final**。用户先看到飞书卡,reputation 后台异步更新。Learn 失败仅 silent log,不影响主路径。

**完整 Learn 节点链路 trace + Bayesian 公式 + reputation row schema**见 `decision-tree.md §8.7 + §9-§10`。

---

### 5.6.1 Learn 22 场景覆盖决策表(用户拍板 2026-07-07)

之前 §5.6 + §8.7.5 仅覆盖 5 场景 S-A~S-E,**未覆盖 Observe 节点的边缘场景**(上游错误 / 用户主动 / force_plan / Learn 失败 / 跨 session / 特殊语义)。本节列出用户 2026-07-07 拍板的 3 个 Learn 策略决策,作为 22 场景全覆盖的依据。

#### 决策 1:上游错误 Learn 触发策略

| 错误源 | Learn 触发? | 累计方向 | 理由 |
|--------|------------|---------|------|
| Plan 超时 (E1) | **NO** | n/a + audit log | Plan 失败时尚无 segment_id,reputation 无 row 可累加;仅 audit log + metric `plan_error_no_learn++` |
| Plan 失败 LLM 5xx (E2) | **NO** | n/a + audit log | 同上 |
| ctx cancel before Plan emit (E4-pre) | **NO** | n/a + audit log | ctx 早期取消,segment 未生成 |
| Worker panic (E3) | **YES** | β++ | segment 已存在,VerdictFail 是有效负信号 |
| LLM judge 超时 (E5) | **YES** | β++ | segment 已存在,SystemAnomaly 是负信号 |
| Decision map miss (E6) | **YES** | β++ | 同 E5 |
| ctx cancel after Verify emit (E4-post) | **YES** | α++ (last good) | 已 emit 的 verdict 有效,正常累计 |

#### 决策 2:用户主动行为 Learn 策略

| 用户行为 | Learn 触发? | 累计方向 | SourceID 前缀 | 理由 |
|---------|------------|---------|--------------|------|
| user-cancel (U1) | **YES** | β++ | `user_cancel:seg_a_id` | 强负信号,用户拒绝 = 该 segment 路径失效;**24h 内同 segment_id ≥ 3 次 → audit_hold**(只 audit 不 β++,防误点/恶意) |
| user-accept (U2) | **YES** | α++ (fast-track) | `user_accept:seg_a_id` | 强正信号,直接累计 α,无需等下次 trigger |
| user-modify (U3) | **NO**(本轮) | n/a | n/a | modify 是新 directive,走新 round;原 round 不再 Learn |

#### 决策 3:Learn 自身失败降级策略

| 失败模式 | 降级策略 | 是否阻塞 emit final |
|---------|---------|-------------------|
| DB 写挂 (L1) | silent + **内部 retry 3 次**(100ms 间隔)+ 最终 slog.Warn("learn_failed") | **不阻塞** |
| AsyncLearner 队列满 (L2) | **降级 Sync.Learn**(同步执行)+ log warn | 不阻塞,但 enqueue 等待 ≤ 5ms |
| session 未 Drain (L3) | **DeferToNextSession marker**,下次 session.Drain() 兜底 | 不阻塞,下次启动时 drain |

#### 决策 4:force_plan 链路 Learn 策略

| 子场景 | Learn 行为 | BayesianAction |
|--------|-----------|----------------|
| F1 force_plan 首次触发(β/(α+β) > 0.7) | enqueue 1 次 + 写 `metadata.next_observation_force_plan=true` | **force_plan** |
| F2 force_plan 下次降级(下 directive 走 Plan 路径) | enqueue 1 次,α++(Plan 路径信号更强) | alpha_bump |

#### 决策 5:跨 session + 特殊语义

| 场景 | Learn 行为 | BayesianAction |
|------|-----------|----------------|
| C1 segment_id 跨 session 复用 | cold start → BuildAdaptivePrior Beta(5,3) → α=5+1=6 | alpha_bump |
| X1 emit 失败 + Learn 已 enqueue | Learn 已触发,emit 失败独立 metric `learn_emitted_after_emit_failure=true`,reputation 正常 α++ | alpha_bump |

**总览**:22 场景 = 15 触发 Learn + 7 降级(audit/no-change)。Learn 节点在 D7 全链路的每个分支都有明确定义,**无歧义、无静默失败**。

**完整 22 行表 + 维度分类 + 触发统计**见 `decision-tree.md §8.7.10`。
**12 条新 AC**(AC43-AC54)覆盖 22 场景的验收见 `spec_delta.md §3.12`。
**5 个新 T 点**(T60-T64)覆盖 22 场景的实现见 `tasks.md P0-11 扩展`。

---

### 5.6.2 Plan 节点 26 用例场景全覆盖(扩展自 §8.5,完整表见 `decision-tree.md §8.8`)

之前 §8.5 仅覆盖 5 场景 S-A~S-E,Plan 节点的 **21 个边缘场景**(Plan LLM 错误 / 字段异常 / Parse Reject / fast-path 命中 / force_plan 链路)均未明示 Execute / Decision / Learn 的具体行为。本 Change 扩展至 **26 场景 × 6 维度**:

| 维度 | 场景数 | 关键策略 |
|------|--------|---------|
| 基础 5 场景 (P22-P26) | 5 | happy path,正常 6 节点流水线 |
| Plan LLM 错误 (P1-P3) | 3 | **Decision 新增 plan_error 路径**(10 行 → 11 行映射表)+ ItemPipelineRunner emit abort + NO Learn |
| 字段异常 (P4-P11) | 8 | **重试 Plan LLM ≤ 2 次**(§9 feedback_loop 3 子类)+ **降级到原 4-channel Plan 路径**(Plan.IntentSegmentSet/DAG 全 nil,PlanKind 决定 channel) |
| Parse Reject (P12-P17) | 6 | 同 P4-P11 重试 + 降级 |
| fast-path 命中 (P18-P19) | 2 | Plan 节点 short-circuit,NO Execute / NO Decision |
| force_plan (P20-P21) | 2 | Plan prompt 注入 "强制 Required AC[] ≥ 1" + PriorityHint,Execute/Decision/Learn 与默认 Plan 一致 |
| **总计** | **26** | |

**4 个推荐决策**(用户 2026-07-07 拍板,见 §5.7.1 详细):

| 决策点 | 推荐策略 | 理由 |
|--------|---------|------|
| **Plan LLM 错误处理** | ItemPipelineRunner 直接 emit abort + Decision plan_error 新路径 | Plan error 无 segment row,与 Learn §5.6.1 决策一致(Plan 错误不 Learn) |
| **Parse Reject 重试与降级** | 重试 Plan LLM ≤ 2 次 + 降级到原 4-channel Plan 路径(IntentSegmentSet/DAG 全 nil) | 重试给 LLM 修正机会,降级保 happy path 完整性 |
| **force_plan Plan 差异** | 强制 Required AC[] ≥ 1 + PriorityHint 注入 prompt | 防止下次又走 fast-path,Plan 路径保持一致 |
| **S-E Decision 边界** | fast-path 部分跳过 Decision,parent rollup 统一决策 A accept | 保持 fast-path 优势,parent rollup 提供决策完整性 |

---

### 5.7.1 Plan 26 场景覆盖决策表(用户拍板 2026-07-07)

之前 §8.5 (Plan LLM 输出) + §8.6 (Decision 5 路径) + §8.7 (Learn 22 场景) 仅覆盖 happy path 5 场景,**Plan 节点的 21 个边缘场景**下 Execute / Decision / Learn 的具体行为未明示。本节列出用户 2026-07-07 拍板的 4 个 Plan 节点边缘场景处理策略,作为 26 场景全覆盖的依据。

#### 决策 1:Plan LLM 调用错误(P1-P3)处理策略

| 错误类型 | Execute | Decision | Learn | emit final |
|---------|---------|----------|-------|-----------|
| **P1** PlanLLMCallTimeout (>30s) | NO Execute | **Decision 新增 path:plan_error → E human_review** | NO Learn + audit `plan_error_no_learn++` | ItemPipelineRunner 直接 emit abort + 飞书卡 "❌ Plan 阶段超时" |
| **P2** PlanLLMCallError (5xx) | NO Execute | 同 P1 (plan_error → E human_review) | NO Learn + audit | ItemPipelineRunner emit abort + 飞书卡 "❌ Plan LLM 5xx" |
| **P3** PlanLLMCallPartialResponse (JSON 截断) | NO Execute | 同 P1 (plan_error → E human_review) | NO Learn + audit | ItemPipelineRunner emit abort + 飞书卡 "❌ Plan 输出不完整" |

**Decision 映射表扩展**:现有 10 行(Verdict 4 态 × Other Conditions + D parent_rollup)扩展为 **11 行**,新增第 11 行:
```
| plan_error | (Plan LLM 调用层失败) | E human_review | 飞书卡"❌" + emit abort | decision.kind=plan_error |
```

**关键设计**:Plan error 时无 Verdict,Decision 节点通过**单独的 plan_error 入口**(由 ItemPipelineRunner 调用,不经 10 行 Verdict-based 映射表)直接触发 E human_review。这与现有 Verdict-based 决策正交,避免污染 10 行语义。

#### 决策 2:Parse Reject 重试与降级策略(P4-P17)

| 字段异常类型 | PlanAcceptanceContractBuilder.Build | 重试 | 降级路径 |
|------------|-------------------------------------|------|---------|
| **P4** Plan.IntentSegmentSet = nil 但 DAG 非空(或反过来,字段不对称) | 拒绝(ParseReject) | ≤ 2 次重试 Plan LLM + feedback CompactJSON | 降级到原 4-channel 路径(Plan.IntentSegmentSet/DAG 全 nil) + PlanAcceptanceContractBuilder.Build 返回 ErrPlanFieldAsymmetry |
| **P5** PriorityHint 提示但 PlanKind ≠ Scenario/Exploration | 通过(降级路径) | 0 次(直接降级) | 走原 PlanKind 通道 + fallback NumericRange Verify |
| **P6** DAG 0 nodes | 拒绝(ParseReject) | ≤ 2 次 + 降级 | 降级同 P4 |
| **P7** AC[] CheckKind 越界 | 拒绝(ParseReject) | ≤ 2 次 + 降级 | 降级同 P5 |
| **P8** AC[] Required=0 | 拒绝(ParseReject) | ≤ 2 次 + 降级 | 降级同 P5 |
| **P9** rationale 缺失 | **通过**(可选字段) | 0 次 | 正常路径(Execute/Decision/Learn 不变)+ Learn metadata 缺 rationale 字段 + audit log 警告 |
| **P10** priorities 越界 | 拒绝(ParseReject) | ≤ 2 次 + 降级 | 降级同 P5 |
| **P11** segments ID 缺失/重复 | 拒绝(ParseReject) | ≤ 2 次 + 降级 | 降级同 P5 |
| **P12-P17** 6 类 Parse Reject(RepeatCycle / TooManyNodes / AC 重复 ID / DAG 边端点缺失 / node 重复 ID / 优先级映射缺失) | 拒绝(ParseReject) | ≤ 2 次 + 降级 | 降级同 P5 |

**重试上限 ≤ 2 次的依据**:与 spec_delta §9 feedback_loop 3 子类(RejectCycle / RejectTooManyNodes / RejectACDuplicateID)的重试上限一致,避免 LLM 循环死锁。

**降级路径(原 4-channel Plan 路径)**:
- Execute:按 PlanKind 路由到 4 个 channel 之一(Commit / Protocol / Scenario / Exploration);Plan 字段(IntentSegmentSet/DAG)全 nil
- Verify:跳过 PerCriterion,只跑 fallback NumericRange 检查(基于 artifact 内容长度 + 关键词,Phase 4 已实现)
- Decision:走简化 Decision A accept(无 AC[],VerdictKind 由 fallback Verify 给出,Phase 4 已实现)
- Learn:正常 Learn α++,SourceID 加前缀 `plan_legacy:seg_a_id` 区分(plan 降级路径标记)

#### 决策 3:force_plan Plan 路径差异(P20-P21)

| 差异点 | 默认 Plan 路径 | force_plan Plan 路径 |
|--------|--------------|---------------------|
| **Plan prompt appendix** | 基础 3-shot example | 基础 + **"该 segment_id 历史负信号较多(β/(α+β) > 0.7),强制每节点 ≥ 1 Required AC[]"** |
| **PriorityHint** | 由 Plan LLM 自由给 | **PriorityHint 注入**:"高 priority 优先 EmitPartial,低 priority 后跑" |
| **Execute** | RunPlanDAG 4 worker | 同(无差异) |
| **Verify** | PerCriterion + CustomLLMJudge | 同(无差异) |
| **Decision** | 11 行映射表 | 同(无差异) |
| **Learn** | 正常 Learn α++ | 同(无差异,但 Plan 路径信号更强,reputation 累积更快) |

**关键设计**:force_plan 仅修改 Plan 节点的 prompt 注入,**不修改 Execute / Decision / Learn 节点**。这样保持 D7 6 节点流水线的对称性,Plan 节点的特殊化逻辑集中在一个地方。

#### 决策 4:S-E Decision 边界(P-S-E 决策 4)

| S-E 子段 | Decision 触发? | decision.kind |
|---------|--------------|---------------|
| **seg_a (fast-path)** | **NO**(跳过 Decision 节点) | (无 decision) |
| **seg_b (verified)** | 走 10 行 Verdict-based 映射表 | A accept |
| **parent rollup (聚合 seg_a + seg_b)** | 走 10 行 Verdict-based 映射表 | **A accept (整体 1 个)** |

**S-E 整体 decision.kind = accept × 1**(parent rollup 统一决策,不是 accept × 2)。

**关键设计**:fast-path 部分的优势保留(无需 Decision 节点,延迟 < 5ms),但 parent rollup 提供决策完整性(用户看到 1 个统一 decision 而非分散的多个)。Learn 节点按 §8.7.10 per-segment 累计 α++,ParentEvidence aggregator 在 rollup 时 sum。

#### 决策 5:Plan → Learn 联动(与 §5.6.1 Learn 22 场景一致性)

| Plan 节点输出 | Learn 触发 | 来源章节 |
|-------------|-----------|---------|
| Plan LLM 错误 (P1-P3) | NO Learn + audit | §5.6.1 决策 1 |
| Plan 字段异常降级成功 (P4-P17 降级 P5) | 正常 Learn α++ | §5.6.1 决策 2 |
| Plan rationale 缺失 (P9) | 正常 Learn no_change + audit | §5.6.1 决策 2 |
| Plan fast-path 命中 (P18-P19) | 正常 Learn α++ per-segment | §5.6.1 维度 1 基础 5 场景 |
| Plan force_plan 路径 (P20-P21) | 正常 Learn α++ (信号更强) | §5.6.1 决策 4 force_plan 链路 |

**总览**:26 场景 = 13 触发 Learn + 13 降级(audit/no-change)。Plan 节点在 D7 全链路的每个输出状态下,Execute / Decision / Learn 都有明确定义,**无歧义、无静默失败**。

**完整 26 行表 + 维度分类 + 触发统计**见 `decision-tree.md §8.8`。
**12 条新 AC**(AC55-AC66)+ 1 条 Decision 10 行扩展(AC67)覆盖 26 场景的验收见 `spec_delta.md §3.13-§3.14`。
**7 个新 T 点**(T66-T72)覆盖 26 场景的实现见 `tasks.md P0-12`。

---

## 6. Risks(细化)

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| LLM 提议的 PlanDAG 含环 | Medium | High(死锁) | validateDAG() 用 DFS 检测;含环 → PlanParseReject.RejectCycle + 重试上限 2 |
| LLM 提议的 PlanDAG 节点数 > MaxFanOut=8 | Medium | Medium(LLM Gateway 限流) | validateDAG() 节点数 check;超 → PlanParseReject.RejectTooManyNodes |
| Stream emit 重复发卡 | Medium | High(用户看到重复卡片) | idempotency key (session_id, segment_id) + InMemoryEmitDedup |
| 并发 4 worker 抢占同一 SessionWorkItem 状态 | Medium | High(并发写冲突) | WorkItem state 加 RWMutex;child 独立锁粒度 = WorkItemID |
| Reputation per-segment 累积噪声 | Low | Low(Low statistical signal) | v1 不修;v2 加 BayesianHierarchical pooling segment α/β |
| Plan LLM 不返回 DAG 字段 | High(v1 早期) | Medium(降级到原 4-channel Plan) | `Plan.DAG == nil` && `Plan.IntentSegmentSet != nil` → 触发 PlanLLMOutputInvalidJSON;→ Decision plan_error 第 11 行映射 → emit abort + NO Learn;降级路径:Plan.DAG 与 Plan.IntentSegmentSet 同时为 nil(走原 Phase 2 PR-B1 4-channel 路径) |
| Stream emit partial 顺序与 parent 最终答案不一致 | Medium | Medium(用户困惑) | parent rollup LockFlag(等所有 partial 确认 ack)再 emit 最终 |
| OpenSpec lite-mode delta 太大 | Low | Low | 复用 DM-20260630-003 lite-mode pattern, spec_delta 控制在 ≤ 200 行 |

---

## 7. Rollout

- **PR 拆分**:7 PR(PR-A1 grammar/Plan fields + IntentSegment + PlanDAG type + DAG validator;PR-A2 AC contract + LLM IO 合并;PR-B WaveScheduler DAG executor + 4 worker pool;PR-C streaming emit + idempotency;PR-D config flag + e2e LP-3+LP-4 test + verify-archive;PR-E Learn+22 场景;PR-F Plan+26 场景)
- **Feature flag**:`devrix.d7.dag_executor.enabled`(bool,默认 off → first 5% → 100% across 2 weeks)
- **数据迁移**:无
- **回滚预案**:feature flag off → 自动 fallback 到原 Phase 2 PR-B1 Plan 路径(`Plan.IntentSegmentSet/DAG` 全 nil → 4-channel);reputation 数据无迁移成本
- **灰度策略**:内部 feishu → DM 群 → 5% 用户随机 → 100%

---

## 8. 待决项 / V2 Backlog

### 8.1 Open Questions(S3 Design 阶段必须解决)

| # | 问题 | 决策选项 | 解决线索 |
|---|------|----------|----------|
| 1 | IntentSegment 切分失败时(LLM 返回模糊边界)如何降级? | A) 切 1 个 IntentSegment(整个 directive 当 1 segment);B) 让 Observe LLM 重试 1 次;C) Plan 节点 fallback 原 4-channel Plan(IntentSegmentSet/DAG 全 nil) | A 简单,无重试延迟;B 体验更准;**倾向 A**(v1 简化)+ 观测指标驱动 v2 |
| 2 | validateDAG() 拒绝后 Plan LLM 看到的错误 feedback 如何格式化才不循环? | A) 纯人类可读文本;B) 机器可读 JSON enum + LLM 友好解释混合 | spec_delta §9 feedback_loop 已定义 3 子类,**倾向 B**(JSON + 解释)避免循环 |
| 3 | WaveScheduler 4-worker pool 是 fixed 还是 lazy spawn? | A) fixed 4 永久占用;B) lazy 按 ready 节点数动态 fork | A 简单无 lifecycle 问题;B 资源省但要处理 fork/join;**v1 倾向 A fixed**(spec_delta §6 硬上限 4)+ v2 metrics-driven 自适应 |
| 4 | parent rollup 的 idempotency key 与普通 segment 共享 dedup 表吗? | A) 共享 dedup 表,B 区分;B) 用不同前缀(如 "rollup:" / "seg:")物理隔离 | A 简单但有概率误命中;B 更安全但要双份 dedup;**倾向 B**(不同 keyspace)+ spec_delta §7 适配 |
| 5 | (跨段)broadcast IntentKind(IntentOrchestrate/IntentVerify)是否需要从 child 升级到 segment? | 留待后续 Change(波及 IntentClassifier 接口) | v1 不做;**留 v2** |
| 6 | Learn 节点 22 场景全覆盖:上游错误(Plan/Execute/Verify)/用户主动(accept/cancel/modify)/force_plan 链路/Learn 自身失败/跨 session + 特殊语义如何处理? | **2026-07-07 已拍板**(详见 §5.6.1) | Plan 错误不 Learn / Execute 错误 Learn;user-cancel β++ / user-accept α++ / user-modify 不 Learn;Learn 失败 silent+retry/sync/defer 3 级降级 |
| 7 | Plan 节点 26 用例场景全覆盖:Plan LLM 错误(P1-P3)/字段异常(P4-P11)/Parse Reject(P12-P17)/fast-path 命中(P18-P19)/force_plan 链路(P20-P21)如何处理 Execute/Decision/Learn? | **2026-07-07 已拍板**(详见 §5.7.1) | Plan 错误 Decision 新增 plan_error 路径 + emit abort + NO Learn;Parse Reject 重试 ≤ 2 次 + 降级旧 Plan;force_plan Plan 注入 "强制 Required AC[] ≥ 1";S-E fast-path 部分跳过 Decision,parent rollup 统一决策 |

**S3-Gate 评审前必须 solve 1-4 + 6 + 7 的明确决策**(5 已拍板),#5 跨域接口改造立项 `devrix-d7-intent-classifier-segment-aware`,#6 在 §5.6.1 已展开 22 场景完整决策,#7 在 §5.7.1 已展开 26 场景完整决策。

### 8.2 v2 Backlog(显式推到 v2 的升级项)

v1 范围内**不**做,留 v2 metrics 驱动立项。每项含:动机 / 触发条件 / 预期效果 / 依赖前置。

#### v2-1: 跨 segment DataEdge 数据依赖(`depends_on_outputs` 解析)

- **v1 状态**:`DataEdge.DependsOnOutputs []string` 字段保留但不使用(spec_delta §3 note)
- **v2 动机**:用户给出 "查 devrix 架构,然后基于结果分析 d2 风险" 串行依赖场景,v1 必须 1-2 段走串行,中间上下文丢失
- **v2 触发条件**:v1 灰度后,≥ 5% directive 含"基于 X 然后 Y"语言模式(LLM judge 检测)
- **v2 效果**:Plan LLM 显式声明 edge.DependsOnOutputs=["n_a.summary"],RunPlanDAG 等前置完成才启动,跨段数据 sub-second 保留
- **v2 前置依赖**:D7 TaskContract(已立项)+ v1 灰度数据累计 ≥ 2 周

#### v2-2: 流式 streaming card append(IM streaming API 集成)

- **v1 状态**:S-E 走 "partial card + final card" 2 卡模式,partial 内容完成后不更新
- **v2 动机**:Worker 输出长文本(>1KB)时用户希望"边写边读",而 v1 只能等整个 child 完成才发 partial
- **v2 触发条件**:v1 灰度后,≥ 10% Worker 输出 > 1KB,且 IM adapter 支持 streaming(飞书 v2 API 已开放)
- **v2 效果**:Worker 边生成边 append partial card 同一条,UX 接近 ChatGPT
- **v2 前置依赖**:D1 飞书 streaming API 评估 + Verify 节点需重新定义"中途 partial 不算完成"

#### v2-3: 自适应 MaxFanOut(metrics-driven)

- **v1 状态**:`MaxFanOut = 8`(validateDAG 硬上限),WaveScheduler `硬上限 4`
- **v2 动机**:不同 directive 最佳并行度不同(简单 deterministic 2-segment vs 复杂 explore 3-segment),v1 一刀切 4 worker
- **v2 触发条件**:v1 灰度后,LLM Gateway P95 latency + P99 error rate 指标稳定 ≥ 2 周
- **v2 效果**:WaveScheduler 根据 LLM Gateway 实时负载动态调整 2-8 worker
- **v2 前置依赖**:D3 LLM Gateway 暴露实时 metrics + v1 灰度数据

#### v2-4: critical path 自动推导

- **v1 状态**:priority 由 Plan LLM 显式给(plan_dag.Priorities map)
- **v2 动机**:LLM 给 priority 不一定准(可能被 prompt injection 抬高);静态分析 DAG + Worker 历史时长 → critical path 自动算
- **v2 触发条件**:v1 灰度后,priority 误判(LLM 给高但实际简单)≥ 10% 节点
- **v2 效果**:critical path 节点 worker slot 抢占更准,总延迟 P95 ↓ 10-20%
- **v2 前置依赖**:D5 observability 暴露 Worker 历史 P50 时长 + v1 priority 误判统计

#### v2-5: hierarchical Bayesian pooling(per-segment reputation 噪声收敛)

- **v1 状态**:每个 segment 独立 α/β,ParentEvidence sum 聚合(learner.go 改造点)
- **v2 动机**:新 segment 冷启动 α/β 噪声大,v1 sum 后容易偏向噪声最大那段
- **v2 触发条件**:v1 灰度后,reputation row 中 P5 (α+β) < 10 的 row ≥ 30%(噪声主导)
- **v2 效果**:partial pooling → segment α/β 在共享 prior 上做 Bayesian shrinkage,新 segment 借力历史 segment 数据
- **v2 前置依赖**:D7 Reputation 框架支持 hierarchical prior(已部分支持) + v1 数据

#### v2 Backlog 优先级排序(灰度数据驱动)

v1 灰度 2 周后,根据指标数据按 v2-4(快赢)→ v2-1(刚需)→ v2-3(运维)→ v2-5(质量)→ v2-2(UX)顺序立项,每项独立 Change。

---
