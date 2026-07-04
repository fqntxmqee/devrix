# Design: MUPS 标签语义层

**Change ID:** `mups-prompt-tag-semantics`  
**Demand ID:** DM-20260705-001  
**Status:** S3_Design  
**Demand:** [`demand.md`](demand.md)  
**Proposal:** [`proposal.md`](proposal.md)

---

## 1. MUPS 六节点与 LLM 边界

```text
Observe ──► Plan ──► Execute ──► Verify ──► Learn ──► Decide
  LLM         LLM       LLM         Go       Go       Go
```

| 节点 | LLM | I/O Profile | 语义本需求 |
|------|-----|-------------|-----------|
| Observe | 可选 proposer | lineframe in / wholebody out | ✅ P0 |
| Plan | 可选 strategic proposer | lineframe in / wholebody out | ✅ P0 |
| Execute | ReAct loop | wiBody + envelope in/out + tools | ✅ P0 |
| Verify | — | Go parse artifact | 引用映射 |
| Learn | — | — | Out of scope |
| Decide | — | SpawnPolicy | 边界一句 |

## 2. 数据面 vs 控制面（prompt 可见）

| Plane | 含义 | Observe 示例 | Plan 示例 | Execute 示例 |
|-------|------|--------------|-----------|--------------|
| **data** | 任务载荷、业务事实 | `directive`, `signal`, `scope_goal` | `directive`, `observation_summary` | `Directive`, tool results, findings |
| **control** | 格式、预算、收敛约束 | `prior_mean`, `incremental_only` | `remaining_children`, `uncertainty_mean` | `deliverable_contract`, `Acceptance:` lines |

Prompt 组装时在 appendix 开头加 **一行总述**（i18n）：

> 下方 user 帧中 `[control]` 字段为编排预算/约束，勿当作待分析业务内容；`[data]` 为任务与信号。

## 3. TagSemanticsRegistry

**Path:** `internal/shared/prompttags/semantics.go`

```go
type PromptPlane string
const (
    PlaneData    PromptPlane = "data"
    PlaneControl PromptPlane = "control"
)

type FieldSemantic struct {
    Name      string      // tag name or lineframe key
    Plane     PromptPlane
    WhenUse   string      // one line, locale key suffix
    WhenNot   string      // optional
    Enforced  bool        // true if Go gate exists
}

type PhaseSemantics struct {
    Phase       contracts.MUPSPhase
    NodeRole    string      // one line: what this node does in 6-node pipe
    OutputRules []FieldSemantic
    InputRules  []FieldSemantic
}
```

**API:**

```go
func SemanticsForPhase(phase contracts.MUPSPhase) PhaseSemantics
func RenderSemanticAppendix(phase contracts.MUPSPhase, loc i18n.Locale) string
func RenderFrameFieldGuide(frame FrameName, loc i18n.Locale) string
```

- `RenderSemanticAppendix` 输出 **纯 bullet**（无 schema 重复）；schema 仍由 `DocBlock*` 提供。
- i18n keys 模式：`prompttags.semantics.observe.kind.obs_uncertainty.when_use`（集中 `i18n/prompttags_semantics_{zh,en}.go`）。

## 4. 三节点语义内容（P0 草案）

### 4.1 Observe 输出 — `obs_*` kind

| kind | When use | When not | Enforced |
|------|----------|----------|----------|
| `obs_uncertainty` | scope 不清、open question 未闭合 | 已有 strong scope fact | partial |
| `obs_fact` | directive/signal 直接陈述、有 evidence | 推测、无 signal 支撑 | strength cap 0.85 |
| `obs_signal` | 结构化 signal 的摘要（非 fact 级） | 可表达为 uncertainty 时 | — |
| `obs_deviation` | 期望 vs 观测偏差（metric delta） | 无 baseline 时 | — |

字段：

- `strength`：0–1，与 kind 一致；uncertainty 高 → strength 高
- `question`：`obs_uncertainty` 必填；其他可空
- `evidence`：仅已有 ID（wi_id、signal 来源），勿编造路径

### 4.2 Plan 输出

**execution_mode 决策树（appendix bullet）：**

```text
IF uncertainty_mean ≥ 0.45 OR remaining_children > 1 needed → decompose (not single)
ELIF scope clear AND fits one pass → single
ELIF parallel probes needed → parallel_probe
```

