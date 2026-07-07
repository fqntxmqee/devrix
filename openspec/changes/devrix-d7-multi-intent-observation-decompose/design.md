# Design: D7 multi-intent observation decompose + Plan DAG + parallel Execute

**Change ID:** `devrix-d7-multi-intent-observation-decompose`

---

## 1. Architecture

### 1.1 系统图

```
                                 ┌──────────────────────────────────┐
   User directive                 │            D7 Observe            │
  "1+1=几? + 查 devrix 架构"   ─→ │                                  │
                                 │  IntentSegmenter                 │
                                 │   ├ LLMIntentSegmenter           │
                                 │   └ RuleBasedSegmenter (fallback)│
                                 │  Output:                         │
                                 │   IntentSegmentSet{              │
                                 │     Segments: [{seg_a: "1+1=几?", │
                                 │                  deterministic}, │
                                 │                 {seg_b: "查       │
                                 │                  devrix 架构",  │
                                 │                  explore}],     │
                                 │   }                              │
                                 └──────────┬───────────────────────┘
                                            ↓
                                 ┌──────────────────────────────────┐
                                 │            D7 Plan                │
                                 │                                  │
                                 │  StrategicPlanProposer (LLM)     │
                                 │   → PlanDAG{                     │
                                 │       Nodes: [{n_a, n_b}],       │
                                 │       Edges: [],                 │
                                 │       Priorities: {n_a: high,    │
                                 │                   n_b: normal}   │
                                 │     }                            │
                                 │                                  │
                                 │  validateDAG(dag)                │
                                 │   → acyclic ✓ + n_nodes ≤ 8 ✓    │
                                 │   → SegmentValidate ✓            │
                                 └──────────┬───────────────────────┘
                                            ↓
                                 ┌──────────────────────────────────┐
                                 │            D7 Execute             │
                                 │                                  │
                                 │  WaveScheduler.RunPlanDAG(dag)   │
                                 │   ├ 4 worker pool                │
                                 │   ├ ready queue (priority heap)  │
                                 │   ├ n_a (high) ── parallel ─┐    │
                                 │   └ n_b (normal) ── parallel ┤    │
                                 │                                  │
                                 │  For each running node:          │
                                 │   └ ItemPipelineRunner.Run()     │
                                 │       (each child独立 SessionWorkItem)│
                                 │                                  │
                                 │  Channel: <SegmentEmit...>       │
                                 │   → partial → IM adapter 即时发卡│
                                 │   → final   → parent rollup      │
                                 └──────────┬───────────────────────┘
                                            ↓
                                 ┌──────────────────────────────────┐
                                 │     D7 Verify (节点 4)            │
                                 │  PerCriterion executor            │
                                 │   → 4 态 Verdict                  │
                                 └──────────┬───────────────────────┘
                                            ↓
                                 ┌──────────────────────────────────┐
                                 │     D7 Decision (节点 5)          │
                                 │  11 行静态映射表 (< 5ms, 0 LLM)   │
                                 │   → 5 路径决策 (A/B/C/D/E)       │
                                 └──────────┬───────────────────────┘
                                            ↓
                                 ┌──────────────────────────────────┐
                                 │     D7 Learn (节点 6, 异步)      │
                                 │  - per-segment BayesianUpdate    │
                                 │  - ParentEvidence aggregator     │
                                 │  - force_plan 触发(β/总>0.7)    │
                                 │  - 不阻塞 emit final              │
                                 │                                  │
                                 │  Reputation DB:                  │
                                 │   每个 segment 一个 reputation row│
                                 │   parent_rollup row = sum(child) │
                                 └──────────────────────────────────┘
```

### 1.2 数据流

```
UserDirective (string)
  ↓ IntentSegmenter.Segment(ctx, observeReq)
IntentSegmentSet
  ↓ UncertaintyReport merges
ObservationReport + IntentSegmentSet
  ↓ StrategicPlanProposer.ProposeStrategicPlan
PlanDAG + Children (legacy)
  ↓ validateDAG(plan.DAG)  [no-op if legacy]
Plan
  ↓ WaveScheduler.RunPlanDAG(ctx, plan)
  ↓ for each node in topological order, ready at parallel slot:
     ItemPipelineRunner.Run(childWorkItem)
       ↓ child Round → opts.Emit EngineEvent
       ↓ opts.Emit idempotency key (session_id, segment_id)
  ↓ parent rollup round → opts.Emit final EngineEvent
[per-child] Learn(ctx, LearnRequest{WorkItemID: child.ID, Verdict})
[rollup]   Learn(ctx, LearnRequest{WorkItemID: parent.ID, ParentEvidence: aggregate})
```

### 1.3 控制流(D7 6 节点流水线)

| 节点 | 输入 | 产出 | 实现 |
|------|------|------|------|
| **1. Observe** | User directive | IntentSegmentSet + ObservationReport | LlmIntentSegmenter (timeout 800ms) → fallback RuleBasedSegmenter |
| **2. Plan** | Observe 输出 | PlanDAG + PlanLLMOutput(含 AC[])+ rationale | StrategicPlanProposer + validateDAG + 3-shot example |
| **3. Execute** | PlanDAG | Artifact(per-segment)+ partial emit | WaveScheduler.RunPlanDAG + 4 worker hard cap |
| **4. Verify** | Artifact + AC[] | PerCriterionVerdict[] + VerdictKind | PerCriterion executor(本地机械 + CustomLLMJudge ≤ 3) |
| **5. Decision** | Verdict + AC[] + RoundMeta | Decision{Kind, Reason, NextWorkItemSpec?} | 11 行静态映射表(10 Verdict-based + 1 plan_error,< 5ms, 0 LLM) |
| **6. Learn** | Decision + Verdict + ArtifactHash | BayesianUpdate + reputation row | AsyncLearner(异步,enqueue < 1ms) |

