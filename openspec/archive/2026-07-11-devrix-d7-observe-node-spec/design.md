# Design: D7 Observe 节点全协议修订 + 实现债闭环

**Change ID:** `devrix-d7-observe-node-spec`
**Demand ID:** DM-20260711-001
**Status:** S3_Design
**Parent:** `proposal.md` / `demand.md`
**Spec SoT:** `specs/d7-orchestration/observe-node-spec.md`
**Created:** 2026-07-11

---

## ① 架构目标

### 1.1 业务目标

把 Observe 从「LLM 对话契约」升级为「**可验证的全节点协议**」：任何维护者读一份 spec 即可预测 `UncertaintyReport` 内容与下游路由（fast-path / Plan / Anomalies），且文档、测试、代码三处叙事一致。

### 1.2 技术目标

| 指标 | 目标 | 验证 |
|------|------|------|
| 全节点 spec 完整度 | §1–§11 + 场景矩阵 5 类 ID | AC1 review |
| fast-path 答案正确性 | prior mean 0.90 仍选 LLM ObsFact | D7-S5-A121-T02 |
| CatSystem 可达性 | ObsDeviation 无测试 hack 进 Anomalies | D7-S5-A121-T03 |
| scope 去重 | 同 open Q 不重复 2 条 ObsUncertainty | D7-S5-A121-T04 |
| 零行为回归（Wave 1） | 现有 16 trace 全 PASS | D7-S5-A121-T09 |

### 1.3 约束

- 4 ObservationKind sealed interface **不变**
- `observeLLMFieldMap` 6 字段过滤语义 **不变**（Wave 1）；Wave 3 可改为 pt-tag 派生但输出等价
- fast-path 四闸门条件 **不变**（仅 G4 选题逻辑收紧）
- 单 PR diff ≤ 400 行（按波次拆分）

---

## ② 架构原则

| 原则 | 落地 |
|------|------|
| **输入类型无关** | 文档用 OBS-E 剖面 + OBS-U 用例；禁止 OBS-S* 按 kind 命名输入 |
| **测试绑定剖面 ID** | trace test 注释标注 `// OBS-E0+U01` 等 |

---

## ③ 业务流程（全节点）

```
User directive + WorkItem context
        │
        ▼
buildObserveSignalInput() ──► ObserveSignalInput (11 fields)
        │
        ├──────────────────────────────────────┐
        │                                      │
        ▼                                      ▼
 Go 机械轨 (0 LLM)                    LLM 分类轨
 observationsFromItem                 observeLLMFieldMap → 6 fields
 mapScopeContract*                    InvokeStream → JSON array
 child bubbles                        validateOneProposal (max 3)
 observeDeliverableSignals                      │
        │                                      │
        └──────────────┬───────────────────────┘
                       ▼
            NewUncertaintyReport
                       ▼
         [Wave 2] promoteSystemCategory
                       ▼
                  Partition()
                       ▼
    ┌──────────────────┴──────────────────┐
    ▼                                      ▼
 fast-path (G1-G4)                    Plan node
```

**合并顺序不变**（`item_observe.go`）；Wave 1 只改 G4 选题；Wave 2 在 `NewUncertaintyReport` 之后、`Partition` 之前插入 promote。

---

## ④ 核心组件设计

### 4.1 `pickHighStrengthBusinessFact` 增强（P1 — Wave 1）

**文件**：`sessionorchestrator/deliverable_execute.go`

**现状**：按 `report.Observations` 顺序取首个 `CatBusiness ObsFact` strength≥threshold。

**目标**：优先 LLM 答案，排除 directive echo。

```go
// 伪代码
func pickHighStrengthBusinessFact(report, threshold, directive string) (id, stmt string, ok bool) {
    // Pass 1: source == observation_proposer && strength >= threshold
    // Pass 2: other sources, excluding item_pipeline echo (statement == TrimSpace(directive))
}
```

**边界**：

| 情况 | 行为 |
|------|------|
| 仅 item_pipeline echo fact | 不命中（fall through Plan） |
| LLM fact + echo fact | LLM 优先 |
| scope_contract fact str≥0.85 | Pass 2 可选命中（非 echo 时） |
| StaticObservationProposer tests | source 仍为 `observation_proposer` |

**调用点**：`item_pipeline.go:302` 传入 `directive`。

### 4.2 `promoteSystemCategory`（P2 — Wave 2）

**文件**：**NEW** `sessionorchestrator/observe_category_promote.go`

```go
func promoteSystemCategory(obs []orchtypes.Observation, signals []string) []orchtypes.Observation
```

**规则 v1（保守）**：

| 条件 | 动作 |
|------|------|
| `Kind==ObsDeviation` 且 `artifact_summary` 匹配 `baseline`+`observed` 数值对 | `Category=CatSystem` |
| 显式 signal 行 `system_signal: true`（预留） | 同行关联 obs → CatSystem |
| 其他 | 不变 |

**接入点**：`observeWorkItem` 在 `mergeProposedObservations` 之后、append deliverable signals 之前，对 `proposed` 切片调用；或统一对全部 obs 调用（仅改 LLM+deviation）。

**测试**：重写 `ObsDeviation_AnomalyTrigger` — 删除手动 `Category=CatSystem` hack。

### 4.3 scope 去重（P3 — Wave 2）

**方案 A（推荐）**：

1. `observeLLMFieldMap` / `buildObserveSignalInput`：**不再**把 `ScopeOpenQuestions` 放入 LLM frame
2. Go `mapScopeContractToObservations` **保留**（确定性、strength=0.9）
3. `ScopeGoal` **保留**在 LLM frame（收缩目标仍有分类价值）

