# Spec: D7 Observe ↔ LLM 完整输入输出协议（Review 版）

**Domain**: D7 (Orchestration)
**Feature**: observe-llm-io-protocol
**Status**: S3_Design（Review）
**Versions**: d7-orchestration v4.29.0 → v4.30.0
**Change ID**: `devrix-d7-observe-node-spec`
**Demand ID**: DM-20260711-001（继承 DM-20260708-003）
**Full Node SoT**: [`observe-node-spec.md`](observe-node-spec.md)（双轨合并、机械层、路由、§12 反查）
**Parent**: `devrix-d7-observational-fastpath` (DM-20260706-011)

> **Review 范围**：本文档仅覆盖 **Observe 节点 ↔ LLM 分类器** 的帧级 I/O。Go 机械轨、fast-path 闸门、Partition 详见 `observe-node-spec.md` §2–§3、§6。

---

## 0. 协议核心原则（必读）

| 原则 | 说明 |
|------|------|
| **输入不编码 ObsKind** | LLM user frame 只有 6 个证据标签；**无** `kind` / `obs_fact` 等输入字段 |
| **输出才声明 kind** | `kind` 仅出现在 LLM 返回的 JSON 数组元素中 |
| **输入类型无关** | 所有用户用例共享同一 schema；差异仅为字段是否非空（证据剖面 OBS-E0–E7） |
| **同剖面不同 kind** | E0（仅 `directive`）下，`2×3=几?` 与 `review d7 plan/` 输入帧相同，输出 kind 由模型判定 |
| **appendix 非输入** | 封闭式分类器引导写在 system prompt appendix，**不是** user frame 字段 |
| **Category 不进 LLM** | LLM 路径恒产出 `CatBusiness`；`CatSystem` 由 Go promote（P2 待实现） |

```
ObserveSignalInput (11 字段 struct)
        │  observeLLMFieldMap 过滤
        ▼
LLM user frame（6 标签，omit_empty）  ← 无 ObsKind
        │
        │  + system prompt（D2 Materialize + observation appendix）
        ▼
LLM InvokeStream
        ▼
JSON 数组 [{ kind, strength, statement, question, evidence }]  ← ObsKind 唯一声明处
        │
        │  parse → mapRawObsProposals → validateOneProposal (max=3)
        ▼
[]Observation (source=observation_proposer)
        │
        │  与 Go 机械轨合并 → UncertaintyReport
        ▼
Plan / fast-path / Learn
```

---

## 1. 端到端调用链

| 步骤 | 函数 / 组件 | 输入 | 输出 |
|------|------------|------|------|
| 1 | `buildObserveSignalInput` | WorkItem + LastRound + ScopeContract | `ObserveSignalInput`（11 字段） |
| 2 | `observeLLMFieldMap` | ObserveSignalInput | `map[TagName]any`（≤6 键） |
| 3 | `buildLLMObservationUserPrompt` | field map + locale | user 消息（guide + frame） |
| 4 | `MUPS.MaterializeForMUPS` | MUPS request | system prompt + UserContextPrepend |
| 5 | `LLM.InvokeStream` | system + messages | raw text |
| 6 | `parseObservationProposalsJSON` | raw text | `[]ObservationProposal` |
| 7 | `mapRawObsProposals` | raw rows | proposals（**Category=CatBusiness**） |
| 8 | `validateOneProposal` × N | proposal | `Observation` 或 reject |
| 9 | `ValidateObservationProposals` | proposals | max **3** 条 |
| 10 | `mergeProposedObservations` | observations | 并入 UncertaintyReport |

**代码锚点**：

- `observation_proposer.go` — struct、signal 构建、validate
- `llm_observation_proposer.go` — field map、user prompt、parse
- `format_hints_mups.go` — appendix 措辞
- `prompttags` — `FrameObserveUser`、BuildLineFrame

---

## 2. System Prompt 组成

### 2.1 结构

```
system_prompt =
  D2 MaterializeForMUPS 基础 system
  + ObservationTaskAppendix(locale)    // 封闭式分类器角色 + JSON schema + 分类规则
  + RenderSemanticAppendix(Observe)    // 术语表
```

源：`LLMObservationProposer.ProposeObservations` → `mergeMUPSPreparedSystem(prepared)`。

### 2.2 封闭式分类器 appendix（DM-20260705-009）

源：`format_hints_mups.go`。

**角色定位（ZH）**：

