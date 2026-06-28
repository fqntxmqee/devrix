# Tasks: D7 Layer SubContext

**Change ID:** `devrix-d7-layer-subcontext`  
**Demand ID:** DM-20260627-003  
**Status:** S4_Development — R1 决议已冻结（2026-06-28）  
**Total Tasks:** Phase 1 = 9 组（P0）；Phase 2 = 3 组（P1）；Phase 3 = 3 组（登记，不编码）

---

## Phase 1 — Materialize 闭环 + ScopeContract + ChildDownlink（P0）

### P1-T1 D2 ContextMaterializer + Partition Store

**Files:** `internal/layers/contextengine/`（新 `materialize/` 或 `prepare/workitem.go`）、`contracts/`  
**L4:** D2-S16-A20, D2-S16-A21  
**L5:** D2-S16-A20-T01..T05  
**Effort:** 2 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T01 | 定义 `ContextPartition`、`MaterializePolicy`、`MaterializeRequest`、`ContextMaterializer` 接口 | D2-S16-A20-T01 | [x] |
| T02 | Partition store：`wi:<sid>:<wi_id>.jsonl` append-only；cohort meta sidecar | D2-S16-A21-T01 | [x] |
| T03 | `Materialize`：BasePrompt + InjectSignals + LoadPrivateChain + Compress(token_budget) | D2-S16-A20-T02 | [x] |
| T04 | `Append(partition, msgs)` — Execute 后写 WorkItemPrivate | D2-S16-A20-T03 | [x] |
| T05 | Jaeger span `D2_Context_Materialize`：`wi_id`, `policy`, `message_count`, `token_est` | D2-S16-A20-T04 | [x] |

**AC:** `go test ./internal/layers/contextengine/... -run Materialize -count=1` PASS

---

### P1-T2 WorkItemExecutor → Materialize 接线

**Files:** `sessionorchestrator/workitem_executor.go`, `sessionorchestrator/item_pipeline.go`, `bootstrap/wire_d7.go`  
**L4:** D7-S16-A70, D7-S16-A71  
**L5:** D7-S16-A70-T01..T04  
**Effort:** 1.5 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T06 | `ResolvePartitionForWorkItem(item)` → Session / Cohort / WorkItem | D7-S16-A71-T01 | [x] |
| T07 | `prepareContext` 替换：depth≥1 默认 Materialize；L0 Goal legacy Prepare | D7-S16-A70-T01 | [x] |
| T08 | Execute 后 `Append(wi:self, turnMsgs)` | D7-S16-A70-T02 | [x] |
| T09 | RollupSynth policy：`NeedsRollup` → `MaterializeMode=RollupSynth`（与 #262 directive 对齐） | D7-S16-A70-T03 | [x] |

**AC:** integration test：子 WI materialize message_count ≪ session 主 Turn

---

### P1-T3 ScopeContract（Goal 范围收敛）

**Files:** `workmodel/workitem.go`, `sessionorchestrator/item_plan.go`, `workmodel/spawn_policy.go`  
**L4:** D7-S16-A60  
**L5:** D7-S16-A60-T01..T04  
**Effort:** 1.5 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T10 | `ScopeContract` 类型 + Goal `WorkItem` 字段（或 LastRound 扩展，按 OQ-LC-1 决议） | D7-S16-A60-T01 | [x] |
| T11 | Goal Plan 模板：产出 `scope_contract` YAML/JSON block | D7-S16-A60-T02 | [x] |
| T12 | `open_questions` 非空 → SpawnPolicy 阻断 `SpawnDecompose` | D7-S16-A60-T03 | [x] |
| T13 | 极具体指令规则推断 ScopeIn（单文件/单函数 regex） | D7-S16-A60-T04 | [x] |

**AC:** Goal fixture：`open_questions=["?"]` 时不 spawn children

---

### P1-T3b Signal→Observation 边界（Observe 规则映射）

**Files:** `sessionorchestrator/item_observe.go`, `sessionorchestrator/item_observe_scope_test.go` (NEW)  
**L4:** D7-S16-A72  
**L5:** D7-S16-A72-T01..T04  
**Effort:** 0.5 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T13b | `mapScopeContractToObservations`：R-OBS-1 open_questions → ObsUncertainty | D7-S16-A72-T01 | [x] |
| T13c | R-OBS-2 完整 ScopeContract → ObsFact（evidence=Goal ID） | D7-S16-A72-T02 | [x] |
| T13d | 单测：Observe report 含 ObsUncertainty 时 SpawnPolicy 不 decompose | D7-S16-A72-T03 | [x] |
| T13e | 单测：Execute wi 私有链 mock **不含** Obs* 强制 taxonomy 块 | D7-S16-A72-T04 | [x] |

**AC:** `go test ./internal/layers/orchestration/sessionorchestrator/... -run ScopeContractObserve -count=1` PASS

---

### P1-T4 ChildDownlink（父→子下行契约）

**Files:** `workmodel/child_spec.go`, `sessionorchestrator/spawn_apply.go`, `workmodel/context_store.go`  
**L4:** D7-S16-A61  
**L5:** D7-S16-A61-T01..T03  
**Effort:** 1 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T14 | `ChildDownlink` struct + Decompose 时写入 sidecar / WorkItem meta | D7-S16-A61-T01 | [x] |
| T15 | Materialize Fresh/InheritCohort：inject Directive + ScopeIn/Out + ExpectedReturn | D7-S16-A61-T02 | [x] |
| T16 | Goal ScopeContract → 每个 L1 子 WI ChildDownlink.ScopeIn/Out | D7-S16-A61-T03 | [x] |

**AC:** 子 WI materialize system prompt 含 ScopeIn 路径列表

---

### P1-T5 LayerCohort Partition 注册

