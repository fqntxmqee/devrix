# Proposal: D7 MUPS v5 PR-V5.6 Review 修复 — t-registry 一致性 + audit 对齐 + span attrs + 测试覆盖

**Change ID:** `devrix-d7-mups-v5-escape-engine-v5-6-review-fixes`
**Demand ID:** DM-20260625-004
**Priority:** P0 (2 Critical t-registry 不一致 + 2 High 语义对齐 + 2 High 测试覆盖 + 1 Medium 守卫)
**Estimated Effort:** 0.5 天
**PR Count:** 2 (Step 1 docs-only + Step 2 code + test)
**Status:** S2_Proposal → S4_Implementing(hotfix 路径) → S5_Accepted → S6_Archived
**Parent:** `devrix-d7-mups-v5-escape-engine-v5-6` (DM-20260625-003 PR #200)
**SoT:** PR #200 commit `532ebea` 整体审查(4-agent 并行 review)

---

## 1. 背景

DM-20260625-003 PR-V5.6 (`532ebea`) 已 2026-06-24 squash-merged,后续 doc sync PR #201 (`e729660`) 已同步 demand-archive-index.md。

整体审查发现 **14 条问题**(3 Critical + 4 High + 4 Medium + 3 Info),按 V4 review-fixes (DM-20260625-002) 同样 hotfix 路径(feedback-devrix-bugfix-skip-openspec)推进,跳过 S3 完整立项,本提案即为后置的 S1-S3 文档。

## 2. 范围

### 2.1 包含(本 change 修复,7 条)

| ID | 类型 | 文件 | 修复要点 |
|---|---|---|---|
| **C-1** | Critical | `openspec/specs/d7-orchestration/t-registry.md:567, 591` | 删除 T12 描述中的 `runLoopWithResume`(代码里**根本不存在**),改为 `applyResumeSession + resumeContentForDecision` |
| **C-2** | Critical | `t-registry.md:434, 449, 450, 498-502` | Statistics 总表 `186\|184\|2\|0\|153` → `186\|186\|0\|0\|153`;by-Scenario D7-S11 `13\|12\|1\|0` → `13\|13\|0\|0`(T13 Phase 6 已闭环);by-Scenario D7-S14 `18\|17\|1\|0` → `18\|18\|0\|0`(T12 已闭环);Revision History 追加 v3.18.0 条目 |
| **H-1** | High | `escape_wiring.go:134-135, 189-190` | "补写 audit" 注释与代码脱节,实际 audit 在 V5.4 `HumanArbitrator.SubmitUserChoice` 已写;resume 是 read-only。改注释说明 |
| **H-2** | High | `orchestrator.go:338-341` | 短路早退时 `priorSessionSpanAttrs(prior, observeReq, req)` 被跳过,D5 trace 上 user_accept/user_cancel 短路的 sessionSpan 永久缺失 `learn.prior.{alpha,beta,mean,track_mode,injected_at}` 5 attr。修复:shortCircuit 分支 endSpan 之前先 `sessionSpan.SetAttributes(priorSessionSpanAttrs(...))` |
| **H-3** | High | `orchestrator_resume_test.go` | 6 个单元测试全部传 nil span,0 验证 4 处 `sessionSpan.SetAttributes` 写入路径。新增 1 个 test 覆盖 |
| **H-4** | High | `orchestrator_resume_test.go` | 集成测试未断言 5 节点未触发 + Metadata 4 字段 (`escape.pending_id`/`escape.reason`/`exit_reason_source`) 漏断言 |
| **M-1** | Medium | `escape_wiring.go:141` 之前 | `req.SessionID == ""` 静默降级 fail-safe (命中 slog.Warn),契约违反但触发瞬时错误日志。加守卫:入口 `if req.SessionID == "" { return nil, false, nil }` 优先于 engine 调用 |

**外加(顺手):**
- **M-3** dead code `stubResume` 删除(`orchestrator_resume_test.go:37-52`,已用 `errStore` + 真实 `*HumanArbitrator` 替代)

### 2.2 不包含(后续 cleanup change 处理)

- **M-2** fail-safe 2/3 span attr 对称(语义不影响,优先级低)
- **M-4** V5.5 archive 描述(archive 不可改,放 V5.6 proposal 注释即可)
- **I-1** `Metadata["escape.resume"]` forward-compat marker(保留无副作用)
- **I-2** 6 个单元测试改 table-driven(风格)
- **I-3** `applyResumeSession` 签名 error 长期 unused(等 V5.7)
- **I-4** sessionSpan 短路 endSpan 不带 err 注释(design.md 加 1 行说明)

## 3. 实施路径

### Step 1 (docs-only PR) — t-registry 内部一致性

**文件**:仅 `openspec/specs/d7-orchestration/t-registry.md`

**改动**:
1. **C-1** line 567 T12 detail:删除 `+ runLoopWithResume (depth 续 T1 状态由 LoopDepthTracker 自动保证)`,改为 `+ resumeContentForDecision (6 类 EscapeAction → 中文 text)`
2. **C-1** line 591 detail tree:`├── T12 ResumeSession + applyResumeSession + runLoopWithResume` → `├── T12 ResumeSession + applyResumeSession + resumeContentForDecision`
3. **C-2** line 434 Statistics: `186 | 184 | 2 | 0 | 153` → `186 | 186 | 0 | 0 | 153`
4. **C-2** line 449 D7-S11: `13 | 12 | 1 | 0` → `13 | 13 | 0 | 0`
5. **C-2** line 450 D7-S14: `18 | 17 | 1 | 0` → `18 | 18 | 0 | 0`
6. **C-2** line 502 之后追加 `| **3.18.0** | **2026-06-25** | **devrix-d7-mups-v5-escape-engine-v5-6 (DM-20260625-003) PR-V5.6 T12 PARTIAL→IMPLEMENTED**：(D7-S14-A50-T12 IMPLEMENTED。SessionOrchestrator.applyResumeSession 实现 + 3 层 fail-safe + 3 决策路由 + 3 sessionSpan attrs + 6 单元 + 2 集成测试 8/8 PASS。IMPLEMENTED 184→186, PARTIAL 2→0, Scenarios D7-S11 0→0 + D7-S14 0→0)`

**验收**:
- [x] `git diff` 干净(1 file, ~10 行)
- [x] 数字与代码 100% 一致(`186 | 186 | 0 | 0 | 153`)
- [x] verify-archive.sh 12/12 PASS
- [x] PR auto-merge

### Step 2 (code + test PR) — H-1/H-2/M-1/M-3 代码修复 + H-3/H-4 测试覆盖

**文件**:
- `internal/layers/orchestration/sessionorchestrator/escape_wiring.go`(H-1 注释 + M-1 守卫)
- `internal/layers/orchestration/sessionorchestrator/orchestrator.go`(H-2 prior attrs 短路补写)
- `internal/layers/orchestration/sessionorchestrator/orchestrator_resume_test.go`(H-3 + H-4 新增测试 + M-3 删 stubResume)

**改动 H-1**(`escape_wiring.go:132-135, 189-190`):
```go
// 修改前 (注释):
//   - B user_accept → EscapeForceExit → emit "complete" + 补写 audit
//   - C user_cancel → EscapeAbortWithAudit → emit "complete" + 补写 audit

// 修改后:
//   - B user_accept → EscapeForceExit → emit "complete" (audit already recorded at
//     SubmitUserChoice time (V5.4); resume is read-only)
//   - C user_cancel → EscapeAbortWithAudit → emit "complete" (audit already recorded)

// 修改前 (line 189-190):
// Terminal decision (B=user_accept → ForceExit, C=user_cancel → AbortWithAudit):
// emit single "complete" EngineEvent + 补写 audit + close channel early.

// 修改后:
// Terminal decision (B=user_accept → ForceExit, C=user_cancel → AbortWithAudit):
// emit single "complete" EngineEvent (audit already written at SubmitUserChoice time) + close channel early.
```

**改动 M-1**(`escape_wiring.go:140-148`,在 `o.escapeEngine == nil` 检查**之前**加守卫):
```go
func (o *SessionOrchestrator) applyResumeSession(
    _ context.Context,
    req orchtypes.ProcessRequest,
    sessionSpan tracer.Span,
) (<-chan *contracts.EngineEvent, bool, error) {
    // M-1 guard: empty SessionID is contract violation, not transient error.
    if req.SessionID == "" {
        if sessionSpan != nil {
            sessionSpan.SetAttributes(tracer.Attribute{
                Key: "escape.resume.attempted", Value: "false",
            })
        }
        return nil, false, nil
    }
    if o.escapeEngine == nil {
        ...
```

**改动 H-2**(`orchestrator.go:338-341`):
```go
// 修改前:
if resumeCh, shortCircuit, _ := o.applyResumeSession(ctx, req, sessionSpan); shortCircuit {
    endSpan(sessionSpan)
    return resumeCh, nil
}

// 修改后:
if resumeCh, shortCircuit, _ := o.applyResumeSession(ctx, req, sessionSpan); shortCircuit {
    // H-2: short-circuit path also writes prior attrs so D5 trace has consistent
    // learn.prior.{alpha,beta,mean,track_mode,injected_at} for resume decisions.
    if sessionSpan != nil {
        sessionSpan.SetAttributes(priorSessionSpanAttrs(prior, observeReq, req)...)
    }
    endSpan(sessionSpan)
    return resumeCh, nil
}
```

**改动 H-3**(`orchestrator_resume_test.go` 新增 test):
```go
// TestApplyResumeSession_SessionSpanAttrs 验证 4 处 SetAttributes 写入路径。
// nil engine / ResumeSession error / !found / found+terminal 4 个分支。
func TestApplyResumeSession_SessionSpanAttrs(t *testing.T) {
    // 用 mock tracer.Span (已存在 recordingTracer 模式)
    // 4 个 sub-test 验证每个分支的 attr 写入
}
```

**改动 H-4**(`orchestrator_resume_test.go` 增强):
- 集成测试加 `recordingExecutor.calls == 0` 断言
- 集成测试加 `events[0].Metadata["escape.pending_id"]` + `["escape.reason"]` + `["exit_reason_source"]` 断言

**改动 M-3**(`orchestrator_resume_test.go`):
- 删除 `stubResume` 类型(line 37-52),保留注释说明走 `errStore` + 真实 `*HumanArbitrator` 路线

**验收**:
- [x] `go build ./...` 通过
- [x] `go vet ./...` 0 issue
- [x] 22/22 orchestration packages `go test -race` PASS(0 race)
- [x] 8/8 V5.6 单元 + 集成测试 PASS(含 H-3 新增)
- [x] 集成测试断言完整:recordingExecutor.calls == 0 + 5 Metadata 字段
- [x] verify-archive.sh 12/12 PASS
- [x] PR auto-merge

## 4. 工作量

| 任务 | 行数 | 时间 |
|---|---|---|
| Step 1 docs-only PR | ~10 行(1 file) | 0.05 天 |
| Step 2 code + test PR | ~80 行(4 files) | 0.30 天 |
| S6 archive + demand-archive-index sync | ~20 行(1 file) | 0.05 天 |
| **总计** | **~110 行** | **0.40 天** |

## 5. 验收

- Step 1 PR + Step 2 PR 全部 squash-merged
- t-registry 数字与代码 100% 一致
- 22/22 orchestration packages PASS + verify-archive.sh 12/12 PASS
- 14 → 7 修复(本 change),剩余 7 条(M/I 级别)留待后续 cleanup change
- DM-20260625-003 在 demand-archive-index.md line 109 描述保持不变(V5.6 主工作已完成 + doc sync 已 PR #201)