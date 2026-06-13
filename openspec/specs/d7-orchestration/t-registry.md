# D7 Orchestration Domain — T 层测试点注册表

**Status:** Active
**Version:** 2.0.0
**Last Updated:** 2026-06-14
**Parent:** `openspec/specs/architecture/layering.md`
**Spec:** `openspec/specs/d7-orchestration/spec.md`

---

## Overview

D7 T 层测试点注册表。现行测试以 ORCH-S2-T* 注释标注，本文档统一映射为 D7-S*-T* 编号。遗留 ORCH ID 保留在「Legacy ID」列以便追溯。

**状态：** IMPLEMENTED · PARTIAL · PLANNED

---

## D7-S4: Execution Flow

| T ID | Legacy ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|-----------|------|----------|-----------|--------|----------|
| D7-S4-T01 | ORCH-S2-T01 | WorkPlan.Snapshot 含 ExecutionFlow + 状态 | D7-S4-A02 | `orchestration/workplan/service_test.go` | IMPLEMENTED | P0 |
| D7-S4-T02 | — | Hub 双通道：WorkPlan + SessionQueue + IM | D7-S4-A01 | `orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| D7-S4-T03 | D4-S10-T04 | FlowStarted 触发 delegate-progress 入队 | D7-S4-A01-F02 | `orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| D7-S4-T04 | D4-S10-T07 | Snapshot 含 Task 投影（link_tasks） | D7-S1-A03-F02 | `orchestration/flow/hub_test.go` | IMPLEMENTED | P0 |
| D7-S4-T05 | D4-S10-T05 | IMSink 发射 worker_progress 事件 | D7-S4-A03-F01 | `orchestration/imsink/gateway_test.go` | IMPLEMENTED | P0 |
| D7-S4-T06 | — | FlowToolCall 节流（throttle_ms） | D7-S4-A01-F04 | `orchestration/flow/hub_test.go` | IMPLEMENTED | P1 |

---

## D7-S3: Wave Scheduler

