# Spec: D7 Observe 节点全协议（双轨架构 + 证据剖面）

**Domain**: D7 (Orchestration)
**Feature**: observe-node
**Status**: S3_Design
**Change ID**: `devrix-d7-observe-node-spec`
**Demand ID**: DM-20260711-001
**Versions**: d7-orchestration v4.29.0 → v4.30.0
**Change Package**: `openspec/changes/devrix-d7-observe-node-spec/`
**Supersedes**: `d7-observe-llm-io-protocol-spec.md` §5 场景章节（LLM-only 叙事）
**Complements**: `d7-observational-fastpath-spec.md`（fast-path 闸门）、`mups-frame-delta-spec.md`（Observe→Plan delta）
**Related Demand**: DM-20260708-003（原文档）、DM-20260705-009（封闭式分类器）、DM-20260706-011（fast-path）

---

## 1. 范围与定位

### 1.1 本 spec 回答什么

| 维度 | 问题 | 本 spec 章节 |
|------|------|-------------|
| 定位 | Observe 节点是什么、不是什么？ | §2 |
| 架构 | Go 机械层与 LLM 分类层如何合并？ | §3 |
| 类型 | 4 Kind × 2 Category 语义与约束？ | §4 |
| 输入 | **类型无关**证据帧（struct / LLM frame / signal 词汇表）？ | §4 |
| 输出 | **ObsKind 唯一声明处**（LLM `kind` + Go 机械规则）？ | §5 |
| 路由 | Partition 与 fast-path 如何决策？ | §6 |
| 剖面/用例 | 证据剖面 E0–E7 + 用例实例 + 期望分类？ | §7 |
| 反查 | OBS-O 期望输出 → (E 剖面 + directive) 反推？ | §12 |
| 缺口 | 当前实现与目标设计的差距？ | §8 |
| 验证 | 测试如何锁死契约？ | §9 |

### 1.2 Observe 节点是什么

**Observe = 观测聚合器（Go 确定性）+ 封闭式分类器（LLM 概率）**

- **输入**：用户 directive、WorkItem 状态、ScopeContract、LastRound 信号、子节点 bubbles、跨轮反馈
- **输出**：`UncertaintyReport`（`[]Observation` + Intent + Prior + Partition）
- **职责**：汇聚证据 → **在输出侧**类型化为 4 种 `ObservationKind`，供 Plan / fast-path / Learn 消费
- **不做**：执行工具、拆解任务、评估完成度、开放散文分析

**关键原则（协议分层）**：

| 层级 | 是否携带 ObsKind？ |
|------|-------------------|
| **LLM 输入帧** | **否** — 仅证据字段（directive / signal / scope 等） |
| **LLM 输出 JSON** | **是** — `kind` 为分类器唯一声明处 |
| **Go 机械轨** | **是** — 由 WorkItem 状态规则确定性产出（与 LLM 输入帧无关） |
| **UncertaintyReport** | 合并后含多种 kind，路由读报告而非读输入 |

封闭式分类器角色（DM-20260705-009，源：`format_hints_mups.go`）：

```
输入 = 类型无关证据帧（directive + 可选 signal/scope/…）
输出 = Obs* JSON 数组 — kind 在此处由模型判定
附录引导（非输入字段）：signal 不足 → 倾向 obs_uncertainty；闭式问答 → 倾向 obs_fact
```

### 1.3 Observe 节点不是什么

| 不是 | 归属节点 |
|------|---------|
| 工具执行 / ReAct 循环 | Execute |
| execution_mode / child_specs 决策 | Plan |
| deliverable 验收 | Verify |
| 信誉写入 | Learn |
| `prior_artifact_summary` / `known_gaps` 的 LLM 解读 | Plan frame delta（不进 Observe LLM） |

### 1.4 与旧 spec 的关系

| 文档 | 范围 | 状态 |
|------|------|------|
| **本文档 `observe-node-spec.md`** | 全节点：机械 + LLM + 合并 + 路由 + 证据剖面 | **SoT（全节点）** |
| **`d7-observe-llm-io-protocol-spec.md`** | **LLM 交互完整 I/O**（Review 版 §0–§5） | **SoT（LLM 子路径 Review）** |
| `d7-observational-fastpath-spec.md` | fast-path 四闸门 | 正交，本文 §6 引用 |

---

## 2. 节点架构（双轨）

```
┌─────────────────────────────────────────────────────────────────────────┐
│  observeWorkItem (item_observe.go)                                       │
│                                                                          │
│  ┌─ Go 机械轨（0 LLM）──────────────────────────────────────────────┐  │
│  │ observationsFromItem          → intent / directive-echo / unc    │  │
│  │ mapScopeContractToObservations → scope open Q / scope goal fact   │  │
│  │ observationsFromChild*        → structured / summary / checklist  │  │
│  │ observeDeliverableSignals       → deliverable_incomplete 等        │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                              +                                           │
│  ┌─ LLM 分类轨 ─────────────────────────────────────────────────────┐  │
│  │ buildObserveSignalInput → observeLLMFieldMap(11→6) → InvokeStream  │  │
│  │ → parseObservationProposalsJSON → validateOneProposal (max=3)    │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                              ↓                                           │
│  NewUncertaintyReport → Partition → item_pipeline 路由                     │
└─────────────────────────────────────────────────────────────────────────┘
```

**合并顺序**（影响 `pickHighStrengthBusinessFact` 遍历顺序，§6.2 注意）：

1. `observationsFromItem`
2. `mapScopeContractToObservations`
3. `observationsFromChildStructuredBubbles`
4. rollup：`observationsFromChildSummaryBubbles` + `observationsFromChecklistChildBubbles`
5. `mergeProposedObservations`（LLM）
6. `observeDeliverableSignals`