- 输入 = directive + 结构化 signal；输出 = Obs* 数组
- 不执行工具、不评估任务完成度、不分析任务本身
- 仅返回 JSON 数组（不要 markdown / 散文）

**分类引导规则（非输入字段）**：

| 条件 | 倾向输出 |
|------|---------|
| signal 不足 / directive 模糊 / 任务需工具 | `obs_uncertainty`（返回 question），**优于**空数组 `[]` |
| directive 是任务指令 | 不假设其完成状态；仅作信号观察 |
| 确定性问答（数学、常识、定义） | `obs_fact`，statement 含完整答案，strength=0.85 |
| 确定性问答 | **禁止**再混 `obs_uncertainty` 追问 |

### 2.3 User 消息结构

```
user_message =
  RenderFrameFieldGuideForFields(FrameObserveUser, locale, fields)   // 仅对已出现字段
  + "\n\n"
  + BuildLineFrame(FrameObserveUser, fields)                        // key: value 行
```

多值字段（`scope_open_question`、`signal`）每元素独立一行。

---

## 3. 输入协议

### 3.1 Layer A — `ObserveSignalInput`（11 frame 字段）

源：`observation_proposer.go:27-70`；注册：`prompttags.FrameObserveUser`。

| # | 字段 | Plane | **给 LLM？** | 出现条件 |
|---|------|-------|-------------|---------|
| — | `SessionID` | — | ❌ | 路由 only |
| 1 | `WorkItemID` | control | ❌ | 始终；Go evidence 兜底 inject |
| 2 | `Directive` | data | ✅ | **无条件** |
| 3 | `PriorParseReject` | control | ✅ | 非空 |
| 4 | `PriorMean` | control | ❌ | Learn 输出；防锚定 |
| 5 | `ScopeGoal` | data | ✅ | 非空 |
| 6 | `ScopeOpenQuestions` | data | ✅ | len>0 |
| 7 | `InboundSignalLines` | data | ✅ | len>0 |
| 8 | `PriorObservationIDs` | control | ✅ | len>0 |
| 9 | `IncrementalOnly` | control | ❌ | 控制流标志 |
| 10 | `PriorArtifactSummary` | data | ❌ | Plan frame delta |
| 11 | `KnownGaps` | data | ❌ | Plan frame delta（stub） |

**不进 LLM 的 5 字段理由**：

| 字段 | 理由 |
|------|------|
| `WorkItemID` | Go 强制 inject 到 evidence；LLM 抄写会污染 |
| `PriorMean` | 锚定效应 + 职责倒挂（Learn 输出非 Observe 输入） |
| `IncrementalOnly` | 控制流，非证据 |
| `PriorArtifactSummary` | 属 Plan frame delta |
| `KnownGaps` | 属 Plan/Verify 消费 |

### 3.2 Layer B — LLM user frame（6 标签，omit_empty）

源：`observeLLMFieldMap`（`llm_observation_proposer.go:69`）。

| 标签 | 来源字段 | 出现条件 |
|------|---------|---------|
| `directive` | `Directive` | 无条件 |
| `prior_parse_reject` | `PriorParseReject` | TrimSpace ≠ "" |
| `scope_goal` | `ScopeGoal` | TrimSpace ≠ "" |
| `scope_open_question` | `ScopeOpenQuestions` | len > 0 |
| `signal` | `InboundSignalLines` | len > 0 |
| `prior_observation_ids` | `PriorObservationIDs` | len > 0 |

**渲染格式**：`key: value\n`；空 slice / 空字符串 **不出现**。

### 3.3 Layer C — signal 行词汇表（v1 生产 SoT）

源：`buildObserveSignalInput`（`observation_proposer.go:106`）。

| 前缀 | 触发条件 | 示例行 |
|------|---------|--------|
| `artifact_summary:` | `LastRound.ArtifactSummary` 非空 | `artifact_summary: connection refused (3rd retry)` |
| `child_downlink_scope_in:` | 父项 child downlink ScopeIn | `child_downlink_scope_in: d7/plan/` |
| `expected_return:` | child downlink ExpectedReturn | `expected_return: <deliverable_schema>p0_p1_file_line</deliverable_schema>` |

> **未注册前缀**（如 `p99_latency_ms: 245`）当前不会自动注入。旧文档场景 3/4 中的 metric 行属虚构，勿作验收依据。

### 3.4 证据剖面 OBS-E0–E7（输入形状唯一点）

