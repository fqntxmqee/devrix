---
demand-id: DM-20260612-011
title: Unified Task Registry — 后台任务统一注册与 output delta
source: clawcode 能力对照（task/framework + diskOutput + task-notification）
priority: P1
status: S2_Clarified
l1-domain: context-engine
created: 2026-06-12
---

# Unified Task Registry — 后台任务统一注册与 output delta

## 1. 背景

Devrix 并行执行的后台任务状态分散在多处：

| 来源 | 注册位置 | 监控方式 |
|------|----------|----------|
| SubQuery async | `BackgroundRegistry` | 30ms 轮询 `IsTerminal` |
| Wave Worker | `WaveScheduler.workerHandle` | `WorkerEvent` slog + IM |
| D4 Delegate | Fork child Agent | `FlowHub` + Join |

clawcode 用 **单一 Task 框架** 打通：

- `registerTask` → `AppState.tasks`
- `appendTaskOutput` → disk（`<task_id>.output`）
- `getTaskOutputDelta(offset)` → 每 turn 增量
- terminal → `enqueuePendingNotification(mode=task-notification)` → QueryLoop 下一 turn 回灌 Leader

Devrix 已有 DM-009（`task_stop` / `task_output`）和 DM-007 v1.2（`wave_completed` 回灌），但缺少 **统一 registry 作为数据源**，导致：

- Wave `finalizeTask` 无稳定 output 可读
- SubAgent 与 Wave Worker 的 task_id 语义不一致
- terminal 通知可能重复或遗漏（无 `notified` 去重）

## 2. 问题陈述

| 场景 | 现状 | 应有行为 |
|------|------|----------|
| Leader 想查看 running Wave Worker 输出 | 仅 IM 卡 / slog | `task_output(block=false)` 读 registry delta |
| SubQuery 完成 | SessionQueue notification | 同一 envelope + `notified` 防重 |
| Wave 全完成 | `WaitForCompletion` 返回 artifacts | **且** QueryLoop 自动注入 `wave_completed` 附件 |
| compact 后 task_id | 可能丢失 | `List(sessionID)` 可恢复 |
| Wave cancel | Scheduler cancel func | 与 `task_stop` 共享 Cancel 协议（DM-009） |

## 3. 澄清记录

### Q1: 与 TaskManager（Plan DAG）的关系？

**A**: **分离** — TaskManager 管 Plan 任务图（pending/blockedBy）；TaskRegistry 管 **运行时后台执行句柄**（running/completed + disk output）。Wave `TaskNode.ID` 可映射为 registry task_id，但不合并数据结构。

### Q2: 与 BackgroundRegistry 的关系？

**A**: **v1.0 包装/适配** — TaskRegistry 作为 facade，SubQuery 路径内部仍用 BackgroundRegistry，对外统一 API。避免大爆炸替换。

### Q3: 通知模式？

**A**: 对齐 clawcode **task-notification 语义**（非 Mailbox）：

- 单任务 terminal → `ModeTaskNotification`（已有 DM-012 路径）
- Wave 全 terminal → `ModeWaveCompleted`（DM-007 v1.2 T15，payload 从 registry + ArtifactStore 组装）

## 4. L1–L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | context-engine | 上下文引擎 | 已有 |
| L2 | L2-CTX-BG-UNIFIED | 统一后台任务观测 | **新增** |
| L3-BE | L3-BE-CTX-BG-NOTIFY | QueryLoop 消费 task/wave 通知 | **新增** |
| L4-BE | L4-BE-CTX-TASK-REGISTRY | TaskRegistry + disk output | **新增** |
| L4-BE | L4-BE-CTX-BG-ATTACHMENTS | collectBackgroundTaskAttachments | **新增** |
| L5 | L5-CTX-REG-01 ~ 05 | 见 §6 | **草拟** |

## 5. 范围

### In Scope

- `TaskRegistry` 接口：`Register`, `UpdateProgress`, `AppendOutput`, `SetTerminal`, `Get`, `List`, `GetOutputDelta`
- disk output：`<store_dir>/<task_id>.output` + byte offset（对齐 clawcode）
- QueryLoop 每 turn 收集 running task output delta（等价 `getUnifiedTaskAttachments`）
- terminal 时 `notified` 原子置位，避免重复入队 SessionQueue
- DM-009 `task_stop` / `task_output` / `task_list_background` **读写同一 registry**
- Wave Worker 注册/注销走 TaskRegistry（SubAgentRunner + AgentToolRunner）
- `wave_completed` 汇总附件的数据源（供 DM-007 T15）

### Out of Scope

- 跨 Session 任务可见性
- 重启后 resume 执行（P2 persist 仅 list，不 resume）
- TaskManager DAG 的 `claimTask`（另见 DM-007 / 后续 v1.3）
- Background Bash（`run_in_background` shell）

## 6. 验收标准

### P0

| ID | 标准 |
|----|------|
| AC1 | SubQuery `RunBackground` 注册后，registry 存在 `status=running` 条目 |
| AC2 | `task_output(task_id, block=false)` 返回 status + 自上次 offset 以来的 output delta |
| AC3 | terminal 后 `notified=true`；同一 task 不重复入队 SessionQueue |
| AC4 | `task_stop(task_id)` 取消 running 任务，registry `status=cancelled`（复用 DM-009） |
| AC5 | Wave Worker 完成时 registry 条目 terminal，且 output 含 runner 最终 result（非仅 directive） |

### P1

| ID | 标准 |
|----|------|
| AC6 | QueryLoop attachment 阶段对 running tasks 更新 output offset（不 clobber 并发 status） |
| AC7 | `wave_completed` 附件含每 task 的 status/summary/error/output_path |
| AC8 | D6 `BackgroundTaskProbe` 覆盖 registry register/delta/notify 路径 |

### P2

| ID | 标准 |
|----|------|
| AC9 | registry 元数据 disk persist（重启可 `List`，不 resume goroutine） |

### L5 测试点（草案，S3 登记 l5-registry）

| L5 ID | Given-When-Then | 优先级 |
|-------|-----------------|--------|
| L5-CTX-REG-01 | Given SubQuery background 启动 When Register Then registry 含 running 且 output 文件创建 | P0 |
| L5-CTX-REG-02 | Given running task 写入 output When GetOutputDelta Then 仅返回新字节 | P0 |
| L5-CTX-REG-03 | Given task terminal When SetTerminal Then notified 置位且 SessionQueue 入队一次 | P0 |
| L5-CTX-REG-04 | Given notified=true When 再次 SetTerminal Then 不入队 | P0 |
| L5-CTX-REG-05 | Given Wave Worker 完成 When List(session) Then 含 worker task_id 与 terminal status | P1 |

## 7. 依赖

| 方向 | 需求 | 说明 |
|------|------|------|
| 前置 | DM-012 QueryLoop | RunBackground、SessionQueue |
| 前置 | DM-009 Background Task 工具 | task_stop/output 工具面 |
| 下游 | DM-007 v1.2 T15 | wave_completed 附件从 registry 组装 |
| 关联 | DM-010 Wave Worktree | Worker WorkDir 变更不影响 registry task_id |

## 8. clawcode 参照

- `clawcode/src/utils/task/framework.ts` — registerTask, pollTasks, generateTaskAttachments
- `clawcode/src/utils/task/diskOutput.ts` — appendTaskOutput, getTaskOutputDelta
- `clawcode/src/tasks/LocalAgentTask/LocalAgentTask.tsx` — enqueueAgentNotification