---

## 3. 类型体系

### 3.0 ObsKind 何时确定（非输入假设）

```
ObserveSignalInput / LLM user frame
        │  （无 kind 字段）
        ▼
   ┌────┴────┐
   ▼         ▼
 Go 规则    LLM 分类器
   │         │
   │         └──► 输出 JSON.kind  ◄── LLM 路径唯一声明处
   └──► 直接 NewObservation(kind=…)
        │
        ▼
   UncertaintyReport（多 kind 并存）
```

- **同一证据剖面**（如 E0：仅 `directive`）下，`2×3=几?` 与 `review d7 plan/` 输入帧形状相同，**LLM 输出 kind 可以不同**。
- 文档中的「期望 kind」= **appendix 引导 + trace 锁定的分类器期望**，不是输入协议字段。

### 3.1 四种 ObservationKind

| Kind | 语义 | Payload 必填 | strength 特殊规则 |
|------|------|-------------|------------------|
| `ObsFact` | 已验证事实 / 可直接作答 | `statement` | Go cap **≤ 0.85** |
| `ObsSignal` | 量化或重复状态信号 | `name` | `value=strength`，`threshold=0.5` 硬编码 |
| `ObsDeviation` | 相对基线偏离 | `metric`（=statement） | `expected=0`，`observed=delta=strength` |
| `ObsUncertainty` | 未闭合问题 | `question` | `confidence = 1 - strength` |

源：`orchtypes/observation.go`、`validateOneProposal`（`observation_proposer.go`）。

### 3.2 两种 Category

| Category | 参与 Overall 均值 | Partition 去向 | 典型来源 |
|----------|------------------|---------------|---------|
| `CatBusiness` | ✅ | `BusinessObservations` | LLM 默认；绝大多数 Go 机械源 |
| `CatSystem` | ❌ | `SystemObservations`；满足条件 → `Anomalies` | **当前 LLM 路径未赋值**（见 §9.2） |

**Partition 异常规则**（`uncertainty_report.go:Partition`）：

- `CatSystem + ObsDeviation` → `Anomalies`
- `CatSystem + ObsUncertainty` 且 `strength ≥ 0.7` → `Anomalies`
- `CatSystem + ObsFact` → 仅 `SystemObservations`

### 3.3 机械观测 Source 注册表

| Source | 函数 | 产出 Kind | 阻断 fast-path？ |
|--------|------|----------|-----------------|
| `item_pipeline` | `observationsFromItem` | Fact / Signal / Uncertainty | Uncertainty **不阻断**（source 过滤） |
| `scope_contract` | `mapScopeContractToObservations` | Uncertainty / Fact | Uncertainty **阻断** |
| `context_structured_bubble` | child bubbles | Fact | 否 |
| `context_summary_bubble` | rollup summary | Fact | 否 |
| `context_checklist_bubble` | rollup checklist | Fact | 否 |
| `verify_signal` | `observeDeliverableSignals` | Signal / Fact | 否 |
| `observation_proposer` | LLM `validateOneProposal` | 任意 4 kind | LLM Uncertainty **阻断** |

`hasObsUncertainty` 排除 `item_pipeline` 与 `verify_signal`（`deliverable_execute.go:198`）。

---

## 4. 输入协议（三层，类型无关）

> **输入协议不编码 ObsKind。** 所有用户用例共享同一 schema；差异仅为字段是否非空及取值。用例分类见 §7 证据剖面 **OBS-E0–E7**。

### 4.0 唯一输入 schema（LLM 可见 6 标签）

| 标签 | 出现条件 | 角色 |
|------|---------|------|
| `directive` | 无条件 | 主证据 |
| `prior_parse_reject` | 非空 | 跨轮格式反馈（control） |
| `scope_goal` | 非空 | 已收缩目标（data） |
| `scope_open_question` | len>0 | 待闭合问题作证据（data） |
| `signal` | len>0 | 注册表前缀行（data） |
| `prior_observation_ids` | len>0 | 跨轮锚点（control） |

### 4.1 Layer A — `ObserveSignalInput` struct（11 frame 字段）

源：`observation_proposer.go:27-70`；frame 顺序：`prompttags.ObserveUserFrame`。

| # | 字段 | Go 类型 | Plane | 给 LLM？ | 出现条件 |
|---|------|---------|-------|---------|---------|
| — | `SessionID` | string | — | ❌ | 路由 only（`pt:"-"`） |
| 1 | `WorkItemID` | string | control | ❌ | 始终填充；Go evidence 兜底 |
| 2 | `Directive` | string | data | ✅ | **无条件** |
| 3 | `PriorParseReject` | string | control | ✅ | `TrimSpace != ""` |
| 4 | `PriorMean` | float64 | control | ❌ | Learn 输出；防锚定 |
| 5 | `ScopeGoal` | string | data | ✅ | `TrimSpace != ""` |
| 6 | `ScopeOpenQuestions` | []string | data | ✅ | `len > 0` |
| 7 | `InboundSignalLines` | []string | data | ✅ | `len > 0`（pt: `signal`） |
| 8 | `PriorObservationIDs` | []string | control | ✅ | `len > 0` |
| 9 | `IncrementalOnly` | bool | control | ❌ | `PriorObservationIDs` 非空时 true |
| 10 | `PriorArtifactSummary` | string | data | ❌ | Plan frame delta |
| 11 | `KnownGaps` | []string | data | ❌ | Plan frame delta（Phase 2 stub） |

