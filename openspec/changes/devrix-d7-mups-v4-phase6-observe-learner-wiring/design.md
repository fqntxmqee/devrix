# Design: D7 MUPS v4.3 Phase 6 — Observe-Learner 跨域闭环集成

**Change ID:** `devrix-d7-mups-v4-phase6-observe-learner-wiring`
**Demand ID:** DM-20260624-001
**Status:** S3_Design → S4_Implemented → S7_Archived
**Created:** 2026-06-24

---

## 1. 架构目标

### 1.1 业务目标

闭合 MUPS v4.3 5 节点管道的 LP-1 跨节点闭环（Observe → Learn → 下一轮 Observe），把 Phase 5 PR-E5 E5.4 (T13 PARTIAL) 延期的工作落地。具体包括：

1. **建立 Observe 子模块**（IntentQuantizer + AnomalyDetector），与 RuleClassifier 并列作为 Observer 三大子模块
2. **新增 3 WithPrior 变体**（QuantizeWithPrior / DetectWithPrior / ClassifyWithPrior），让 AdaptivePrior 真正影响 Observe 决策
3. **Orchestrator 集成 Learner**：ProcessMessage 入口前 `Inject` AdaptivePrior，注入到 `ObserveRequest.Prior`
4. **端到端 LP-1 集成测试**：5 节点管道完整跑通 + AdaptivePrior 跨 session 累积

### 1.2 技术目标

- **T 点 IMPLEMENTED**：6 P0 T 点 100% IMPLEMENTED（无 PARTIAL）
- **测试覆盖**：35+ tests，coverage ≥ 80% per file
- **并发安全**：3 Observer 子模块 + Learner.Inject 全程 race-clean
- **冷启动兜底**：prior == nil / Inject 失败 / Learner 为 nil → 用 `DefaultDeveloperPrior` 兜底，不阻塞
- **Layer lint**：orchtypes / decisionplanning / sessionorchestrator / learn 4 层依赖合法，0 import cycle
- **既有路径不变**：RuleClassifier.Classify / IntentQuantizer.Quantize / AnomalyDetector.Detect baseline 签名不变，仅新增 `*WithPrior` 变体

### 1.3 约束条件

- **跨包依赖**：sessionorchestrator 可 import learn（沿 Phase 5 precedent，learn 包 import sessionorchestrator 反向不允许）
- **失败不阻塞**：Learner.Inject 失败 → log + DefaultDeveloperPrior 兜底
- **冷启动延迟**：prior == nil → Beta(5,3) Mean=0.625，与 Phase 5 严格一致
- **可证伪沉淀**：prior.PriorBeta.Mean() 作为 confidence 乘数，可证伪（prior 越高 → confidence 越高）
- **不破坏既有 4 类行为**：FastPath / CommandHandler / OrchestratePath 既有 behavior 不变

## 2. 架构原则

| 编号 | 原则 | 落地方式 |
|------|------|---------|
| **OP-1** | **baseline 与 WithPrior 并列** | Observer 子模块的 `Classify` / `Quantize` / `Detect` 保持无 prior 路径（既有调用方零修改），新增 `*WithPrior` 变体 |
| **OP-2** | **Prior 注入统一** | 所有 Observer 子模块用 `ObserveRequest` 统一接收 prior，避免逐方法传递 |
| **OP-3** | **冷启动 + 失败兜底** | prior == nil / Inject 失败 / Learner nil → DefaultDeveloperPrior 兜底，3 层 fail-safe |
| **OP-4** | **prior 不修改** | Observer 子模块只读 prior，不修改（immutable pattern） |
| **OP-5** | **可证伪影响** | prior.PriorBeta.Mean() 作为 confidence / threshold 乘数，可证伪（prior 越高 → 越信任用户） |

## 3. 业务流程

### 3.1 LP-1 跨节点闭环时序图

