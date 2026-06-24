# Design: D7 MUPS v5 PR-V5.6 T2 ResumeSession SessionOrchestrator 入口

**Change ID:** `devrix-d7-mups-v5-escape-engine-v5-6`
**Demand ID:** DM-20260625-003
**Design Status:** Approved (S3-Gate)
**Created:** 2026-06-25

---

## 1. 整体架构

V5.6 收口 T2 ResumeSession 续跑入口，是 V5.3 ResumeSession (HumanArbitrator) + V5.5 5 节点接线之间的连接桥梁。

```
V5.3 (已落地)            V5.5 (已落地)              V5.6 (本 PR)
─────────────            ─────────────              ──────────
HumanArbitrator          SessionOrchestrator        SessionOrchestrator
.ResumeSession    →      .ProcessMessage     →      .applyResumeSession
 (one-shot consume)        (无 T2 入口)                (T2 入口 + 短路)
                                                       │
                                                       ↓
                                                  buildObserveRequest (after)
                                                       │
                                                       ↓
                                                  applyResumeSession (新增)
                                                       │
                                                       ├─ found A → fall through → classify → 5 节点
                                                       ├─ found B → emit "complete" + close channel
                                                       └─ found C → emit "complete" + close channel
```

## 2. 数据流

### 2.1 输入: ProcessRequest

```go
type ProcessRequest struct {
    SessionID string  // 用于 ResumeSession 查 PendingResolutionStore
    Message   string  // 不参与 T2 续跑逻辑
    TrackMode string  // 不参与 T2 续跑逻辑 (LP-1 prior 在 buildObserveRequest 处理)
    // ...
}
```

### 2.2 PendingResolutionStore 内容

```go
type EscapeDecision struct {
    Action     EscapeAction  // Continue/ForceExit/AbortWithAudit
    Reason     string        // "user_continue" / "user_accept" / "user_cancel"
    AuditLevel uint8
    PendingID  string        // 飞书卡片 pending ID
    SessionID  string
    CreatedAt  time.Time
}
```

### 2.3 输出: 短路时 EngineEvent

```go
&contracts.EngineEvent{
    Type:      "complete",
    Content:   "（用户接受当前结果）",  // 中文 text, 飞书卡片显示
    SessionID: req.SessionID,
    Metadata: map[string]string{
        "escape.resume":      "true",
        "escape.action":      "force_exit",
        "escape.reason":      "user_accept",
        "escape.pending_id":  decision.PendingID,
        "exit_reason_source": "user_resume",
    },
}
```

## 3. 详细设计

### 3.1 applyResumeSession

```go
func (o *SessionOrchestrator) applyResumeSession(
    _ context.Context,  // 预留 (未来 audit/tracing)
    req orchtypes.ProcessRequest,
    sessionSpan tracer.Span,
) (<-chan *contracts.EngineEvent, bool, error) {
    // 3 层 fail-safe
    if o.escapeEngine == nil {
        if sessionSpan != nil {
            sessionSpan.SetAttributes(tracer.Attribute{
                Key: "escape.resume.attempted", Value: "false",
            })
        }
        return nil, false, nil
    }

    decision, found, err := o.escapeEngine.ResumeSession(req.SessionID)
    if err != nil {
        slog.Warn("orchestrator: escape: resume_session_error",
            "session_id", req.SessionID, "err", err)
        if sessionSpan != nil {
            sessionSpan.SetAttributes(
                tracer.Attribute{Key: "escape.resume.attempted", Value: "true"},
                tracer.Attribute{Key: "escape.resume.decision_action", Value: "error_failsafe"},
            )
        }
        return nil, false, nil
    }
    if !found {
        if sessionSpan != nil {
            sessionSpan.SetAttributes(tracer.Attribute{
                Key: "escape.resume.attempted", Value: "true",
            })
        }
        return nil, false, nil
    }

    // 找到 decision
    if sessionSpan != nil {
        sessionSpan.SetAttributes(
            tracer.Attribute{Key: "escape.resume.attempted", Value: "true"},
            tracer.Attribute{Key: "escape.resume.decision_action", Value: decision.Action.String()},
            tracer.Attribute{Key: "escape.resume.decision_pending_id", Value: decision.PendingID},
        )
    }

    // A user_continue → fall through
    if decision.Action == escape.EscapeContinue {
        return nil, false, nil
    }

    // terminal decision (B/C) → emit "complete" + close channel early
    out := make(chan *contracts.EngineEvent, 1)
    out <- &contracts.EngineEvent{
        Type:      "complete",
        Content:   resumeContentForDecision(decision),
        SessionID: req.SessionID,
        Metadata: map[string]string{
            "escape.resume":      "true",
            "escape.action":      decision.Action.String(),
            "escape.reason":      decision.Reason,
            "escape.pending_id":  decision.PendingID,
            "exit_reason_source": "user_resume",
        },
    }
    close(out)
    return out, true, nil
}
```

