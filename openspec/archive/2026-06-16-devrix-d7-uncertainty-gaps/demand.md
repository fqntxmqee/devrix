# Demand: D7 不确定性处理能力缺口修复

**Demand ID:** DM-20260616-001
**Phase:** S1 Demand
**Priority:** P0
**DSAFT Domain:** d7-orchestration

---

## 1. 背景

D7 编排层设计了一套五层不确定性管理体系：分类（确定性规则 + 置信度门控）→ 探索（PlanMode 只读 + 用户审批）→ 规划（LLM 优先 → 规则回退 → DAG 校验）→ 执行（连续调度 + WorkerPool + ConflictGuard + ContextPolicy）→ 反馈（FlowEvent 流式 + 3 级压缩降级 + 建议性 D6 校验）。

2026-06-16 架构审查发现 5 个关键缺口，其中部分缺口导致安全边界仅依赖 prompt 约束、配置项完全无运行时效力、以及默认路径开箱即坏。

## 2. 问题陈述

### Gap 1: PlanAgent 只读白名单缺少运行时门控 (Security)

**位置**: `workmodel/plan_agent.go:29-37`, `workmodel/plan_agent.go:181-200`

`PlanAgentReadOnlyTools` 白名单仅在 `buildPlanPrompt()` 中注入到 prompt 文本，靠 LLM 自觉遵守。代码中没有任何运行时拦截机制——如果 LLM 在 plan mode 中返回 `write`/`edit`/`bash` tool call，系统不会阻止。`PlanAgent.IsReadOnlyTool()` 方法存在但未被任何 tool 执行路径调用。

**影响**: 探索阶段的只读安全边界完全依赖 LLM 对齐，违背 defense-in-depth 原则。

### Gap 2: PlanModeApproveGate 是死配置 (Approval Flow)

**位置**: `coordinator/config.go:22,42,59,86-87`

`PlanModeApproveGate` 配置项完整定义了声明、默认值（true）、YAML 解析——但整个 `internal/layers/orchestration/` 目录中，除 config.go 自身外，**没有任何代码引用此字段**。PlanMode 的 `Approve()`/`Reject()` 方法存在但无调用方执行门控检查。

**影响**: 设计上要求 "Wave 触发需显式 Plan approve"，但实际运行中此门控完全不存在。

### Gap 3: ConflictGuard Allow()→Register() TOCTOU 窗口 (Concurrency)

**位置**: `wave/conflict.go:37-68`, `wave/scheduler.go` dispatch loop

`Allow()` 和 `Register()` 是两个独立调用，中间存在时间窗口。两个并发 goroutine 可同时通过 `Allow()` 检查，然后各自 `Register()`，导致互斥约束被绕过。

**影响**: 理论上两个写任务可能同时操作同一文件范围。窗口极小（微秒级）但存在。

### Gap 4: OrchestratePath FlowEvent sink 静默忽略 (Observability)

**位置**: `coordinator/orchestrate_path.go:216-217`

```go
func emit(ctx context.Context, sink EventPublisher, out chan<- *contracts.EngineEvent, ...) {
    _ = sink  // ← sink 被显式忽略
    ...
}
```

`emit()` 函数接收 `sink EventPublisher` 参数但不调用。OrchestratePath 中所有 Wave Worker 产生的进度事件（text/thinking/tool_use）只写入 `out` channel，不推送到 sink。这意味着 IM 进度卡片、WebSocket 通知等在编排路径上完全静默。

**影响**: 编排路径的用户体验退化——用户看不到任务执行进度，只知道最终完成或失败。

### Gap 5: 默认 PlanMode (nil LLM) 开箱即坏 (Availability)

**位置**: `coordinator/command_handler.go:168`, `workmodel/plan_agent.go:140-144`

生产代码中 PlanMode 以 `NewPlanMode(nil, nil)` 创建（nil LLM）。`Enter()` 检查 `planAgent == nil`（而非 LLM），因此Enter 成功；但 `Execute()` → `Plan()` 在 `plan_agent.go:140` 检测到 `a.llm == nil` 时立即返回 `ErrLLMNotConfigured`。用户执行 `/plan` 命令会进入 plan mode 但永远无法完成规划。

**影响**: `/plan` CLI 命令在所有默认部署中均不可用。测试代码中 `NewPlanMode(nil, nil)` 被 5 处调用。

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | PlanAgent tool call 执行前有运行时白名单校验，禁止 write/edit/bash/delete/mkdir/rm/mv/cp | P0 |
| AC2 | PlanModeApproveGate 配置项在 Wave 调度前产生实际门控效果（或移除此死配置并更新文档） | P0 |
| AC3 | ConflictGuard.Allow()+Register() 合并为原子操作或调度循环使用单 goroutine 消除竞态 | P1 |
| AC4 | OrchestratePath.emit() 将事件推送到 sink，使 IM/WebSocket 可接收编排进度 | P0 |
| AC5 | PlanMode 在 LLM 为 nil 时 Enter() 即返回明确错误，或提供合理的 fallback 行为 | P1 |
| AC6 | 移除或标记 Deprecated: LLMFallbackClassifier, ExecutorSelector（如确认无调用方） | P2 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | D2 ContextEngine tool execution pipeline（Gap 1 需拦截 tool call） |
| 依赖 | D1 CommunicationGateway EventPublisher 接口（Gap 4 需 sink 实现） |
| 约束 | 不得破坏现有 25 个 D7 集成测试 |
| 约束 | 不得改变 Public API（AllowedTools、IsReadOnlyTool 签名稳定） |
| 约束 | PlanAgentReadOnlyTools 常量保持向后兼容 |

## 5. 变更范围

### 修改

| 文件 | 变更 |
|------|------|
| `workmodel/plan_agent.go` | Plan() 增加运行时 tool call 白名单校验 |
| `workmodel/plan_mode.go` | Enter() 增加 LLM nil 检查 |
| `wave/conflict.go` | Allow+Register 合并为 AllowAndRegister 原子方法 |
| `wave/scheduler.go` | dispatch loop 使用原子 AllowAndRegister |
| `coordinator/orchestrate_path.go` | emit() 将事件推送到 sink |
| `coordinator/command_handler.go` | PlanMode 创建时传入有效 LLM 或明确报错 |

### 移除 / 标记废弃

| 文件 | 变更 |
|------|------|
| `coordinator/classifier_fallback.go` | 标记 Deprecated（LLMFallback 默认 false，无调用方） |
| `coordinator/executor.go` | 标记 Deprecated（WaveScheduler 直接使用 WorkerRunner） |
| `coordinator/config.go` | 移除 PlanModeApproveGate 或实现其门控逻辑 |

### 不变更

- `classifier.go` — RuleClassifier 逻辑正确，保持不变
- `decomposer.go` — LLM→rule 回退链路正确，保持不变
- `scheduler.go` dispatch loop 整体架构（20ms ticker + wakeupCh）保持不变
- `plan_agent_whitelist_test.go` — 现有测试不受影响

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| Gap 1 修复需接入 D2 tool pipeline | 中等 | 在 `PlanAgent` 内部增加拦截层，不改变 tool pipeline 接口 |
| Gap 3 原子化可能引入锁竞争 | 低 | Allow 已持有 mu.Lock，合并 Register 不增加额外锁操作 |
| Gap 4 sink 推送增加事件流量 | 低 | 事件已在 out channel 中传输，sink 是额外推送非替代 |
| Gap 2 实现门控可能改变用户流程 | 中等 | 方案 A：实现门控；方案 B：移除死配置。S3 设计阶段决定 |
