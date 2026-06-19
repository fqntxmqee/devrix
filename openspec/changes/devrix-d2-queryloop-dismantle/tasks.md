# Implementation Tasks: D2 QueryLoop 拆解

**Demand ID:** DM-20260618-010  
**Change ID:** devrix-d2-queryloop-dismantle

---

## Phase 1: D7 SubTurn API（~200 行）

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T1.1 | 扩展 `TurnScope` + `SubTurnRequest` | `turn/types.go` | ~40 |
| T1.2 | `RunTurn` 支持 sub/background/wave_worker scope（FlowReporter, sidechain hook） | `turn/orchestrator.go` | ~80 |
| T1.3 | `SubTurnExecutor` 接口 + bootstrap wiring | `turn/subturn.go`, `wire_coordinator.go` | ~60 |
| T1.4 | 单测：sub scope 等价 SubQuery happy path | `turn/subturn_test.go` | ~80 |

**AC:** AC9（Phase 1 子集）  
**PR:** `feat/d7-subturn-api`

---

## Phase 2: 迁移活跃调用方（~400 行）

| ID | 任务 | 文件 | 估行 |
|----|------|------|------|
| T2.1 | `enforce/subquery.go` — `Loop.Run` → `SubTurnExecutor` | `enforce/subquery.go` | ~60 |
| T2.2 | `enforce/background.go` — async SubTurn | `enforce/background.go` | ~40 |
| T2.3 | `wire_wave.go` — SubAgent Start 改 SubTurn | `bootstrap/wire_wave.go` | ~50 |
| T2.4 | `delegate.go` — BuildSubQueryRunner 改 SubTurn | `bootstrap/delegate.go` | ~30 |
| T2.5 | 迁移/更新 subquery + background + subagent 测试 | `*_test.go` | ~120 |

**AC:** AC2  
**PR:** `feat/migrate-subquery-to-subturn`

---

## Phase 3: 删除 QueryLoop（~300 行删 + ~50 改）

| ID | 任务 | 文件 |
|----|------|------|
| T3.1 | 删除 `query/loop.go` + loop_*_test.go | `contextengine/query/` |
| T3.2 | 删除 `turn/query_llm_caller.go` + test | `turn/` |
| T3.3 | `engine_builder.go` / `engine.go` — 去除 queryLoop 字段与 Process loop | `contextengine/` |
| T3.4 | 删除 `query_loop_export.go` 或改为 stub error | `contextengine/` |
| T3.5 | 删除 `routing_mode=rule_orchestrate` + `query_loop.enabled` 配置 | `coordinator/`, `config/` |
| T3.6 | 删除 `d2_query_loop_legacy_invocations_total` metric | `observability/` |
| T3.7 | 静态断言：无 `Loop.Run` / `QueryLLMCaller` 引用 | CI script 或 test |

**AC:** AC1, AC3, AC4, AC5, AC6, AC7  
**PR:** `feat/remove-d2-queryloop`

---

## Phase 4: Recovery + Spec（~200 行）

| ID | 任务 | 文件 |
|----|------|------|
| T4.1 | TD-QL-01 413 → D7 runCompress 扩展 | `turn/orchestrator.go` |
| T4.2 | TD-QL-03 fallback → GatewayInvoker | `turn/llm.go` |
| T4.3 | 合并 delta spec → canonical | `specs/d2-context-engine/`, `specs/d7-orchestration/` |
| T4.4 | 更新 `queryloop-location.md` → CLOSED | `tech-debt/` |
| T4.5 | acceptance-report + archive | `openspec/archive/` |

**AC:** AC8, AC10  
**PR:** `docs/queryloop-dismantle-archive`

---

## Quality Gate（每 Phase）

- [ ] `go test ./...`
- [ ] `go test -race ./internal/layers/contextengine/... ./internal/layers/orchestration/turn/...`
- [ ] layer-lint strict
- [ ] `TestD2_D3Ban` PASS
- [ ] path regression T09/T10 PASS

---

## 完成清单

- [ ] Phase 1–4 PR 全部合并
- [ ] demand.md AC1–AC10 PASS
- [ ] TD-QL-LOC 关闭
- [ ] Ready for `/openspec-archive devrix-d2-queryloop-dismantle`