```
SessionOrchestrator.ProcessMessage(ctx, req{SessionID, Message})
  │
  ├─[1]─→ o.buildObserveRequest(ctx, req)
  │       │
  │       ├─ o.learner == nil ?
  │       │   YES → Prior = learn.BuildAdaptivePrior(nil, developer) → DefaultDeveloperPrior Beta(5,3)
  │       │
  │       └─ o.learner != nil
  │           │
  │           ├─ o.learner.Inject(ctx, req.SessionID)
  │           │   │
  │           │   ├─ err != nil → log + Prior = DefaultDeveloperPrior
  │           │   └─ err == nil → Prior = injected (含 Reputation + PriorBeta)
  │           │
  │           └─ return ObserveRequest{SessionID, Message, Prior}
  │
  ├─[2]─→ o.classifier.ClassifyWithPrior(ctx, observeReq.Message, observeReq.Prior)
  │       │
  │       ├─ baseline Classify → IntentClassification{Kind, Confidence, Reason}
  │       └─ WithPrior adjustment → Confidence *= prior.PriorBeta.Mean() (clamp [0, 100])
  │
  ├─[3]─→ switch intent.Kind: Skip / Command / Fast / Orchestrate
  │
  └─[4]─→ (异步) Verify 节点完成 → Learn 节点 Learn(verdict) → BayesianUpdate → ReputationStore.Update

  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─

  下一轮 ProcessMessage:
  SessionOrchestrator.ProcessMessage(ctx, req{SessionID, Message2})
  │
  ├─[1]─→ o.buildObserveRequest(ctx, req)
  │       └─ o.learner.Inject(ctx, req.SessionID) → 拿到上轮 Learn 累积的 ReputationStore → PriorBeta = Beta(5+α, 3+β)
  │
  └─[2]─→ o.classifier.ClassifyWithPrior(message2, Prior with new PriorBeta) → confidence 受 ReputationStore 影响
```

### 3.2 异常补偿

| 异常 | 补偿 |
|------|------|
| Learner 为 nil | DefaultDeveloperPrior (Beta(5,3)) |
| Learner.Inject 返回 err | log + DefaultDeveloperPrior (Beta(5,3)) |
| Prior 为 nil | DefaultDeveloperPrior (Beta(5,3)) |
| PriorBeta.Alpha+Beta = 0 (冷启动) | Mean = 0 → 不调整 confidence |
| PriorBeta.Mean() > 1.0 | clamp confidence to 100 |
| PriorBeta.Mean() < 0.0 | clamp confidence to 0 |
| RuleClassifier.ClassifyWithPrior 内部 panic | recover → fallback to baseline Classify |

### 3.3 分支处理

- **shadow 路径**：ShadowClassifier 内部走 legacy rule（不变），但 Orchestrator 在调 shadow 之前先 buildObserveRequest，把 prior 通过 ObserveRequest 传出去（shadow 不直接读 prior，shadow 只对照 rule）
- **Command 路径**：CommandHandler 不读 prior（命令路径不依赖用户信誉）
- **Fast / Orchestrate 路径**：IntentClassifier 输出已含 prior 影响，downstream FastPath / OrchestratePath 不变

## 4. 领域模型

### 4.1 ObserveRequest 核心数据结构

```go
package orchtypes

import "github.com/devrix/devrix/internal/layers/orchestration/learn"

// ObserveRequest is the unified input to all Observer submodules
// (IntentQuantizer + AnomalyDetector + RuleClassifier). The Prior field
// is the AdaptivePrior from Learner.Inject (Phase 5 LP-1 closed loop).
type ObserveRequest struct {
    SessionID string
    Message   string
    Prior     *learn.AdaptivePrior // nullable; nil → caller uses DefaultDeveloperPrior
}

func NewObserveRequest(sessionID, message string, prior *learn.AdaptivePrior) (ObserveRequest, error) {
    if sessionID == "" {
        return ObserveRequest{}, fmt.Errorf("orchtypes: ObserveRequest.SessionID is empty")
    }
    if message == "" {
        return ObserveRequest{}, fmt.Errorf("orchtypes: ObserveRequest.Message is empty")
    }
    return ObserveRequest{
        SessionID: sessionID,
        Message:   message,
        Prior:     prior,
    }, nil
}

func (r ObserveRequest) EffectivePrior() *learn.AdaptivePrior {
    if r.Prior != nil {
        return r.Prior
    }
    return learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
}

func (r ObserveRequest) Validate() error {
    if r.SessionID == "" {
        return fmt.Errorf("orchtypes: ObserveRequest.SessionID is empty")
    }
    if r.Message == "" {
        return fmt.Errorf("orchtypes: ObserveRequest.Message is empty")
    }
    return nil
}
```

