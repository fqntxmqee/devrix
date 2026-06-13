# Devrix A 层活动注册表（索引）

**Status:** Active
**Version:** 2.1.0
**Last Updated:** 2026-06-13
**Parent:** `openspec/specs/architecture/layering.md`

---

## Overview

本文档为 Devrix A 层注册表的**索引入口**。各域的 A 层活动注册表已拆分为独立文件。

---

## 域级注册表

| 域 | 路径 | Scenarios | Activities |
|----|------|-----------|------------|
| D1 Communication | `openspec/specs/d1-communication/a-registry.md` | 12 | 21 |
| D2 Context Engine | `openspec/specs/d2-context-engine/a-registry.md` | 14 | 22 |
| D3 LLM Gateway | `openspec/specs/d3-llm-gateway/a-registry.md` | 6 | 6 |
| D4 Multi-Agent | `openspec/specs/d4-multi-agent/a-registry.md` | 10 | 17 |
| D5 Observability | `openspec/specs/d5-observability/a-registry.md` | 9 | 9 |
| D6 Evolution | `openspec/specs/d6-evolution/a-registry.md` | 4 | 5 |
| D7 Orchestration | `openspec/specs/d7-orchestration/a-registry.md` | 5 | 15 |

**总计**: 63 Scenarios · 97 Activities · 3 RETIRED (D2-S1 PEV)

---

## 跨域活动

| A ID | Name | Type | Input | Output | State Change | Code Location |
|------|------|------|-------|--------|--------------|---------------|
| CROSS-A01 | ValidateLayers | A-BE | import_graph | violation_report | — | `internal/lint/layer/` |
| CROSS-A02 | CheckContracts | A-BE | contract_catalog | check_result | — | `internal/shared/contracts/registry.go` |
