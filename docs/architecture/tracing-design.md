# Jaeger/OTLP 链路追踪埋点设计方案

> 基于 D5 可观测性层自定义 OpenTelemetry 兼容 Tracer，输出 OTLP JSON 至 Jaeger/Tempo。

## 1. 当前状态评估

### 已埋点的领域（现有 36 个操作名）


| 层                | 操作名                                                                                                              | 粒度  |
| ---------------- | ---------------------------------------------------------------------------------------------------------------- | --- |
| D1 Gateway       | `gateway.session.*`, `gateway.store.*`, `gateway.permission.check`, `gateway.message.receive`                    | 中   |
| D1 Adapter       | `adapter.message.receive`, `adapter.cli.send`, `adapter.feishu.outbound`                                         | 粗   |
| D2 ContextEngine | `context.process`, `context.snapshot.load`, `context.longterm.*`, `context.compression.run`, `context.harness.*` | 中-细 |
| D3 LLMGateway    | `llm.stream`, `llm.provider.route`, `llm.circuit_breaker`, `llm.retry`, `llm.adapter.stream`                     | 细   |
| D4 MultiAgent    | `agent.run`, `agent.fork`, `agent.join`, `agent.terminate`, `agent.state.transition`, `agent.tool.call`          | 中   |


### 关键缺失（导致无法快速定位问题的根因）

1. **工具调用无追踪** — `toolrunner.PluginRunner.Execute()` 和 `query.ToolExecutor.Execute()` 完全没有 span，无法看到每个工具的执行耗时、入参和出参
2. **QueryLoop 回合无追踪** — `loop.go` 的每次 LLM call + tool round 没有独立 span，无法区分是 LLM 慢还是工具执行慢
3. **任务探索/规划无追踪** — `PlanAgent.Plan()`, `PlanMode.Enter/Execute/Approve` 无 span
4. **压缩管道步骤无细粒度追踪** — 只有一个大 span `context.compression.run`，7 个子步骤无法独立观察
5. **编排层（Orchestration）无追踪** — WaveScheduler、TaskGraph、Delegate、FlowHub 均无 span
6. **任务管理无追踪** — `TaskManager` CRUD 操作无 span

---

## 2. 新增操作名规范

命名格式：`{domain}.{module}.{action}`，符合现有约定。

### 2.1 QueryLoop 域（D2-S10）


| 操作名                   | 位置                      | 父子关系                 | 关键属性                                                                                    |
| --------------------- | ----------------------- | -------------------- | --------------------------------------------------------------------------------------- |
| `query.loop.run`      | `query/loop.go` `Run()` | 父: `context.process` | `session.id`, `max_turns`, `result.turn_count`, `result.total_tokens`                   |
| `query.loop.turn`     | `query/loop.go` for 循环内 | 父: `query.loop.run`  | `turn.number`, `turn.tool_count`, `turn.has_fallback`                                   |
| `query.loop.llm.call` | `query/loop.go` LLM 调用处 | 父: `query.loop.turn` | `llm.model`, `llm.prompt_tokens`, `llm.completion_tokens`, `llm.latency_ms`, `gen_ai.`* |


### 2.2 工具执行域（D2-S5）


| 操作名                       | 位置                                             | 父子关系                     | 关键属性                                                                                                        |
| ------------------------- | ---------------------------------------------- | ------------------------ | ----------------------------------------------------------------------------------------------------------- |
| `tool.execute.single`     | `query/adapters.go` `toolExecutor.Execute()`   | 父: `query.loop.turn`     | `tool.name`, `tool.input`(截断 500B), `tool.risk_level`, `tool.output_size`, `tool.duration_ms`, `tool.error` |
| `tool.execute.batch`      | `query/streaming_executor.go` `ExecuteBatch()` | 父: `query.loop.turn`     | `tool.count`, `tool.names`, `tool.concurrent`                                                               |
| `tool.execute.permission` | `query/adapters.go` `permChecker.Request()`    | 父: `tool.execute.single` | `tool.name`, `permission.granted`                                                                           |


