# Proposal: Wave Scheduler — DAG 并行 Worker 池

**Change ID:** devrix-wave-scheduler  
**Demand ID:** DM-20260611-007  
**Status:** S3_Design（design.md 已就绪，待 S3-Gate 审查）

## 1. Background

Devrix v2 已有 ExecutionFlowHub、SubQuery、D4 Delegate、`call_cursor` / `call_claude-code` Agent Tool，但：

- 并行仅限 QueryLoop 内 **读类 tool 批并行** 或 Leader **按需** 调 delegate/async
- CLI Agent 在设计 v2 中 **不参与委派路径**
- IM 仅 **单 Agent 流式卡** 或 **单 Task 进度汇总卡**

产品需要：Plan 产出 DAG 后，**固定 5 槽 Worker 池**自动派活，飞书 **一 Worker 一卡双区块**，并按 Task 关系决定上下文是否续接。

## 2. Problem Statement

| 问题 | 影响 |
|------|------|
| 无 DAG→Worker 池调度器 | 复杂任务无法稳定 5 路并行 |
| CLI Agent 仅被动 tool 调用 | 无法与 SubAgent 统一配额调度 |
| IM 单卡/汇总卡 | 多 Worker 并行时用户无法区分进度 |
| 上下文默认继承 Leader 全量 | 新派 Task 噪音大；续接 Task 又缺 artifact |
| 同目录并行写 | 需调度层规避，非 worktree |

## 3. Decision（已确认）

| # | 决策 | 选择 |
|---|------|------|
| D1 | DAG 来源 | **Plan Engine 自动生成** |
| D2 | 调度模式 | **持续填槽**（ready + 有空槽即派） |
| D3 | WorkDir | **同目录**；ConflictGuard 上层规避写冲突 |
| D4 | Worker 池 | cursor=1, claude-code=1, subagent=3 |
| D5 | IM | 每 Worker 独立卡：thinking + output |

## 4. What Changes

1. **TaskGraph 扩展**：Plan 产出节点含 `worker_type`、`context_policy`、`file_scope`、`conflict_group`
2. **WaveScheduler**（`orchestration/wave/`）：槽位注册表 + ready 队列 + 持续 dispatch loop
3. **WorkerRunner 适配**：SubQuery / call_cursor / call_claude-code 统一 `WorkerHandle` 接口
4. **ContextPolicy Resolver**：fresh / resume / upstream
5. **ConflictGuard**：同 `conflict_group` 或 `file_scope` 交集的写 Task 不并行
6. **Feishu WorkerCardRenderer**：`worker_id` → 独立双区块 card session
7. **Leader 集成**：Plan 完成后 `scheduler.Start(sessionID, graph)`；wave 事件回灌 Leader Queue

## 5. Non-Goals

- D7 跨 Session SoT 升格
- 用户 ↔ Worker 直接 IM 对话
- 默认 worktree 隔离

## 6. Risks

| 风险 | 缓解 |
|------|------|
| 同目录并行写 | ConflictGuard + Plan 标注 file_scope |
| 5 张卡飞书限流 | cardkit 节流 + Worker 卡 patch 合并 |
| Plan DAG 质量 | Milestone 降级 + Leader 人工 task 修正工具（后续） |

## 7. Success Metrics

- P0 L5 全绿（见 design.md §6）
- 飞书真机：5 Worker 并行可见 5 卡双区块流式
- 集成测试：DAG 10 节点持续调度，峰值并发=5
