# Implementation Tasks: D7 v2.0 结构重构

**Demand ID:** DM-20260619-005  
**Change ID:** devrix-d7-v2-structure  
**Owner Decisions:** 范围 C；其余按推荐默认

---

## Phase A: 规格同步（docs-only）

| ID | 任务 | 文件 | T 锚点 |
|----|------|------|--------|
| A1 | 刷新 design.md v3.0：S1–S5 IMPLEMENTED，删 PLANNED API | `openspec/specs/d7-orchestration/design.md` | — |
| A2 | layer-delta 新增 §v2.0-Structure；§PLANNED → HISTORICAL | `layer-delta.md` | — |
| A3 | d7-boundary Task/PlanMode 🔶→✅；更新包路径 | `d2-context-engine/d7-boundary.md` | — |
| A4 | code-layout §4.2 全 ✅ + hubspoke 拆分登记 | `architecture/code-layout.md` | — |
| A5 | a-registry Code Location 列预填目标路径 | `a-registry.md` | — |
| A6 | dsaft-architecture Stub → 真实计数 | `dsaft-architecture.md` | — |
| A7 | task-planning-design 状态同步 | `task-planning-design.md` | — |

**AC:** AC-A1..AC-A5  
**PR:** `docs/d7-v2-structure-spec-sync`  
**Gate:** S3-Gate 文档一致性审查

---

## Phase B1: wavescheduler 重命名

| ID | 任务 | 文件 |
|----|------|------|
| B1.1 | `git mv wave/ wavescheduler/`；package 改名 | `orchestration/wavescheduler/` |
| B1.2 | 更新 bootstrap / delegatetools / coordinator 引用 | `bootstrap/`, `delegatetools/` |
| B1.3 | 更新 integration tests | `tests/integration/d7/` |
| B1.4 | a-registry + terminal-state-guide 路径 | specs |

**AC:** AC-B1, AC-B8  
**T:** D7-S3-T01..T10（ID 不变）  
**PR:** `feat/d7-wavescheduler-rename`

---

## Phase B2: executionflow 收敛

| ID | 任务 | 文件 |
|----|------|------|
| B2.1 | 创建 `executionflow/{hub,workplan,imsink}/` | `orchestration/executionflow/` |
| B2.2 | 迁移 flow/ workplan/ imsink/ | 同上 |
| B2.3 | 更新 GlobalHub / contracts 引用 | `flow/hub.go` → `executionflow/hub/` |
| B2.4 | 更新 span-registry 路径注释 | specs |

**AC:** AC-B2, AC-B8  
**T:** D7-S4-A01-T01 等（ID 不变）  
**PR:** `feat/d7-executionflow-consolidate`

---

## Phase B3: coordinator 拆包

| ID | 任务 | 文件 |
|----|------|------|
| B3.1 | 创建 `sessionorchestrator/`；迁 S2 文件 | orchestrator, fastpath, orchestrate_path, command_handler, interrupt, routing, types, contracts, tracing, spans |
| B3.2 | 创建 `decisionplanning/`；迁 S5 文件 | classifier*, decomposer*, executor*, shadow_classifier*, llm_decomposer* |
| B3.3 | `sessionorchestrator.Entry` 实现 `IOrchestrationEntry` | `sessionorchestrator/entry.go` |
| B3.4 | S2 通过接口调 S5（不反向依赖） | `sessionorchestrator/orchestrator.go` |
| B3.5 | coordinator/ 留 type alias shim | `coordinator/doc.go` + aliases |
| B3.6 | 更新 `wire_coordinator.go` | `bootstrap/wire_coordinator.go` |
| B3.7 | 更新 layer-lint d7_boundary_test | `internal/lint/layer/` |
| B3.8 | workmodel.go 归属裁决（CreateWorkPlan → workmodel 或 sessionorchestrator facade） | 见 design §4 |

**AC:** AC-B3, AC-B5, AC-B6, AC-B7, AC-B8  
**T:** D7-S2-A01-T01, D7-S5-A01-T01 等（ID 不变）  
**PR:** `feat/d7-coordinator-split`

---

## Phase B4: hubspoke 拆分

| ID | 任务 | 文件 |
|----|------|------|
| B4.1 | `dispatch.go` → `sessionorchestrator/dispatch.go` | S2 |
| B4.2 | `agent_bridge.go` → `executionflow/bridge.go` | S4 |
| B4.3 | 删除空 `hubspoke/` 或留 shim | `hubspoke/doc.go` |
| B4.4 | 更新 hubspoke 测试路径 | `*_test.go` |

**AC:** AC-B4, AC-B8  
**T:** D7-S2-A04-T*, D7-S4-A05-T*（ID 不变）  
**PR:** `feat/d7-hubspoke-split`

---

## Phase C: WorkTree Legacy 清债

| ID | 任务 | 文件 | TD |
|----|------|------|-----|
| C1 | WaveScheduler 只读 WorkTree；TaskNode 从 SyncWaveNodes 投影 | `wavescheduler/`, `workmodel/worktree_wave.go` | TD-WT-02 |
| C2 | 删除 TaskNode 独立持久化（若有） | `wavescheduler/` | TD-WT-02 |
| C3 | `sc.Todos` Deprecated 标注 + 写入 audit | D2 prepare 只读投影 | TD-WT-03 |
| C4 | 更新 tech-debt 登记 CLOSED | `worktree-v2-deferred.md` | — |
| C5 | 单测：Wave 调度与 WorkTree 一致性 | `wavescheduler/*_test.go`, `workmodel/*_test.go` | — |

**AC:** AC-C1..AC-C3  
**T:** D7-S1-T07, D7-S3-T01（ID 不变）  
**PR:** `feat/d7-worktree-legacy-cleanup`  
**依赖:** Phase B1 完成（wavescheduler 路径稳定）

---

## Phase D: 归档

| ID | 任务 |
|----|------|
| D1 | 合并 `specs/d7-orchestration_delta.md` → canonical spec |
| D2 | acceptance-report |
| D3 | 更新 `demand-archive-index.md` |
| D4 | archive → `openspec/archive/2026-06-19-devrix-d7-v2-structure/` |

**AC:** 全部 AC  
**PR:** `docs/d7-v2-structure-archive`

---

## Quality Gate（每 Phase）

- [x] `go test ./...`
- [x] `go test -race ./internal/layers/orchestration/...`
- [x] layer-lint strict
- [x] `tests/integration/d7/` 全绿
- [x] t-registry 66/66 IMPLEMENTED 保持
- [x] `go vet ./...` 0 错

---

## PR 顺序（强制）

```
A (docs) → S3-Gate → B1 → B2 → B3 → B4 → C → D (archive)
```

Phase A 可与 B1 并行开发，但 **A 应先 merge**（规格先行）。

---

## 完成清单

- [x] Phase A merge + S3-Gate Approved
- [x] Phase B1–B4 merge
- [x] Phase C merge
- [x] TD-WT-02/03 PARTIAL CLOSED
- [x] S7 archive