**链路**:1→2→3→4→5→6,Decision A 路径终止于 Learn(emit final),B/C 路径回到 1-4 循环,D 触发 parent rollup 节点,E 飞书 abort + 仍进 Learn。

**Stream emit**:emitPartial(per child round) + emitFinal(rollup round) by idempotency key,**与 Learn 异步解耦**。

### 1.4 边界

**负责**:
- 多意图 directive 的 Observe 节点切分(IntentSegmenter)
- Plan 节点的 DAG 生成 + 验证
- Execute 节点的拓扑并行执行(4 worker hard cap)
- 流式 emit(idempotency 保证不重复)
- per-segment reputation 累积

**不负责**:
- v1 不引入跨 segment DataDep(`depends_on_outputs` 字段保留但不使用)
- v1 不引入 IM streaming(走 2 卡:partial + final)
- v1 不引入 critical path 自动推导(priority 由 LLM 显式给)
- v1 不引入跨 session segment 复用

---

## 2. Implementation Notes

### 2.1 模块 1:IntentSegment grammar

- 修改文件:`orchtypes/intent_segment.go` (NEW), `plan/spawn_policy.go`
- 关键 API:
  ```go
  type IntentSegment struct {
      ID         string
      Text       string
      IntentKind string  // "deterministic" | "explore" | "commit" | "analyze"
      Priority   int     // 0-100, default 50
      Confidence float64 // 0-1, ≥0.85 才允许走 fast-path
  }
  type IntentSegmentSet struct {
      Segments        []IntentSegment
      SourceDirective string
      DetectedAt      time.Time
  }
  ```
  `plan.SpawnPolicy` 新增 `DecomposeByIntentSegments IntentSegmentSet` variant (string-tagged union)。
- 数据结构:`orchtypes.NewIntentSegment(...)` + `Validate()` + `MarshalJSON()`。
- 错误处理:`ErrIntentSegmentInvalid`,LLM 返回 0 segment 时降级到 1-element set(整个 directive = 1 segment)。
- 性能影响:单 struct 序列化 ~微秒级,可忽略。

### 2.2 模块 2:IntentSegmenter (Observe)

- 修改文件:`sessionorchestrator/intent_segmenter.go` (NEW)
- 关键 API:
  ```go
  type IntentSegmenter interface {
      Segment(ctx context.Context, req ObserveRequest) (IntentSegmentSet, error)
  }
  type LLMIntentSegmenter struct { ... uses D2 ctx + D3 llm ... }
  type RuleBasedSegmenter struct { ... regex ... }
  type SegmenterDispatcher struct {
      LLM   IntentSegmenter
      Rule  IntentSegmenter
      LLMTimeout time.Duration  // 800ms default
  }
  ```
- 数据结构:`IntentSegment` (按上述)。
- 错误处理:LLM 超时/失败 → fallback RuleBased;RuleBased 也失败(零 regex 命中)→ 1-element set。
- 性能影响:LLM 1 次额外调用 ~300-600ms(已在 Observe 节点内并行 LLM 调用窗口)。

### 2.3 模块 3:PlanDAG + DAG validator

- 修改文件:`plan/plan_dag.go` (NEW), `plan/dag_validator.go` (NEW)
- 关键 API:
  ```go
  type PlanNode struct {
      ID                    string
      SegmentID             string    // links to IntentSegment.ID
      WorkerHint            string    // "wave" | "agent" | "tool"
      ExpectedArtifactTags  []string  // ["final_text", "evidence"]
  }
  type DataEdge struct {
      From              string
      To                string
      DependsOnOutputs  []string  // v1 保留字段,使用为空
  }
  type PlanDAG struct {
      Nodes          []PlanNode
      Edges          []DataEdge
      Priorities     map[string]int
      MaxParallelism int  // v1 ignored;hard 4 在 WaveScheduler 控制
  }
  type DAGValidator interface {
      Validate(dag PlanDAG, opts ValidateOpts) error
  }
  ```
- 数据结构:errors 含 Cycle / TooManyNodes / DuplicateNode / DanglingEdge。
- 错误处理:validateDAG 失败 → PlanParseReject JSON + PlanProposer 重试(上限 2)。
- 性能影响:DAG 校验 < 1ms (DFS + 节点数 check)。

### 2.4 模块 4:WaveScheduler DAG executor + 4 worker pool

- 修改文件:`wavescheduler/dag_executor.go` (NEW)
- 关键 API:
  ```go
  type DAGExecutor interface {
      RunPlanDAG(ctx context.Context, planDAG PlanDAG) (<-chan SegmentEmit, error)
  }
  type SegmentEmit struct {
      SegmentID string
      WorkItemID string
      Round     *WorkItemPipelineRound
      IsFinal   bool  // parent rollup 时为 true
  }
  ```
- 数据结构:
  - 4 worker pool = `chan struct{}` semaphore
  - ready queue = `container/heap` priority heap over (priority desc, nodeID asc)
- 错误处理:
  - 单 child error → cancel 未启动 sibling + drain emit
  - context canceled → 终止所有 worker
  - MaxFanOut 超过 → validateDAG 已 reject,这里 redundant check
- 性能影响:channel + heap 调度 < 1ms overhead per child launch。

### 2.5 模块 5:ItemPipelineRunner.Run() streaming + per-segment Learn