### 2.3 任务探索/规划域（D2-S8）


| 操作名                      | 位置                               | 父子关系                 | 关键属性                                                                                               |
| ------------------------ | -------------------------------- | -------------------- | -------------------------------------------------------------------------------------------------- |
| `task.plan.generate`     | `tasks/plan_agent.go` `Plan()`   | 父: `context.process` | `task.user_goal`(截断), `task.tool_count`, `task.result_count`, `task.critical_files_count`, `error` |
| `task.plan_mode.enter`   | `tasks/plan_mode.go` `Enter()`   | 父: `context.process` | `plan_mode.state`                                                                                  |
| `task.plan_mode.execute` | `tasks/plan_mode.go` `Execute()` | 父: `context.process` | `plan_mode.state`, `plan_mode.result_tasks`, `plan_mode.duration_ms`                               |
| `task.plan_mode.approve` | `tasks/plan_mode.go` `Approve()` | 父: `context.process` | `plan_mode.task_count`                                                                             |
| `task.plan_mode.reject`  | `tasks/plan_mode.go` `Reject()`  | 父: `context.process` | —                                                                                                  |
| `task.manager.create`    | `tasks/task_manager.go`          | 父: `query.loop.turn` | `task.id`, `task.subject`, `task.status`                                                           |
| `task.manager.update`    | `tasks/task_manager.go`          | 父: `query.loop.turn` | `task.id`, `task.status`                                                                           |
| `task.manager.list`      | `tasks/task_manager.go`          | 父: `query.loop.turn` | `task.count`                                                                                       |


### 2.4 压缩管道域（D2-S2）


| 操作名                               | 位置                             | 父子关系                         | 关键属性                                                                              |
| --------------------------------- | ------------------------------ | ---------------------------- | --------------------------------------------------------------------------------- |
| `context.compression.step.{name}` | `compression/pipeline.go` 各步骤  | 父: `context.compression.run` | `step.name`, `messages.before`, `messages.after`, `tokens_before`, `tokens_after` |
| `context.compression.check`       | `engine.go` `shouldCompress()` | 父: `context.process`         | `messages.count`, `budget.target`, `should_compress`                              |


### 2.5 编排域（D6 Orchestration）


| 操作名                                | 位置                                      | 父子关系                                  | 关键属性                                                                               |
| ---------------------------------- | --------------------------------------- | ------------------------------------- | ---------------------------------------------------------------------------------- |
| `orchestration.wave.schedule`      | `orchestration/wave/scheduler.go`       | 父: `context.process`                  | `wave.task_count`, `wave.worker_count`                                             |
| `orchestration.wave.task.execute`  | `orchestration/wave/scheduler.go`       | 父: `orchestration.wave.schedule`      | `task.id`, `task.title`, `task.worker_type`, `task.depends_on`, `task.duration_ms` |
| `orchestration.wave.worker.run`    | `orchestration/wave/pool.go`            | 父: `orchestration.wave.task.execute`  | `worker.type`, `worker.slot`                                                       |
| `orchestration.flow.event.publish` | `orchestration/flow/hub.go` `Publish()` | 父: 按上下文                               | `event.type`, `event.session_id`                                                   |
| `orchestration.workplan.record`    | `orchestration/workplan/service.go`     | 父: `orchestration.flow.event.publish` | `workplan.event_type`                                                              |


### 2.6 Delegate 域（D4）


| 操作名                           | 位置                                                | 父子关系                     | 关键属性                                                                                |
| ----------------------------- | ------------------------------------------------- | ------------------------ | ----------------------------------------------------------------------------------- |
| `agent.delegate.sync`         | `multiagent/delegate/service.go` `DelegateSync()` | 父: `agent.run`           | `delegate.mode`, `delegate.worker_type`, `delegate.duration_ms`, `delegate.success` |
| `agent.delegate.subagent.run` | `multiagent/delegate/service.go`                  | 父: `agent.delegate.sync` | `subagent.id`, `subagent.mode`                                                      |


