# Design: D7 不确定性处理能力缺口修复

**Change ID:** devrix-d7-uncertainty-gaps
**Demand ID:** DM-20260616-001
**阶段:** S3 Design
**版本:** v1.0
**关联:** `proposal.md`

---

## 1. 概述

本设计覆盖 5 个关键缺口的修复方案，按影响层次排列：安全（Gap 1）→ 可用性（Gap 5）→ 可观测性（Gap 4）→ 并发正确性（Gap 3）→ 配置清理（Gap 2）。

## 2. Gap 1: PlanAgent 运行时 tool call 门控

### 2.1 根因

`PlanAgentReadOnlyTools` 白名单在 `buildPlanPrompt()` 中仅作为 prompt 提示注入（`plan_agent.go:182-200`）。`IsReadOnlyTool()` 方法存在（`plan_agent.go:63-73`）但无任何 tool 执行管线调用。

### 2.2 方案

在 `PlanAgent` 中新增 `ValidateToolCall` 方法，由 tool 执行管线在调用前检查：

```go
// ValidateToolCall reports whether the named tool is allowed in PlanAgent's
// read-only sandbox. Returns an error describing the violation if denied.
func (a *PlanAgent) ValidateToolCall(name string) error {
    if a == nil {
        return nil // passthrough: no PlanAgent means no sandbox
    }
    if a.IsReadOnlyTool(name) {
        return nil
    }
    for _, fb := range PlanAgentForbiddenTools {
        if fb == name {
            return fmt.Errorf("tool %q is forbidden in plan mode (write/execute not allowed)", name)
        }
    }
    return fmt.Errorf("tool %q is not in the plan mode read-only whitelist", name)
}
```

**调用点**: `OrchestratePath.Run()` 或 `TurnOrchestrator` 在执行 tool call 前，当 task 的 worker_type 为 explore/plan 时，调用 `PlanAgent.ValidateToolCall()`。

**接口变更**: 新增 `ValidateToolCall(name string) error` 公开方法。`AllowedTools()` 和 `IsReadOnlyTool()` 保持不变。

### 2.3 数据流

```
LLM returns tool_call
  → TurnOrchestrator.ExecuteToolRound()
    → if task.ReadOnly:
        PlanAgent.ValidateToolCall(toolName)
          → 在白名单: 继续执行
          → 不在白名单: 返回 error，tool call 被拒绝
```

## 3. Gap 2: PlanModeApproveGate 死配置

### 3.1 根因

`PlanModeApproveGate` 配置项在 `config.go` 中完整定义但无任何运行时引用。PlanMode 的 approve/reject 流程通过 CLI 命令（`/plan approve`）显式触发，不依赖此配置。

### 3.2 方案（选择方案 A）

移除死配置：
1. 从 `Config` 结构体中移除 `PlanModeApproveGate` 字段
2. 从 `ConfigFile` 结构体中移除对应 YAML 字段
3. 从 `DefaultConfig()` 中移除默认值
4. 从 `ApplyConfigFile()` 中移除解析逻辑
5. 更新 `config.go` 注释

保留 PlanMode 的 `Approve()`/`Reject()` 方法——它们由 CLI 命令驱动，不需要配置开关。

## 4. Gap 3: ConflictGuard TOCTOU

### 4.1 根因

调度循环调用流程（`scheduler.go`）：
```
guard.Allow(candidate, running)  // 步骤 1: 检查
  → true
guard.Register(task)              // 步骤 2: 注册（非原子）
```

两个 goroutine 可同时通过步骤 1，然后各自执行步骤 2。

### 4.2 方案

新增 `AllowAndRegister` 原子方法：

```go
// AllowAndRegister atomically checks the candidate against running tasks
// and, if allowed, registers it. Returns true if the task was registered.
func (g *ConflictGuard) AllowAndRegister(candidate TaskNode, slotID SlotID, running []RunningTask) bool {
    if g == nil {
        return true
    }
    g.mu.Lock()
    defer g.mu.Unlock()

    effective := make([]RunningTask, 0, len(g.running)+len(running))
    for _, r := range g.running {
        effective = append(effective, r)
    }
    effective = append(effective, running...)

    for _, r := range effective {
        if conflictBetween(candidate, r.Node) {
            return false
        }
    }
    g.running[slotID] = RunningTask{Node: candidate, SlotID: slotID}
    return true
}
```

调度循环改为：
```go
if !guard.AllowAndRegister(node, slotID, guard.Running()) {
    continue // 冲突，跳过
}
```

### 4.3 兼容性

