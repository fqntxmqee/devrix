# Proposal: Background Task 工具

**Change ID:** devrix-background-task-tools  
**Demand ID:** DM-20260611-009  
**Status:** S4_Developing（task_stop / task_output / task_list_background 工具已实现，D2-S9-T16~19 IMPLEMENTED，-race 测试全绿，待 Wave Worker 接线与 S4-Gate 审查）

## Summary

为 QueryLoop 异步 SubQuery（`BackgroundRegistry`）暴露 `task_stop` / `task_output` LLM 工具，对齐 clawcode 后台任务可观测/可取消能力，并为 Wave Scheduler Worker 生命周期提供统一 Cancel 协议。

## Capabilities

| Capability | 说明 |
|------------|------|
| `background_task_stop` | 按 task_id 取消 running SubQuery |
| `background_task_output` | poll/block 获取输出与状态 |
| `background_task_cancel_protocol` | 共享 CancelFunc 注册表，Wave Worker 复用 |

## Non-Goals

- TaskManager DAG 工具变更
- Background Bash

## Risks

- Cancel 时 partial tool_result 需与 TD-QL-06 tombstone 策略一致
- 与 Wave 并行 cancel 需 `-race` 测试
