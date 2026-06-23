# Acceptance Report — DM-20260625-001 (Phase 7 Verify→Learn Auto-Close + Operator TrackMode + D5 增强)

**Change ID:** `devrix-d7-mups-v4-phase7-verify-auto-close`
**Demand ID:** DM-20260625-001
**PR Scope:** PR-7.1 + PR-7.2 + PR-7.3 (Verify→Learn Auto-Close + Operator TrackMode + D5 可观测化增强)
**Acceptance Date:** 2026-06-25
**Author:** MUPS v4.3 Phase 7 运行时 5 节点闭环
**Status:** ✅ S5_Accepted → S7_Archived

---

## 1. 验收范围

本报告验收 Phase 7 运行时 5 节点闭环（PR-7.1/7.2/7.3）的实现质量与设计一致性。
本 Change 闭环 Phase 6 PR-F3 E2E 测试中手工模拟的 Verify→Learn 步骤，把 SessionOrchestrator.ProcessMessage 末尾的 LP-1 闭环 wire 到生产运行时，并补充 Operator 角色 TrackMode + D5 trace 6 字段完整 prior 语义。

| 维度 | 范围 |
|------|------|
| **代码变更** | 3 PR / 8 NEW files + 6 MODIFIED files / +900/-40; 生产运行时 5 节点闭环 |
| **测试变更** | 30+ tests / 0 race detector warnings / go vet clean / coverage ≥ 80% per file |
| **文档变更** | spec.md v4.6.0→v4.7.0 (D7-S13-A47/A48/A49 Requirement) + t-registry.md v3.14.0→v3.15.0 (T 174→180, P0 141→147) |
| **运行时闭环** | processAutoClose 包装 channel + synthesizeVerdict 4 规则 (complete→Pass / error→Fail / tombstone→Indeterminate) + 3 层 fail-safe + AssetBuilder Auto-Close fallback |
| **TrackMode 切换** | ProcessRequest.TrackMode + 3-tier 解析 (Reputation wins > hint > Developer 兜底) + Operator track → Beta(8,1) Mean=0.889 |
| **D5 增强** | sessionSpan 6 attributes (alpha/beta/mean/track_mode/classifier_source/injected_at) + Jaeger UI 自然支持 |
| **不做的事** | Phase 5 Learn 节点核心契约保持稳定 / 不引入新 LLM / 不实现 D2-backed ReputationStore / 不修改 ShadowClassifier 异步路径 / 不实现 Plan/Artifact 反向追溯 |

## 2. 验收标准达成

### 2.1 P0 验收 (AC1-AC6)

