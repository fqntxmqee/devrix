---
demand-id: DM-20260617-003
title: 修复 D7 turn 路径下多轮会话历史丢失（P0）
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-17
---

# D7 Turn History Persist — 多轮对话上下文丢失修复

## 1. 背景

2026-06-16 上线 `devrix-d7-loop-first-routing`（DM-20260616-013）后，D7 Turn 上移（DM-20260617-002 中的 DM-020 D-c+d+e）将 LLM 多轮对话编排从 D2 ContextEngine 抽到 D7 `DefaultOrchestrator`。`loop_first` 路由下，`SessionOrchestrator.ProcessMessage` 调用 `FastPath.Run`，再经 `turnOrchExecutor.RunQueryLoop` 桥到 `turn.DefaultOrchestrator.RunTurn`。

完成本轮 LLM ↔ Tool 后，D7 通过 `SessionPersister.PersistTurn(ctx, PersistRequest)` 期望把这一轮 `Messages`（user + assistant + tool_calls + tool_results）写回 D2 ContextEngine 的 `SessionContext.Messages`，下一轮 `ContextPreparer.Prepare` 再从中读出喂给 LLM。

但当前实现 `internal/bootstrap/turn_adapter.go:154-161` 的 `PersistTurn` 是 stub：

```go
func (a *contextEngineAdapter) PersistTurn(ctx context.Context, req turn.PersistRequest) error {
    if ce, ok := a.engine.(*contextengine.ContextEngine); ok {
        _, err := ce.ExportSessionSnapshot(req.SessionID)
        return err
    }
    return nil
}
```

`ExportSessionSnapshot` 只序列化「D2 内存里现有的 SessionContext」（永远是空的），`req.Messages`（本轮的完整转写）被完全丢弃。

## 2. 问题陈述

| 现象 | 根因 | 影响 |
|------|------|------|
| 飞书同 session 连续发消息，LLM 不记得前几轮 | `PersistTurn` 没把 `req.Messages` append 到 D2 sc | 所有 `loop_first` 路由的 session 无历史 |
| LLM 每次只看到 system + 当前 user message | `Prepare` 总是从空 sc 读 Messages | 任何工具调用上下文、多轮指令、问答引用全部失效 |
| SessionStore 的 `ContextSnapshot` 也是空的 | 上面 stub 永远不会写入新消息 | 即使重启也是无状态 |

**Timeline 验证：** `journalctl` / `devrix.log` 2026-06-16 19:18 首次出现 `routing_mode=loop_first` 日志。该时刻之后所有 Feishu session（sess_1781674641805_7000 等）均无历史。属于 DM-20260616-013 引入的设计 gap，**非 PR #60（diagnostic-tools-wiring）引入**。

## 3. 验收标准

| ID | 标准 | 优先级 | 验证方式 |
|----|------|--------|----------|
| AC1 | D7 turn 路径下，连续 N (N≥3) 轮同 session，LLM 第 N 轮能引用第 1 轮的内容 | P0 | 飞书 IM E2E：先发 "记住数字 42"，再发 "我刚才让你记的数字是几？" |
| AC2 | `turn_adapter.PersistTurn` 实现 ≥ 1 个单测覆盖 (a) Messages append、(b) TrimMessages 触发、(c) 空 session 初始化 | P0 | `go test ./internal/bootstrap -run TestPersistTurn -race` |
| AC3 | SessionStore 中 `ContextSnapshot` 字段在 LLM 回合结束后被持久化（≥ 1 个集成测试覆盖） | P0 | `tests/integration/turn_history_persist_test.go` |
| AC4 | CompressHint 触发时，压缩目标覆盖到历史，不仅仅是当前轮 | P1 | 单测：超 4000 token 历史触发压缩 |
| AC5 | 旧 `rule_orchestrate` 路由不受影响（行为不变） | P1 | E2E 回归：`routing_mode=rule_orchestrate` 多轮正常 |
| AC6 | D7 Turn 在 PersistTurn 失败时不 hang，标记 session 错误并继续（不阻塞飞书回包） | P1 | 单测：mock engine persist error → 不阻塞后续 turn |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 上游 | DM-20260616-013 `devrix-d7-loop-first-routing`（v1.0，已合并 master） |
| 上游 | DM-20260617-002 `devrix-diagnostic-tools-wiring`（v1.0，已合并 master，与本 bug 无关） |
| 上游 | D2 ContextEngine 已有 API：`SessionContext(sid)`, `Memory().Get/AppendFullMessage/TrimMessages/PersistSnapshot` |
| 约束 | 文件规模 < 800 行；函数 < 50 行；不可变性：sc 必须通过 With* / Append 方法变更 |
| 约束 | `PersistTurn` 失败不能阻塞飞书回包（IM 实时性） |
| 约束 | 单测覆盖该方法 100%（P0） |

## 5. 变更范围

### 5.1 新增
- `internal/bootstrap/turn_adapter_test.go`：`PersistTurn` 单测（AC2）
- `tests/integration/turn_history_persist_test.go`：D7 → D2 集成测试（AC3）

### 5.2 修改
- `internal/bootstrap/turn_adapter.go:154-161`：实现 `PersistTurn` 真正把 `req.Messages` 追加到 D2 `SessionContext.Messages`，调用 `TrimMessages` 后再 `ExportSessionSnapshot`
- `openspec/specs/d7-orchestration/t-registry.md`：新增 P0 测试点 `D7-S5-A01-T01` 覆盖本需求

### 5.3 不变更
- `turn.SessionPersister` / `PersistRequest` 接口（D7 已定义正确）
- `DefaultOrchestrator.RunTurn`（PREPARE/LLM/PERSIST state machine 已正确）
- `FastPath.Run` / `OrchestratePath.Run`（router 层不需要感知）
- Feishu / IM 适配器
- `rule_orchestrate` 路由路径

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| D2 内存 SessionContext 锁竞争（D7 写 vs D7 读 vs D1 Snapshot 读） | 中 | 用 D2 既有 `Memory().AppendFullMessage`（已含 RWMutex） |
| 重复消息（Pre-existing messages + new messages overlap） | 中 | `TrimMessages` 用 dedup key（msg ID）去重；若未提供，则按 `AppendFullMessage` 自带语义 |
| PersistTurn 慢导致 IM 超时 | 低 | 同步写 D2 内存是 O(N) 微秒级；`ExportSessionSnapshot` 已是异步给 SessionStore |
| 已上线的 `loop_first` session 历史数据缺失（重启不会恢复） | 低 | 这是预期内修复行为，不做数据回填（飞书 session 可重发） |
| TrimMessages 把刚 append 的核心 system prompt 干掉 | 低 | Trim 策略只看 `len(tokens) > budget`，system prompt 单独计数；参考 `contextengine/memory_trim.go` 既有逻辑 |

## 7. 关联

- **Spec SoT:** `openspec/specs/d7-orchestration/spec.md`
- **D2 API:** `openspec/specs/d2-context-engine/spec.md`（SessionContext 规范）
- **Routing Config:** `devrix.yaml` `d7.orchestrator.routing_mode`（默认 `loop_first`，修复后保持）
