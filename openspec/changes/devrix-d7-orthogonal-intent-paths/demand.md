---
demand-id: DM-20260615-004
title: D7 Intent 路径正交化 — 4 分支 = 4 真实执行链
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-15
---

# D7 Intent 路径正交化

## 1. 背景

DM-020（D7 Turn 编排上移，v1.0 closure 2026-06-15）已经将 D7 确立为 D1 总入口，分类器识别出 4 种 IntentKind（`IntentSkip` / `IntentCommand` / `IntentFast` / `IntentOrchestrate`），但 `ProcessMessage` 的 switch 在 v1.0 closure 阶段**只落地了 1 条执行链**（FastPath.Run）+ 3 种 system_prompt 前缀字符串。

具体事实（来自 `coordinator/orchestrator.go:154-159, 227-237`）：

```go
case IntentCommand:     return o.handleCommand(ctx, req, intent)   // → fastPath.Run("[command:xxx]")
case IntentFast:        return o.fastPath.Run(ctx, req, "")          // → fastPath.Run
case IntentOrchestrate: return o.orchestrate(ctx, req, intent)        // → fastPath.Run("[orchestrate:...]")
```

Command 与 Orchestrate 的"执行"实际是把命令字符串塞进 LLM 的 system_prompt，**让 LLM 自己去理解这是命令 / 让他自己拆解**。v1.0 closure 注释（line 222-237）明确说明这是"v1.0 simplification"。

## 2. 问题陈述

### 2.1 IntentKind 语义撒谎

`IntentKind` 枚举的设计意图是**路由决策**（Intent → 哪条执行链），但 v1.0 实际只用作**LLM 提示前缀**。同一个字段同时被宣称为"决策"又实际只是"hint"，产生语义超载：

- 读代码的人会以为 `IntentCommand` 是命令执行路径
- 实际 `IntentCommand` 是"让 LLM 假装看到命令的 fast path"
- 后续维护者无法信任 `IntentKind` 的真实含义

### 2.2 分类器的价值被浪费

分类器花了 5 条规则去识别"这是命令 / 这是简单问候 / 这是要编排的"，但识别对了也是丢给同一个 FastPath 执行器。**如果识别不影响执行，分类本身价值为 0**。要么分类有用，要么分类冗余。

### 2.3 v1.1 迁移路径被堵

v1.1 已规划：
- IntentCommand 真实落地 → `PlanMode.Enter` / `BackgroundRun.Start`（D7-S5-A04 + D7-S1-A02）
- IntentOrchestrate 真实落地 → `SynthesizeTaskGraph` → `WaveScheduler.Schedule`（D7-S5-A02 + D7-S3-A01）

v1.0 把它们都收敛到 FastPath 意味着 v1.1 落地时要在 `fastPath.Run` 内部做 `if hint prefix == "[command:" do X / if hint prefix == "[orchestrate:" do Y` —— **把所有真实路径塞进同一个函数，函数膨胀不可逆**。早合并的债，晚合并还不起。

### 2.4 observability 失真

`intent_kind` metric 已按 4 类记，但执行链路只有 1 条。分析"哪类意图延迟高 / 哪类失败多"无意义——样本不可比。等真路径切出来，旧 metric 全部要回填重算。

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `ProcessMessage` 的 4 个 switch case 调用 4 个**独立**的执行函数（Skip 仍内联 close-channel，其余三个为独立函数） | P0 |
| AC2 | `handleCommand` 不再调 `fastPath.Run`，改为调 `workmodel.CLICommands.Handle`（`/task`）或 `workmodel.PlanCLICommands.Handle`（`/plan`）的 explicit dispatch | P0 |
| AC3 | `orchestrate` 不再调 `fastPath.Run`，改为调 `TaskDecomposer.SynthesizeTaskGraph` → `WaveScheduler.Start` → `WaitForCompletion`，把 Wave 产生的 FlowEvent 流式回放 | P0 |
| AC4 | `IntentFast` 保持 `FastPath.Run` 不变 | P0 |
| AC5 | 新增 `command_handler.go` + `orchestrate_path.go`，与 `orchestrator.go` 同 package（`coordinator`），便于选项注入和单测 mock | P0 |
| AC6 | 现有 3 个 P0 单测（`TestSessionOrchestrator_ProcessMessage_Command` / `_Skip` / `_FastPath`）中 Command 与 Orchestrate 断言需更新以反映新行为；Skip 与 FastPath 断言保持 | P0 |
| AC7 | `intent_kind` metric 仍按 4 类记，且每类绑定**真实**的执行路径（`path=fast|command|orchestrate|skip`）| P0 |
| AC8 | `process_message_test.go` 新增 3 个 P0 测试：Command → CLICommands.Handle（mock）/ Orchestrate → WaveScheduler.Start（mock）/ Skip 仍 close channel | P0 |
| AC9 | `go vet ./...` 0 错误；`go test -race ./internal/layers/orchestration/coordinator/...` 全部 PASS | P0 |
| AC10 | `internal/lint/layer/d2_d3_ban_test.go` 不回归（仍 ≤ 4 whitelist entries）| P1 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `coordinator/decomposer.go`（D7-S5-A02）已实现且 5s timeout；`orchestration/wave/scheduler.go` `Start` + `WaitForCompletion` 已实现；`workmodel/cli_commands.go::Handle` 已实现；`workmodel/plan_mode.go::Enter` + `Approve` 已实现 |
| 依赖 | `bootstrap/wire_coordinator.go` 已 wiring PlanCLICommands、TaskDecomposer、WaveScheduler |
| 约束 | 不引入新 import（`coordinator` 包已有所有依赖） |
| 约束 | 不动 `classifier.go`（识别逻辑正确，只是不被执行）|
| 约束 | 不动 `IntentClassification` 类型 |
| 约束 | 不动 `FastPath`（Fast 路径继续用） |
| 约束 | 不动 `coordinator/orchestrator.go::ProcessMessage` 的外部签名（`<-chan *contracts.EngineEvent, error`）|
| 约束 | 不动 `bootstrap/`（注入点不变，dispatch 行为变化对 bootstrap 透明）|
| 约束 | 不动 `coordinator/metrics` 计数器 key（避免回填旧 metric）|

