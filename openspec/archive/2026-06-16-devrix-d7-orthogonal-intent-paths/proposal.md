# Proposal: D7 Intent 路径正交化

**Change ID:** devrix-d7-orthogonal-intent-paths
**Demand ID:** DM-20260615-004
**Status:** S2_Design

## 1. Background

DM-020 (D7 Turn 编排上移) v1.0 closure (2026-06-15) 之后，D7 拥有 LLM 调用权与 D1→D7 总入口。Session Orchestrator 的 `ProcessMessage` 通过 `IntentClassifier` 识别 4 种 IntentKind，但 v1.0 closure 阶段**只落地 1 条执行链**（FastPath.Run）+ 3 种 system_prompt 前缀字符串。

`IntentCommand` 与 `IntentOrchestrate` 的"执行"是把字符串塞进 LLM 的 system_prompt，**让 LLM 自己去理解命令 / 拆解任务**。这是 v1.0 阶段的务实妥协（v1.0 closure 注释 line 222-237 明确说明），但本质是债，不是设计。

## 2. Problem Statement

### 2.1 核心问题
4 个 IntentKind switch case 收敛到 1 个执行器（`FastPath.Run`），差异只剩 system_prompt 前缀字符串。这违反"先正交再合并"的工程原则：

- **IntentKind 语义撒谎** — 宣称为"路由决策"实际只是"hint"
- **分类器价值被浪费** — 5 条规则识别对错都走同一条执行链
- **v1.1 迁移路径被堵** — 后续 PlanMode / WaveScheduler 真实落地时要在 fastPath.Run 内部塞 if-else
- **observability 失真** — intent_kind metric 4 类样本不可比

### 2.2 触发条件
- v1.1 已经 ready（PlanMode + SynthesizeTaskGraph + WaveScheduler + PlanCLICommands 都已 IMPLEMENTED）
- v1.0 closure 已发布，继续保留占位 = 把临时妥协固化成架构

## 3. Proposed Solution

### 3.1 方案 A：4 真实执行链（推荐）

按"先正交再合并"原则，让 4 个 IntentKind 对应 4 条独立执行链：

| Intent | 真实路径 | 代码位置 |
|--------|---------|---------|
| `IntentSkip` | 关闭空 channel | `orchestrator.go` 内联（保持） |
| `IntentCommand` | 显式 dispatch：`/task` → `CLICommands.Handle`；`/plan` → `PlanCLICommands.Handle` → `PlanMode.Enter/Approve/Reject` | 新增 `command_handler.go` |
| `IntentFast` | `FastPath.Run`（保持） | `fastpath.go`（保持） |
| `IntentOrchestrate` | `TaskDecomposer.SynthesizeTaskGraph` → `WaveScheduler.Start` → `WaitForCompletion` → 流式回放 FlowEvent | 新增 `orchestrate_path.go` |

**核心原则：** IntentKind 真正成为路由决策。分类器识别对了，对应路径就真的被走。

### 3.2 方案 B：在 FastPath.Run 内部做 hint prefix 分发（不推荐）

保留 1 个 `FastPath.Run`，但 `system_prompt` 不再是字符串前缀，而是结构化 `IntentHint{Command, Mode}` 字段，由 FastPath.Run 内部 switch 决定走 D2 / PlanMode / Wave。

**否决理由：** 1. 把债从 `orchestrator.go` 推到 `fastpath.go`，文件膨胀同样不可逆；2. FastPath 名字与"快速"语义绑定，挂载 Wave 异步路径会污染概念；3. AC4（FastPath 不变）也无法满足。

### 3.3 方案对比

| 维度 | A. 4 真实链 | B. FastPath 内分发 |
|------|------------|-------------------|
| 文件数 | +2 (command_handler / orchestrate_path) | 0 |
| fastpath.go 行数 | 75 不变 | 估计 200+ |
| 概念清晰度 | 高（4 路径 = 4 函数）| 中（FastPath 名实不符）|
| 可单测性 | 4 路径分别 mock | 1 个 switch 内部多分支 |
| 风险面 | 3 文件 | 1 文件（但容易失控）|

**选择: A。** 符合 DSAFT 活动边界（每个 A 独立可测）+ 防止 fastpath.go 膨胀。

## 4. Success Metrics

| 指标 | 基线 | 目标 | 测量方式 |
|------|------|------|----------|
| ProcessMessage switch case 收敛度 | 4 → 1（1 执行器）| 4 → 4（4 执行器）| `grep` / 代码审查 |
| IntentKind metric 样本可比性 | 不可比 | 可比 | 按 `path` 标签聚合 |
| `coordinator/orchestrator.go::ProcessMessage` 函数行数 | 30 | 30 | diff 校验 |
| `fastpath.go` 行数 | 75 | 75 | 不变 |
| 新增单测 P0 AC 覆盖 | 0 | 6 | test count |
| 回归测试 PASS | n/a | 100% | `go test -race` |

## 5. Implementation Plan

5 个 commit / 5 个文件改动：

1. **commit 1**：`command_handler.go` 新增 + `command_handler_test.go` 单测
2. **commit 2**：`orchestrate_path.go` 新增 + `orchestrate_path_test.go` 单测
3. **commit 3**：`orchestrator.go` 改 3 case 调独立函数（orchestrator_test.go 同步更新）
4. **commit 4**：spec.md / a-registry.md / layering.md 文档同步
5. **commit 5**：`go vet` + `go test -race` 全绿（实际是 commit 4 之后的验证，不开新 commit）

## 6. Risks & Mitigations

详见 `demand.md` §6。

关键缓解：
- **nil 注入 panic** → `NewSessionOrchestrator` 加 nil guard + 测试用 fake 注入
- **anti-fabrication 测试** → fakeWaveScheduler 注入完整事件流
- **/plan approve 输出格式变化** → 不修改 `PlanMode.Approve` 本身，只改 dispatch 路径

## 7. Out of Scope

- `classifier.go` / `classifier_fallback.go` 不动（识别逻辑正确）
- `IntentKind` 枚举不变
- `bootstrap/wire_coordinator.go` 不动（注入点已存在）
- `coordinator/metrics` 计数器 key 不变（避免回填旧 metric）
- PlanMode / WaveScheduler / TaskDecomposer 内部实现不动
- D2 / D3 / D4 域规范不动
- LLM Caller / Summarizer 拆面（DM-020 上一轮已闭合）不动