- 保留 `Allow(candidate, running) bool` — 仅用于测试和只读查询
- 保留 `Register(t RunningTask)` — 标记 Deprecated
- 调度循环迁移到 `AllowAndRegister`

## 5. Gap 4: OrchestratePath sink 推送

### 5.1 根因

`orchestrate_path.go:216-217`:
```go
func emit(ctx context.Context, sink EventPublisher, out chan<- *contracts.EngineEvent, ev *contracts.EngineEvent) {
    _ = sink  // 显式忽略
    ...
}
```

### 5.2 方案

```go
func emit(ctx context.Context, sink EventPublisher, out chan<- *contracts.EngineEvent, ev *contracts.EngineEvent) {
    // Push to sink for IM/WebSocket notifications.
    if sink != nil {
        _ = sink.Publish(ctx, ev) // fire-and-forget; best-effort delivery
    }
    select {
    case out <- ev:
    case <-ctx.Done():
    }
}
```

`EventPublisher` 接口需确认 `Publish(ctx, event) error` 方法签名。如果接口不同，适配。

## 6. Gap 5: PlanMode nil LLM

### 6.1 根因

`command_handler.go:168`: `workmodel.NewPlanMode(nil, nil)` 传入 nil LLM。
`plan_agent.go:140`: `a.llm == nil` → `ErrLLMNotConfigured`。
`plan_mode.go:68`: `Enter()` 只检查 `planAgent == nil`，不检查 `planAgent.llm == nil`。

### 6.2 方案

在 `PlanMode.Enter()` 中增加 LLM 检查：

```go
func (p *PlanMode) Enter(ctx context.Context, sessionID, userGoal string) error {
    if p.planAgent == nil || p.planAgent.llm == nil {
        return ErrLLMNotConfigured
    }
    // ... rest of Enter
}
```

同时更新 `command_handler.go:168` 使其通过 `llmgateway` 传入有效 LLM（与 D3 对齐），或保持 nil 并在 CLI 响应中返回明确错误消息。

**生产路径**: `NewPlanMode(llmCompleter, obsBridge)` — 需从 D7 bootstrap 传入有效 LLMCompleter。
**临时方案**: 在 command_handler 中捕获 `ErrLLMNotConfigured` 并返回用户友好的错误消息。

## 7. 死代码清理

### 7.1 LLMFallbackClassifier (`classifier_fallback.go`)

- 有完整实现和测试（`classifier_fallback_test.go`）
- 生产代码中无调用方（`LLMFallback` config 默认 false，无创建路径）
- **处理**: 文件顶部添加 `// Deprecated: LLM fallback classification is deferred to v1.1.`；保留代码和测试

### 7.2 ExecutorSelector (`executor.go`)

- `SelectExecutor`/`MatchExecutorByTaskType`/`CheckExecutorAvailability` 有实现和测试
- 实际 WaveScheduler 通过 `WorkerRunner` 接口直接分发，不经过 ExecutorSelector
- **处理**: 添加 Deprecated 标记；保留代码和测试

## 8. 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `workmodel/plan_agent.go` | 修改 | 新增 `ValidateToolCall()` 方法 |
| `workmodel/plan_agent_whitelist_test.go` | 修改 | 新增 `TestPlanAgent_ValidateToolCall_*` 测试 |
| `workmodel/plan_mode.go` | 修改 | `Enter()` 增加 LLM nil 检查 |
| `wave/conflict.go` | 修改 | 新增 `AllowAndRegister()` 方法 |
| `wave/scheduler.go` | 修改 | 调度循环使用 `AllowAndRegister` |
| `wave/conflict_test.go` | 修改 | 新增 `TestConflictGuard_AllowAndRegister_*` 测试 |
| `coordinator/orchestrate_path.go` | 修改 | `emit()` 调用 `sink.Publish()` |
| `coordinator/command_handler.go` | 修改 | PlanMode 错误处理或传入有效 LLM |
| `coordinator/config.go` | 修改 | 移除 `PlanModeApproveGate` |
| `coordinator/classifier_fallback.go` | 修改 | 添加 Deprecated 注释 |
| `coordinator/executor.go` | 修改 | 添加 Deprecated 注释 |

## 9. 统计

| 指标 | 值 |
|------|-----|
| 新增方法 | 2 (`ValidateToolCall`, `AllowAndRegister`) |
| 修改方法 | 2 (`Enter`, `emit`) |
| 移除配置项 | 1 (`PlanModeApproveGate`) |
| 标记 Deprecated | 2 (`LLMFallbackClassifier`, `ExecutorSelector`) |
| 新增 T 点 | 6 |
| 爆炸半径 | D7 orchestration 包内，不影响 D1-D6 |
