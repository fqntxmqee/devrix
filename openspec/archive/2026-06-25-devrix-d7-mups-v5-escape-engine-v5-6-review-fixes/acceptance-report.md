# Acceptance Report: D7 MUPS v5 PR-V5.6 Review 修复 (DM-20260625-004)

**Change ID:** `devrix-d7-mups-v5-escape-engine-v5-6-review-fixes`
**Demand ID:** DM-20260625-004
**Parent Change:** `devrix-d7-mups-v5-escape-engine-v5-6` (PR-V5.6, PR #200, S7_Archived)
**PR Scope:** 14 review findings → 8 applied (2 Critical + 4 High + 2 Medium) + 6 deferred
**Acceptance Date:** 2026-06-25
**Acceptance Status:** ✅ Accepted (S5)

---

## 1. 修复清单 (8/14)

### 1.1 Critical (2/2)

| # | Fix | 文件 | PR | 验收证据 |
|---|-----|------|----|---------|
| C-1 | t-registry 删除不存在的 `runLoopWithResume` | `openspec/specs/d7-orchestration/t-registry.md` | #202 | 4 行 removal, line 567 detail row + line 591 detail tree |
| C-2 | t-registry Statistics 表 4 处数字 + Revision History v3.18.0 条目 | `openspec/specs/d7-orchestration/t-registry.md` | #202 | line 434 + 449 + 450 + 502 v3.18.0 entry |

### 1.2 High (4/4)

| # | Fix | 文件 | PR | 验收证据 |
|---|-----|------|----|---------|
| H-1 | audit 注释对齐 (V5.4 SubmitUserChoice 已写, resume read-only) | `sessionorchestrator/escape_wiring.go` | #203 | 注释 2 处修改 (line 132-135 + line 200) |
| H-2 | 短路早退补写 prior attrs (5 字段) | `sessionorchestrator/orchestrator.go` | #203 | line 338-348 短路前 SetAttributes(priorSessionSpanAttrs(prior, observeReq, req)...) |
| H-3 | 新增 span attr 单测 (4 sub-test) | `sessionorchestrator/orchestrator_resume_test.go` | #203 | `TestApplyResumeSession_SessionSpanAttrs` + 4 sub-test PASS |
| H-4 | 集成测试增强 (recordingExecutor + 5 字段 Metadata 断言) | `sessionorchestrator/orchestrator_resume_test.go` | #203 | 2 new tests: `TestProcessMessage_WithResume_UserAccept_EarlyClose_NoExecutorCall` + `UserCancel_EarlyClose_NoExecutorCall` |

### 1.3 Medium (2/4)

| # | Fix | 文件 | PR | 验收证据 |
|---|-----|------|----|---------|
| M-1 | 空 SessionID 守卫 | `sessionorchestrator/escape_wiring.go` | #203 | line 142-153 + `TestApplyResumeSession_EmptySessionID_FallThrough` |
| M-3 | 删除 dead code `stubResume` | `sessionorchestrator/orchestrator_resume_test.go` | #203 | 16 行 stubResume + Save/Load/Delete 删除, 改用 errStore + 真实 HumanArbitrator |

---

## 2. 测试结果

### 2.1 单元/集成测试

| 测试 | 类型 | 状态 | 备注 |
|------|------|------|------|
| `TestApplyResumeSession_NoEngine` | 单元 (已有) | ✅ PASS | 1 fail-safe |
| `TestApplyResumeSession_NoPending` | 单元 (已有) | ✅ PASS | 1 fail-safe |
| `TestApplyResumeSession_UserAccept` | 单元 (已有) | ✅ PASS | B user_accept |
| `TestApplyResumeSession_UserCancel` | 单元 (已有) | ✅ PASS | C user_cancel |
| `TestApplyResumeSession_UserContinue` | 单元 (已有) | ✅ PASS | A user_continue |
| `TestApplyResumeSession_ResumeError_Failsafe` | 单元 (已有) | ✅ PASS | 1 fail-safe |
| `TestProcessMessage_WithResume_UserAccept_EarlyClose` | 集成 (已有) | ✅ PASS | 端到端 B |
| `TestProcessMessage_WithResume_UserCancel_EarlyClose` | 集成 (已有) | ✅ PASS | 端到端 C |
| `TestApplyResumeSession_EmptySessionID_FallThrough` | 单元 (M-1) | ✅ PASS | 契约违反守卫 |
| `TestApplyResumeSession_SessionSpanAttrs` (4 sub-test) | 单元 (H-3) | ✅ PASS | nil_engine / err_failsafe / not_found / found_terminal |
| `TestProcessMessage_WithResume_UserAccept_EarlyClose_NoExecutorCall` | 集成 (H-4) | ✅ PASS | RunTurn calls == 0 + 5 字段 Metadata |
| `TestProcessMessage_WithResume_UserCancel_EarlyClose_NoExecutorCall` | 集成 (H-4) | ✅ PASS | RunTurn calls == 0 + 5 字段 Metadata |

**总计: 11/11 PASS (含 4 sub-test), 0 race, 3/3 稳定性验证**

### 2.2 完整 orchestration packages

```
go test -race ./internal/layers/orchestration/...  →  22/22 PASS
```

### 2.3 静态检查

```
go build ./...  →  0 errors
go vet ./...    →  0 issues
```

---

## 3. 关键代码变更

### 3.1 H-1 audit 注释修复 (escape_wiring.go)

```go
// 之前:
- B user_accept → EscapeForceExit → emit "complete" + 补写 audit
- C user_cancel → EscapeAbortWithAudit → emit "complete" + 补写 audit
// emit single "complete" EngineEvent + 补写 audit + close channel early.

// 修复后 (honest fix — resume is read-only):
+ B user_accept → EscapeForceExit → emit "complete" (audit already recorded
+   at SubmitUserChoice time (V5.4); resume is read-only, 不重复写 audit)
+ C user_cancel → EscapeAbortWithAudit → emit "complete" (audit already recorded)
// emit single "complete" EngineEvent (audit already written at SubmitUserChoice time) + close channel early.
```

### 3.2 H-2 短路早退补写 prior attrs (orchestrator.go)

```go
if resumeCh, shortCircuit, _ := o.applyResumeSession(ctx, req, sessionSpan); shortCircuit {
    // H-2: short-circuit path also writes prior attrs so D5 trace has
    // consistent learn.prior.{alpha,beta,mean,track_mode,injected_at} for
    // resume decisions (user_accept / user_cancel short-circuit would
    // otherwise permanently miss these attrs in Jaeger trace).
    if sessionSpan != nil {
        sessionSpan.SetAttributes(priorSessionSpanAttrs(prior, observeReq, req)...)
    }
    endSpan(sessionSpan)
    return resumeCh, nil
}
```

### 3.3 M-1 空 SessionID 守卫 (escape_wiring.go)

```go
// M-1: guard against empty SessionID.
// Empty SessionID is a contract violation (ProcessRequest constructor
// requires non-empty), not a transient error — silently fall through
// without triggering slog.Warn to avoid log noise / misdiagnosis.
if req.SessionID == "" {
    if sessionSpan != nil {
        sessionSpan.SetAttributes(tracer.Attribute{
            Key: "escape.resume.attempted", Value: "false",
        })
    }
    return nil, false, nil
}
```

---

## 4. 6 Deferred (留待后续 cleanup change)

| # | 级别 | 标题 | 优先级 | 说明 |
|---|------|------|--------|------|
| M-2 | Medium | fail-safe attr symmetry | low | err_failsafe 路径不写 decision_action, 不对称 (not_found 也不写) — 风格, 不影响功能 |
| M-4 | Medium | V5.5 archive description | immutable | 已 s7_archived 不可变, 等下次 archive 迁移时修正 |
| I-1 | Info | escape.resume marker | forward-compat | 当前 no-op, forward-compat 留位 |
| I-2 | Info | table-driven test refactor | style | 8 个 ApplyResumeSession_* 测试可改 table-driven, 不影响覆盖 |
| I-3 | Info | unused error return | V5.7 | applyResumeSession 第 3 返回值 err 保留供未来 fail-fast, 当前总 nil |
| I-4 | Info | sessionSpan endSpan err design.md comment | style | design.md 注释精修 |

---

## 5. PR 联动

| Step | PR | branch | files | + | - | merged |
|------|----|----|-------|---|---|--------|
| 1 (docs-only) | [#202](https://github.com/fqntxmqee/devrix/pull/202) | feat/d7-mups-v5-escape-engine-v5-6-review-fixes | 1 | 6 | 5 | b46612d |
| 2 (code+test) | [#203](https://github.com/fqntxmqee/devrix/pull/203) | feat/d7-mups-v5-escape-engine-v5-6-review-fixes-step2 | 3 | 337 | 30 | 3001ff3 |
| **合计** | 2 PR | — | **4** | **343** | **35** | — |

---

## 6. 验收结论

- ✅ 8/8 修复 (2 Critical + 4 High + 2 Medium) 全部落地
- ✅ 11/11 测试 PASS (含 4 sub-test), 0 race
- ✅ 22/22 orchestration packages 100% PASS
- ✅ 0 v2 regression (Phase 1-7 + V5.1-V5.5 既有 tests 全部 PASS)
- ✅ go vet 0 issue, go build 0 error
- ✅ 6 deferred 项已记入后续 cleanup change backlog

**S5 验收通过, S6 归档完成。**
