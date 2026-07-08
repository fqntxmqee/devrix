# Spec: D7 Observe ↔ LLM 输入输出协议 (5 场景)

**Domain**: D7 (Orchestration)
**Feature**: observe-llm-io-protocol
**Status**: S2_Proposal
**Versions**: d7-orchestration v4.28.0 → v4.29.0 (follow-up to observational-fastpath)
**Change ID**: devrix-d7-observe-llm-protocol-doc
**Demand ID**: DM-20260708-003
**Parent**: `devrix-d7-observational-fastpath` (DM-20260706-011, S7_Archived 2026-07-07)

---

## 1. 范围

本 spec 定义 D7 **Observe 节点** ↔ **LLM** 的帧级 I/O 协议，覆盖 4 种 ObservationKind (ObsFact / ObsSignal / ObsDeviation / ObsUncertainty) + 1 种**混合场景** (fact + uncertainty 同时出现) = 5 个用户场景。

**不变性承诺**：本 spec **不修改** 任何契约，只是把散落在 6 个 Go 文件 + 1 个 i18n 文件里的契约**显式文档化**，便于 future maintainer / code reviewer / 用户验证。

**与 d7-observational-fastpath-spec.md 的关系**：父 spec 定义了 fast-path 闸门（4 gate 触发条件），本 spec 定义 Observe↔LLM 的输入输出帧。两者正交：fast-path 是 Observe 之后的路由决策，I/O 协议是 Observe 与 LLM 的对话契约。

## 2. 协议总览

