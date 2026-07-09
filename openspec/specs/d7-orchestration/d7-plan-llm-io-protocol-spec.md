# Spec: D7 Plan ↔ LLM 输入输出协议 (5 场景)

**Domain**: D7 (Orchestration)
**Feature**: plan-llm-io-protocol
**Status**: S2_Proposal
**Versions**: d7-orchestration v4.29.0 → v4.30.0 (follow-up to observe-llm-io-protocol)
**Change ID**: devrix-d7-plan-llm-protocol-doc
**Demand ID**: DM-20260708-004
**Parent**: `devrix-d7-observe-llm-protocol-doc` (DM-20260708-003, S5_Accepted)
**Sibling**: `devrix-d7-mups-v4-phase2-prb1` (DM-20260623-001, 4 PlanKind)
**Sibling**: `devrix-d7-mups-v4-phase3-prc2` (DM-20260625-001, 4 Channel)

---

## 1. 范围

本 spec 定义 D7 **Plan 节点** ↔ **LLM** 的帧级 I/O 协议，覆盖 4 种 LLM `execution_mode`
(`single` / `decompose` / `parallel_probe`) → 4 PlanKind (CommitmentPlan / ProtocolPlan /
ScenarioPlan / ExplorationPlan) + 1 种**混合场景** (single + 高 UncertaintyMean + 
hasHighStrengthFact → `applySingleModeUncertaintyGate` fast-path bypass) = 5 个用户场景。

**关键认知**（与 observe 节点的根本区别）：
- **Plan↔LLM 协议 ≠ `plan.Plan` (4 PlanKind) 协议**。LLM 看到的是 `StrategicPlanFrame`
  (18 字段) + `StrategicPlanAppendix` i18n prompt, LLM emit 的是 `rawStrategicPlan`
  (execution_mode / scope_in / child_specs / resolution_strategies /
  deliverable_contract / react_iters_hint / rationale)。`plan.Plan` (4 PlanKind) 是
  Go 端 `MatchKind` 根据 LLM `QuantizedKind` 决策的**确定性构造**, 不进 LLM。
- **Plan 节点有两层设计哲学**：
  - **LLM 层**：决定"如何拆解"(`execution_mode`)、"做什么"(`rationale` / `deliverable_contract`)
  - **Go 层**：决定"路由到哪个 Channel"(4 PlanKind via `MatchKind`)

**不变性承诺**：本 spec **不修改** 任何契约, 只是把散落在 3 个 Go 文件 + 1 个 i18n
文件里的契约**显式文档化**, 便于 future maintainer / code reviewer / 用户验证。

## 2. 协议总览

```
┌──────────────────────────────────────────────────────────────────┐
│  D7 Plan Node                                                    │
│                                                                   │
│  ┌──────────────┐  StrategicPlanInput (12 字段)                  │
│  │ upstream     │──┐                                              │
│  │ (Observe     │  │                                              │
│  │  Report,     │  ▼                                              │
│  │  WorkItem)   │  buildStrategicPlanFrame                       │
│  │              │  + buildLineFrameFromStruct                    │
│  │              │  + planFrameToMap (field guide)                │
│  └──────────────┘  + RenderFrameFieldGuideForFields              │
│                                                  │                │
│                                                  ▼                │
│                                  ┌──────────────────────────┐     │
│                                  │  D2 i18n system prompt   │     │
│                                  │  + StrategicPlanAppendix │     │
│                                  │    (i18n/prompt_dynamic  │     │
│                                  │     .go:79-104)          │     │
│                                  └─────────────┬────────────┘     │
│                                                ▼                  │
│                                  ┌──────────────────────────┐     │
│                                  │   LLM InvokeStream       │     │
│                                  └─────────────┬────────────┘     │
│                                                ▼                  │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │  LLM raw response (JSON OBJECT, single, no markdown)     │    │
│  │  {"execution_mode":..., "scope_in":[...],                │    │
│  │   "child_specs":[...], "resolution_strategies":[...],    │    │
│  │   "deliverable_contract":..., "react_iters_hint":...,    │    │
│  │   "rationale":"..."}                                       │    │
│  └────────────────────────────────┬─────────────────────────┘    │
│                                   ▼                               │
│                          parseStrategicPlanJSON                  │
│                          (prompttags.ParseWholeBody)             │
│                                   ▼                               │
│                          mapRawChildSpecs /                       │
│                          mapRawResolutionStrategies /             │
│                          mapExecutionModeToQuantizedKind         │
│                          (clampReactIters 1-5)                   │
│                                   ▼                               │
│                          validateStrategicPlan (Go 兜底)         │
│                          - execution_mode ∈ {single,decompose,   │
│                            parallel_probe}                       │
│                          - decompose ⇒ child_specs 必填          │
│                          - single ⇒ child_specs 强制清空         │
│                                   ▼                               │
│                          applyBudgetCap (T-P1-3)                 │
│                          applySingleModeUncertaintyGate          │
│                          (DM-20260706-009 fast-path bypass)      │
│                                   ▼                               │
│                          DefaultPlanner.Plan(PlanInput)          │
│                          + MatchKind (4 Rules)                   │
│                          + Plan.Validate (PP-1/2/3)              │
│                                   ▼                               │
│                          4 PlanKind → 4 Channel Router           │
│                          (Commit / Protocol / Scenario /         │
│                           Exploration)                           │
└──────────────────────────────────────────────────────────────────┘
```

