---
demand-id: DM-20260610-012
title: Devrix 上下文引擎 QueryLoop 全量对齐 Claude Code Harness
source: 架构演进 / Claude Code 源码对标
priority: P0
status: S7_Archived
l1-domain: context
created: 2026-06-10
---

# Devrix 上下文引擎 QueryLoop 全量对齐 Claude Code Harness

## 1. 背景

Devrix Layer 2 已具备 V5 Harness Bootstrap、七步压缩、PEV 循环、孤立 tasks 包等能力，但**推理运行时**仍以 PEV 固定迭代为主，与 Claude Code 的 `queryLoop`（while tool_use continue）在语义、上下文组装时机、Attachment 注入、PermissionMode、SubQuery 上下文隔离上存在结构性差距。

我们有 Claude Code v2.1.88 完整源码（`claude-code-source-code/`）可对齐实现，复杂度不是约束；目标是在能力上**尽量拉齐** Claude Code 12 层 Progressive Harness，Devrix 独有增强（PEV Verify、Milestone、Feishu Gateway）作为 Hook 叠加而非替代。

## 2. 目标

| 维度 | 目标 |
|------|------|
| 运行时 | 单一 `QueryLoop` 替代/包裹所有 LLM↔Tool 往返（主线程、SubQuery、Background Agent） |
| 上下文 | System / UserContext(API 边界) / Messages / Attachments 四层分离，对齐 cache 与 token 策略 |
| 规划 | PermissionMode Plan + Enter/Exit + Explore/Plan 子 Agent + Plan 文件 + Task 图 |
| 压缩 | 每轮迭代前 messages-only 七步管道（snip/micro/collapse/autocompact） |
| 可观测 | queryChainId/depth、sidechain transcript、compression step 事件 |

## 3. 非目标（本需求）

- call_claude CLI 适配器内部改造（仅消费 SubQuery 契约）
- Feishu 卡片 UX 重设计
- Claude Code 内部 feature flag 未开源模块的 1:1 复刻（Coordinator/KAIROS 等仅预留接口）

## 4. 验收标准（P0）

| ID | 标准 |
|----|------|
| AC1 | 主线程多轮 tool_use 由 QueryLoop 驱动，直至无 tool 或 max_turns/stop hook |
| AC2 | UserContext（AGENTS.md + date）仅在 API 调用前 prepend，不写入 SessionContext.Messages |
| AC3 | Plan Mode 下 tool pool 只读 + plan 文件可写；轮间注入 plan_mode attachment（full/sparse throttle） |
| AC4 | task_create/update/get/list 注册主 ToolRunner，磁盘持久化 |
| AC5 | SubQuery 可 fork 父上下文或干净启动；Explore/Plan 内置 agent omit AGENTS.md |
| AC6 | `harness.enabled=false` 保持 V4 回归路径可用 |

## 5. L1–L5 映射

| 层级 | 资产 |
|------|------|
| L1 | context |
| L3-BE | CTX-QueryLoop, CTX-PlanMode, CTX-SubQuery |
| L4 | query_loop, user_context, attachments, permission_mode, task_tools, subquery, sidechain_transcript |
| L5 | L5-CTX-34 ~ L5-CTX-42（见 design.md） |