```
┌──────────────────────────────────────────────────────────────────┐
│  D7 Observe Node                                                   │
│                                                                     │
│  ┌─────────────┐   ObserveSignalInput (11 字段)                     │
│  │ upstream    │──┐                                                  │
│  │ (Turn Loop, │  │                                                  │
│  │  Rollup)    │  ▼                                                  │
│  └─────────────┘  observeLLMFieldMap ──── filter ────► 6 字段      │
│                                                  │                  │
│                                                  ▼                  │
│                                  ┌──────────────────────────┐       │
│                                  │  D2 i18n system prompt   │       │
│                                  │  + observation appendix  │       │
│                                  └─────────────┬────────────┘       │
│                                                ▼                    │
│                                  ┌──────────────────────────┐       │
│                                  │   LLM InvokeStream       │       │
│                                  └─────────────┬────────────┘       │
│                                                ▼                    │
│  ┌──────────────────────────────────────────────────────────┐      │
│  │  LLM raw response (JSON array)                             │      │
│  │  [{"kind":..., "strength":..., "statement":...,            │      │
│  │    "question":..., "evidence":[...]}, ...]                  │      │
│  └────────────────────────────────┬─────────────────────────────┘      │
│                                   ▼                                    │
│                          parseObservationProposalsJSON                 │
│                                   ▼                                    │
│                          mapRawObsProposals                            │
│                          (硬编码 Category=CatBusiness)                │
│                                   ▼                                    │
│                          validateOneProposal (Go 兜底)                │
│                                   ▼                                    │
│                          ValidateObservationProposals                 │
│                          (max=3 cap)                                   │
│                                   ▼                                    │
│                          UncertaintyReport.NewUncertaintyReport        │
│                          + .Partition()                                 │
│                          → BusinessObservations / Anomalies            │
│                                   ▼                                    │
│                          fast-path gate (4 gates)                      │
│                          OR Plan/Execute/Verify/Learn                   │
│  └──────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

## 3. 输入协议：11 字段 → 6 字段（observeLLMFieldMap）

### 3.1 输入字段全表（11 字段）

源：`internal/layers/orchestration/sessionorchestrator/observation_proposer.go:32-66` (ObserveSignalInput struct)

| # | 字段名 | Go 类型 | 默认值 | 角色 |
|---|---|---|---|---|
| 1 | `SessionID` | string | 必填 | 全局 session 标识 |
| 2 | `WorkItemID` | string | 必填 | 当前 WorkItem 标识 |
| 3 | `Directive` | string | 必填 | 用户原话（**主信号**）|
| 4 | `PriorParseReject` | string | "" | 上一轮 parse 失败原因 (LLM 自纠用) |
| 5 | `PriorMean` | float64 | 0.5 | Bayesian 信誉均值（**Go-only**）|
| 6 | `ScopeGoal` | string | "" | scope 收缩目标 |
| 7 | `ScopeOpenQuestions` | []string | nil | 待闭合开放问题 |
| 8 | `InboundSignalLines` | []string | nil | 多行结构化输入 |
| 9 | `PriorObservationIDs` | []string | nil | 跨轮去重锚点 |
| 10 | `IncrementalOnly` | bool | false | bool 标志 (**Go-only**) |
| 11 | `PriorArtifactSummary` | string | "" | 上一轮 Execute 收敛度（**Plan frame delta**, Go-only）|
| 12 | `KnownGaps` | []string | nil | Phase 2 stub = []（**Go-only**）|

> 注：标 (Go-only) 的字段不进 LLM prompt，详见 §3.3。

### 3.2 实际发给 LLM 的 6 字段

源：`internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go:69-89` (observeLLMFieldMap)

| 标签 | 来源字段 | 出现条件 |
|---|---|---|
| `directive` | `Directive` | 无条件 |
| `prior_parse_reject` | `PriorParseReject` | `TrimSpace != ""` |
| `scope_goal` | `ScopeGoal` | `TrimSpace != ""` |
| `scope_open_question` | `ScopeOpenQuestions` | `len > 0` |
| `signal` | `InboundSignalLines` | `len > 0` |
| `prior_observation_ids` | `PriorObservationIDs` | `len > 0` |

**渲染协议**：`prompttags.BuildLineFrame(...)` (D7-S16-A105-T01 注入 i18n frame delta)，每字段以 `key: value\n` 单行形式串接。

### 3.3 被过滤的 5 字段 + 理由

| 字段 | 理由 |
|---|---|
| `WorkItemID` | Go 内部 ID；LLM 看到会污染 evidence 字段（已有 trace 实证 LLM 倾向"抄字段名"）；Go 端 `validateOneProposal:191-196` 强制 inject 到 evidence 兜底。|
| `PriorMean` | Bayesian 信誉值；给 LLM 引发 3 连锁：① 锚定效应（LLM 围绕 prior 回归）② 自我实现预言（prior 高 → strength 高 → 下轮 prior 更高）③ 职责倒挂（prior 是 Learn 输出不是 Observe 输入）。Test #13 锁死。|
| `IncrementalOnly` | bool 调度标志；LLM 是概率模型，看到 bool 会过度泛化；属于控制流而非证据。|
| `PriorArtifactSummary` | 错层数据；是 **Plan frame delta** 输入而非 Observe classifier 输入；Go 端在 `BuildObservePriorDelta` 单独注入到 Plan frame。|
| `KnownGaps` | Phase 2 stub = []，当前无内容；即便将来填充也属于 Plan/Verify 节点消费。|

**实测**：trace test `TestObserveTraceE2E_OnlyFieldsVisibleToLLM` 把 11 字段全填，断言实际 LLM prompt 只出现 6 个标签。

## 4. 输出协议：JSON Schema + 4 种 kind

### 4.1 顶层 schema（4 字段）

源：`internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go:118-124` (rawObsProposal)

```json
[
  {
    "kind":       "obs_fact | obs_signal | obs_deviation | obs_uncertainty",
    "strength":   0.0..1.0,
    "statement":  "string",
    "question":   "string",
    "evidence":   ["string", ...]
  }
]
```

**约束**：
- 必须 JSON 数组（不要 markdown 包裹）— `i18n.ObservationTaskAppendix` 显式约束
- `kind` 接受大小写不敏感别名（`obs_fact` / `fact` 都行）— `mapRawObsKind:166-180`
- `strength` LLM 给的任意浮点（Go 端 cap / lift 兜底）
- `statement` 与 `question` 至少有一个非空（按 kind 不同侧重）

### 4.2 4 种 kind 的 payload 契约

源：`internal/layers/orchestration/orchtypes/observation.go:130-200`

#### 4.2.1 ObsFact → `FactPayload`

```go
type FactPayload struct {
    Statement string   `json:"statement"`            // 必填, TrimSpace != ""
    Evidence  []string `json:"evidence,omitempty"`   // Go 端自动 append work_item_id + session_id
}
```

**Go 兜底** (`validateOneProposal:201-205`)：
- `statement` 空 → reject (`obs fact: empty statement`)
- `strength > 0.85` → **cap 到 0.85** (`maxLLMObsFactStrength`)
- `strength <= 0` → **lift 到 0.5** (zero protection)

#### 4.2.2 ObsSignal → `SignalPayload`

```go
type SignalPayload struct {
    Name      string  `json:"name"`         // 必填
    Value     float64 `json:"value"`        // = LLM strength
    Threshold float64 `json:"threshold"`    // 硬编码 0.5
    Unit      string  `json:"unit,omitempty"`
}
```

**Go 兜底** (`validateOneProposal:206-211`)：
- `statement` 空 → `"llm_signal"` 默认值
- `Value` = strength，`Threshold` = 0.5（**不在 prompt 暴露**，LLM 不感知 threshold）

#### 4.2.3 ObsDeviation → `DeviationPayload`

```go
type DeviationPayload struct {
    Metric   string  `json:"metric"`     // = statement
    Expected float64 `json:"expected"`   // 硬编码 0
    Observed float64 `json:"observed"`   // = strength
    Delta    float64 `json:"delta"`      // = strength
}
```

**Go 兜底** (`validateOneProposal:212-217`)：
- `statement` 空 → reject (`obs deviation: empty statement`)
- `Metric` = `TrimSpace(statement)`，LLM 必填 statement

#### 4.2.4 ObsUncertainty → `UncertaintyPayload`

```go
type UncertaintyPayload struct {
    Question     string  `json:"question"`         // 必填
    Confidence   float64 `json:"confidence"`       // = 1 - strength
    RequiresMore bool    `json:"requires_more"`    // 硬编码 true
}
```

**Go 兜底** (`validateOneProposal:218-230`)：
- `question` 空 → fallback 到 `statement`
- 都空 → reject (`obs uncertainty: empty question`)
- `Confidence = 1 - strength`（**反向**映射：strength 越高 = 越确定 = confidence 越低？这看似反直觉，实际是"不确定性信号的强度 = 模型的困惑程度"，strength=1.0 表示"非常确定我不知道"）

### 4.3 解析容错

源：`internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go:126-145` (parseObservationProposalsJSON)

| 输入 | 行为 |
|---|---|
| `[{...}]` 纯 JSON | 直接 parse |
| 前面有散文 + `[{...}]` | `strings.Index("[")` 截取首个 `[` 到末尾 `]` |
| 末尾有 note + `[{...}]` | 同上 |
| `oops, garbage` | parse 失败 → 返回 error |
| 空字符串 / `[]` | 返回 (nil, nil) |
| `[{...garbage...}]` | JSON parse 失败 → 返回 error |

Test #12 (JSONParseLeniency) 6 sub-cases 全部覆盖。

## 5. 5 场景输入输出协议

### 场景 1：纯确定性问答（ObsFact fast-path）

**输入协议**：

```yaml
directive: "2×3=几?"              # 必填, 用户原话
prior_parse_reject: ""             # 首轮无
scope_goal: ""                     # trivial Q&A 无 scope
scope_open_question: []            # 无
signal: []                         # 无结构化 input
prior_observation_ids: []          # 无
```

**期望 LLM 输出**：

```json
[{
  "kind": "obs_fact",
  "strength": 0.85,
  "statement": "在标准算术下，2×3=6。",
  "question": "",
  "evidence": []
}]
```

**Go 侧处理**：
1. `parseObservationProposalsJSON` → 1 个 `ObservationProposal{kind=ObsFact, strength=0.85, statement="...", evidence=[]}`
2. `validateOneProposal` → `Observation{kind=ObsFact, category=CatBusiness, strength=0.85, payload=FactPayload{...}}`
3. `UncertaintyReport.NewUncertaintyReport + Partition` → BusinessObservations[0] = fact
4. **fast-path 闸门** (`item_pipeline.go:300-314`)：
   - Gate 1 (`!isRollup && !isDeliverableSynth && !isParentRollup`) ✓
   - Gate 2 (`r.Learner != nil`) ✓
   - Gate 3 (`!hasObsUncertainty(report)`) ✓ (无 ObsUncertainty)
   - Gate 4 (`pickHighStrengthBusinessFact(report, 0.85)`) ✓
5. 触发 fast-path emit：`ExitReason="observational_answer"`、`round.ArtifactSummary="在标准算术下，2×3=6。"`

**测试**：`TestObserveTraceE2E_ObsFact_FastPathTrigger` + `TestObserveTraceE2E_FullPipeline_FactFastPath`

### 场景 2：纯不确定性（ObsUncertainty → Plan decompose）

**输入协议**：

```yaml
directive: "帮我看看 cache 命中率"    # 模糊 directive
prior_parse_reject: ""
scope_goal: ""
scope_open_question: []
signal:
  - "artifact_summary: 之前的 attempt 失败"