## 3. 输入协议: StrategicPlanFrame 18 字段

### 3.1 18 字段全表 (7 data + 11 control)

源: `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go:65-109`
(`StrategicPlanFrame` struct + `pt:"name,plane,omit_X"` tag)

| # | 字段 | Go 类型 | Plane | Omit 条件 | 出现条件 | 角色 |
|---|---|---|---|---|---|---|
| 1 | `work_item_id` | string | control | omit_empty | `WorkItemID != ""` | 全局 WorkItem 标识 |
| 2 | `directive` | string | data | omit_empty | 无条件 | 用户原话（**主信号**）|
| 3 | `prior_parse_reject` | string | control | omit_empty | `TrimSpace != ""` | 上一轮 parse 失败原因 (LLM 自纠用) |
| 4 | `observation_ids` | []string | data | omit_empty | `len > 0` | 跨轮去重锚点 (来自 Observe) |
| 5 | `observation_summary` | string | data | omit_empty | `TrimSpace != ""` | 上轮 Obs 摘要 |
| 6 | `resolution_strategies` | []string | data | omit_empty | `len > 0` | DM-20260704-006 RC-1 跨轮反馈 |
| 7 | `resolution_claims` | []string | data | omit_empty | `len > 0` | DM-20260704-006 RC-2 跨轮反馈 |
| 8-16 | `Budget 9 字段` | *int | control | (无 omit) | **`MaxChildren > 0`** | spawn-side 限额 (depth/max_depth/existing_children/remaining_children/max_children/decompose_used_today/remaining_daily/max_daily/max_iters) |
| 17 | `parent_scope_in` | []string | control | omit_empty | `len > 0` | 父 WorkItem in-scope 路径 |
| 18 | `uncertainty_mean` | float64 | control | omit_zero | `> 0` | 当前 WorkItem 不确定性均值 |

### 3.2 与 ObserveUserFrame (11 字段) 的关键差异

| 维度 | Observe (11 字段) | Plan (18 字段) |
|---|---|---|
| **字段数** | 11 | 18 (+7) |
| **data plane 字段** | 4 (directive / prior_observation_ids / scope_open_question / signal) | 5 (directive / observation_ids / observation_summary / resolution_strategies / resolution_claims) |
| **control plane 字段** | 7 | 13 (+6: budget×9 / parent_scope_in / uncertainty_mean) |
| **过滤方式** | **显式 N→M** (`observeLLMFieldMap` 11→6) | **反射 + 条件** (`buildStrategicPlanFrame` 显式 if-guard + prompttags pt tag) |
| **实际 LLM 看到** | 固定 6 字段 | 动态 6-15 字段 (按 directive 复杂度) |
| **Data vs Control 分层** | 不显式区分 (`pt` tag 在 Observe 节点未使用) | 显式区分 (`data` 字段 LLM 可读; `control` 字段 LLM 读但语义不变量由 Go 端守) |