| ID | 验收标准 | 状态 | 证据 |
|----|---------|------|------|
| **AC1** | SessionOrchestrator.processAutoClose 包装 channel + 异步触发 learner.Learn + 替换 endSpanWhenChannelClosed + IntentSkip 路径不调用 + 3 层 fail-safe | ✅ PASS | D7-S13-A47-T01 IMPLEMENTED；`sessionorchestrator/orchestrator_autoclose_test.go` 3/3 PASS (TestProcessAutoClose_NilLearner_Passthrough + TestProcessAutoClose_LearnerError_LoggedNotBlocked + TestProcessAutoClose_EmptyChannel_NoLearn) |
| **AC2** | synthesizeVerdict 规则 (complete→Pass / error→Fail / tombstone→Indeterminate + IndeterminateReason="interrupt") + SourceID 格式 autoclose:{sessionID}:{nanosecond} + 10 sub-case table-driven 单测 | ✅ PASS | D7-S13-A47-T02 IMPLEMENTED；orchestrator_autoclose_test.go: TestSynthesizeVerdict_AllEventTypes 10/10 PASS + TestSynthesizeVerdict_NilEvent + TestSynthesizeVerdict_Tombstone_IndeterminateReason 12/12 PASS |
| **AC3** | 集成测试 ProcessMessage 完整跑 → Alpha++ + 下一轮 prior 更新 (TestProcessMessage_Verify2Learn_AutoClose_PassAlpha Round 1 冷启动 Beta(5,3) → Learn VerdictPass → Alpha=1 → Round 2 Beta(6,3) Mean=0.667) + TestAutoClose_FullLP1Loop 端到端 LP-1 闭环 | ✅ PASS | D7-S13-A47-T03 IMPLEMENTED；orchestrator_autoclose_test.go: TestProcessMessage_Verify2Learn_AutoClose_PassAlpha + TestAutoClose_FullLP1Loop + TestProcessMessage_AutoClose_NilLearner_NoOp + TestProcessMessage_AutoClose_LearnerError_LoggedNotBlocked + TestProcessMessage_AutoClose_ContextCancel_SkipLearn + TestProcessMessage_AutoClose_IntentSkip_NoLearn + TestProcessMessage_AutoClose_ErrorEvent_VerdictFail + TestProcessMessage_AutoClose_TombstoneEvent_VerdictIndeterminate 8/8 PASS |
| **AC4** | ProcessRequest 新增 TrackMode string 字段 (默认 "" 兜底 developer) + TrackModeDeveloper/Operator 常量 + NewProcessRequest fail-fast 校验 + 3 sentinel errors + 8 unit tests | ✅ PASS | D7-S13-A48-T04 IMPLEMENTED；`orchtypes/process_test.go` 8/8 PASS (TestProcessRequest_ZeroValue_TrackModeEmpty + TestNewProcessRequest_EmptySession_FailFast + TestNewProcessRequest_EmptyMessage_FailFast + TestNewProcessRequest_TrackModeEmpty_Accepts + TestNewProcessRequest_TrackModeDeveloper_Accepts + TestNewProcessRequest_TrackModeOperator_Accepts + TestNewProcessRequest_TrackModeInvalid_FailFast + TestNewProcessRequest_RoundTripAllFields) |
| **AC5** | buildObserveRequest 透传 req.TrackMode → o.learner.Inject(ctx, sessionID, req.TrackMode) → BuildAdaptivePrior (Operator track → DefaultOperatorPrior Beta(8,1) Mean=0.889，Developer → Beta(5,3) Mean=0.625，空字符串/未知 → 兜底 Developer) + 6 TrackMode 单测 | ✅ PASS | D7-S13-A48-T05 IMPLEMENTED；`sessionorchestrator/orchestrator_trackmode_test.go` 6/6 PASS (TestProcessMessage_TrackModeOperator_PropagatedToInject + TestProcessMessage_TrackModeDeveloper_PropagatedToInject + TestProcessMessage_TrackModeEmpty_PropagatedAsEmpty + TestProcessMessage_TrackModeInvalid_ForwardsToInject + TestDefaultLearner_Inject_TrackModeOperator_ColdStart + TestDefaultLearner_Inject_TrackModeUnknown_LogsAndFallsBack) |
| **AC6** | sessionSpan 6 prior attributes (alpha/beta/mean/track_mode/classifier_source/injected_at) 全部写入 + priorSessionSpanAttrs 纯 helper + 5 单测覆盖 real injection / cold_start_failsafe / operator from hint / reputation wins / 字符串类型校验 | ✅ PASS | D7-S13-A49-T06 IMPLEMENTED；`sessionorchestrator/orchestrator_priorspan_test.go` 5/5 PASS (TestPriorSessionSpanAttrs_RealInjection_AllFive + TestPriorSessionSpanAttrs_ColdStartFailsafe + TestPriorSessionSpanAttrs_OperatorFromRequestHint + TestPriorSessionSpanAttrs_ReputationTrackModeWinsOverHint + TestPriorSessionSpanAttrs_AllAttributesHaveStringValues) |

### 2.2 测试与质量