prior_observation_ids: []
```

**期望 LLM 输出**：

```json
[{
  "kind": "obs_uncertainty",
  "strength": 0.7,
  "statement": "",
  "question": "cache 命中率需要看哪个服务? 哪个时间窗口?",
  "evidence": []
}]
```

**Go 侧处理**：
1. `parseObservationProposalsJSON` → 1 个 ObsUncertainty proposal
2. `validateOneProposal` → `UncertaintyPayload{Question=..., Confidence=0.3, RequiresMore=true}`
3. `UncertaintyReport.Partition` → BusinessObservations[0] = uncertainty (strength=0.7, CatBusiness)
4. **fast-path 闸门**：
   - Gate 4 miss（无 ObsFact）
   - 降级到 Plan+Execute+Verify

**测试**：`TestObserveTraceE2E_ObsUncertainty_PlanDecompose`

### 场景 3：结构化信号（ObsSignal → Plan 走指标）

**输入协议**：

```yaml
directive: "review p99 latency"
signal:
  - "artifact_summary: 之前的 attempt 失败"
  - "p99_latency_ms: 245"
prior_observation_ids: ["obs_001"]
```

**期望 LLM 输出**：

```json
[{
  "kind": "obs_signal",
  "strength": 0.6,
  "statement": "p99_latency_ms",
  "question": "",
  "evidence": ["obs_001"]
}]
```

**Go 侧处理**：
1. `parseObservationProposalsJSON` → 1 个 ObsSignal proposal
2. `validateOneProposal` → `SignalPayload{Name="p99_latency_ms", Value=0.6, Threshold=0.5}`
3. `UncertaintyReport.Partition` → BusinessObservations[0] = signal
4. fast-path Gate 4 miss → Plan 路径

**测试**：`TestObserveTraceE2E_ObsSignal_StructuredMetric`

### 场景 4：异常检测（ObsDeviation + CatSystem → Anomalies）

**输入协议**：

```yaml
directive: "监控 d7 plan 目录"
signal:
  - "p99_latency_baseline_ms: 50"
  - "p99_latency_observed_ms: 250"
