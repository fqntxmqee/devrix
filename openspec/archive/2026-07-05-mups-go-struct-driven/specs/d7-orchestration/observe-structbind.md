# Delta: d7-orchestration — Observe node go-struct binding

**Change ID:** `mups-go-struct-driven`
**Demand:** DM-20260705-003
**Affects:** `internal/layers/orchestration/sessionorchestrator/observation_proposer.go` (MOD), `internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go` (MOD)

## MODIFIED Requirements

### Requirement: `ObserveSignalInput` 9 字段 struct + `pt:"..."` tag

`ObserveSignalInput` 是 D7-S5 Observe 节点向 LLM 输入 user frame 的值对象，9 字段 + 1 internal（`SessionID`），每个字段带 `pt:"<tag>,<plane>,<flags>"` struct tag。

字段列表（顺序由 Go struct 字段顺序决定，反射注册时写入 `FrameObserveUser.Fields`）：

| # | Field | pt tag | plane | flags |
|---|-------|--------|-------|-------|
| 1 | `WorkItemID` | `work_item_id` | control | — |
| 2 | `Directive` | `directive` | data | — |
| 3 | `PriorParseReject` | `prior_parse_reject` | control | `omit_empty` |
| 4 | `PriorMean` | `prior_mean` | control | `omit_zero` |
| 5 | `ScopeGoal` | `scope_goal` | data | `omit_empty` |
| 6 | `ScopeOpenQuestions` | `scope_open_question` | data | `omit_empty` |
| 7 | `InboundSignalLines` | `signal` | data | `omit_empty` |
| 8 | `PriorObservationIDs` | `prior_observation_ids` | control | `omit_empty` `join=,` |
| 9 | `IncrementalOnly` | `incremental_only` | control | `omit_zero` |
| — | `SessionID` | `-` (skip) | — | 非 user frame 字段 |

#### Scenario: 字段一致性
- **GIVEN** `ObserveSignalInput` struct 定义
- **WHEN** `init()` 调 `MustRegisterFrame[ObserveSignalInput](FrameObserveUser)`
- **THEN** 反射结果 == `LineFrameRegistry[FrameObserveUser].Fields`；字段数 == 9；i18n 翻译条目 == 9

#### Scenario: 字段顺序反射写入 FrameSpec
- **GIVEN** struct 字段定义顺序为 WorkItemID, Directive, PriorParseReject, ...
- **WHEN** `MustRegisterFrame` 反射
- **THEN** `FrameObserveUser.Fields` 顺序为 `[work_item_id, directive, prior_parse_reject, ...]`

### Requirement: `buildObserveSignalInput` 扁平化 ScopeContract

`buildObserveSignalInput(sessionID, item, tm) ObserveSignalInput` 仍由 D7 Go 编排调用，但内部增加 3 项扁平化逻辑：

1. `ScopeContract.GoalStatement` → `ObserveSignalInput.ScopeGoal`（若非空）
2. `ScopeContract.OpenQuestions` → `ObserveSignalInput.ScopeOpenQuestions`（过滤空字符串）
3. `len(LastRound.ObservationIDs) > 0` → `ObserveSignalInput.IncrementalOnly = true` 且 `PriorObservationIDs` 已复制

#### Scenario: ScopeContract 扁平化
- **GIVEN** `item.ScopeContract = &ScopeContract{GoalStatement:"fix login bug", OpenQuestions:["q1", "", "q3"]}`
- **WHEN** `buildObserveSignalInput("s1", item, tm)`
- **THEN** `in.ScopeGoal = "fix login bug"`；`in.ScopeOpenQuestions = ["q1", "q3"]`（空字符串被过滤）

#### Scenario: PriorObservationIDs 触发 IncrementalOnly
- **GIVEN** `item.LastRound.ObservationIDs = ["obs-1", "obs-2"]`
- **WHEN** `buildObserveSignalInput("s1", item, tm)`
- **THEN** `in.PriorObservationIDs = ["obs-1", "obs-2"]`；`in.IncrementalOnly = true`

#### Scenario: 无 ScopeContract
- **GIVEN** `item.ScopeContract = nil`
- **WHEN** `buildObserveSignalInput("s1", item, tm)`
- **THEN** `in.ScopeGoal = ""`；`in.ScopeOpenQuestions = nil`；omit_empty 跳过该 2 字段

### Requirement: `buildLLMObservationUserPrompt` 35 行 → 2 行