### 2.7 记忆域（D2-S6）


| 操作名                    | 位置                                | 父子关系                 | 关键属性                                                             |
| ---------------------- | --------------------------------- | -------------------- | ---------------------------------------------------------------- |
| `context.memory.load`  | `engine.go` `memory.LoadOrInit()` | 父: `context.process` | `memory.had_snapshot`, `memory.message_count`, `memory.restored` |
| `context.memory.store` | `engine.go` 持久化步骤                 | 父: `context.process` | `memory.session_id`, `memory.message_count`                      |


---

## 3. 分层 Span 树结构（核心链路）

以下是完整请求的预期 Span 树结构：

```
gateway.message.receive                          ← D1 入口
├── gateway.permission.check
├── gateway.session.lifecycle
│   └── gateway.session.get
├── context.process                              ← D2 主流程
│   ├── context.memory.load
│   ├── context.system_prompt.load
│   ├── context.snapshot.load
│   ├── context.longterm.recall
│   ├── context.compression.check
│   │
│   ├── task.plan.generate                       ← [新增] 任务规划
│   │   └── (LLM call within PlanAgent)
│   ├── task.plan_mode.enter
│   ├── task.plan_mode.execute
│   │   └── task.plan.generate
│   │       └── (LLM call)
│   ├── task.plan_mode.approve
│   │
│   ├── query.loop.run                           ← [新增] QueryLoop
│   │   ├── query.loop.turn #1                   ← [新增] 每回合
│   │   │   ├── context.compression.run
│   │   │   │   ├── context.compression.step.tool_result_budget  ← [新增]
│   │   │   │   ├── context.compression.step.snip                ← [新增]
│   │   │   │   ├── context.compression.step.microcompact        ← [新增]
│   │   │   │   └── ...
│   │   │   ├── query.loop.llm.call              ← [新增] LLM 调用
│   │   │   │   ├── llm.provider.route
│   │   │   │   ├── llm.stream
│   │   │   │   │   └── llm.adapter.stream
│   │   │   │   └── llm.retry (on error)
│   │   │   │
│   │   │   ├── tool.execute.single:bash         ← [新增] 工具执行
│   │   │   │   ├── tool.execute.permission
│   │   │   │   └── (bash 命令执行时间)
│   │   │   ├── tool.execute.single:read_file
│   │   │   │   └── tool.execute.permission
│   │   │   │
│   │   │   ├── task.manager.create              ← [新增] 任务操作
│   │   │   ├── task.manager.update
│   │   │   │
│   │   │   └── agent.tool.call                  ← 已有（Agent 工具桥接）
│   │   │       ├── agent.delegate.sync           ← [新增] Delegate
│   │   │       │   └── agent.delegate.subagent.run
│   │   │       └── orchestration.wave.schedule   ← [新增] Wave
│   │   │           ├── orchestration.wave.task.execute
│   │   │           │   └── orchestration.wave.worker.run
│   │   │           └── orchestration.wave.task.execute
│   │   │
│   │   └── query.loop.turn #2
│   │       ├── query.loop.llm.call
│   │       └── tool.execute.single:write_file
│   │
│   └── context.memory.store
│
├── gateway.store.update
└── gateway.engine_event.handle
```

---

## 4. 关键 Span 属性规范

### 4.1 工具调用 span 属性（核心调试能力）

```go
// tool.execute.single 必须携带的属性
tracer.Attribute{Key: "tool.name",        Value: toolName}
tracer.Attribute{Key: "tool.input",       Value: truncate(input, 500)}  // 截断避免 span 过大
tracer.Attribute{Key: "tool.risk_level",  Value: riskLevel}
tracer.Attribute{Key: "tool.output_size", Value: fmt.Sprintf("%d", len(output))}
tracer.Attribute{Key: "tool.duration_ms", Value: fmt.Sprintf("%d", durationMs)}
// 出错时记录
tracer.Attribute{Key: "tool.error",       Value: errMsg}
// 状态码
span.SetStatus(tracer.StatusOk / tracer.StatusError)
```

