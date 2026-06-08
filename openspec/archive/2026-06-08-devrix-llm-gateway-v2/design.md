# LLM Gateway Reliability Enhancement Design

**Change ID:** devrix-llm-gateway-v2
**Parent:** devrix-llm-gateway (V1)
**Status:** S2 Design

---

## 一、CB + Retry 协调

### 1.1 问题根因

当前 `gateway.go:121-124` 逻辑：

```go
adapterCh, err := g.retry.Stream(ctx, streamCall, primaryModel, ...)
if err != nil {
    g.breaker.RecordFailure(provider)  // Retry 已内部记录每次失败
    ...
}
```

但 Retry 内部 `retry.go:66-73` 并不会调用 CB——问题出在 **Gateway 在 Retry 失败后只记录一次 CB failure**，这看起来合理，但流内的 **stream error** (`gateway.go:131-163`) 会在 goroutine 中每一块 chunk 出错时都调用 `g.breaker.RecordFailure(provider)`，包括 Retry 成功返回后流内发生的错误。

此外，**Half-Open 状态无限并发探测**：

```go
// breaker.go:68-69 — Half-Open 允许所有请求通过
case llmgateway.CircuitHalfOpen:
    return true, nil  // 没有限制！可能导致大量请求涌入
```

### 1.2 修复方案

**A. Half-Open 探测限制（breaker.go）：**

```go
type circuitRecord struct {
    state              llmgateway.CircuitState
    failureCount       int
    halfOpenSuccesses  int
    halfOpenInFlight   int     // 新增：探测请求计数
    openedAt           time.Time
}

func (b *CircuitBreaker) Allow(circuitKey string) (bool, error) {
    // ...
    case llmgateway.CircuitHalfOpen:
        if rec.halfOpenInFlight >= b.cfg.HalfOpenMaxProbes {
            return false, sharederrors.NewCircuitOpenError(circuitKey)
        }
        rec.halfOpenInFlight++
        return true, nil
}

// RecordSuccess/RecordFailure 需在状态变更后调用 finalize 以 decrement in-flight
func (b *CircuitBreaker) finalize(circuitKey string) {
    rec := b.circuits[circuitKey]
    if rec.halfOpenInFlight > 0 {
        rec.halfOpenInFlight--
    }
}

func (b *CircuitBreaker) RecordSuccess(circuitKey string) {
    b.mu.Lock()
    defer b.mu.Unlock()
    // ... existing success logic ...
    b.finalize(circuitKey)  // 新增
}

func (b *CircuitBreaker) RecordFailure(circuitKey string) {
    b.mu.Lock()
    defer b.mu.Unlock()
    // ... existing failure logic ...
    b.finalize(circuitKey)  // 新增
}
```

**B. Gateway 流错误处理优化（gateway.go）：**

流内 goroutine 中，区分 "stream 中途异常" 和 "整个请求失败"：

```go
go func() {
    defer close(out)

    var streamErr error
    var usage llmgateway.TokenUsage
    for ac := range adapterCh {
        if ac.Error != nil {
            streamErr = ac.Error
            break
        }
        // ... 正常处理 chunks ...
    }

    if streamErr != nil {
        // 仅对非 context-cancel 错误调用 CB
        if !errors.Is(streamErr, context.Canceled) && !errors.Is(streamErr, context.DeadlineExceeded) {
            g.breaker.RecordFailure(provider)
        }
        g.recordError(provider, primaryModel)
        finishSpan(streamErr, usage)
        return
    }
    g.breaker.RecordSuccess(provider)
    // ...
}()
```

**C. Retry 最终失败才触发 CB：**

Retry 内部的 attempt 失败不通知 CB，只有 Retry 返回 error 时 Gateway 才 `RecordFailure`。当前 `gateway.go:121` 已经做到这点（只有 retry.Stream 返回 error 才记录），但需确保 stream error 不会重复触发。

---

## 二、Context 超时传播

### 2.1 新增默认超时

**文件：** `gateway/gateway.go`

```go
func (g *Gateway) Stream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
    // 确保 ctx 有 deadline
    if _, ok := ctx.Deadline(); !ok {
        timeout := g.cfg.DefaultTimeout
        if timeout <= 0 {
            timeout = 30 * time.Second
        }
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, timeout)
        defer cancel()
    }
    // ... 后续逻辑 ...
}
```

### 2.2 流内超时检测

goroutine 中已有 `case <-ctx.Done()` 检测 (`gateway.go:145-150`)，保持不变。

---

## 三、Retry Jitter

### 3.1 修复方案

**文件：** `retry/retry.go`

