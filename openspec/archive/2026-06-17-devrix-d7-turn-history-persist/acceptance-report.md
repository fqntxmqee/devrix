# Acceptance Report: devrix-d7-turn-history-persist

**Change ID:** devrix-d7-turn-history-persist
**Demand ID:** DM-20260617-003
**Status:** ACCEPTED (S5 验收通过, S6 归档)
**PR:** [#62](https://github.com/fqntxmqee/devrix/pull/62)
**Generated:** 2026-06-17

---

## 1. 背景

DM-20260616-013 `devrix-d7-loop-first-routing` 将 D7 Turn 编排上移（DM-020 D-c+d+e）后, `turn.DefaultOrchestrator.RunTurn` 通过 `SessionPersister.PersistTurn` 期望把本轮 `Messages` 写回 D2 内存, 但 `internal/bootstrap/turn_adapter.go:PersistTurn` 是 stub (仅调 `ExportSessionSnapshot`, 不写 `req.Messages`), 导致飞书 IM 多轮上下文全部丢失 (P0 bug).

**根因** (`internal/bootstrap/turn_adapter.go:155-161` stub):
```go
func (a *contextEngineAdapter) PersistTurn(ctx context.Context, req turn.PersistRequest) error {
    if ce, ok := a.engine.(*contextengine.ContextEngine); ok {
        _, err := ce.ExportSessionSnapshot(req.SessionID)
        return err
    }
    return nil
}
```

`req.Messages` 字段被完全忽略. `ExportSessionSnapshot` 序列化的是 D2 内存里空的 `SessionContext.Messages` (因为没人写过), 所以 SessionStore 持久化的 snapshot 也是空.

**修复** (commit `133ea2b`):
1. 新增 `ContextEngine.AppendAndTrimMessages(sessionID, msgs)` 公开 API, lazy-init 不存在的 session, 调用 `Memory().AppendFullMessage` + `Memory().TrimMessages`
2. `turn_adapter.PersistTurn` 改为调 `AppendAndTrimMessages`, 错误显式传播

---

## 2. AC 验收 (4/4 PASS)

### AC1 — 飞书 IM 同 session 3 轮历史引用 — PASS

| 步骤 | 用户操作 | 期望 | 实测 |
|------|---------|------|------|
| 1 | 发 "记住数字 42" | LLM 回 "好的, 已记住 42" | ✓ |
| 2 | 发 "我刚才让你记的数字是几?" | LLM 回 "42" | ✓ |
| 3 | 发 "再记住一个颜色: 蓝色" | LLM 回 "已记 42 + 蓝色" | ✓ |
| 4 | 发 "我有两个秘密, 分别是什么?" | LLM 回 "42 和蓝色" | ✓ |

用户在 PR #62 验收后确认 (飞书真机 E2E 通过).

### AC2 — `go test -race ./...` 全绿 — PASS

- `internal/layers/contextengine/engine_persist_bridge_test.go` — 5 单测 (empty/existing/fresh/trim/race-safety) 全绿
- `internal/bootstrap/turn_adapter_persist_test.go` — 4 单测 (writes-to-D2/full-round/nil-engine/append-error) 全绿
- `tests/integration/d7/turn_history_persist_test.go` — 1 集成测试 (3-turn E2E) 全绿
- 覆盖率: `AppendAndTrimMessages` 92.3%, `PersistTurn` 75%

PR #62 + PR #64 合并前 `go test -race ./...` 100% 绿, `go vet ./...` 0 output.

### AC3 — D7 ↔ D2 接口边界保留 — PASS

- `SessionPersister` 接口签名不变 (commit `133ea2b` 仅替换实现, 不改接口)
- D7 不直接调 `engine.SessionContext` 或 `Memory().AppendFullMessage` (D7 ↔ D2 解耦保留)
- turn_adapter 仍持有 `*ContextEngine` 接口, 不持有 `*types.SessionContext` (不可变性约束保留)

### AC4 — SessionStore 持久化恢复 — PASS

- 修复前: `session.ContextSnapshot` = `serialize(empty sc)` = `[]byte{}` (0 bytes)
- 修复后: 第三次 PersistTurn 后 `os.Stat snapshot` > 0 bytes
- Session 重启时 `g.syncSnapshotFromEngine` 从 D2 读最新 sc (非空), 喂给下一轮 LLM ✓

---

## 3. 关键文件变更 (5 文件, +310 行)

| 文件 | 操作 | 行数 | 说明 |
|------|------|------|------|
| `internal/layers/contextengine/engine.go` | 新增 `AppendAndTrimMessages` | +25 | lazy-init + append + trim |
| `internal/bootstrap/turn_adapter.go` | 改 `PersistTurn` 实现 | -3 / +5 | 从 stub → AppendAndTrimMessages |
| `internal/layers/contextengine/engine_persist_bridge_test.go` | 新增 | +120 | 5 单测 |
| `internal/bootstrap/turn_adapter_persist_test.go` | 新增 | +80 | 4 单测 |
| `tests/integration/d7/turn_history_persist_test.go` | 新增 | +60 | 3-turn E2E |

---

## 4. P0 T 点覆盖 (4/4 IMPLEMENTED)

| T 点 | 描述 | 位置 | 状态 |
|------|------|------|------|
| **D2-S3-A02-T01** | AppendAndTrimMessages 写入 D2 内存 + budget 裁剪 | `engine_persist_bridge_test.go::Test{EmptyMessages,FreshSession,ExistingSession,TrimTriggered,RaceSafety}` | **IMPLEMENTED** |
| **D2-S3-A02-T02** | AppendAndTrimMessages lazy-init 不存在的 sid | `engine_persist_bridge_test.go::TestAppendAndTrimMessages_FreshSession` | **IMPLEMENTED** |
| **D7-S5-A04-T01** | turn_adapter.PersistTurn 提交 req.Messages 到 D2 内存 | `turn_adapter_persist_test.go::TestPersistTurn_{WritesMessagesToD2Memory,FullRound,NilEngine,AppendError}` | **IMPLEMENTED** |
| **D7-S5-A04-T02** | 三轮同 session 连续 PersistTurn → Prepare 返回全历史 | `tests/integration/d7/turn_history_persist_test.go::TestTurnHistory_ThreeTurns` | **IMPLEMENTED** |

---

## 5. 质量基线

- [x] 文件 < 800 行: `engine.go` 增量 25 行, `turn_adapter.go` 净增 2 行, 测试文件均 < 200 行
- [x] 函数 < 50 行: `AppendAndTrimMessages` 25 行, `PersistTurn` 5 行
- [x] 不可变性: turn_adapter 不持有 `*types.SessionContext`, 仅通过 `*ContextEngine` API 写入
- [x] 错误处理: wrapped error 透传 (`turn adapter: persist: %w`), 上层 D7 `DefaultOrchestrator.runLoop` 不阻塞 IM 回包
- [x] 无 hardcoded secrets
- [x] 无 mutation: sc 修改走 `Memory().AppendFullMessage` 内部 RWMutex 串行化

---

## 6. Hotfix 路径说明

本 change 走 hotfix 路径, **S1-S3 ceremony 跳过** (用户指令 2026-06-17 13:50 飞书验收发现后要求: "先修复, 不要走完整研发流程, 我验收后再确认"). 实现 + 测试在 PR #62 一次性合入, S4-S6 文档工作 (proposal / design / demand / tasks / acceptance-report / index / t-registry) 在 PR #62 squash merge 时一并落库 (commit `bbb7178`). PR #64 引用本 change 的 sub-commit 重新触达但代码已就位.

**Follow-up**: 无. 修复已闭环.

---

## 7. 验收签字

- 实施: 2026-06-17 (PR #62 squash merge `6e913c4`)
- 文档归档: 2026-06-17 (PR #62 内含 commit `bbb7178`)
- 用户验收: 2026-06-17 飞书 IM E2E 通过
- 引用 (跨 PR): PR #64 包含本 change 的 sub-commit
- S6 归档: 2026-06-17 (本次 follow-up PR)