- 修改文件:`sessionorchestrator/item_pipeline.go` + `mups/learn/learner.go`
- 关键 API:
  - `ItemPipelineRunOpts.Emit(segmentID, round, opts.EmitOpts{IsPartial: bool})` 已存在,扩展 emit 路径
  - `LearnRequest{WorkItemID, Verdict, Plan, Artifact, Observations}` 新加 WorkItemID 字段
  - `learn.ParentEvidence{SumAlpha, SumBeta, ChildCount}`
- 数据结构:`opts.EmitOpts{IdempotencyKey string, IsPartial bool}`。
- 错误处理:
  - stream emit 重复调 → idempotency table hit → noop (slog.Debug)
  - Learn 失败 per-child 不影响其他 child,continue propagation
- 性能影响:emit channel 加 idempotency table,O(1) lookup。

### 2.6 模块 6:D1 飞书流式 + idempotency

- 修改文件:`communication/feishu/streaming.go` (NEW)
- 关键 API:
  ```go
  type StreamingEmitter interface {
      EmitPartial(ctx context.Context, key IdempotencyKey, content string) error
      EmitFinal(ctx context.Context, key IdempotencyKey, content string) error
  }
  ```
- 数据结构:`map[IdempotencyKey]bool` in-memory dedup table,SESSION 生命周期内有效。
- 错误处理:飞书 API 失败 → fallback 到现有 UpdateCard(已有)。
- 性能影响:key 长度 ~60 bytes,dedup map < 1ms lookup。

### 2.7 模块 7:Config flag + e2e LP-3

- 修改文件:`bootstrap/config.go` + `scripts/verify-archive.sh`
- 关键 API:`devrix.d7.dag_executor.enabled bool`(default false)+ e2e test in `scripts/eval/`
- 数据结构:`config.D7DAGExecutorConfig{Enabled bool}`。
- 错误处理:flag 关闭时直接走旧 Plan + 顺序 await 路径。
- 性能影响:零(grep check)。

### 2.8 模块 8:PlanLLMIO(Plan ↔ LLM 输入输出契约)

- 修改文件:`plan/llm_io.go` (NEW) + `contextengine/i18n/format_hints_mups.go` (MODIFIED,Plan appendix v1)
- 关键 API:
  ```go
  type PlanLLMInput struct {
      Directive        string
      Segments         orchtypes.IntentSegmentSet
      PriorParseReject *PlanParseReject  // 上轮 feedback,首轮 nil
  }
  type PlanLLMOutput struct {
      DAG                 PlanDAG
      AcceptanceCriteria  []AcceptanceCriterion  // 每个 node 至少 1 Required
      Rationale           string
  }
  type PlanParseReject struct {
      Kind    PlanRejectKind  // RejectCycle | RejectTooManyNodes | RejectACDuplicateID
      Detail  string          // 人类可读解释
      Context json.RawMessage // 机器可读(cycle_path / node_count / duplicate_ids)
  }
  type PlanRejectKind string
  const (
      RejectCycle          PlanRejectKind = "cycle"
      RejectTooManyNodes   PlanRejectKind = "too_many_nodes"
      RejectACDuplicateID  PlanRejectKind = "ac_duplicate_id"
  )
  ```
- 数据结构:
  - 输入 schema:`plan_llm_input_v1.json` — i18n prompt appendix 强制声明
  - 输出 schema:`plan_dag_v1.json` + `plan_ac_v1.json` — LLM 严格按 schema 产出
  - 3-shot example:全 deterministic / 全 uncertain / 混合;每个 example 必须含 AC[]
- 错误处理:
  - 解析失败 → `ErrPlanLLMOutputInvalidJSON` → 不重试,降级旧 Plan
  - 缺 DAG → `ErrPlanLLMOutputMissingDAG`
  - 缺 AC → `ErrPlanLLMOutputMissingAC` + 提示"每个 node 至少 1 Required"
  - validateDAG 失败 → `PlanParseReject` + Plan LLM 重试 ≤ 2 次
  - 累计耗时 > 4s → `ErrPlanLLMIOBudgetExceeded` → 降级
- **错误码注册 (M4 修复,2026-07-07)**:所有本 Change 新增错误(共 ~15 个,见上 §2.8 / §2.9 / §2.10 / §2.11 / §2.12 / §2.14 / §2.17)必须按现有 SentinelError 模式注册到 `internal/shared/errors/` 目录的 `orchestration.go`(NEW,与 `llm.go` / `multiagent.go` 同级),具体:
  - `errors.New("d7 orchestration xxx")` 形式定义 sentinel
  - `CodeD7Xxx = "ORC_7XXX"` 常量对应(7xxx 段是 D7 域前缀)
  - 与 APICodeProvider duck-typing 集成(IM 差异化文案 + HTTP status 映射复用 `devrix-api-error-classification` S7_Archived DM-20260628-001 的 7 类闭集模式)
- 性能影响:3-shot example ~3KB prompt,单次 Plan LLM 调用延迟 +100-200ms。

### 2.9 模块 9:VerifyLLMIO(Verify ↔ LLM 输入输出契约)

