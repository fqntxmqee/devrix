# Design: D7 MUPS v4.3 Phase 7 — Verify→Learn Auto-Close + Operator TrackMode + D5 增强

**Change ID:** `devrix-d7-mups-v4-phase7-verify-auto-close`
**Demand ID:** DM-20260625-001
**Status:** S3_Design → S4_Implemented → S7_Archived
**Created:** 2026-06-25

---

## 1. 架构目标

### 1.1 业务目标

闭合 MUPS v4.3 5 节点管道的**运行时 LP-1 闭环**（生产代码自动触发 Learn）+ Operator 角色 TrackMode 切换 + D5 Trace 增强 6 字段。具体包括：

1. **Auto-Close 包装函数** `processAutoClose`：监听 FastPath/CommandHandler/OrchestratePath 的 channel 终端事件 → 合成 Verdict → 异步触发 `learner.Learn` → BayesianUpdate + ReputationStore 更新
2. **Operator TrackMode 字段** `ProcessRequest.TrackMode` + `buildObserveRequest` 透传 → `learn.BuildAdaptivePrior` 区分 Developer/Operator
3. **D5 sessionSpan 4 新增属性**：mean / track_mode / classifier_source / injected_at，trace 排查一目了然
4. **3 层 fail-safe**：nil learner / Learn error / channel 提前关闭 → log warning + skip, 不阻塞 caller

### 1.2 技术目标

- **T 点 IMPLEMENTED**：6 P0 T 点 100% IMPLEMENTED（无 PARTIAL）
- **测试覆盖**：9+ tests（PR-7.1: 3 unit + 2 integration / PR-7.2: 2 unit / PR-7.3: 2 trace），coverage ≥ 80% per file
- **并发安全**：processAutoClose 异步 goroutine 全程 race-clean
- **冷启动兜底**：nil learner / Inject error / Learn error → log + skip, 不阻塞 ProcessMessage
- **Layer lint**：sessionorchestrator / orchtypes / learn 3 层依赖合法, 0 import cycle
- **既有路径不变**：ProcessMessage 同步返回语义不变, endSpanWhenChannelClosed 行为不变, baseline Classify 路径不变

### 1.3 约束条件

- **跨包依赖**：sessionorchestrator 可 import learn (Phase 5 precedent), learn 不能 import sessionorchestrator
- **失败不阻塞**：Auto-Close 失败 → log + skip, ProcessMessage 同步返回不受影响
- **冷启动延迟**：prior == nil → Beta(5,3) Mean=0.625, 与 Phase 6 严格一致
- **Verdict 合成规则**：
  - `complete` → `VerdictPass` (Success)
  - `error` → `VerdictFail` (Failed, Reason = event.Content)
  - `tombstone` → `VerdictIndeterminate` (IndeterminateReason = "interrupt")
  - `skip` 路径 (IntentSkip) → 不触发 Learn
- **Operator 角色**：高自信阈值, Beta(8,1) Mean=0.889, 用于运维类会话

## 2. 架构原则

| 编号 | 原则 | 落地方式 |
|------|------|---------|
| **OP-1** | **Auto-Close 异步非阻塞** | processAutoClose 包装 channel, 内部 goroutine 异步调用 Learn, 同步立即返回 channel proxy |
| **OP-2** | **3 层 fail-safe** | nil learner / Learn error / channel 提前关闭 → log warning + skip, 永不阻塞 caller |
| **OP-3** | **Verdict 合成规则明确** | EngineEvent.Type → workmodel.VerdictKind 映射表, 测试 pinned 防止漂移 |
| **OP-4** | **TrackMode 字段向后兼容** | ProcessRequest.TrackMode 用 `string` 类型 + 空值兜底 developer, 旧调用方零修改 |
| **OP-5** | **D5 完整 prior 语义** | sessionSpan 6 字段 (alpha/beta/mean/track_mode/classifier_source/injected_at) 一气呵成 |
| **OP-6** | **skip 路径不学** | IntentSkip 路由直连 close channel, processAutoClose 不被调用, 不触发 Learn |
| **OP-7** | **Plan/Artifact 反向追溯 (Phase 7 v1.0 暂不实现)** | LearnRequest.Plan/Artifact/Observations 暂时为 nil, 由 PR-7.4+ 后续补 |

## 3. 业务流程

### 3.1 Auto-Close 运行时 LP-1 闭环时序图