## 5. 变更范围

### 新增
- `internal/layers/orchestration/coordinator/command_handler.go` — IntentCommand 显式 dispatch
- `internal/layers/orchestration/coordinator/orchestrate_path.go` — IntentOrchestrate SynthesizeTaskGraph → WaveScheduler
- `internal/layers/orchestration/coordinator/command_handler_test.go` — 3 P0 AC
- `internal/layers/orchestration/coordinator/orchestrate_path_test.go` — 3 P0 AC
- `openspec/changes/devrix-d7-orthogonal-intent-paths/specs/d7-orchestration/spec.md` — Gherkin Scenarios

### 修改
- `internal/layers/orchestration/coordinator/orchestrator.go` — 改 3 个 case 调独立函数（`handleCommand` / `orchestrate` / `handleCommand`）
- `internal/layers/orchestration/coordinator/orchestrator_test.go` — 更新 Command / Orchestrate 断言
- `openspec/specs/d7-orchestration/spec.md` — Cross-Domain Contracts + Revision History
- `openspec/specs/d7-orchestration/a-registry.md` — D7-S2-A01 标注 ✅ v1.1.0（按路径分发，3 真实链）

### 不变更
- `internal/layers/orchestration/coordinator/classifier.go`
- `internal/layers/orchestration/coordinator/classifier_fallback.go`
- `internal/layers/orchestration/coordinator/fastpath.go`
- `internal/layers/orchestration/coordinator/types.go`（`IntentKind` 枚举保持）
- `internal/layers/orchestration/coordinator/shadow_classifier.go`
- `internal/layers/orchestration/coordinator/interrupt.go`
- `internal/layers/orchestration/coordinator/spans.go` / `tracing.go`
- `internal/bootstrap/wire_coordinator.go`
- `internal/layers/orchestration/coordinator/decomposer.go`（只调用，不改）
- `internal/layers/orchestration/coordinator/executor.go`（只调用，不改）
- `internal/layers/orchestration/coordinator/workmodel.go`（LocalWorkModel 已实现）

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| `/plan approve` 现在经 fastPath.Run → 改走 `PlanCLICommands.Handle` → `PlanMode.Approve` 行为变化 | 用户可见的 plan approve 输出格式可能不同 | 单测覆盖 `PlanCLICommands.Handle` 入口；不修改 `PlanMode.Approve` 本身，只改 dispatch 路径 |
| `/plan <goal>` 改走 `PlanMode.Enter` + `Execute` 后产生异步 LLM 调用 | Fast path 同步 P99 ≤2ms 指标可能受影响（Command 路径无该 SLO） | 文档明确：FastPath ≤2ms 仅约束 `IntentFast`；Command 路径不进入 SLO 比较 |
| Orchestrate 路径调 `WaveScheduler.Start` + `WaitForCompletion`，单步消息也会创建 Wave | 资源开销大于 FastPath（即使最终只调度 1 个 TaskNode） | 单 TaskNode 的 Wave 不创建新资源（仍走 5-slot 池）；`WaitForCompletion` 阻塞直到该 TaskNode terminal |
| `TestSessionOrchestrator_ProcessMessage_Command` 断言 "D2 call = 1" 失败 | 测试需要更新 | 同步更新测试断言：command path 不再调 D2 |
| `TestSessionOrchestrator_AntiFabrication_NoSyntheticProgress` 用 "do something complex"（当前 IntentOrchestrate → fastPath）| 测试需要更新 | 同步更新测试：orchestrate path 走 Wave 异步流，需要在 fakeWaveScheduler 注入完整事件流 |
| `TestSessionOrchestrator_FastPath_NoWaveScheduled` 仍正确 | 保持 | FastPath 路径不变，测试不变 |
| bootstrap 注入 PlanCLICommands / TaskDecomposer / WaveScheduler 的顺序或 nil 检查 | orchestrator 调用 nil 方法会 panic | 在 NewSessionOrchestrator 加 nil guard；测试用 fake 注入；现有 `bootstrap/wire_coordinator.go` 已 wired（DM-020 v1.0 closure）|
| `coordinator` 包大小（已是 24 文件）继续增长 | 文件粒度问题 | 新增文件保持 < 200 行；遵循 `coding.md` §9 单文件 ≤800 行 |
