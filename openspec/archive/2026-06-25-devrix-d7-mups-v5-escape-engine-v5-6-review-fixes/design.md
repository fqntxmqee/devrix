# Design: D7 MUPS v5 PR-V5.6 Review 修复

**Change ID:** `devrix-d7-mups-v5-escape-engine-v5-6-review-fixes`
**Demand ID:** DM-20260625-004
**Design Status:** Approved (S3-Gate 与 S4 实现同步,hotfix 路径)
**Created:** 2026-06-25

---

## 1. 整体架构

本 change 是 V5.6 PR #200 (`532ebea`) 的"事后 review 修复",与 V5.6 主工作是 **parent → child** 关系:

```
V5.6 主工作 (PR #200, S7_Archived)
├── SessionOrchestrator.applyResumeSession 落地 + 8 测试 + 3 spec/t-registry
└── doc sync PR #201 (demand-archive-index.md line 109)
    │
    └── V5.6 Review 修复 (本 change, DM-20260625-004)
        ├── Step 1 docs-only PR (C-1 + C-2 t-registry 内部一致性)
        └── Step 2 code + test PR (H-1 + H-2 + H-3 + H-4 + M-1 + M-3)
```

## 2. 修复点设计

### 2.1 C-1:runLoopWithResume 不存在

**问题**:`t-registry.md:567, 591` 描述 T12 包含 `runLoopWithResume` 作为独立函数,但 Go 代码 grep 0 命中。

**设计简化**:`runLoopWithResume` 在 V5.6 实际合并到 `EscapeContinue` fall through 路径 — 用户点击 A 决策后,`applyResumeSession` 返回 `(nil, false, nil)`,caller 走完整 5 节点(包括 LoopDepthTracker 自动 depth tracking),不需要额外 wrapper 函数。

**修复**:t-registry 描述同步。

### 2.2 C-2:Statistics 表 4 处数字未刷新

**问题**:`t-registry.md:434` 总表 + line 449 D7-S11 + line 450 D7-S14 + line 498-502 Revision History 全部停留在 V5.5 状态,与 V5.6 实际不符。

**修复**:5 处数字更新,数学:
- Total = 186 (V5.6 新增 6 测试 = 180+6,正确)
- IMPLEMENTED = 186 = Total (PARTIAL 全闭环)
- PARTIAL = 0
- D7-S11: T13 已 Phase 6 闭环,13/13/0/0
- D7-S14: T12 已 V5.6 闭环,18/18/0/0
- Revision History 追加 v3.18.0 entry

### 2.3 H-1:audit 注释脱节

**问题**:`escape_wiring.go:134-135, 189-190` 注释 "emit complete + 补写 audit",但 applyResumeSession **未调用** `escapeEngine.AuditLog().Record(...)`。

**实际行为**(grep `arbitrator.go:455` 验证):
- audit 在 V5.4 `HumanArbitrator.SubmitUserChoice` (`arbitrator.go:559`) 时已写
- V5.3 `ResumeSession` (one-shot consume) 是 read-only Load + Delete
- resume 路径不需要再写 audit

**修复**:改注释,与代码行为对齐。**不补写 audit**(避免与 V5.4 重复记录,且补写会引入新的 audit key 需要 D5 配套)。

### 2.4 H-2:短路早退 prior attrs 漏写

**问题**:`orchestrator.go:338-341` 短路返回时跳过 `priorSessionSpanAttrs(prior, observeReq, req)` (line 346-348 Phase 7 PR-7.3)。

**修复**:在 `endSpan(sessionSpan)` 之前补写 prior attrs,保持 trace 一致性。

```go
if resumeCh, shortCircuit, _ := o.applyResumeSession(ctx, req, sessionSpan); shortCircuit {
    if sessionSpan != nil {
        sessionSpan.SetAttributes(priorSessionSpanAttrs(prior, observeReq, req)...)
    }
    endSpan(sessionSpan)
    return resumeCh, nil
}
```

### 2.5 H-3:span attr 测试 0 覆盖

**问题**:6 个单元测试全部传 nil span,4 处 SetAttributes 路径未验证。