```
SessionOrchestrator.ProcessMessage(ctx, req{SessionID, Message, TrackMode})
  │
  ├─[1]─→ o.buildObserveRequest(ctx, req)        (Phase 6 PR-F2, 不变)
  │       │
  │       ├─ learner.Inject (with TrackMode="operator")
  │       └─ return ObserveRequest{Prior: AdaptivePrior{Beta(8,1) Mean=0.889}}
  │
  ├─[2]─→ classifier.ClassifyWithPrior            (Phase 6 PR-F2, 不变)
  │       → IntentFast, Confidence = 95 × 0.889 = 84
  │
  ├─[3]─→ FastPath.Run                            (Phase 3, 不变)
  │       → channel emit events: thinking / text / complete
  │       → close channel
  │
  └─[4]─→ processAutoClose(ch, sessionCtx, ...)   (PR-7.1 新增)
          │
          ├─ 启动 goroutine, 透传 channel
          │   │
          │   ├─ 监听最后事件 Type="complete" → VerdictPass
          │   ├─ 构造 LearnRequest{Verdict: VerdictPass, SessionID}
          │   └─ o.learner.Learn(ctx, req)
          │       │
          │       ├─ BuildAdaptivePrior(verdict) → 5 类 Content 分发
          │       ├─ BayesianUpdate(prior, verdict) → Alpha++
          │       └─ ReputationStore.Update(next)
          │
          └─ (同步立即返回 channel proxy)
              caller 消费 events → 关闭 → goroutine 退出
```

### 3.2 异常补偿

| 失败模式 | 行为 | 日志 | ProcessMessage 影响 |
|---------|------|------|---------------------|
| `o.learner == nil` | processAutoClose 跳过, 透传 channel | none | 0 |
| `o.learner != nil`, Learn 返回 error | log warning, 透传 channel | slog.Warn | 0 |
| channel 提前关闭 (context cancel) | log warning, skip Learn | slog.Warn | 0 |
| 整段 path context 取消 | 同上 | slog.Warn | 0 |
| EngineEvent 包含 error event | 合成 VerdictFail, 触发 Learn | none (正常) | 0 |
| EngineEvent 包含 tombstone event | 合成 VerdictIndeterminate, 触发 Learn | none (正常) | 0 |

### 3.3 分支处理

| IntentKind | 路径 | processAutoClose 行为 |
|-----------|------|----------------------|
| IntentSkip | close channel (orchestrator.go:374-376) | **不调用** (无执行结果) |
| IntentCommand | CommandHandler.Handle | 调用, 监听最后事件合成 Verdict |
| IntentFast | FastPath.Run | 调用, 监听最后事件合成 Verdict |
| IntentOrchestrate | OrchestratePath.Run | 调用, 监听最后事件合成 Verdict |

## 4. 领域模型

### 4.1 核心数据结构（含 Go 代码骨架）

```go
// === orchtypes/process.go ===
// ProcessRequest is the D1 gateway-facing request type.
//
// v1.1.0+ Phase 7 PR-7.2: TrackMode field added for Operator role support.
// Empty string ("") → defaults to TrackModeDeveloper (Phase 5 cold-start prior).
type ProcessRequest struct {
    SessionID string
    Message   string
    TrackMode string  // NEW Phase 7 PR-7.2: "developer" / "operator" / "" (default developer)
    Metadata  map[string]string
}

// === sessionorchestrator/orchestrator.go ===
// processAutoClose wraps the path-returned channel and asynchronously triggers
// learner.Learn after channel close, based on the last EngineEvent's Type.
//
// Returns a proxy channel that the caller consumes normally; the actual Learn
// call happens in a background goroutine (fail-safe: errors are logged but do
// not propagate to the caller).
func (o *SessionOrchestrator) processAutoClose(
    ch <-chan *contracts.EngineEvent,
    sessionCtx context.Context,
    sessionID string,
    intent orchtypes.IntentClassification,
) <-chan *contracts.EngineEvent {
    if o.learner == nil {
        return endSpanWhenChannelClosed(ch, nil)  // no-op passthrough
    }
    out := make(chan *contracts.EngineEvent, 32)
    go func() {
        defer close(out)
        var lastEvent *contracts.EngineEvent
        for ev := range ch {
            lastEvent = ev
            out <- ev
        }
        // Channel closed → synthesize Verdict + fire Learn
        if lastEvent == nil {
            return  // empty channel (skip path, no events)
        }
        verdict := synthesizeVerdict(lastEvent, sessionID)
        if verdict == nil {
            return  // skip path or unsupported event type
        }
        req := learn.LearnRequest{
            SessionID: sessionID,
            Verdict:   *verdict,
        }
        if _, err := o.learner.Learn(sessionCtx, req); err != nil {
            slog.Warn("orchestrator: processAutoClose learner.Learn failed",
                "session_id", sessionID, "verdict_kind", verdict.Kind, "err", err)
        }
    }()
    return out
}

// synthesizeVerdict maps EngineEvent.Type → workmodel.Verdict.
// Returns nil for skip path or unsupported event types.
func synthesizeVerdict(last *contracts.EngineEvent, sessionID string) *workmodel.Verdict {
    switch last.Type {
    case "complete":
        return &workmodel.Verdict{
            Kind:     types.VerdictPass,
            SourceID: fmt.Sprintf("autoclose:%s:%d", sessionID, time.Now().UnixNano()),
            Reason:   "process complete",
        }
    case "error":
        return &workmodel.Verdict{
            Kind:     types.VerdictFail,
            SourceID: fmt.Sprintf("autoclose:%s:%d", sessionID, time.Now().UnixNano()),
            Reason:   last.Content,
        }
    case "tombstone":
        return &workmodel.Verdict{
            Kind:                types.VerdictIndeterminate,
            SourceID:            fmt.Sprintf("autoclose:%s:%d", sessionID, time.Now().UnixNano()),
            IndeterminateReason: "interrupt",
        }
    default:
        // text / thinking / tool_call / tool_result / status / permission
        // → not a terminal event, no Verdict
        return nil
    }
}
```

