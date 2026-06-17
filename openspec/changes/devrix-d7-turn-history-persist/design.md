# Design: D7 Turn History Persist — 实现细节

**Change ID:** devrix-d7-turn-history-persist
**Demand ID:** DM-20260617-003
**Status:** S3_Design
**Depends on:** DM-20260616-013 (loop_first), DM-20260617-002 (D7 turn 接线)
**Parent Proposal:** `proposal.md`

---

## 1. Root Cause Analysis

### 1.1 调用链定位

```
飞书 IM msg
  → D1: CommunicationGateway.RouteInbound         (capture/gateway.go:258)
  → D1: g.orchestrationEntry.ProcessMessage      (capture/gateway.go:300)
  → D7: SessionOrchestrator.ProcessMessage       (orchestration/coordinator/orchestrator.go:207)
  → FastPath.Run                                  (orchestration/coordinator/fastpath.go:41)
  → turnOrchExecutor.RunQueryLoop                (bootstrap/wire_coordinator.go:178)
  → D7: turn.DefaultOrchestrator.RunTurn         (orchestration/turn/orchestrator.go:67)
  → loop: LLM invoke (D3) + Tool execute (D2)   (orchestrator.go:138-260)
  → PersistTurn:                                  (orchestrator.go:217)
        o.persist.PersistTurn(ctx, PersistRequest{
            SessionID: req.SessionID,
            Messages:   messages,    ← 本轮完整转写（user + assistant + tool_calls/results）
            ...
        })
  → turn_adapter.contextEngineAdapter.PersistTurn  (bootstrap/turn_adapter.go:155) ← BUG
  → return event chan
  → D1: g.persistSessionAfterProcess(session)    (capture/gateway.go:353)
  → sessionStore.Update                           (capture/session_snapshot.go:25)
```

### 1.2 Bug 点定位

**`internal/bootstrap/turn_adapter.go:155-161`**:

```go
func (a *contextEngineAdapter) PersistTurn(ctx context.Context, req turn.PersistRequest) error {
    if ce, ok := a.engine.(*contextengine.ContextEngine); ok {
        _, err := ce.ExportSessionSnapshot(req.SessionID)
        return err
    }
    return nil
}
```

`req.Messages` 完全没被读。`ExportSessionSnapshot` 仅序列化 D2 内存里现有的 `SessionContext.Messages`（永远是空的，因为没人写过）。

### 1.3 连锁影响

| 下游 | 行为 | 触发条件 |
|------|------|---------|
| `turn_adapter.Prepare` (line 67-78) | 总是返回空 `Messages` | 每次进 `Prepare` 都从空 sc 读 |
| `DefaultOrchestrator.runLoop` line 127 | `messages := prepared.Messages + req.UserMessage` | `prepared.Messages == []` |
| LLM InvokeRequest | 只看到 system prompt + 当前 user message | 多轮上下文全部丢失 |
| `g.persistSessionAfterProcess` | `session.ContextSnapshot = serialize(empty sc) = []byte{}` | SessionStore 持久化也是空的 |
| Session 重启 | 无历史可恢复 | 用户体验降级 |

### 1.4 历史 Timeline

| 时间 | 事件 |
|------|------|
| 2026-06-16 19:18 | 首次 `routing_mode=loop_first` 日志（DM-020 D-c+d+e 上线） |
| 2026-06-17 | DM-20260617-002 (diagnostic-tools-wiring) PR #60 合并 — **与本 bug 无关** |
| 2026-06-17 13:50 | 用户飞书 3 轮连续消息，session `sess_1781674641805_7000` 无历史（本次 E2E 验收发现） |

## 2. Proposed Solution — 详细

### 2.1 新增 ContextEngine 公开方法

**文件：** `internal/layers/contextengine/engine.go`