- 修改文件:`verify/llm_io.go` (NEW) + `contextengine/i18n/format_hints_mups.go` (MODIFIED,Verify appendix v1)
- 关键 API:
  ```go
  type VerifyLLMInput struct {
      ArtifactSummary   string
      ArtifactMetadata  map[string]any
      Criteria          []AcceptanceCriterion
      PlanRationale     string  // 辅助 LLM judge 理解 AC 意图
  }
  type VerifyLLMOutput struct {
      PerCriterionVerdicts []PerCriterionVerdict  // 顺序对齐 criteria
      OverallVerdict       VerdictKind      // Pass | Partial | Fail | Indeterminate
      Evidence             string
  }
  type PerCriterionVerdict struct {
      CriterionID string
      Outcome     CriterionOutcome  // Pass | Fail | Skipped | Error
      Evidence    string
      Error       string
  }
  // VerdictKind 复用 internal/shared/types/verdict.go 已定义的 uint8 枚举
  // (VerdictPass=0 / VerdictPartial=1 / VerdictIndeterminate=2 / VerdictFail=3),
  // wire 格式 "pass" / "partial" / "indeterminate" / "fail"。
  // 这里不再重复定义,直接 import 使用。
  ```
- 数据结构:
  - 输入 schema:`verify_llm_input_v1.json` — i18n prompt appendix 强制
  - 输出 schema:`per_criterion_v1.json` — 顺序对齐校验
  - 2-shot example:全 Pass / 含 Fail 路径
- 错误处理:
  - `len(verdicts) != len(criteria)` → `ErrVerifyLLMOutputMismatchCount` → 重试 ≤ 1 次
  - CustomLLMJudge 数量 > 3 → `ErrVerifyLLMJudgeBudgetExceeded` + 截断 + log warning
  - LLM 调用失败 → 退化为 `Outcome=Error` + VerdictIndeterminate
- 性能影响:
  - 短路径:0 CustomLLMJudge → 本地机械 CheckKind 执行,< 50ms
  - 长路径:N CustomLLMJudge → 串行调用 D3 llmgateway,延迟 = N × 1s

### 2.10 模块 10:AcceptanceCriterion Grammar(Plan ↔ Verify 契约桥梁)

- 修改文件:`plan/acceptance_criteria.go` (NEW) + `verify/per_criterion.go` (NEW)
- 关键 API:
  ```go
  type AcceptanceCriterion struct {
      ID          string                  // 格式 "AC-{node_id}.{n}"
      Description string
      CheckKind   CheckKind               // 见下 enum
      CheckArgs   map[string]any          // per CheckKind 异构
      Severity    CriterionSeverity       // Required | Preferred
      Rationale   string
  }
  type CheckKind string
  const (
      CheckContainsString   CheckKind = "contains_string"
      CheckNotContainsString CheckKind = "not_contains_string"
      CheckMentionsAll      CheckKind = "mentions_all"
      CheckMentionsAny      CheckKind = "mentions_any"
      CheckNumericRange     CheckKind = "numeric_range"
      CheckLengthRange      CheckKind = "length_range"
      CheckJSONPath         CheckKind = "json_path"
      CheckCustomLLMJudge   CheckKind = "custom_llm_judge"
  )
  type CriterionSeverity string
  const (
      SeverityRequired   CriterionSeverity = "required"
      SeverityPreferred  CriterionSeverity = "preferred"
  )
  ```
- 数据结构:
  - CheckArgs per CheckKind:
    - `ContainsString`: `{target: "D7", case_insensitive: true}`
    - `NumericRange`: `{min: 0.5, max: 1.0}`
    - `MentionsAll`: `{tokens: ["D1","D2",...]}`
    - `JSONPath`: `{path: "$.layers[*].name"}`
    - `CustomLLMJudge`: `{judge_prompt: "评估代码是否引用了 D1 通信层"}`
  - PlanNode.AcceptanceCriteria []AcceptanceCriterion — 扩 §3.5 spec
- 错误处理:
  - `ErrAcceptanceCriterionCheckUnsupported` — CheckKind 不在 enum
  - `ErrAcceptanceCriterionArgsMissing` — CheckKind 要求的 CheckArgs 字段缺失
  - `PlanAcceptanceContractBuilder.Build(dag, ac)` 一致性校验:
    - ∀ node ≥ 1 Required criterion
    - AC.ID 全局唯一
    - AC 引用 node 必须存在(NodeCoverageMissing)
- 性能影响:本地机械 CheckKind(ContainsString/NotContains/Numeric/Length/JSONPath) — O(artifact size),< 5ms。
  - CustomLLMJudge — D3 LLM 调用 ~1s,数量 ≤ 3(预算控制)

### 2.11 模块 11:Verify 节点 Per-Criterion Consumer(本地执行器)

- 修改文件:`verify/per_criterion_executor.go` (NEW) + `verify/verifier.go` (MODIFIED,接收 AC[])
- 关键 API:
  ```go
  type PerCriterionExecutor struct {
      LLMJudge Invoker  // D3 llmgateway 注入,只有 CustomLLMJudge 用
  }
  func (e *PerCriterionExecutor) Execute(ctx, artifact, acs []AcceptanceCriterion) ([]PerCriterionVerdict, error)
  func (e *PerCriterionExecutor) Aggregate(verdicts []PerCriterionVerdict) VerdictKind
  ```
- 执行逻辑:
  1. 对每条 AC dispatch 到对应 CheckKind executor
  2. 机械 CheckKind → 本地执行,无 LLM 调用
  3. CustomLLMJudge → 收集后批量调 D3 LLM(≤ 3 调用)
  4. 顺序对齐返回 PerCriterionVerdict[]
- 聚合规则(Plan ↔ Verify 验收聚合):
  ```
  ∃ Required Fail        → VerdictFail
  全部 Required Pass + ∃ Preferred Fail → VerdictPartial
  全部 Pass              → VerdictPass
  任一 Error 且无 Fail    → VerdictIndeterminate
  ```
- 错误处理:
  - 单条 AC Error → 继续其它 AC,不全 fail
  - 全部 Error → VerdictIndeterminate + slog.Warn
  - 持久化:verdict JSON 落到 `round.Metadata["ac_verdicts"]`
