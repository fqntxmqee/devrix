---
proposal-id: devrix-s5-p2-shadow-classifier
title: S5-P2 Tail-only LLM Classify Shadow — 提案
demand-id: DM-20260614-004
status: S2_Proposal
created: 2026-06-14
last-updated: 2026-06-14
---

# S5-P2 Tail-only LLM Classify Shadow — 提案

## 1. 方案概览

| 方案 | 概述 | Tail-only | 异步 | Hot path 零成本 | 决策 |
|------|------|-----------|------|----------------|------|
| A. 同步 LLM + 短超时 | rule + LLM 串行，LLM 50ms 超时 | ✅ | ❌ | ❌ | ❌ |
| **B. 异步 tail-only shadow** | rule 决策后，tail 时异步触发 LLM | ✅ | ✅ | ✅ | ✅ |
| C. 全量同步 LLM | rule + LLM 串行，结果合并 | ❌ | ❌ | ❌ | ❌ |

**决议**：选 **B**。理由：
- B 满足 R2 §5 命题 C 决议："仅对规则未命中 tail（~20%）异步 LLM classify"
- 不影响 FastPath P99 ≤ 2ms 性能预算
- 决策路径不变（v1.0 仍为规则权威）
- 默认 disabled，热路径零成本

## 2. 方案 B 详细设计

### 2.1 接口定义

```go
// LLMIntentClassifier is the abstract LLM interface for intent classification.
// d7 does not depend on D3 directly; the gateway or any stub can implement.
type LLMIntentClassifier interface {
    ClassifyIntent(ctx context.Context, message string) (IntentClassification, error)
}
```

### 2.2 ShadowClassifier 包装

```go
// ShadowMetrics counts shadow outcomes and measures latency.
type ShadowMetrics struct {
    Match    *metrics.Counter  // orchestration.intent.classify.shadow.match
    Mismatch *metrics.Counter  // orchestration.intent.classify.shadow.mismatch
    Error    *metrics.Counter  // orchestration.intent.classify.shadow.error
    Disabled *metrics.Counter  // orchestration.intent.classify.shadow.disabled
    Latency  *metrics.Histogram  // orchestration.intent.classify.shadow.latency_ms
}

// ShadowClassifier wraps a rule-based classifier with an optional LLM
// "shadow" that runs asynchronously when the rule returns IntentOrchestrate
// (the ~20% tail of messages the rule does not fast-match).
type ShadowClassifier struct {
    rule    IntentClassifier
    llm     LLMIntentClassifier  // may be nil when disabled
    metrics *ShadowMetrics
    timeout time.Duration
    log     *slog.Logger
}

// NewShadowClassifier builds a ShadowClassifier. If llm is nil or
// metrics is nil, the shadow path is a no-op (rule result returned as-is).
func NewShadowClassifier(rule IntentClassifier, llm LLMIntentClassifier, metrics *ShadowMetrics, timeout time.Duration) *ShadowClassifier {
    if timeout <= 0 {
        timeout = 500 * time.Millisecond
    }
    return &ShadowClassifier{rule: rule, llm: llm, metrics: metrics, timeout: timeout}
}

// Classify returns the rule's decision synchronously; the LLM shadow
// runs in a background goroutine when the rule falls through to
// IntentOrchestrate. The LLM result is recorded in metrics + log
// but never influences the return value.
func (s *ShadowClassifier) Classify(ctx context.Context, message string) (IntentClassification, error) {
    result, err := s.rule.Classify(ctx, message)
    if err != nil {
        return result, err
    }
    if s.llm == nil || s.metrics == nil {
        return result, nil
    }
    if result.Kind != IntentOrchestrate {
        // tail-only: only when rule fell through
        return result, nil
    }
    // Fire-and-forget: detached context so request cancellation does not abort shadow.
    go s.shadowAsync(context.WithoutCancel(ctx), message, result)
    return result, nil
}

// shadowAsync is the goroutine body. It never panics out.
func (s *ShadowClassifier) shadowAsync(ctx context.Context, message string, ruleResult IntentClassification) {
    defer func() {
        if r := recover(); r != nil {
            s.log.Error("d7: shadow classify panic recovered", "panic", r)
            s.metrics.Error.Inc()
        }
    }()
    ctx, cancel := context.WithTimeout(ctx, s.timeout)
    defer cancel()
    start := time.Now()
    llmResult, err := s.llm.ClassifyIntent(ctx, message)
    elapsed := time.Since(start)
    s.metrics.Latency.Record(elapsed.Milliseconds())
    if err != nil {
        s.metrics.Error.Inc()
        s.log.Warn("d7: shadow classify error", "err", err, "latency_ms", elapsed.Milliseconds())
        return
    }
    if llmResult.Kind == ruleResult.Kind {
        s.metrics.Match.Inc()
    } else {
        s.metrics.Mismatch.Inc()
        s.log.Info("d7: shadow mismatch",
            "rule_kind", ruleResult.Kind, "rule_conf", ruleResult.Confidence,
            "llm_kind", llmResult.Kind, "llm_conf", llmResult.Confidence)
    }
}
```