### 4.2 ProcessRequest 关键字段

| 字段 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|
| SessionID | string | 是 | "" | 反向追溯 + ReputationStore 键 |
| Message | string | 是 | "" | 用户消息 |
| TrackMode | string | 否 | "" | Phase 7 PR-7.2 新增, "developer"/"operator"/空 |
| Metadata | map | 否 | nil | D1 gateway 透传 |

### 4.3 sessionSpan 6 字段

| Key | Type | Example | Phase | 说明 |
|-----|------|---------|-------|------|
| `learn.prior.alpha` | int | 5 | 6 | Beta prior Alpha |
| `learn.prior.beta` | int | 3 | 6 | Beta prior Beta |
| `learn.prior.mean` | float | 0.625 | 7 PR-7.3 | Alpha / (Alpha + Beta) |
| `learn.prior.track_mode` | string | "developer" | 7 PR-7.3 | developer / operator |
| `learn.classifier_source` | string | "rule" | 7 PR-7.3 | rule / shadow |
| `learn.prior.injected_at` | string | "phase6_lp1" | 7 PR-7.3 | phase6_lp1 / cold_start_failsafe |

## 5. 核心链路

### 5.1 D7 内部链路

```
ProcessMessage 入口
  ├─ buildObserveRequest → Learner.Inject → AdaptivePrior{TrackMode}
  ├─ classifier.ClassifyWithPrior → IntentClassification
  ├─ 路由 (Skip/Command/Fast/Orchestrate)
  └─ processAutoClose (NEW) ← 替换 endSpanWhenChannelClosed
      │
      ├─ goroutine 监听 channel events
      ├─ 最后事件 Type → synthesizeVerdict
      ├─ learner.Learn(LearnRequest{Verdict, SessionID}) 异步
      └─ BayesianUpdate → ReputationStore.Update
```

### 5.2 跨域依赖

- **D1 通信层** (D7 邻居): `ProcessRequestContract(ctx, sessionID, message)` 默认 `TrackMode=""`, 兼容旧调用
- **D5 可观测性**: sessionSpan 4 新增 attribute, Jaeger UI 自然支持
- **D2 上下文引擎** (D7 邻居): 无影响 (Learn 节点跨域, 不动 D2)
- **D4 多智能体** (D7 邻居): 无影响 (Execute 节点内部)

## 6. 接口/API 设计

### 6.1 对外契约（接口 + Sentinel Errors）

- `ProcessRequest.TrackMode` (string) — 新增字段, 兼容旧调用
- `processAutoClose` (private method) — 包装 channel + 异步 Learn, 不暴露 public API
- `synthesizeVerdict` (private function) — EngineEvent → Verdict 合成, pinned by test

无新 Sentinel Error, 沿用 Phase 5 Learn 包 5 个 SentinelError:
- `ErrAssetIncomplete` (SessionID 空)
- `ErrAssetClassMismatch` (class 不匹配)
- `ErrAssetBuildFailed` (构造失败)
- `ErrReputationStoreUnavailable` (store 故障)
- `ErrAdaptivePriorNotReady` (prior 缺失)

### 6.2 配置项（CFG）

无新增配置。TrackMode 通过 ProcessRequest 字段传递, 飞书适配器层 (D1) 决定。

### 6.3 度量指标（Metrics）

无新增 metrics。沿用 Phase 5 已有 ReputationStore metrics + Phase 6 prior.alpha/beta span attribute。

## 7. D7 代码现状对照

### 7.1 已有（Phase 6 落地）