- 性能影响:
  - 短路径:0 CustomLLMJudge → 全部本地,< 50ms(spec_delta §10 short_circuit)
  - 长路径:N CustomLLMJudge → 顺序 LLM,延迟 = N × 1s

### 2.12 模块 12:Decision Node(D7 6 节点流水线新增独立第 5 stage)

- 修改文件:`verify/decision_node.go` (NEW) + `verify/verifier.go` (MODIFIED,Decide() 方法)
- 关键 API:
  ```go
  type DecisionKind string
  const (
      DecisionAccept        DecisionKind = "accept"         // A 接受
      DecisionRetry         DecisionKind = "retry"          // B 重试
      DecisionChildWorker   DecisionKind = "child_worker"   // C 子 Worker
      DecisionParentRollup  DecisionKind = "parent_rollup"  // D 父 rollup
      DecisionHumanReview   DecisionKind = "human_review"   // E 人工
  )
  type Decision struct {
      Kind              DecisionKind
      Reason            string              // 人类可读解释
      NextWorkItemSpec  *ChildWorkItemSpec  // Decision=ChildWorker 时携带
  }
  type ChildWorkItemSpec struct {
      ParentWorkItemID   string
      SubSegmentIDs      []string            // parent AC[] 中 Fail 的子集
      InheritACSubset    []AcceptanceCriterion
      MaxBudget          int                 // 上限 2
  }
  type DecisionNode interface {
      Decide(ctx, verdict VerdictKind, acs []AcceptanceCriterion,
             verdicts []PerCriterionVerdict, meta RoundMeta) (Decision, error)
  }
  type RoundMeta struct {
      AttemptNo           int
      ChildBudgetRemaining int
      RiskLevel           string  // "high" | "normal" | "low"
      IsChildSegment      bool
      SiblingDecidedCount int
      SiblingTotalCount   int
  }
  ```
- 数据结构:**纯规则引擎映射表**(静态查表,0 LLM 调用)。
- 5 路径决策表(同 proposal §5.5):
  ```
  Pass            │ (default)                              │ A 接受
  Partial         │ Tolerance=high OR ChildBudget=0        │ A 接受
  Partial         │ Partial 部分 AC 可独立分解 + Budget>0  │ C 子W
  Partial         │ 其它                                   │ A 接受
  Fail            │ AttemptNo < MaxRetry=1                 │ B 重试
  Fail            │ AttemptNo >= MaxRetry                  │ E 人工
  Indeterminate   │ RiskLevel=high                         │ E 人工
  Indeterminate   │ RiskLevel=normal/low                   │ B 重试
  Error(全Err)    │ Network/Timeout 类                     │ B 重试
  (任意)          │ IsChildSegment + SiblingDecidedCount==SiblingTotalCount │ D 父R
  ```
- 错误处理:
  - 决策失败(空映射)→ 降级 A 接受 + slog.Warn("decision_map_miss")
  - C 子 Worker 触发但 ChildBudgetRemaining=0 → 降级 A 接受
  - D 父 rollup 触发但 parent 已结束 → 降级 A 接受
  - 决策结果落 `round.Metadata["decision"] = JSON{kind, reason}`
- 性能影响:**< 5ms**(纯静态映射表查表,无 I/O,无 LLM 调用)。
- 持久化:
  - decision_kind 落 reputation row(`reputation.metadata.decision_kind`)
  - 飞书 final card 显示"✅ 接受 / 🔄 重试 N 次 / 🔀 子 Worker 分解 / 📦 父 rollup / ❓ 需人工"

### 2.13 模块 13:Sub-Worker Spawn & Child WorkItem 生命周期

- 修改文件:`sessionorchestrator/child_workitem.go` (NEW) + `workmodel/child.go` (MODIFIED)
- 关键 API:
  ```go
  type ChildWorkItem struct {
      ParentID     string
      SubSegmentID string
      InheritAC    []AcceptanceCriterion
      AttemptNo    int
      CreatedAt    time.Time
  }
  type SubWorkerSpawner interface {
      SpawnChild(ctx, spec ChildWorkItemSpec) (*WorkItem, error)
      OnChildComplete(ctx, child *WorkItem) (Decision, error)  // 触发 parent decision 重算
  }
  ```
- 数据结构:
  - child WorkItem 复用现有 workmodel(只加 metadata 字段)
  - parent 持有 children map[childID]→ChildWorkItem
  - 决策闭环:child complete → parent.onChildComplete → parent decision(可能 D 父 rollup)
- 错误处理:
  - child spawn 失败 → parent decision 降级 A 接受(显示 partial_failure)
  - child context cancel → parent drain children + decision 降级
  - child budget 超(> 2)→ 拒绝 spawn,降级 A 接受
- 性能影响:child WorkItem 创建 ~10ms(数据库 round trip)。

### 2.14 模块 14:Learn Node(D7 6 节点流水线最末 stage,per-segment 升格 + 异步化)