**deliverable_contract 示例（一行）：**

```json
{"citation":"file_line","severity":"p0_p1","reject":["planning_meta"],"structure":"findings_json"}
```

与 `ContractDimensionPromptDoc` 维度枚举交叉引用，不重复长表。

### 4.3 Execute 输出

| Tag / block | Plane | Required | Verify 映射 |
|-------------|-------|----------|-------------|
| `<deliverable_contract>` | control | when contract applicable | VerifyDeliverableContract |
| findings JSON body | data | when structure=findings_json | findings_json_* reasons |
| `<open_questions>` | data | optional | residual uncertainty |
| `<scope_contract>` | control | optional update | scope monotonic |
| `<conclusion>` | human prose | optional | presentation only |

**分段契约（appendix）：**

1. ReAct 阶段：tool calls + 短文本
2. 终局回复：**先** machine block（contract/findings），**后** `<conclusion>` 人类摘要

## 5. Prompt 组装变更

### 5.1 System prompt 顺序（不变）

```text
outputHints → wiBody → phaseAppendix → staticBase
```

### 5.2 phaseAppendix 新结构

```text
[Node role — 1 line]
[Semantic appendix — RenderSemanticAppendix]
[Schema line — DocBlock*]
[Legacy rules — i18n suffix, trimmed if redundant]
```

Observe 示例拼接：

```text
你是 Observe 节点观察提案助手（六节点管道第 1 步；分类 Obs*，不执行工具）。
语义：
- obs_uncertainty: 范围/目标不清时使用；...
- obs_fact: 仅当 signal 强支撑；...
...
{"kind":"obs_fact|..."}   ← DocBlockObserveSchema
规则：...（保留 max 3、no invent tools）
```

### 5.3 User frame 可选 header

在 `BuildLineFrame` 前注入 `RenderFrameFieldGuide` 的 **compact 注释块**（P1），或仅在 first LLM call 注入一次（token 权衡 — 实现时默认 appendix 含 input 语义，frame 仅 `[control]`/`[data]` 前缀）。

## 6. 与 enforce 对齐表

| Prompt 声明 | Go enforce | 节点 |
|-------------|------------|------|
| max 3 proposals | `ValidateObservationProposals` | Observe |
| uncertainty_mean ≥ 0.45 → no single | `applySingleModeUncertaintyGate` | Plan |
| child_specs max 2 | `applyBudgetCap` + `CapChildSpecs` | Plan |
| reject planning_meta | `DetectPlanningMeta` | Execute Verify |
| findings_json required | `VerifyDeliverableContract` | Execute Verify |

Advisory-only（prompt 建议、Go 不 hard fail）：`obs_signal` vs `obs_uncertainty` 选用 — 仅影响 Obs 质量，不 block pipeline。

## 7. 测试策略

| 测试 | 覆盖 L5 |
|------|---------|
| `semantics_test.go` — 每 phase 语义 entry 非空、Enforced 与 registry 一致 | — |
| `format_hints_mups_test.go` — Observe appendix 含 obs_uncertainty when-use | L5-MUPS-TAG-01 |
| `prompt_dynamic_test.go` — Plan appendix 含 execution_mode 决策 | L5-MUPS-TAG-02 |
| `workitem_execute_test.go` — Execute hints 含 Required/Optional 表 | L5-MUPS-TAG-03 |
| Golden hash zh/en system prompt（materialize） | L5-MUPS-TAG-04 |

## 8. Token 预算

- 语义 appendix 目标：Observe ≤350 tokens，Plan ≤450，Execute ≤500（zh）
- 实现：`RenderSemanticAppendix` 返回固定条目数；i18n 审查删冗余
- Materialize `TokenEst` 现有 gate 不变

## 9. 文件布局

```text
internal/shared/prompttags/
  semantics.go          # NEW — registry + render hooks
  semantics_test.go
internal/layers/contextengine/i18n/
  prompttags_semantics_zh.go   # NEW
  prompttags_semantics_en.go   # NEW
  format_hints_mups.go         # MOD — compose semantic + schema
  prompt_dynamic.go            # MOD
  workitem_execute.go          # MOD
internal/layers/contextengine/materialize/
  phase_prompts.go             # MOD — optional helper only
```

D7 `sessionorchestrator/*_proposer*.go`：**无战术字符串新增**；测试断言 prepared system 含语义 marker。