```go
import "math/rand"

func backoffDelay(cfg sharedconfig.LLMRetryConfig, attempt int) time.Duration {
    initial := cfg.InitialDelay
    if initial <= 0 {
        initial = time.Second
    }
    maxDelay := cfg.MaxDelay
    if maxDelay <= 0 {
        maxDelay = 10 * time.Second
    }
    backoff := cfg.Backoff
    if backoff <= 0 {
        backoff = 2.0
    }

    delay := float64(initial) * pow(backoff, float64(attempt))

    // Full jitter: randomize between 0 and delay
    jitter := time.Duration(rand.Int63n(int64(delay)))
    delay = float64(jitter)

    if delay > float64(maxDelay) {
        delay = float64(maxDelay)
    }
    return time.Duration(delay)
}
```

使用 **Full Jitter** 而非简单的 ±25%，避免多个实例因相近的退避时间同步重试。Full Jitter 是 AWS 推荐的退避策略。

### 3.2 测试可确定性

在测试中通过可注入的 `rand.Source` 控制随机性。`backoffDelay` 改为 `Executor` 的方法以使用注入的 rng：

```go
type Executor struct {
    rng *rand.Rand
}

func NewExecutor() *Executor {
    return &Executor{
        rng: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

// WithRNG 注入随机源（测试用）
func (e *Executor) WithRNG(rng *rand.Rand) *Executor {
    e.rng = rng
    return e
}

func (e *Executor) backoffDelay(cfg sharedconfig.LLMRetryConfig, attempt int) time.Duration {
    initial := cfg.InitialDelay
    if initial <= 0 { initial = time.Second }
    maxDelay := cfg.MaxDelay
    if maxDelay <= 0 { maxDelay = 10 * time.Second }
    backoff := cfg.Backoff
    if backoff <= 0 { backoff = 2.0 }

    delay := float64(initial) * pow(backoff, float64(attempt))

    // Full jitter: [0, delay)
    jitter := time.Duration(e.rng.Int63n(int64(delay)))
    if jitter > time.Duration(maxDelay) {
        jitter = time.Duration(maxDelay)
    }
    return jitter
}
```

---

## 四、中文 Token 补偿（P2 附带）

**文件：** `token/counter.go`

当前使用 `cl100k_base` 编码，中文每个汉字约 1.5-2 token。对于中文为主的项目，实际 token 数可能被低估。增加一个简单的补偿系数：

```go
type Counter struct {
    mu              sync.RWMutex
    encoding         *tiktoken.Tiktoken
    cjkMultiplier   float64  // 中文补偿系数，默认 1.0（不补偿）
}

func (c *Counter) CountText(text string) int {
    tokens := c.encoding.Encode(text, nil, nil)
    count := len(tokens)
    if c.cjkMultiplier > 1.0 && containsCJK(text) {
        count = int(float64(count) * c.cjkMultiplier)
    }
    return count
}

// containsCJK 检测文本是否包含中日韩字符
func containsCJK(s string) bool {
    for _, r := range s {
        if (r >= 0x4E00 && r <= 0x9FFF) ||   // CJK Unified Ideographs
           (r >= 0x3400 && r <= 0x4DBF) ||      // CJK Extension A
           (r >= 0x3000 && r <= 0x303F) {       // CJK Symbols/Punctuation
            return true
        }
    }
    return false
}
```

---

## 五、受影响的文件

```
internal/layers/llmgateway/
├── breaker/
│   ├── circuit_breaker.go       # MODIFIED: Half-Open in-flight 限制
│   └── circuit_breaker_test.go  # MODIFIED: Half-Open 探测测试
├── retry/
│   ├── retry.go                 # MODIFIED: Full Jitter
│   └── retry_test.go            # MODIFIED: Jitter 分布测试
├── gateway/
│   ├── gateway.go               # MODIFIED: 默认超时 + 流错误优化
│   └── gateway_test.go          # MODIFIED: 超时测试
├── token/
│   └── counter.go               # MODIFIED: 中文补偿（P2）
└── breaker/
    └── state.go                 # MODIFIED: 新增 halfOpenMaxProbes 配置
```

---

## 六、回归风险评估

| 变更 | 回归风险 | 缓解措施 |
|------|---------|---------|
| Half-Open 探测限制 | 中 — 可能降低故障恢复速度 | 配置化 maxProbes，默认 1 |
| 默认超时注入 | 低 — 仅在上层无 deadline 时触发 | 不影响已有显式超时调用 |
| Full Jitter | 低 — 仅改变退避延迟 | 固定种子测试 |
| Stream error 处理 | 中 — 改变 CB 触发条件 | 集成测试覆盖流错误场景 |