- 修改文件:`mups/learn/learner.go` (MODIFIED) + `mups/learn/reputation/row.go` (NEW) + `mups/learn/reputation/parent_evidence.go` (NEW)
- 关键 API(精简契约,仅依赖 Decision + Verify + ArtifactHash):
  ```go
  // LearnRequest 接收节点 5(Decision) + 节点 4(Verify) + 节点 3(ArtifactHash) 的输出
  type LearnRequest struct {
      WorkItemID        string                  // per-segment attribution
      Decision          Decision                // 来自节点 5,5 路径决策
      Verdict           Verdict                 // 来自节点 4,Kind/SourceID/Confidence/Evidence/IndeterminateReason
      ArtifactHash      string                  // 可选,来自节点 3,防重
      PlanRationaleHash string                  // ≤64B,SHA256[:16] hex,plan_rationale 的指纹(诊断用)
  }
  // 注:LearnRequest 不再收 Plan/Observations/ParentContext(契约精简)
  //   - Plan 输出已在 Plan 节点持久化,reputation 不需要冗余
  //   - Observations 已在 Observe 节点持久化,Learn 不消费
  //   - ParentContext 改为 Learn 内部通过 Decision.parent_rollup 触发,
  //     从 Reputation DB SELECT child rows 后 sum,而不是 LearnRequest 携带
  // Verdict.IndeterminateReason 关键性:区分 verifier_parse_failure vs env_limited,
  //   - verifier_parse_failure: AssetBuilder 路由到 PendingAsset(等下轮补全);
  //   - env_limited: 同上但语义略不同(环境受限 vs 解析失败),Evidence.go 仅对前者 β++;
  //   - G8-1 修复扩展,2026-07-07 锁定为 LearnRequest 必带字段(防止契约反向破坏)。
  // PlanRationaleHash (H1 修复):不存全量 plan_rationale 字符串(可能 1-2KB,Learn 异步队列单条 LearnRequest 内存膨胀),
  //   只存 16 字节 SHA256 指纹(hex 64B),用于 reputation.metadata.rationale_hash 关联 + 诊断时反查 DB。
  //   plan_rationale 全量文本本身已由 Plan 节点持久化,reputation 不冗余。

  type LearnResponse struct {
      UpdatedAlpha    float64
      UpdatedBeta     float64
      ReputationRowID string
      BayesianAction  BayesianAction
  }
  type BayesianAction string
  const (
      BayesianAlphaBump BayesianAction = "alpha_bump"
      BayesianBetaBump  BayesianAction = "beta_bump"
      BayesianNoChange  BayesianAction = "no_change"
      BayesianForcePlan BayesianAction = "force_plan"  // β/(α+β) > 0.7
  )
  ```
- 数据结构:
  - **reputation_row schema**:
    ```go
    type ReputationRow struct {
        ID                    string
        SegmentID             string
        ParentID              string  // 指向 parent rollup row
        Alpha                 float64
        Beta                  float64
        LastUpdated           time.Time
        DecisionKindHistory   []string  // 最近 5 次
        SourceIDHistory       []string  // 最近 5 次
        PlanRationale         string
        ArtifactMetadataHash  string    // 防重
    }
    ```
  - **cold start**: `BuildAdaptivePrior(Developer Beta(5, 3))`
  - **BayesianUpdate 公式**(纯函数,无 I/O):
    ```go
    func BayesianUpdate(prior ReputationRow, v Verdict, d Decision) ReputationRow {
        switch v.Kind {
        case VerdictPass:    prior.Alpha++
        case VerdictFail:    prior.Beta++
        case VerdictAbstain: /* no change */
        }
        if d.Kind == DecisionHumanReview {
            prior.Beta++  // 强 negative
        }
        if d.Kind == DecisionRetry {
            return prior  // retry 不累计
        }
        // 防重
        if hash(prior.ArtifactMetadataHash) == currentHash { return prior }
        return prior
    }
    ```
- 错误处理:
  - Learn 失败(reputation DB 写挂)→ silent log + `slog.Warn("learn_failed", segment_id)` + 不阻塞 emit final
  - artifact_metadata_hash 重复 → 跳过,no_change
  - ParentEvidence sum 时 child row 缺失 → 降级只更新存在的 child,log warning
  - force_plan 触发 → 写 `next_observation_force_plan=true` 到 segment_id metadata
- 性能影响:
  - BayesianUpdate 纯函数:< 1ms
  - DB 写 reputation_row:50-100ms(SQLite round trip)
  - **异步执行**,不阻塞主流程
- 持久化:
  - reputation_row DB table:`reputation(id, segment_id, parent_id, alpha, beta, last_updated, decision_kind_history, source_id_history, artifact_hash)`
  - 跨 session 累积:同一 segment_id 多次 learn 累加 α/β
  - cold start 触发条件:`SELECT * FROM reputation WHERE segment_id=?` 0 行 → BuildAdaptivePrior
  - plan_rationale 字段已移除(Learn 不收 Plan,Plan 节点自己持久化 plan_rationale 到 plan 表)

### 2.15 模块 15:Async Learn Goroutine & 错误聚合

- 修改文件:`mups/learn/learner.go` (MODIFIED,新增 goroutine 启动)+ `mups/learn/async.go` (NEW)
- 关键 API:
  ```go
  type AsyncLearner struct {
      Sync   Learner             // 同步执行入口
      Queue  chan LearnRequest   // 异步队列
      Worker int                 // 默认 2
  }
  func (a *AsyncLearner) Start(ctx context.Context) error
  func (a *AsyncLearner) Enqueue(req LearnRequest)  // 立即 return,不阻塞主流程
  func (a *AsyncLearner) Drain(ctx context.Context) error  // 等所有 enqueued 处理完
  ```
- 数据结构:`chan LearnRequest` 有界队列(size = 100,防内存爆)。
- 错误处理:
  - 队列满 → 降级同步执行(走 Sync.Lean)+ log warning
  - worker 失败 → 重新 enqueue 1 次 + log warning
  - Drain 阻塞直到队列空(测试 / session 结束用)
- 性能影响:
  - Enqueue 非阻塞:`< 1ms`
  - 异步执行:2 worker 并行,DB 写 50-100ms
  - **不阻塞 emit final**:emit 完再 enqueue