```

**期望 LLM 输出**：

```json
[{
  "kind": "obs_deviation",
  "strength": 0.9,
  "statement": "P99 latency",
  "question": "",
  "evidence": []
}]
```

**Go 侧处理**：
1. `parseObservationProposalsJSON` → 1 个 ObsDeviation proposal (LLM 默认 CatBusiness)
2. **Go 端** `validateOneProposal` 返回 ObsDeviation + CatBusiness → 但本场景需要演示 CatSystem 路由
3. 上游 detector (`DetectAnomalies`) 在 `validateOneProposal` 之后把 CatSystem 标记注入到 proposal.Category
4. `UncertaintyReport.Partition` → Anomalies[0] = deviation (CatSystem)

**关键事实**：**LLM 不能直接 emit CatSystem**。`mapRawObsProposals:156` 硬编码 `Category: orchtypes.CatBusiness`。CatSystem / Anomaly 路由完全是 Go 端基于信号特征 (e.g. latency baseline vs observed delta > threshold) 决定的。

**测试**：`TestObserveTraceE2E_ObsDeviation_AnomalyTrigger`

### 场景 5（混合）：fact + uncertainty 同时出现 → fast-path 被阻断

**输入协议**：

```yaml
directive: "2×3=几?"
signal: []
prior_observation_ids: []
```

**期望 LLM 输出**（矛盾信号 — 实际 LLM 不会输出，但 prompt 边界 case）：

```json
[
  {"kind": "obs_fact", "strength": 0.85, "statement": "在标准算术下 2×3=6", "evidence": []},
  {"kind": "obs_uncertainty", "strength": 0.6, "statement": "", "question": "上下文用十进制还是二进制?", "evidence": []}
]
```

**Go 侧处理**：
1. `parseObservationProposalsJSON` → 2 proposals
2. `validateOneProposal` → 2 observations (1 fact + 1 uncertainty)
3. `UncertaintyReport.Partition` → BusinessObservations[2]
4. **fast-path 闸门**（关键）：
   - Gate 4 ✓ (`pickHighStrengthBusinessFact` 命中 fact str=0.85)
   - **Gate 3 ✗** (`hasObsUncertainty(report) = true`)
   - 闸门组合 miss → 降级到 Plan+Execute+Verify

**为什么这么设计**：fact + uncertainty 是**矛盾信号**（LLM 既说"我知道答案"又说"我不确定"）。让 Plan 节点裁决，而不是直接 emit 一个 LLM 自己都不确定的答案。

**i18n prompt 防呆**：`format_hints_mups.go:36` 显式约束"对于确定性问答，不要混着 obs_uncertainty 追问"，降低 LLM 输出矛盾组合的概率。

**测试**：`TestObserveTraceE2E_FactPlusUncertainty_FastPathBlocked`

## 6. 闸门契约（与 d7-observational-fastpath-spec.md 的关系）

本 spec **不重复** fast-path 闸门契约。父 spec `d7-observational-fastpath-spec.md` 定义 4 gate 触发条件，本 spec 只覆盖：

| 闸门 | 本 spec 覆盖范围 |
|---|---|
| Gate 1 (`!isRollup && ...`) | ❌ 父 spec |
| Gate 2 (`r.Learner != nil`) | ❌ 父 spec |
| Gate 3 (`!hasObsUncertainty(report)`) | ✅ **场景 5 显式覆盖** |
| Gate 4 (`pickHighStrengthBusinessFact`) | ✅ **场景 1 显式覆盖** |

## 7. 兜底规则（Go-side invariants）

| 规则 | 触发位置 | 行为 |
|---|---|---|
| 强度 cap | `validateOneProposal:187-189` | `ObsFact.strength > 0.85` → cap 到 0.85 |
| 零保护 | `validateOneProposal:184-186` | `strength <= 0` → lift 到 0.5 |
| Evidence 注入 | `validateOneProposal:191-196` | append `WorkItemID` + `SessionID` 到 FactPayload/SignalPayload |
| Max proposals | `ValidateObservationProposals:171-173` | `len(out) >= 3` break |
| 空 reject | `validateOneProposal:202-203` | FactPayload.Statement 空 → skip |
| 空 reject | `validateOneProposal:214-215` | DeviationPayload.Metric 空 → skip |
| 空 reject | `validateOneProposal:223-224` | UncertaintyPayload.Question + Statement 都空 → skip |
| Question fallback | `validateOneProposal:219-222` | Uncertainty.Question 空 → fallback 到 Statement |
| Category 强制 | `validateOneProposal:180-181` | `category > CatSystem` → 兜底 CatBusiness |
| Kind alias | `mapRawObsKind:166-180` | 大小写不敏感 + 短别名 (`fact`/`signal`/...) |

## 8. Partition 路由

源：`internal/layers/orchestration/orchtypes/uncertainty_report.go:97-128`

| 规则 | 输入 | 输出 |
|---|---|---|
| CatBusiness | 任何 Kind | `BusinessObservations` |
| CatSystem + ObsDeviation | | `SystemObservations` AND `Anomalies` |
| CatSystem + ObsUncertainty (strength ≥ 0.7) | | `SystemObservations` AND `Anomalies` |
| CatSystem + ObsUncertainty (strength < 0.7) | | `SystemObservations` only |
| CatSystem + ObsFact | | `SystemObservations` only |

**Overall 字段** = `mean(BusinessObservations.strength)`，system 永远不参与。

## 9. Test 覆盖（16 cases）

| # | Test | 场景 | 关键断言 |
|---|---|---|---|
| 1 | `OnlyFieldsVisibleToLLM` | 输入过滤 | 11→6 字段过滤 + 字段意图表 |
| 2 | `ObsFact_FastPathTrigger` | 场景 1 | strength cap 0.85 + evidence inject + fast-path 命中 |
| 3 | `ObsUncertainty_PlanDecompose` | 场景 2 | UncertaintyPayload {Question, Confidence=1-strength, RequiresMore=true} |
| 4 | `ObsSignal_StructuredMetric` | 场景 3 | SignalPayload {Name=stmt, Value=strength, Threshold=0.5} |
| 5 | `ObsDeviation_AnomalyTrigger` | 场景 4 | CatSystem+ObsDeviation → Anomalies |
| 6 | `StrengthClamping` (5 sub) | 兜底 | 0.99→0.85 / 0.85 / 0.50 / 0.0→0.5 / ObsUncertainty 无 cap |
| 7 | `MaxProposalsTruncated` | 兜底 | max=3 cap |
| 8 | `EmptyProposalsRejected` | 兜底 | `[]` parse reject |
| 9 | `FactEmptyStatementRejected` | 兜底 | FactPayload statement 空 reject |
| 10 | `UncertaintyQuestionFallback` (3 sub) | 兜底 | Question 空 → fallback → reject |
| 11 | `KindAliasCaseInsensitive` (4 sub) | 解析容错 | 4 alias × 大小写 |
| 12 | `JSONParseLeniency` (6 sub) | 解析容错 | 6 种 wrapper 容错 |
| 13 | `BayesianPrior_GoSideOnly` | 输入过滤 | PriorMean 永不进 LLM prompt |
| 14 | `AllKinds_PartitionRouting` | Partition | 3 kind 混合 → Business + Anomalies 路由 |
| 15 | `FactPlusUncertainty_FastPathBlocked` | **场景 5 (混合)** | Gate 4 ✓ + Gate 3 ✗ → fast-path miss |
| 16 | `FullPipeline_FactFastPath` | 场景 1 end-to-end | Run() 整条 pipeline, round.ArtifactSummary 直 emit |

**运行**：

```bash
go test -v -run TestObserveTraceE2E \
  ./internal/layers/orchestration/sessionorchestrator/...
