# Devrix T 层测试点注册表（索引）

**Status:** Active
**Version:** 4.9.0
**Last Updated:** 2026-06-25
**Layering Spec:** `openspec/specs/project/dsaft-methodology.md`

---

## Overview

本文档为 Devrix T 层注册表的**索引入口**。各域的 T 层测试点已拆分为独立文件。

> **编号格式**: `D{X}-S{X}-A{XX}-T{XX}`（T 归属 A）或 `D{X}-S{X}-A{XX}-F{XX}-T{XX}`（T 归属 F）
>
> **横切契约域** (TOOL-SURFACE-1 / PERMISSION-GATE-1) T 点用 `TOOL-SURFACE-1-T{nn}` / `PERMISSION-GATE-1-T{nn}` 平铺编号，归属 D2 (Context Engine) + D7 (Orchestration) 共同 consumption。

---

## 域级注册表

| 域 | 路径 | Total | IMPLEMENTED | PLANNED | P0 |
|----|------|-------|-------------|---------|-----|
| D1 Communication | `openspec/specs/d1-communication/t-registry.md` | 60 | 60 | 0 | 30 |
| D2 Context Engine | `openspec/specs/d2-context-engine/t-registry.md` | 114 | 114 | 0 | 61 |
| D3 LLM Gateway | `openspec/specs/d3-llm-gateway/t-registry.md` | 36 | 35 | 1 | 20 |
| D4 Multi-Agent | `openspec/specs/d4-multi-agent/t-registry.md` | 40 | 40 | 0 | 21 |
| D5 Observability | `openspec/specs/d5-observability/t-registry.md` | 44 | 42 | 0 | 27 |
| D6 Evolution | `openspec/specs/d6-evolution/t-registry.md` | 24 | 22 | 2 | 6 |
| D7 Orchestration | `openspec/specs/d7-orchestration/t-registry.md` | 186 | 186 | 0 | 153 |

**总计**: 504 · IMPLEMENTED 499 · PLANNED 3 · PARTIAL 0 · P0 318

> 2026-06-20 增量：DM-20260620-003 (devrix-error-handling-tier1-tier2) — 8 个 P0 T 点（D7-S1-T18 + D7-S2-A02-T18 + D7-S2-A06-T24/T25/T26/T27 + D5-S23-A06-T03 + D3-S3-A01-T16）— 全 IMPLEMENTED。
> 详见 `docs/error-handling.md` §1-9 (SentinelError 类型统一 + SanitizeForUser + 子 agent stream 哨兵 + retry nil-sentinel)。

> 2026-06-20 增量：DM-20260620-001 (Phase A) + DM-20260620-001-B (Phase B) + DM-20260620-002 (Phase C) — 三个 change 共加 22 个 P0 T 点（D1-S2-A05-T05~T08 = 4 + D2-S17-A05 T01-T05 + D2-S17-A06 T01-T03 + D2-S15-A08 T01-T10 = 18）— 全 IMPLEMENTED。
> 详见 `docs/context-budget.md` §9 (Phase C nested-branch budget injection) + §1-8 (Phase A/B 入口隔离 + 多 turn budget audit)。

> 2026-06-18 增量：DM-20260618-001/002/003 三个 change 共加 15 个 P0 T 点（T22-T34 + PERMISSION-GATE-1-T01/T02）— 全 IMPLEMENTED。
> 详见 `openspec/specs/d2-context-engine/t-registry.md` §"TOOL-SURFACE-1: v2 / v3 / Lazy Loading"。

> 2026-06-18 增量：DM-20260618-007 (devrix-tools-terminal-architecture) 5 Surface × 跨切面 LTL-Lite — 加 25 个 T 点 (D2-S4-A01-T01~T06 + TOOL-SEC-2-A02-T01~T03 + D5-S23-A02-T01~T04 + D4-S11-A02-T01~T04 + D4-S13-A02-T01 + D6-S11-A02-T01~T03 + D4-S12-A03-T01 + PERMISSION-GATE-1-T01/T02/T03) — 全 IMPLEMENTED。
> 详见 `openspec/changes/devrix-tools-terminal-architecture/acceptance-report.md` §2 T 层验证。

> 2026-06-22 增量：DM-20260622-001 (devrix-d7-metrics-and-concurrency-hardening) — D7 编排层 metric 命名 spec/code 对齐 + 并发硬化：加 6 个 P0 T 点（D7-S6-A14-T01 dispatch_loop_wakeups plural + T02 worker_panics plural + T03 sandbox_exit_failed 跨域归属 D4 + T04 state.cancels/handles markWaveDone 释放 + T05 ConflictGuard hot path AllowAndRegister 原子化 + T06 CommandHandler emit select-default 防阻塞）— 全 IMPLEMENTED。D7 t-registry v3.7.0 → v3.8.0 (P0 90→96, IMPLEMENTED 123→129)。
> 详见 `openspec/archive/2026-06-22-devrix-d7-metrics-and-concurrency-hardening/acceptance-report.md` §2 T 层验证 + `openspec/changes/devrix-d7-metrics-and-concurrency-hardening/proposal.md` §2 5 fix 清单。

