# Devrix T 层测试点注册表（索引）

**Status:** Active
**Version:** 4.6.0
**Last Updated:** 2026-06-20
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
| D7 Orchestration | `openspec/specs/d7-orchestration/t-registry.md` | 115 | 115 | 0 | 86 |

**总计**: 433 · IMPLEMENTED 428 · PLANNED 3 · PARTIAL 0 · P0 251

> 2026-06-20 增量：DM-20260620-003 (devrix-error-handling-tier1-tier2) — 8 个 P0 T 点（D7-S1-T18 + D7-S2-A02-T18 + D7-S2-A06-T24/T25/T26/T27 + D5-S23-A06-T03 + D3-S3-A01-T16）— 全 IMPLEMENTED。
> 详见 `docs/error-handling.md` §1-9 (SentinelError 类型统一 + SanitizeForUser + 子 agent stream 哨兵 + retry nil-sentinel)。

> 2026-06-20 增量：DM-20260620-001 (Phase A) + DM-20260620-001-B (Phase B) + DM-20260620-002 (Phase C) — 三个 change 共加 22 个 P0 T 点（D1-S2-A05-T05~T08 = 4 + D2-S17-A05 T01-T05 + D2-S17-A06 T01-T03 + D2-S15-A08 T01-T10 = 18）— 全 IMPLEMENTED。
> 详见 `docs/context-budget.md` §9 (Phase C nested-branch budget injection) + §1-8 (Phase A/B 入口隔离 + 多 turn budget audit)。

> 2026-06-18 增量：DM-20260618-001/002/003 三个 change 共加 15 个 P0 T 点（T22-T34 + PERMISSION-GATE-1-T01/T02）— 全 IMPLEMENTED。
> 详见 `openspec/specs/d2-context-engine/t-registry.md` §"TOOL-SURFACE-1: v2 / v3 / Lazy Loading"。

> 2026-06-18 增量：DM-20260618-007 (devrix-tools-terminal-architecture) 5 Surface × 跨切面 LTL-Lite — 加 25 个 T 点 (D2-S4-A01-T01~T06 + TOOL-SEC-2-A02-T01~T03 + D5-S23-A02-T01~T04 + D4-S11-A02-T01~T04 + D4-S13-A02-T01 + D6-S11-A02-T01~T03 + D4-S12-A03-T01 + PERMISSION-GATE-1-T01/T02/T03) — 全 IMPLEMENTED。
> 详见 `openspec/changes/devrix-tools-terminal-architecture/acceptance-report.md` §2 T 层验证。

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
