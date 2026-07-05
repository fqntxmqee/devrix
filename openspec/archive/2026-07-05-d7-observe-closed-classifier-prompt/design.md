# Design: Observe 节点封闭式分类器定位强化

**Change ID:** `d7-observe-closed-classifier-prompt`
**Demand:** DM-20260705-009
**Status:** S3_Design
**Date:** 2026-07-05

---

## ① 架构目标

### 业务目标

修复 Observe 节点 LLM 调用的 3 个症状根因——system_prompt 缺"封闭式分类器"定位声明,让 LLM 在面对开放式 directive(无 signal)时,正确返回 `obs_uncertainty` 而非 markdown / 空数组 / 错 kind。

### 技术目标

| 指标 | 当前 | 目标 |
|------|------|------|
| system_prompt 包含"封闭式分类器"措辞 | ❌ | ✅ P0 |
| system_prompt 包含"signal 不足 → obs_uncertainty"引导 | ❌ | ✅ P0 |
| 现有 8 测试 0 修改 PASS | (基线) | ✅ P0 |
| 9 字段契约 / i18n guide header / 4 alias 解析 0 修改 | (基线) | ✅ P0 |
| 覆盖率 | 已有基线 | ≥ 80% P0 T 100% |

### 约束条件

- 不动 M1 9 字段契约 / i18n guide header / parse 链路
- 文件 < 800 行,函数 < 50 行
- 0 行为变化 (parse 逻辑、字段、反馈链路)
- go-struct-driven 不能回退
- 不引入任务类型感知 system_prompt 分支(下个 change)

---

## ② 架构原则

| 原则 | 说明 |
|------|------|
| **P1: 精准修复** | 只改 system_prompt 措辞,不动 9 字段 / guide / parse |
| **P2: 0 行为变化** | parse / 字段 / 反馈链路不动,只改文本 |
| **P3: i18n 同步** | en/zh 双语同步改,避免单语漂移 |
| **P4: golden snapshot 锁定** | 关键 marker 锁定为 test,防止后续漂移 |
| **P5: 不引入新概念** | 任务类型感知推迟到下个 change,本 change 不引入 |

### 命名规范

- 沿用现有 `observationTaskAppendixZHIntro` / `observationTaskAppendixENIntro` 常量名
- 沿用 `observe.node_role` i18n key 名
- 沿用 `ObserveSignalInput` / `ObservationProposal` / `ObservationKind` 类型名

### 代码风格

- 文件 < 800 行
- 函数 < 50 行
- i18n 字符串常量,const block 组织
- 测试文件命名 `<原文件名>_test.go` 或 `<主题>_test.go`

---

## ③ 业务流程

### 3.1 当前流程 (修复前)

```
User directive: "review d7 领域 plan目录下代码"
  │
  ▼
WorkItem created
  │
  ▼
Observe 节点触发 (M1 9 字段契约)
  │
  ▼
buildObserveSignalInput → ObserveSignalInput {
  WorkItemID:    "wi_65d7819c"
  Directive:     "review d7 领域 plan目录下代码"
  PriorMean:     0.625
  // 6 字段 omit_empty/omit_zero 跳过
}
  │
  ▼
buildLLMObservationUserPrompt → "[control]/[data] guide + 9 行 user frame"
  │
  ▼
ObservationTaskAppendix → "你是 Observe 助手 + 4 kinds 语义 + DocBlock schema + 2 规则"
  │
  ▼
LLM InvokeStream
  │
  ▼ (LLM 困惑)
  - 返 markdown 代码块
  - 返单个 review 文本
  - 返空数组
  - 返错误 kind
```

### 3.2 修复后流程

```
User directive: "review d7 领域 plan目录下代码"
  │
  ▼
WorkItem created
  │
  ▼
Observe 节点触发 (M1 9 字段契约,不动)
  │
  ▼
buildObserveSignalInput → ObserveSignalInput { ... 9 字段 (不动)
  │
  ▼
buildLLMObservationUserPrompt → "[control]/[data] guide + 9 行 user frame" (不动)
  │
  ▼
ObservationTaskAppendix → "封闭式分类器定位 + signal 不足 → obs_uncertainty + 4 kinds 语义 + DocBlock schema"  ← 本 change 改这里
  │
  ▼
LLM InvokeStream
  │
  ▼ (LLM 明确"我是分类器")
  → [{kind:"obs_uncertainty", strength:0.7, question:"需要先 review 哪些 plan 文件?", evidence:["wi_65d7819c"]}]
```

### 3.3 异常路径

| 异常 | 现状(已稳定) | 修复后(不变) |
|------|---------------|---------------|
| LLM 返空 | parseObservationProposalsJSON 返 `[]` | 不变 |
| LLM 返错 kind | mapRawObsKind 过滤,跳过 unknown | 不变 |
| LLM 返错 JSON | parseObservationProposalsJSON 提取 `[]` 区间,继续解析 | 不变 |
| LLM 返 markdown | prompttags.ParseWholeBody 优先,fallback json.Unmarshal | 不变 |
| 所有 proposal 失败验证 | parseRejectFromObserveError 注入 prior_parse_reject 下一轮 | 不变(DM-20260705-002) |