**Files:** `workmodel/context_store.go`, `workmodel/context_scope.go`  
**L4:** D7-S16-A62  
**L5:** D7-S16-A62-T01..T02  
**Effort:** 0.5 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T17 | `EnsureCohortScope(sessionID, parentWIID)` — 同 parent 兄弟共享 cohort 元数据 | D7-S16-A62-T01 | [x] |
| T18 | CG2′ 文档注释：cohort 共享契约 vs transcript 隔离 | D7-S16-A62-T02 | [x] |

---

### P1-T6 Feature Flag + 回归

**Files:** `internal/config/features.go`, `sessionorchestrator/*_test.go`  
**L4:** —  
**Effort:** 0.5 天

| ID | Description | Status |
|----|-------------|--------|
| T19 | bootstrap 注入 Materializer（depth≥1 默认启用，无 feature flag） | [x] |
| T20 | L0 Goal legacy Prepare 回归测试绿 | [x] |

---

### P1-T6b Execute 结构化交付模板（非 Obs  taxonomy）

**Files:** D2 Materialize hub prompt / `sessionorchestrator/workitem_executor.go` directive 模板  
**L4:** D7-S16-A73  
**L5:** D7-S16-A73-T01..T02  
**Effort:** 0.5 天

| ID | Description | L5 | Status |
|----|-------------|-----|--------|
| T20b | Materialize 模板：软引导 `<conclusion>` / `<open_questions>`；**禁止** Obs* 自报 | D7-S16-A73-T01 | [x] |
| T20c | 文档注释 + 单测：LastRound 是 Signal 载体，非 UncertaintyReport SoT | D7-S16-A73-T02 | [x] |

---

### P1-T7 集成测试 + 验收

**Files:** `sessionorchestrator/item_pipeline_materialize_test.go`  
**Effort:** 1 天

| ID | Description | AC | Status |
|----|-------------|-----|--------|
| T21 | 同层 A/B 无 BlockedBy：B payload 不含 A tool result | LC2 | [x] |
| T22 | BlockedBy B→A：B 含 structured bubble，不含 A 私有链 | LC2 | [x] |
| T23 | depth≥1 SubContext：prompt/budget/tools ≠ session main | LC1 | [x] |
| T24 | Jaeger fixture：`D2_Context_Materialize` span 存在 | LC3 | [x] |
| T25 | ScopeContract open_questions → ObsUncertainty → 不 spawn | LC4/LC6 | [x] |
| T26 | wi private transcript 无 per-iter Obs* 标签 | LC6 | [x] |

---

## Phase 2 — Upstream + PeerStatus + CLI（P1）

### P2-T1 UpstreamSignal（BlockedBy 物化）

**Files:** `sessionorchestrator/workitem_executor.go`, `workmodel/context_decide.go`  
**L4:** D7-S16-A63  
**Effort:** 1 天

| ID | Description | Status |
|----|-------------|--------|
| T27 | `MaterializeMode=Upstream`：仅 inject structured + ArtifactSummary | [x] |
| T28 | 单测：upstream 不含 blocker wi 私有 jsonl 全文 | [x] |

---

### P2-T2 PeerStatusSignal（ParallelExplore）

**Files:** `sessionorchestrator/item_parallel_explore.go`, `workmodel/cohort_signals.go`  
**L4:** D7-S16-A64  
**Effort:** 1 天

| ID | Description | Status |
|----|-------------|--------|
| T29 | terminal 后写 `PeerStatusSignal` 至 cohort signals.jsonl | [ ] |
| T30 | Materialize optional inject（policy flag，默认 OFF） | [ ] |

**Note:** `RunParallelExplore` 实装依赖 rollup Phase 2 Wave；本组可与 Wave PR 联动。

---

### P2-T3 CLI / ResolveHint

**Files:** `internal/cli/task/context.go`  
**L4:** D7 F6 延续  
**Effort:** 0.5 天

| ID | Description | Status |
|----|-------------|--------|
| T31 | `/task context show --wi=<id>` 展示 partition + inbound signals | [x] |
| T32 | ResolveHint：`cohort domain`, `upstream inject`, `private chain len` | [x] |

---

## Phase 3 — SubTurn 统一 + Wave 合并（登记，不编码）

| ID | Description | L4 | 依赖 |
|----|-------------|-----|------|
| T33 | SubTurn brief/fork/full → MaterializePolicy 映射 | D7-S16-A65 | context-budget Phase B |
| T34 | Wave ContextResolver 合并进 ContextMaterializer | TD-WT-02 | rollup Phase 2 Wave |
| T35 | LLM ObservationProposer @ Observe（提案 + 规则校验） | D7-S8 PR-A4 | Phase 2 登记 |

---

## 依赖与顺序

```text
rollup Phase 1 (#262) ✅
  → P1-T1 Materializer
  → P1-T2 Executor 接线
  → P1-T3/T3b ScopeContract + Signal→Obs 映射
  → P1-T4 ChildDownlink（可并行）
  → P1-T7 验收
  → Phase 2 Upstream/Peer/CLI
  → Phase 3 SubTurn/Wave
```

---

## Review 阻塞项（R1 已冻结 — 见 review-r1.md）

| # | 项 | 决议 |
|---|-----|------|
| R1 | ScopeContract 持久化 | WorkItem 字段 |
| R2 | Materialize 路径 | 轻量 path |
| R3 | CG2 版本 | 0.4.0 + ADR-001 |
| R4 | Execute Obs 标签 | 禁止 |
| R5 | 软引导模板 | Materialize 默认 |
| R6 | ExpectedReturn | 非空 |
| R7 | cohort 预算 | 8KB（T20d） |
| R8 | flag 迁移 | 30 天 deadline |