> 2026-06-25 增量：DM-20260625-003 (devrix-d7-mups-v5-escape-engine) — MUPS v5 统一逃逸机制 (LoopDepthTracker v2 + PlanKindSwitchPolicy + EscapeAction 6 类 + ChainedArbitrator LLM/Rule/Human + EscapeEngine + CircuitBreaker 5 层 + AuditLog + 5 节点 EscapeEngine 接线点 + 13 类失败降级矩阵)：加 18 个 P0 T 点（D7-S14-A50 T01..T18）— 17 IMPLEMENTED + 1 PARTIAL (T12 ResumeSession T2 续跑 SessionOrchestrator 入口留待 PR-V5.6)。D7 t-registry v3.16.0 → v3.17.0 (P0 135→153, IMPLEMENTED 168→184, PARTIAL 1→2)。S4-Gate review C-1 修复: processEscapeDecision signature `bool` → `(bool, error)` 透传 augmented error。
> 详见 `openspec/changes/devrix-d7-mups-v5-escape-engine/proposal.md` + `design.md` + `tasks.md` + `specs/d7-orchestration/spec.md`。

> 2026-06-25 增量 (V5.6 续跑入口收口)：DM-20260625-003 PR-V5.6 落地 T12 PARTIAL → IMPLEMENTED — SessionOrchestrator.applyResumeSession(ctx, req, sessionSpan) 在 ProcessMessage 入口 (after buildObserveRequest, before classify) 检查 PendingResolutionStore → 调用 EscapeEngine.ResumeSession (one-shot consume via HumanArbitrator.LoadAndDelete) → 3 层 fail-safe (nil engine / ResumeSession error / TTL 过期 → 静默 fall through) → terminal decision (B=user_accept → EscapeForceExit, C=user_cancel → EscapeAbortWithAudit) emit single "complete" EngineEvent + 补写 audit + close channel early / A=user_continue fall through to full 5-node pipeline。D7 t-registry v3.17.0 → v3.18.0 (D7-S14 T12 PARTIAL → IMPLEMENTED, D7 T 184→186 IMPLEMENTED, P0 147→153, D7 PARTIAL 2→0, 总 PARTIAL 2→0)。6 个单元测试 (TestApplyResumeSession_NoEngine / NoPending / UserAccept / UserCancel / UserContinue / ResumeError_Failsafe) + 2 个集成测试 (TestProcessMessage_WithResume_UserAccept_EarlyClose / TestProcessMessage_WithResume_UserCancel_EarlyClose) 全 PASS。

---

## Legacy ID Mapping

本表记录过渡格式 `D{X}-S{X}-T{NN}` → 标准格式 `D{X}-S{X}-A{XX}-T{NN}` 的映射。

| 旧 ID | 新 ID | 说明 |
|-------|-------|------|
| D1-S2-T03~T08 | D1-S2-A02-T03~T08 | SendOutbound 活动 |
| D1-S9-T02~T04 | D1-S9-A02-T02~T04 | ManageBusLifecycle 活动 |
| D2-S1-T02,T05,T06,T09,T10 | D2-S1-A02-T* | VerifyExecution 活动 |
| D2-S1-T07,T08 | D2-S1-A03-T07,T08 | PlanExecution 活动 |
| D2-S9-T05,T14 | D2-S9-A03-T05,T14 | FilterToolPool 活动 |
| D2-S9-T10,T12,T13 | D2-S9-A02-T10,T12,T13 | AssembleSystemPrompt 活动 |
| D4-S2-T02,T03 | D4-S2-A02-T02,T03 | ResolvePermission 活动 |
| D4-S3-T05 | D4-S3-A02-T05 | JoinAgents 活动 |
| D4-S6-T02~T07 | D4-S6-A02-T02~T07 | ExecuteAgentTool 活动 |
| D4-S10-T04~T07 | D4-S10-A02-T04~T07 | TrackProgress 活动 |
| D6-S3-T02 | D6-S3-A02-T02 | JudgeResult 活动 |
| CROSS-T03 | CROSS-A02-T03 | CheckContracts 活动 |
| D4-S12-T01 | D2-S12-A01-T01 | 修正域编号（D4→D2） |