### 3.2 ProcessMessage 入口插入

```go
func (o *SessionOrchestrator) ProcessMessage(ctx context.Context, req orchtypes.ProcessRequest) (<-chan *contracts.EngineEvent, error) {
    ctx, sessionSpan := o.startSpan(ctx, telemetry.OpD7_S2_Orchestration_Session_Process, ...)
    sessionCtx := ctx

    // Phase 6 LP-1 (existing)
    observeReq, err := o.buildObserveRequest(ctx, req)
    if err != nil { ... }
    prior := observeReq.EffectivePrior()

    // PR-V5.6: T2 ResumeSession 续跑入口
    if resumeCh, shortCircuit, _ := o.applyResumeSession(ctx, req, sessionSpan); shortCircuit {
        endSpan(sessionSpan)
        return resumeCh, nil
    }

    // 后续 classify + 5 节点流程 (existing)
    ...
}
```

### 3.3 resumeContentForDecision helper

```go
func resumeContentForDecision(d escape.EscapeDecision) string {
    switch d.Action {
    case escape.EscapeContinue:
        return "（用户选择继续完整流程）"
    case escape.EscapeForceExit:
        return "（用户接受当前结果）"
    case escape.EscapeAbortWithAudit:
        return "（用户取消当前任务）"
    case escape.EscapePendingHuman:
        return "（等待用户响应超时）"
    case escape.EscalateToRule, escape.EscalateToHuman:
        return "（需要进一步决策）"
    default:
        return "（会话终止）"
    }
}
```

## 4. 测试设计

### 4.1 单元测试 (6)

| Test | 验证目标 |
|------|---------|
| TestApplyResumeSession_NoEngine | nil engine → fall through |
| TestApplyResumeSession_NoPending | resume 找到 = false → fall through |
| TestApplyResumeSession_UserAccept | B user_accept → EscapeForceExit 短路 |
| TestApplyResumeSession_UserCancel | C user_cancel → EscapeAbortWithAudit 短路 |
| TestApplyResumeSession_UserContinue | A user_continue → fall through |
| TestApplyResumeSession_ResumeError_Failsafe | ResumeSession error → fail-safe fall through |

### 4.2 集成测试 (2)

| Test | 验证目标 |
|------|---------|
| TestProcessMessage_WithResume_UserAccept_EarlyClose | 端到端: B user_accept 短路早退 |
| TestProcessMessage_WithResume_UserCancel_EarlyClose | 端到端: C user_cancel 短路早退 |

## 5. 失败降级矩阵

| 场景 | 行为 | 原因 |
|------|------|------|
| o.escapeEngine == nil | fall through | 兼容 V5.4- 行为 (无 escape) |
| ResumeSession() error | slog.Warn + fall through | V5.3 panic recover 已处理 |
| PendingResolutionStore TTL expired | fall through | 正常续跑 |
| A user_continue | fall through | 用户希望继续走完整 5 节点 |
| B user_accept | 短路 emit "complete" | 用户接受当前结果 |
| C user_cancel | 短路 emit "complete" | 用户取消当前任务 |

## 6. 与 V5.5 5 节点接线的关系

V5.5 已有 4 个接线点 (1a/1b/2/3) 在 ProcessMessage 中段，V5.6 在 ProcessMessage 入口 (T2 续跑)。

- V5.5 接线点 1a (Plan fails): 处理 Observe 失败的 EscapeForceExit
- V5.5 接线点 1b (Plan 前): 处理 Plan 前的 EscapeForceExit
- V5.5 接线点 2 (Execute fails): 处理执行失败的 EscapeForceExit
- V5.5 接线点 3 (Verify fails): 处理验证失败的 EscapeForceExit (待 processAutoClose 暴露)
- **V5.6 (本 PR) T2 续跑入口**: 处理用户主动决策 A/B/C

两者互补不冲突：V5.5 接线点是 5 节点内部 fail-fast 出口；V5.6 是 5 节点外部用户决策入口。

## 7. 不在本次范围

- ❌ 不修改 EscapeEngine / HumanArbitrator / PendingResolutionStore
- ❌ 不修改 5 节点接线 (V5.5)
- ❌ 不修改 LoopDepthTracker / PlanKindSwitchPolicy / ChainedArbitrator / CircuitBreaker
- ❌ 不修改 Learner / ReputationStore (Phase 5/6/7)
- ❌ 不修改 PendingResolutionStore TTL