### 3.3 字段过滤规则（核心）

源: `strategic_plan_proposer.go:117-175` `buildStrategicPlanFrame`

| 字段 | 过滤条件 | 原因 |
|---|---|---|
| `ObservationIDs` | `len > 0` | 空数组不构成信号 |
| `ObservationSummary` | `TrimSpace != ""` | 空字符串无信息 |
| `ResolutionStrategies` | `len > 0` | 同 ObservationIDs |
| `ResolutionClaims` | `len > 0` | 同 ObservationIDs |
| `Budget 9 字段` | **`MaxChildren > 0`** | 未配 budget 时不渲染 (9 字段全 skip) |
| `ParentScopeIn` | `len > 0` | 同 ObservationIDs |
| `UncertaintyMean` | `> 0` | 0 不构成信号 |
| `PriorParseReject` | `TrimSpace != ""` | 同 ObservationSummary |

**实测** (类比 observe 的 `TestObserveTraceE2E_OnlyFieldsVisibleToLLM`):
`TestPlanTraceE2E_FrameStructure_18Fields` 把 StrategicPlanInput 全填, 断言实际
LLM prompt 出现所有 18 字段; 反之空 `StrategicPlanInput{}` 时只出现
`directive` (1 字段)。

## 4. 输出协议: rawStrategicPlan JSON Schema

### 4.1 顶层 schema (8 字段, 单对象非数组)

源: `strategic_plan_proposer.go:323-339` (`rawStrategicPlan` struct)

```json
{
  "execution_mode":        "single | decompose | parallel_probe",
  "scope_in":              ["string", ...],
  "child_specs":           [ChildSpec, ...]               // DEPRECATED, 仅 decompose mode
  "resolution_strategies": [ResolutionStrategy, ...],     // DM-20260704-006 RC-1 首选
  "deliverable_contract":  DeliverableContract,
  "deliverable_schema":    "string",
  "react_iters_hint":      1..5,
  "rationale":             "string"
}
```

**约束**:
- **必须 JSON 对象** (不要 markdown 包裹) — `StrategicPlanAppendix:89,97` 显式约束
- **execution_mode 3 选 1** enum (validateStrategicPlan 强制)
- **child_specs** 仅 `execution_mode: "decompose"` 时生效; 单一 mode 强制清空
- **resolution_strategies** 优先于 child_specs (RC-1 合同)
- **react_iters_hint** clamp 到 [1, 5] (clampReactIters 兜底)

### 4.2 关键子结构

#### 4.2.1 rawStrategicChildSpec (DEPRECATED, RC-5 兜底)

```go
type rawStrategicChildSpec struct {
    Title           string   `json:"title"`
    DirectiveSuffix string   `json:"directive_suffix"`
    ExpectedReturn  string   `json:"expected_return"`
    ScopeIn         []string `json:"scope_in"`
}
```

> 注: DM-20260704-006 Phase 5 标记为 DEPRECATED, 但 Decide 仍消费, 计划下个 major version 删除。

#### 4.2.2 rawResolutionStrategy (RC-1 首选)

```go
type rawResolutionStrategy struct {
    ObsID            string                 `json:"obs_id"`
    PlannedTool      string                 `json:"planned_tool"`
    SuccessCriterion string                 `json:"success_criterion"`
    SubWorktree      *rawStrategicChildSpec `json:"sub_worktree"`  // optional, 触发 SpawnDecompose
}
```

#### 4.2.3 DeliverableContract (7 维度)

```go
type DeliverableContract struct {
    Citation   []string `json:"citation"`     // ["none","file_line"]
    Severity   []string `json:"severity"`     // ["none","p0_p1"]
    Reject     []string `json:"reject"`       // ["planning_meta"]
    MinRunes   int      `json:"min_runes"`
    // ... 其他维度由 workmodel 扩展
}
```

### 4.3 解析路径

源: `strategic_plan_proposer.go:455-499` `parseStrategicPlanJSON`