### 4.2 QueryLoop 回合 span 属性

```go
// query.loop.turn
tracer.Attribute{Key: "turn.number",        Value: fmt.Sprintf("%d", turn)}
tracer.Attribute{Key: "turn.tool_count",    Value: fmt.Sprintf("%d", len(toolCalls))}
tracer.Attribute{Key: "turn.tool_names",    Value: strings.Join(toolNames, ",")}
tracer.Attribute{Key: "turn.has_fallback",  Value: "true"}  // 仅当触发了 fallback

// query.loop.llm.call
tracer.Attribute{Key: "llm.model",               Value: model}
tracer.Attribute{Key: "llm.prompt_tokens",       Value: fmt.Sprintf("%d", promptTokens)}
tracer.Attribute{Key: "llm.completion_tokens",   Value: fmt.Sprintf("%d", completionTokens)}
tracer.Attribute{Key: "llm.latency_ms",          Value: fmt.Sprintf("%d", latencyMs)}
```

### 4.3 任务规划 span 属性

```go
// task.plan.generate
tracer.Attribute{Key: "task.user_goal",           Value: truncate(userGoal, 200)}
tracer.Attribute{Key: "task.tool_count",          Value: fmt.Sprintf("%d", len(tools))}
tracer.Attribute{Key: "task.result_count",        Value: fmt.Sprintf("%d", len(result.Tasks))}
tracer.Attribute{Key: "task.critical_files_count",Value: fmt.Sprintf("%d", len(result.CriticalFiles))}
```

### 4.4 压缩步骤 span 属性

```go
// context.compression.step.{name}
tracer.Attribute{Key: "step.name",          Value: stepName}
tracer.Attribute{Key: "messages.before",    Value: fmt.Sprintf("%d", before)}
tracer.Attribute{Key: "messages.after",     Value: fmt.Sprintf("%d", after)}
tracer.Attribute{Key: "tokens_saved",       Value: fmt.Sprintf("%d", before - after)}
```

---

## 5. Layer/Component 映射扩展

在 `telemetry/names.go` 的 `LayerAndComponent()` 中新增：

```go
case strings.HasPrefix(operation, "query."):
    return LayerContext, "query_loop"
case strings.HasPrefix(operation, "tool.execute."):
    return LayerContext, "tool_runner"
case strings.HasPrefix(operation, "tool."):
    return LayerContext, "tool_runner"
case strings.HasPrefix(operation, "task.plan"):
    return LayerContext, "plan_agent"
case strings.HasPrefix(operation, "task.plan_mode"):
    return LayerContext, "plan_mode"
case strings.HasPrefix(operation, "task.manager"):
    return LayerContext, "task_manager"
case strings.HasPrefix(operation, "task."):
    return LayerContext, "task_manager"
case strings.HasPrefix(operation, "orchestration."):
    return LayerOrchestration, "orchestrator"
case strings.HasPrefix(operation, "context.memory."):
    return LayerContext, "context_engine"
```

需要新增 Layer 常量：

```go
const LayerOrchestration = "orchestration"
```

---

## 6. 实现策略

### 6.1 不侵入工具执行器本身（方案 A，推荐）

> 在 `query/adapters.go` 的 `toolExecutor.Execute()` 中插入 span，而不是修改每个 `PluginRunner`。

**理由**：所有工具调用最终都经过该适配器，一处埋点覆盖所有工具。

