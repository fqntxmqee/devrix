# Design: D7 ItemPipelineRunner emit hook

## Emit Chain Topology

```
SessionOrchestrator
  └─> session_turn_loop.go goroutine
        ├─> emit (session-level: SessionProcess + IntentClassify + TurnRun)
        └─> itemPipeline.Emit  (per-WorkItem level)
              └─> DefaultWorkItemExecutor.Emit  (LLM↔Tool iter level)
                    ├─> streamLLM → emit text/thinking/tool_call per chunk
                    └─> stepOneIter → emit tool_result with Name lookup
```

## Field Wiring

1. **DefaultWorkItemExecutor.Emit** (worker package):
   ```go
   type DefaultWorkItemExecutor struct {
       // ... existing fields
       Emit func(*contracts.EngineEvent)
   }
   ```
   - Called via `e.emit(ev)` helper which nil-checks.

2. **ItemPipelineRunner.Emit** (item_pipeline.go):
   ```go
   type ItemPipelineRunner struct {
       // ... existing fields
       Emit func(*contracts.EngineEvent)
   }
   ```
   - In `Run`, after Executor is created/retrieved:
     ```go
     if exec, ok := r.Executor.(*DefaultWorkItemExecutor); ok && exec.Emit == nil {
         exec.Emit = r.Emit
     }
     ```

3. **session_turn_loop.go wrapper**:
   ```go
   emitFn := func(ev *contracts.EngineEvent) {
       if ev != nil {
           ev.SessionID = sessionID
       }
       emit(ev)  // session-level emit
   }
   o.itemPipeline.Emit = emitFn
   ```

## Why This Works

- **Wave path 已有 emit**：OrchestratePath.Run 通过 subagent.streamEmit 把事件送 session-level emit
- **ItemPipelineRunner path 现在也走同一个 emit**：只是之前没接线
- **Nil-safe bridge**：Executor.Emit 未设时 nil bridge 不爆，与现状一致
- **Goroutine wrapper**：在 worker goroutine 内调 emit，确保所有事件带 SessionID

## ToolResult Name Lookup

`ToolResult` 结构只有 `ToolCallID / Output / Error`，**没有 Name 字段**。要在 `tool_result` 事件带 Name，必须从 `llmgateway.ToolCall[].Name` 反查：

```go
func nameOf(callID string, calls []llmgateway.ToolCall) string {
    for _, c := range calls {
        if c.ID == callID {
            return c.Name
        }
    }
    return ""
}
```

## Coverage 配套修复

PR #254/255/257 加 inner observability span 时（3 ops: D7_Worktree_Op / D7_SubWorktree_Run / D7_SubTurn_Iteration），D5 coverage 注册表遗漏更新：

- `coverage/registry_test.go` expected 列表没加 → registry size 84 vs 81
- `telemetry/names.go` LayerAndComponent 没加前缀 → fallback "context/devrix"

修复后 component 选取：
- D7_Worktree_Op + D7_SubWorktree_Run → `worktree`（D7-S1 task tree）
- D7_SubTurn_Iteration → `executor`（D7-S5 ReAct loop）

后续加 span 必须同步 2 处 D5 注册表。