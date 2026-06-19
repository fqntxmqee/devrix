# Acceptance Report: D2 QueryLoop Dismantle

**Change ID:** devrix-d2-queryloop-dismantle  
**Demand ID:** DM-20260618-010  
**Status:** S5_Acceptance  
**Date:** 2026-06-18  
**Branch:** feat/d2-queryloop-dismantle

---

## 1. AC 验证结果

| AC | 描述 | 结果 |
|----|------|------|
| AC1 | `query/loop.go` 删除；D2-S16 REMOVED | ✅ |
| AC2 | Wave/SubQuery/Background 零 `Loop.Run` | ✅ |
| AC3 | `engine.Process` 委托 D7 `PreparedTurnRunner` | ✅ |
| AC4 | Engine 构造不再 require `QueryLLMCaller` | ✅ |
| AC5 | `turn.QueryLLMCaller` 删除 | ✅ |
| AC6 | `rule_orchestrate` thin-wrap D7（保留 ingress 路由，无 D2 loop） | ✅ |
| AC7 | `query_loop.enabled` 配置删除 | ✅ |
| AC8 | TD-QL-01/03 收编至 D7/D3 | ✅ |
| AC9 | `go test -short ./...` + `TestD2_D3Ban` | ✅ |
| AC10 | spec delta + archive | ✅ |

---

## 2. Quality Gate

| Gate | 结果 |
|------|------|
| `go test -short ./...` | ✅ PASS |
| `internal/lint/layer` (`TestD2_D3Ban`) | ✅ PASS |
| `TestD2_NoQueryLoopProductionReferences` | ✅ PASS |
| path regression T02/T03 | ✅ PASS |

---

## 3. 关键变更摘要

- **删除：** `contextengine/query/loop*.go`、`turn/query_llm_caller.go`、legacy metric、`QueryLLMCaller` 字段
- **新增：** D7 `SubTurnExecutor`、`PreparedTurnRunner`、`turn/recovery.go`（TD-QL-01）
- **迁移：** SubQuery/Background/Wave → D7 SubTurn；`engine.Process` → PreparedTurnAdapter
- **配置：** `query_loop.enabled` 移除；`query_loop.max_turns` / `compress_per_turn` 保留供 D7

---

## 4. 后续

- TD-QL-02/06/07 仍在 `openspec/tech-debt/queryloop-error-recovery.md`
- `rule_orchestrate` 保留为 OrchestratePath ingress 模式（非 D2 loop）
