# Proposal: D7 MUPS v5 统一逃逸机制 — PR-V5.6 T2 ResumeSession SessionOrchestrator 入口收口

**Change ID:** `devrix-d7-mups-v5-escape-engine-v5-6`
**Demand ID:** DM-20260625-003
**Priority:** P0
**Sprint:** mups-v5
**Estimated Effort:** 0.6 天
**PR Count:** 1
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**Parent Change:** `2026-06-25-devrix-d7-mups-v5-escape-engine` (V5.1..V5.5 已 S7_Archived, T12 PARTIAL 留待本 PR-V5.6)
**SoT:** `brain/.../core-concepts/38-mature-uncertainty-methodology.md §21` (V5.6 收口)

---

## 1. 背景

MUPS v5 统一逃逸机制（V5.1..V5.5, PR #195..#198）已于 2026-06-25 全部 S7_Archived。V5.3 实现了 HumanArbitrator.ResumeSession (T2 续跑, 设计稿见 doc 38 §21.3.6)，但 SessionOrchestrator.ProcessMessage 入口尚未接入 T2 ResumeSession 续跑检查。

当前现状 (V5.5 落地后)：
- **T12 ResumeSession 标记 PARTIAL** (V5.5 archive 留待 PR-V5.6) — 设计稿 §22 (V5.6)
- EscapeEngine.ResumeSession 已存在（V5.3 落地 + 单元测试覆盖）
- PendingResolutionStore (HumanArbitrator) 已存在（V5.3 落地）
- SessionOrchestrator.ProcessMessage 入口未调用 ResumeSession

**V5.6 收口目标**：让 SessionOrchestrator.ProcessMessage 在 T1 (EscapePendingHuman 状态) → T2 (用户已选择 A/B/C) 续跑时，根据 PendingResolutionStore 中的用户决策：
- A user_continue → 继续走完整 5 节点
- B user_accept → 立即 emit "complete" + EscapeForceExit
- C user_cancel → 立即 emit "complete" + EscapeAbortWithAudit

## 2. 范围

### 2.1 包含

- `SessionOrchestrator.applyResumeSession(ctx, req, sessionSpan)` 方法
  - 3 层 fail-safe (nil engine / ResumeSession error / TTL expired → fall through)
  - terminal decision (B/C) → emit "complete" EngineEvent + close channel early
  - non-terminal (A) → fall through to full 5-node pipeline
- `resumeContentForDecision(d EscapeDecision) string` helper
  - 6 类 EscapeAction → 中文 text 消息
- `ProcessMessage` 入口插入点 (after buildObserveRequest, before classify)
- sessionSpan 3 attributes (escape.resume.attempted / decision_action / decision_pending_id)
- 8 测试 (6 单元 + 2 集成)，100% PASS

### 2.2 不包含

- LoopDepthTracker / PlanKindSwitchPolicy / ChainedArbitrator / EscapeEngine 主体（V5.1..V5.4 已落地）
- 5 节点接线 1a/1b/2/3（V5.5 已落地）
- PendingResolutionStore TTL 调整
- Learn 节点后续影响 (V5.6 短路早退，learner 不参与)

## 3. 实现路径

### 3.1 applyResumeSession 设计

```go
// 入参: ctx, req (含 SessionID), sessionSpan (可空)
// 出参: (ch, shortCircuit, err)
//   - ch != nil + shortCircuit=true → 短路早退, caller 返 ch
//   - ch=nil + shortCircuit=false → fall through, caller 走 5 节点
//   - err 当前未使用 (3 层 fail-safe 兜底)
//
// 6 类 fail-safe + 3 类决策处理:
//   1. nil engine        → (nil, false, nil)
//   2. ResumeSession err → (nil, false, nil) + slog.Warn
//   3. found=false (TTL) → (nil, false, nil)
//   4. EscapeContinue (A user_continue) → (nil, false, nil)
//   5. EscapeForceExit (B user_accept) → (ch, true, nil) + emit "complete"
//   6. EscapeAbortWithAudit (C user_cancel) → (ch, true, nil) + emit "complete"
//   7. EscalateTo* / EscapePendingHuman → (ch, true, nil) (兜底 emit "complete")
```

### 3.2 短路时 EngineEvent 设计

```go
&contracts.EngineEvent{
    Type:      "complete",
    Content:   "（用户接受当前结果）"  // 中文 text, 飞书卡片可直接显示
    SessionID: req.SessionID,
    Metadata: map[string]string{
        "escape.resume":      "true",
        "escape.action":      "force_exit",  // EscapeAction.String()
        "escape.reason":      "user_accept", // 决策 reason
        "escape.pending_id":  decision.PendingID,
        "exit_reason_source": "user_resume",  // Phase 4 Verifier 反向追溯
    },
}
```

### 3.3 sessionSpan 3 attributes

- `escape.resume.attempted` (true|false|error_failsafe)
- `escape.resume.decision_action` (action.String())
- `escape.resume.decision_pending_id` (decision.PendingID)

## 4. 验收

- 6 单元测试 + 2 集成测试 100% PASS
- 22/22 orchestration 包 go test -race 全 PASS
- spec.md v4.9.0 → v4.10.0
- t-registry v3.17.0 → v3.18.0 (D7-S14 T12 PARTIAL → IMPLEMENTED, 18/18 IMPLEMENTED)
- 根 t-registry v4.8.0 → v4.9.0 (D7 184→186 IMPLEMENTED, PARTIAL 2→0)
- verify-archive.sh 全部通过

## 5. 工作量

| 内容 | 行数 | 时间 |
|------|------|------|
| applyResumeSession + helper | ~140 行 | 0.3 天 |
| 8 测试 | ~340 行 | 0.2 天 |
| 文档同步 (spec + t-registry) | ~50 行 | 0.1 天 |
| **总计** | **~530 行** | **0.6 天** |
