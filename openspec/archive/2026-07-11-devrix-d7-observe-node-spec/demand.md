---
demand-id: DM-20260711-001
title: D7 Observe 节点全协议修订与实现债闭环
priority: P1
status: S4_InProgress
dsaft_domain: orchestration
created: 2026-07-11
parent_change: devrix-d7-observe-llm-protocol-doc
parent_demand: DM-20260708-003
origin: |
  DM-20260708-003 沉淀了 Observe↔LLM 子协议，但审查发现：(1) 文档将 LLM 子路径当作全节点协议；
  (2) §5 场景与 trace test / 生产代码不对齐；(3) fast-path 选题、CatSystem 提升、signal 词汇表等
  实现债未闭合。需升级为全节点 spec + 分波次代码修复。
---

# D7 Observe 节点全协议修订与实现债闭环

## 1. 原始描述

将 Observe 节点从「LLM I/O 文档」升级为「双轨全节点协议」：明确 Go 机械层 + LLM 封闭式分类器的合并语义、三层输入协议、完整场景矩阵，并闭合 P1–P5 实现债（fast-path 选题、CatSystem、scope 去重、signal 注册表、字段 SoT 统一）。

## 2. 澄清记录

| # | 问题 | 结论 |
|---|------|------|
| Q1 | 全节点 spec 是否替代旧 `d7-observe-llm-io-protocol-spec.md`？ | **部分替代**：旧文档 §3–§4、§7 保留；§5 由 `observe-node-spec.md` §7 OBS-E/U 矩阵替代 |
| Q2 | 是否本 change 一次性修完 P1–P5？ | **分波次**：P0=文档+fast-path(P1)；P1=CatSystem+scope；P2=signal 注册表+SoT 统一 |
| Q3 | CatSystem 由谁赋值？ | **Go 端 promote**，LLM 不 emit category；与现有 `mapRawObsProposals` 硬编码 CatBusiness 一致 |
| Q4 | scope_open_question 双重注入如何处理？ | **方案 A（推荐）**：保留 Go `mapScopeContract`，LLM frame 省略 `scope_open_question` |
| Q5 | 验收锚点？ | 证据剖面 OBS-E0–E7 + 用例 OBS-U01–U12 + OBS-I01–I07 全绿；P1 fast-path 新增 prior≥0.85 回归测试 |
| Q6 | 输入协议是否按 ObsKind 分场景？ | **否** — 输入仅证据剖面 OBS-E*；kind 在 LLM 输出 `kind` 或 Go 机械规则声明 |

## 3. 澄清范围

### L1 领域

- **orchestration**（D7 编排层）

### L2 场景

| L2 | 名称 | 说明 |
|----|------|------|
| D7-S5 | Observe Node | MUPS Phase 1：观测聚合 + 封闭式分类 |

### L3 活动（后端）

| L3 ID | 活动 | 映射 |
|-------|------|------|
| D7-S5-A122 | Observe 全节点协议修订 | 本文 change |

### L4 功能点（草案）

| L4 ID | 功能点 | 文件锚点 |
|-------|--------|---------|
| `observe_node_merge` | 双轨观测合并 | `item_observe.go` |
| `observe_llm_classifier` | LLM 6 字段帧 + 4 kind 解析 | `llm_observation_proposer.go` |
| `observe_fastpath_pick` | fast-path ObsFact 选题 | `deliverable_execute.go` |
| `observe_category_promote` | CatSystem 提升 | **NEW** `observe_category_promote.go` |
| `observe_signal_registry` | signal 行词汇表 | `observation_proposer.go` |

### L5 / T 测试点草案

| T ID | Given-When-Then（摘要） | 优先级 |
|------|------------------------|--------|
| D7-S5-A121-T01 | Given 全字段 ObserveSignalInput，When buildLLMObservationUserPrompt，Then 仅 6 标签且 omit_empty | P0 |
| D7-S5-A121-T02 | Given prior mean≥0.85 + LLM ObsFact 答案，When fast-path，Then 选中 LLM fact 非 directive echo | P0 |
| D7-S5-A121-T03 | Given latency delta artifact_summary，When promoteCategory，Then ObsDeviation CatSystem→Anomalies | P1 |
| D7-S5-A121-T04 | Given ScopeContract open questions，When observeWorkItem，Then 仅 Go 注入 uncertainty（无 LLM 重复） | P1 |
| D7-S5-A121-T05 | Given OBS-E/U 矩阵，When trace e2e，Then 输入剖面与路由与 spec 一致 | P0 |

## 4. In Scope / Out of Scope

### In Scope

- `observe-node-spec.md` 全节点 SoT（双轨 + 证据剖面 OBS-E* + 用例 OBS-U*）
- 旧 spec §5 废弃说明 + 交叉引用
- P1：`pickHighStrengthBusinessFact` source 过滤
- P2：`promoteSystemCategory`（ObsDeviation 路径）
- P3：scope 去重（LLM frame 省略 open questions）
- trace test 对齐 OBS-E/U + U08 补测
- t-registry 登记 D7-S5-A121

### Out of Scope

- 新增 ObservationKind（sealed 4 类不变）
- i18n 封闭式分类器措辞大改（DM-20260705-009 已落地）
- Plan / Execute 节点协议（见 sibling specs）
- `known_gaps` Phase 3 实算（仍为 stub）

## 5. 验收标准（S5 预览）

| AC | 标准 | 优先级 |
|----|------|--------|
| AC1 | `observe-node-spec.md` 覆盖 §1–§12，证据剖面 OBS-E* + 用例 OBS-U* + OBS-O 反查 + OBS-O/G/P/I 完整 | P0 |
| AC2 | 旧 spec §5 标注 superseded，§7.7 偏离表可追溯 | P0 |
| AC3 | D7-S5-A121-T02 fast-path source 过滤 PASS | P0 |
| AC4 | 16+2 trace test PASS（含 prior≥0.85 回归） | P0 |
| AC5 | P2 CatSystem promote 有独立 trace（OBS-P02 无 hack） | P1 |
| AC6 | 域文档同步：`spec.md` + `t-registry.md` + CHANGELOG | P0 |

## 6. 依赖

| 类型 | 内容 |
|------|------|
| 前置 | DM-20260708-003（LLM 子协议）、DM-20260706-011（fast-path）、DM-20260705-009（封闭式分类器） |
| 并行 | 无阻塞 |
| 下游 | Plan 节点读 `UncertaintyReport` 摘要格式不变 |
