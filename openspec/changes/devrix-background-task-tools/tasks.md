# Tasks: Background Task 工具

**Demand ID:** DM-20260611-009

## Phase 1 — Registry Cancel 协议（~1d）

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T1 | `BackgroundRegistry.RegisterWithCancel` + `Cancel` + `List` | L4-BE-CTX-BG-STOP | D2-S9-T01 | ~80 | ✅ |
| T2 | `RunBackground` 接入 cancel ctx；`CompleteCancelled` | L4-BE-CTX-BG-STOP | D2-S9-T01 | ~60 | ✅ |
| T3 | 单测：stop / idempotent cancel / race | — | D2-S9-T01 | ~80 | ✅ |


## Phase 2 — LLM 工具（~1d）

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T4 | `task_stop` tool runner + schema | L4-BE-CTX-BG-STOP | D2-S9-T01 | ~60 | ✅ |
| T5 | `task_output` block/poll + timeout | L4-BE-CTX-BG-OUTPUT | D2-S9-T02, D2-S9-T03 | ~100 | ✅ |
| T6 | QueryLoop 工具注册 + permission 白名单 | — | — | ~40 | ✅ |
| T7 | 集成测试：async delegate → output → stop | — | D2-S9-T04 | ~80 | ✅ |

## Phase 3 — Wave 对接（~0.5d，可与 DM-007 PR-2 合并）

| ID | 任务 | L4 | L5 |
|----|------|-----|-----|
| T8 | 定义 `WorkerCancelRegistry` 接口 = BackgroundRegistry | L4-BE-ORCH-WAVE-CANCEL | — |
| T9 | 文档：DM-007 引用本 Cancel 协议 | — | — |

## 依赖

```
T1 → T2 → T3 → T4 → T5 → T6 → T7
T8 依赖 T1（可与 DM-007 并行规划）
```

## 分支

`feat/DM-20260611-009-background-task-tools`

## S4 准入

- [x] demand.md 澄清
- [x] proposal / design / tasks
- [ ] 用户确认后编码
