# Design: D7 Loop-First 路由

**Change ID:** `devrix-d7-loop-first-routing`
**Demand ID:** `DM-20260616-002`

---

## 1. 设计目标

将 Devrix harness 路由从 **「ingress 规则分叉」** 迁移到 **「单 Turn Loop + Tool 门控」**，对齐 Clawcode QueryEngine 模型，同时 **复用** DM-20260615-004 已落地的 OrchestratePath / CommandHandler / TurnOrchestrator。

## 2. 路由决策对比

### 2.1 现行（rule_orchestrate）

```
ClassifyIntent
  ├─ Skip
  ├─ Command (100)
  ├─ Fast (95 | 70) ── threshold<90? ──► Orchestrate
  └─ Orchestrate (60)
```

### 2.2 目标（loop_first）

```
ClassifyIntent
  ├─ Skip
  ├─ Command (100)
  └─ Turn (default, confidence=100, reason="loop_first_default")

Turn.RunTurn
  └─ LLM → tool_use?
        ├─ enter_plan_mode → PlanMode.Enter / Execute
        ├─ delegate_wave   → OrchestratePath.Run (async goroutine → events 写入 turn out channel)
        ├─ delegate_agent  → D4 fork / SubAgent
        └─ (none)          → text → complete
```

## 3. 组件变更

### 3.1 Classifier（`coordinator/classifier.go`）

新增配置 `RoutingMode`:

```go
type RoutingMode string

const (
    RoutingModeLoopFirst      RoutingMode = "loop_first"
    RoutingModeRuleOrchestrate RoutingMode = "rule_orchestrate" // legacy
)
```

`loop_first` 行为：

1. 保留 empty / command 规则
2. 删除「短单行 → confidence 70」作为 Orchestrate 降级输入
3. 非 Skip/Command → 返回 `{Kind: IntentTurn, Confidence: 100}`（新增 alias 或复用 `IntentFast` 语义为 Turn）

> **兼容策略：** 复用 `IntentFast` enum 值，metrics label 改为 `path=turn`；文档标注 `IntentFast` ≡ Turn loop。

`rule_orchestrate`：保持现有 classifier + threshold 行为（L5-06 回归）。

### 3.2 Orchestrator（`coordinator/orchestrator.go`）

```go
switch intent.Kind {
case IntentSkip: ...
case IntentCommand: ...
case IntentFast: // Turn loop
    ch, err = o.fastPath.Run(sessionCtx, req, turnSystemPrompt(o.cfg))
default:
    // loop_first 不应到达；rule_orchestrate legacy
    ch, err = o.orchestratePath.Run(...)
}
```

`turnSystemPrompt` 注入 Clawcode 风格 guidance（精简版）：

- 问候/短问答：**直接回复**，不调用 delegate_wave
- 多步/并行/跨文件：考虑 delegate_wave 或 enter_plan_mode
- 用户已给出明确实现细节：直接执行

### 3.3 Turn Tool Surface（新：`turn/orchestration_tools.go`）

注册到 `OrchestratorDeps.Tools`（bootstrap 接线）：

| Tool name | Input | Handler |
|---|---|---|
| `enter_plan_mode` | `{ goal: string }` | `PlanMode.Enter` + 返回 plan 摘要 tool_result |
| `delegate_wave` | `{ goal: string }` | 调 `OrchestratePath.Run`，将 channel 事件 **转发** 到当前 Turn out channel |
| `delegate_agent` | `{ directive, agent_type? }` | 现有 D4 delegate adapter |

**delegate_wave 转发伪码：**

```go
func (h *OrchestrationToolHandler) DelegateWave(ctx context.Context, goal string, out chan<- *EngineEvent) error {
    ch, err := h.orchestrate.Run(ctx, ProcessRequest{...}, IntentClassification{})
    if err != nil { return err }
    for ev := range ch {
        select {
        case out <- ev:
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return nil
}
```

Turn loop 在 tool round 内阻塞直到 wave 完成（与 Clawcode AgentTool 同步语义一致）。

### 3.4 单路径投递

| 位置 | 变更 |
|---|---|
| `fastpath.go` | 删除 sink mirror；`return out, nil` |
| `orchestrate_path.go` `emit()` | `_ = sink`；不 Publish |
| `agent_route.go` | `hasActiveProcess` 时跳过 Agent sink |
| `wire_coordinator.go` | 文档注释：sink 仅 out-of-band |

**Invariant（写入 spec）：**

> 同一 Turn 的 streaming EngineEvent MUST NOT 同时经 ProcessMessage channel 与 PublishEngineEvent 投递。

### 3.5 Config（`shared/config/coordinator.go`）

```yaml
coordinator:
  routing_mode: loop_first  # loop_first | rule_orchestrate
```

默认 `loop_first`。`fast_path_threshold` 仅在 legacy 模式读取。

### 3.6 Observability

| Metric / Span | 变更 |
|---|---|
| `orchestration.route` | `turn` / `command` / `skip`；legacy 保留 `orchestrate` |
| `orchestration.tool.delegate_wave` | 新 counter |
| Classify span attrs | `routing_mode` label |

## 4. Clawcode ↔ Devrix 映射表

| Clawcode 机制 | Devrix Loop-First |
|---|---|
| `query.ts` while loop | `turn/orchestrator.go` RunTurn |
| Tool dispatch map | `turn` ToolRoundExecutor + D2 tools |
| EnterPlanModeTool | `enter_plan_mode` → PlanMode |
| AgentTool | `delegate_agent` + Wave SubAgent runners |
| Coordinator mode | **Out of scope**（后续 change） |
| `--bare` / SIMPLE | 类比 `CLAUDE_CODE_SIMPLE` 缩小 tool 面（已有 toolpolicy） |
| BriefTool / IM | D1 conclusion + Feishu adapter（不变） |
| 无 ingress classifier | Classifier 仅 Skip/Command |

## 5. 迁移与回滚

1. **Phase 1** 合入：默认 `loop_first` + 单投递修复（可独立验收 L5-01/03/04）
2. **Phase 2** 合入：orchestration tools（L5-02/05）
3. **回滚**：配置切 `rule_orchestrate`，无需 revert 代码
4. **Deprecation**：`IntentOrchestrate` ingress 路由标记 deprecated；v2.0 删除

## 6. 测试策略

| L5 ID | 测试类型 | 位置 |
|---|---|---|
| L5-01 | integration + manual 飞书 | `tests/integration/d7/d7_loop_first_greeting_test.go` |
| L5-02 | integration stub LLM | 同上 |
| L5-03 | unit | `command_handler_test.go` |
| L5-04 | unit | `orchestrator_test.go` + `capture/coordinator_integration_test.go` |
| L5-05 | unit | `turn/orchestration_tools_test.go` |
| L5-06 | unit | `classifier_test.go` with legacy mode |

## 7. 文件清单（预估）

| 操作 | 路径 |
|---|---|
| Modify | `coordinator/classifier.go`, `config.go`, `orchestrator.go` |
| Modify | `fastpath.go`, `orchestrate_path.go` |
| Modify | `capture/agent_route.go` |
| Add | `turn/orchestration_tools.go`, `*_test.go` |
| Modify | `bootstrap/wire_coordinator.go`, `shared/config/coordinator.go` |
| Modify | `openspec/specs/d7-orchestration/spec.md` (post-S5) |