```go
// AppendAndTrimMessages writes a batch of messages into the session context,
// then trims to the configured token budget. Safe to call for any session,
// including fresh ones (will be initialized on demand).
//
// DM-20260617-003 (d7-turn-history-persist): D7 SessionPersister.PersistTurn
// uses this to commit a turn's transcript back into D2 memory so the next
// Prepare call can read it.
func (e *ContextEngine) AppendAndTrimMessages(sessionID string, msgs []types.Message) error {
    if len(msgs) == 0 {
        return nil
    }
    sc, ok := e.memory.Get(sessionID)
    if !ok || sc == nil {
        // Fresh session from D7 path: lazy init via loadOrInitSession.
        // We need a session stub to loadOrInit — synthesize a minimal one.
        // D1 will refresh session.ContextSnapshot on next persistSessionAfterProcess.
        stub := &types.Session{
            SessionID: sessionID,
            // WorkDir/Model come from D1 session later, not required for persist path.
        }
        var initOK bool
        sc, initOK = e.loadOrInitSession(context.Background(), stub, nil)
        if !initOK || sc == nil {
            return fmt.Errorf("append+trim: cannot init session %s", sessionID)
        }
    }
    if err := e.memory.AppendFullMessage(sc, msgs...); err != nil {
        return fmt.Errorf("append+trim: append: %w", err)
    }
    if err := e.memory.TrimMessages(sc); err != nil {
        return fmt.Errorf("append+trim: trim: %w", err)
    }
    return nil
}
```

**设计选择：**
- 单一公开入口 `AppendAndTrimMessages`，内部按需 `loadOrInitSession`，避免暴露 `memory` 字段
- 不暴露 `*types.SessionContext` 写入（避免外部直接 mutate sc）
- 失败用 wrapped error，便于上层分类处理
- `loadOrInitSession` 在 D7 路径下会被调用 — 这是首次让 D7 路径"启动"D2 session（之前 stub ExportSessionSnapshot 会失败，但被吞掉了，看 §1.2 的 `_ = err` 风险）

### 2.2 修复 turn_adapter.PersistTurn

**文件：** `internal/bootstrap/turn_adapter.go`

```go
// PersistTurn implements turn.SessionPersister.
// DM-20260617-003 (d7-turn-history-persist): commit this turn's transcript
// into D2 memory so the next Prepare call can read it back.
func (a *contextEngineAdapter) PersistTurn(ctx context.Context, req turn.PersistRequest) error {
    if ce, ok := a.engine.(*contextengine.ContextEngine); ok {
        if err := ce.AppendAndTrimMessages(req.SessionID, req.Messages); err != nil {
            return fmt.Errorf("turn adapter: persist: %w", err)
        }
    }
    return nil
}
```

**变化：**
- 不再调 `ExportSessionSnapshot`（D1 gateway 后续会通过 `g.persistSessionAfterProcess` 自动完成）
- 直接调 `AppendAndTrimMessages` 写入 D2 内存
- 错误传播（之前 stub 吞错，现在显式返回）

### 2.3 数据流（修复后）

```
turn.DefaultOrchestrator.runLoop
  ↓ o.persist.PersistTurn(ctx, {Messages: messages})
contextEngineAdapter.PersistTurn
  ↓ ce.AppendAndTrimMessages(sid, messages)
  ├─ e.memory.Get(sid) → sc
  ├─ e.memory.AppendFullMessage(sc, messages...)  ← 写入 D2 内存
  └─ e.memory.TrimMessages(sc)                     ← 按 budget 裁剪
  ↓ return nil
turn.DefaultOrchestrator emits 'complete'
  ↓ return event chan
D7: SessionOrchestrator.ProcessMessage returns
  ↓
D1: g.persistSessionAfterProcess(session)
  ├─ g.syncSnapshotFromEngine(session)
  │   └─ snapshotExporter.ExportSessionSnapshot(sid)  ← 现在 sc 有内容，序列化非空
  │       └─ session.ContextSnapshot = 非空 bytes
  └─ g.sessionStore.Update(session)  ← 持久化到 SessionStore
```

