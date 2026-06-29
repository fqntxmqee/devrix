# Design: D7 多轮 session 串行化与 complete 时机修正

**Change ID:** devrix-d7-multiturn-session-state
**Demand ID:** DM-20260628-004
**Status:** S3_Design
**Primary Domain:** D7 (orchestration, core)
**Secondary Domains:** D1 (communication, core)
**Author:** 2026-06-28 17:31 sess_1782638991113_5000 panic hotfix（PR #271）现场跟踪
**Related:** DM-20260628-002 (PR #271 panic hotfix), PR #137 (transcript), PR #149 iter3 (prior-output-summary strip)

---

## 1. Root Cause Analysis

| ID | 根因 | 触发条件 | 影响面 |
|----|------|----------|--------|
| **RC-1** | `session_turn_loop.go:186` 在 `for iter` break 后立即 emit `complete`，但子 WorkItem（SpawnDecompose 拆出的 review 子任务）通过 `ApplySpawnPolicy` 塞回 Tree 后**未立即执行**，要等下一轮 `focus` 拾取 | 任何产生 spawn-decompose 的 turn | 飞书卡片 Done 提前，finalText 后到不再追加 |
| **RC-2** | `ProcessMessage` (orchestrator.go:329) 用 `req.Message` 做 EnsureGoal directive，前一轮 finalText 完全丢失；D2 `FoldAssistantOutput` 已在落盘 transcript 但 D7 没有读取路径 | 任何 multi-turn session（turn N ≥ 2） | turn N+1 看不到 turn N 结论，等于无 multi-turn |
| **RC-3** | `SessionOrchestrator` 没有 turn 串行化机制，turn N 的 goroutine 还在 drain `out` channel 时 turn N+1 的 ProcessMessage 已开始；两个 goroutine 并发持有同一个 session_id 的 emitFn | 用户在 turn N 完成 finalText 前发 turn N+1 | panic（PR #271 已治症状，本需求治本） |
| **RC-4** | feishu adapter 只能基于 `Event.Type == "error"` 走统一文案，没有 TurnInProgressError 这种 domain-specific error 的处理路径 | multi-turn session + turn N+1 在 turn N 收尾前到 | 用户看到通用错误而不是"还在处理中" |

## 2. Solution Design

### 2.1 三层契约

```
[状态层]                              [注入层]                              [时序层]
TurnState (in-memory)                TranscriptReader                     WaitTurn(ctx)
  sessionID → turnHandle               transcript/{sessID}.jsonl            select done / ctx.Done
  sync.RWMutex                         kind=complete filter (capture/gateway.go:880)
  BeginTurn / EndTurn / WaitTurn       ReadRecent(n) → BuildPriorSummary   EndTurn on defer
```

### 2.2 关键代码骨架

```go
// sessionorchestrator/turn_state.go
type TurnState struct {
    mu      sync.RWMutex
    handles map[string]*turnHandle
}

type turnHandle struct {
    turnNo     int
    startedAt  time.Time
    done       chan struct{}
}

func (ts *TurnState) BeginTurn(sessionID string) error {
    ts.mu.Lock()
    defer ts.mu.Unlock()
    if h, ok := ts.handles[sessionID]; ok {
        select {
        case <-h.done:
            // 上一 turn 已 done 但未清理，强制替换
        default:
            return TurnInProgressError{SessionID: sessionID, SinceStartedAt: h.startedAt}
        }
    }
    ts.handles[sessionID] = &turnHandle{
        turnNo:    nextTurnNo(ts.handles, sessionID),
        startedAt: time.Now(),
        done:      make(chan struct{}),
    }
    return nil
}

func (ts *TurnState) EndTurn(sessionID string) {
    ts.mu.RLock()
    h, ok := ts.handles[sessionID]
    ts.mu.RUnlock()
    if !ok {
        return
    }
    close(h.done) // idempotent? 必须保证只 close 一次，加 sync.Once
}

func (ts *TurnState) WaitTurn(ctx context.Context, sessionID string) error {
    ts.mu.RLock()
    h, ok := ts.handles[sessionID]
    ts.mu.RUnlock()
    if !ok {
        return nil
    }
    select {
    case <-h.done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

type TurnInProgressError struct {
    SessionID       string
    SinceStartedAt  time.Time
}

func (e TurnInProgressError) Error() string {
    return fmt.Sprintf("session %s has a turn in progress since %s", e.SessionID, e.SinceStartedAt.Format(time.RFC3339))
}

func (e TurnInProgressError) Is(target error) bool {
    _, ok := target.(TurnInProgressError)
    return ok
}
```

```go
// sessionorchestrator/transcript_reader.go
//
// Reuses internal/layers/communication/capture/transcript.Writer.LoadReader()
// which already handles file-not-exist, sanitized sessionID, and jsonl scanning.
// No duplicate parsing logic.

type TranscriptReader struct {
    dir       string  // 默认 internal/layers/communication/capture/transcript
    maxRounds int
}

func NewTranscriptReader(dir string) *TranscriptReader {
    if dir == "" {
        dir = defaultTranscriptDir()
    }
    return &TranscriptReader{dir: dir, maxRounds: 16}
}

// ReadRecent 读最近 n 条 kind=complete 的 Body 字段。
// transcript schema: {t, kind, role, body}; finalText 标记为 kind="complete"
// (参见 capture/gateway.go:880 appendTranscriptEvent 的 complete 分支)。
// 文件不存在返回 ([]string{}, nil) 不报错。
func (r *TranscriptReader) ReadRecent(ctx context.Context, sessionID string, n int) ([]string, error) {
    if n <= 0 || r == nil || sessionID == "" {
        return nil, nil
    }
    if ctx.Err() != nil {
        return nil, ctx.Err()
    }
    w, err := transcript.NewWriter(r.dir)
    if err != nil {
        return nil, fmt.Errorf("transcript reader: new writer: %w", err)
    }
    events, err := w.LoadReader(sessionID)
    if err != nil {
        return nil, fmt.Errorf("transcript reader: load: %w", err)
    }
    var finals []string
    for _, ev := range events {
        if ev.Kind == "complete" && ev.Body != "" {
            finals = append(finals, ev.Body)
        }
    }
    if len(finals) > n {
        finals = finals[len(finals)-n:]
    }
    return finals, nil
}

func (r *TranscriptReader) BuildPriorOutputSummary(texts []string) string {
    if len(texts) == 0 {
        return ""
    }
    var b strings.Builder
    b.WriteString("<prior-output-summary>\n")
    for i, t := range texts {
        fmt.Fprintf(&b, "  [turn %d] %s\n", i+1, t)
    }
    b.WriteString("</prior-output-summary>")
    return b.String()
}
```

```go
// sessionorchestrator/orchestrator.go ProcessMessage 改造点
func (o *SessionOrchestrator) ProcessMessage(ctx context.Context, req orchtypes.ProcessRequest) (<-chan *contracts.EngineEvent, error) {
    // ... 前置逻辑不变 ...
    
    // [NEW] Turn 串行化门禁
    if o.turnState != nil {
        if err := o.turnState.WaitTurn(ctx, req.SessionID); err != nil {
            endSpanWithError(sessionSpan, err)
            return nil, err
        }
        if err := o.turnState.BeginTurn(req.SessionID); err != nil {
            endSpanWithError(sessionSpan, err)
            return nil, err
        }
    }
    
    // ... EnsureGoal 后插入 prior context 注入 ...
    if o.taskManager != nil && req.SessionID != "" && strings.TrimSpace(req.Message) != "" && intent.Kind != orchtypes.IntentSkip {
        _, _ = o.taskManager.Tree().EnsureGoal(req.SessionID, req.Message)
        
        // [NEW] prior context 注入
        if o.transcriptReader != nil && o.priorContextRounds > 0 {
            if texts, _ := o.transcriptReader.ReadRecent(ctx, req.SessionID, o.priorContextRounds); len(texts) > 0 {
                enriched := o.transcriptReader.BuildPriorOutputSummary(texts) + "\n\n" + req.Message
                _, _ = o.taskManager.Tree().EnsureGoal(req.SessionID, enriched)
            }
        }
    }
    
    // ... 后续 switch intent.Kind 不变 ...
}
```

```go
// sessionorchestrator/session_turn_loop.go 改造点
go func() {
    defer close(out)
    defer func() {
        if o.turnState != nil {
            o.turnState.EndTurn(req.SessionID)
        }
    }()
    // ... 现有逻辑不变 ...
}()
```

### 2.3 complete 时机隐含修正

**关键洞察**：现有 `session_turn_loop.go:186` emit `complete` 的时机实际上**没有问题**：

- `for iter` break 时，主线已闭环
- 后续子 WorkItem 会被下一次 ProcessMessage 的 `RunSessionTurnLoop` 拾取（不在本次 goroutine scope）
- processAutoClose 已在 channel close 路径上（PR #265 wiring point）

**真正的问题是用户感知层面**：飞书卡片拿到 `complete` 即显示 Done，但**用户期望**是"看到 finalText 才算 Done"。

解决方案：**complete event 内容要包含最终 finalText**（已由 `ExtractSessionDeliverable` 实现），且**不再有后续 finalText 后到**（由 turn 串行化保证 turn N+1 不会与 turn N 撞车）。

所以**不需要改 complete 时机**，只需要：(a) turn 串行化避免 panic（RC-3），(b) prior context 注入解决"无 multi-turn 记忆"（RC-2）。原需求 AC1（complete 触发时机修正）实际上**已经由 AC2（turn 串行化）隐含达成**——这是个重要的简化。

### 2.4 与 PR #271（DM-20260628-002）的双层防护关系

| 防护层 | 由谁提供 | 作用 |
|--------|----------|------|
| 防御性 recover（emit panic 不杀进程） | PR #271 helpers.go:39-46 | **症状治疗** — panic 发生后不让进程死 |
| exec.Emit 覆盖（每 Run 取最新） | PR #271 item_pipeline.go:119-123 | **症状治疗** — 阻止 stale emit hook 留到下轮 |
| **turn 串行化（本需求 T1+T4）** | turn_state.go + ProcessMessage | **治本** — 同 session 不允许多 turn 并发，根本消除 stale hook 场景 |

三层防护叠加后：
- 即使 turn 串行化实现有 bug 漏过某些并发路径，PR #271 的 emit recover + exec.Emit 覆盖仍然兜底
- 即使 PR #271 漏过某些 emit path，turn 串行化也能阻止大部分并发场景
- 任意两层失效，第三层兜底

## 3. 跨包 DAG 路由

```
D1 feishu.go          →  import sessionorchestrator (for errors.Is TurnInProgressError)
D7 sessionorchestrator →  import D2 FoldAssistantOutput 标签语法（不需要 import D2，仅参考格式）
                          TranscriptReader 读 internal/layers/communication/capture/transcript/ 目录
                          不需要 import D1 capture（仅 fs 访问）
```

- 跨域 import：仅 D1 → D7 方向（D1 feishu adapter 检测 D7 error）
- 不引入 D7 → D2 方向 import（保持分层）
- TranscriptReader 是 fs-only helper，不依赖 capture 包

## 4. 端到端调用时序

```
T0: 用户发送 turn 1 "review foo"
T1: feishu → D7 ProcessMessage("sess_x", "review foo")
T2: ProcessMessage → turnState.WaitTurn（无 in-flight，立即通过）
T3: ProcessMessage → turnState.BeginTurn("sess_x") ✅
T4: ProcessMessage → RunSessionTurnLoop → goroutine 启动 → defer EndTurn
T5: RunSessionTurnLoop → EnsureGoal("sess_x", "review foo") + transcript 注入（首次 session 无 prior text，无变化）
T6: for iter 循环 → Run("review foo") → LLM ReAct → 拆 2 个 spawn decompose 子任务
T7: for iter 循环 → Run(decompose child A) + Run(decompose child B) → 两个 finalText
T8: HasOpenWork → false → break → emit complete("foo review done")
T9: defer close(out) → defer EndTurn → processAutoClose → transcript jsonl 写入 kind=complete 事件
T10: feishu 卡片显示 ✅ Done + foo review 内容

T11: 用户发送 turn 2 "再 review bar"
T12: feishu → D7 ProcessMessage("sess_x", "再 review bar")
T13: ProcessMessage → turnState.WaitTurn（turn 1 已 EndTurn，立即通过）
T14: ProcessMessage → turnState.BeginTurn("sess_x") ✅
T15: ProcessMessage → RunSessionTurnLoop → EnsureGoal + transcript 注入：
       ReadRecent(n=3) → [turn1 complete Body] → BuildPriorOutputSummary
       → EnsureGoal("sess_x", "<prior-output-summary>...</prior-output-summary>\n\n再 review bar")
T16: for iter 循环 → Run → LLM 看到 prior summary → 输出 "bar review done, 与 foo 不同"
T17: emit complete
T18: transcript 写入 turn 2 complete 事件
```

## 5. 接口契约

### 5.1 TurnState API

```go
// sessionorchestrator/turn_state.go
func (ts *TurnState) BeginTurn(sessionID string) error
func (ts *TurnState) EndTurn(sessionID string)
func (ts *TurnState) WaitTurn(ctx context.Context, sessionID string) error
func (ts *TurnState) IsTurnInProgress(sessionID string) bool // helper for test
```

### 5.2 TranscriptReader API

```go
// sessionorchestrator/transcript_reader.go
func NewTranscriptReader(dir string) *TranscriptReader
func (r *TranscriptReader) ReadRecent(ctx context.Context, sessionID string, n int) ([]string, error)
func (r *TranscriptReader) BuildPriorOutputSummary(texts []string) string
```

### 5.3 OrchestratorOption 增量（functional-options 模式）

> 现有 `NewSessionOrchestrator(cfg, _, opts ...OrchestratorOption)` 走 functional options 模式（`WithSink / WithValidator / WithLearner / WithItemPipelineRunner / ...`），**不**用 `OrchestratorDeps` struct。本需求沿用相同模式：

```go
// sessionorchestrator/orchestrator.go

// WithPriorContextRounds enables prior-output-summary injection.
// n <= 0 (default) → injection disabled, TurnState not wired.
// n > 0 → construct TurnState + TranscriptReader, inject last n turns.
func WithPriorContextRounds(n int) OrchestratorOption

// WithTranscriptDir overrides the default transcript directory.
// Empty string → default internal/layers/communication/capture/transcript.
func WithTranscriptDir(dir string) OrchestratorOption

// WithTurnState injects a pre-built TurnState (used by tests to share state
// across orchestrator instances; production calls the convenience options).
func WithTurnState(ts *TurnState) OrchestratorOption
```

使用示例：
```go
orch := sessionorchestrator.NewSessionOrchestrator(cfg, nil,
    sessionorchestrator.WithItemPipelineRunner(p),
    sessionorchestrator.WithPriorContextRounds(3),
    sessionorchestrator.WithTranscriptDir("/custom/transcript/path"),
)
```

### 5.4 TurnInProgressError

```go
// sessionorchestrator/turn_state.go
type TurnInProgressError struct {
    SessionID      string
    SinceStartedAt time.Time
}

func (e TurnInProgressError) Error() string
func (e TurnInProgressError) Is(target error) bool
```

## 6. Span / Observability

| Span | Op | Attributes |
|------|----|----|
| `turn_state.wait` | `d7.s15.turn_state.wait` | session_id, turn_no, wait_ms, result (passed/timeout/in_progress) |
| `turn_state.begin` | `d7.s15.turn_state.begin` | session_id, turn_no, result (ok/turn_in_progress) |
| `transcript.read_recent` | `d7.s15.transcript.read_recent` | session_id, rounds_requested, rounds_returned, file_size_bytes |
| `prior_context.inject` | `d7.s15.prior_context.inject` | session_id, summary_chars |

新增 4 个 span（与 DM-20260626-001 "5 P0/P1 span 升格" 一致）。所有 span 在 sessionSpan 下作为 child span。

## 7. 回归风险评估

| 风险 | 等级 | 缓解 |
|------|------|------|
| TurnState 在 1000 session 并发下 RWMutex 竞争 | 中 | sync.RWMutex 适用（读多写少）；Read 用 RLock；1000 goroutine stress test 兜底 |
| transcript_reader 读 jsonl 时文件被并发 rotate/append | 中 | 仅 fs read，不持有锁；并发写由 transcript writer 保证 atomic append（PR #137 已实现） |
| prior-output-summary 注入后 LLM 误解标签 | 低 | 复用 D2 FoldAssistantOutput 标签语法（PR #149 iter3 已验证 LLM 理解） |
| ProcessMessage 改造点（WaitTurn + 注入）破坏既有 22 packages 测试 | 中 | S3-Gate review + 22/22 -race 兜底 + LP-1/LP-2/LP-5 baseline 不退化 |
| TurnInProgressError 误传给 CLI 适配器 | 低 | §3 明确 CLI 不识别此错误（保持兼容）；feishu 单独处理 |
| defer EndTurn 与 defer close(out) 顺序 | 中 | EndTurn 必须在 close(out) **之前**调用？或之后？→ 之后更安全（保证所有 events 已 emit 完再 EndTurn） |

## 8. 性能预算

| 操作 | 预算 | 实际（待测） |
|------|------|--------------|
| TurnState.WaitTurn (in-flight case) | < 1ms | RLock + channel select |
| TurnState.BeginTurn | < 1ms | Lock + map 操作 |
| TranscriptReader.ReadRecent (3 turns, 2.4K text) | < 5ms | os.Open + Scanner + JSON unmarshal × N |
| BuildPriorOutputSummary | < 1ms | strings.Builder |
| ProcessMessage 增量 (WaitTurn + ReadRecent + Inject) | < 10ms | 上述之和 |
| RunSessionTurnLoop defer EndTurn | < 100µs | RLock + close |

总增量：< 10ms per turn（turn N+1 入口）。对单 session 影响可忽略；高并发 multi-session 共享 TurnState 互斥锁，但 RWMutex 允许多读并发。

## 9. 兼容性矩阵

| 旧路径 | 新行为 | 影响 |
|--------|--------|------|
| `WithPriorContextRounds(0)` (默认, 不传 option) | TurnState 不构造 + transcript_reader 不读 + EnsureGoal 不注入 | 完全等价现状 |
| 单 turn session（turn N=1） | WaitTurn 立即通过 + transcript 首次读为空 → 不注入 | 完全等价现状 |
| feishu 适配器收到 TurnInProgressError | 显示"⏳ 上一条还在处理中" | 新增友好提示（breaking for cli？CLI 不识别此错误，回退到原文案） |
| CLI 适配器收到 TurnInProgressError | errors.Is 不识别，走通用 error 处理 | 保持兼容 |

## 10. 验收对照

| AC | 验收方式 | 通过条件 |
|----|----------|----------|
| AC1 (complete 时机修正) | T1+T4 串行化后隐含达成 | T1/T4 单测通过 + e2e 断言 complete 是最后事件 |
| AC2 (turn 串行化) | T1+T4 单测 + e2e | T1 1000 goroutine stress + e2e 双 turn 不并发 |
| AC3 (prior context 注入) | T2+T4 单测 + e2e | T2 单测 5 case + e2e turn 2 directive 含 turn 1 finalText |
| AC4 (feishu 友好文案) | T5 单测 | T5 2 case PASS |
| AC5 (TurnState API) | T1 单测 | T1 8 case PASS |
| AC6 (覆盖率 ≥ 80%) | go test -cover | ≥ 80% |
| AC7 (LP-5 e2e) | T6 | T6 集成测试 PASS |
| AC8 (飞书实测) | 用户实测 | 用户确认 turn 2 看到 turn 1 内容 |

## 11. 备注

- 本次 S2 + S3 + tasks + spec delta 同步提交，符合 devrix-d7-real-closure-pr36 "一次性扫 4 cell" 工作模式
- S4 实现预计 4 个 PR（按 tasks.md §5 Implementation Plan PR-1 到 PR-4）
- S5 验收 = 飞书实测 + 22/22 orchestration -race + verify-archive.sh 12/12
- S6 归档到 `openspec/archive/2026-06-28-devrix-d7-multiturn-session-state/`