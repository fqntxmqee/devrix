# Design: D7 Layer SubContext Phase 3

**Change ID:** `devrix-d7-layer-subcontext-phase3`  
**Demand ID:** DM-20260628-002  
**Parent:** `openspec/archive/2026-06-28-devrix-d7-layer-subcontext/` (DM-20260627-003)  
**Status:** S7_Archived  
**Created:** 2026-06-28

---

## 1. 背景

Phase 1+2 已让 depth≥1 WorkItem Execute 走 D2 Materialize，但三条旁路仍绕过统一接口：

| 旁路 | 问题 |
|------|------|
| SubTurn `brief`/`fork`/`full` | `SubTurnRunner.applyMode` 本地拼 messages，与 Materialize 语义重复 |
| Wave `ContextResolver` | Wave worker 独立 resolver，未复用 `PartitionWave` |
| Observe 规则-only | 结构化信号丰富时缺 LLM 提案层（G3：提案 + 规则校验） |

## 2. 设计决策

### T33 — SubTurn → MaterializePolicy

| SubTurn mode | Materialize mode | Partition |
|--------------|------------------|-----------|
| brief | fresh | `PartitionAgent` |
| fork | fork | `PartitionAgent` + parent prefix |
| full | resume | `PartitionAgent` + agent sidechain |

- `SubTurnRunner.Materializer` 可选注入；nil 时 legacy `applyMode` 兜底。
- Bootstrap 通过 `newDefaultMaterializer()` 统一 wiring。

### T34 — Wave ContextResolver merge

| Wave policy | Materialize mode |
|-------------|------------------|
| fresh | fresh |
| resume | resume (+ agent sidechain) |
| upstream | upstream (+ artifact summary) |

- `NewMaterializingContextResolver` 包装 legacy resolver；Materializer nil 时回退。

### T35 — LLM ObservationProposer @ Observe

- **输入：** directive + ScopeContract + structured signal lines + prior（**不含** wi private ReAct）
- **G3 门控：** `ValidateObservationProposals` — ObsFact strength ≤ 0.85，必须有 evidence
- **Fail-safe：** LLM 失败 → rules-only Observe 继续

## 3. 文件映射

| 组件 | 路径 |
|------|------|
| SubTurn policy | `contextengine/materialize/subturn.go` |
| SubTurn wiring | `sessionorchestrator/subturn_materialize.go` |
| Wave materialize | `contextengine/materialize/wave.go`, `wavescheduler/context_materialize.go` |
| ObservationProposer | `sessionorchestrator/observation_proposer.go`, `llm_observation_proposer.go` |
| Shared Materializer | `internal/bootstrap/materialize.go` |

## 4. 域文档同步

| 文档 | 版本 |
|------|------|
| `openspec/specs/d7-orchestration/spec.md` | v4.15.0 — A65, A66, A74 |
| `openspec/specs/d2-context-engine/spec.md` | v8.2.0 — D2-S16-A22 |
| `openspec/specs/d7-orchestration/t-registry.md` | v4.11.0 |

## 5. 不在本 change 范围

WorkTree v2 其余项见 `openspec/tech-debt/worktree-v2-deferred.md`（DecomposeProposer、ParallelExplore min_coverage 等）。
