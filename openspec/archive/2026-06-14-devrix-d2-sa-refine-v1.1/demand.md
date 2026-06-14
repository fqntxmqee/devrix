---
demand-id: DM-20260614-010
title: D2 SA Refine v1.1 — D2 Thin 契约测试 + Canonical Span
priority: P0
status: S5_Accepted
dsaft_domain: context-engine
parent: DM-20260614-009
created: 2026-06-14
---

# D2 SA Refine v1.1 — 可追溯性闭合

## 1. 背景

v1.0（DM-009）完成 Registry + D7 边界 SoT。v1.1 闭合 **可执行契约**：D2 Thin import 边界、D7 ingress 边界、FlowHub 注入（移除 query→orchestration 依赖）。

## 2. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `query/` 不 import orchestration / multiagent | P0 |
| AC2 | `D2-S16-A01-T03` 回归测试在 `internal/lint/layer/` | P0 |
| AC3 | `capture/` 不 direct import `contextengine`（D7 ingress） | P0 |
| AC4 | `flow_report.go` 移除 `flow.GlobalHub` fallback；调用方注入 FlowHub | P0 |
| AC5 | span-registry Canonical S 列 | P1 |

## 3. 变更范围

### 代码

- `internal/lint/layer/d2_thin_test.go`
- `internal/lint/layer/d7_boundary_test.go`
- `internal/layers/contextengine/query/flow_report.go`
- `internal/layers/contextengine/query/flow_report_test.go`

### 规格

- `t-registry.md` T03 → IMPLEMENTED
- `span-registry.md` Canonical S 映射

## 4. 不在范围

- tasks/ / delegate_tools 物理迁移（v2.0）
- scenario 目录重构（v2.0）