> **计数约定**：11 = frame 字段数；不含 `SessionID`。

### 4.2 Layer B — LLM user frame（动态渲染）

由 `observeLLMFieldMap`（`llm_observation_proposer.go:69`）+ `BuildLineFrame` 渲染。

**渲染规则**：

- 仅 map 中存在的 key 输出；**空 slice / 空字符串不出现**
- 格式：`key: value\n`（多行字段每元素一行）
- 前缀：i18n `RenderFrameFieldGuideForFields`（仅对已出现字段）

**LLM 可见字段全集（6 标签，均 omit_empty）**：

| 标签 | 条件 |
|------|------|
| `directive` | 无条件 |
| `prior_parse_reject` | 非空 |
| `scope_goal` | 非空 |
| `scope_open_question` | len > 0 |
| `signal` | len > 0 |
| `prior_observation_ids` | len > 0 |

**示例 — OBS-E0 剖面（仅 directive，与 ObsKind 无关）**：

```
directive: 2×3=几?
```

（同一 E0 剖面亦可为 `directive: review d7 plan 目录` — 输入协议相同，输出 kind 由模型判定。）

**示例 — 全字段非空（`TestObserveTraceE2E_OnlyFieldsVisibleToLLM`）**：

```
directive: review d7 plan 目录
prior_parse_reject: 上一轮 strength 越界 1.0
scope_goal: review d7 编排层
scope_open_question: 是否包括 plan 子包?
scope_open_question: test 覆盖到 plan/ 吗?
signal: artifact_summary: 之前的 attempt 失败
signal: child_downlink_scope_in: d7/plan/
prior_observation_ids: obs_1, obs_2
```

（`work_item_id`、`prior_mean`、`incremental_only`、`prior_artifact_summary`、`known_gaps` **不出现**。）

### 4.3 Layer C — signal 行词汇表（生产 SoT）

`buildObserveSignalInput`（`observation_proposer.go:106`）**仅**生成下列前缀：

| 前缀 | 触发条件 | 示例 |
|------|---------|------|
| `artifact_summary:` | `LastRound.ArtifactSummary` 非空 | `artifact_summary: connection refused (3rd retry)` |
| `child_downlink_scope_in:` | 父项有 child downlink ScopeIn | `child_downlink_scope_in: d7/plan/, d7/observe/` |
| `expected_return:` | child downlink ExpectedReturn 非空 | `expected_return: <deliverable_schema>p0_p1_file_line</deliverable_schema>` |

**未注册前缀**（如 `p99_latency_ms: 245`）当前 **不会** 由 Go wiring 自动注入。若将来支持指标行，须：

1. 在词汇表注册
2. 更新 i18n appendix
3. 增加 trace test

---

## 5. 输出协议（ObsKind 声明处）

### 5.0 两轨输出对比

| 轨道 | ObsKind 来源 | 时机 |
|------|-------------|------|
| **LLM** | 输出 JSON 的 `kind` 字段 | `parseObservationProposalsJSON` 之后 |
| **Go 机械** | 代码规则（见 §5.3） | `observeWorkItem` 合并前，**不读 LLM 输入帧** |

### 5.1 LLM 分类器输出（JSON 数组）

源：`llm_observation_proposer.go:118-179`；appendix：`format_hints_mups.go`。

```json
[
  {
    "kind": "obs_fact | obs_signal | obs_deviation | obs_uncertainty",
    "strength": 0.0,
    "statement": "string",
    "question": "string",
    "evidence": ["string"]
  }
]
```

| 约束 | 行为 |
|------|------|
| 必须 JSON 数组 | 散文包裹时截取首个 `[`…`]` |
| `kind` 别名 | `fact`/`obs_fact` 等大小写不敏感 |
| 空 `[]` / 空串 | `(nil, nil)` → 无 LLM 提案 |
| max proposals | **3**（`ValidateObservationProposals` 截断） |
| Category | LLM 路径 **恒为 `CatBusiness`**（`mapRawObsProposals:156`）；CatSystem 由 Go promote（§8.2） |

> **`kind` 是本协议中 LLM 侧对 Obs 类型的唯一声明。** 输入帧无任何对应字段。

### 5.2 Go 兜底（validateOneProposal）

| 规则 | 行为 |
|------|------|
| strength ≤ 0 | lift → 0.5 |
| ObsFact strength > 0.85 | cap → 0.85 |
| ObsFact 空 statement | reject |
| ObsSignal 空 statement | default `"llm_signal"` |
| ObsDeviation 空 statement | reject |
| ObsUncertainty 空 question | fallback statement → 仍空则 reject |
| evidence | append `WorkItemID` + `SessionID`（若缺失） |

### 5.3 机械轨规则产出（Go 状态 → kind，与 LLM 输入无关）

**`observationsFromItem`**（每轮）：

| 条件 | Kind | Statement / Name | Strength |
|------|------|-----------------|----------|
| `item.Uncertainty ≥ 0.6` | ObsUncertainty | directive 原文 | item.Uncertainty |
| `intent == Orchestrate` | ObsSignal | `orchestrate_intent` | prior mean 或 0.55 |
| 始终 | ObsFact | **directive 原文 echo** | prior mean 或 **0.85** |

> ⚠️ directive echo ObsFact 在 prior mean ≥ 0.85 时会参与 fast-path 竞选（§9.1）。

**`mapScopeContractToObservations`**：

| 条件 | Kind |
|------|------|
| `HasOpenQuestions()` | 每条 open Q → ObsUncertainty（str=0.9） |
| `IsCompleteEnoughForDecompose()` | ObsFact（goal 或 ScopeIn 摘要） |