### 4.2 IntentQuantizer 子模块

```go
package orchtypes

import "github.com/devrix/devrix/internal/layers/orchestration/learn"

// IntentPayload is the 4-class quantized intent.
type IntentPayload struct {
    Kind       string  // "fact" / "command" / "orchestrate" / "skip"
    Confidence int     // 0-100
    Reason     string
    Class      string  // optional sub-class
}

// IntentQuantizer quantizes raw messages into 4 IntentKind classes.
// Concurrency-safe (no internal state).
type IntentQuantizer struct {
    fastPatterns   []fastRule
    emptyPattern   *regexp.Regexp
    commandRegexes []*regexp.Regexp
}

func NewIntentQuantizer(cfg *Config) *IntentQuantizer { ... }

// Quantize is the baseline (no prior).
func (q *IntentQuantizer) Quantize(ctx context.Context, message string) (IntentPayload, error) { ... }

// QuantizeWithPrior applies AdaptivePrior.PriorBeta.Mean() as a confidence multiplier.
// prior == nil → Quantize (baseline).
func (q *IntentQuantizer) QuantizeWithPrior(ctx context.Context, message string, prior *learn.AdaptivePrior) (IntentPayload, error) {
    payload, err := q.Quantize(ctx, message)
    if err != nil {
        return payload, err
    }
    if prior == nil {
        return payload, nil
    }
    mean := prior.PriorBeta.Mean()
    if mean == 0 {
        return payload, nil
    }
    adjusted := int(float64(payload.Confidence) * mean)
    if adjusted > 100 {
        adjusted = 100
    }
    if adjusted < 0 {
        adjusted = 0
    }
    payload.Confidence = adjusted
    payload.Reason = fmt.Sprintf("%s [prior.Mean=%.3f]", payload.Reason, mean)
    return payload, nil
}
```

### 4.3 AnomalyDetector 子模块

```go
package orchtypes

import "github.com/devrix/devrix/internal/layers/orchestration/learn"

// Anomaly is an atomic anomaly record.
type Anomaly struct {
    Category string
    Severity float64 // 0-1
    Evidence string
}

// AnomalyReport aggregates anomaly detection results.
type AnomalyReport struct {
    Anomalies               []Anomaly
    TriggeredSystemAnomaly  bool
    Severity                float64
}

// AnomalyDetector detects anomalies from message + history.
type AnomalyDetector struct {
    threshold float64 // baseline
}

func NewAnomalyDetector() *AnomalyDetector {
    return &AnomalyDetector{threshold: 0.5}
}

// HistoricalDetector is the history-aware detector.
func (d *AnomalyDetector) HistoricalDetector() *AnomalyDetector { return d }

// Detect is the baseline.
func (d *AnomalyDetector) Detect(ctx context.Context, anomalies []Anomaly) (AnomalyReport, error) { ... }

// DetectWithPrior applies AdaptivePrior.PriorBeta.Mean() to threshold.
// prior.Mean() higher → threshold higher (more trust → fewer false positives).
func (d *AnomalyDetector) DetectWithPrior(ctx context.Context, anomalies []Anomaly, prior *learn.AdaptivePrior) (AnomalyReport, error) {
    threshold := d.threshold
    if prior != nil {
        m := prior.PriorBeta.Mean()
        if m > 0 {
            threshold = 0.5 * m // Beta(5,3) Mean=0.625 → threshold=0.3125; Beta(8,1) Mean=0.889 → threshold=0.444
        }
    }
    // ... apply threshold
}
```

### 4.4 RuleClassifier.ClassifyWithPrior

```go
// ClassifyWithPrior applies AdaptivePrior.PriorBeta.Mean() as a confidence multiplier.
func (c *RuleClassifier) ClassifyWithPrior(_ context.Context, message string, prior *learn.AdaptivePrior) (orchtypes.IntentClassification, error) {
    result, err := c.Classify(context.Background(), message)
    if err != nil {
        return result, err
    }
    if prior == nil {
        return result, nil
    }
    mean := prior.PriorBeta.Mean()
    if mean == 0 {
        return result, nil
    }
    adjusted := int(float64(result.Confidence) * mean)
    if adjusted > 100 {
        adjusted = 100
    }
    if adjusted < 0 {
        adjusted = 0
    }
    result.Confidence = adjusted
    result.Reason = fmt.Sprintf("%s [prior.Mean=%.3f]", result.Reason, mean)
    return result, nil
}
```