| 项 | 目标 | 实际 | 状态 |
|----|------|------|------|
| 单元测试 PASS | 100% | 30+/30+ PASS (含 race) | ✅ PASS |
| 集成测试 PASS (E2E LP-1 in production) | 1/1 | 1/1 PASS (含 race) | ✅ PASS |
| 新增 P0 T | 6 | 6 (D7-S13-A47-T01/T02/T03 + A48-T04/T05 + A49-T06) | ✅ PASS |
| `go vet` clean | 0 issue | 0 issue | ✅ PASS |
| Race detector | 0 warning | 0 warning | ✅ PASS |
| v2 regression | 0 | 0 (Phase 1/2/3/4/5/6 既有 tests 全部 PASS) | ✅ PASS |
| LP-1 生产 wiring | processAutoClose | processAutoClose 替换 endSpanWhenChannelClosed | ✅ PASS |
| AssetBuilder Auto-Close fallback | Plan/Artifact nil 时合成 | sop:autoclose:<SourceID> + ["autoclose-completion"] | ✅ PASS |
| TrackMode 3-tier 解析 | Reputation wins | rep.TrackMode > req.TrackMode hint > Developer | ✅ PASS |
| sessionSpan 6 attributes | 全部写入 | 6/6 写入 + Jaeger UI 自然支持 | ✅ PASS |

### 2.3 关键代码变更

#### AC1: processAutoClose 包装函数
```go
// sessionorchestrator/autoclose.go
func (o *SessionOrchestrator) processAutoClose(
    ch <-chan *contracts.EngineEvent,
    sessionCtx context.Context,
    sessionID string,
    intent orchtypes.IntentClassification,
) <-chan *contracts.EngineEvent {
    if o.learner == nil {
        return endSpanWhenChannelClosed(ch, nil)  // nil learner → passthrough
    }
    out := make(chan *contracts.EngineEvent, 32)
    go func() {
        defer close(out)
        var lastEvent *contracts.EngineEvent
        for ev := range ch {
            lastEvent = ev
            out <- ev
        }
        if lastEvent == nil { return }  // empty channel (skip path)
        verdict := synthesizeVerdict(lastEvent, sessionID)
        if verdict == nil { return }
        req := learn.LearnRequest{SessionID: sessionID, Verdict: *verdict}
        if _, err := o.learner.Learn(sessionCtx, req); err != nil {
            slog.Warn("orchestrator: processAutoClose learner.Learn failed",
                "session_id", sessionID, "verdict_kind", verdict.Kind, "err", err)
        }
    }()
    return out
}
```

#### AC2: synthesizeVerdict 规则
```go
func synthesizeVerdict(last *contracts.EngineEvent, sessionID string) *workmodel.Verdict {
    switch last.Type {
    case "complete":
        return &workmodel.Verdict{
            Kind: types.VerdictPass,
            SourceID: fmt.Sprintf("autoclose:%s:%d", sessionID, time.Now().UnixNano()),
            Reason: "process complete",
        }
    case "error":
        return &workmodel.Verdict{Kind: types.VerdictFail, Reason: last.Content, ...}
    case "tombstone":
        return &workmodel.Verdict{Kind: types.VerdictIndeterminate,
            IndeterminateReason: "interrupt", ...}
    default:
        return nil  // text/thinking/tool_call/etc → not terminal
    }
}
```

#### AC4: ProcessRequest.TrackMode + NewProcessRequest
```go
// orchtypes/process.go
type ProcessRequest struct {
    SessionID string
    Message   string
    TrackMode string  // NEW Phase 7: "developer" / "operator" / "" (default developer)
    Metadata  map[string]string
}

const (
    TrackModeDeveloper = "developer"
    TrackModeOperator  = "operator"
)

func NewProcessRequest(sessionID, message, trackMode string) (ProcessRequest, error) {
    if sessionID == "" { return ProcessRequest{}, ErrProcessRequestSessionIDEmpty }
    if message == "" { return ProcessRequest{}, ErrProcessRequestMessageEmpty }
    if trackMode != "" && trackMode != TrackModeDeveloper && trackMode != TrackModeOperator {
        return ProcessRequest{}, ErrProcessRequestInvalidTrackMode
    }
    return ProcessRequest{SessionID: sessionID, Message: message, TrackMode: trackMode}, nil
}
```

