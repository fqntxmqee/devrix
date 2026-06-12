# Tasks: Unified Task Registry

**Demand ID:** DM-20260612-011  
**Status:** S2_Clarified

## Phase 1 — TaskRegistry 核心

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T1 | 定义 `TaskRegistry` 接口 + 内存模型（status/outputOffset/notified） | L4-BE-CTX-TASK-REGISTRY | {T}-CTX-REG-01 | ~80 |
| T2 | disk output：`AppendOutput` / `GetOutputDelta` | L4-BE-CTX-TASK-REGISTRY | {T}-CTX-REG-02 | ~100 |
| T3 | `SetTerminal` + SessionQueue 入队 + notified 去重 | L4-BE-CTX-TASK-REGISTRY | {T}-CTX-REG-03, {T}-CTX-REG-04 | ~80 |

## Phase 2 — 适配层

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T4 | BackgroundRegistry → TaskRegistry 适配（RunBackground） | L4-BE-CTX-TASK-REGISTRY | {T}-CTX-REG-01 | ~60 |
| T5 | Wave SubAgentRunner / AgentToolRunner 注册 terminal | L4-BE-CTX-TASK-REGISTRY | {T}-CTX-REG-05 | ~80 |
| T6 | DM-009 `task_output` / `task_stop` 改读 TaskRegistry | L4-BE-CTX-BG-OUTPUT | D2-S9-T17 | ~60 |

## Phase 3 — QueryLoop 与 wave_completed

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T7 | `collectBackgroundTaskAttachments`（per-turn delta） | L4-BE-CTX-BG-ATTACHMENTS | — | ~80 |
| T8 | `BuildWaveCompletedAttachment(sessionID)` 供 DM-007 T23 | L4-BE-CTX-TASK-REGISTRY | {T}-ORCH-22 | ~60 |
| T9 | bootstrap 注册 GlobalTaskRegistry + devrix.yaml | L4-BE-CTX-TASK-REGISTRY | — | ~40 |

## Phase 4 — 测试与 T 层登记

| ID | 任务 | L5 |
|----|------|-----|
| T10 | 单元：delta / notified / List | {T}-CTX-REG-01~04 |
| T11 | 集成：SubQuery background + task_output | {T}-CTX-REG-02 |
| T12 | 登记 {T}-CTX-REG-01~05 → t-registry.md | ALL |

## 依赖顺序

```
T1 → T2 → T3 → T4 → T6
         ↓
        T5 → T8
T7 可与 T4 并行
T9 在 T1 后
DM-007 T23 依赖 T8
```

## 建议 PR 拆分

1. **PR-1**: T1–T3 + T12（registry 核心）
2. **PR-2**: T4 + T6（Background 适配 + DM-009）
3. **PR-3**: T5 + T8（Wave 集成 + wave_completed payload）
4. **PR-4**: T7 + T9 + T10–T11（QueryLoop + 测试）