### 2.3 配置项

```go
type Config struct {
    // ... existing fields ...
    // ShadowLLMClassify enables async LLM classify shadow for the
    // IntentOrchestrate tail (the ~20% rule falls through to). Default
    // false; enable per-deployment to gather v1.1 cold-start samples.
    ShadowLLMClassify bool
    // ShadowLLMTimeoutMs is the LLM call timeout in milliseconds. Default 500.
    ShadowLLMTimeoutMs int
}
```

### 2.4 Orchestrator 接入

```go
type SessionOrchestrator struct {
    // ... existing fields ...
    shadowClassifier *ShadowClassifier  // may be nil
}

// WithShadowClassifier wires the optional LLM shadow.
func WithShadowClassifier(s *ShadowClassifier) OrchestratorOption {
    return func(o *SessionOrchestrator) { o.shadowClassifier = s }
}

// In ProcessMessage, when classifier is the shadow:
if o.shadowClassifier != nil {
    result, err = o.shadowClassifier.Classify(ctx, req.Message)
} else {
    result, err = o.classifier.Classify(ctx, req.Message)
}
```

## 3. 备选方案

### 3.1 方案 A：同步 LLM + 短超时

- 实施：rule + LLM 串行，LLM 50ms 超时
- 缺点：
  - 50ms LLM 不可靠（多数 LLM 至少 200ms）
  - 拖慢 IntentOrchestrate 路径
  - 不符合"shadow 零成本"原则

### 3.2 方案 C：全量同步 LLM

- 实施：所有消息都跑 LLM classify
- 缺点：
  - 80% 流量被规则匹配的也被 LLM 重复分类
  - LLM 成本 × 5
  - 破坏 R2 决议

## 4. 关键决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 默认 enabled / disabled | **disabled** | 避免对未明确开启 shadow 的部署产生 LLM 成本 |
| 触发条件 | **rule returns IntentOrchestrate** | R2 §5 命题 C 决议 |
| 调用方式 | **goroutine fire-and-forget** | 不阻塞 rule 决策路径 |
| 上下文 | **WithoutCancel(parent)** | 请求取消不影响 shadow |
| 超时 | **500ms** | LLM 95p 通常 < 500ms |
| 错误处理 | **只入 metric + log，不传播** | shadow 不影响决策路径 |
| nil llm | **no-op** | 默认 disabled 状态；调用方不强制注入 |
| 接口定义位置 | **d7 包内** | 与 orchestrator 同包，避免循环依赖 |

## 5. 实施计划

| 阶段 | 估算 | 备注 |
|------|------|------|
| S3 设计 | 30 分钟 | proposal + design |
| S3-Gate | 15 分钟 | 内部 review |
| S4 实现 | 2 小时 | shadow_classifier.go + config.go + orchestrator.go + 9 测试 |
| S4-Gate | 15 分钟 | review-code.md |
| S5 验收 | 30 分钟 | acceptance-report.md |
| S6 归档 | 10 分钟 | 移动 archive |

总计约 3.5 小时。
