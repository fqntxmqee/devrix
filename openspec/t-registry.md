# Devrix T 层测试点注册表（索引）

**Status:** Active
**Version:** 4.1.0
**Last Updated:** 2026-06-15
**Layering Spec:** `openspec/specs/project/dsaft-methodology.md`

---

## Overview

本文档为 Devrix T 层注册表的**索引入口**。各域的 T 层测试点已拆分为独立文件。

> **编号格式**: `D{X}-S{X}-A{XX}-T{XX}`（T 归属 A）或 `D{X}-S{X}-A{XX}-F{XX}-T{XX}`（T 归属 F）

---

## 域级注册表

| 域 | 路径 | Total | IMPLEMENTED | PLANNED | P0 |
|----|------|-------|-------------|---------|-----|
| D1 Communication | `openspec/specs/d1-communication/t-registry.md` | 44 | 44 | 0 | 19 |
| D2 Context Engine | `openspec/specs/d2-context-engine/t-registry.md` | 59 | 58 | 0 | 27 |
| D3 LLM Gateway | `openspec/specs/d3-llm-gateway/t-registry.md` | 26 | 25 | 1 | 11 |
| D4 Multi-Agent | `openspec/specs/d4-multi-agent/t-registry.md` | 38 | 38 | 0 | 19 |
| D5 Observability | `openspec/specs/d5-observability/t-registry.md` | 38 | 35 | 3 | 11 |
| D6 Evolution | `openspec/specs/d6-evolution/t-registry.md` | 21 | 19 | 2 | 5 |
| D7 Orchestration | `openspec/specs/d7-orchestration/t-registry.md` | 45 | 35 | 8 | 25 |

**总计**: 271 · IMPLEMENTED 254 · PLANNED 14 · PARTIAL 2 · P0 117

> D2 含 1 条 PARTIAL（`D2-S11-A01-TD03`），计入 Total，不计入 IMPLEMENTED。

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