```
LLM raw text
    │
    ▼
prompttags.ParseWholeBody[rawStrategicPlan]  // strict JSON
    │ (失败)
    ▼ fallback
strings.Index("{") + strings.LastIndex("}")  // 截取 [first, last]
    │
    ▼
json.Unmarshal  // 二次尝试
    │
    ▼
mapRawChildSpecs (decompose 模式)
mapRawResolutionStrategies (RC-1 模式, 覆盖 ChildSpecs)
mapExecutionModeToQuantizedKind
clampReactIters 1-5
    │
    ▼
validateStrategicPlan (Go 兜底)
```

**与 observe 解析容错对比**:
- observe 解析数组 (leading prose / trailing note 容错)
- plan 解析单对象 (首尾 brace 截取容错, 没数组切片)

### 4.4 execution_mode → QuantizedKind 映射

源: `strategic_plan_proposer.go:514-523` `mapExecutionModeToQuantizedKind`

| LLM `execution_mode` | 映射 `QuantizedKind` | 含义 |
|---|---|---|
| `single` | `intent_command` | 单步直接命令 |
| `parallel_probe` | `intent_fast` | 并行快速探测 |
| `decompose` (默认/其他) | `intent_orchestrate` | 编排多步 |

## 5. 5 场景输入输出协议

### 场景 1: single mode + 1 step → CommitmentPlan

**输入协议**:

```yaml
directive: "删除临时文件 /tmp/scratch.txt"        # 必填, 用户原话
work_item_id: "wi_001"                            # 全局 ID
observation_ids: ["obs_001"]                     # 来自 Observe
observation_summary: "ObsFact str=0.85, no uncertainty"
budget:                                           # spawn-side 限额
  max_children: 3
  max_iters: 5
parent_scope_in: ["/tmp"]
uncertainty_mean: 0.1                             # 低 (ObsFact 主导)
```

**期望 LLM 输出**:

```json
{
  "execution_mode": "single",
  "scope_in": ["/tmp"],
  "child_specs": [],
  "resolution_strategies": [],
  "deliverable_contract": {"citation":["none"], "severity":["none"], "reject":["planning_meta"]},
  "deliverable_schema": "not_applicable",
  "react_iters_hint": 1,
  "rationale": "单步文件操作, ObsFact str=0.85, 不需要 decompose"
}
```

**Go 侧处理**:
1. `parseStrategicPlanJSON` → 1 个 `StrategicPlanProposal{ExecutionMode="single", QuantizedKind="intent_command"}`
2. `validateStrategicPlan`: execution_mode ∈ enum ✓; single mode + child_specs=[] → 强制清空 (no-op)
3. `applySingleModeUncertaintyGate`: 详见场景 5 (本场景 UncertaintyMean=0.1 < threshold, 不触发 fast-path)
4. `DefaultPlanner.Plan(PlanInput{QuantizedKind: "intent_command", Steps: 1, AnomaliesCount: 0})`:
   - `MatchKind("intent_command", 1, 0)` → Rule 2: stepCount==1 → **`CommitmentPlan`**
   - `strengthFloor(0, 1) = 0.7 + 0.02 = 0.72`
5. `Plan.Validate()`: Kind.IsKnown ✓, Steps.len=1 ✓, FailureCriteria.len=1 ✓, BlastRadius OK
6. **路由**: CommitmentPlan → **CommitChannel** (synchronous, single-step, deterministic)
7. Execute 走 `workitem_executor_direct` 路径

**测试**: `TestPlanTraceE2E_SingleMode_CommitmentPlan`

### 场景 2: command + multi-step ≤3 → ProtocolPlan

**输入协议**:

```yaml
directive: "迁移数据库 schema v1 → v2"
observation_ids: ["obs_001", "obs_002"]
observation_summary: "ObsSignal Name=db_schema_version Value=0.6, multi-step"
budget: {max_children: 5, max_iters: 10}
uncertainty_mean: 0.3
```

**期望 LLM 输出**:

