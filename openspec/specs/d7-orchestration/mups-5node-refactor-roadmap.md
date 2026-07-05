# MUPS 5-Node Refactor Roadmap — M1-M5 + cleanup 总图

**Spec version:** v1.0
**Demand IDs:** DM-20260701-001 (M4), DM-20260701-002 (cleanup), DM-20260703-001 (CC-1.x),
DM-20260705-005 (M2), DM-20260705-006 (M5), DM-20260705-007 (cleanup legacy),
**DM-20260705-008 (M3 — this document finalizes the roadmap)**
**Status:** s5_accepted
**Date:** 2026-07-05

---

## 目的 (Purpose)

MUPS (Modular Uncertainty-aware Planning System) Decide 节点的 5 个子决策 (Observe, Plan,
Verify, Spawn, Decide) 在 2026-06 ~ 2026-07 期间通过 5 个独立 Change (M1-M5) 完成了
go-struct 化重构, 解决了原来的 tag-driven 设计的二义性和重复链路问题。本文档是 5 节点
重构总图的最终落地文档, 描述完整 timeline、设计意图、闭环总结, 以及 M3 节点的
Strategy 抽象注入 (DM-20260705-008) 如何作为行为增量最后一步收口。

## 5 节点总览 (5-Node Overview)

| 节点 | 范围 | Change ID | Status | PR |
|------|------|-----------|--------|-----|
| **M1** | Observe go-struct 化 | (early) | ✅ S7_archived | merged |
| **M2** | Plan go-struct 化 (kernel 复用) | mups-plan-kernel-reuse (DM-20260705-005) | ✅ S7_archived | merged |
| **M3** | Strategy 抽象注入 WorkItemExecContext (行为增量) | d7-mups-strategy-injection (DM-20260705-008) | ⏳ S5 accepted, S6 pending | this PR |
| **M4** | Verify 决策表化 | mups-verify-table-driven (DM-20260705-004) | ✅ S7_archived | merged |
| **M5** | SpawnDecision 3 子决策代数化 | d7-spawn-decision-algebra (DM-20260705-006) | ✅ S7_archived | merged (PR #409) |
| **cleanup** | _legacy_test.go 死代码清理 | mups-cleanup-legacy (DM-20260705-007) | ✅ S7_archived | merged (PR #411+#412) |

## 设计演进时间线 (Design Evolution Timeline)

### Phase 1 (2026-06) — tag-driven 原型
- **设计**: MUPS Decide 节点用 string tag (`"commit_channel"`, `"protocol_channel"`,
  `"scenario_channel"`, `"exploration_channel"`) 路由 4 PlanKind 行为
- **问题**: tag 散落在 if/switch case, L1 (mups/execute) 与 L2 (workmodel) 双向耦合,
  新增 PlanKind 需要修改多处代码
- **里程碑**: Phase 3 PR-C2 ChannelRouter 4 PlanKind 路由 — 当时被注释为 "design intent"

### Phase 2 (2026-07 上旬) — go-struct 化重构 (M1+M2+M4+M5)
- **M1 Observe**: 引入 `Observation` struct + `ObservationKind` enum 替代 string tag
- **M2 Plan**: 引入 `Plan` struct + `PlanKind` enum (4 named + 1 zero) + `PlanInput` 投影
- **M4 Verify**: Verify 决策表化 (`verify_table.go`) — VerdictKind × Schema 矩阵
- **M5 Spawn**: SpawnDecision 3 子决策代数化 (`spawn_decision_algebra.go`) — checkBudget +
  checkRollupGuard + checkVerdictDirection 3-step composition
- **cleanup**: 删除 `_legacy_test.go` 死代码 (M1+M2 重构后保留的 tripwire)
- **问题**: 5-case default (M5) 是 "general purpose" 行为, 但 PlanKind 4 类有差异化
  需求 (commitment 1-step terminal, scenario read-only no retry, exploration
  parallel continue). 需要在 default 之上注入 per-PlanKind 行为.

### Phase 3 (2026-07-05) — Strategy 抽象注入 (M3, DM-20260705-008) — **本文档**
- **设计**: workmodel 包新增 `Strategy` interface + 4 PlanKind Strategy 实现 +
  DefaultStrategy registry. spawn_decision_algebra `checkVerdictDirection` 末尾 1 行
  `LookupStrategy(round.PlanKind).SpawnOverride(round)` 覆盖默认 SpawnPolicy
- **行为增量**: 4 PlanKind × 5 VerdictKind = 20 组合, 仅 4 组合有行为变化, 其他 16 兜底
  0 行为变化 (5-case default)
- **CC-1.4 precedence**: commitment + Partial + incomplete deliverable → 兜底
  (deliverable continuation wins over terminal override)

## 5 节点闭环 (5-Node Closure)

```
                  ┌────────────────────────────────────────────┐
                  │  MUPS Decide Node (5-Node Refactor)       │
                  └────────────────────────────────────────────┘
                                     │
       ┌─────────────────┬───────────┼───────────┬─────────────────┐
       │                 │           │           │                 │
       ▼                 ▼           ▼           ▼                 ▼
   ┌────────┐      ┌────────┐  ┌────────┐  ┌────────┐      ┌────────┐
   │  M1    │      │  M2    │  │  M4    │  │  M5    │      │  M3    │
   │Observe │      │  Plan  │  │Verify  │  │ Spawn  │      │Strategy│
   │struct  │      │struct  │  │table   │  │3-sub   │      │inject  │
   │ +enum  │      │+kernel │  │driven  │  │algebra │      │(PlanKind│
   │        │      │reuse   │  │        │  │        │      │ routing)│
   └────────┘      └────────┘  └────────┘  └────────┘      └────────┘
       │                │           │           │                │
       │                │           │           │                │
       └────────────────┴───────────┴───────────┴────────────────┘
                                     │
                  ┌──────────────────┴──────────────────┐
                  ▼                                      ▼
           ┌──────────────┐                    ┌──────────────┐
           │  go-struct   │                    │  default     │
           │  driven      │                    │  behavior    │
           │ (4 PlanKind  │                    │ (5-case)     │
           │  Strategy)   │                    │              │
           └──────────────┘                    └──────────────┘
                  │                                      │
                  └──────────────┬───────────────────────┘
                                 ▼
                  ┌──────────────────────────────────┐
                  │  SpawnPolicy (final decision)    │
                  │  4 M3 overrides + 16 fall-through│
                  └──────────────────────────────────┘
```

### 节点职责 (Node Responsibilities)

- **M1 Observe**: 解析 LLM 响应 → `Observation` struct (Kind: Finding/Question/...
  + Severity + File + Line + Message). 消除 string tag 二义性.
- **M2 Plan**: IntentQuantizer + AnomaliesCount + Steps → `Plan` struct (Kind:
  Commitment/Protocol/Scenario/Exploration + Steps + FailureCriteria + BlastRadius).
  PlanInput 投影避免循环依赖.
- **M4 Verify**: `verify_table.go` — VerdictKind × Schema 矩阵决策表, 替代 if/switch chain.
- **M5 Spawn**: SpawnDecision 3 子决策代数 — checkBudget (R0/R0.5/R1/R2) +
  checkRollupGuard (RH-MUPS-03) + checkVerdictDirection (R3-R8).
- **M3 Strategy**: per-PlanKind behavior abstraction — 4 PlanKind Strategy + 1 default
  registry + 1-line SpawnOverride hook in checkVerdictDirection. 完成 Phase 3 PR-C2
  ChannelRouter 设计意图的可观察性.

### 节点协作 (Node Coordination)

```
Observe (M1) ──┐
               ├──> UncertaintyReport ──> Plan (M2) ──> Plan struct
                                                            │
                                                            ▼
Verify (M4) ───┴──> Verdict ─────────> SpawnDecision (M5) ──> SpawnPolicy
                                                            │
                                                            ▼
                                                  Strategy.SpawnOverride (M3)
                                                            │
                                                            ▼
                                                  Final SpawnPolicy
```

## 设计意图 (Design Intent)

### DI1 — 解耦 L1/L2 (mups/execute ↔ workmodel)
- **before**: ChannelRegistry 在 L1 (mups/execute) 隐式持有 4 PlanKind 路由逻辑,
  spawn_decision_algebra 在 L2 (workmodel) 通过 tag 字符串判断
- **after**: L2 (workmodel) 拥有 Strategy interface + 4 实现. L1 通过
  WorkItemExecContext.Strategy (sessionorchestrator 包) 桥接, 不知道 workmodel 内部
- **L1 不依赖 workmodel**: 单向依赖 L2 ← L1, 无循环

### DI2 — per-PlanKind 行为差异化 (4 行为增量)
- **CommitmentPlan**: 1-step synchronous, terminal on Fail/Partial
  (1-Step 同步 + IdempotencyKey 强制)
- **ProtocolPlan**: multi-step async, tolerate partial completion (default behavior)
- **ScenarioPlan**: read-only probe, no retry on Fail (并行探测 + 多数派投票 one-shot)
- **ExplorationPlan**: parallel experiments, continue on Pass (多 agent + 优先级排序)

### DI3 — 兜底 0 行为变化 (16/20 兜底)
- 5-case default (M5) 处理 "general purpose" 行为
- M3 Strategy 只在 4 known override 显式覆盖
- 其他 16 组合 = Strategy returns (SpawnNone, false) → fall through to default
- 0 行为变化承诺: 现有 50+ 测试 (除 2 个 M3 行为增量对齐) 0 修改 PASS

### DI4 — CC-1.4 deliverable continuation precedence
- 1 个 M3 设计 refinement: `SpawnOverride` 签名从 `(planKind, verdictKind)` 演进到
  `(round *WorkItemPipelineRound)`. 让 commitmentStrategy 能感知 DeliverableSchema/
  DeliverableStatus, 在 incomplete deliverable 时返回 `ok=false` 让兜底 5-case
  default 处理 `spawnForDeliverableContinuation`
- 4 个现有 CC-1.4 deliverable continuation 测试 0 修改 PASS

## 5 节点行为矩阵 (5-Node Behavior Matrix)

| Combination | Default (M5) | M3 Override | Final |
|-------------|--------------|-------------|-------|
| Commitment+Pass | SpawnNone | (no override) | SpawnNone |
| Commitment+Fail | SpawnNone | (no override) | SpawnNone |
| Commitment+Partial (low U, no deliv) | SpawnNone | (no override) | SpawnNone |
| Commitment+Partial (high U) | SpawnDecompose | SpawnNone | **SpawnNone (M3)** |
| Commitment+Partial (incomplete deliv) | spawnForDelivCont | (no override) | spawnForDelivCont (CC-1.4) |
| Commitment+Indeterminate | SpawnInline | (no override) | SpawnInline |
| Protocol+5 verdicts | varies | (no override) | 0 change (safe default) |
| Scenario+Pass | SpawnNone | (no override) | SpawnNone |
| Scenario+Fail | SpawnParallelExplore | SpawnNone | **SpawnNone (M3)** |
| Scenario+Partial | varies | (no override) | varies (R5 default) |
| Scenario+Indeterminate | SpawnInline | (no override) | SpawnInline |
| Exploration+Pass | SpawnNone | SpawnDecompose | **SpawnDecompose (M3)** |
| Exploration+Fail | SpawnDecompose/Inline | (no override) | same |
| Exploration+Partial | varies | (no override) | varies (R5 exploratory) |
| Exploration+Indeterminate | SpawnInline | (no override) | SpawnInline |
| (all unknown verdicts) | SpawnNone | (no override) | SpawnNone |

## 测试覆盖 (Test Coverage)

| 节点 | 测试数 | 覆盖维度 |
|------|--------|----------|
| M1 Observe | (early) | Observation struct + enum |
| M2 Plan | (DM-20260705-005) | Plan struct + PlanInput projection + 4 PlanKinds |
| M4 Verify | (DM-20260705-004) | VerdictKind × Schema 决策表 |
| M5 Spawn | 50+ (DM-20260705-006) | 3-sub algebra + 5-case default + 22+18 spawn policy |
| **M3 Strategy** | **19 NEW** (DM-20260705-008) | **4×5 cases + 4 兜底 + 5 集成** |
| cleanup | (DM-20260705-007) | _legacy_test.go 删除 |

## 演进路径 (Evolution Path)

- **M6+ (future)**: plan proposer consumes `Strategy.SpawnOverride` for budget
  planning (predict spawn cost)
- **M7+ (future)**: verify uses `Strategy.IsReadOnly` to scope verify contract
- **M8+ (future)**: channel router consumes `Strategy.RouteChannel` for
  PlanKind-aware channel selection (replaces Phase 1 ChannelRegistry)

## 总结 (Summary)

5 节点重构总图 (M1+M2+M3+M4+M5+cleanup) 在 2026-07-05 完成, 解决了 MUPS Decide
节点的 tag-driven 设计的二义性和重复链路问题, 建立了 go-struct-driven 的
per-PlanKind behavior 抽象 (M3 Strategy), 让 PlanKind 路由策略从 L1 隐式
ChannelRegistry 抽提为 L2 workmodel Strategy interface + 4 PlanKind 实现 + 1
default registry. 4 行为增量锁定, 16 兜底 0 行为变化, CC-1.4 deliverable
continuation precedence 保留. 5 节点闭环完成, 为未来 M6+ (plan proposer budget
planning) / M7+ (verify contract scoping) / M8+ (channel router) 提供清晰的
扩展点.
