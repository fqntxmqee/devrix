# Tasks: devrix-d7-multiturn-session-state

**Change ID:** devrix-d7-multiturn-session-state
**Demand ID:** DM-20260628-003
**Status:** S2_Design
**Created:** 2026-06-28

---

## 任务清单（按 F-T 映射）

### T1: TurnState struct + Begin/End/Wait API [D7-S15-A55-T01]

- **范围**: `internal/layers/orchestration/sessionorchestrator/turn_state.go` + `turn_state_test.go`
- **F**: 新 F12 `TurnState` struct + `BeginTurn / EndTurn / WaitTurn`
- **预估**: 130 行实现 + 180 行单测
- **依赖**: 无

**步骤：**
- [ ] 定义 `TurnState struct { mu sync.RWMutex; handles map[string]*turnHandle }`
- [ ] 定义 `turnHandle struct { turnNo int; startedAt time.Time; done chan struct{} }`
- [ ] 实现 `BeginTurn(sessionID string) (turnHandle, error)`：map 已有 handle 且 handle.done 未关闭 → 返回 `TurnInProgressError`；否则替换 handle（关闭旧 done 防泄漏）
- [ ] 实现 `EndTurn(sessionID string)`：`close(handle.done)` + 保留 handle 一段时间供 WaitTurn 用；可加 cleanup goroutine
- [ ] 实现 `WaitTurn(ctx context.Context, sessionID string) error`：`select { case <-handle.done: return nil; case <-ctx.Done(): return ctx.Err() }`
- [ ] 单测 8 case：单 turn Begin→End→Wait 顺序、双并发 Begin 一个胜一个 TurnInProgress、ctx 取消中途退出、EndTurn 重复幂等、Beginturn 后清理旧 handle、nil receiver 边界、并发 1000 goroutine stress

### T2: transcript_reader.go 读最近 N 轮 finalText [D7-S15-A56-T03]

- **范围**: `internal/layers/orchestration/sessionorchestrator/transcript_reader.go` + `transcript_reader_test.go`
- **F**: 新 F13 `TranscriptReader.ReadRecent` + `BuildPriorOutputSummary`
- **预估**: 100 行实现 + 100 行单测
- **依赖**: 无（独立 helper）

**步骤：**
- [ ] 定义 `TranscriptReader struct { dir string; maxRounds int }`
- [ ] 实现 `NewTranscriptReader(dir string) *TranscriptReader`：dir 默认 `internal/layers/communication/capture/transcript`
- [ ] 实现 `ReadRecent(ctx, sessionID, n int) ([]string, error)`：复用 `internal/layers/communication/capture/transcript.Writer.LoadReader()`（避免重复 fs/json 解析逻辑），filter `kind=="complete"`，取 Body 字段，按时间序取最后 n 条
- [ ] 实现 `BuildPriorOutputSummary(texts []string) string`：拼成 `<prior-output-summary>\n  [turn 1] xxx\n  [turn 2] yyy\n</prior-output-summary>` 格式（参考 D2 `FoldAssistantOutput` 标签语法）
- [ ] 单测 5 case：空文件、单 complete、多 complete 取最近 n、超 n 截断、文件不存在返回空 slice 不报错

### T3: SessionOrchestrator 加 WithPriorContextRounds / WithTranscriptDir functional options [D7-S15-A55-T02]

- **范围**: `internal/layers/orchestration/sessionorchestrator/orchestrator.go` `SessionOrchestrator` struct + functional options
- **F**: 新增 3 个 OrchestratorOption（functional options 模式，与 `WithLearner / WithSink` 一致）
- **预估**: 30 行
- **依赖**: T1, T2

**步骤：**
- [ ] `WithPriorContextRounds(n int) OrchestratorOption` functional option：n>0 时构造 `TurnState` 并挂 `transcriptReader`
- [ ] `WithTranscriptDir(dir string) OrchestratorOption` functional option：覆盖默认 transcript 目录
- [ ] `WithTurnState(ts *TurnState) OrchestratorOption` functional option：测试用，注入预构建 TurnState
- [ ] `SessionOrchestrator.turnState` + `transcriptReader` 私有字段
- [ ] 单测 2 case：默认（无 option）不构造 turnState；WithPriorContextRounds(3) 时构造

### T4: ProcessMessage 接入 WaitTurn + prior-output-summary 注入 [D7-S15-A56-T04]

- **范围**: `internal/layers/orchestration/sessionorchestrator/orchestrator.go` `ProcessMessage`
- **F**: 新增 turn 串行化门禁 + directive 注入
- **预估**: 80 行
- **依赖**: T1, T2, T3

**步骤：**
- [ ] 在 `ProcessMessage` 入口（line 200+ 早期）插入 `if o.turnState != nil { WaitTurn(ctx, req.SessionID) }` → 返回 `TurnInProgressError` 时 `endSpanWithError(sessionSpan, err)` 并 return
- [ ] 在 `EnsureGoal`（line 329-331）之后、sessionSpan/IntentClassification 之前插入 prior context 读取 + directive 注入：
  ```go
  if o.transcriptReader != nil && req.Message != "" {
      if texts, _ := o.transcriptReader.ReadRecent(ctx, req.SessionID, o.priorContextRounds); len(texts) > 0 {
          enriched := o.transcriptReader.BuildPriorOutputSummary(texts) + "\n\n" + req.Message
          _, _ = o.taskManager.Tree().EnsureGoal(req.SessionID, enriched)
      }
  }
  ```
