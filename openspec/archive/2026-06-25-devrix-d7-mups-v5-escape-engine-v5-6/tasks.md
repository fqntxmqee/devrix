# Tasks: D7 MUPS v5 PR-V5.6 T2 ResumeSession SessionOrchestrator 入口收口

**Change ID:** `devrix-d7-mups-v5-escape-engine-v5-6`
**Demand ID:** DM-20260625-003
**Total Tasks:** 6 P0 T (1 实现 + 5 测试)
**Implementation Date:** 2026-06-25

---

## T01 [IMPLEMENTED] applyResumeSession + resumeContentForDecision helper

**File:** `internal/layers/orchestration/sessionorchestrator/escape_wiring.go`

**设计要点**:
- 3 层 fail-safe (nil engine / ResumeSession error / TTL expired)
- 3 类决策路由 (A fall through / B ForceExit / C AbortWithAudit)
- sessionSpan 3 attributes (D5 observability)
- resumeContentForDecision helper 覆盖 6 类 EscapeAction

**代码量**: ~140 行 (含注释 + helper)

**验收**:
- [x] 编译通过 (`go build`)
- [x] go vet 通过

---

## T02 [IMPLEMENTED] 6 单元测试 (TestApplyResumeSession_*)

**File:** `internal/layers/orchestration/sessionorchestrator/orchestrator_resume_test.go`

**测试覆盖**:
- TestApplyResumeSession_NoEngine        — nil engine → fall through
- TestApplyResumeSession_NoPending       — TTL expired → fall through
- TestApplyResumeSession_UserAccept      — B → ForceExit 短路
- TestApplyResumeSession_UserCancel      — C → AbortWithAudit 短路
- TestApplyResumeSession_UserContinue    — A → fall through
- TestApplyResumeSession_ResumeError_Failsafe — error → fail-safe

**测试设计**:
- 真实 InMemoryPendingResolutionStore + 真实 HumanArbitrator (避免 mock 漂移)
- 直接调用 applyResumeSession 方法 (单测隔离)

**验收**:
- [x] 6/6 PASS
- [x] 3/3 稳定性验证 PASS

---

## T03 [IMPLEMENTED] 2 集成测试 (TestProcessMessage_WithResume_*)

**File:** `internal/layers/orchestration/sessionorchestrator/orchestrator_resume_test.go`

**测试覆盖**:
- TestProcessMessage_WithResume_UserAccept_EarlyClose  — 端到端 B 短路
- TestProcessMessage_WithResume_UserCancel_EarlyClose  — 端到端 C 短路

**测试设计**:
- 真实 SessionOrchestrator + 真实 InMemoryPendingResolutionStore
- 验证 1 个 "complete" event + channel close
- 验证 Metadata 5 字段 (escape.resume/action/reason/pending_id/exit_reason_source)

**验收**:
- [x] 2/2 PASS

---

## T04 [IMPLEMENTED] ProcessMessage 入口接入

**File:** `internal/layers/orchestration/sessionorchestrator/orchestrator.go`

**改动**:
```go
// 在 buildObserveRequest 之后, classify 之前
if resumeCh, shortCircuit, _ := o.applyResumeSession(ctx, req, sessionSpan); shortCircuit {
    endSpan(sessionSpan)
    return resumeCh, nil
}
```

**验收**:
- [x] 22/22 orchestration 包 go test -race 全 PASS
- [x] 集成测试 PASS

---

## T05 [IMPLEMENTED] spec.md v4.9.0 → v4.10.0

**File:** `openspec/specs/d7-orchestration/spec.md`

**改动**:
- 版本号 v4.9.0 → v4.10.0
- 新增 4.10.0 entry 详述 V5.6 设计 + 6 测试 + 3 fail-safe

**验收**:
- [x] spec.md diff 合规

---

## T06 [IMPLEMENTED] t-registry v3.17.0 → v3.18.0 (T12 PARTIAL → IMPLEMENTED)

**File:** `openspec/specs/d7-orchestration/t-registry.md` + `openspec/t-registry.md`

**改动**:
- 域 t-registry v3.17.0 → v3.18.0
- 域 t-registry D7-S14 T12 PARTIAL → IMPLEMENTED
- 域 t-registry D7-S14 18/18 IMPLEMENTED, 0 PARTIAL
- 域 t-registry D7 总数 180 → 186 (含 V5.6 新增 6 测试)
- 域 t-registry D7 P0 147 → 153
- 根 t-registry v4.8.0 → v4.9.0
- 根 t-registry D7 row 184 → 186 IMPLEMENTED
- 根 t-registry 总 PARTIAL 2 → 0

**验收**:
- [x] 根 t-registry IMPLEMENTED 499 / PARTIAL 0 / PLANNED 3 / P0 318
- [x] 域 t-registry 计数自洽

---

## 总结

| 任务 | 状态 | 文件 |
|------|------|------|
| T01 applyResumeSession 实现 | IMPLEMENTED | escape_wiring.go |
| T02 6 单元测试 | IMPLEMENTED | orchestrator_resume_test.go |
| T03 2 集成测试 | IMPLEMENTED | orchestrator_resume_test.go |
| T04 ProcessMessage 接入 | IMPLEMENTED | orchestrator.go |
| T05 spec.md 同步 | IMPLEMENTED | spec.md |
| T06 t-registry 同步 | IMPLEMENTED | t-registry.md (×2) |

**D7-S14-A50-T12 PARTIAL → IMPLEMENTED** ✅