### 3.4 决策树

```
LLM 收到 system_prompt + user frame
  │
  ▼
signal 充足 (有 artifact_summary / child_downlink)?
  ├── 是 → 优先 obs_fact (需 evidence) / obs_signal (摘要)
  └── 否 → 优先 obs_uncertainty (返回问题) ← 修复后引导
  │
  ▼
directive 模糊?
  ├── 是 → 优先 obs_uncertainty ← 修复后引导
  └── 否 → obs_fact / obs_signal
  │
  ▼
max_proposals ≤ 3 [Go enforce]
```

---

## ④ 领域模型

### 4.1 聚合根

- **`ObservationProposal`** (现有,不动) — LLM 提案单元
- **`Observation`** (现有,不动) — 验证后入库单元
- **`ObserveSignalInput`** (现有,不动) — M1 9 字段契约

### 4.2 限界上下文 (包边界)

```
internal/layers/contextengine/i18n/  ← 本 change 修改
  ├── format_hints_mups.go          (MODIFIED)  ← system_prompt 模板
  ├── prompttags_semantics_zh.go    (MODIFIED)  ← 同步改写
  └── prompttags_semantics_en.go    (MODIFIED)  ← 同步改写

internal/layers/orchestration/sessionorchestrator/  ← 不修改,只加测试
  ├── observation_proposer.go       (不动)
  ├── llm_observation_proposer.go   (不动)
  └── observation_closed_classifier_test.go (NEW)  ← 集成测试
```

### 4.3 领域事件 (无新增)

现有 Span/Metric 全部不动。本 change 不引入新事件。

### 4.4 跨域消费模型

- D2 context-engine: i18n 模板被 D7 sessionorchestrator 调用
- D7 sessionorchestrator: 通过 `i18n.ObservationTaskAppendix(loc)` 获取 system_prompt
- D3 llm-gateway: 不动,本 change 不涉及

---

## ⑤ 核心链路图

### 5.1 端到端路径 (修复点高亮)

```
[User] → Directive "review d7 领域 plan目录下代码"
   │
   ▼
[D7 S1 WorkModel] Create WorkItem
   │
   ▼
[D7 S2 SessionOrchestrator] RunSessionTurnLoop
   │
   ▼
[D7 S6 MUPS] ItemPipelineRunner
   │
   ▼
[D7 S5 DecisionPlanning] observeWorkItem
   │
   ▼
[D7 S5 ObservationProposer] buildObserveSignalInput  ← M1 9 字段契约 (不动)
   │
   ▼
[D7 S5 LLMObservationProposer] buildLLMObservationUserPrompt  ← M1 反射 (不动)
   │
   ▼
[D7 S5 LLMObservationProposer] MergeMUPSPreparedSystem ← system_prompt 拼接
   │                                                ↑
   │  ┌─────────────────────────────────────────────┘
   │  │
   │  └── [D2 i18n] ObservationTaskAppendix(loc)  ← 本 change 修复点
   │      ├─ observationTaskAppendixZHIntro      (MODIFIED)
   │      │   "你是封闭式分类器;输入=directive+signal;输出=Obs* 数组"
   │      ├─ observationTaskAppendixZHSuffix     (MODIFIED)
   │      │   "signal 不足 → 优先 obs_uncertainty"
   │      └─ observationTaskAppendixZHIntro → RenderSemanticAppendix + DocBlock
   │
   ▼
[D3 LLMGateway] InvokeStream
   │
   ▼
[D7 S5 LLMObservationProposer] parseObservationProposalsJSON  ← 4 alias (不动)
   │
   ▼
[D7 S5 ValidateObservationProposals]  ← Go enforce max=3 + strength cap 0.85 (不动)
   │
   ▼
[D7 S5 UncertaintyReport] 注入 workitem.LastRound.ObservationIDs
```

### 5.2 时序标注

| 节点 | SLA | 现状 | 修复后 |
|------|-----|------|--------|
| `buildObserveSignalInput` | < 1ms | < 1ms | 不变 |
| `buildLLMObservationUserPrompt` | < 50μs (反射) | < 50μs | 不变 |
| `ObservationTaskAppendix` | < 1ms | < 1ms | 不变 (仅字符串拼接) |
| LLM InvokeStream | 秒级 | 秒级 | 不变 (token 数 +50 增长 < 1%) |

### 5.3 单点风险与缓解

| 风险 | 缓解 |
|------|------|
| system_prompt 措辞改错导致 LLM 完全错乱 | 现有 8 测试 (llm_observation_proposer_test.go 3 + observation_proposer_test.go 5) 0 修改 PASS 强制 |
| i18n en/zh 漂移 | 双语同步改 + 双语测试 |
| 修复后 LLM 仍不理解 | golden snapshot 覆盖关键 marker,可追踪 |