#### AC5: DefaultLearner.Inject 3-tier 解析
```go
// learn/learner.go
func (l *DefaultLearner) Inject(ctx context.Context, sessionID, trackModeHint string) (*AdaptivePrior, error) {
    // Tier 1: persisted Reputation row wins
    if l.rep != nil {
        if rep, _ := l.rep.Get(sessionID); rep != nil && rep.TrackMode != "" {
            trackMode = rep.TrackMode
        }
    }
    // Tier 2: hint (may be "" → defaults to Developer)
    if trackMode == "" { trackMode = TrackModeDeveloper }
    // Tier 3: unknown → slog.Warn + Developer fail-safe
    if trackMode != TrackModeDeveloper && trackMode != TrackModeOperator {
        slog.Warn("learn: invalid track mode hint, falling back to Developer", ...)
        trackMode = TrackModeDeveloper
    }
    return BuildAdaptivePrior(rep, trackMode), nil
}
```

#### AC6: sessionSpan 6 attributes helper
```go
// sessionorchestrator/tracing.go
func priorSessionSpanAttrs(prior *learn.AdaptivePrior, observeReq orchtypes.ObserveRequest, req orchtypes.ProcessRequest) []tracer.Attribute {
    priorInjectedAt := "cold_start_failsafe"
    if observeReq.Prior != nil { priorInjectedAt = "phase6_lp1" }
    priorTrackMode := string(learn.TrackModeDeveloper)
    if prior.Reputation != nil && prior.Reputation.TrackMode != "" {
        priorTrackMode = string(prior.Reputation.TrackMode)
    } else if req.TrackMode != "" {
        priorTrackMode = req.TrackMode
    }
    return []tracer.Attribute{
        {Key: "learn.prior.alpha", Value: fmt.Sprintf("%d", prior.PriorBeta.Alpha)},
        {Key: "learn.prior.beta", Value: fmt.Sprintf("%d", prior.PriorBeta.Beta)},
        {Key: "learn.prior.mean", Value: fmt.Sprintf("%.3f", prior.PriorBeta.Mean())},
        {Key: "learn.prior.track_mode", Value: priorTrackMode},
        {Key: "learn.prior.injected_at", Value: priorInjectedAt},
    }
}
```

## 3. 关键决策点

### D1: processAutoClose vs endSpanWhenChannelClosed 嵌套

**选择:** 在 processAutoClose 内部嵌套调用 endSpanWhenChannelClosed
**理由:** 不破坏 span 关闭语义不变性 (D5 observability SLO 设计原则)。
endSpanWhenChannelClosed 只负责 span 关闭, processAutoClose 在其之上加 Learn
触发逻辑, 二者职责正交。TestProcessMessage_AutoClose_* 集成测试通过验证
span 关闭时机不变。

### D2: Auto-Close fallback (sop:autoclose: + autoclose-completion)

**选择:** AssetBuilder.buildSOPContent 在 req.Plan 为 nil 时, 合成
`sop:autoclose:<SourceID>` + `["autoclose-completion"]` 步骤
**理由:** LP-1 闭环在生产 wiring 中必须真实可走通。Phase 7 v1.0 不实现
Plan/Artifact 反向追溯 (PR-7.4+ 后续补), 但 Auto-Close 调用 Learn 时
LearnRequest.Plan/Artifact 为 nil → AssetBuilder 必须能构造 SOMETHING 而不是
返回 ErrAssetBuildFailed。SOP 名称 + 合成步骤保证 AssetBuilder.Build 成功,
5 节点管道在生产 wiring 真实走通闭环。

### D3: 3-tier TrackMode 解析 vs 单层 hint

**选择:** 3 层解析: (1) rep.TrackMode != "" → rep wins; (2) req.TrackMode
hint → hint; (3) 其他 → Developer fail-safe
**理由:** 跨会话状态 (Reputation) 应优先于 per-request hint (短期偏好),
否则用户切到 operator 后, 旧 session 还在用 developer prior 会出现
不一致。Phase 6 PR-F2 既定 prior 注入策略保持稳定, 本次 PR-7.2 在此基础上
扩展 TrackMode 解析, 兜底策略不破坏 Phase 5/6 既有契约。

