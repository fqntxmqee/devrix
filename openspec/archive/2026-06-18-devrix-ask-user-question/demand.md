---
demand-id: DM-20260618-006
title: ask_user_question ToolSurface — LLM 主动提问能力
priority: P1
status: S7 Archived
dsaft_domain: TOOL-SURFACE-1 (D2 横切)
created: 2026-06-18
parent_doc: openspec/archive/2026-06-17-devrix-tool-surface-contract/
---

# S1 需求：ask_user_question ToolSurface

## 1. 背景

devrix LLM 在执行多步任务时，常遇到需要用户决策的歧义点：
- "你要查的是文件诊断还是工具调用历史？"
- "修复方案 A 用正则改、方案 B 用 AST 改，你选哪个？"
- "是否开启 worktree 隔离？"

目前 LLM 只能通过 plain text 在 reply 里问用户，但飞书 IM 的文本回复没有"结构化选项" affordance，用户也容易答非所问。

clawcode (Claude Code v2.1.88) 通过 `AskUserQuestionTool` 解决了这个问题：LLM 调工具时填 1-4 个多选问题，每个问题 2-4 个选项，UI 渲染成可点击卡片。

devrix 需要 mirror 这个能力，但 IM bot 没有 UI 渲染，必须改为：
- LLM 调工具 → 工具向用户 IM 通道推送**纯文本格式化问题**（带编号 + "其他" 自动提示）
- 工具立即返回 success，让 LLM 知道"问题已发出"
- 用户回复通过下一轮 inbound message 自然流转

## 2. 目标

1 个 change 内实现 ask_user_question ToolSurface，沿用 devrix-tool-surface-contract (DM-007) 的 4 方法 interface，无 breaking change。

## 3. 验收标准

| ID | 标准 | 度量 | 关联 T 点 |
|---|---|---|---|
| AC1 | `ask_user_question` ToolSurface 实现，1-4 questions / 2-4 options / header≤12 / 唯一 label | 12 个 `TestAskUserQuestionSurface_*` 单测 | A04-T01 |
| AC2 | `BuildSurfaces` 装配新 surface，输出按 name 字典序排序稳定 | `go test -race ./internal/bootstrap/...` 既有 +1 测试 | A04-T02 |
| AC3 | `main.go` 装配 sender 桥接到 `gw.RouteOutbound` | 二进制启动无错，IM 推消息 | A04-T03 |
| AC4 | orthogonal flags 表登记（ReadOnly=T, OpenWorld=T, ConcurrencySafe=F, Destructive=F） | `OrthogonalFlagFor("ask_user_question")` 返回值匹配 | A04-T02 |
| AC5 | `InterruptBehaviorFor("ask_user_question")` 返回 `InterruptCancel` | 单测断言 | A04-T02 |
| AC6 | 启动期 0 错误，tool list 可见 `[ask_user_question] 1 tools` | `./bin/devrix tool list` 输出含新 surface | A04-T04 |
| AC7 | 既有 TOOL-SURFACE-1 P0 T 点 (T01-T11) 保持 PASS | `go test -race ./...` 0 fail | (回归) |
| AC8 | `go vet ./...` + `go build ./...` 无新 warning | CI | — |
| AC9 | 文件规模 < 800 行，函数 < 50 行 | review | — |

## 4. 风险

| 风险 | 等级 | 缓解 |
|---|---|---|
| IM 通道 sender 未装配 | M | `SetAskUserQuestionSender(nil)` → graceful no-op（Delivered=false），工具仍返回 success |
| 同步阻塞反模式 | M | 设计文档明确禁止；description 引导 LLM "do NOT use for trivial clarifications" |
| 与既有 v2 workmodel `task_*` 工具命名冲突 | H | 见 §5 hotfix 发现 — 主动收敛范围 |
| LLM 滥用 ask_user_question 拖延 | L | 工具 description 内置 "just ask in plain text" guidance；后续可加 per-turn 配额 |

## 5. Hotfix 发现（v2 workmodel task_* 冲突）

> 关联提案 §4.2

本次 change 的**初始范围**同时包含"实现 TaskCreate/TaskGet/TaskUpdate 任务管理工具"。但实现完成后启动报 `register task_create: tool already registered: task_create`。

排查：`devrix.yaml` `tasks.mode: v2` + `workmodel.RegisterTaskTools` 早已注册同名 5 个工具（task_create/get/list/update/delete），且功能**更全**（owner / blocked_by / 依赖图 / delete）。新增的 `TaskRegistry` + `RegisterTaskLifecycleTools` 是严格子集，触发冲突。

**处理**：本 change 范围收敛到 ask_user_question；任务管理 LLM 能力沿用 v2 workmodel 既有 5 个工具（无需新代码）。在 `context_engine.go` 加注释记录这个所有权边界。

**效果**：B 部分需求 0 新增代码就达成（v2 workmodel 工具早就 cover 了），避免重复造轮子。

## 6. 上下游 change

| 方向 | change | 关系 |
|---|---|---|
| 上游 | devrix-tool-surface-contract (DM-007, PR #63+#64) | 4 方法 interface 基线 + 6 单例 + 3 filter 拆面 |
| 上游 | devrix-tool-surface-phase2-full (DM-008, PR #64) | ctor 注入替代 process-wide global |
| 上游 | devrix-tool-spec-enrichment (DM-001, PR #67) | ToolSpec 4 bool 字段 + InterruptBehavior + BuildSurfaces sort |
| 上游 | devrix-surface-permission-extension (DM-002, PR #68) | CheckPermission hook |
| 上游 | devrix-surface-lazy-loading (DM-003, PR #70) | ToolSearchSurface + Zod schema |
| 下游 | （none planned） | 本 change 是 v1.0 终态的 tool surface 套件 |