### 2.16 模块 16:Learn 22 场景全覆盖(Observe + 5 维度扩展)

- 修改文件:`mups/learn/learner.go` (MODIFIED,新增 5 维度策略分发)+ `mups/learn/scenarios.go` (NEW,场景 → 策略映射)
- 关键 API:
  ```go
  // LearnPolicy 是 22 场景的统一入口
  type LearnPolicy struct {
      Planer *Decision    // 节点 5 输出
      Verify *Verdict     // 节点 4 输出
      SourceID string     // 含场景前缀 (obs_fact:/verify:/user_cancel:/user_accept:/...)
  }
  func (p *LearnPolicy) ShouldEnqueue() bool    // 是否触发 Learn
  func (p *LearnPolicy) BayesianAction() Action // alpha_bump / beta_bump / no_change / force_plan

  // 场景 → 策略的纯函数映射(无 I/O,易测试)
  func classifyScenario(p LearnPolicy) ScenarioKind
  ```
- 22 场景 × 6 维度策略表(详见 `decision-tree.md §8.7.10` 完整表 + `proposal.md §5.6.1` 决策说明):
  ```
  维度 1 基础 5 场景 (S-A~S-E): happy path,正常 BayesianUpdate
  维度 2 上游错误 7 场景 (E1-E6):
    - Plan 阶段错误 (E1/E2/E4-pre): NO Learn + audit log
    - Execute/Verify 阶段错误 (E3/E5/E6/E4-post): 正常 Learn β++ (E3/E5/E6) 或 α++ (E4-post last good)
  维度 3 用户主动 3 场景 (U1-U3):
    - U1 user-cancel: β++ (SourceID="user_cancel:seg_a_id")
    - U2 user-accept: α++ fast-track (SourceID="user_accept:seg_a_id")
    - U3 user-modify: 本轮不 Learn
  维度 4 force_plan 2 场景 (F1-F2):
    - F1 β/(α+β) > 0.7: enqueue + 写 metadata.next_observation_force_plan=true,Action=force_plan
    - F2 下次 directive 走 Plan: enqueue α++ (Plan 路径信号更强)
  维度 5 Learn 失败 3 场景 (L1-L3):
    - L1 DB 写挂: silent + retry 3 + 最终 warn
    - L2 AsyncLearner 队列满: 降级 Sync.Learn
    - L3 session 未 Drain: DeferToNextSession marker
  维度 6 跨 session + 特殊 2 场景 (C1 + X1):
    - C1 segment_id 跨 session: cold start Beta(5,3) → α=5+1=6
    - X1 emit 失败 + Learn 已 enqueue: Learn 独立 metric,reputation 正常 α++
  ```
- 数据结构:`map[ScenarioKind]LearnStrategy`,key 是 22 个 enum 值,value 是 `(ShouldEnqueue, BayesianAction)` 元组。
- 错误处理:
  - Plan 错误 NO Learn 时,记 `audit_log{event="plan_error_no_learn", plan_error=E1/E2/E4-pre}` + metric `plan_error_no_learn++`
  - user-cancel/accept 通过 `feishu.UserActionEvent` 触发,在 D1 ingress 边界调用 `AsyncLearner.Enqueue` (SourceID 加前缀)
  - **user-cancel rate-limit (H2 修复,2026-07-07)**:24h 滑动窗口内同一 `segment_id` 累计 ≥ 3 次 user-cancel → 进入 `audit_hold` 状态(只 audit 不 β++),metric `user_cancel_audit_hold++`。理由:避免用户误点/手抖/恶意点击导致 reputation 雪崩;`audit_hold` 让管理员人工确认是否真要降级该 segment。Configurable:`devrix.yaml → user_cancel_rate_limit_max: 3`(默认) + `user_cancel_window_hours: 24`(默认)。
  - L1 DB 写挂:100ms 间隔重试 3 次,最终 `slog.Warn("learn_failed")` + metric `learn_failed_total++`
  - L2 队列满:enqueue 等待 ≤ 5ms 后降级 `Sync.Learn` + metric `learn_queue_full_fallback_total++`
  - L3 未 Drain:`session.DeferToNextSession(req)` + metric `learn_deferred_total++`
- 性能影响:
  - classifyScenario 纯函数:`< 1μs`
  - LearnPolicy.ShouldEnqueue + BayesianAction:合计 `< 10μs`
  - audit log 写入:同步 1-5ms,不阻塞主流程(可在 defer goroutine)
- **D5 observability 注册 (M3 修复,2026-07-07)**:本节所有新增 metric(`plan_error_no_learn++` / `user_cancel_audit_hold++` / `learn_failed_total++` / `learn_queue_full_fallback_total++` / `learn_deferred_total++` / `learn_emitted_after_emit_failure` 等共 6 个)必须注册到 `internal/layers/observability/instrument/metrics/registry.go` 的 `Meter` 列表,与 D5 现有 metrics 同源,Otel SDK 上报到 Grafana。命名规范:`d7.<scenario>.<event>` 前缀(如 `d7.learn.failed` / `d7.user_cancel.audit_hold`),便于 dashboard filter。
- 测试:
  - 22 场景单元测试(spec_delta AC43-AC54 验收)
  - 6 维度并发压测(每维度 ≥ 100 round)
  - L1/L2/L3 故障注入(模拟 DB busy / chan 满 / ctx timeout)
  - **metric 注册测试**:每个 metric 在 registry.go 中存在 + 单元测试断言调用后 counter++

### 2.17 模块 17:Plan 字段验证 + Parse Reject 重试 + Decision plan_error 新路径(Plan 26 场景全覆盖)