下一轮（Turn N+1）：
```
D7 Prepare
  ↓ ce.SessionContext(sid) → sc (含 N 轮历史)
  └─ result.Messages = sc.Messages  ← LLM 看到完整历史 ✓
```

## 3. 接口与数据契约

### 3.1 ContextEngine 新增 API

| Method | 签名 | 行为 |
|--------|------|------|
| `AppendAndTrimMessages` | `(sessionID string, msgs []types.Message) error` | 把 msgs 写入 sid 对应的 sc，按 token budget 裁剪；sid 不存在则 lazy init |

### 3.2 PersistRequest 契约（不变）

参考 `internal/layers/orchestration/turn/contracts.go:66-72`：

```go
type PersistRequest struct {
    SessionID  string
    Messages   []types.Message  ← 至少 1 条（user + assistant + 可能 tool_calls/results）
    TurnCount  int
    Usage      llmgateway.TokenUsage
    FinalText  string
}
```

**约定：** Messages 是「本轮累计」的转写，包含 user → assistant → (tool_call + tool_result)*。orchestrator.go line 127-128 `messages := prepared.Messages + req.UserMessage` 起步，line 263-272 会把 tool 结果追加到 messages 末尾（在 tool round 后）。

### 3.3 不可变性约束

- `*types.SessionContext` 通过 `e.memory.AppendFullMessage` 内部修改（持有 RWMutex）
- 外部不能直接 mutate sc（`*types.SessionContext` 没有暴露 setter）
- turn_adapter 只持有 `*ContextEngine` 引用，不持有 sc 指针

## 4. 测试矩阵

### 4.1 单元测试 (P0)

**`internal/layers/contextengine/engine_persist_bridge_test.go`** (新增)：

| Test ID | 场景 | 预期 |
|---------|------|------|
| `TestAppendAndTrimMessages_EmptyMessages` | msgs=nil 或 len=0 | nil error，sc 不变 |
| `TestAppendAndTrimMessages_FreshSession` | sid 不存在（lazy init） | nil error，sc 创建并包含 msgs |
| `TestAppendAndTrimMessages_ExistingSession` | sid 已有 sc | 追加到 sc.Messages 末尾 |
| `TestAppendAndTrimMessages_TrimTriggered` | msgs + 已有 sc 总 token > budget | TrimMessages 触发，len(sc.Messages) < total |
| `TestAppendAndTrimMessages_AppendError` | mock memory.AppendFullMessage error | wrapped error 透传 |
| `TestAppendAndTrimMessages_RaceSafety` | 100 goroutine 并发 append | 最终 len(sc.Messages) == N*msgs_per_goroutine，无 panic |

### 4.2 turn_adapter 集成测试 (P0)

**`internal/bootstrap/turn_adapter_persist_test.go`** (新增)：

| Test ID | 场景 | 预期 |
|---------|------|------|
| `TestPersistTurn_WritesMessagesToD2Memory` | mock engine，req.Messages=[user, assistant] | engine.AppendAndTrimMessages 被以 sid+msgs 调用一次 |
| `TestPersistTurn_NilEngine` | a.engine 不是 *ContextEngine | nil error（不阻塞） |
| `TestPersistTurn_AppendError` | engine.AppendAndTrimMessages returns error | wrapped error 透传 |
| `TestPersistTurn_FullRound` | 真 ContextEngine 实例 | sc.Messages 长度从 0 增长到 N；SessionContext(sid) 能读回 |

### 4.3 端到端测试 (P1)

**`tests/integration/d7/turn_history_persist_test.go`** (新增)：

| Test ID | 场景 | 预期 |
|---------|------|------|
| `TestTurnHistory_ThreeTurns` | 同 session 三次 PersistTurn 调用（user1+asst1, user2+asst2, user3+asst3） | 第三次 Prepare 返回的 Messages 包含前三轮的 user+asst |

### 4.4 飞书 IM E2E (P0，AC1)

