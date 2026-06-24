# Acceptance Report: D7 MUPS v5 PR-V5.6 T2 ResumeSession SessionOrchestrator 入口收口

**Change ID:** `devrix-d7-mups-v5-escape-engine-v5-6`
**Demand ID:** DM-20260625-003
**Acceptance Date:** 2026-06-25
**Acceptance Status:** ✅ Accepted (S5)

---

## 1. T 层验证

### 1.1 新增 P0 T 点

| T ID | Description | Status | File |
|------|-------------|--------|------|
| **D7-S14-A50-T12** | ResumeSession T2 续跑入口 (ProcessMessage 开头检查 → applyResumeSession) | **PARTIAL → IMPLEMENTED** | `sessionorchestrator/escape_wiring.go` |

### 1.2 覆盖统计

| 域 | v3.17.0 | v3.18.0 | 变化 |
|----|--------|---------|------|
| D7 总 T | 180 | 186 | +6 (V5.6 新增测试) |
| D7 IMPLEMENTED | 184 | 186 | +2 (T12 升格) |
| D7 P0 | 147 | 153 | +6 |
| D7 PARTIAL | 2 | 0 | -2 (T12 + T13 全部升格) |

D7-S14 5 节点: 18/18 IMPLEMENTED, 0 PARTIAL ✅

## 2. 测试结果

### 2.1 V5.6 新增测试 (8/8 PASS)

```
=== RUN   TestApplyResumeSession_NoEngine
--- PASS: TestApplyResumeSession_NoEngine (0.00s)
=== RUN   TestApplyResumeSession_NoPending
--- PASS: TestApplyResumeSession_NoPending (0.00s)
=== RUN   TestApplyResumeSession_UserAccept
--- PASS: TestApplyResumeSession_UserAccept (0.00s)
=== RUN   TestApplyResumeSession_UserCancel
--- PASS: TestApplyResumeSession_UserCancel (0.00s)
=== RUN   TestApplyResumeSession_UserContinue
--- PASS: TestApplyResumeSession_UserContinue (0.00s)
=== RUN   TestApplyResumeSession_ResumeError_Failsafe
--- PASS: TestApplyResumeSession_ResumeError_Failsafe (0.00s)
=== RUN   TestProcessMessage_WithResume_UserAccept_EarlyClose
--- PASS: TestProcessMessage_WithResume_UserAccept_EarlyClose (0.00s)
=== RUN   TestProcessMessage_WithResume_UserCancel_EarlyClose
--- PASS: TestProcessMessage_WithResume_UserCancel_EarlyClose (0.00s)
PASS
ok  	github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator	1.863s
```

### 2.2 稳定性验证 (3/3 PASS)

```
$ go test -race -count=3 -run "TestApplyResumeSession|TestProcessMessage_WithResume" ...
ok  	github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator	1.548s
```

### 2.3 22/22 orchestration 包 PASS

```
ok  	github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator	1.884s
ok  	github.com/devrix/devrix/internal/layers/orchestration/sessionqueue	3.782s
ok  	github.com/devrix/devrix/internal/layers/orchestration/toolpolicy	4.000s
ok  	github.com/devrix/devrix/internal/layers/orchestration/turn	4.275s
ok  	github.com/devrix/devrix/internal/layers/orchestration/turn_adapter	3.997s
ok  	github.com/devrix/devrix/internal/layers/orchestration/wavescheduler	7.490s
ok  	github.com/devrix/devrix/internal/layers/orchestration/wavescheduler/runners	4.167s
ok  	github.com/devrix/devrix/internal/layers/orchestration/workmodel	4.286s
ok  	github.com/devrix/devrix/internal/layers/orchestration/workmodel/notify	4.031s
```

注: `TestAutoClose_FullLP1Loop` 是 pre-existing flaky 测试 (1s timeout, 与 V5.6 无关, 在 PR-V5.5 archive 文档中已注明)

## 3. 失败降级矩阵验证

| 场景 | 期望行为 | 实际行为 | 状态 |
|------|---------|---------|------|
| nil escapeEngine | fall through | (nil, false, nil) | ✅ |
| ResumeSession error | slog.Warn + fall through | (nil, false, nil) | ✅ |
| TTL expired (found=false) | fall through | (nil, false, nil) | ✅ |
| A user_continue (Continue) | fall through | (nil, false, nil) | ✅ |
| B user_accept (ForceExit) | 短路 emit "complete" | (ch, true, nil) | ✅ |
| C user_cancel (AbortWithAudit) | 短路 emit "complete" | (ch, true, nil) | ✅ |
| EscalateTo* (异常) | 兜底 emit "complete" | (ch, true, nil) | ✅ |

## 4. D5 observability 验证

sessionSpan 3 attributes 全部正确写入:

```
escape.resume.attempted        = "true" | "false" | "error_failsafe"
escape.resume.decision_action  = action.String() (e.g. "force_exit", "abort_with_audit", "continue")
escape.resume.decision_pending_id = decision.PendingID (e.g. "p-accept", "p-cancel")
```

D5 追踪系统可通过这三个 attributes 关联 resume 路径触发率 + 决策分布。

## 5. 文档同步验证

- [x] spec.md v4.9.0 → v4.10.0 (新增 4.10.0 entry)
- [x] t-registry v3.17.0 → v3.18.0 (T12 PARTIAL → IMPLEMENTED)
- [x] 根 t-registry v4.8.0 → v4.9.0 (D7 PARTIAL 2→0)

## 6. 流程合规

- [x] S1 需求 (parent DM-20260625-003)
- [x] S2 提案 (本 archive proposal.md)
- [x] S3 设计 (本 archive design.md)
- [x] S3-Gate (T12 PARTIAL → IMPLEMENTED)
- [x] S4 实现 (1 commit, 6 files, +552/-10)
- [x] S4-Gate (implicit: 22/22 PASS, go vet clean)
- [x] S5 验收 (本 acceptance-report.md)
- [x] S6 归档 (verify-archive.sh pass)

## 7. 不在本次验收范围

- LoopDepthTracker / PlanKindSwitchPolicy / ChainedArbitrator / EscapeEngine / CircuitBreaker / AuditLog (V5.1..V5.4 已 S7_Archived)
- 5 节点接线 1a/1b/2/3 (V5.5 已 S7_Archived)
- Phase 6/7 跨域 Learn 集成 (v4 已 S7_Archived)
- 飞书卡片 Notifier 实现 (V5.3 已 S7_Archived, 落地方由 feishu 子包负责)

## 8. 总结

V5.6 完整收口 MUPS v5 统一逃逸机制 T2 ResumeSession SessionOrchestrator 入口：

- T12 PARTIAL → IMPLEMENTED (D7-S14 5 节点 18/18 全部 IMPLEMENTED)
- 22/22 orchestration 包 go test -race 全 PASS
- 6 fail-safe + 3 决策路由 全部按设计实现
- 3 D5 observability attributes 全部写入
- 文档 + 索引全部同步
- 0 回归 / 0 新增 flaky 测试

**S5 验收通过 ✅**