**`observeDeliverableSignals`**（`DeliverableStatusIncomplete`）：

| Kind | Name / Statement |
|------|-----------------|
| ObsSignal | `deliverable_incomplete` |
| ObsSignal | `evidence_tool_calls`（若有 tool calls） |
| ObsFact | `deliverable_reason: …` |

---

## 6. 路由

### 6.1 UncertaintyReport.Partition

见 §3.2。`Overall = mean(BusinessObservations.strength)`；CatSystem 不参与。

### 6.2 Fast-path 四闸门（Observe 之后）

源：`item_pipeline.go:300`；详规：`d7-observational-fastpath-spec.md`。

| # | 条件 | Observe 相关 miss 原因 |
|---|------|----------------------|
| G1 | 非 rollup / 非 deliverable synth | 父项合成 |
| G2 | `Learner != nil` | 无 Learn |
| G3 | `!hasObsUncertainty(report)` | LLM 或 scope_contract Uncertainty |
| G4 | `pickHighStrengthBusinessFact(≥0.85)` 命中 | 无合格 ObsFact；CatSystem；低 strength |

**G4 选题规则（当前实现）**：遍历 `report.Observations` **按合并顺序**，首个 `CatBusiness ObsFact` 且 `strength ≥ threshold` 且 statement 非空者胜出。**无 source 优先级**。

---

## 7. 证据剖面、用例实例与分类期望

### 7.1 组织原则（修正）

| 维度 | ID 前缀 | 是否含 ObsKind？ | 说明 |
|------|---------|-----------------|------|
| **证据剖面** | `OBS-E*` | **否** | 仅描述 LLM 输入帧哪些字段非空 |
| **用例实例** | `OBS-U*` | **否** | 用户意图 + 绑定 E 剖面 + directive 示例 |
| **LLM 分类期望** | 附在用例后 | **是（输出侧）** | appendix 引导 / trace canned，**非输入假设** |
| **LLM 输出组合** | `OBS-O*` | **是** | 分类器输出空间 |
| **机械层规则** | `OBS-M*` | **是** | Go 状态 → kind，独立于 LLM 输入 |
| **路由负向** | `OBS-G*` | — | fast-path 闸门 |
| **协议不变量** | `OBS-I*` | — | 过滤/解析 |

> **禁止**用 ObsKind 命名输入协议（如「ObsSignal 输入场景」）。旧 `OBS-S*` 已废弃，映射见 §7.8。

---

### 7.2 证据剖面 OBS-E0–E7（输入协议唯一点）

描述 **LLM user frame 形状**；与 directive 文案无关；**不预判 kind**。

| ID | 非空 LLM 标签 | User frame 形状 | Layer A 触发条件 |
|----|--------------|----------------|-----------------|
| **E0** | 仅 `directive` | 1 行 | 首轮；无 LastRound 摘要；无 scope；无 parse reject |
| **E1** | `directive` + `signal`（artifact_summary） | 2+ 行 | `LastRound.ArtifactSummary` 非空 |
| **E2** | `directive` + `scope_open_question`×N | 2+ 行 | `ScopeContract.OpenQuestions` 非空 |
| **E3** | `directive` + `scope_goal` | 2 行 | `ScopeContract.GoalStatement` 非空 |
| **E4** | `directive` + `prior_observation_ids` | 2 行 | `LastRound.ObservationIDs` 非空 |
| **E5** | `directive` + `prior_parse_reject` | 2 行 | `LastRound.ObserveParseReject` 非空 |
| **E6** | `directive` + `signal`（downlink） | 3+ 行 | child downlink ScopeIn / ExpectedReturn |
| **E7** | 6 标签全出现 | 多行 | 压力/全字段测试 |

**同剖面、不同 kind 示例（E0）**：

| directive 示例 | LLM user frame | 可能 LLM `kind`（模型判定） |
|---------------|----------------|---------------------------|
| `2×3=几?` | `directive: 2×3=几?` | `obs_fact`（appendix 倾向） |
| `review d7 plan 目录` | `directive: review d7 plan 目录` | `obs_uncertainty`（appendix 倾向） |

输入协议 **完全相同**；差异仅在 **输出 `kind`**。

---

### 7.3 用例实例 OBS-U01–U12

每例：**输入（E 剖面 + 实例值）** → **LLM 分类期望（输出侧）** → **机械层（OBS-M）** → **路由**。

#### OBS-U01 — 闭式问答（E0）

| 栏 | 内容 |
|----|------|
| **用户意图** | 算术/常识/定义，可闭式作答 |
| **输入剖面** | **E0** |
| **Layer A** | `Directive="2×3=几?"` |
| **Layer B** | `directive: 2×3=几?` |
| **LLM 分类期望** | `obs_fact` str=0.85，statement 含完整答案（**非输入字段**） |
| **机械层 OBS-M** | M-default（§7.4） |
| **路由** | G3 ✓ G4 ✓ → `observational_answer` |
| **测试** | `ObsFact_FastPathTrigger`, `FullPipeline_FactFastPath` |

#### OBS-U02 — 开放任务（E0）

| 栏 | 内容 |
|----|------|
| **用户意图** | 任务型 directive，无额外证据 |
| **输入剖面** | **E0**（与 U01 同形！） |
| **Layer B** | `directive: review d7 领域 plan目录下代码` |
| **LLM 分类期望** | `obs_uncertainty` + question（非 `[]`） |
| **机械层** | M-default |
| **路由** | G3 ✗（若 LLM 产 uncertainty）→ Plan |
| **测试** | `OpenDirectiveNoSignal_ClassifierPrompt` |