```

**当前状态**：16/16 PASS (2026-07-08)。

## 10. 关系网

### 上游
- `devrix-d7-observational-fastpath` (DM-20260706-011, S7_Archived 2026-07-07) — 父 spec, 定义 fast-path 闸门
- `devrix-d7-mups-v4-phase2-observe-plan` (DM-20260623-001, S7_Archived 2026-06-23) — 4 ObservationKind + sealed Payload 类型定义
- `devrix-d7-mups-v4-phase3-execute` (DM-20260625-001, S7_Archived 2026-06-23) — ExecutionContext 上下文注入

### 下游
- `devrix-d7-multi-intent-observation-decompose` (DM-20260707-001) — multi-intent directive 切分，Observe LLM 是入口
- 任何新增 ObservationKind 必须先扩 `mapRawObsKind` + `validateOneProposal` + `Payload` sealed interface，并加对应 trace test

### 关联 PR
- #472 (Trace validation 16 tests) — 本 spec 的运行验证
- #470 (D7 fast-path task_incomplete bypass)
- #471 (D1 fast-path task_incomplete bypass)

## 11. 沉淀物清单

| 类型 | 路径 | 描述 |
|---|---|---|
| Spec | `openspec/changes/devrix-d7-observe-llm-protocol-doc/specs/d7-orchestration/d7-observe-llm-io-protocol-spec.md` | 本文档 |
| Test | `internal/layers/orchestration/sessionorchestrator/observe_trace_e2e_test.go` | 16 trace test |
| Demand | `openspec/changes/devrix-d7-observe-llm-protocol-doc/demand.md` | DM-20260708-003 S1 |
| Proposal | `openspec/changes/devrix-d7-observe-llm-protocol-doc/proposal.md` | S2 lite |

## 12. 后续工作（不在本 Change 范围）

- 把本 spec 合并到主 `openspec/specs/d7-orchestration/spec.md` (lite-mode) — S6 归档时决策
- 把混合场景加入 `d7-observational-fastpath-spec.md` 的 "Scenario: complex multi-intent" 章节
- 添加 `prior_observation_ids` 跨轮去重实测（当前 trace 还没覆盖 dedup 行为）
- i18n appendix 加 EN 版本的 mixed-scenario 引导，避免英文 prompt 矛盾输出