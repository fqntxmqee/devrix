# Demand: D7 MUPS v5 PR-V5.6 T2 ResumeSession SessionOrchestrator 入口收口

**Change ID:** `devrix-d7-mups-v5-escape-engine-v5-6`
**Demand ID:** DM-20260625-003
**Created:** 2026-06-25
**Parent Demand:** DM-20260625-003 (V5.1..V5.5 已 S7_Archived, T12 PARTIAL 留待本 PR-V5.6)
**Priority:** P0

---

## 1. 业务诉求

MUPS v5 回路深度统一逃逸机制 (V5.1..V5.5) 已于 2026-06-25 全部 S7_Archived。但 T12 (ResumeSession T2 续跑 SessionOrchestrator 入口) 标记为 PARTIAL——V5.3 已实现 HumanArbitrator.ResumeSession (one-shot consume)，但 SessionOrchestrator.ProcessMessage 入口尚未接入该检查。

**用户场景**：当 ProcessMessage 在 T1 触发 EscapePendingHuman（飞书卡片弹出 A/B/C 三选一按钮），用户离线 30s 后点击"接受当前结果"（B 按钮），下一条 ProcessMessage 应该立即 emit "complete" 事件 + EscapeForceExit，不再走完整 5 节点流程。

**当前缺陷**：用户选择 B/C 后，下次 ProcessMessage 仍走完整 Observe → Plan → Execute → Verify → Learn 流程，浪费 1-3s 时间 + 占用回路深度计数，**与 V5.3 设计稿不符**。

## 2. 验收标准

### 2.1 功能验收 (P0 必过)

- [x] `applyResumeSession` 方法在 ProcessMessage 入口正确检查 PendingResolutionStore
- [x] nil engine / ResumeSession error / TTL expired 3 类 fail-safe 全部 fall through
- [x] B user_accept → emit "complete" + EscapeForceExit
- [x] C user_cancel → emit "complete" + EscapeAbortWithAudit
- [x] A user_continue → fall through (不破坏主链路)
- [x] sessionSpan 3 attributes 正确写入 D5 observability

### 2.2 测试验收

- [x] 6 单元测试 PASS (TestApplyResumeSession_*)
- [x] 2 集成测试 PASS (TestProcessMessage_WithResume_*)
- [x] 22/22 orchestration 包 go test -race 全 PASS
- [x] 3/3 V5.6 测试稳定性验证 PASS

### 2.3 文档验收

- [x] spec.md v4.9.0 → v4.10.0 (新增 4.10.0 entry)
- [x] t-registry v3.17.0 → v3.18.0 (T12 PARTIAL → IMPLEMENTED)
- [x] 根 t-registry v4.8.0 → v4.9.0 (D7 PARTIAL 2→0)

### 2.4 流程验收

- [x] S6 archive (verify-archive.sh) 全部通过

## 3. 设计决策

### 3.1 插入点选择

- 选 `ProcessMessage` 入口, 在 `buildObserveRequest` 之后, `classify` 之前
- 原因:
  1. resume 决策不应受 LP-1 prior 影响（prior 是为了调节 Observe 决策）
  2. 短路早退应在 classify 之前，避免 LLM 消耗 token

### 3.2 fail-safe 策略

3 层 fail-safe 全部 fall through（不抛 error），保证主链路鲁棒性：
1. nil engine → fall through (兼容 V5.4- 行为)
2. ResumeSession error → slog.Warn + fall through (V5.3 ResumeSession 内部 panic recover 已处理)
3. TTL expired → fall through (正常续跑)

### 3.3 terminal decision 短路 vs 报错

- 选 emit "complete" + close channel, 不返回 error
- 原因:
  1. 与 V5.5 ForceExit 接线点 (1a/1b/2) 不同：V5.5 是 wiring point 检测到 ForceExit 后 return error；V5.6 是入口短路, 已经走完 5 节点 pipeline 后用户主动选择, 应当正常返回 "complete" + 配合用户决策
  2. 飞书卡片 / CLI 终端需要"正常完成"语义，而不是错误

## 4. 影响域

- SessionOrchestrator: 1 个新方法 + 1 个新 helper + ProcessMessage 1 处调用
- spec.md / t-registry: 文档同步
- 不影响其他域 (D1-D6)
- 不影响其他 escape 子包 (LoopDepthTracker / PlanKindSwitchPolicy / ChainedArbitrator / EscapeEngine / CircuitBreaker / AuditLog)

## 5. 回滚方案

回滚 V5.6 commit 即可，不影响 V5.1..V5.5 已落地能力。SessionOrchestrator 行为降级为 V5.5 (无 T2 续跑入口)。