```go
func (e *toolExecutor) Execute(ctx context.Context, call ToolCall) (string, string, error) {
    ctx, span := startToolSpan(ctx, telemetry.OpToolExecuteSingle, call.Name, call.Input, e.toolsReg.RiskLevel(call.Name))
    if span != nil {
        defer func() {
            span.SetAttributes(tracer.Attribute{Key: "tool.duration_ms", Value: ...})
            span.End()
        }()
    }
    // ... existing logic ...
}
```

### 6.2 QueryLoop 回合埋点（方案 B）

在 `loop.go` 的 `Run()` 方法中：

- 进入 for 循环前创建 `query.loop.run` span
- 每回合开始时创建 `query.loop.turn` span
- LLM 调用处创建 `query.loop.llm.call` span

### 6.3 添加 ObsBridge 到需要的新组件

QueryLoop 目前不持有 `obsBridge`，需要从上层注入。方式：

- 在 `Loop` 结构体中增加 `Observability *observability.Bridge` 字段
- 在 `bootstrap/context_engine.go` 装配时注入

### 6.4 压缩步骤埋点

Pipeline 已有 `StepObserver` 接口，可以：

- 新增一个 `tracingStepObserver` 实现 `StepObserver`
- 在 `OnStep()` 中创建/结束对应步骤的 span
- 将 `tracingStepObserver` 在 `newPipelineStepObserver()` 中注入

### 6.5 编排层埋点

WaveScheduler、FlowHub、DelegateService 需要：

- 接受 `*observability.Bridge` 注入
- 使用 `startSpan()` 辅助模式（与 `engine.go` 一致）

---

## 7. 注册表更新

所有新增操作名需注册到 `coverage/registry.go` 的 `AllOperations()`：

```go
// QueryLoop
{Name: "query.loop.run", Layer: "context", Component: "query_loop", SinceVersion: "2.1.0", Instrumented: true},
{Name: "query.loop.turn", Layer: "context", Component: "query_loop", SinceVersion: "2.1.0", Instrumented: true},
{Name: "query.loop.llm.call", Layer: "context", Component: "query_loop", SinceVersion: "2.1.0", Instrumented: true},

// Tool Execution
{Name: "tool.execute.single", Layer: "context", Component: "tool_runner", SinceVersion: "2.1.0", Instrumented: true},
{Name: "tool.execute.batch", Layer: "context", Component: "tool_runner", SinceVersion: "2.1.0", Instrumented: true},
{Name: "tool.execute.permission", Layer: "context", Component: "tool_runner", SinceVersion: "2.1.0", Instrumented: true},

// Task / Plan
{Name: "task.plan.generate", Layer: "context", Component: "plan_agent", SinceVersion: "2.1.0", Instrumented: true},
// ... etc
```

---

## 8. 分阶段实施建议

### Phase 1（高优先级 — 解决"无法定位问题"的核心痛点）

- `tool.execute.single` — 工具调用入参/出参/耗时
- `query.loop.turn` — 每回合独立追踪
- `query.loop.llm.call` — LLM 调用耗时/Token 拆分

### Phase 2（中优先级 — 补齐任务链路）

- `task.plan.generate` — PlanAgent 规划链路
- `task.plan_mode.*` — PlanMode 状态流转
- `task.manager.*` — Task CRUD 操作

### Phase 3（低优先级 — 完善可观测性）

- `context.compression.step.*` — 压缩管道分步骤
- `orchestration.*` — 编排层
- `agent.delegate.*` — Delegate 链路
- `context.memory.*` — 记忆操作

---

## 9. Jaeger 查询示例

### 定位"慢工具调用"

```
operation_name = "tool.execute.single" AND tool.duration_ms > 10000
```

### 定位"LLM 响应超时"

```
operation_name = "query.loop.llm.call" AND llm.latency_ms > 30000
```

### 定位"规划阶段失败"

```
operation_name = "task.plan.generate" AND status = error
```

### 查看完整请求的调用树

```
operation_name = "context.process" AND session.id = "xxx"
```

### 定位"压缩过度"场景

```
operation_name = "context.compression.step.snip" AND tokens_saved > 5000
```