### 4.5 SessionOrchestrator.buildObserveRequest

```go
// buildObserveRequest constructs the ObserveRequest with AdaptivePrior injection.
// Fail-safe: prior == nil / Inject error / learner == nil → DefaultDeveloperPrior.
func (o *SessionOrchestrator) buildObserveRequest(ctx context.Context, req orchtypes.ProcessRequest) (orchtypes.ObserveRequest, error) {
    var prior *learn.AdaptivePrior
    if o.learner != nil {
        injected, err := o.learner.Inject(ctx, req.SessionID)
        if err != nil {
            o.log.Warn("orchestrator: learner.Inject failed, using DefaultDeveloperPrior",
                "session_id", req.SessionID, "err", err)
        } else {
            prior = injected
        }
    }
    return orchtypes.NewObserveRequest(req.SessionID, req.Message, prior)
}
```

## 5. 核心链路

### 5.1 D7 内部链路

```
D1 (通信层) → SessionOrchestrator.ProcessMessage
  │
  ├─[1]─→ buildObserveRequest(ctx, req)
  │        └─ learner.Inject(ctx, req.SessionID) → AdaptivePrior
  │
  ├─[2]─→ Classify (RuleClassifier.ClassifyWithPrior)  ← Phase 6 PR-F1
  │        └─ baseline Classify + prior.Mean() 调整 confidence
  │
  ├─[3]─→ intent switch:
  │        ├─ Skip        → close channel
  │        ├─ Command     → CommandHandler.Handle
  │        ├─ Fast        → FastPath.Run
  │        └─ Orchestrate → OrchestratePath.Run
  │
  └─[4]─→ (异步) 后续 Verify → Learn → ReputationStore → 下一轮 buildObserveRequest 拿到新 prior
```

### 5.2 跨域依赖（D7 ↔ Learn）

- **sessionorchestrator → learn**：`learn.Learner` interface + `learn.AdaptivePrior` + `learn.BuildAdaptivePrior`
- **orchtypes → learn**：`learn.AdaptivePrior` 用作 `ObserveRequest.Prior` 字段类型
- **decisionplanning → learn**：`RuleClassifier.ClassifyWithPrior(ctx, message, *learn.AdaptivePrior)`
- **反向不允许**：learn 不能 import sessionorchestrator / orchtypes / decisionplanning（避免 import cycle）

## 6. 接口/API 设计

### 6.1 对外契约

| 接口 | 方法 | 失败模式 |
|------|------|---------|
| `learn.Learner.Inject` | `Inject(ctx, sessionID) (*AdaptivePrior, error)` | `ErrAdaptivePriorNotReady` (sessionID 空) |
| `orchtypes.ObserveRequest.Validate` | `Validate() error` | `SessionID` 或 `Message` 为空 |
| `orchtypes.IntentQuantizer.QuantizeWithPrior` | `QuantizeWithPrior(ctx, message, prior) (IntentPayload, error)` | `prior == nil` → baseline |
| `orchtypes.AnomalyDetector.DetectWithPrior` | `DetectWithPrior(ctx, anomalies, prior) (AnomalyReport, error)` | `prior == nil` → baseline |
| `decisionplanning.RuleClassifier.ClassifyWithPrior` | `ClassifyWithPrior(ctx, message, prior) (IntentClassification, error)` | `prior == nil` → baseline |
| `SessionOrchestrator.WithLearner` | `WithLearner(learn.Learner) OrchestratorOption` | nil learner → 不注入 |

### 6.2 Sentinel Errors

- `learn.ErrAdaptivePriorNotReady` (Phase 5 已有，sessionID 为空)
- `learn.ErrReputationStoreUnavailable` (Phase 5 已有)

新增：
- 无（Phase 6 不新增 SentinelError，复用 Phase 5 的）

### 6.3 配置项

- `Config.AdvisoryValidationTimeoutMs` (已有)
- 不新增 CFG（Phase 6 复用既有 config）