`buildLLMObservationUserPrompt(in ObserveSignalInput, loc i18n.Locale) string` 函数体从 35 行手工 `fields := map[prompttags.TagName]any{...}` 拼接，改造为 2 行反射调用：

```go
func buildLLMObservationUserPrompt(in ObserveSignalInput, loc i18n.Locale) string {
    frame := prompttags.BuildLineFrameFromStruct(prompttags.FrameObserveUser, &in)
    guide := i18n.RenderFrameFieldGuideForFields(prompttags.FrameObserveUser, loc, nil)
    if guide == "" {
        return frame
    }
    return guide + "\n\n" + frame
}
```

**行为不变承诺**：与改造前字节级等价（modulo 字段顺序；FrameSpec 顺序由 struct 字段顺序反射写入，与改造前手写顺序一致）。

#### Scenario: 全字段 user prompt
- **GIVEN** 完整 9 字段 ObserveSignalInput
- **WHEN** `buildLLMObservationUserPrompt(in, i18n.DefaultLocale)`
- **THEN** 输出含 i18n guide header + 9 行 `[plane] tag: value`（与 golden snapshot 文件 `testdata/observe_user_prompt.golden` 字节一致）

#### Scenario: omit_empty 字段跳过
- **GIVEN** `ObserveSignalInput` 5 个 omit_empty 字段全空
- **WHEN** `buildLLMObservationUserPrompt(in, loc)`
- **THEN** 输出仅含 `work_item_id` / `directive` 2 行 user frame

#### Scenario: omit_zero 字段跳过
- **GIVEN** `ObserveSignalInput{PriorMean: 0, IncrementalOnly: false}`
- **WHEN** `buildLLMObservationUserPrompt(in, loc)`
- **THEN** 输出不含 `prior_mean:` / `incremental_only:` 2 行

## MODIFIED

| 文件 | 变更 |
|------|------|
| `observation_proposer.go` | `ObserveSignalInput` 加 9 个 `pt:"..."` struct tag + `init()` 注册 + `buildObserveSignalInput` 扁平化 ScopeContract + 计算 IncrementalOnly |
| `llm_observation_proposer.go` | `buildLLMObservationUserPrompt` 函数体重写（35 行 → 2 行） |

## ADDED

| 文件 | 用途 |
|------|------|
| `internal/layers/orchestration/sessionorchestrator/testdata/observe_user_prompt.golden` | golden snapshot 基准（4 组合：空 / 仅 directive / 完整 / 包含 prior_parse_reject） |

## Test Points

| T ID | 描述 | L5 |
|------|------|-----|
| D7-S5-A99-T01 | `ObserveSignalInput` 9 字段 + pt tag 反射注册成功 | L5-MUPS-GSD-01 |
| D7-S5-A99-T02 | `buildObserveSignalInput` 扁平化 ScopeContract 正确 | — |
| D7-S5-A99-T03 | `IncrementalOnly` 计算正确 | — |
| D7-S5-A99-T04 | `buildLLMObservationUserPrompt` 字节等价旧实现 | L5-MUPS-GSD-02 |
| D7-S5-A99-T05 | golden snapshot 4 组合 PASS | L5-MUPS-GSD-06 |
| D7-S5-A99-T06 | 现有 `llm_observation_proposer_test.go` 3 测试 PASS | L5-MUPS-GSD-05 |
| D7-S5-A99-T07 | 现有 `observation_proposer_test.go` 5 测试 PASS | L5-MUPS-GSD-05 |
| D7-S5-A99-T08 | 现有 `item_observe_test.go` E2E 测试 PASS | L5-MUPS-GSD-05 |
| D7-S5-A99-T09 | 现有 `parse_reject_feedback_test.go` E2E 测试 PASS（DM-20260705-002 链路不破） | L5-MUPS-GSD-05 |

## Invariants

1. **0 行为变化**：M1 阶段所有现有测试 PASS；user prompt token 数与改造前一致
2. **prior_parse_reject 链路保留**：DM-20260705-002 引入的跨轮反馈链路不被破坏
3. **i18n guide header 保留**：`RenderFrameFieldGuideForFields` 调用方式不变（传入 `nil` 让 i18n 走全字段 guide 路径）
4. **SessionID 不进 user frame**：`pt:"-"` 标记；不写入 `LineFrameRegistry`；i18n 不校验
5. **no panic in production**：所有 panic 仅在 `init()` 期；运行期永不 panic（除非输入违反 L4 invariant）