描述 **LLM user frame 哪些标签非空**；**不预判**输出 kind。

| ID | 非空标签 | User frame 行数 | Layer A 触发 |
|----|---------|----------------|-------------|
| **E0** | 仅 `directive` | 1 | 首轮；无 LastRound 摘要；无 scope |
| **E1** | `directive` + `signal`（artifact_summary） | 2+ | `LastRound.ArtifactSummary` 非空 |
| **E2** | `directive` + `scope_open_question`×N | 2+ | `ScopeContract.OpenQuestions` 非空 |
| **E3** | `directive` + `scope_goal` | 2 | `ScopeContract.GoalStatement` 非空 |
| **E4** | `directive` + `prior_observation_ids` | 2 | `LastRound.ObservationIDs` 非空 |
| **E5** | `directive` + `prior_parse_reject` | 2 | `LastRound.ObserveParseReject` 非空 |
| **E6** | `directive` + `signal`（downlink） | 3+ | child downlink |
| **E7** | 6 标签全现 | 多行 | 压力 / 全字段测试 |

#### E0 示例（同剖面、不同期望输出）

**输入 A**（闭式问答倾向 → OBS-O02）：

```
directive: 2×3=几?
```

**输入 B**（开放任务倾向 → OBS-O04）：

```
directive: review d7 plan 目录
```

两帧 **形状相同**（均 1 行）；差异仅在 appendix 引导下的模型分类。

#### E1 示例（U04）

```
directive: 再试一次
signal: artifact_summary: connection refused (3rd retry)
```

#### E2 示例（U03）

```
directive: 帮我优化一下
scope_open_question: 优化哪个模块?
scope_open_question: 优化什么指标?
```

#### E7 示例（OBS-I01 全字段）

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

---

## 4. 输出协议

### 4.1 LLM 原始 JSON Schema

源：`llm_observation_proposer.go:118-124`；appendix `DocBlockObserveSchema`。

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

| 顶层约束 | 行为 |
|---------|------|
| 必须 JSON 数组 | 散文包裹时截取首个 `[`…`]` |
| `kind` 别名 | `fact`/`obs_fact` 等，大小写不敏感 |
| 空 `[]` / 空串 | `(nil, nil)` → 无 LLM 提案 |
| max 条数 | **3**（`ValidateObservationProposals` 截断） |
| `category` | **LLM 不输出**；Go 恒设 `CatBusiness` |

### 4.2 四种 kind → 领域 Payload 映射

源：`validateOneProposal` + `orchtypes/observation.go`。

#### obs_fact → `FactPayload`

| JSON 字段 | Payload 字段 | 约束 |
|----------|-------------|------|
| `statement` | `Statement` | **必填**；空 → reject |
| `strength` | `Strength` | >0.85 → **cap 0.85**；≤0 → lift 0.5 |
| `evidence` | `Evidence` | Go append `WorkItemID` + `SessionID` |

#### obs_signal → `SignalPayload`

| JSON 字段 | Payload 字段 | 约束 |
|----------|-------------|------|
| `statement` | `Name` | 空 → default `"llm_signal"` |
| `strength` | `Value` | 直接映射 |
| — | `Threshold` | 硬编码 **0.5**（LLM 不可见） |

#### obs_deviation → `DeviationPayload`

| JSON 字段 | Payload 字段 | 约束 |
|----------|-------------|------|
| `statement` | `Metric` | **必填**；空 → reject |
| `strength` | `Observed`, `Delta` | 直接映射 |
| — | `Expected` | 硬编码 **0** |

#### obs_uncertainty → `UncertaintyPayload`

| JSON 字段 | Payload 字段 | 约束 |
|----------|-------------|------|
| `question` | `Question` | **必填**；空 → fallback `statement` → 仍空 reject |
| `strength` | — | `Confidence = 1 - strength` |
| — | `RequiresMore` | 硬编码 **true** |

### 4.3 Go 兜底规则汇总（validateOneProposal）

| 规则 | 行为 |
|------|------|
| strength ≤ 0 | lift → 0.5 |
| ObsFact strength > 0.85 | cap → 0.85 |
| ObsFact 空 statement | reject |
| ObsSignal 空 statement | default `"llm_signal"` |
| ObsDeviation 空 statement | reject |
| ObsUncertainty 空 question | fallback statement → 仍空 reject |
| evidence | append `WorkItemID` + `SessionID`（若缺失） |
| unknown kind | skip（`mapRawObsKind` 失败） |