**i18n**：无需改 appendix（仍允许 LLM 对模糊 directive 产 uncertainty）。

**测试**：`ObsUncertainty_PlanDecompose` 改为断言 Go 注入的 scope uncertainty 阻断 fast-path。

### 4.4 signal 注册表（P4 — Wave 3）

**文件**：`sessionorchestrator/observe_signal_registry.go`

```go
const (
    SignalPrefixArtifactSummary    = "artifact_summary"
    SignalPrefixChildDownlinkScope = "child_downlink_scope_in"
    SignalPrefixExpectedReturn     = "expected_return"
)

func AppendRegisteredSignal(lines []string, prefix, value string) []string
func IsRegisteredSignalLine(line string) bool
```

`buildObserveSignalInput` 改调 registry；i18n appendix 列出合法前缀表。

### 4.5 字段 SoT 统一（P5 — Wave 3）

**目标**：删除手写 `observeLLMFieldMap`，由 `prompttags` 根据 `FrameObserveUser` + `LLMVisibleTags` 子集生成。

**不变性**：`TestObserveTraceE2E_OnlyFieldsVisibleToLLM` 输出 bit-exact 或语义等价。

---

## ⑤ 数据模型

无新 DB 表。类型层面：

| 类型 | 变更 |
|------|------|
| `ObserveSignalInput` | 无字段增删 |
| `Observation` | 无 kind 增删；Category 赋值路径增加 promote |
| `UncertaintyReport` | 无结构变更 |

---

## ⑥ 证据剖面 → 实现映射

| 剖面/用例 ID | Wave | 实现要点 |
|-------------|------|---------|
| E0 + U01 | 1 | P1 确保选 LLM fact |
| E0 + U02 | — | 文档 + prompt test 已有（与 U01 **同剖面**） |
| E2 + U03 | 2 | P3 scope 去重 |
| E1 + U04 | — | trace 已绑定 |
| E4 + U05 | 3 | dedup test 补 U05 |
| E0 + U06 | — | 已有 |
| E0 + U07 | — | item_observe_test 已有 |
| E0/E1 + U08 | 1 | 新增 deliverable trace |
| OBS-P02 | 2 | P2 promote |
| OBS-I01–I07 | 1–3 | 按表维护 |

---

## ⑦ 测试策略

### 7.1 新增测试

| T ID | 测试 | 文件 |
|------|------|------|
| D7-S5-A121-T02 | `TestPickHighStrength_PrefersLLMOverDirectiveEcho` | `deliverable_execute_test.go` |
| D7-S5-A121-T02b | `TestPickHighStrength_Prior09_EmitsLLMAnswer` | `item_pipeline_fastpath_test.go` |
| D7-S5-A121-T03 | `TestPromoteSystemCategory_DeviationFromArtifactSummary` | `observe_category_promote_test.go` |
| D7-S5-A121-T04 | `TestObserveLLMFrame_OmitsScopeOpenQuestions` | `llm_observation_proposer_test.go` |
| D7-S5-A121-T08 | `TestObserveTraceE2E_DeliverableIncomplete` | `observe_trace_e2e_test.go` |

### 7.2 现有测试调整

| 测试 | 调整 |
|------|------|
| `ObsDeviation_AnomalyTrigger` | 移除 Category hack，依赖 promote |
| `ObsUncertainty_PlanDecompose` | 输入去掉 ScopeOpenQuestions；断言 Go scope obs |
| trace 注释 | 标注 `// OBS-E0+U01` 等 |

---

## ⑧ 部署与回滚

| 波次 | 回滚策略 |
|------|---------|
| Wave 1 | revert `pickHighStrengthBusinessFact`；fast-path 恢复旧行为（有 latent bug） |
| Wave 2 | revert promote + scope map 变更；Anomalies 回测 hack |
| Wave 3 | revert registry；buildObserveSignalInput 内联 |

无 migration、无配置开关（v1）。若 CatSystem 误 promote 率高，Wave 2 可加 `OBSERVE_PROMOTE_SYSTEM=0` env guard。

---

## ⑨ PR 拆分建议

| PR | 分支 | 内容 | 预估行数 |
|----|------|------|---------|
| PR-1 | `docs/d7-observe-node-spec` | spec + delta + 旧 spec 标注 + t-registry 登记 | ~600 doc |
| PR-2 | `fix/d7-observe-fastpath-pick` | P1 + T02/T02b + U08 | ~120 |
| PR-3 | `feat/d7-observe-category-scope` | P2 + P3 + T03/T04 | ~200 |
| PR-4 | `refactor/d7-observe-signal-sot` | P4 + P5 | ~180 |

PR-1 可 docs-only 先合；PR-2 为 P0 行为修复。

---

## ⑩ 域文档同步清单（S5 门禁）

| 文件 | 动作 |
|------|------|
| `openspec/specs/d7-orchestration/observe-node-spec.md` | 状态 → IMPLEMENTED |
| `openspec/specs/d7-orchestration/d7-observe-llm-io-protocol-spec.md` | §5 superseded -banner |
| `openspec/specs/d7-orchestration/spec.md` | lite 条目 + v4.30.0 |
| `openspec/specs/d7-orchestration/t-registry.md` | D7-S5-A121 段 |
| `openspec/specs/d7-orchestration/CHANGELOG.md` | 顶部条目 |
| `openspec/demand-archive-index.md` | S7 归档时追加 |