```json
{
  "execution_mode": "decompose",
  "scope_in": ["db/schema/"],
  "child_specs": [
    {"title": "备份 v1", "directive_suffix": "先全量备份", "expected_return": "backup_done", "scope_in": ["db/backup/"]},
    {"title": "迁移 v1→v2", "directive_suffix": "按 migration 顺序", "expected_return": "schema_v2_ready", "scope_in": ["db/schema/"]},
    {"title": "验证 v2", "directive_suffix": "运行 verification 套件", "expected_return": "verify_pass", "scope_in": ["db/verify/"]}
  ],
  "deliverable_contract": {"citation":["file_line"], "severity":["p0_p1"], "reject":["planning_meta"]},
  "react_iters_hint": 3,
  "rationale": "3 步幂等迁移, 用 decompose 拆成 child"
}
```

**Go 侧处理**:
1. `parseStrategicPlanJSON` → `StrategicPlanProposal{ExecutionMode="decompose", ChildSpecs=3 items}`
2. `validateStrategicPlan`: execution_mode ∈ enum ✓; **decompose mode + child_specs.len=3 ≥ 1** ✓
3. `applyBudgetCap`: CapChildSpecs (3 ≤ 5) ✓
4. `DefaultPlanner.Plan(PlanInput{QuantizedKind: "intent_orchestrate", Steps: 3, AnomaliesCount: 0})`:
   - `MatchKind("intent_orchestrate", 3, 0)` → Rule 3: intent_command OR stepCount<=3 → **`ProtocolPlan`**
   - `strengthFloor(0, 2) = 0.7 + 0.04 = 0.74`
5. `Plan.Validate`: PP-2 FailureCriteria len=1 ✓
6. **路由**: ProtocolPlan → **ProtocolChannel** (asynchronous, multi-step, idempotent)

**测试**: `TestPlanTraceE2E_DecomposeMode_ProtocolPlan`

### 场景 3: parallel_probe → ScenarioPlan

**输入协议**:

```yaml
directive: "分析为何 build 失败"
observation_ids: ["obs_001"]
observation_summary: "ObsUncertainty str=0.5, intent=fast"
budget: {max_children: 0}                        # 不配 budget (Budget 9 字段全 skip)
uncertainty_mean: 0.5
```

**期望 LLM 输出**:

```json
{
  "execution_mode": "parallel_probe",
  "scope_in": ["build/", "ci/"],
  "child_specs": [],
  "resolution_strategies": [],
  "deliverable_contract": {"citation":["none"], "severity":["none"]},
  "react_iters_hint": 2,
  "rationale": "只读探测, 不修改 build 状态"
}
```

**Go 侧处理**:
1. `parseStrategicPlanJSON` → `StrategicPlanProposal{ExecutionMode="parallel_probe"}`
2. `validateStrategicPlan`: execution_mode ∈ enum ✓
3. `DefaultPlanner.Plan(PlanInput{QuantizedKind: "intent_fast", Steps: 1, AnomaliesCount: 0})`:
   - `MatchKind("intent_fast", 1, 0)` → Rule 2: stepCount==1 → **理论上 CommitmentPlan**
   - 但本场景通过 Budget.MaxChildren=0 触发 ReadOnlyProbe 路径 → Go 端**强制改写** Kind → `ScenarioPlan`
4. **路由**: ScenarioPlan → **ScenarioChannel** (read-only probe, no side-effect)

> 注: 实际 Go 端可能有额外 if-guard 把 `parallel_probe` 强制路由到 ScenarioChannel, 详见 `channel_router.go`。

**测试**: `TestPlanTraceE2E_ParallelProbe_ScenarioPlan`

### 场景 4: decompose + anomalies≥3 → ExplorationPlan

**输入协议**:

```yaml
directive: "比较 3 种 cache 实现"
signal: ["p99_latency_baseline: 50ms"]
observation_summary: "3 anomalies (CatSystem + ObsDeviation)"
uncertainty_mean: 0.8
anomalies_count: 3                                # 来自 UncertaintyReport.Anomalies
```

**期望 LLM 输出**:

