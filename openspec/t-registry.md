# Devrix T 层测试点注册表（索引）

**Status:** Active
**Version:** 5.10.0
**Last Updated:** 2026-07-01 (devrix-mups-propagation-convergence — DM-20260701-001 — D7 +21 T points IMPLEMENTED 230→251)
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
| D1 Communication | `openspec/specs/d1-communication/t-registry.md` | 61 | 61 | 0 | 31 |
| D2 Context Engine | `openspec/specs/d2-context-engine/t-registry.md` | 114 | 114 | 0 | 61 |
| D3 LLM Gateway | `openspec/specs/d3-llm-gateway/t-registry.md` | 39 | 38 | 1 | 23 |
| D4 Multi-Agent | `openspec/specs/d4-multi-agent/t-registry.md` | 40 | 40 | 0 | 21 |
| D5 Observability | `openspec/specs/d5-observability/t-registry.md` | 44 | 42 | 0 | 27 |
| D6 Evolution | `openspec/specs/d6-evolution/t-registry.md` | 24 | 22 | 2 | 6 |
| D7 Orchestration | `openspec/specs/d7-orchestration/t-registry.md` | 251 | 251 | 0 | 207 |

**总计**: 579 · IMPLEMENTED 574 · PLANNED 3 · PARTIAL 2 · P0 385

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
> 2026-06-26 增量：DM-20260626-001 (devrix-d7-six-s-simplification PR #215) — D7 编排层 6 S 精简 + 5 个新 P0/P1 Span（v6.0.0 域升级）：14 S → **6 S + 1 横切** 博弈角色对齐 (State Authority / Mediator+Turn Leader+Error Recovery / Mechanism Designer / Costly Signaler+Certifier / Information Producer+Quantizer / Pipeline Coordinator+Memory Curator / 横切 Discipline Keeper)；A 活动 56 → **49** (S1:4 · S2:7 · S3:4 · S4:9 · S5:8 · S6:15 + Hardening:2)；F 层 75 → **68** (Legacy 41 + Canonical 27)；T 总数持平（重归类不删）但 S 编号重映射；Span 18 → **23** ops（+5 新 P0/P1: channel.route / memory.persist / system.anomaly_detect / taskgraph.synthesize / executor.select）。加 20 个 P0 T 点：(1) D7-S6-A48 channel.route +2 (T01 EmitChannelRoute happy + T02 nil-bridge fail-safe)；(2) D7-S6-A49 memory.persist +2 (T03 EmitMemoryPersist happy + T04 nil-bridge fail-safe)；(3) D7-S4-A47 system.anomaly_detect +8 (T05-T06 emit + T11-T16 DetectSystemAnomaly 6 测试 triggered/not/override/nil-bridge/default)；(4) D7-S5-A33 taskgraph.synthesize +6 (T07-T08 emit + T17-T19 dagDepth 3 测试 + T20 Span emit fail-safe)；(5) D7-S5-A34 executor.select +2 (T09-T10 emit happy/nil-bridge)。20 新 P0 T 全 IMPLEMENTED，D7 t-registry v3.18.0 → v4.0.0（D7 T 186→206 IMPLEMENTED, P0 153→173）。22/22 orchestration packages `go test -race` 100% PASS 0 race；LP-1 兼容（Phase 4 PR-D4 UncertaintyCoord Value=0.95 路径 0 变化）。
> 详见 `openspec/archive/2026-06-26-devrix-d7-six-s-simplification/acceptance-report.md` §3 20 P0 T 点验收 + §4 6 S 博弈角色重归类验收 + `openspec/specs/d7-orchestration/d7-domain.md` v2.0.0 §DSAFT 资产。

> 2026-06-26 增量（mups 包路径迁移 IMPLEMENTED 收口）：DM-20260626-002 (devrix-d7-mups-package-migration) — v6.0.0 域升级 Step 2 follow-up 落地：execute/ + learn/ → mups/ 子树物理目录迁移完成 (commit cb965d9: 24 文件 git mv rename 100%) + 17 处 import path 全仓替换完成 (commit e22ef5d: decisionplanning 2 + orchtypes 6 + sessionorchestrator 9) + go build/vet/test -race 全绿 (22/22 orchestration packages PASS + 130 全仓包 0 FAIL)。D7 加 4 个 P0 T 点全部 IMPLEMENTED：D7-S6-A51 T01 mups/execute/ 目录 + 7 .go git mv / T02 mups/learn/ 目录 + 17 .go git mv / T03 17 处 import path 全仓替换 + grep 0 残留 / T04 go build/vet/test -race 全绿 (22/22 orchestration pkgs) + LP-1/LP-2/LP-5 路径 0 变化。Total 528, PLANNED 7→3, P0 342（IMPLEMENTED 519→523）。D7 t-registry v4.1.0 → v4.2.0。22 包 baseline 持平 (PR #215 验证)，LP-1/LP-2/LP-5 集成测试 100% 兼容。

> 2026-06-26 增量（Hardening 横切包物理落地）：DM-20260626-003 (devrix-d7-hardening-cross-cutting) — v6.0.0 域升级 Step 3 follow-up 落地：`orchestration/hardening/` 目录新建（5 .go: doc.go + metrics.go + metrics_test.go + recovery.go + recovery_test.go），承接 6 S + 1 横切 Discipline Keeper 横切角色；`sessionorchestrator/metrics.go` (61 行 InterruptMetrics) + `turn/recovery.go` subset（4 纯函数 + 1 const）git mv 迁 hardening/；`escape/circuit_breaker.go` 留 escape/（V5 EscapeEngine 核心机制，Decision 1，git diff 0 变化）；receiver methods（compressMessagesForRecovery + invokeStreamWithRecovery）保留 turn/ 紧耦合 *DefaultOrchestrator（Decision 2）。D7 加 4 个 P0 T 点全部 IMPLEMENTED：D7-S7-A01-T01 hardening/metrics 目录 + 4 测试 git mv / D7-S7-A02-T02 hardening/recovery 子集拆分 + 3 测试 / D7-S7-A01-T03 0 残留 import path 全仓替换 + grep 0 命中 / D7-S7-A01-T04 go build/vet/test -race 全绿 (23/23 orchestration pkgs, 含 hardening 1 新包) + LP-1（Bayesian reputation TestAutoClose_FullLP1Loop）/ LP-2（5 节点 TestIntegration_5NodePipeline_End2End）/ LP-5（Cross-session traceability）100% 兼容。Total 532, P0 346（IMPLEMENTED 523→527, P0 342→346）。D7 t-registry v4.2.0 → v4.3.0。详见 `openspec/archive/2026-06-26-devrix-d7-hardening-cross-cutting/` (S6 归档阶段)。

> 2026-06-26 增量（turn/ → sessionorchestrator/ 整包物理合并）：DM-20260626-004 (devrix-d7-6s-package-merge) — v6.0.0 域升级 Step 4 follow-up 落地：D7-S2 SessionOrchestrator 单一博弈角色单一 Go 包封装（pure physical migration + import path replace）。24 .go 文件 git mv + 14 importer 文件 import path replace + 跨包 import cycle 打破（LLMInvoker/LLMInvokeRequest/ToolSchema 上提 `orchtypes/` + sessionorchestrator 用 type alias）。**0 函数签名变化** + 0 行为变化 + `hardening/` + `escape/circuit_breaker.go` + `sessionorchestrator/autoclose.go` 0 变更验证。22/22 orchestration packages go test -race PASS + go build + go vet 全绿。**4 新 P0 T** IMPLEMENTED：D7-S2-A50-T01 `orchestration/turn/` 24 .go git mv → `orchestration/sessionorchestrator/`（5 文件 turn_ 前缀解决同名）/ D7-S2-A50-T02 24 文件 `package turn` → `package sessionorchestrator` / D7-S2-A50-T03 14 importer import path + identifier 全替换 + 跨包 import cycle 打破 / D7-S2-A50-T04 `orchestration/turn/` 0 残留 + `hardening/` + `escape/circuit_breaker.go` + `sessionorchestrator/autoclose.go` git diff 0 + 22/22 PASS。Total 536, P0 350（IMPLEMENTED 527→531, P0 346→350）。D7 t-registry v4.3.0 → v4.4.0。
> 详见 `openspec/archive/2026-06-26-devrix-d7-mups-package-migration/acceptance-report.md` §3 T 层验证 + §4 22 包回归验证 + §5 LP-1/LP-2/LP-5 兼容验证（待 S6 归档生成）。

> 2026-06-26 增量（verify-promotion 包归属迁移 PLANNED 预登记）：DM-20260626-005 (devrix-d7-6s-verify-promotion) — v6.0.0 域升级 Step 5 follow-up 立项：DM-20260626-004 turn/ → sessionorchestrator/ 时为避免单 PR scope 膨胀临时留存的 `sessionorchestrator/{exit_reason.go (72 行) + verdict_to_exit_reason.go (49 行) + verdict_to_exit_reason_test.go (97 行)}` 3 文件 (218 行) 物理 promote 到 `executionflow/verify/`；让 S4 ExecutionFlow + Verify (Costly Signaler + Certifier) 角色的可验证承诺 (14 ExitReason + VerdictToExitReason 4 态映射) 在 spec/code 完全对齐；`sessionorchestrator/turn_orchestrator.go` 11 处 `ExitReason*` → `verify.ExitReason*` 跨包引用替换 + `turn_orchestrator_test.go` 2 处替换。**0 函数签名变化**（pure physical migration，安全网与 DM-20260626-004 一致）；14 ExitReason 字符串值 + 6 测试函数测试矩阵全不变。加 **4 新 P0 T** PLANNED：D7-S4-A50-T01 3 文件 git mv + git log --follow 100% rename detection / D7-S4-A50-T02 3 文件 package 改名 + 13 处 ExitReason* 全替换 + grep 0 残留 / D7-S4-A50-T03 executionflow/verify/ 包 0 sessionorchestrator 反向依赖 + 跨包 cycle 0 风险 / D7-S4-A50-T04 go build/vet/test -race 22/22 PASS + LP-1/LP-2/LP-5 集成测试 100% 兼容 + hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 变化。Total 540, PLANNED 3→7, P0 350（IMPLEMENTED 531, P0 350）。D7 t-registry v4.4.0 → v4.5.0。22 包 baseline 持平（DM-20260626-004 PR #220+#221 验证），0 函数签名变化。

> 2026-06-26 增量（Bootstrap Wire 拓扑收口 PLANNED 预登记）：DM-20260626-007 (devrix-d7-6s-bootstrap-slim) — v6.0.0 域升级 follow-up 序列收官（5/6 S7_Archived + 1/6 S1_Cancelled + 1/1 S7_Archived = #007 bootstrap-slim）：6 S × WireFunc 命名一致（新增 `WireDecisionPlanning` 16 行 S5 + `WireMUPSPipeline` + `MUPSPipelinesDeps` 75 行 S6 包装，6 个 wire 函数对齐）；3rd adapter `contextEngineAdapter` 已在 `turn_adapter.go` 独立，PR-2 抽 `turnOrchExecutor` + `gatewayEventPublisher` 2 内嵌 adapter 到 `adapters.go` (48 行)；PR-1 抽 4 util 函数 (`boolPtr` + `intPtr` + `strPtr` + `mapBackgroundStatus`) 到 `util.go` (30 行)；PR-4 抽 `loadOrchestratorConfigs` (24 行) + `resolveObsBridge` (6 行) 辅助函数分离 config 加载与类型断言；InitOrchestration 函数体 275 → **140 行**（≤ 200 目标达成）+ cmd/devrix + cmd/obs-verify + tests/testutil/d7_stack.go 调用方 0 变化 + hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 baseline stability。加 **4 新 P0 T** PLANNED：D7-S2-A51-T01 6 S × WireFunc 命名一致（`grep -E "^func Wire" internal/bootstrap/*.go` 应列 5 Wire* + 1 BuildOrchestratePath）/ D7-S2-A51-T02 InitOrchestration 主体 ≤ 200 行 + 6 S 组合入口清晰（`wc -l internal/bootstrap/wire_coordinator.go` ≤ 250；函数体 140 行 ≤ 200）/ D7-S2-A51-T03 3 内嵌 adapter + 4 util 函数已抽到独立文件（grep 0 命中 wire_coordinator.go）/ D7-S2-A51-T04 go build/vet/test -race 22/22 PASS + 0 函数签名变化（pure physical refactor, InitOrchestration 外部接口 100% 不变）+ hardening/ + escape/circuit_breaker.go + sessionorchestrator/autoclose.go git diff 0 baseline stability。Total 544, PLANNED 7（持平）, P0 354（IMPLEMENTED 535→535 持平, P0 350→354）。D7 t-registry v4.5.0 → v4.6.0。22 包 baseline 持平（DM-20260626-005 PR #222+#223 验证），0 函数签名变化。

> 2026-06-28 增量（DM-20260628-001 devrix-api-error-classification PLANNED）：D3 LLM Gateway API 错误分类与可恢复语义 — `sharederrors.APIErrorCode` 7 类闭集枚举（RateLimit/AuthenticationFailed/ServerError/MediaSize/PromptTooLong/ImageSize/Unknown）+ `NewAPIErrorCodeFromStatus` HTTP status 自动映射 + `sharederrors.IsCode` 包装链识别 + 4 adapter（minimax/deepseek/anthropic/openai）HTTP 错误构造统一走 `NewAPIError(status, msg)` 工厂 + `OrchestratorDeps.FallbackModel string` 字段预留 + `TurnState.Withheld bool` 字段 + `emitError` 路径用 `sharederrors.Code(err)` 填 `Event.Metadata["error_code"]` 受控枚举 + 主模型 2 次连续 RateLimit/ServerError 触发 `fallback_trigger_candidate` 日志（fallback_model 未 wire 显式标注）+ prompt_too_long 错误标 `withheld=true` 不 surface + feishu/cli IM 适配器基于 error_code 走差异化文案（5 类 code + 兜底 Unknown）。加 **6 新 P0 T** PLANNED：D3-S1-A01-T04 HTTP status 映射覆盖 8 类 (401/403/408/413/429/529/5xx/4xx-unknown) + D3-S1-A01-T05 IsCode 包装链识别 (WithCode→Unwrap→bare APIError) + D3-S3-A01-T17 4 adapter NewAPIError 工厂统一 + D7-S2-A50-T05 OrchestratorDeps.FallbackModel + TurnState.Withheld + emitError code 注入 + D7-S2-A50-T06 2 次连续触发 fallback 日志 + withheld + SanitizeForUser 回归 + D1-S3-A08-T01 feishu/cli 5 类 code 差异化文案（7 sub-test）。Total 565→571, PLANNED 7→13, P0 375→377（IMPLEMENTED 556 持平）。D3 t-registry v3.2.0 → v3.3.0 (P0 20→23) + D7 t-registry v4.8.0 → v4.9.0 (P0 191→193) + D1 t-registry v3.0.0 → v3.1.0。S4 实现后回填 IMPLEMENTED。Out of Scope：完整 streaming fallback 自动切换（放 P0-2）+ prompt_too_long fold 闭环（放 P0-3）+ per-tool maxResultSizeChars（放 P1-4）。

> 2026-06-28 S5 验收（DM-20260628-001 devrix-api-error-classification, PR #265 squash merged）：6 新 P0 T 全部 PLANNED→IMPLEMENTED。T04 (HTTP status 8 类映射) + T05 (IsCode 包装链) + T17 (4 adapter NewAPIError) 在 D3-S1/S3 测试单测全 PASS；T05+T06 (FallbackModel + emitError code 注入 + 2 次连续 fallback 日志) 在 D7 orchestrator 单测 5 case 全 PASS；T01 (feishu/cli 5 类 code 差异化文案 + 兜底) 在 D1 adapter 单测 7 sub-case 全 PASS。Total 571→572, PLANNED 13→3, P0 377→378（IMPLEMENTED 556→567）。D3 t-registry v3.3.0 → v3.3.1 + D7 t-registry v4.9.0 → v4.9.1 + D1 t-registry v3.1.0 → v3.1.1。Out of Scope P0-2/P0-3/P1-4 不变。

> 2026-06-29 S7 归档（DM-20260625-019 devrix-d7-mups-v4-5node-coverage-orchestration, FULL — PR #235+#236 squash merged 2026-06-26）：D7 MUPS 5-node Span 全覆盖 + mups/{execute,learn} 目录结构治理。加 **5 IMPLEMENTED P0 T**：D7-S8-A30-T01 (5 节点 Span 注册 coverage registry) + D7-S8-A31-T01 (D7_MUPS_Pipeline 根 Span 端到端串联) + D7-S9-A30-T01 (mups/execute/ channel_ 前缀清理 5 文件) + D7-S12-A30-T01 (mups/learn/ 拆 4 subpackage) + D7-S12-A30-T02 (import cycle 打破 DefaultPendingMaxRetries 上提 asset/)。0 函数签名变化 (pure physical migration) + 23 orchestration packages -race PASS 0 FAIL。Total 572→577, P0 378→383（IMPLEMENTED 567→572）。D7 t-registry v4.9.1 → v4.10.0。详见 `openspec/archive/2026-06-29-devrix-d7-mups-v4-5node-coverage-orchestration/acceptance-report.md` §2 T 层验证。

> 2026-06-29 S6 归档（DM-20260628-004 devrix-d7-multiturn-session-state, **PARTIAL** — PR #271 squash merged 2026-06-28）：D7 多轮 session 串行化与 complete 时机修正 — RC-3 panic hotfix done via PR #271。加 **2 IMPLEMENTED P0 T**：D7-S2-A16-T01 (emit recover middleware) + D7-S2-A16-T02 (exec.Emit overwrite per Run) — **避免 send-on-closed-channel panic + stale emit hook 串扰**。生产环境 smoke test sess_1782638991113_5000 二轮消息不 panic 验证 PASS。**5 DESIGN T points DEFERRED to v1.1**：D7-S2-A14-T01 WaitForTurnCompletion + D7-S2-A14-T02 TurnState in-memory + D7-S2-A15-T01 TranscriptReader + D7-S2-A15-T02 turn directive auto-injection + D7-S2-A17-T01 feishu TurnInProgressError（设计 4 层契约已就位：TurnState + TranscriptReader + WaitTurn + feishu adapter）。22/22 orchestration packages -race PASS 0 FAIL。Total 577→579, P0 383→385（IMPLEMENTED 572→574, PARTIAL 持平 0, DESIGN +5 待 v1.1）。D7 t-registry v4.10.0 → v4.11.0。详见 `openspec/archive/2026-06-29-devrix-d7-multiturn-session-state/acceptance-report.md` §2 T 层验证。**DM ID 重新分配：** 原 DM-20260628-003 → DM-20260628-004（与 D1 DSAFT Refactor DM-20260628-003 冲突）。

> 2026-06-29 S6 归档（DM-20260626-006 devrix-d7-6s-observe-merge-cancel, **S1_Cancelled**）：D7 6s observe-merge-cancel — observe/orchtypes/ → decisionplanning/ 物理合并 S1_Cancelled: observe/orchtypes/ 目录从未存在, 原 follow-up #5' scope 基于错误假设, 实际 v6.0.0 域升级后子包已全部归位 sessionorchestrator/ + decisionplanning/ + mups/{observe,plan,execute,learn}/ + hardening/。仅 demand.md 在 archive/（CANCELLED precedent: no .openspec.yaml + no acceptance-report.md + no specs/）。0 T 点（无 S2/S3 design phase），不影响 t-registry 计数。

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
