---
design-id: devrix-s5-p2-shadow-classifier
title: S5-P2 Tail-only LLM Classify Shadow — 设计
proposal-id: devrix-s5-p2-shadow-classifier
status: S3_Design
created: 2026-06-14
last-updated: 2024-06-14
---

# S5-P2 Tail-only LLM Classify Shadow — 设计

## 1. 架构图

```
                 ┌────────────────────┐
   msg ────────▶ │  SessionOrchestrator.ProcessMessage
                 └──────────┬─────────┘
                            │
                            ▼
                 ┌────────────────────┐
                 │  classifier.Classify
                 └──────────┬─────────┘
                            │
              ┌─────────────┴──────────────┐
              │                            │
              ▼                            ▼
   ┌────────────────────┐      ┌────────────────────────┐
   │  RuleClassifier     │      │  ShadowClassifier      │
   │  (synchronous)      │      │  (wrap rule + llm)     │
   └─────────┬───────────┘      └──────────┬─────────────┘
             │                              │
             │ IntentSkip/Command/Fast      │ IntentOrchestrate
             │                              │ (tail)
             ▼                              ▼
        return rule                   return rule
                                            │
                                            │ fire-and-forget goroutine
                                            ▼
                              ┌──────────────────────────┐
                              │  shadowAsync             │
                              │  ├─ ctx.WithTimeout(500) │
                              │  ├─ llm.ClassifyIntent   │
                              │  ├─ metrics.Match/Mismatch/Error │
                              │  ├─ Latency.Record       │
                              │  └─ log.Warn on error    │
                              └──────────────────────────┘
                                            │
                                            ▼
                              ┌──────────────────────────┐
                              │  D5 observability         │
                              │  orchestration.intent     │
                              │  .classify.shadow.*       │
                              │  (match / mismatch /      │
                              │   error / latency)        │
                              └──────────────────────────┘
```

## 2. 数据结构

### 2.1 LLMIntentClassifier 接口

```go
// LLMIntentClassifier is the abstract LLM interface for intent classification.
// d7 does not depend on D3 directly; any implementation (gateway, stub) is
// acceptable. Implementations should be safe for concurrent calls.
type LLMIntentClassifier interface {
    ClassifyIntent(ctx context.Context, message string) (IntentClassification, error)
}
```

### 2.2 ShadowMetrics

```go
// ShadowMetrics holds 4 counters + 1 histogram for shadow outcomes.
// Constructed via NewShadowMetrics(meter) which registers the 5 metrics
// with the D5 observability Meter. nil is a valid "disabled" state.
type ShadowMetrics struct {
    Match    *metrics.Counter    // orchestration.intent.classify.shadow.match
    Mismatch *metrics.Counter    // orchestration.intent.classify.shadow.mismatch
    Error    *metrics.Counter    // orchestration.intent.classify.shadow.error
    Disabled *metrics.Counter    // orchestration.intent.classify.shadow.disabled (no LLM configured)
    Latency  *metrics.Histogram  // orchestration.intent.classify.shadow.latency_ms
}
```

### 2.3 ShadowClassifier

```go
type ShadowClassifier struct {
    rule    IntentClassifier
    llm     LLMIntentClassifier  // may be nil
    metrics *ShadowMetrics       // may be nil
    timeout time.Duration
    log     *slog.Logger
}
```

## 3. 流程

### 3.1 Hot path（Rule 命中）

```
classifier.Classify(ctx, "hi")
  └─▶ RuleClassifier.Classify(ctx, "hi")
        └─▶ matches greeting regex
              └─▶ returns IntentFast (synchronous)
  return IntentFast  (zero shadow overhead)
```

### 3.2 Tail path（Rule 未命中）

```
classifier.Classify(ctx, "请帮我设计一个分布式缓存")
  └─▶ ShadowClassifier.Classify(ctx, msg)
        ├─▶ s.rule.Classify(ctx, msg) → IntentOrchestrate
        ├─▶ s.llm != nil && s.metrics != nil
        ├─▶ go s.shadowAsync(WithoutCancel(ctx), msg, IntentOrchestrate)
        │     ├─▶ ctx, cancel = WithTimeout(ctx, 500ms)
        │     ├─▶ llm.ClassifyIntent(ctx, msg)
        │     │     └─▶ returns IntentOrchestrate (or other)
        │     ├─▶ metrics.Latency.Record(elapsed_ms)
        │     ├─▶ if rule.Kind == llm.Kind → Match.Inc
        │     └─▶ else → Mismatch.Inc + log
        └─▶ return IntentOrchestrate (immediate)
```

### 3.3 LLM Error / Timeout

```
shadowAsync(...)
  ├─▶ llm.ClassifyIntent(ctx, msg) → error (timeout or LLM error)
  ├─▶ metrics.Error.Inc()
  └─▶ log.Warn(...)
       (no panic propagation, no caller impact)
```

## 4. 测试点

| T ID | 描述 | 优先级 | 测试位置 |
|------|------|--------|----------|
| **D7-S5-T07** | Tail-only LLM classify shadow（rule 未命中时异步 LLM，结果只入 metric） | **P0** | `internal/layers/d7/shadow_classifier_test.go` |
|   └─ T07-AC3 | rule 命中 fast/command/skip → LLM 不调用 | P0 | 同上 |
|   └─ T07-AC4 | rule 返回 orchestrate → 异步 LLM 触发 | P0 | 同上 |
|   └─ T07-AC5 | LLM 超时 → error metric + 不传播 | P0 | 同上 |
|   └─ T07-AC6 | LLM match rule → match counter | P0 | 同上 |
|   └─ T07-AC7 | LLM mismatch rule → mismatch counter + log | P0 | 同上 |
|   └─ T07-AC8 | llm=nil → no-op | P0 | 同上 |
|   └─ T07-AC9 | nil receiver 不 panic | P1 | 同上 |

## 5. 兼容性

| 项 | 影响 |
|----|------|
| `IntentClassifier` 接口 | 不变（保持向后兼容） |
| `RuleClassifier` 行为 | 完全不变 |
| `Config` 字段 | 新增 2 字段（默认 false / 500） |
| `SessionOrchestrator` 字段 | 新增 `shadowClassifier`（可为 nil） |
| `NewSessionOrchestrator` 签名 | 不变（新增 `WithShadowClassifier` option） |
| `ProcessMessage` 行为 | 当 shadow 未注入时完全等价于原行为 |
| D5 metric | 新增 4 counter + 1 histogram（orchestration.intent.classify.shadow.*） |

## 6. 不变更

- `RuleClassifier.Classify` 实现
- `FastPath` 性能预算
- D7-S2 路由矩阵（规则+command-first 仍为权威）
- 现有 D7-S5 T 测试点（D7-S5-T01~T06）
- D6 metric 增强（devrix-d6-validation-metric）的代码

## 7. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| LLM panic 影响 rule 决策 | 高 | goroutine + defer recover + error metric |
| LLM 成本失控 | 中 | 默认 disabled + tail-only（~20%） |
| 测试 flaky（LLM 异步返回顺序） | 中 | 测试用 `sync.WaitGroup` + 显式等待 |
| shadow 调用延迟反超 P99 预算 | 低 | shadow 不在 hot path 计时窗口内 |
| 跨域循环依赖（d7 ↔ D3 gateway） | 中 | 接口定义在 d7 包内；D3 gateway 反向依赖 d7 即可 |