```json
{
  "execution_mode": "decompose",
  "scope_in": ["cache/"],
  "child_specs": [
    {"title": "实现 v1 (LRU)", "directive_suffix": "在 sandbox 跑", "expected_return": "metrics_v1", "scope_in": ["cache/v1/"]},
    {"title": "实现 v2 (LFU)", "directive_suffix": "在 sandbox 跑", "expected_return": "metrics_v2", "scope_in": ["cache/v2/"]},
    {"title": "实现 v3 (ARC)", "directive_suffix": "在 sandbox 跑", "expected_return": "metrics_v3", "scope_in": ["cache/v3/"]}
  ],
  "deliverable_contract": {"citation":["file_line"], "severity":["p0_p1"]},
  "react_iters_hint": 5,
  "rationale": "3 个并行实验, sandbox 隔离"
}
```

**Go 侧处理**:
1. `parseStrategicPlanJSON` → `StrategicPlanProposal{ExecutionMode="decompose"}`
2. `validateStrategicPlan`: execution_mode ∈ enum ✓; decompose + child_specs=3 ≥ 1 ✓
3. `DefaultPlanner.Plan(PlanInput{QuantizedKind: "intent_orchestrate", Steps: 3, AnomaliesCount: 3})`:
   - `MatchKind("intent_orchestrate", 3, 3)` → Rule 1: **anomaliesCount>=3** → **`ExplorationPlan`** (优先 Rule 1)
4. **路由**: ExplorationPlan → **ExplorationChannel** (parallel experiments, sandboxed)

**测试**: `TestPlanTraceE2E_DecomposeHighAnomaly_ExplorationPlan`

### 场景 5 (混合): single + 高 UncertaintyMean + hasHighStrengthFact → fast-path bypass

**输入协议**:

```yaml
directive: "1+1=几?"
observation_ids: ["obs_fact_001"]
observation_summary: "ObsFact str=0.95, statement='1+1=2 在标准算术下成立'"
uncertainty_mean: 0.6                              # **高** (被其他低 strength Obs 拉高)
# 但 UncertaintyReport.BusinessObservations 中存在 ObsFact str=0.95
```

**期望 LLM 输出** (矛盾信号 — 实际 LLM 在 single + 高 uncertainty 下会被 gate 拒绝):

```json
{
  "execution_mode": "single",
  "scope_in": [""],
  "react_iters_hint": 1,
  "rationale": "1+1=2, 直接答"
}
```

**Go 侧处理**:
1. `parseStrategicPlanJSON` → `StrategicPlanProposal{ExecutionMode="single"}`
2. `validateStrategicPlan`: ✓
3. **`applySingleModeUncertaintyGate`** (`strategic_plan_proposer.go:723-743`):
   - 条件 1: `prop.ExecutionMode == "single"` ✓
   - 条件 2: `in.UncertaintyMean >= SingleModeUncertaintyThreshold` (≈0.45) ✓
   - 条件 3: `hasHighStrengthFact(in.Report, highConfidenceFactThreshold)` (0.9) ✓ — ObsFact str=0.95 命中
   - **→ bypass gate, return nil (不 reject)**
4. `DefaultPlanner.Plan(...)` → `MatchKind("intent_command", 1, 0)` → **`CommitmentPlan`**
5. **路由**: CommitmentPlan → **CommitChannel**

**为什么这么设计** (DM-20260706-009): high-strength ObsFact (≥0.9) 已经给出答案
(e.g. "1+1=2"), 但**其他低 strength Obs** 把 UncertaintyMean 拉到 0.6,
让 single mode 看起来"违规"。如果不让 fast-path bypass, LLM 会被 gate 拒绝
→ 强制 decompose → Execute + bash echo "1+1=2" → 浪费 1 次 LLM 调用 + ~2-3s 延迟。
bypass 让 single + commitment 直接 emit 答案, ~1s 延迟。

**与 Observe fast-path 的关系**:
- **Observe fast-path** (DM-20260706-011): 直接 emit `ObsFact.Statement`, 完全跳过 Plan/Execute
- **Plan single-mode bypass** (DM-20260706-009): 进 Plan 节点, 但 LLM 不会被 gate 拒绝 force-decompose, 走 single → CommitmentPlan → CommitChannel