### 4.4 LLM 输出组合空间 OBS-O

| ID | 输出形态 | 典型下游 | 绑定用例 |
|----|---------|---------|---------|
| **O01** | `[]` | Plan fallback | — |
| **O02** | 1× fact str≥0.85 | fast-path | U01 |
| **O03** | 1× fact str<0.85 | Plan | StrengthClamping |
| **O04** | 1× uncertainty | Plan | U02, U03 |
| **O05** | 1× signal | Plan | U04 |
| **O06** | 1× deviation | Plan / Anomalies* | U04 分支 |
| **O07** | fact + uncertainty | Plan（G3 阻断） | U06, U12 |
| **O08** | 4 条 → 截 3 | Plan | MaxProposalsTruncated |

\* Anomalies 需 Go promote CatSystem（§8 P2，`observe-node-spec.md`）。

**正查**：§5 用例库。**反查**：`observe-node-spec.md` §12。

---

## 5. 完整 I/O 用例库（trace 对齐）

每例格式：**证据剖面** → **User frame** → **期望 LLM 输出** → **Go 后处理要点**。

### U01 — 闭式问答（E0 → O02）

| 栏 | 内容 |
|----|------|
| 剖面 | E0 |
| User frame | `directive: 2×3=几?` |
| 期望 LLM 输出 | `[{"kind":"obs_fact","strength":0.85,"statement":"在标准算术下，2×3=6。","question":"","evidence":[]}]` |
| Go 后处理 | strength cap 0.85；evidence inject；fast-path G4 命中 |
| 测试 | `ObsFact_FastPathTrigger`, `FullPipeline_FactFastPath` |

### U02 — 开放任务（E0 → O04）

| 栏 | 内容 |
|----|------|
| 剖面 | E0（与 U01 **同形**） |
| User frame | `directive: review d7 领域 plan目录下代码` |
| 期望 LLM 输出 | `[{"kind":"obs_uncertainty","strength":0.7,"statement":"","question":"…","evidence":[]}]` |
| Go 后处理 | hasObsUncertainty → fast-path G3 miss → Plan |
| 测试 | `OpenDirectiveNoSignal_ClassifierPrompt` |

### U03 — 模糊 + scope 未闭合（E2 → O04）

| 栏 | 内容 |
|----|------|
| 剖面 | E2 |
| User frame | `directive: 帮我优化一下` + 2× `scope_open_question:` |
| 期望 LLM 输出 | `[{"kind":"obs_uncertainty",…}]` |
| 并行机械层 | Go `mapScopeContractToObservations` 亦注入 uncertainty（P3 待去重） |
| 测试 | `ObsUncertainty_PlanDecompose` |

### U04 — 重试 + 上轮摘要（E1 → O05）

| 栏 | 内容 |
|----|------|
| 剖面 | E1 |
| User frame | `directive: 再试一次` + `signal: artifact_summary: connection refused (3rd retry)` |
| 期望 LLM 输出 | `[{"kind":"obs_signal","strength":0.6,"statement":"重复 attempt",…}]` |
| Go 后处理 | SignalPayload{Name, Value=strength, Threshold=0.5} |
| 测试 | `ObsSignal_StructuredMetric` |

### U05 — 跨轮增量（E4，可叠加 E1）

| 栏 | 内容 |
|----|------|
| 剖面 | E4（常 + E1） |
| User frame | `prior_observation_ids: obs_001` + 视情况 `signal:` |
| 期望 LLM 输出 | 新 obs，evidence 引用已知 id |
| 测试 | `OnlyFieldsVisibleToLLM`；dedup 待补 |

### U06 — 混合输出（E0 → O07）

| 栏 | 内容 |
|----|------|
| 剖面 | E0（`directive: 2×3=几?`） |
| 期望 LLM 输出 | fact + uncertainty 同时存在 |
| Go 后处理 | G4 命中 fact，**G3 阻断** → Plan |
| 测试 | `FactPlusUncertainty_FastPathBlocked` |

### U07–U08 — 机械层主导（LLM 任意/空）

| 用例 | 说明 |
|------|------|
| U07 rollup | LLM 不决定路由；G1 miss |
| U08 deliverable incomplete | `observeDeliverableSignals` 主导 |

### U09 — child downlink（E6）

```
directive: …
signal: child_downlink_scope_in: d7/plan/
signal: expected_return: <deliverable_schema>…</deliverable_schema>
```

