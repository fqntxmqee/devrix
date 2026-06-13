# Devrix F 层功能点注册表（索引）

**Status:** Active
**Version:** 2.1.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

本文档为 Devrix F 层注册表的**索引入口**。各域的 F 层功能点注册表已拆分为独立文件。

---

## 域级注册表

| 域 | 路径 | Activities with F | Total F Points |
|----|------|-------------------|----------------|
| D1 Communication | `openspec/specs/d1-communication/f-registry.md` | 12 | 43 |
| D2 Context Engine | `openspec/specs/d2-context-engine/f-registry.md` | 18 | 38 |
| D3 LLM Gateway | `openspec/specs/d3-llm-gateway/f-registry.md` | 8 | 22 |
| D4 Multi-Agent | `openspec/specs/d4-multi-agent/f-registry.md` | 16 | 37 |
| D5 Observability | `openspec/specs/d5-observability/f-registry.md` | 18 | 39 |
| D6 Evolution | `openspec/specs/d6-evolution/f-registry.md` | 2 | 6 |
| D7 Orchestration | `openspec/specs/d7-orchestration/f-registry.md` | 15 | 38 |

**总计**: 94 Activities with F · 236 F Points

---

## 跨域功能点

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| CROSS-A01-F01 | ScanImports | F-BE | packages | import_graph | `lint/layer/scanner.go` |
| CROSS-A01-F02 | CheckViolation | F-BE | import_graph, rules | []Violation | `lint/layer/scanner.go` |
| CROSS-A02-F01 | ComputeCtxPct | F-BE | prompt_tokens, max_tokens | pct (0-100) | `contracts/ctxutil.go` |
| CROSS-A02-F02 | SelfCheck | F-BE | — | []ContractStatus | `contracts/registry.go` |

## Bridges

| F ID | Name | Type | Input | Output | Code Location |
|------|------|------|-------|--------|---------------|
| D3-S1-A01-F03 | AdaptToContextEngine | F-BE | llmgateway_request | contextengine_response | `bridges/llm/bridge.go` |
| D1-S5-A01-F06 | AdaptToPlanner | F-BE | milestone_service | planner_interface | `bridges/milestone/wire.go` |
