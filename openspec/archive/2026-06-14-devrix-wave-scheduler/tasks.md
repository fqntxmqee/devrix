# Tasks: Wave Scheduler

**Demand ID:** DM-20260611-007  
**Status:** S3_Planning

## Phase 1 — 数据模型与 Plan 集成

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T1 | TaskNode 扩展：worker_type, context_policy, file_scope, conflict_group | L4-BE-ORCH-WAVE-SCHEDULER | {T}-ORCH-17 | ~80 |
| T2 | Plan Engine 物化 TaskGraph 写入 TaskManager | L4-BE-ORCH-WAVE-SCHEDULER | {T}-ORCH-17 | ~120 |
| T3 | ArtifactStore：Task 完成摘要持久化（内存+磁盘） | L4-BE-ORCH-CONTEXT-POLICY | {T}-ORCH-11 | ~60 |

## Phase 2 — WaveScheduler 核心

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T4 | WorkerPool 槽位：cursor×1, claude-code×1, subagent×3 | L4-BE-ORCH-WAVE-SCHEDULER | {T}-ORCH-10 | ~80 |
| T5 | WaveScheduler 持续 dispatch loop | L4-BE-ORCH-WAVE-SCHEDULER | {T}-ORCH-15 | ~150 |
| T6 | ConflictGuard：conflict_group + file_scope 互斥 | L4-BE-ORCH-CONFLICT-GUARD | {T}-ORCH-13 | ~100 |
| T7 | ContextPolicy resolver（fresh/resume/upstream） | L4-BE-ORCH-CONTEXT-POLICY | {T}-ORCH-11, {T}-ORCH-12 | ~120 |
| T7b | WorkerHandle + CancelWorker/CancelAll + 槽位释放 | L4-BE-ORCH-WAVE-CANCEL | {T}-ORCH-19, {T}-ORCH-20 | ~120 |
| T7c | CLI Runner SIGTERM/KILL on cancel | L4-BE-ORCH-WAVE-CANCEL | {T}-ORCH-21 | ~80 |

## Phase 3 — Worker Runner

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T8 | SubAgentRunner → query.Run + FlowEvent | L4-BE-ORCH-WAVE-SCHEDULER | {T}-ORCH-10 | ~80 |
| T9 | AgentToolRunner（call_cursor / call_claude-code）流式桥接 | L4-BE-ORCH-WAVE-SCHEDULER | {T}-ORCH-16 | ~120 |
| T10 | ExecutionFlowHub 扩展 task_id / worker_type metadata | ORCH-S2 | {T}-ORCH-14 | ~60 |

## Phase 4 — IM 双区块卡片

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T11 | WorkerCardSession 模型（per task_id） | L4-FE-IM-WORKER-CARD-RENDER | {T}-ORCH-14 | ~80 |
| T12 | feishu_worker_card.go：thinking + output 双区块 upsert | L4-FE-IM-WORKER-CARD-RENDER | {T}-ORCH-14 | ~200 |
| T13 | 接入 cardkit 流式（复用 feishu-streaming） | L4-FE-IM-WORKER-CARD-RENDER | {T}-ORCH-14 | ~80 |

## Phase 5 — Leader 集成与配置

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T14 | bootstrap WireWaveScheduler + devrix.yaml 配置 | L4-BE-ORCH-WAVE-SCHEDULER | — | ~60 |
| T15 | Plan **批准** → Scheduler.Start；AllTerminal → `ModeWaveCompleted` 入队 → QueryLoop 附件回灌 Leader（依赖 DM-011） | L3-BE-ORCH-DISPATCH | {T}-ORCH-18, {T}-ORCH-22~24 | ~120 |
| T15b | `finalizeTask` 实现：真实 Artifact + FlowHub + SessionQueue | L4-BE-ORCH-WAVE-SCHEDULER | {T}-ORCH-23, {T}-ORCH-24 | ~80 |
| T16 | 登记 {T}-ORCH-10~24 至 t-registry.md | — | ALL | ~40 |

## Phase 6 — 测试与验收

| ID | 任务 | L5 |
|----|------|-----|
| T17 | 单元：pool + conflict + context policy | {T}-ORCH-10~13 |
| T18 | 集成：mock DAG 持续调度 10 节点 | {T}-ORCH-10, {T}-ORCH-15 |
| T19 | 集成：IM worker card 双区块 | {T}-ORCH-14 |
| T20 | E2E（可选 P1）：飞书真机 5 卡并行 | {T}-ORCH-14 |

## 依赖顺序

```
T1 → T2 → T4 → T5 → T6 → T7 → T7b → T7c
              ↓
         T8, T9 → T10 → T11 → T12 → T13
              ↓
         T14 → T15 → T16 → T17~T20
T3 可与 T7 并行
T7b 依赖 DM-009 T1（Cancel 协议）；可先 stub 后对接
T15/T15b 依赖 DM-20260612-011 TaskRegistry v1.0
```

## Phase 7 — v1.2 Leader 闭环（DM-011 之后）

| ID | 任务 | L4 | L5 | 估行 |
|----|------|-----|-----|------|
| T21 | Plan gate：未批准 Plan 禁止 WaveScheduler.Start | L3-BE-ORCH-DISPATCH | {T}-ORCH-23 | ~40 |
| T22 | Runner terminal → 真实 Artifact（SubAgentResult / AgentTool complete） | L4-BE-ORCH-CONTEXT-POLICY | {T}-ORCH-24 | ~80 |
| T23 | `ModeWaveCompleted` SessionQueue + QueryLoop attachment 消费 | L3-BE-CTX-BG-NOTIFY | {T}-ORCH-22 | ~100 |

## 建议 PR 拆分（≤400 行/PR）

1. **PR-1**: T1–T3 + T16（模型 + Artifact）
2. **PR-2**: T4–T7（Scheduler 核心）
3. **PR-3**: T8–T10（Runners + Flow）
4. **PR-4**: T11–T13（IM 卡片）
5. **PR-5**: T14–T15 + 集成测试 T17–T18