倾向 O04 或 O05。

### U10 — parse 自纠（E5）

```
directive: …
prior_parse_reject: {"error":"strength out of range",…}
```

期望：修正后的合法 JSON 数组。

### U11 — scope 齐备（E3）

```
directive: …
scope_goal: review d7 编排层
```

LLM 倾向 fact 或 uncertainty；Go 可能并行注入 scope goal fact。

### U12 — 多意图复合（E0 → O07）

```
directive: 1+1=几 + 查 devrix 项目结构
```

期望：fact（算术部分）+ uncertainty（探索部分）。

---

## 6. 协议不变量（OBS-I）

| ID | 主题 | 断言 | 测试 |
|----|------|------|------|
| I01 | 11→6 过滤 | user frame 仅 6 标签 | `OnlyFieldsVisibleToLLM` |
| I02 | prior_mean 隔离 | PriorMean 永不进 prompt | `BayesianPrior_GoSideOnly` |
| I03 | kind 别名 | fact/signal/deviation/uncertainty 大小写不敏感 | `KindAliasCaseInsensitive` |
| I04 | JSON 容错 | 散文包裹仍可 parse | `JSONParseLeniency` |
| I05 | ObsFact cap | LLM 0.95 → Go 0.85 | `StrengthClamping` |
| I06 | uncertainty fallback | question 空 → statement → reject | `UncertaintyQuestionFallback` |
| I07 | 封闭式分类器 | appendix 含角色 + 分类引导 | `OpenDirectiveNoSignal_ClassifierPrompt` |

---

## 7. 测试运行

```bash
go test -v -run 'TestObserveTraceE2E' \
  ./internal/layers/orchestration/sessionorchestrator/...
```

| 覆盖 | 状态 |
|------|------|
| 16 trace e2e | ✅ PASS（2026-07-08） |
| U08 deliverable trace | ❌ 待补 |
| E4 dedup | ⚠️ 待补 |

---

## 8. 与全节点 spec 的分工

| 主题 | 本文档 | `observe-node-spec.md` |
|------|--------|------------------------|
| LLM system + user frame | ✅ §2–§3 | §4 摘要 |
| LLM JSON 输出 + validate | ✅ §4 | §5.1–§5.2 |
| 证据剖面 E0–E7 | ✅ §3.4 | §7.2 |
| 用例 U01–U12 I/O | ✅ §5 | §7.3 |
| OBS-O 反查 | 指向 §12 | ✅ §12 |
| Go 机械轨 OBS-M | 仅提及 | ✅ §5.3、§7.4 |
| fast-path / Partition | 仅提及 | ✅ §6 |
| 实现债 P1–P5 | 指向 §8 | ✅ §8 |

---

## 9. 变更记录

| 日期 | 变更 |
|------|------|
| 2026-07-08 | 初稿：11→6 字段、4 kind payload、5 场景（DM-20260708-003） |
| 2026-07-11 | **Review 版重写**：输入类型无关、OBS-E/U/O 体系；废弃旧 §5 五场景（含虚构 signal） |
| 2026-07-11 | 与 `observe-node-spec.md` 对齐；§5 用例库 trace 绑定 |

---

## 附录 A：旧 §5 场景勘误

| 旧场景 | 问题 | 新结构 |
|--------|------|--------|
| 场景 1 确定性问答 | ✅ 基本正确 | E0 + U01 |
| 场景 2 cache 命中率 | 虚构 signal 组合 | E0/E2 + O04 |
| 场景 3 p99 latency | `p99_latency_ms:` 未注册 | E1 + O05（仅 artifact_summary） |
| 场景 4 监控目录 | 虚构 metric + DetectAnomalies 夸大 | O06 + Go promote |
| 场景 5 混合 | ✅ 正确 | E0 + U06 |

---

## 附录 B：快速 Review Checklist

- [ ] 输入帧是否 **从不** 出现 ObsKind / category？
- [ ] 6 标签 omit_empty 是否与 trace `OnlyFieldsVisibleToLLM` 一致？
- [ ] signal 词汇表是否仅 3 个注册前缀？
- [ ] E0 同形不同 kind 是否在 U01/U02 中体现？
- [ ] 每种 kind 的 payload 映射是否与 `validateOneProposal` 一致？
- [ ] OBS-O07 混合场景是否明确 G3 阻断 fast-path？
- [ ] appendix 分类引导是否与 DM-20260705-009 一致？
