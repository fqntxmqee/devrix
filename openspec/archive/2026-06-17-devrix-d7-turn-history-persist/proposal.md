# Proposal: 修复 D7 turn 路径下多轮会话历史丢失

**Change ID:** devrix-d7-turn-history-persist
**Demand ID:** DM-20260617-003
**Status:** S6_Archived (2026-06-17, hotfix 路径 — S1-S3 ceremony 跳过, S4-S6 文档 + S6 归档在 follow-up PR 闭环)
**Priority:** P0
**PR:** [#62](https://github.com/fqntxmqee/devrix/pull/62)
**DSAFT:** D7-S5（D7 Session Persistence 场景，原 D7-S5 占位；本 change 落地）

---

## 1. Background

DM-20260616-013 `devrix-d7-loop-first-routing` 将 D7 Turn 编排上移（DM-020 D-c+d+e）后：

```
Feishu IM msg
  → D1 SessionStore.getOrCreate
  → D7 SessionOrchestrator.ProcessMessage
  → intent classification → IntentFast (loop_first 强制 confidence=100)
  → FastPath.Run → turnOrchExecutor.RunQueryLoop
  → turn.DefaultOrchestrator.RunTurn  (PREPARE → LLM ↔ TOOL_ROUND → PERSIST)
  → turnOrchExecutor returns EngineEvent chan
  → SessionOrchestrator returns
  → D1 persistSessionAfterProcess → D1 SessionStore.Update
```

D7 `DefaultOrchestrator.RunTurn` 在 LLM 回合结束后调用 `SessionPersister.PersistTurn`，但 `internal/bootstrap/turn_adapter.go` 提供的 `PersistTurn` 实现是 stub：仅调 `ExportSessionSnapshot`，不写新 `Messages`。

这导致 D2 ContextEngine 内存中的 `SessionContext.Messages` 永远是空，`Prepare` 永远读不到历史。

## 2. Problem Statement

| 维度 | 当前 | 影响 |
|------|------|------|
| 多轮上下文 | ❌ 全部丢失 | LLM 无法引用前轮事实/指令/工具结果 |
| SessionStore 持久化 | ❌ ContextSnapshot 为空（bytes 0） | 重启无状态 |
| Debug 难度 | 高 | 无任何 session 内 turn 数 metric（与 §D5-S24-A02-T04 legacy counter 同性质盲区） |
| 用户体验 | ❌ 飞书多轮对话等于单轮对话 | P0 |

## 3. Proposed Solution

### 3.1 方案 A（推荐）：补齐 `PersistTurn` 实现

改动量 **~30 行**（turn_adapter.go）+ **1 个单测文件** + **1 个集成测试**。

```go
// internal/bootstrap/turn_adapter.go (proposed)
func (a *contextEngineAdapter) PersistTurn(ctx context.Context, req turn.PersistRequest) error {
    ce, ok := a.engine.(*contextengine.ContextEngine)
    if !ok {
        return nil
    }
    sc, ok := ce.SessionContext(req.SessionID)
    if !ok || sc == nil {
        // Fresh session: initialize via D2 Init
        if err := ce.InitSession(req.SessionID); err != nil {
            return fmt.Errorf("turn persist: init session: %w", err)
        }
        sc, _ = ce.SessionContext(req.SessionID)
    }
    if sc == nil {
        return fmt.Errorf("turn persist: sc still nil after init")
    }
    // Append this turn's full transcript (user + assistant + tool_calls/results)
    if err := ce.Memory().AppendFullMessage(sc, req.Messages...); err != nil {
        return fmt.Errorf("turn persist: append: %w", err)
    }
    // Trim to budget (idempotent, no-op if under budget)
    if err := ce.Memory().TrimMessages(sc); err != nil {
        return fmt.Errorf("turn persist: trim: %w", err)
    }
    // Snapshot to D1 SessionStore (existing path)
    if _, err := ce.ExportSessionSnapshot(req.SessionID); err != nil {
        return fmt.Errorf("turn persist: snapshot: %w", err)
    }
    return nil
}
```

**为何选这个：**
1. D2 已有 `Memory().AppendFullMessage(sc, msgs...)` API（见 `internal/layers/contextengine/memory.go`），复用即可
2. D2 已有 `TrimMessages` 自动按 token budget 压缩（与 §CompressHint 逻辑协同）
3. `ExportSessionSnapshot` 已正确桥到 D1 SessionStore，**只是缺数据**，不需要改
4. 改一处，约束最小，符合「bug fix 不需要 cleanup」原则

### 3.2 方案 B（不推荐）：在 D7 `DefaultOrchestrator.RunTurn` 内直接写 D2

绕过 `SessionPersister` 接口，d7 turn 直接调 `engine.SessionContext` + `Memory().AppendFullMessage`。

- ❌ 破坏 D7 ↔ D2 解耦（DM-020 D-c 设计原则）
- ❌ 未来如果 D2 替换实现（Redis / Postgres），D7 要重写
- ✅ 改动量略小（~20 行）

### 3.3 方案 C（备选，不推荐）：回滚 `routing_mode=loop_first` 到 `rule_orchestrate`

`rule_orchestrate` 走 `OrchestratePath.Run`，不经过 D7 Turn 上移架构，没有这个 bug。

- ❌ 治标不治本（DM-020 设计目标丢失：D7 编排未独立）
- ❌ 已上线 1 天，部分 session 已习惯 `loop_first` 体验
- ✅ 1 行 config 改动（治急性）

### 3.4 决策

**选择方案 A。** 改动量与方案 B 相当，但保留 D7 ↔ D2 接口边界。方案 C 作为紧急 hotfix 备选。

## 4. Success Metrics

| Metric | Baseline | Target | 测量 |
|--------|----------|--------|------|
| 飞书同 session 第 N 轮 LLM 引用第 1 轮事实成功率 | 0% | 100% | 飞书 IM E2E（AC1） |
| `turn_adapter.PersistTurn` 单测覆盖率 | 0% | 100% | `go test -cover` |
| `go test -race` 全量绿 | ✓ | ✓ | CI |
| SessionStore.ContextSnapshot 长度（典型 session 第 5 轮后） | 0 bytes | > 0 bytes | `os.Stat` snapshot 文件 |
| D7 ↔ D2 接口耦合度 | 1 stub + 3 真实现 | 4 真实现 | grep 静态扫描 |

## 5. Implementation Plan

| Step | 任务 | 工时 | 交付物 |
|------|------|------|--------|
| 1 | `feat/devrix-d7-turn-history-persist` 分支创建 | 1 min | git branch |
| 2 | 改 `turn_adapter.go` 实现 `PersistTurn`（方案 A 伪代码落地） | 30 min | code diff |
| 3 | 写 `turn_adapter_test.go` 单测（覆盖 append / trim / 空 session） | 1 h | test diff |
| 4 | 写 `tests/integration/turn_history_persist_test.go` | 30 min | test diff |
| 5 | `go test -race ./...` + `go vet ./...` 全绿 | 15 min | CI green |
| 6 | 飞书 IM E2E：sess_xxx 连发 3 轮验证历史 | 30 min | AC1 证据 |
| 7 | PR 提交 + auto-merge + S6 归档 | 30 min | PR + archive |

**总计：约 3.5 小时**

## 6. Risks & Mitigations

| 风险 | 等级 | 缓解 |
|------|------|------|
| `AppendFullMessage` 与既有 sc.Messages 重复（如果 D2 已经在某路径里写过） | M | 单测覆盖（AC2.b）；AppendFullMessage 自身已含 dedup by ID 逻辑（查 `internal/layers/contextengine/memory.go`） |
| TrimMessages 把刚 Append 的核心消息 trim 掉 | L | Trim 策略看 token budget 是否超；新 Append 消息通常不会被 trim 除非真超长 |
| PersistTurn 抛错导致 D7 RunTurn 返回失败 → 飞书收不到回复 | M | `DefaultOrchestrator.RunTurn` 现有错误处理：PersistTurn 失败不阻塞 IM 回包（验 `orchestrator.go` line 175-185） |
| 并发：同一 session 两条消息几乎同时到（D1 不去重） | M | `Memory().AppendFullMessage` 内部用 RWMutex（`memory.go` 现有） |
| Feishu session 持久化的 ContextSnapshot 与 D2 内存不一致（重启时） | L | `ExportSessionSnapshot` 是同步路径，PersistTurn → Snapshot 同一调用链 |

## 7. Out of Scope

- 不补历史数据（2026-06-16 19:18 之后已发但丢失历史的 session，告知用户重发即可）
- 不改 D7 Turn State Machine（PREPARE/LLM/PERSIST）
- 不改 `SessionPersister` 接口
- 不改 routing config（保持 `loop_first` 为默认）
- 不动 DM-020 其他能力（DM-020 D-a/D-b/D-e 不在本 change 范围）

## 8. Open Questions

| Q | 状态 | 决策 |
|---|------|------|
| 是否需要新增 `turn_orchestrator_persist_error_total` metric？ | 待 S3 | S3 design.md 评估 |
| `AppendFullMessage` 现有 dedup key 是？ | 待 S3 核实 | S3 design.md 引用 memory.go 实际签名 |
| TrimMessages 对 tool_call 消息的处理策略？ | 待 S3 核实 | S3 design.md 引用 memory_trim.go |