#### OBS-U03 — 模糊 directive + scope 问题（E2）

| 栏 | 内容 |
|----|------|
| **输入剖面** | **E2** |
| **Layer B** | `directive: 帮我优化一下` + 2× `scope_open_question:` |
| **LLM 分类期望** | `obs_uncertainty` |
| **机械层** | M-default + M-scope-openQ（**Go 已注入 uncertainty，与 LLM 输入独立**） |
| **路由** | G3 ✗ → Plan |
| **测试** | `ObsUncertainty_PlanDecompose` |

#### OBS-U04 — 重试 + 上轮摘要（E1）

| 栏 | 内容 |
|----|------|
| **输入剖面** | **E1** |
| **Layer B** | `directive: 再试一次` + `signal: artifact_summary: connection refused (3rd retry)` |
| **LLM 分类期望** | 倾向 `obs_signal`（**亦可能** uncertainty/deviation，由模型判） |
| **trace 绑定** | canned `obs_signal` statement=`重复 attempt` |
| **机械层** | M-default |
| **路由** | G4 ✗ → Plan |
| **测试** | `ObsSignal_StructuredMetric` |

#### OBS-U05 — 跨轮增量（E4，可叠加 E1）

| 栏 | 内容 |
|----|------|
| **输入剖面** | **E4**（常 + E1） |
| **Layer B** | `prior_observation_ids: obs_001` + 视情况 `signal:` |
| **LLM 分类期望** | 新 obs，evidence 引用已知 id |
| **机械层** | M-default |
| **测试** | `OnlyFieldsVisibleToLLM`（字段）；dedup 待补 |

#### OBS-U06 — 混合分类输出（E0 + 输出组合 O07）

| 栏 | 内容 |
|----|------|
| **输入剖面** | **E0**（`directive: 2×3=几?`） |
| **LLM 分类期望** | **同时** fact + uncertainty（矛盾，边界用例） |
| **路由** | G4 ✓ 且 G3 ✗ → Plan |
| **测试** | `FactPlusUncertainty_FastPathBlocked` |

#### OBS-U07 — Rollup 父项（E0 + 机械层主导）

| 栏 | 内容 |
|----|------|
| **输入剖面** | **E0** 典型 |
| **LLM** | 任意/空；**不决定本用例** |
| **机械层** | M-rollup-bubbles |
| **路由** | G1 ✗ → rollup |
| **测试** | `ObserveWorkItem_RollupDualBubbles` |

#### OBS-U08 — Deliverable 未完成（E0/E1 + 机械层主导）

| 栏 | 内容 |
|----|------|
| **机械层** | M-deliverable-incomplete |
| **LLM** | 任意/空 |
| **路由** | Plan |
| **测试** | 待补 trace |

#### OBS-U09 — 父项 child downlink（E6）

| 栏 | 内容 |
|----|------|
| **Layer B** | `signal: child_downlink_scope_in:…` + `expected_return:…` |
| **LLM 分类期望** | 倾向 uncertainty 或 signal |

#### OBS-U10 — parse 自纠（E5）

| 栏 | 内容 |
|----|------|
| **Layer B** | `prior_parse_reject: {compact JSON}` |
| **LLM 分类期望** | 修正后的合法 JSON 数组 |

#### OBS-U11 — scope 齐备（E3）

| 栏 | 内容 |
|----|------|
| **Layer B** | `scope_goal: …` |
| **机械层** | M-scope-goal-fact（若 `IsCompleteEnoughForDecompose`） |

#### OBS-U12 — 多意图复合（E0）

| 栏 | 内容 |
|----|------|
| **Layer B** | `directive: 1+1=几 + 查 devrix 项目结构` |
| **LLM 分类期望** | fact + uncertainty（见 fastpath spec） |

---

### 7.4 机械层规则 OBS-M（Go 状态 → kind）

**每轮默认（M-default，`observationsFromItem`）**— 与用户用例类型无关：

| 规则 | kind | source | 阻断 fast-path？ |
|------|------|--------|-----------------|
| `uncertainty ≥ 0.6` | ObsUncertainty | item_pipeline | **否** |
| `intent == Orchestrate` | ObsSignal | item_pipeline | 否 |
| 始终 | ObsFact（directive echo） | item_pipeline | 否（冷启动 str≈0.625） |

| ID | 触发 | 额外产出 |
|----|------|---------|
| **M-scope-openQ** | `HasOpenQuestions()` | 每条 → ObsUncertainty str=0.9，`scope_contract` |
| **M-scope-goal** | `IsCompleteEnoughForDecompose()` | ObsFact，`scope_contract` |
| **M-rollup-bubbles** | `NeedsRollup` + 子节点完成 | ObsFact×N，`context_*_bubble` |
| **M-deliverable-incomplete** | `DeliverableStatusIncomplete` | ObsSignal/Fact，`verify_signal` |

---

### 7.5 LLM 输出组合 OBS-O（分类器输出空间）

| ID | LLM 输出 `kind` 组合 | G3 | G4 | 下游 | 绑定用例 |
|----|---------------------|----|----|------|---------|
| OBS-O01 | `[]` | — | ✗ | Plan fallback | — |
| OBS-O02 | 1× fact str≥0.85 | ✓ | ✓ | fast-path | U01 |
| OBS-O03 | 1× fact str<0.85 | ✓ | ✗ | Plan | StrengthClamping |
| OBS-O04 | 1× uncertainty | ✗ | ✗ | Plan | U02/U03 |
| OBS-O05 | 1× signal | ✓ | ✗ | Plan | U04 |
| OBS-O06 | 1× deviation | ✓ | ✗ | Plan / Anomalies* | U04 可能 |
| OBS-O07 | fact + uncertainty | ✗ | ✓* | Plan | U06, U12 |
| OBS-O08 | 4 条 → 截 3 | — | ✗ | Plan | MaxProposalsTruncated |

