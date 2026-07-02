# D7 Spec Delta: Execute 节点 Partition + Bash Sibling Abort + Discard

**Change:** devrix-d2-tool-input-aware-concurrency-and-classifier (DM-20260702-009)
**Archived:** 2026-07-02
**Status:** S7_Archived

## Delta Summary

D7 编排层 Execute 节点承接本 change 3 个 T 点:
- T18 partitionToolCalls (ExecuteRound 集成)
- T26 Bash sibling abort (per-batch controller)
- T27 StreamingToolExecutor.Discard() + fallback 路径 wiring

## Delta 1: partitionToolCalls 集成 (T18)

### ExecuteRound 改造

原 ExecuteRound:
```go
// 单层 errgroup, 全部并行
g.Go(func() error {
    return executeOne(ctx, call)
})
```

改造后 ExecuteRound:
```go
// 两层: batch 间串行 + batch 内并发
batches := partitionToolCalls(calls, surfaces, concurrency)
for _, b := range batches {
    executeOneBatch(ctx, b, exec, concurrencyLimit, results)
}
```

### Wire Point

`internal/bootstrap/turn_adapter.go::ExecuteRound` line 277 改造点

## Delta 2: Bash Sibling Abort (T26)

### New Controller

```go
// BashSiblingAbortController 是 per-batch controller,
// 当 watched call (当前仅 bash) 失败时, 取消其它 watched siblings.
type BashSiblingAbortController struct {
    parentCtx context.Context
    registry  map[string]context.CancelFunc
    mu        sync.Mutex
    once      sync.Once
    closed    bool
}

func (c *BashSiblingAbortController) Register(callID, toolName string) (ctx context.Context, cancel context.CancelFunc, ok bool)
func (c *BashSiblingAbortController) Unregister(callID string)
func (c *BashSiblingAbortController) AbortSiblings(callID, reason string) bool
func (c *BashSiblingAbortController) Close()
```

### Watched Tool Predicate

```go
// isSiblingAbortWatched: 当前仅 bash.
func isSiblingAbortWatched(name string) bool {
    return name == "bash"
}
```

### Integration

```go
// executeOneBatch parallel branch
controller := bash.NewBashSiblingAbortController(ctx, len(b.Calls))
defer controller.Close()

var g errgroup.Group
if concurrencyLimit > 0 {
    g.SetLimit(concurrencyLimit)
}
for j, c := range b.Calls {
    g.Go(func() error {
        result := executeOneWithSiblingAbort(ctx, controller, exec, c)
        results[b.Indices[j]] = result
        return nil
    })
}
```

### AC12 Invariant

- Watched call 失败时, AbortSiblings 取消其它 watched siblings
- 非 watched call (read_file / grep / glob) 不参与, 保持 read-only 语义
- 多次 AbortSiblings 调用 idempotent (sync.Once wrapped)

## Delta 3: StreamingToolExecutor.Discard() (T27)

### New Executor

```go
// StreamingToolExecutor buffers tool_use blocks as they stream from LLM.
// Per LLM iteration instance — one executor lives for one round-trip.
type StreamingToolExecutor struct {
    mu            sync.Mutex
    buffered      []llmgateway.ToolCall
    discarded     bool
    discardReason string
}

func (e *StreamingToolExecutor) Buffer(call llmgateway.ToolCall)
func (e *StreamingToolExecutor) Buffered() []llmgateway.ToolCall
func (e *StreamingToolExecutor) BufferedCount() int
func (e *StreamingToolExecutor) IsDiscarded() bool
func (e *StreamingToolExecutor) DiscardReason() string
func (e *StreamingToolExecutor) Discard(reason string) []sessionorchestrator.ToolResult
```

### Sentinel Constant

```go
const ErrStreamingFallbackSentinel = "streaming_fallback"
```

### DiscardOnFallback Wiring

```go
type DiscardOnFallback struct {
    executor        *StreamingToolExecutor
    discardedCount  atomic.Int64
}

func NewDiscardOnFallback(executor *StreamingToolExecutor) *DiscardOnFallback
func (d *DiscardOnFallback) OnFallback(reason string) []sessionorchestrator.ToolResult
func (d *DiscardOnFallback) DiscardedCount() int64
```

### AC13 Invariant

- 主 LLM fallback 触发前, OnFallback(reason) 被调用
- Discard(reason) 合成 streaming_fallback sentinel 结果, 喂给 fallback 模型
- Idempotent: 多次 OnFallback 调用只 +1 计数 (sync.Once 语义)
- Nil receiver / nil executor 都是 no-op

## Delta 4: AutoModeClassifier P2 Stub (T22'-T23')

### New Interface

```go
// AutoModeClassifier — P2 interface only, 不实施.
// 触发升 P1 实施的条件: verify_contract.deny_rate > 5% (任意 7 天窗口)
type AutoModeClassifier interface {
    ClassifyToolUse(ctx context.Context, transcript []TranscriptBlock) (YoloResult, error)
}

type YoloResult struct {
    Decision YoloDecision
    Reason   string
    Source   string  // "anthropic" | "external" | "rule-fallback"
}
```

### Stub Behavior

```go
func (s *autoClassifierStub) ClassifyToolUse(ctx context.Context, transcript []TranscriptBlock) (YoloResult, error) {
    panic("P2 interface, not implemented; see gaming-debate-round3-convergence.md")
}
```

### ChannelRouter TODO

`internal/bootstrap/turn_adapter.go::ExecuteRound`:

```go
// TODO(gaming-debate-round3): 升 P1 时接入 AutoModeClassifier
// 触发 metric: verify_contract.deny_rate 7d 滑动 > 5%
// 触发 change: devrix-d2-tool-input-aware-concurrency-and-classifier-pr-d-followup
```

## tech-debt Closed

| tech-debt | Closed-by | Files |
|-----------|-----------|-------|
| TD-STE-02 | PR-F T26 Bash sibling abort | `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` |
| TD-STE-03 | PR-F T27 StreamingToolExecutor.Discard() | `internal/bootstrap/streaming_executor.go` + `discard_on_fallback.go` |

## Cross-Reference

- d2-spec-delta: ToolSurface v4 + 19 工具 default + partition + toCompactBlock + inputsEquivalent
- d5-spec-delta: LTL-Lite L4-L6 终止不变量 cross-check 配套