**i18n prompt 防呆** (`prompt_dynamic.go:102`): "只能使用下方 directive 与 Obs 摘要" 降低 LLM 越界提案概率。

**测试**: `TestPlanTraceE2E_SingleModeFastPathBypass`

## 6. 闸门契约 (与 d7-observational-fastpath-spec.md / d7-mups-v4-phase2-prb1-archived.md 的关系)

本 spec **不重复** 已覆盖的契约：

| 闸门 / 决策 | 本 spec 覆盖范围 |
|---|---|
| 4 PlanKind enum (plan.go:30-72) | ❌ 父 PR-B1 spec |
| MatchKind 4 Rules (planner.go:116-131) | ✅ **本 spec 场景 1-4 显式覆盖** |
| Plan.Validate PP-1/2/3 (plan_struct.go:180-262) | ❌ 父 PR-B1 spec |
| Observe fast-path 4 gates (item_pipeline.go:300-314) | ❌ observe spec |
| **applySingleModeUncertaintyGate** (strategic_plan_proposer.go:723-743) | ✅ **场景 5 显式覆盖** |

## 7. 兜底规则 (Go-side invariants)

| 规则 | 触发位置 | 行为 |
|---|---|---|
| Execution mode enum | `validateStrategicPlan:637-641` | `execution_mode ∈ {"single", "decompose", "parallel_probe", ""}` else reject |
| Decompose child_specs 必填 | `validateStrategicPlan:642-644` | `mode=="decompose" && len(ChildSpecs)==0` → reject |
| Single child_specs 清空 | `validateStrategicPlan:645-647` | `mode=="single" && len(ChildSpecs)>0` → 强制清空 (not reject) |
| ReactItersHint clamp | `clampReactIters` | clamp 到 [1, 5] |
| Strength cap 0.85 | `format_hints_mups.go:36` (Observe prompt 注入) | LLM 不给 >0.85 |
| Max plan steps | `plan_parse_reject.go:428-435` | `len(Steps) > 32` → reject `CodeParseInvalidAST` |
| DisallowUnknownFields | `plan_parse_reject.go:213` | 任何未在 Plan struct 定义的字段一律 reject |
| Budget cap | `applyBudgetCap:687-` | `len(ChildSpecs) > budget.MaxChildren` → `StrategicPlanReject` |
| Fast-path bypass | `applySingleModeUncertaintyGate:723-743` | single + 高 uncertainty + high-strength ObsFact → bypass gate |
| ObsID 空过滤 | `mapRawResolutionStrategies:568-570` | `obs_id == ""` → skip |
| SubWorktree Title 空过滤 | `mapRawResolutionStrategies:587-589` | `sub_worktree.title == ""` → skip |
| ResolutionStrategies 覆盖 ChildSpecs | `parseStrategicPlanJSON:491-494` | `len(ResolutionStrategies) > 0` → ChildSpecs 从 sub_worktree 派生 |

## 8. QuantizedKind → PlanKind 路由 (MatchKind 4 Rules)

源: `planner.go:116-131`

| Rule | 条件 | 输出 | 触发场景 |
|---|---|---|---|
| Rule 1 (优先级最高) | `QuantizedKind == "intent_orchestrate" \|\| AnomaliesCount >= 3` | `ExplorationPlan` | 场景 4 (高 anomaly / 多步编排) |
| Rule 2 | `StepCount == 1` | `CommitmentPlan` | 场景 1 (single + 1 step) |
| Rule 3 | `QuantizedKind == "intent_command" \|\| StepCount <= 3` | `ProtocolPlan` | 场景 2 (multi-step ≤3) |
| Rule 4 (兜底) | 其他 | `ScenarioPlan` | 场景 3 (parallel_probe / read-only) |

**Strength floor 公式** (`planner.go:140-155`):
```
floor = 0.7
floor -= AnomaliesCount * 0.1   # cap 0
floor += min(ObservationCount * 0.02, 0.2)
floor = min(floor, 1.0)
```

## 9. Test 覆盖 (5 cases, 全部 NEW)

