# Tasks: MUPS Go-struct-driven I/O contract (M1 Observe)

**Change ID:** `mups-go-struct-driven`
**Demand:** DM-20260705-003
**Status:** S4_Implementation (planned)

## P0

| Task | L4/L5 | Status |
|------|-------|--------|
| T1 `structbind.go`: `ptTag` struct + `parseFrameFieldTag` | shared-A99 | [ ] |
| T2 `structbind.go`: `MustRegisterFrame[T]` 反射注册 + 4 项 init panic 校验 | shared-A99 | [ ] |
| T3 `structbind.go`: `BuildLineFrameFromStruct` 反射序列化 | shared-A99 | [ ] |
| T4 `structbind.go`: `DocBlockFromStruct[T]` 反射 schema 文档 | shared-A99 | [ ] |
| T5 `semantics.go`: `HasFrameFieldGuide(frame, tag)` 校验函数 | shared-A99 | [ ] |
| T6 `structbind_test.go`: 5 子测试（register / build / doc / panic 4 项 / 字段数漂移） | shared-A99-T05 | [ ] |
| T7 `observation_proposer.go`: `ObserveSignalInput` 加 9 个 `pt:"..."` struct tag | D7-S5-A99 | [ ] |
| T8 `observation_proposer.go`: `init()` 调 `MustRegisterFrame[ObserveSignalInput](FrameObserveUser)` | D7-S5-A99 | [ ] |
| T9 `observation_proposer.go`: `buildObserveSignalInput` 扁平化 ScopeContract → ScopeGoal / ScopeOpenQuestions | D7-S5-A99 | [ ] |
| T10 `observation_proposer.go`: 计算 `IncrementalOnly = len(PriorObservationIDs) > 0` | D7-S5-A99 | [ ] |
| T11 `llm_observation_proposer.go`: `buildLLMObservationUserPrompt` 35 行 → 2 行（BuildLineFrameFromStruct + RenderFrameFieldGuide） | D7-S5-A99 | [ ] |
| T12 `llm_observation_proposer_test.go`: 3 现有测试 0 行为变化验证 | D7-S5-A99 | [ ] |
| T13 `observation_proposer_test.go`: 现有 5 测试 0 行为变化验证 | D7-S5-A99 | [ ] |
| T14 `item_observe_test.go` + `parse_reject_feedback_test.go`: 现有 E2E 测试 0 行为变化验证 | D7-S5-A99 | [ ] |
| T15 L5-MUPS-GSD-01: `MustRegisterFrame[ObserveSignalInput]()` init 成功 | L5 | [ ] |
| T16 L5-MUPS-GSD-02: `BuildLineFrameFromStruct` 字节等价旧 `buildLLMObservationUserPrompt` | L5 | [ ] |
| T17 L5-MUPS-GSD-03: `DocBlockFromStruct[ObserveSignalInput]()` 字段一致 `DocBlockObserveSchema` | L5 | [ ] |
| T18 L5-MUPS-GSD-04: pt tag 缺失 / plane 错误 / i18n 缺翻译 → init panic 4 项 | L5 | [ ] |
| T19 L5-MUPS-GSD-05: 现有 E2E 测试套件 0 行为变化（item_observe + llm_observation + parse_reject + observation_proposer） | L5 | [ ] |

## P1

| Task | L4/L5 | Status |
|------|-------|--------|
| T20 L5-MUPS-GSD-06: golden snapshot `testdata/observe_user_prompt.golden` 4 组合 PASS | L5 | [ ] |
| T21 `t-registry.md` D7-S5-A99 + shared-A99 注册 9+5 T 点 | d7 t-registry | [ ] |
| T22 `a-registry.md` D7-S5-A99 + shared-A99 活动登记 | d7 a-registry | [ ] |
| T23 `openspec/specs/shared/prompttags.md` §API contract 加 `structbind` 一节 | shared/prompttags.md | [ ] |
| T24 `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md` 新增（M2-M5 follow-on 索引） | d7 spec | [ ] |
| T25 `CHANGELOG.md` d7-orchestration 追加一行 | d7 CHANGELOG | [ ] |
| T26 Draft PR 创建 + 标 ready | git-workflow | [ ] |

## Verification

```bash
# 单包验证
go vet ./internal/shared/prompttags/... ./internal/layers/orchestration/sessionorchestrator/... ./internal/layers/contextengine/i18n/...
go test ./internal/shared/prompttags/... -race -count=1
go test ./internal/layers/orchestration/sessionorchestrator/... -race -count=1
go test ./internal/layers/contextengine/i18n/... -race -count=1

# 全仓回归
go test ./... -race -count=1

# 行为不变性（手工）
diff <(git show master:internal/layers/orchestration/sessionorchestrator/observation_proposer.go | grep -A 30 'buildLLMObservationUserPrompt') \
     <(cat internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go | grep -A 5 'buildLLMObservationUserPrompt')
# 期望：旧版 35 行 → 新版 2 行；user prompt 输出 token 等价
```

## Rollback Plan

- `git revert <commit>` 一行回滚（pure refactor，无数据迁移）
- 旧 `BuildAnnotatedLineFrame` API 保留（M2 移除），无需同步改动 Plan 节点
- `LineFrameRegistry` 仍是合法手写入口，`MustRegisterFrame` 是新增而非替换

## Out-of-scope（不实现）

- M2 Plan 节点 go-struct 化
- M3 Strategy 抽象
- M4 Verify 表驱动
- M5 SpawnDecision 代数化
- 任何 Execute / Verify / Learn 节点改造