- 修改文件:`plan/plan_validator.go` (NEW,字段级验证)+ `plan/reject_retry.go` (NEW,重试 + 降级)+ `sessionorchestrator/plan_error_decision.go` (NEW,plan_error 决策入口)
- 关键 API:
  ```go
  // PlanAcceptanceContractBuilder 字段级验证(扩展 §2.11)
  type PlanFieldValidator struct {
      ContractBuilder *PlanAcceptanceContractBuilder
      MaxRetries      int            // 默认 2
      FallbackPolicy  SpawnPolicy    // 默认 DecomposeIntoChildren(降级旧 Plan)
  }
  func (v *PlanFieldValidator) Validate(p StrategicPlanProposal) (*ValidatedPlan, *PlanParseReject, error)
  func (v *PlanFieldValidator) RetryWithFeedback(ctx, priorProposal, reject *PlanParseReject) (*StrategicPlanProposal, error)

  // Decision plan_error 入口(扩展 §2.12)
  type PlanErrorDecision struct {
      ErrorType  string  // "timeout" / "5xx" / "partial_response"
      Reason     string  // 详细错误信息
  }
  func (d *PlanErrorDecision) Decide() Decision {
      return Decision{Kind: HumanReview, Reason: "plan_error:" + d.ErrorType, NextSpec: nil}
  }
  ```
- 数据结构:`PlanParseReject` 扩展 6 子类:
  ```
  RejectEmptyChildren     // P4
  RejectEmptyDAG          // P6
  RejectInvalidCheckKind  // P7
  RejectRequiredZero      // P8
  RejectPrioritiesOutOfRange // P10
  RejectSegmentIDMissing  // P11
  (已有 6 子类扩展自 §9):
  RejectCycle / RejectTooManyNodes / RejectDuplicateNode / RejectDanglingEdge / RejectACDuplicateID / RejectNodeCoverageMissing
  ```
- 错误处理:
  - Plan LLM 错误(P1-P3):ItemPipelineRunner 捕获 → 构造 `PlanErrorDecision` → 直接 emit abort + 飞书卡 "❌ Plan 阶段失败" + NO Learn
  - 字段异常(P4-P11):PlanFieldValidator.Validate 拒绝 → PlanParseReject + RetryWithFeedback(CompactJSON 错误)→ ≤ 2 次重试 → 仍失败 → FallbackPolicy=DecomposeIntoChildren(降级旧 Plan,无 AC)
  - Parse Reject(P12-P17):同上(同 P4-P11)
  - rationale 缺失(P9):**Validate 通过**(可选字段),Execute/Decision 不变,Learn metadata 缺 rationale + audit log 警告
  - 字段异常降级成功后:Execute 顺序串行 + Verify fallback NumericRange + Decision 简化 A accept + Learn 正常 α++
- 性能影响:
  - PlanFieldValidator.Validate 纯函数:`< 1ms`
  - RetryWithFeedback LLM 调用:每次 1-3s,共 ≤ 2 次
  - PlanErrorDecision.Decide 纯函数:`< 1μs`
- 测试:
  - 26 场景单元测试(spec_delta AC55-AC66 验收)+ 1 条 AC67 验证 Decision 11 行映射表扩展(10 baseline + 1 plan_error)
  - PlanFieldValidator 8 字段级测试 + 6 ParseReject 子类测试
  - RetryWithFeedback mock LLM 测试重试 ≤ 2 次上限
  - PlanErrorDecision 3 错误类型测试
  - 降级路径 Execute/Verify/Decision/Learn 端到端测试

---

## 3. Verification

| Check | Command | Pass Criterion |
|-------|---------|----------------|
| 编译 | `go build ./...` | 0 error |
| 单元测试 | `go test -race ./internal/layers/orchestration/...` | 27 packages PASS |
| 集成测试 | `go test -race ./internal/layers/communication/...` | PASS |
| 覆盖率 | `go test -cover ./internal/layers/orchestration/...` | ≥ 80% (新增模块) |
| Lint | `go vet ./...` | 0 warning |
| E2E LP-3 | `./scripts/devrix.sh start + 飞书发 multi-intent` | 2 张卡片 + finalText 准确 |
| 性能 | `go test -bench ./internal/layers/orchestration/wavescheduler/` | 4 worker DAG 调度 < 50ms |
| Idempotency | unit test + e2e | 重复 emit 不发重复卡 |

---

## 4. Backward Compatibility

- **API 兼容**:PlanDAG 是 Plan 的可选扩展,旧 Plan 路径保留,`d7.dag_executor.enabled=false` 时 100% 行为不变。
- **数据格式变化**:IntentSegmentSet 是 JSON-encoded 字段,旧 session 没该字段 → 运行时降级到 1-element set。
- **灰度策略**:feature flag 灰度 → first 5% → 100% across 2 weeks,内部 feishu 优先
- **旧版本支持**:无 dependency 切换;flag off 即旧行为

---

## 5. Review Notes

(S3-Gate 完成后填写)

- 5 维度检查:
  - 数据:IntentSegment + PlanDAG grammar 完整(JSON schema + Validate),4 种子类型 errors
  - 逻辑:DAG executor 拓扑序 + 4 worker pool + priority heap + error propagation 收敛
  - 边界:Stream emit idempotency + per-segment Learn + PlanParseReject 重试上限都锁死
  - 调用:D1 飞书 API 复用 + config flag 控制 + PlanParseReject JSON 反馈给 LLM
  - 异常:context cancel + Plan LLM 超时 + Validate 错误 4 类
- 结论:待 S3-Gate 评审后填写