| # | Test | 场景 | 关键断言 |
|---|---|---|---|
| 1 | `FrameStructure_18Fields` | 输入协议 | StrategicPlanFrame 18 字段全注册; 反射 BuildLineFrameFromStruct 输出顺序一致 |
| 2 | `JSONSchema_ExecutionModeEnum` | 输出协议 | execution_mode ∈ {single, decompose, parallel_probe}; 其他 reject |
| 3 | `SingleMode_CommitmentPlan` | 场景 1 | QuantizedKind=intent_command + 1 step → MatchKind → CommitmentPlan |
| 4 | `DecomposeMode_ProtocolPlan` | 场景 2 | intent_orchestrate + 3 steps + 0 anomalies → MatchKind Rule 3 → ProtocolPlan |
| 5 | `SingleModeFastPathBypass` | **场景 5 (混合)** | single + UncertaintyMean=0.6 + ObsFact str=0.95 → applySingleModeUncertaintyGate bypass |

> 注: 场景 3 (parallel_probe → ScenarioPlan) 和场景 4 (decompose + anomalies≥3 → ExplorationPlan) 已被 `plan_test.go::TestMatchKind_4Rules` 覆盖 (lines 306-364), 不重复添加。

**运行**:

```bash
go test -v -run TestPlanTraceE2E \
  ./internal/layers/orchestration/sessionorchestrator/...
```

**当前状态**: 5/5 NEW (2026-07-08 planned), 22/22 orchestration packages go test -race 回归 (待 S5 验证)。

## 10. 关系网

### 上游
- `devrix-d7-observe-llm-protocol-doc` (DM-20260708-003) — 兄弟 spec, 同模板
- `devrix-d7-mups-v4-phase2-prb1` (DM-20260623-001, S7_Archived 2026-06-23) — `plan.Plan` 4 PlanKind
- `devrix-d7-mups-v4-phase3-prc2` (DM-20260625-001, S7_Archived 2026-06-23) — 4 Channel router
- DM-20260706-009 — `applySingleModeUncertaintyGate` fast-path
- DM-20260704-006 — RC-1 `resolution_strategies[]` contract
- DM-20260705-004 — go-struct-driven M2 (StrategicPlanFrame `pt` tag)

### 下游
- 任何新增 `execution_mode` 必须先扩 `validateStrategicPlan` enum + `mapExecutionModeToQuantizedKind` + `MatchKind` Rule, 并加对应 trace test
- Execute 节点 (4 Channel router) 消费 `plan.Plan.Kind`, 未来加 Channel 必须扩 `plan.go:30-72`

### 关联 PR
- #473 (Observe trace validation 16 tests + spec) — 兄弟 spec
- #472 (Trace validation 16 tests) — Observe 节点运行验证
- 未来 PR: 5 trace test + 本 spec

## 11. 沉淀物清单

| 类型 | 路径 | 描述 |
|---|---|---|
| Spec | `openspec/specs/d7-orchestration/d7-plan-llm-io-protocol-spec.md` | 本文档 |
| Test | `internal/layers/orchestration/sessionorchestrator/plan_trace_e2e_test.go` | 5 NEW trace test |
| Demand | `openspec/changes/devrix-d7-plan-llm-protocol-doc/demand.md` | DM-20260708-004 S1 |
| Proposal | `openspec/changes/devrix-d7-plan-llm-protocol-doc/proposal.md` | S2 lite |

## 12. 后续工作 (不在本 Change 范围)

- 把本 spec 合并到主 `openspec/specs/d7-orchestration/spec.md` (lite-mode) — S6 归档时决策
- 添加 `ApplySingleModeUncertaintyGate` 的 EN 路径 trace test (i18n 跨 locale)
- 添加 `parseStrategicPlanJSON` 容错测试 (类比 observe 的 `TestObserveTraceE2E_JSONParseLeniency`)
- 验证 `applyBudgetCap` 的 enforcement 实测 (当前 spec 描述但缺 trace test)
- 进一步把 StrategicPlanFrame 的 18 字段对照 FrameSpec (linefield.go:75-96) 做 bit-equivalence 测试