---

## ⑥ 接口/API 设计

### 6.1 风格

- 沿用 i18n 模板常量 + `RenderSemanticAppendix` 模式
- 不引入新接口

### 6.2 契约 (无变化)

| 接口 | 签名 | 契约 |
|------|------|------|
| `ObservationTaskAppendix` | `(loc Locale) string` | 返回完整 system_prompt;本 change 强化措辞,接口签名不变 |
| `RenderSemanticAppendix` | `(phase, loc) string` | 沿用,observe.node_role 措辞同步 |
| `RenderFrameFieldGuideForFields` | `(frame, loc, fields) string` | 不动 |

### 6.3 错误码三元组 (无变化)

- 沿用 `prompttags.ParseRejectCode` (RejectParseFail / RejectBudgetCap / ...)
- 沿用 `prompttags.NewObserveParseReject` API

### 6.4 幂等性

- system_prompt 生成是纯函数 (无副作用),天然幂等

### 6.5 版本演进

- D7 域 v4.26.0 → v4.26.1 (微 patch):措辞强化,无 API 变化
- D2 i18n 包:无版本号

---

## 附录 A: 关键修改 diff 预览

### A.1 `format_hints_mups.go`

```go
// BEFORE:
const observationTaskAppendixZHIntro = `你是编排 Observe 节点的观察提案助手。仅返回 JSON 数组（不要 markdown）。`
const observationTaskAppendixZHSuffix = `

规则：
- 只能使用下方提供的 directive 与结构化 signal；不要编造工具输出。
- 空数组 [] 合法。`

// AFTER:
const observationTaskAppendixZHIntro = `你是编排 Observe 节点的封闭式分类助手。
角色定位：
- 输入 = directive + 结构化 signal；输出 = Obs* 数组（每个元素: kind/strength/statement/question/evidence）
- 不执行工具、不评估任务完成度、不分析任务本身
- 不返回 markdown、不返回散文、不返回非 Obs* 格式

仅返回 JSON 数组（不要 markdown）。`
const observationTaskAppendixZHSuffix = `

规则：
- 只能使用下方提供的 directive 与结构化 signal；不要编造工具输出。
- signal 不足 / directive 模糊 / 任务需工具时 → 优先 obs_uncertainty (返回问题) 而非空数组
- directive 本身是任务指令,不要假设其完成状态;只将其作为信号观察
- 空数组 [] 合法。`
```

### A.2 `prompttags_semantics_{zh,en}.go::observe.node_role`

```go
// BEFORE zh:
"observe.node_role": "六节点管道第 1 步 Observe：从结构化 signal 分类 Obs*，不执行工具。",
// AFTER zh:
"observe.node_role": "六节点管道第 1 步 Observe：封闭式分类器；输入=directive+signal，输出=Obs* 数组，不执行工具、不评估任务本身。",
```

---

## 附录 B: 测试矩阵

| T ID | 描述 | L5 标签 | P0 |
|------|------|---------|-----|
| D7-S5-A99-T10 (NEW) | `ObservationTaskAppendix(LocaleZH)` 包含"封闭式分类器"marker | — | ✅ |
| D7-S5-A99-T11 (NEW) | `ObservationTaskAppendix(LocaleZH)` 包含"signal 不足 → obs_uncertainty"marker | — | ✅ |
| D7-S5-A99-T12 (NEW) | `ObservationTaskAppendix(LocaleEN)` 同步 AC10/AC11 英文版 | — | ✅ |
| D7-S5-A99-T13 (NEW) | 集成测试:开放式 directive + 无 signal → system_prompt 引导 obs_uncertainty | — | ✅ |
| D7-S5-A99-T01~T09 (EXISTING) | M1 9 字段契约,i18n guide header,4 alias,prior_parse_reject 0 修改 PASS | L5-MUPS-GSD-01..06 | ✅ |

---

## 附录 C: 回归风险

| 变更 | 风险 | 缓解 |
|------|------|------|
| `format_hints_mups.go` 措辞 | LLM 行为变化 | 现有 8 测试 0 修改 PASS 强制;golden snapshot 锁定关键 marker |
| `prompttags_semantics_{zh,en}.go::observe.node_role` | i18n 漂移 | 双语同步 + 双语测试 |
| 新增 2 测试文件 | (无) | — |

---

## 附录 D: S3 Checklist

- [x] 6 段式骨架完整 (①②③④⑤⑥)
- [x] 业务目标 + 技术目标量化
- [x] 决策树 + 时序图 + 单点风险
- [x] 关键修改 diff 预览
- [x] 测试矩阵 (P0 T 100%)
- [x] 回归风险表
- [x] Out of Scope 与 proposal.md 一致
- [x] DSAFT 域标注 (d2-context-engine + d7-orchestration)
