# Implementation Tasks: MUPS 标签语义层

**Change ID:** `mups-prompt-tag-semantics`  
**Demand ID:** DM-20260705-001  
**Status:** S4_Complete  
**Design:** [`design.md`](design.md)

---

## Phase P0 — Semantics registry + appendix

| Task | L4/L5 | 文件 |
|------|-------|------|
| [x] T1 创建 `semantics.go` — `FieldSemantic`, `PhaseSemantics`, `SemanticsForPhase` | shared-A97 | `internal/shared/prompttags/semantics.go` |
| [x] T2 创建 `prompttags_semantics_{zh,en}.go` — i18n 短 bullet | D2-S15-A97 | `internal/layers/contextengine/i18n/` |
| [x] T3 `RenderSemanticAppendix` + 接入 `ObservationTaskAppendix` | D2-S15-A97 | `format_hints_mups.go` |
| [x] T4 接入 `StrategicPlanAppendix`（mode 决策树 + contract 示例） | D2-S15-A97 | `prompt_dynamic.go` |
| [x] T5 接入 `WorkItemExecuteOutputHints`（Required/Optional 矩阵） | D2-S15-A97 | `workitem_execute.go` |
| [x] T6 `semantics_test.go` — enforced 标志与 tagPhases 一致 | — | `prompttags/` |

**L5 映射：**

- T3 → **L5-MUPS-TAG-01** Observe kind when-use 出现在 appendix
- T4 → **L5-MUPS-TAG-02** Plan execution_mode 决策 + uncertainty 说明
- T5 → **L5-MUPS-TAG-03** Execute envelope Required/Optional 三分

## Phase P1 — User frame + golden

| Task | L4/L5 | 文件 |
|------|-------|------|
| [x] T7 `RenderFrameFieldGuide` — Observe/Plan 控制面字段一行说明 | D2-S15-A97 | `semantics.go` + linefield 调用点 |
| [x] T8 Golden hash — zh/en Observe/Plan/Execute system prompt | L5-MUPS-TAG-04 | `materialize/*_test.go`, `i18n/*_test.go` |
| [x] T9 proposer 测试断言语义 marker（无 D7 字符串） | — | `llm_observation_proposer_test.go`, `strategic_plan_proposer_test.go` |

**L5 映射：**

- T7 → **L5-MUPS-TAG-05** user frame control/data 标注

## Phase S3 — OpenSpec closure（规划阶段）

- [x] `demand.md`
- [x] `proposal.md`
- [x] `design.md`
- [x] `tasks.md`
- [x] `specs/shared/prompttags-semantics.md`
- [x] `.openspec.yaml`

## Phase S4 — t-registry（开发时）

- [x] 登记 D2-S15-A97-T01..T04、D7-S5-A97-T01..T02；T 点计数更新 `openspec/t-registry.md` + 域 registry

## Phase P2 — defer

- [ ] parse reject feedback loops → next-round user frame（DM-005 P2 或子 change）

## Verification

```bash
go test ./internal/shared/prompttags/... -count=1
go test ./internal/layers/contextengine/i18n/... -count=1
go test ./internal/layers/contextengine/materialize/... -count=1
go test ./internal/layers/orchestration/sessionorchestrator/... -count=1
```

**Token 检查：** Materialize Observe/Plan/Execute `TokenEst` 增量记录在 acceptance-report（S5）。
