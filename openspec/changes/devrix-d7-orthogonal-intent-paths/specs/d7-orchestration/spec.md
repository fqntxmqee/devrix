# D7 Intent 路径正交化 Specification

**Capability:** d7-orchestration
**Status:** Active
**Change:** devrix-d7-orthogonal-intent-paths
**Demand:** DM-20260615-004
**Last Updated:** 2026-06-15

---

## ADDED

### Requirement: D7-S2 Intent 路径正交分发

`SessionOrchestrator.ProcessMessage` MUST dispatch 4 个 IntentKind 到 4 个**独立**的执行链。`IntentCommand` 不再走 FastPath，而是显式调用 `workmodel.CLICommands` 或 `workmodel.PlanCLICommands`。`IntentOrchestrate` 不再走 FastPath，而是显式调用 `TaskDecomposer.SynthesizeTaskGraph` + `WaveScheduler.Start`。

**Priority:** P0
**Package:** `internal/layers/orchestration/coordinator/`
**T:** D7-S2-A01-T04, D7-S2-A01-T05, D7-S2-A01-T06

#### Scenario: IntentCommand 显式分发到 PlanCLICommands

- GIVEN `d7_enabled=true` 且消息为 `/plan add auth`
- WHEN `ProcessMessage` 被调用
- THEN classifier 输出 `Intent{IntentCommand, Command:"/plan", ...}`
- AND orchestrator 调 `CommandHandler.Handle`（不调 FastPath）
- AND `CommandHandler` 解析 `args=["add","auth"]` 并调 `PlanCLICommands.Handle`
- AND D1 sink 收到 `command_reply` 事件 + `text` 事件 + `complete` 事件
- AND FastPath 不被调用（`D2.QueryLoop` 调用计数 = 0）

<!-- T: D7-S2-A01-T04 -->

#### Scenario: IntentCommand 显式分发到 CLICommands

- GIVEN `d7_enabled=true` 且消息为 `/task list`
- WHEN `ProcessMessage` 被调用
- THEN classifier 输出 `Intent{IntentCommand, Command:"/task", ...}`
- AND `CommandHandler` 调 `workmodel.ParseCommand` + `CLICommands.Handle` 走 task list 分支
- AND FastPath 不被调用

<!-- T: D7-S2-A01-T04 -->

#### Scenario: IntentOrchestrate 走 SynthesizeTaskGraph + WaveScheduler

- GIVEN `d7_enabled=true` 且消息为 `fix bug in auth.go && add tests`
- WHEN `ProcessMessage` 被调用
- THEN classifier 输出 `IntentOrchestrate`
- AND orchestrator 调 `OrchestratePath.Run`（不调 FastPath）
- AND `OrchestratePath` 调 `TaskDecomposer.SynthesizeTaskGraph(sessionID, message)`
- AND 调 `WaveScheduler.Start(ctx, sessionID, graph)`
- AND 调 `WaveScheduler.WaitForCompletion(ctx, sessionID)`
- AND 输出 `plan_formed` → `wave_started` → `text` (summary) → `complete` 事件序列

<!-- T: D7-S2-A01-T05 -->

#### Scenario: IntentFast 保持 FastPath

- GIVEN `d7_enabled=true` 且消息为 `hello`（短单 token 命中 fastPattern 或 ≤32 chars）
- WHEN `ProcessMessage` 被调用
- THEN orchestrator 调 `FastPath.Run`（保持 v1.0 closure 行为）
- AND `CommandHandler` 与 `OrchestratePath` 不被调用

<!-- T: D7-S2-A01-T06 -->

#### Scenario: IntentSkip 仍 close channel

- GIVEN `d7_enabled=true` 且消息为空字符串或仅空白
- WHEN `ProcessMessage` 被调用
- THEN 返回一个已 close 的 `EngineEvent` channel
- AND `FastPath` / `CommandHandler` / `OrchestratePath` 均不被调用

## MODIFIED

(None)

## REMOVED

### Requirement: D7-S2 v1.0 IntentHint passthrough（已移除）

v1.0 closure 阶段（2026-06-15）的临时妥协：把 `IntentCommand` 与 `IntentOrchestrate` 路由到 `FastPath.Run` 并在 `system_prompt` 注入 `[command:xxx]` / `[orchestrate:...]` 字符串前缀，让 LLM 自行解释命令 / 拆解任务。

**v1.1.0 移除原因：**
- 违反 DSAFT 活动边界原则（4 路径应 4 真实执行链）
- IntentKind 语义超载（决策 vs hint）
- intent_kind metric 样本不可比
- v1.1 已 ready PlanMode / SynthesizeTaskGraph / WaveScheduler 真实实现

**v1.1.0 替代实现：** 见本文件 `ADDED Requirements` §`D7-S2 Intent 路径正交分发`。
