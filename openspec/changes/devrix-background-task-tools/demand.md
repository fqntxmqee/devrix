---
demand-id: DM-20260611-009
title: Background Task 工具 — task_stop / task_output 对齐 clawcode
source: clawcode 能力对照（TaskStopTool / TaskOutputTool）
priority: P1
status: S2_Clarified
dsaft_domain: context-engine
created: 2026-06-11
---

# Background Task 工具 — task_stop / task_output

## 1. 背景

Devrix QueryLoop v2 已有 **异步 SubQuery** 基础设施：

- `query.RunBackground` → `BackgroundRegistry`
- 完成时 `SessionQueue` 推送 `task-notification`

但 Leader **无法通过 LLM 工具**：

- 主动 **停止** 运行中的 background SubQuery
- **轮询/阻塞** 获取 background 任务输出（仅依赖被动 notification）

clawcode 对照：

| clawcode Tool | 作用 | Devrix |
|---------------|------|--------|
| `TaskStop` | 停止 background task（agent/shell） | ❌ |
| `TaskOutput` | block/poll 获取 task 输出 | ❌ |
| `TaskCreate/Get/List/Update` | Todo 任务图 | ✅ `task_*`（TaskManager） |

> **命名说明：** Devrix `task_create` 等指 **Plan 任务图**（TaskManager）；本需求 `task_stop`/`task_output` 指 **Background SubQuery**（BackgroundRegistry），与 clawcode 语义一致。

## 2. 问题陈述

| 场景 | 现状 | 应有行为 |
|------|------|----------|
| Leader 启动 async delegate 后想先看别的 | 只能等 notification | `task_output(block=false)` 查状态 |
| 用户 `/new` 或 Leader 决定取消 | goroutine 继续跑 | `task_stop(task_id)` 取消 ctx |
| Wave Worker 运行过久 | 无统一 cancel API | Scheduler 内部 + 可选暴露给 Leader |
| compact 后 task_id 丢失 | notification 可能遗漏 | `task_list_background` 读 registry |

## 3. 范围

**In scope：**

- `task_stop`：取消 BackgroundRegistry 中 running 任务（context cancel）
- `task_output`：按 task_id 返回 status/result；支持 `block` + `timeout_ms`
- `BackgroundRegistry` 扩展：`Cancel(taskID)`, `List(sessionID)`, 持久化可选 P2
- QueryLoop 注册两工具；permission mode 下 Leader 可用
- Wave Scheduler Worker cancel **复用同一 Cancel 协议**（DM-007 依赖）

**Out of scope：**

- Background **Bash**（`run_in_background`）— IM 场景 P3
- TaskManager DAG 的 stop（用 `task_update status=cancelled`）
- 跨 session 任务可见性

## 4. 验收标准

### P0

- [ ] `task_stop(task_id)` 取消 running SubQuery，registry status → `cancelled`
- [ ] `task_output(task_id, block=false)` 返回 running/completed/failed + 已有 result 片段
- [ ] `task_output(task_id, block=true, timeout_ms=30000)` 阻塞至完成或超时
- [ ] 取消后 SessionQueue **不**再发 completed notification（或发 cancelled notification）
- [ ] D2-S9-T01~03 单测/集成测试绿

### P1

- [ ] `task_list_background(session)` 列出 registry 中任务（compact 后可恢复 ID）
- [ ] D6 `BackgroundTaskProbe`：stop/output 路径覆盖率
- [ ] Wave Worker `CancelWorker(taskID)` 与 `task_stop` 共享 `context.CancelFunc` 注册表

### P2

- [ ] BackgroundRegistry 磁盘 persist（重启后可 list，不要求 resume 执行）

## 5. 领域映射

| L4 | 模块 |
|----|------|
| L4-BE-CTX-BG-STOP | `query/background.go` Cancel |
| L4-BE-CTX-BG-OUTPUT | `query/background_tools.go` |
| L4-BE-ORCH-WAVE-CANCEL | Wave 复用（DM-007） |

## 6. 依赖

- **前置：** DM-012 QueryLoop + RunBackground（已交付）
- **下游：** DM-007 Wave Worker shutdown 依赖本需求 Cancel 协议

## 7. clawcode 参照文件

- `clawcode/src/tools/TaskStopTool/TaskStopTool.ts`
- `clawcode/src/tools/TaskOutputTool/TaskOutputTool.tsx`
- `clawcode/src/utils/task/framework.ts`