\* G4 命中 fact 但 G3 阻断；\* deviation 进 Anomalies 需 Go promote CatSystem。

**正查**（输入 → 期望输出）：§7.3 OBS-U01–U12。**反查**（期望输出 → 用户指令）：§12。

---

### 7.6 fast-path 负向 OBS-G

| ID | Miss Gate | 典型构造 |
|----|-----------|---------|
| OBS-G01 | G1 | U07 rollup |
| OBS-G02 | G2 | Learner=nil |
| OBS-G03 | G3 | U03/U06 LLM 或 scope uncertainty |
| OBS-G04 | G4 | OBS-O03 低 strength |
| OBS-G05 | G4 | CatSystem fact |
| OBS-G06 | G4 | 空 statement → Go reject |

---

### 7.7 Partition OBS-P

| ID | 报告中的 kind+category | Business | Anomalies |
|----|----------------------|----------|-----------|
| OBS-P01 | business fact+signal | 2 | 0 |
| OBS-P02 | system deviation | 0 | 1 |
| OBS-P03 | system uncertainty str≥0.7 | 0 | 1 |
| OBS-P04 | 3 kind 混合 max=3 | 2 | 1 |

---

### 7.8 协议不变量 OBS-I

| ID | 主题 | 测试 |
|----|------|------|
| OBS-I01 | 11→6 字段过滤 | `OnlyFieldsVisibleToLLM` |
| OBS-I02 | prior_mean 不进 LLM | `BayesianPrior_GoSideOnly` |
| OBS-I03 | kind 别名 | `KindAliasCaseInsensitive` |
| OBS-I04 | JSON 解析容错 | `JSONParseLeniency` |
| OBS-I05 | ObsFact cap | `StrengthClamping` |
| OBS-I06 | uncertainty question fallback | `UncertaintyQuestionFallback` |
| OBS-I07 | 封闭式分类器 prompt | `OpenDirectiveNoSignal_ClassifierPrompt` |

---

### 7.9 旧 ID 映射与文档勘误

| 废弃 ID | 新结构 | 说明 |
|---------|--------|------|
| OBS-S01 | **E0** + U01 | 输入不是「ObsFact 场景」 |
| OBS-S02 | **E0** + U02 | 与 S01 **同剖面**，不同期望 kind |
| OBS-S03 | **E2** + U03 | |
| OBS-S04 | **E1** + U04 | 输入不是「ObsSignal 场景」 |
| OBS-S05 | **E4** + U05 | |
| OBS-S06 | **E0** + U06 | |
| OBS-S07 | **E0** + U07 | |
| OBS-S08 | **E0/E1** + U08 | |

| 旧 spec §5 场景 | 问题 |
|----------------|------|
| 场景 2 cache 命中率 | 虚构输入；应为 E0/E2 + 输出期望 |
| 场景 3 p99 latency | 虚构 signal 前缀 |
| 场景 4 监控目录 | 虚构；Anomalies 属输出+Go promote |

---

## 8. 已知缺口与后续重构

### 8.1 P1 — fast-path 选题无 source 优先级

**现象**：`pickHighStrengthBusinessFact` 按合并顺序取首个合格 ObsFact。`item_pipeline` directive echo 在 prior mean ≥ 0.85 时 strength 亦 ≥ 0.85，可能抢在 LLM 答案前被选中。

**建议**：

- 方案 A：`pickHighStrength` 仅接受 `source=observation_proposer`
- 方案 B：directive echo 改为 ObsSignal（`name=directive_echo`），退出 fact 池

### 8.2 P2 — CatSystem 提升未实现

**现象**：LLM 恒 `CatBusiness`；ObsDeviation→Anomalies 仅在测试手动 `Category=CatSystem`。

**建议**：新增 `promoteSystemCategory(obs, signals)`，规则见 signal 中 baseline/observed delta 模式。

### 8.3 P3 — scope 双重注入

**现象**：`scope_open_question` 同时进 LLM frame 与 `mapScopeContractToObservations`。

**建议**：二选一（推荐保留 Go 机械层，LLM frame 省略 open questions）。

### 8.4 P4 — signal 词汇表未注册扩展项

**现象**：文档/旧 spec 中的 `metric_kv` 行不在 `buildObserveSignalInput`。

**建议**：注册表 + i18n + wiring 三件套后再写进场景矩阵。

### 8.5 P5 — observeLLMFieldMap 与 pt tag 双 SoT

**建议**：由 `FrameObserveUser` + omit 规则生成 LLM 可见字段，删除手写 map。

---

## 9. 测试覆盖清单

```bash
go test -v -run 'TestObserveTraceE2E|TestObservePipeline_|TestObserveWorkItem_' \
  ./internal/layers/orchestration/sessionorchestrator/...
```

| 用例/剖面 ID | 测试函数 | 状态 |
|-------------|---------|------|
| E7 + OBS-I01 | `OnlyFieldsVisibleToLLM` | ✅ |
| E0 + U01 | `ObsFact_FastPathTrigger`, `FullPipeline_FactFastPath` | ✅ |
| E0 + U02 | `OpenDirectiveNoSignal_ClassifierPrompt` | ✅ |
| E2 + U03 | `ObsUncertainty_PlanDecompose` | ✅ |
| E1 + U04 | `ObsSignal_StructuredMetric` | ✅ |
| E4 + U05 | dedup 行为 | ⚠️ 待补 |
| E0 + U06 | `FactPlusUncertainty_FastPathBlocked` | ✅ |
| E0 + U07 | `ObserveWorkItem_RollupDualBubbles` | ✅ |
| U08 | — | ❌ 待补 |
| OBS-P02 | `ObsDeviation_AnomalyTrigger` | ✅（hack） |
| OBS-I01–I07 | trace §7.8 表 | ✅ |