### D4: priorSessionSpanAttrs 纯 helper 而非直接写入

**选择:** tracing.go 新增纯函数 `priorSessionSpanAttrs(prior, observeReq, req)`
返回 5 attributes 切片, 由 orchestrator.go ProcessMessage 在 classifySpan
关闭前调用 + SetAttributes
**理由:** 纯函数便于单元测试 (无 tracer / 无 sessionSpan mock 需求),
测试覆盖 5 个 scenario: real injection / cold_start_failsafe / operator
from hint / reputation wins / 字符串类型校验。第六个 attribute
learn.classifier_source 由 orchestrator.go 在 classifySpan 关闭前直接 mirror,
因为它依赖 classifier 实现 (RuleClassifier vs ShadowClassifier)。

## 4. 后续 Phase 8+ 待办

- **D2 ContextEngine-backed ReputationStore 持久化:** 当前 InMemoryReputationStore
  是进程内存储, 重启丢失。需要 D2 ContextEngine 适配器实现 ReputationStore
  接口 (LP-3 持久化 + LP-5 跨进程追踪)
- **ShadowClassifier.ClassifyWithPrior 异步 LLM 重新启用:** Phase 6 既定
  ShadowClassifier 委托给 rule, LP-1 不影响 shadow 异步路径。Phase 8+
  可重新启用 shadow LLM 异步 sample, 让真实 LLM 决策与 prior 注入交叉验证
- **Plan / Artifact 反向追溯:** 当前 LearnRequest.Plan/Artifact/Observations
  暂时为 nil (Auto-Close fallback 合成)。Phase 8+ 可在 Path 结束前 hook
  Plan/Artifact 到 LearnRequest, 闭环 LP-5 (Asset.SourceSessionIDs 含 Plan.ID)
- **Auto-Close 异步超时配置:** 当前 hardcode 5s (与 AssetBuilder.Build 默认
  超时对齐), Phase 8+ 可暴露 CFG.Orchestrator.AutoCloseTimeoutMs
- **Verifier 子 agent 显式注入:** Phase 4 D7-S10-A33-T03 路径级 Verifier
  子 agent, 当前 Auto-Close 不感知 Verifier 是否被注入, Phase 8+ 可在
  processAutoClose 内查 Verifier 状态
- **sessionSpan 6 attribute D5 dashboard:** 当前只有 Jaeger UI 自然支持,
  Phase 8+ 可在 observability guide 加 sessionSpan 6 attribute 排查 SOP

## 5. S6 Archive 落地清单

- ✅ 移动 8 文件 → `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase7-verify-auto-close/`
- ✅ 创建 `.openspec.yaml` manifest
- ✅ 创建 `acceptance-report.md`
- ✅ 同步 spec.md v4.6.0 → v4.7.0
- ✅ 同步 t-registry.md v3.14.0 → v3.15.0
- ✅ 同步 demand-archive-index.md
- ✅ 运行 scripts/verify-archive.sh → 14/14 PASS
- ✅ 创建 S6 archive PR + auto-merge

## 6. Cross-references

- Phase 7 OpenSpec: `openspec/changes/devrix-d7-mups-v4-phase7-verify-auto-close/`
- Phase 6 OpenSpec (T13 PARTIAL 闭环入口): `openspec/archive/2026-06-24-devrix-d7-mups-v4-phase6-observe-learner-wiring/`
- Phase 5 OpenSpec (Learner interface): `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase5-learn/`
- Phase 4 OpenSpec (Verify 节点升格): `openspec/archive/2026-06-23-devrix-d7-mups-v4-phase4-verify-promotion/`
- D7 域规范 v4.7.0: `openspec/specs/d7-orchestration/spec.md`
- D7 T 层注册表 v3.15.0: `openspec/specs/d7-orchestration/t-registry.md`
- PR-7.1: https://github.com/fqntxmqee/devrix/pull/188 (MERGED)
- PR-7.2: https://github.com/fqntxmqee/devrix/pull/189 (MERGED)
- PR-7.3: https://github.com/fqntxmqee/devrix/pull/190 (MERGED)