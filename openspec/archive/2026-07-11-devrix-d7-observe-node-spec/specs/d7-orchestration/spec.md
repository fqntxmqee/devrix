# Delta Spec: D7 Observe Node — Full Protocol

**Change ID:** `devrix-d7-observe-node-spec`
**Demand ID:** DM-20260711-001
**Capability:** `observe-node-protocol`
**Status:** S3_Design
**SoT:** `observe-node-spec.md`（同目录完整版）

---

## Overview

Observe 节点采用 **双轨架构**：Go 机械观测（deterministic）与 LLM 封闭式分类器（4 kind JSON）合并为 `UncertaintyReport`，经 Partition 与 fast-path 四闸门路由至 Plan 或直接作答。

本 delta 登记 ADDED/MODIFIED 需求；完整证据剖面与用例矩阵见 `observe-node-spec.md` §7。

---

## ADDED Requirements

### Requirement: 输入协议类型无关（证据剖面）

LLM user frame MUST NOT 编码或暗示 ObsKind。输入仅由证据剖面 OBS-E0–E7 描述（哪些字段非空）；ObsKind 仅在 LLM 输出 `kind` 或 Go 机械规则中声明。

**Priority:** P0
**L3 映射:** D7-S5 Observe Node

#### Scenario: 同剖面不同 kind（E0）

Given 两个用例均为 OBS-E0（仅 `directive` 一行）
And directive A 为 `2×3=几?`，directive B 为 `review d7 plan 目录`
When buildLLMObservationUserPrompt 渲染
Then A 与 B 的 user frame 形状相同（仅 directive 值不同）
And LLM 输出 kind 由分类器判定，输入协议不做假设

---

### Requirement: Observe 双轨合并顺序

系统 MUST 按固定顺序合并观测：item_pipeline → scope_contract → child bubbles → rollup bubbles → LLM proposals → deliverable signals。

**Priority:** P0
**Rationale:** 合并顺序影响 fast-path 选题与调试 trace。
**L3 映射:** D7-S5 Observe Node

#### Scenario: E0+U01 确定性问答合并

Given WorkItem directive 为确定性 Q&A（剖面 E0）
When observeWorkItem 完成
Then 报告含 item_pipeline ObsFact（directive echo）及 LLM ObsFact（完整答案）
And LLM 提案 source 为 `observation_proposer`

---

### Requirement: 三层输入协议

系统 MUST 区分：(A) ObserveSignalInput 11 字段 struct；(B) LLM user frame 动态 6 标签 omit_empty 子集；(C) signal 行注册表前缀。

**Priority:** P0
**L3 映射:** D7-S5

#### Scenario: OBS-I01 11→6 字段过滤

Given ObserveSignalInput 全字段非空
When buildLLMObservationUserPrompt 渲染
Then 输出含 directive、prior_parse_reject、scope_goal、scope_open_question、signal、prior_observation_ids
And 不含 work_item_id、prior_mean、incremental_only、prior_artifact_summary、known_gaps

#### Scenario: E0 首轮仅 directive

Given 首轮 WorkItem 无 LastRound 无 ScopeContract
When 渲染 LLM user frame
Then 仅一行 `directive: {verbatim}`

---

### Requirement: signal 行词汇表（v1）

系统 MUST 仅通过注册表前缀生成 signal 行：`artifact_summary`、`child_downlink_scope_in`、`expected_return`。

**Priority:** P1（Wave 3 代码；文档 P0）
**L3 映射:** D7-S5

#### Scenario: E1+U04 重试 artifact_summary

Given LastRound.ArtifactSummary 非空
When buildObserveSignalInput
Then InboundSignalLines 含一行 `artifact_summary: {truncated}`

---

### Requirement: fast-path ObsFact 选题 source 感知

系统 MUST 在 pickHighStrengthBusinessFact 中优先选择 source=`observation_proposer` 的 ObsFact，并排除 item_pipeline directive echo（statement 等于 directive 原文）。

**Priority:** P0
**Rationale:** 闭合 prior≥0.85 时 emit 错误答案的 latent bug。
**L3 映射:** D7-S5

#### Scenario: E0+U01 fast-path 选中 LLM 答案

Given prior mean=0.90 且 LLM 返回 ObsFact statement 为完整答案 strength=0.85
When fast-path 四闸门评估
Then ArtifactSummary 为 LLM statement 而非 directive 原文

#### Scenario: OBS-G04 无合格 fact 降级

Given 仅 item_pipeline directive echo ObsFact strength≥0.85 且无 LLM fact
When pickHighStrengthBusinessFact
Then 返回 miss，走 Plan 路径

---

### Requirement: CatSystem 提升（promoteSystemCategory）

系统 MUST 在 Go 端将满足规则的 ObsDeviation 提升为 CatSystem，使 Partition 路由至 Anomalies；LLM MUST NOT 直接 emit CatSystem。

**Priority:** P1
**L3 映射:** D7-S5

#### Scenario: OBS-P02 deviation 进 Anomalies

Given LLM 返回 ObsDeviation 且 artifact_summary 含 baseline/observed 数值对
When promoteSystemCategory 运行后 Partition
Then Anomalies 含 1 条 CatSystem ObsDeviation
And 测试无需手动设置 Category

---

### Requirement: scope 不确定性单轨注入

系统 MUST 仅由 Go mapScopeContractToObservations 注入 scope open question 对应的 ObsUncertainty；LLM user frame MUST NOT 包含 scope_open_question（P3 方案 A）。

**Priority:** P1
**L3 映射:** D7-S5

#### Scenario: E2+U03 scope 阻断 fast-path

Given ScopeContract 含 OpenQuestions
When observeWorkItem 完成且 LLM 返回 ObsFact
Then hasObsUncertainty 为 true（scope_contract source）
And fast-path Gate 3 miss

---

### Requirement: 证据剖面可追溯

文档 MUST 为每个 P0 用例标注 OBS-E/U/O/G/P/I ID，并映射至 trace test 函数名；旧 spec §5 场景 2–4 标记为 superseded。

**Priority:** P0

#### Scenario: 文档验收

Given observe-node-spec.md §7.7 偏离表
When S5 review
Then E0+U01/U06 与 test 对齐；旧场景 2–4 标为 ❌ 偏离

---

## MODIFIED Requirements

### Requirement: Observe↔LLM 子协议范围声明

`d7-observe-llm-io-protocol-spec.md` 范围 MODIFIED 为「LLM 分类器子路径」；全节点行为以 `observe-node-spec.md` 为准。

**Priority:** P0

---

## 关联测试注册（预登记）

| T ID | Covers |
|------|--------|
| D7-S5-A121-T01 | 双轨 + 三层输入 + OBS-E/U |
| D7-S5-A121-T09–T13 | fast-path P1 |
| D7-S5-A121-T14–T16 | CatSystem + scope |
| D7-S5-A121-T17–T18 | signal + SoT |

完整表见 `tasks.md`。
