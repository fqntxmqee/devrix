# D7 Orchestration Domain — T 层测试点注册表

**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## ORCH-S1: WorkPlan Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| ORCH-S2-T01 | WorkPlan.Snapshot 含 Task + ExecutionFlow | WorkPlan | `internal/layers/orchestration/workplan/service_test.go` | IMPLEMENTED | P0 |

## ORCH-S2: Wave Scheduler Module

| T ID | 描述 | S 映射 | Test 位置 | Status | Priority |
|-------|------|---------|-----------|--------|----------|
| ORCH-S2-T10 | DAG 6 ready subagent + 1 cursor 持续调度峰值并发=5 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T11 | upstream policy 收到 A artifact，无 Leader 全量 | ContextPolicy | `internal/layers/orchestration/wave/scheduler_orch_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T12 | fresh policy SubAgent 启动 Messages 仅含 directive | ContextPolicy | `internal/layers/orchestration/wave/context_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T13 | 同 conflict_group Task 不并行 | ConflictGuard | `internal/layers/orchestration/wave/scheduler_orch_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T14 | 每 Task 独立双区块 IM 卡流式 | WorkerCard | `internal/layers/communication/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T15 | 槽位释放后 ready Task 立即派发 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T16 | cursor + claude-code 并行 file_scope 不交 | ConflictGuard | `internal/layers/orchestration/wave/scheduler_orch_test.go` | IMPLEMENTED | P1 |
| ORCH-S2-T17 | Plan 产出 DAG 仅 ready 节点被派发 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T18 | wave 全完成 Leader 收到 wave_completed 汇总 | WaveScheduler | `internal/layers/orchestration/wave/scheduler_orch_test.go` | IMPLEMENTED | P1 |
| ORCH-S2-T19 | CancelWorker 槽位释放 status=cancelled | WorkerLifecycle | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T20 | CancelAll 5 running 全部 terminal pool 全释放 | WorkerLifecycle | `internal/layers/orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| ORCH-S2-T21 | CLI Worker cancel 进程终止 IM 卡 cancelled | WorkerLifecycle | `internal/layers/orchestration/wave/runners/agent_tool_orch_test.go` | PARTIAL | P1 |

---

## Statistics

| Total | IMPLEMENTED | PARTIAL | P0 |
|-------|-------------|---------|-----|
| 13 | 12 | 1 | 8 |