---

## 10. 关联文档与代码索引

| 类型 | 路径 |
|------|------|
| 节点入口 | `sessionorchestrator/item_observe.go` |
| LLM 分类器 | `sessionorchestrator/llm_observation_proposer.go` |
| 输入构建 | `sessionorchestrator/observation_proposer.go` |
| fast-path | `sessionorchestrator/item_pipeline.go:300` |
| 闸门函数 | `sessionorchestrator/deliverable_execute.go` |
| 类型定义 | `orchtypes/observation.go`, `uncertainty_report.go` |
| i18n appendix | `contextengine/i18n/format_hints_mups.go` |
| frame 注册 | `shared/prompttags/linefield.go`, `semantics.go` |
| trace 验证 | `sessionorchestrator/observe_trace_e2e_test.go` |
| 旧 LLM-only spec | `d7-observe-llm-io-protocol-spec.md`（§3–§4 仍有效） |
| fast-path spec | `archive/.../d7-observational-fastpath-spec.md` |

---

## 11. 变更记录

| 日期 | 变更 |
|------|------|
| 2026-07-11 | 初稿：双轨架构、三层输入、场景矩阵 OBS-S/O/G/P/I |
| 2026-07-11 | S3 设计完成：change 包 `devrix-d7-observe-node-spec` |
| 2026-07-11 | **协议修正**：输入不携带 ObsKind；改 OBS-E 证据剖面 + OBS-U 用例实例；废弃 OBS-S* |
| 2026-07-11 | 新增 §12：OBS-O → (E, directive) 反查附录 |

---

## 12. 附录：OBS-O 反查表（期望输出 → 用户指令）

> 与 §7.3 **正查**（OBS-U：用例 → 期望输出）对称。本节从 **LLM 分类器期望输出 OBS-O** 反推 **证据剖面 E + directive 语义 + 轮次上下文** 组合，并给出典型用户指令示例。
>
> **不能**从用户指令唯一推出 ObsKind（E0 同形反例见 §7.2）。反查表描述的是 **appendix 引导 + trace 锁定** 下的期望分类，非确定性映射。

### 12.1 反推公式

```
期望 OBS-O*
  ← 证据剖面 OBS-E*（LLM user frame 哪些字段非空）
  + directive 语义类别（闭式 / 开放 / 模糊 / 复合 / 重试）
  + 轮次上下文（LastRound / ScopeContract / child downlink）
  + appendix 引导（format_hints_mups.go，非输入字段）
```

**双轨提醒**：下表仅覆盖 **LLM 分类轨**。Go 机械轨（OBS-M*）由 WorkItem 状态确定性产出，与用户指令无一对一关系；见 §12.4。

### 12.2 OBS-O → (E, directive) 反查主表

| OBS-O | LLM 输出形态 | 证据剖面 | 轮次上下文 | directive 语义 | 典型用户指令 | 绑定用例 | appendix 倾向 |
|-------|-------------|---------|-----------|--------------|-------------|---------|--------------|
| **O01** | `[]` 空数组 | E0 常见 | 首轮 | 极短/不可分类 | `？`、空串 | — | 应优先 O04 而非空数组 |
| **O02** | 1× fact str≥0.85 | **E0** | 首轮；无 scope/signal | **闭式问答** | `2×3=几?`、`Go 的 map 是什么` | U01 | 确定性问答 → fact + 完整 statement |
| **O02** | 1× fact str≥0.85 | E3 | scope_goal 齐备 | 任务已收缩 | `按已确认 scope 实现缓存层` | U11 | 同左 |
| **O03** | 1× fact str<0.85 | E0/E1 | 任意 | 推测性/低信心 | `这个 bug 可能是什么原因` | StrengthClamping | 无强制 |
| **O04** | 1× uncertainty | **E0** | 首轮 | **开放任务**（需工具/探索） | `review d7 领域 plan 目录下代码` | U02 | signal 不足 / 需工具 → uncertainty |
| **O04** | 1× uncertainty | **E0** | 首轮 | **模糊意图** | `帮我优化一下`（无 scope） | U02 | directive 模糊 → uncertainty |
| **O04** | 1× uncertainty | **E2** | ScopeContract 有 open Q | 模糊 + scope 未闭合 | `帮我优化一下` + open questions | U03 | 同左 |
| **O04** | 1× uncertainty | **E6** | child downlink | 父项编排 | 父 directive + downlink signal | U09 | 倾向 uncertainty 或 signal |
| **O04** | 1× uncertainty | E4 | 跨轮 obs id | 增量追问 | `基于上轮结论继续` | U05 | 视模型判断 |
| **O05** | 1× signal | **E1** | LastRound.ArtifactSummary | **重试/继续** | `再试一次` + `artifact_summary: connection refused` | U04 | 重复状态 → 命名 signal |
| **O05** | 1× signal | E1 | 上轮失败摘要 | 继续执行 | `继续` + `artifact_summary: timeout after 30s` | U04 | 同左 |
| **O06** | 1× deviation | E1 | summary 含数值对 | 性能/指标分析 | `分析延迟` + summary 含 baseline/observed | U04 分支 | deviation = 相对基线偏离 |
| **O07** | fact + uncertainty | **E0** | 首轮 | **闭式 + 歧义**（边界） | `2×3=几?`（模型同时答 6 又追问进制） | U06 | **禁止**混 uncertainty，但边界可出现 |
| **O07** | fact + uncertainty | **E0** | 首轮 | **多意图复合** | `1+1=几 + 查 devrix 项目结构` | U12 | 闭式部分 fact，探索部分 uncertainty |
| **O08** | 4 条截为 3 | E7 常见 | 丰富上下文 | 复合长指令 | 压力测试合成输入 | MaxProposalsTruncated | Go max=3 硬截断 |

