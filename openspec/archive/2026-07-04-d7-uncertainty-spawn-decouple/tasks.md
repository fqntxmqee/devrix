# Implementation Tasks: D7 不确定性驱动 Spawn

**Change ID:** `d7-uncertainty-spawn-decouple`  
**Demand ID:** DM-20260704-001

---

## Phase 1: Evidence + Spawn 决策（P0）— CC-U1 / CC-U6

| ID | 任务 | L5 | 状态 |
|----|------|-----|------|
| T-P0-1 | 新增 `EvidenceProgress` + 单测 | U-01 | ✅ |
| T-P0-2 | `SpawnPolicyEvaluator` CC-U1 分支 | U-01, U-02 | ✅ |
| T-P0-3 | `spawnRationale` CC-U6 | U-03 | ✅ |
| T-P0-4 | spawn/evidence 测试 | U-01～03 | ✅ |

## Phase 2: Rollup Synth 收敛路径（P0）— CC-U3

| ID | 任务 | L5 | 状态 |
|----|------|-----|------|
| T-P0-5 | `RollupSynthRequested` + `ApplySpawnPolicy` NeedsRollup | U-01 | ✅ |
| T-P0-6 | `item_pipeline` ExecuteToolCalls/ScopeIn/DeliverableReason | U-01 | ✅ |
| T-P0-7 | rollup Materialize 已有 wiring | U-01 | ✅（既有） |
| T-P0-8 | `spawn_apply_rollup_test` | U-01 | ✅ |

## Phase 3: Plan + Observe 信号（P1）— CC-U4 / CC-U5

| ID | 任务 | L5 | 状态 |
|----|------|-----|------|
| T-P1-1 | `applySingleModeUncertaintyGate` | U-04 | ✅ |
| T-P1-2 | `observeDeliverableSignals` | U-05 | ✅ |
| T-P1-3 | U 权重微调 | U-05 | ✅ |
| T-P1-4 | strategic single U gate 单测 | U-04 | ✅ |

---

## Phase 4: Session Complete Salvage（P1）— CC-U2

| ID | 任务 | L5 | 文件 | 状态 |
|----|------|-----|------|------|
| T-P1-5 | `ExtractSessionDeliverable` lenient salvage hook（`SalvageSessionDeliverable` before raw artifact fallback） | U-05 | `rollup_gate.go`, `deliverable_salvage.go` | ✅ |
| T-P1-6 | `buildUserFacingEscalationSummary` 优先 salvage findings（via `ExtractSessionDeliverable`） | U-05 | `session_complete.go` | ✅ |
| T-P1-7 | Deliverable parse: alias registry + structural fence extract + verify JSON-body-only planning_meta | U-05 | `deliverable_finding_aliases.go`, `deliverable_findings_parse.go`, `deliverable_contract_verify.go` | ✅ |

---

## Phase 5: 文档与注册（S5 前）

| ID | 任务 | 状态 |
|----|------|------|
| T-DOC-1 | 本 change `specs/d7-orchestration_uncertainty_spawn_delta.md` 评审 | ✅ |
| T-DOC-2 | S5 验收后 delta 写入 `openspec/specs/d7-orchestration/` | ✅ |
| T-DOC-3 | `t-registry.md` 登记 L5-D7-U-01～05 | ✅ |
| T-DOC-4 | `pipeline-architecture.md` 增 §CC-U 交叉引用 | ✅ |

---

## Phase 6: 验收（S5）

| ID | 类型 | 内容 |
|----|------|------|
| T-ACC-1 | CI | 全 Phase 1–4 单测/集成 | ✅ |
| T-ACC-2 | Manual | staging 飞书：大 scope 探索类指令 → 观察 decompose/rollup，非 inline 耗尽 | SKIP（合入后补验） |
| T-ACC-3 | Manual | 复现 sess 类路径：读文件+格式失败 → 有任务总结，非假中断 | SKIP（合入后补验） |

---

## 依赖顺序

```
T-P0-1 → T-P0-2 → T-P0-4
T-P0-2 → T-P0-5 → T-P0-6 → T-P0-8
T-P1-1, T-P1-2 可与 Phase 2 并行
T-P1-5 在 Phase 2 后
T-DOC-* 在 Phase 4 后
```

---

## 预估规模

| Phase | 行数（估） |
|-------|-----------|
| 1 | ~120 |
| 2 | ~100 |
| 3 | ~80 |
| 4 | ~60 |
| **合计** | **~360**（分 2 PR：P0 Phase1+2，P1 Phase3+4） |
