# D7 Orchestration Spec Delta — TurnState 串行化 + prior-output-summary 注入 (DM-20260628-003)

**Change ID:** devrix-d7-multiturn-session-state
**Demand ID:** DM-20260628-003
**Delta Type:** ADDED (v4.13.0 → v4.14.0)
**SOT:** `internal/layers/orchestration/sessionorchestrator/{turn_state,transcript_reader,orchestrator,session_turn_loop}.go`

---

## 1. 修改总览

| 内容 | 文件 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. `TurnState` struct + BeginTurn/EndTurn/WaitTurn API | `turn_state.go` (NEW) | NEW | 同 session_id turn 串行化门禁 |
| 2. `TurnInProgressError` 类型 + `errors.Is` 支持 | `turn_state.go` (NEW) | NEW | turn N+1 在 turn N 收尾前到 → 明确错误返回 |
| 3. `TranscriptReader` struct + ReadRecent/BuildPriorOutputSummary | `transcript_reader.go` (NEW) | NEW | 读最近 N 轮 finalText helper |
| 4. `WithPriorContextRounds(n int) OrchestratorOption` functional option | `orchestrator.go` | NEW | 0 关闭注入（默认，向后兼容）；>0 启用 |
| 5. `WithTranscriptDir(dir string) OrchestratorOption` functional option | `orchestrator.go` | NEW | 默认 `internal/layers/communication/capture/transcript`；空 = 默认 |
| 6. `SessionOrchestrator.turnState` + `transcriptReader` 字段 | `orchestrator.go` | NEW | `WithPriorContextRounds(n>0)` 时挂载 |
| 7. `ProcessMessage` 接入 WaitTurn + BeginTurn 门禁 | `orchestrator.go` | MODIFIED | 入口增加 turn 串行化校验 |
| 8. `ProcessMessage` 在 EnsureGoal 后注入 prior-output-summary | `orchestrator.go` | MODIFIED | directive 自带 prior context |
| 9. `RunSessionTurnLoop` defer 链加 EndTurn | `session_turn_loop.go` | MODIFIED | goroutine 退出时释放 turn slot |

---

## 2. TurnState 串行化契约（D7 ↔ D1）

```go
// D7 turn_state.go
type TurnState struct { ... }

func (ts *TurnState) BeginTurn(sessionID string) error  // 返回 TurnInProgressError
func (ts *TurnState) EndTurn(sessionID string)          // close handle.done
func (ts *TurnState) WaitTurn(ctx context.Context, sessionID string) error

type TurnInProgressError struct {
    SessionID      string
    SinceStartedAt time.Time
}

// D1 feishu.go
if errors.Is(err, sessionorchestrator.TurnInProgressError{}) {
    // 发送 "⏳ 上一条消息还在处理中，请稍候..." 卡片
}
```

---

## 3. TranscriptReader 契约（D7 ↔ D2 fs）

```go
// D7 transcript_reader.go
type TranscriptReader struct {
    dir       string  // 默认 internal/layers/communication/capture/transcript
    maxRounds int
}

// ReadRecent 读最近 n 条 kind=complete 的 Body 字段。
// transcript schema: {t, kind, role, body}; finalText 标记为 kind="complete"
// (capture/gateway.go:880 appendTranscriptEvent complete 分支)。
// 文件不存在返回 ([]string{}, nil) 不报错。
func (r *TranscriptReader) ReadRecent(ctx context.Context, sessionID string, n int) ([]string, error)

func (r *TranscriptReader) BuildPriorOutputSummary(texts []string) string
// 输出: <prior-output-summary>\n  [turn 1] xxx\n  [turn 2] yyy\n</prior-output-summary>
// 复用 D2 FoldAssistantOutput 标签语法（PR #149 iter3 验证 LLM 理解）
```

---

## 4. T 点增量（v4.13.0 → v4.14.0）

| T ID | 描述 | Status |
|------|------|--------|
| D7-S15-A55-T01 | `TurnState` struct + BeginTurn/EndTurn/WaitTurn + 并发安全（1000 goroutine stress） | PENDING (S4) |
| D7-S15-A55-T02 | `OrchestratorDeps.PriorContextRounds` + `TranscriptDir` 字段 + `SessionOrchestrator.turnState` 挂载 | PENDING (S4) |
| D7-S15-A56-T03 | `transcript_reader.go` 读最近 N 轮 finalText helper（filter kind=complete，Body 字段；复用 capture.transcript.Writer.LoadReader） | PENDING (S4) |
| D7-S15-A56-T04 | `ProcessMessage` 接入 WaitTurn + BeginTurn 门禁 + `TurnInProgressError` 定义 + prior-output-summary 注入 | PENDING (S4) |
| D7-S15-A58-T06 | feishu adapter 识别 `TurnInProgressError` + "⏳ 上一条还在处理中" 文案 | PENDING (S4) |
| D7-S15-A59-T07 | LP-5 e2e 集成测试：sess_e2e_multiturn_v1（断言 complete 时机 + prior context 注入） | PENDING (S4) |

6 个 T 点全部 P0。

---

## 5. 行为不变保证

- **PriorContextRounds = 0 时**（默认）：TurnState 不构造 + transcript_reader 不读 + EnsureGoal 不注入 → 完全等价 v4.13.0 行为
- **单 turn session（turn N=1）**：transcript 首次读为空 → 不注入 → 完全等价 v4.13.0 行为
- **CLI 适配器**：`errors.Is(err, TurnInProgressError)` 不识别，走通用 error 处理 → 保持兼容
- **processAutoClose**：channel close 已隐含 processAutoClose 触发顺序（PR #265 wiring point 不变）
- **complete 事件 emit 时机**：`session_turn_loop.go:186` 原位置不变，由 turn 串行化保证 turn N+1 不会与 turn N 撞车 → 用户感知层 complete 时机"自然修正"
- **PR #271 emit recover + exec.Emit 覆盖**：作为双层防护保留（即使 turn 串行化实现有 bug 漏过某些并发路径，PR #271 仍兜底 panic）

---

## 6. Span 增量（v4.13.0 → v4.14.0）

| Span | Op | 归属 |
|------|----|----|
| `turn_state.wait` | `d7.s15.turn_state.wait` | sessionSpan child |
| `turn_state.begin` | `d7.s15.turn_state.begin` | sessionSpan child |
| `transcript.read_recent` | `d7.s15.transcript.read_recent` | sessionSpan child |
| `prior_context.inject` | `d7.s15.prior_context.inject` | sessionSpan child |

新增 4 个 span，与 DM-20260626-001 "5 P0/P1 span 升格" 一致。

---

## 7. Out of Scope（本 Change 不做）

- transcript jsonl 文件 rotate（follow-up `devrix-d2-transcript-rotate`）
- session 级跨 turn metrics（follow-up `devrix-d7-turn-metrics`）
- session 级 UI 历史 turn 列表显示（D1 IM 适配器 scope）
- CLI 适配器 TurnInProgressError 友好提示（保持兼容）
- Streaming fallback 自动切换（DM-20260628-001 P0-2）
- prompt_too_long withhold-then-recover（DM-20260628-001 P0-3）

---

## 8. 关联变更

- **DM-20260628-002 (PR #271)**：panic hotfix，本需求是治本
- **PR #137**：transcript 写入已就绪，本需求补读取
- **PR #149 iter3**：D1 边界剥离 `<prior-output-summary>` 标签已就绪，本需求在 D7 边界写入
- **PR #257**：emit 链路补齐，本需求扩展 emit 时序约束
- **DM-20260626-001**：v6.0.0 14S → 6S，本需求在 D7-S15 落 S 层