---
design-id: devrix-d6-validation-metric
demand-id: DM-20260614-002
proposal-id: devrix-d6-validation-metric
status: S3_Design
version: 1.0.0
created: 2026-06-14
last-updated: 2026-06-14
---

# D6 校验指标可观测性 — 设计

## 1. 架构

```
┌─────────────────────────────────────────────────────────────┐
│ D7 SessionOrchestrator                                      │
│  ┌────────────────────────────────────────────────────┐     │
│  │ ProcessMessage(ctx, req)                           │     │
│  │  ├── ClassifyIntent                                │     │
│  │  └── if validator != nil:                          │     │
│  │       └── d6Metrics.Record(validator, ctx, dec) ───┼──┐  │
│  │            │ elapsed, panic-guard                 │  │  │
│  │            └── result ← safe-call                  │  │  │
│  │                 ├── pass → inc(pass)               │  │  │
│  │                 ├── fail → inc(fail)               │  │  │
│  │                 ├── timeout → inc(timeout)         │  │  │
│  │                 └── error → inc(error)             │  │  │
│  └────────────────────────────────────────────────────┘  │  │
└─────────────────────────────────────────────────────────────┘  │
                                                                 │
┌────────────────────────────────────────────────────────────┐  │
│ D5 observability (D6ValidationMetrics) ◀────────────────────┘  │
│  ┌────────────────────────────────────────────────────┐     │
│  │ 4 × Counter (Meter.Int64Counter)                  │     │
│  │  pass / fail / timeout / error                    │     │
│  ├────────────────────────────────────────────────────┤     │
│  │ 5min sliding window 计数                          │     │
│  │  timeout_rate = timeout / (pass+fail+timeout)     │     │
│  │  if rate > 5% → WARN log + AlertHook             │     │
│  └────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

## 2. 数据结构

### 2.1 D6ValidationMetrics

```go
// D6ValidationMetrics owns the 4 D6 validation counters and the
// sliding-window timeout-rate computation.
//
// v1.0 P1 entry (per R2 §5 P1 #6). Lives in d7 package because the
// D6 advisory validation is invoked from the D7 orchestrator.
type D6ValidationMetrics struct {
    pass     metrics.Counter
    fail     metrics.Counter
    timeout  metrics.Counter
    error    metrics.Counter
    
    mu       sync.Mutex
    window   []windowSample  // ring buffer
    rate     float64
    lastRate time.Time
    
    onAlert  AlertHook  // injectable for v1.1 AlertManager integration
}

type windowSample struct {
    at       time.Time
    outcome  string  // "pass"|"fail"|"timeout"|"error"
}
```

### 2.2 OrchestratorOption

```go
// WithMetrics wires the D6 validation metrics sink.
func WithMetrics(m *D6ValidationMetrics) OrchestratorOption {
    return func(o *SessionOrchestrator) { o.d6Metrics = m }
}
```

### 2.3 AlertHook

```go
// AlertHook is invoked when timeout_rate exceeds the configured
// threshold. v1.0 default: WARN log; v1.1+: Prometheus AlertManager
// webhook.
type AlertHook func(rate float64, samples uint64)
```

## 3. 流程

### 3.1 计时与分流

```go
func (o *SessionOrchestrator) callD6Validator(ctx context.Context, intent IntentClassification, sessionID string) {
    if o.validator == nil || o.d6Metrics == nil {
        return  // graceful no-op
    }
    timeoutMs := o.cfg.D6ValidationTimeoutMs
    if timeoutMs <= 0 {
        timeoutMs = 50
    }
    vctx, cancel := context.WithTimeout(ctx, durationOrDefault(timeoutMs))
    defer cancel()
    
    start := time.Now()
    var result ValidationResult
    func() {
        defer func() {
            if r := recover(); r != nil {
                o.d6Metrics.RecordError(sessionID, time.Since(start))
                slog.Error("d7: D6 validator panic", "panic", r, "session", sessionID)
            }
        }()
        result = o.validator.ValidateOrchestration(vctx, OrchestrationDecision{Intent: intent, SessionID: sessionID})
    }()
    elapsed := time.Since(start)
    
    switch {
    case elapsed > 2*durationOrDefault(timeoutMs):
        o.d6Metrics.RecordError(sessionID, elapsed)
    case elapsed > durationOrDefault(timeoutMs):
        o.d6Metrics.RecordTimeout(sessionID, elapsed)
    case result.Pass:
        o.d6Metrics.RecordPass(sessionID, elapsed)
    default:
        o.d6Metrics.RecordFail(sessionID, elapsed)
    }
}
```

### 3.2 滑窗 rate 计算

```go
const (
    rateWindow   = 5 * time.Minute
    rateAlertThr = 0.05
)

func (m *D6ValidationMetrics) RecordOutcome(outcome string, at time.Time) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // ring buffer prune
    cutoff := at.Add(-rateWindow)
    pruned := m.window[:0]
    for _, s := range m.window {
        if s.at.After(cutoff) {
            pruned = append(pruned, s)
        }
    }
    m.window = append(pruned, windowSample{at: at, outcome: outcome})
    
    // increment counter
    switch outcome {
    case "pass":    m.pass.Inc()
    case "fail":    m.fail.Inc()
    case "timeout": m.timeout.Inc()
    case "error":   m.error.Inc()
    }
    
    m.recomputeRate(at)
}

func (m *D6ValidationMetrics) recomputeRate(at time.Time) {
    var total, timeouts uint64
    for _, s := range m.window {
        total++
        if s.outcome == "timeout" || s.outcome == "error" {
            timeouts++
        }
    }
    if total == 0 {
        m.rate = 0
        return
    }
    m.rate = float64(timeouts) / float64(total)
    m.lastRate = at
    
    if m.rate > rateAlertThr && m.onAlert != nil && total >= 20 {
        // 20 sample 最小值避免冷启动误报
        m.onAlert(m.rate, total)
    }
}
```

## 4. 测试点

| T ID | 描述 | 归属 A | Test 函数 |
|------|------|--------|-----------|
| D7-D6-T03 | 4 counter 注入并按 result.Pass 分流 | D5-METRICS | `TestD6ValidationMetrics_Record_PassFail` |
| D7-D6-T04 | timeout_rate > 5% 触发 AlertHook | D5-ALERT | `TestD6ValidationMetrics_TimeoutRate_Alert` |
| D7-D6-T05 | panic-recovered 计入 error 路径 | D7-OBS | `TestOrchestrator_D6Validator_Panic_RecordsError` |
| D7-D6-T06 | nil validator 与 nil metrics 都降级为 no-op | D7-OBS | `TestOrchestrator_NoValidator_NoMetrics` |

## 5. 向后兼容

- `D6Validator` 接口签名不变
- `WithMetrics` 是 optional option；不传则 orchestrator 走现有路径（结果仍 `_ =` 丢弃）
- `D6ValidationMetrics` 可独立测试，不依赖 Orchestrator

## 6. 不在本设计内

- 真实 D6 validator 实现（仍是接口）
- Prometheus AlertManager rules yaml 推送（v1.1+）
- metrics 持久化（counter 启动清零，符合 D5 multiagent 现有约定）