**修复**:新增 `TestApplyResumeSession_SessionSpanAttrs`,用 mock `tracer.Span`(grep 现有 mock pattern,如 `internal/layers/observability/instrument/tracer/` 测试)覆盖 4 分支:
- nil engine → `attempted="false"`
- err failsafe → `attempted="true"`, `decision_action="error_failsafe"`
- not found → `attempted="true"`
- found terminal → `attempted="true"`, `decision_action=<action>`, `decision_pending_id=<id>`

### 2.6 H-4:集成测试断言不全

**问题**:`TestProcessMessage_WithResume_*_EarlyClose` 未断言:
1. 5 节点 executor **未被调用** (`recordingExecutor.calls == 0`)
2. Metadata `escape.pending_id` / `escape.reason` / `exit_reason_source` 3 字段

**修复**:`recordingExecutor` (仿 V5.5 测试模式) + 5 行 Metadata 断言。

### 2.7 M-1:空 sessionID 守卫

**问题**:`req.SessionID == ""` 时,`InMemoryPendingResolutionStore.Load("")` 返回 `(zero, false, err)`,命中 fail-safe 2 路径,触发 `slog.Warn` — 空 sessionID 是契约违反,非瞬时错误,误诊日志。

**修复**:入口加守卫,优先于 engine 调用:
```go
if req.SessionID == "" {
    if sessionSpan != nil {
        sessionSpan.SetAttributes(tracer.Attribute{
            Key: "escape.resume.attempted", Value: "false",
        })
    }
    return nil, false, nil
}
```

### 2.8 M-3:dead code stubResume 删除

**问题**:`orchestrator_resume_test.go:37-52` `stubResume` 类型定义但 0 引用。

**修复**:删除 16 行 + 注释说明走 `errStore` + 真实 `*HumanArbitrator` 路线。

## 3. 测试设计

### 3.1 H-3 新增单测

```go
func TestApplyResumeSession_SessionSpanAttrs(t *testing.T) {
    // 4 sub-test: nil engine / err failsafe / not found / found terminal
    // 每个 case 用 mock SpanRecorder 验证 SetAttributes 调用次数 + key/value
}
```

### 3.2 H-4 集成测试增强

```go
func TestProcessMessage_WithResume_UserAccept_EarlyClose(t *testing.T) {
    // ...existing setup...
    rec := &recordingExecutor{}
    orch := NewSessionOrchestrator(..., WithEscapeEngine(engine))  // (替换 completingExecutor)
    
    ch, err := orch.ProcessMessage(...)
    // ...existing event assertions...
    
    // NEW: 5 节点未触发
    if rec.calls != 0 { t.Errorf("executor.calls: want 0 (short-circuit), got %d", rec.calls) }
    // NEW: Metadata 5 字段
    for _, k := range []string{"escape.resume", "escape.action", "escape.reason", "escape.pending_id", "exit_reason_source"} {
        if _, ok := events[0].Metadata[k]; !ok { t.Errorf("missing metadata[%q]", k) }
    }
}
```

## 4. 关键文件改动汇总

| 文件 | 改动 | 行数 |
|---|---|---|
| `openspec/specs/d7-orchestration/t-registry.md` | C-1 删除 `runLoopWithResume` + C-2 Statistics/Revision History 更新 | ~10 行 |
| `internal/layers/orchestration/sessionorchestrator/escape_wiring.go` | H-1 注释修正 + M-1 空 sessionID 守卫 | ~10 行 |
| `internal/layers/orchestration/sessionorchestrator/orchestrator.go` | H-2 短路补写 prior attrs | +4 行 |
| `internal/layers/orchestration/sessionorchestrator/orchestrator_resume_test.go` | H-3 新增 1 test + H-4 增强 + M-3 删除 stubResume | +50 行 / -16 行 |

## 5. 不在本次范围

- ❌ 不修改 V5.6 主工作 PR #200(已 squash-merged)
- ❌ 不修改 doc sync PR #201(已 merged)
- ❌ 不修改 V5.5 archive(不可改)
- ❌ 不创建新的 spec.md 版本号(纯 t-registry 修正)
- ❌ 不修改 V5.6 archive 目录(`archive/2026-06-25-devrix-d7-mups-v5-escape-engine-v5-6/`)
- ❌ 不实现 fail-fast 语义(applyResumeSession 第三个返回值 error 维持 reserved)