| T ID | Legacy ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|-----------|------|----------|-----------|--------|----------|
| D7-S3-T01 | ORCH-S2-T10 | 6 ready subagent + 1 cursor 峰值并发≤5 | D7-S3-A01 | `orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T02 | ORCH-S2-T15 | 槽位释放后 ready Task 立即派发 | D7-S3-A01-F04 | `orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T03 | ORCH-S2-T17 | Plan DAG 仅 ready 节点被派发 | D7-S3-F03 | `orchestration/wave/scheduler_test.go`, `taskgraph_test.go` | IMPLEMENTED | P0 |
| D7-S3-T04 | ORCH-S2-T11 | upstream policy 收到 artifact，无 Leader 全量 | D7-S3-A02-F02 | `orchestration/wave/context_test.go`, `scheduler_orch_test.go` | IMPLEMENTED | P0 |
| D7-S3-T05 | ORCH-S2-T12 | fresh policy Messages 仅含 directive | D7-S3-A02-F01 | `orchestration/wave/context_test.go` | IMPLEMENTED | P0 |
| D7-S3-T06 | ORCH-S2-T13 | 同 conflict_group Task 不并行 | D7-S3-A03-F01 | `orchestration/wave/scheduler_orch_test.go` | IMPLEMENTED | P0 |
| D7-S3-T07 | ORCH-S2-T16 | cursor + claude-code 并行 file_scope 不交 | D7-S3-A03-F03 | `orchestration/wave/scheduler_orch_test.go` | IMPLEMENTED | P1 |
| D7-S3-T08 | ORCH-S2-T18 | wave 全完成返回全部 artifacts | D7-S3-A01-F03 | `orchestration/wave/scheduler_orch_test.go` | IMPLEMENTED | P1 |
| D7-S3-T09 | ORCH-S2-T19 | CancelWorker 槽位释放 status=cancelled | D7-S3-A01-F05 | `orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T10 | ORCH-S2-T20 | CancelAll 5 running 全部 terminal | D7-S3-A01-F05 | `orchestration/wave/scheduler_test.go` | IMPLEMENTED | P0 |
| D7-S3-T11 | ORCH-S2-T21 | CLI Worker cancel 进程终止 | D7-S3-F06 | `orchestration/wave/runners/agent_tool_orch_test.go` | PARTIAL | P1 |

---

## D7-S1: Work Model

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| D7-S1-T01 | Task create 生成唯一 ID | D7-S1-A02-F01 | `contextengine/tasks/task_manager_test.go` | IMPLEMENTED | P0 |
| D7-S1-T02 | Task 依赖 blocked_by 正确 | D7-S1-A02-F03 | `contextengine/tasks/task_manager_test.go` | IMPLEMENTED | P0 |
| D7-S1-T03 | DiskStore v2 持久化恢复 | D7-S1-A02-F05 | `contextengine/tasks/disk_store_test.go` | IMPLEMENTED | P0 |
| D7-S1-T04 | ListReadyTasks 仅返回无阻塞任务 | D7-S1-A02-F04 | `contextengine/tasks/task_manager_test.go` | IMPLEMENTED | P1 |
| D7-S1-T05 | FlowEvent link_tasks 状态联动 | D7-S1-A02-F06 | `orchestration/flow/hub_test.go` | IMPLEMENTED | P1 |
| D7-S1-T06 | CreateWorkPlan DAG 校验 | D7-S1-A01-F02 | — | PLANNED | P0 |

---

## D7-S5: Decision & Planning

| T ID | 描述 | 归属 A/F | Test 位置 | Status | Priority |
|------|------|----------|-----------|--------|----------|
| D7-S5-T01 | PlanMode inactive→active 转换 | D7-S1-A04-F01 | `contextengine/tasks/task_manager_test.go` | IMPLEMENTED | P1 |
| D7-S5-T02 | PlanAgent 只读模式拒绝写操作 | D7-S5-A04 | — | PLANNED | P0 |
| D7-S5-T03 | ClassifyIntent 规则高置信跳过 LLM | D7-S5-A01 | — | PLANNED | P0 |
| D7-S5-T04 | SynthesizeTaskGraph 产出有效 DAG | D7-S5-A02 | — | PLANNED | P0 |
| D7-S5-T05 | SelectExecutor explore→D2 execute→D4 | D7-S5-A03 | — | PLANNED | P0 |

---

## D7-S2: Session Orchestrator (PLANNED)

| T ID | 描述 | 归属 A | Test 位置 | Status | Priority |
|------|------|--------|-----------|--------|----------|
| D7-S2-T01 | ProcessMessage 为 D1 主入口 | D7-S2-A01 | — | PLANNED | P0 |
| D7-S2-T02 | FastPath 延迟增量≤2ms | D7-S2-A01-F02 | — | PLANNED | P0 |
| D7-S2-T03 | OrchestratePath 创建 Plan | D7-S2-A01-F03 | — | PLANNED | P0 |
| D7-S2-T04 | HandleInterrupt 取消活跃任务 | D7-S2-A03 | — | PLANNED | P0 |

---

## Cross-Domain (D7 契约)

| T ID | 描述 | 归属 | Test 位置 | Status | Priority |
|------|------|------|-----------|--------|----------|
| D7-D1-T01 | D1 调用 D7 而非 D2（d7_enabled） | D7-S2 | — | PLANNED | P0 |
| D7-D4-T01 | D2 loop 无 delegate hooks | D7-S2 | — | PLANNED | P0 |
| D7-D6-T01 | D6 校验编排决策（advisory） | D7-S5 | — | PLANNED | P1 |
| D7-THIN-T01 | loop.go 无编排字段 | D2 瘦身 | — | PLANNED | P0 |
| D7-THIN-T02 | loop.go Run ≤200 行 | D2 瘦身 | — | PLANNED | P0 |

---

## D1 集成（IM 渲染）

| T ID | Legacy ID | 描述 | 归属 | Test 位置 | Status | Priority |
|------|-----------|------|------|-----------|--------|----------|
| D7-S4-T07 | ORCH-S2-T14 | 每 Task 独立双区块 IM 卡流式 | D1-S8 + D7-S4 | `communication/adapters/feishu_worker_card_test.go` | IMPLEMENTED | P0 |

---

## Statistics

| Total | IMPLEMENTED | PARTIAL | PLANNED | P0 |
|-------|-------------|---------|---------|-----|
| 33 | 22 | 1 | 10 | 22 |

### 按 Scenario

| Scenario | Total | IMPLEMENTED | PLANNED |
|----------|-------|-------------|---------|
| D7-S1 | 6 | 5 | 1 |
| D7-S2 | 4 | 0 | 4 |
| D7-S3 | 11 | 10 | 1 |
| D7-S4 | 7 | 7 | 0 |
| D7-S5 | 5 | 1 | 4 |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-13 | 初始（仅 ORCH-S2-T* 遗留 ID） |
| 2.0.0 | 2026-06-14 | D7-S*-T* 统一编号、Legacy 映射、S1/S5/契约 T 点补全 |
