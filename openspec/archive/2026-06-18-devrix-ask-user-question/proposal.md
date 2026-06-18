---
demand-id: DM-20260618-006
title: ask_user_question ToolSurface — 交互式 LLM Q&A via IM
priority: P1
status: S7 Archived
created: 2026-06-18
dsaft_domain: TOOL-SURFACE-1 (D2 横切)
parent_doc: openspec/archive/2026-06-17-devrix-tool-surface-contract/
---

# S2 提案：ask_user_question ToolSurface

## 1. 目标

为 devrix 增加一个 `ask_user_question` LLM 工具，让 LLM 在执行过程中遇到二选一/多选歧义时，可以主动向用户提问，拿到结构化答案再继续。

参考 clawcode (Claude Code v2.1.88) 的 `AskUserQuestionTool` schema，但适配 devrix 的 IM bot 设计：

- clawcode 工具调用是 **同步阻塞**（前端 UI 等用户点击）—— devrix 没有前端，必须改为**非阻塞**：
  - 工具调用 → 立即向用户 IM 通道推送格式化问题 → 工具结果返回 `Delivered=true` 给 LLM
  - 用户的回复通过下一次 inbound message 到达，LLM 在下一轮 turn 看到

## 2. 关键决策

### 2.1 沿用 TOOL-SURFACE-1 契约（不破坏 DM-007 拆面）

新工具按 4 方法 `contracts.ToolSurface`（Name / Tools / RiskLevel / Execute）实现为**stateless** surface。`InterruptBehavior` 显式返回 `InterruptCancel`（在 orthogonal_flags.go 表里登记）。Sender 走 process-global hook（`SetAskUserQuestionSender`），平行于 `BackgroundTaskToolsDeps` 模式。

### 2.2 schema 与 clawcode 完全对齐

- 1-4 questions（`minItems: 1, maxItems: 4`）
- 每个 question 2-4 options（`minItems: 2, maxItems: 4`）
- `header` 字段 maxLength 12（IM 卡片 chip 宽度限制）
- `multi_select` bool 字段支持多选
- 选项 label 唯一（clawcode UI 自动检查，devrix 显式校验）
- 必填字段 `question` 不可空

### 2.3 IM 渲染格式（devrix-specific）

clawcode UI 自动给每个问题加 "Other" 入口；IM 通道没有这个 affordance。devrix 在每个问题的选项列表末尾显式追加 `其他. 直接回复你的想法` 行。

整体格式：
```
【Header A】
question text (可多选)
  1. label A — description A
  2. label B — description B
  其他. 直接回复你的想法

【Header B】
question text
  1. ...
  其他. ...

回复序号 (例如 1) 或选项文字即可。
```

### 2.4 sender 桥接

`cmd/devrix/main.go` 在 gateway 装配完成后，调用 `asksurface.SetAskUserQuestionSender(func(ctx, sessionID, text) error)`，内部调 `gw.RouteOutbound` 把消息推到飞书。metadata 带 `source=ask_user_question, blocking=false`，方便后续遥测区分。

## 3. 范围外（明确不做）

- **不做** 真正的同步阻塞（事件总线 rendezvous）—— 那是 v1.1+ 的事（CLI/REPL 适配器才需要）
- **不做** 历史问题 / 草稿保存 —— ask 是单轮交互
- **不做** webhook 回调 —— 用户回复走既有 inbound 通道
- **不动** v2 workmodel 任务管理工具 `task_create/get/list/update/delete`（见 §4.2 hotfix 发现）

## 4. 关键发现

### 4.1 (设计借鉴) clawcode 的 `AskUserQuestion` schema 严格度

clawcode 在 prompt 里明确禁止用 `ask_user_question` 问琐碎问题（"just ask in plain text"）。devrix 在 tool description 里 mirror 了同样的 guidance，避免 LLM 滥用。

### 4.2 (hotfix 发现) 任务管理工具 v2 workmodel 已覆盖

初始需求 B 是 "实现 TaskCreate/TaskGet/TaskUpdate"，本意是新增 task_lifecycle Tools package（`TaskRegistry` + `RegisterTaskLifecycleTools`）。但实现完成后启动时报错：

```
register task lifecycle tools error="register task_create: tool already registered: task_create"
```

排查发现：`devrix.yaml` 里 `tasks.mode: v2`，`workmodel.RegisterTaskTools` 在 `ToolRegistry` 早期就已经注册了同名 `task_create/get/list/update/delete` 5 个工具，且功能更全（带 `owner` / `blocked_by` / 依赖图 / `delete`）。新写的代码是严格子集，会触发同名冲突。

**最终处理**：
- 回滚新增的 `task_lifecycle_tools.go` / `task_registry.go` / 18 个单测
- `context_engine.go` + `context_engine_builder.go` 加注释说明 v2 workmodel 所有权
- 任务管理对 LLM 开放的能力沿用 v2 workmodel 既有 5 个工具

> 这是一个有价值的发现：v2 workmodel 任务管理工具已经覆盖了用户 B 的需求，**新增工作量为 0**。归档本 change 时同步记录，避免后续 contributor 重蹈覆辙。
