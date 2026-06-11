# Proposal: Devrix QueryLoop Context Harness

**Change ID:** devrix-queryloop-context  
**Demand ID:** DM-20260610-001  
**Status:** S3_Design

## 1. Background

Claude Code 的生产级 Agent Harness 以 `query.ts` 为中心，叠加 12 层机制（Loop → Tool → Plan → SubAgent → Knowledge → Compress → Tasks → Background → Teams → Protocol → Autonomous → Worktree）。Devrix V5 实现了其中部分（压缩管道、Harness Bootstrap、孤立 Plan/Task 模块），但缺少统一的 Query 运行时与 Attachment/Permission 语义。

## 2. Problem Statement

- **双循环语义**：PEV `max_iterations` + Verify 与 Claude Code「直到无 tool_use」不一致，长任务易提前截断或重复 synthesis。
- **上下文污染**：AGENTS.md  baked 进 system prompt，无法像 CC 一样 API 边界 prepend + prompt cache。
- **Plan/Task 孤岛**：`tasks/plan_mode.go` 仅 CLI；`tool_suite` 未注册 ToolRunner。
- **SubAgent 无统一运行时**：multiagent 与 contextengine 各有一套消息组装逻辑。

## 3. Proposed Solution

引入 **Devrix Query Runtime（DQR）** 作为 Layer 2 唯一 LLM↔Tool 引擎：

```
Process → Bootstrap → QuerySession → QueryLoop.Run → Hooks(PEV Verify) → Persist
                              ↑
                    SubQuery.Run (AgentTool / builtin Explore|Plan)
```

Capabilities 清单见 `design.md` §3 Claude Code 12 层映射表。

## 4. Success Metrics

- 主路径 tool 轮次不再受 PEV `max_iterations=3` 硬限制（可配置 max_turns，0=无限）
- Plan Mode 端到端：Enter → Explore 并行 → Plan agent → plan 文件 → Exit 审批
- 上下文 token：UserContext prepend 模式下 system prompt 体积下降 ≥30%（同会话多轮）
- L5-CTX-34~42 集成测试全绿；V4 回归套件无退化

## 5. Implementation Plan

| 版本 | 范围 | Claude Code 机制 |
|------|------|------------------|
| **v1.0** | QueryLoop, UserContext, Attachments, PermissionMode, TaskTools, Sidechain 基础 | s01–s07 |
| **v1.1** | SubQuery/Fork, BackgroundTask 通知, StreamingToolExecutor | s04, s08 |
| **v2.0** | TeamCreate, SendMessage, Coordinator claim, Worktree | s09–s12 |
| **v3.0** | Devrix 超越层（见 design.md §8） | — |

## 6. Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| 大规模重构回归 | V4 路径冻结；feature flag `query_loop.enabled` |
| Provider tool_call 兼容 | 保留 synthesis 降级路径；normalizeMessages 层 |
| 复杂度 | 分 PR ≤400 行；源码注释标注 CC 对标文件:行 |

## 7. Out of Scope

- call_claude 深度改造
- 未开源 CC 模块（contextCollapse 真实算法、KAIROS daemon）— 接口预留