### 6.4 度量指标

- `orchestration.learn.inject.latency_ms` (D5 Histogram, Phase 6 新增)
- `orchestration.learn.inject.error` (D5 Counter, Phase 6 新增)
- `orchestration.observe.prior.applied` (D5 Counter, Phase 6 新增)
- `orchestration.observe.prior.nil_fallback` (D5 Counter, Phase 6 新增)

## 7. D7 代码现状对照

### 7.1 已有

| 模块 | 文件 | 状态 |
|------|------|------|
| `learn.Learner` interface | `learn/learner.go` | ✅ Phase 5 PR-E5 |
| `learn.DefaultLearner` | `learn/learner.go` | ✅ Phase 5 PR-E5 |
| `learn.AdaptivePrior` + `BuildAdaptivePrior` | `learn/adaptive_prior.go` | ✅ Phase 5 PR-E3 |
| `learn.InMemoryReputationStore` | `learn/reputation_store.go` | ✅ Phase 5 PR-E5 |
| `decisionplanning.RuleClassifier.Classify` | `decisionplanning/classifier.go` | ✅ Phase 1 Foundation |
| `SessionOrchestrator` | `sessionorchestrator/orchestrator.go` | ✅ Phase 1 Foundation |

### 7.2 缺失（Phase 6 新增）

| 模块 | 缺失原因 |
|------|---------|
| `orchtypes.ObserveRequest` | 缺统一 Observe 输入 |
| `orchtypes.IntentQuantizer` | Phase 5 设计有但未实现 |
| `orchtypes.AnomalyDetector` | Phase 5 设计有但未实现 |
| `RuleClassifier.ClassifyWithPrior` | 缺 WithPrior 变体 |
| `SessionOrchestrator.learner` 字段 | Orchestrator 未集成 Learner |
| `SessionOrchestrator.WithLearner` option | 缺 option 入口 |
| `SessionOrchestrator.buildObserveRequest` | 缺 Inject 调用 |

### 7.3 不一致（需重构）

- 无（Phase 6 纯增量，不重构既有代码）

## 8. 实施路径

### 8.1 工作量（人天）

| PR | 范围 | 工作量 |
|----|------|--------|
| PR-F1 | Observer 子模块 + WithPrior 变体 | 1.5 天 |
| PR-F2 | Orchestrator 集成 Learner | 1 天 |
| PR-F3 | E2E LP-1 闭环集成测试 | 0.5 天 |
| S6 Archive | 归档 | 0.3 天 |
| **总计** | — | **3.3 天** |

### 8.2 PR 拆分

- **PR-F1**（D7-S12-A41）：ObserveRequest + IntentQuantizer + AnomalyDetector + RuleClassifier.ClassifyWithPrior（5 NEW + 1 MODIFIED +700/-0）
- **PR-F2**（D7-S12-A42）：Orchestrator 集成 Learner（1 NEW + 1 MODIFIED +200/-30）
- **PR-F3**（D7-S12-A43）：E2E LP-1 闭环集成测试（1 NEW +800/-0）
- **S6 Archive**：6 NEW（含 manifest + acceptance-report + spec snapshot）

### 8.3 验收标准

- 6 P0 T 点 100% IMPLEMENTED（D7-S12-A41/42/43-T01..T06）
- 35+ tests 100% PASS 含 race
- coverage ≥ 80% per file
- 0 import cycle / 0 layer lint violation
- E2E LP-1 闭环测试覆盖 5 节点管道 + AdaptivePrior 跨 session 累积
- go vet / go build 0 issue

## 9. Cross-references

- doc 35 §三.1 (Observe 节点方法论)
- doc 46 §五.1 (AdaptivePrior 传递路径)
- doc 37 §2.1-2.6 (5 节点数据模型)
- doc 25 §四 (Developer/Operator 默认先验)
- Phase 5 PR-E5 E5.4 (D7-S11-A40-T13 PARTIAL 延期到 Phase 6)
- Phase 5 design.md (Learn 节点 + ReputationEvidence + AdaptivePrior 落地)
- Phase 5 acceptance-report.md §2.4 LP-1 闭环行为正确性
- Phase 4 G8-1 P0-3 修复 (parse failure → INDETERMINATE)