| 步骤 | 用户操作 | 期望 |
|------|---------|------|
| 1 | 发 "记住数字 42" | LLM 回 "好的，已记住 42" |
| 2 | 发 "我刚才让你记的数字是几？" | LLM 回 "42" |
| 3 | 发 "再记住一个颜色：蓝色" | LLM 回 "已记 42 + 蓝色" |
| 4 | 发 "我有两个秘密，分别是什么？" | LLM 回 "42 和蓝色" |

## 5. 关键文件变更

| 文件 | 操作 | 行数估算 |
|------|------|---------|
| `internal/layers/contextengine/engine.go` | 新增 `AppendAndTrimMessages` 方法 | +25 行 |
| `internal/bootstrap/turn_adapter.go` | 改 `PersistTurn` 实现 | -3 行 / +5 行 |
| `internal/layers/contextengine/engine_persist_bridge_test.go` | 新增 6 个单测 | +120 行 |
| `internal/bootstrap/turn_adapter_persist_test.go` | 新增 4 个单测 | +80 行 |
| `tests/integration/d7/turn_history_persist_test.go` | 新增 1 个集成测试 | +60 行 |
| `openspec/specs/d7-orchestration/t-registry.md` | 新增 `D7-S5-A01-T01` P0 T 点 | +10 行 |
| `openspec/specs/d2-context-engine/t-registry.md` | 新增 `D2-S5-A04-T01` 单测 T 点 | +10 行 |

**总计：~310 行新增/修改**

## 6. 回归风险评估

| 风险 | 等级 | 触发条件 | 缓解 |
|------|------|---------|------|
| `loadOrInitSession` 在 D7 路径被首次调用，D2 session 初始化语义与 D1 不同步 | M | 飞书第一次发消息 | 单测 `TestAppendAndTrimMessages_FreshSession` 验证；并加日志 `turn persist: lazy init session` |
| `loadOrInitSession` 内部读 prompt 文件（`e.prompt.Load(workDir)`），workDir 为空会怎样？ | L | D7 stub session WorkDir="" | 查 `loadOrInitSession` 行为；如依赖 WorkDir，改用 `e.cfg.SystemPrompt.Sources` 默认路径 |
| `TrimMessages` 把刚 Append 的 user 消息裁掉 | L | msgs 总长超 budget 且 user 是最后一条 | Trim 策略按 token count 而非时间裁剪；不丢最新 user；查 `prepare/memory/trim.go` 实际语义 |
| 并发：同 session 两条 IM 消息几乎同时到，两次 PersistTurn 并发 | L | 高频 IM 用户 | `Memory().AppendFullMessage` 内部 RWMutex 串行化 |
| Append 后 session.ContextSnapshot 没更新（D1 没感知） | L | 不在本 change 范围 | D1 `persistSessionAfterProcess` 已经在 ProcessMessage 返回后调，`syncSnapshotFromEngine` 会从 D2 读最新 sc |
| 老 session（2026-06-16 19:18 后历史丢失的）启动后无历史 | L | 用户期望恢复 | 这是预期修复行为，不做数据回填，飞书 session 重发即可 |

## 7. 性能预算

| 操作 | 当前 stub | 修复后 | 差异 |
|------|-----------|--------|------|
| `PersistTurn` 单次调用 | O(serialize empty sc) ~10µs | O(append + trim + serialize) ~1ms（msgs 长度 10） | +990µs |
| 内存 | 0 | +len(msgs)*sizeof(Message) ≈ +2KB | 极小 |
| IM 端到端延迟 | 0 影响 | +1ms（不可感知） | 无影响 |

## 8. Open Questions（已闭环）

| Q | 答案 | 备注 |
|---|------|------|
| D2 memory.AppendFullMessage dedup key？ | `Message.ID`（若无 ID，按 index） | 见 `internal/layers/contextengine/prepare/memory/*.go` |
| TrimMessages 对 tool_call 处理？ | 按 token budget 整体裁剪，不区分 role | 同上 |
| 是否需要新增 `turn_persist_error_total` metric？ | 否，超出本 change 范围，留 P2 backlog | S5 验收时记录 |