- [ ] 在 `RunSessionTurnLoop` goroutine 入口增加 `defer o.turnState.EndTurn(req.SessionID)`（确保 channel close 后立即释放 turn slot）
- [ ] 在 `RunSessionTurnLoop` 入口增加 `BeginTurn` 调用：失败（TurnInProgress）时直接返回 error channel
- [ ] 定义 `TurnInProgressError struct { SessionID string; SinceStartedAt time.Time }` 含 `Error() string` + `Is(target error) bool` (支持 `errors.Is(err, TurnInProgressError)`)
- [ ] 单测 4 case：WaitTurn 正常通过、TurnInProgress 错误传播、prior context 注入后 EnsureGoal directive 包含标签、RunSessionTurnLoop defer EndTurn 触发

### T5: feishu adapter 识别 TurnInProgressError + 友好文案 [D7-S15-A58-T06]

- **范围**: `internal/layers/communication/feishu/feishu.go`
- **F**: IM 适配器差异化文案（仅 feishu，CLI 不动）
- **预估**: 30 行
- **依赖**: T4

**步骤：**
- [ ] feishu 处理 ProcessMessage 返回 error 时，先 `errors.Is(err, sessionorchestrator.TurnInProgressError)` 判断
- [ ] 命中则发送 IM 卡片 "⏳ 上一条消息还在处理中，请稍候..."（不返回 error 事件，避免 Done 卡片被覆盖）
- [ ] 不命中走原有 error 处理路径
- [ ] 单测 2 case：TurnInProgressError 返回正确文案；其他 error 走原文案

### T6: LP-5 e2e 集成测试 [D7-S15-A59-T07]

- **范围**: `tests/integration/multiturn_session_e2e_test.go`
- **F**: 端到端验证 multi-turn 串行化 + 上下文注入
- **预估**: 200 行
- **依赖**: T1-T5

**步骤：**
- [ ] 构造 mock LLM（脚本化返回 2 轮 finalText）
- [ ] turn 1：发送 "review foo"，等 complete 事件，断言 finalText 包含 "foo"
- [ ] turn 2：发送 "再 review bar"，等 complete 事件
- [ ] 断言：turn 2 directive 包含 "review foo" 关键词（prior-output-summary 注入成功）
- [ ] 断言：turn 2 finalText 包含 "bar" 关键词
- [ ] 断言：两个 complete 事件是各自 turn 的最后一个事件（无提前 Done）
- [ ] 断言：没有并发 panic（`go test -race`）

### T7: 跨包回归 + verify-archive 兜底 [S6 归档]

- **范围**: 22/22 orchestration packages `go test -race` + `scripts/verify-archive.sh` + 现有 LP-1/LP-2/LP-5 baseline
- **F**: 回归保护
- **预估**: 0 行新增
- **依赖**: T1-T6

**步骤：**
- [ ] `go test -race ./internal/layers/orchestration/...` 必须 22/22 PASS
- [ ] `scripts/verify-archive.sh` 必须 12/12 PASS
- [ ] LP-1/LP-2/LP-5 baseline 测试不能退化
- [ ] `go vet ./...` 干净

---

## P0 T 层汇总（最终以 S3 design 定稿为准）

| T ID | 描述 | Status |
|------|------|--------|
| D7-S15-A55-T01 | `TurnState` struct 定义 + BeginTurn/EndTurn/WaitTurn API + 并发安全（1000 goroutine stress） | PENDING |
| D7-S15-A55-T02 | `WithPriorContextRounds` + `WithTranscriptDir` + `WithTurnState` functional options + `SessionOrchestrator.turnState`/`transcriptReader` 字段挂载 | PENDING |
| D7-S15-A56-T03 | `transcript_reader.go` 读最近 N 轮 finalText helper（filter kind=complete 事件 Body 字段；复用 transcript.Writer.LoadReader） | PENDING |
| D7-S15-A56-T04 | `ProcessMessage` 接入 WaitTurn + prior-output-summary 注入 + `TurnInProgressError` 定义 + RunSessionTurnLoop defer EndTurn | PENDING |
| D7-S15-A57-T05 | `session_turn_loop.go` complete 事件 emit 时机隐含修正（processAutoClose 已在 channel close 路径，无须显式改动） | N/A (无需新增 T，已隐含) |
| D7-S15-A58-T06 | feishu adapter 识别 `TurnInProgressError` + "上一条还在处理中" 文案 | PENDING |
| D7-S15-A59-T07 | LP-5 e2e 集成测试：sess_e2e_multiturn_v1（断言 complete 时机 + prior context 注入） | PENDING |

7 个 T 点（其中 T5 因 processAutoClose 已就绪无新增改动），全部 P0。