**同剖面、不同 OBS-O（E0 反例）**：

| directive 示例 | 证据剖面 | 期望 OBS-O | 差异来源 |
|---------------|---------|-----------|---------|
| `2×3=几?` | E0 | **O02** | 闭式问答语义 + appendix |
| `review d7 plan 目录` | E0 | **O04** | 开放任务语义 + appendix |
| `1+1=几 + 查 devrix 结构` | E0 | **O07** | 多意图复合 |

输入 user frame 形状 **完全相同**（均仅 `directive:` 一行）；差异仅在模型对 directive 语义的分类。

### 12.3 E 剖面 × directive 语义 → OBS-O 速查

| E 剖面 | 非空 LLM 标签 | directive 语义 | 期望 OBS-O（倾向） | 示例 |
|--------|--------------|--------------|-------------------|------|
| E0 | 仅 directive | 闭式问答 | O02 | `2×3=几?` |
| E0 | 仅 directive | 开放任务 | O04 | `review d7 plan/` |
| E0 | 仅 directive | 多意图复合 | O07 | `1+1=几 + 查结构` |
| E1 | + artifact_summary | 重试/继续 | O05（亦可能 O04/O06） | `再试一次` |
| E1 | + artifact_summary | 含数值 delta | O06（亦可能 O05） | 性能分析类 |
| E2 | + scope_open_question | 模糊 + 未闭合 | O04 | `帮我优化一下` |
| E3 | + scope_goal | 任务已收缩 | O02 或 O04 | 视 scope 完备度 |
| E4 | + prior_observation_ids | 增量追问 | O02/O04/O05 | 跨轮继续 |
| E5 | + prior_parse_reject | 格式纠错 | 合法 JSON（任意 kind） | 带 parse reject 反馈 |
| E6 | + downlink signal | 父项编排 | O04/O05 | child downlink |
| E7 | 6 标签全现 | 任意 | O01–O08 均可 | trace 压力输入 |

### 12.4 Go 机械轨反查（不经 LLM）

以下 kind **不经过 LLM 分类**，无法从「用户指令 → LLM」反推；须从 **WorkItem 状态** 反查：

| 机械规则 | 触发状态 | 产出 kind | 典型场景 | 用户指令角色 |
|---------|---------|----------|---------|------------|
| M-default echo | 每轮 | ObsFact（directive 原文） | 任意 | 原文 echo，非 LLM 答案 |
| M-default orchestrate | `intent==Orchestrate` | ObsSignal | 编排型 | `拆解并实现 X` |
| M-default high unc | `item.Uncertainty≥0.6` | ObsUncertainty | 高不确定 WorkItem | 不阻断 fast-path |
| M-scope-openQ | `HasOpenQuestions()` | ObsUncertainty×N | scope 未闭合 | 与 LLM O04 **并行** |
| M-scope-goal | `IsCompleteEnoughForDecompose()` | ObsFact | scope 齐备 | 与 LLM 独立 |
| M-rollup-bubbles | `NeedsRollup` + 子完成 | ObsFact×N | 父项 rollup | LLM 可空/任意 |
| M-deliverable-incomplete | `DeliverableStatusIncomplete` | ObsSignal/Fact | deliverable 未完成 | LLM 可空/任意 |

**最终 UncertaintyReport** = LLM(O*) ∪ Go(M*)；路由读合并后报告，不读输入假设。

### 12.5 反查使用约束

| 约束 | 说明 |
|------|------|
| 非双射 | 同一 directive 在不同轮次（E0→E1→E4）可对应不同 OBS-O |
| 倾向非保证 | E1 倾向 O05，亦可能 O04/O06；以 trace canned + e2e 验收 |
| 机械层并行 | U03 同时有 Go scope uncertainty（M-scope-openQ）与 LLM O04 |
| 验收方式 | trace test 用 canned LLM 锁定期望；生产用 appendix + e2e 抽检 |
| 正查对照 | §7.3 OBS-U01–U12；§7.5 OBS-O01–O08 |

### 12.6 与 fast-path / Partition 的衔接

| OBS-O | G3 hasObsUncertainty | G4 pickHighStrength | 典型下游 | 反查构造要点 |
|-------|---------------------|--------------------|---------|-------------|
| O02 | ✓ | ✓ | observational_answer | E0 + 闭式 + 无 scope unc |
| O03 | ✓ | ✗ | Plan | fact str<0.85 |
| O04 | ✗ | ✗ | Plan | E0 开放 / E2 模糊 |
| O05 | ✓ | ✗ | Plan | E1 重试 signal |
| O06 | ✓ | ✗ | Plan / Anomalies* | E1 + 数值 delta |
| O07 | ✗ | ✓* | Plan | E0 复合/歧义；G3 阻断优先 |
| O08 | 视内容 | 视内容 | Plan | max=3 截断 |

\* G4 可命中 fact 但被 G3 阻断；\* Anomalies 需 Go promote CatSystem（§8.2 P2）。