- ✅ `SessionOrchestrator.learner` 字段 (orchestrator.go:67) — Phase 6 PR-F2
- ✅ `SessionOrchestrator.WithLearner` option (orchestrator.go:167) — Phase 6 PR-F2
- ✅ `SessionOrchestrator.buildObserveRequest` 方法 (orchestrator.go:255) — Phase 6 PR-F2
- ✅ sessionSpan `learn.prior.alpha/beta` attribute (orchestrator.go:307-310) — Phase 6 PR-F2
- ✅ `learn.DefaultLearner` (learner.go:54) — Phase 5
- ✅ `learn.BuildAdaptivePrior(rep, trackMode)` (reputation_evidence.go) — Phase 5
- ✅ `DefaultDeveloperPrior Beta(5,3)` + `DefaultOperatorPrior Beta(8,1)` — Phase 5

### 7.2 缺失（需要新增）

- ❌ `ProcessRequest.TrackMode` 字段 (orchtypes/process.go)
- ❌ `SessionOrchestrator.processAutoClose` 方法
- ❌ `synthesizeVerdict` 私有函数
- ❌ sessionSpan 4 新增 attribute (mean/track_mode/classifier_source/injected_at)

### 7.3 不一致（需要重构）

- ⚠️ `endSpanWhenChannelClosed` (tracing.go:95) — processAutoClose 内部嵌套调用, span 关闭时机不变

## 8. 实施路径

### 8.1 工作量估算

| PR | T 点 | 文件 | LOC | 风险 | 工作量 |
|----|----|------|-----|------|--------|
| PR-7.1 | 3 T (D7-S13-A47-T01/T02/T03) | 2 MODIFIED + 2 NEW | +200/-30 | Medium | 2 天 |
| PR-7.2 | 2 T (D7-S13-A48-T04/T05) | 2 MODIFIED + 1 NEW | +50/-5 | Low | 0.5 天 |
| PR-7.3 | 1 T (D7-S13-A49-T06) | 1 MODIFIED + 1 NEW | +60/-0 | Low | 0.5 天 |
| **总计** | **6 T 点** | **5 MODIFIED + 4 NEW** | **+310/-35** | — | **3 天** |

### 8.2 PR 拆分

| PR | 标题 | 范围 | 验收 |
|----|------|------|------|
| **PR-7.1** | Verify→Learn Auto-Close | processAutoClose + synthesizeVerdict + 集成测试 | 3 T 点 IMPLEMENTED, AC1+AC2+AC3 |
| **PR-7.2** | Operator TrackMode | ProcessRequest.TrackMode + buildObserveRequest 透传 | 2 T 点 IMPLEMENTED, AC4+AC5 |
| **PR-7.3** | D5 可观测化增强 | sessionSpan 4 新增 attribute | 1 T 点 IMPLEMENTED, AC6 |

### 8.3 验收标准

- **6 个 P0 T 点全部 IMPLEMENTED** (T01-T06)
- **go test -race ./...** 全部通过
- **Layer lint** 0 import cycle
- **3 层 fail-safe 单元测试**: nil learner / Learn error / channel cancel 3 种全部 log + skip
- **集成测试**: TestProcessMessage_Verify2Learn_AutoClose_PassAlpha 跑通 (Pass × 3 → Alpha=3 → 下一轮 prior Beta(8,3))
- **D5 trace 测试**: TestSessionSpan_Attributes_AllPriorFields 6 字段全部写入

## 9. 不在本次任务范围

- ❌ D2 ContextEngine-backed ReputationStore 持久化 (P2 backlog, 太大需要独立 Change)
- ❌ ShadowClassifier.ClassifyWithPrior 异步 LLM 重新启用 (Phase 6 design gap, P2 backlog)
- ❌ Phase 6 P2/P3 audit closure (D5 span attribute 测试 + ctx cancellation + error propagation + 5 SentinelError 集成, 独立 hotfix)
- ❌ Verifier 子 agent 显式注入 (Phase 4 D7-S10-A33-T03 路径级, 不在 Orchestrator 层)
- ❌ Plan / Artifact 反向追溯 (Phase 5 LP-5 已就绪, 不需要 Phase 7 重复)
- ❌ Auto-Close 异步超时配置 (Phase 7 v1.0 hardcode 5s, 由 PR-7.4+ 后续补)

## 10. Cross-references

- **Phase 5 PR-E5** (D7-S11-A40-T10) — Learner interface + DefaultLearner + BayesianUpdate (上游)
- **Phase 6 PR-F2** (D7-S12-A42-T04) — WithLearner option + buildObserveRequest (上游)
- **Phase 5 PR-E5 E5.3** (D7-S11-A38-T07) — DefaultOperatorPrior + BuildAdaptivePrior (上游)
- **Phase 4 PR-D** (D7-S10-A32-A35) — Verifier 节点升格 (上游, 路径级不聚合)
- **doc 35 §三.4** — 5 节点运行时闭环 (理论)
- **doc 46 §五.2** — Learn 节点依赖契约 (理论)
- **doc 36** — Observe 节点设计稿 (已 S7_Archived)
- **Phase 6 P2/P3 audit** — 6 缺口已记录, hotfix 单独处理
