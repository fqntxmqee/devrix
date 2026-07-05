---
demand-id: DM-20260705-008
title: "d7-mups-strategy-injection — Strategy 抽象注入 WorkItemExecContext (PlanKind 路由恢复) — M3 5 节点重构总图行为增量最后一步"
source: MUPS 5 节点重构总图 M3 (M1+M2+M4+M5 已 S7_archived + cleanup-legacy 已 S7_archived)
priority: P0
status: S3_Design
l1-domain: orchestration
created: 2026-07-05
related:
  - openspec/specs/d7-orchestration/spec.md
  - openspec/specs/d7-orchestration/mups-spawn-data-objects.md
  - openspec/specs/d7-orchestration/pipeline-architecture.md
  - openspec/specs/d7-orchestration/uncertainty-spawn-contract.md
  - openspec/specs/d7-orchestration/mups-spawn-sequence-goal-decompose-rollup.md
  - internal/layers/orchestration/workmodel/spawn_policy.go
  - internal/layers/orchestration/workmodel/spawn_decision_algebra.go
  - internal/layers/orchestration/workmodel/pipeline_round.go
  - internal/layers/orchestration/mups/execute/channel.go
  - internal/layers/orchestration/plan/plan.go
  - internal/layers/orchestration/sessionorchestrator/workitem_exec_context.go
parent_demands:
  - DM-20260705-003  # M1 Observe go-struct-driven
  - DM-20260705-004  # M2 Plan go-struct-driven
  - DM-20260705-005  # M4 Verify decision-table-driven
  - DM-20260705-006  # M5 Spawn decision-algebra
  - DM-20260705-007  # cleanup-legacy
---

# d7-mups-strategy-injection — Strategy 抽象注入 WorkItemExecContext (M3)

## 1. 原始描述

> MUPS 5 节点重构总图（M1+M2+M4+M5 + cleanup）的最后一步 M3 落地：把 `PlanKind` (Commitment/Protocol/Scenario/Exploration) 路由策略从 mups/execute/channel.go 内的隐式 `ChannelRegistry` 抽提为 **Strategy 抽象**，注入到 `WorkItemExecContext`，让 spawn policy / plan proposer / verify 等下游节点能根据 PlanKind 显式选择 strategy（per-PlanKind 行为：commitment=1-step synchronous / protocol=multi-step async / scenario=read-only probe / exploration=parallel experiments）。本 change 是 5 节点重构中**唯一行为增量** —— 之前 4 节点都是 0 行为变化 refactor。M3 让 PlanKind 不再"被忽略"（之前 spawn_policy 只把 PlanKind 当数字日志字段，未真正用作行为分支），恢复 Phase 3 PR-C2 设计的 `ChannelRouter 4 PlanKind 路由` 的可观察性（ChannelRegistry 已存在，但 spawn policy 决策未与之联动）。

## 2. 问题陈述（现状诊断）

### 2.1 已有能力（不重复建设）

| 能力 | 状态 | 路径 |
|------|------|------|
| `PlanKind` 4 类 typed enum (Commitment/Protocol/Scenario/Exploration) + IsKnown/String/Parse | ✅ S7_archived (Phase 2 PR-B1) | `plan/plan.go:30-130` |
| `ChannelRegistry` 1:1 绑定 + `ChannelRouter` 4 PlanKind 路由 (CommitChannel/ProtocolChannel/ScenarioChannel/ExplorationChannel) | ✅ S7_archived (Phase 3 PR-C2) | `mups/execute/channel.go:113-180 + 206-260` |
| `CommitChannel` 1-Step 同步 + IdempotencyKey 强制 | ✅ | `mups/execute/commit.go` |
| `ProtocolChannel` 顺序多步 + reverse-order rollback | ✅ | `mups/execute/protocol.go` |
| `ScenarioChannel` 并行探测 + 多数派投票 | ✅ | `mups/execute/scenario.go` |
| `ExplorationChannel` 多 agent + 优先级排序 + PersistScope | ✅ | `mups/execute/exploration.go` |
| `WorkItemExecContext` 7 字段 (Item/Tasks/MaxItersOverride/DeliverableContract/DeliverableSchema/PriorVerifyReason/Emit) + With/From 访问器 | ✅ | `sessionorchestrator/workitem_exec_context.go:32-67` |
| `SpawnPolicyEvaluator` 3 子决策代数化 (checkBudget/checkRollupGuard/checkVerdictDirection) | ✅ S7_archived (M5) | `workmodel/spawn_decision_algebra.go` |
| 22 现有 spawn policy 测试 + 18 M5 新增 sub-decision 测试 | ✅ | `workmodel/spawn_policy_test.go` + `spawn_decision_algebra_test.go` |

### 2.2 缺口（PlanKind 路由"被忽略"）

| 关注点 | 现状 | 风险 |
|--------|------|------|
| **PlanKind 字段在 spawn policy 决策中无作用** | `round.PlanKind` (int) 仅在 `spawnRationale` 文案中日志，未用作 SpawnPolicy 决策分支 | Phase 3 PR-C2 ChannelRouter 4 通道已实现但 spawn policy 不与之联动 → PlanKind 路由"只在执行节点有，决策节点没有"，行为割裂 |
| **没有 Strategy 抽象层** | 下游节点（spawn policy / plan proposer / verify）需要"按 PlanKind 选行为"时，只能：(a) 硬编码 `switch round.PlanKind { case CommitmentPlan: ... }` 散落 5+ 处；(b) 临时 import `mups/execute.ChannelRouter` 反向依赖 | 散落 → 重复 → 二义性（新加 PlanKind 要改 5+ 处）；反向依赖 → 分层违规 |
| **WorkItemExecContext 缺 Strategy 字段** | 7 字段无 Strategy，无法传递 PlanKind-aware 行为给下游 | spawn policy 在 workmodel 包，不能直接 import mups/execute ChannelRouter；必须通过 WorkItemExecContext 注入 |
| **0 行为变化承诺的边界** | 之前 4 节点都是 0 行为变化 refactor；M3 是**行为增量**（PlanKind 路由恢复） | 5 节点重构总图最后一步，需要充分 S5 验收（新功能单测 + 行为契约 + 回归测试） |

**风险（PlanKind "被忽略"导致的 4 类问题）**：
1. **行为割裂**：PlanKind 4 类只在 Execute 节点有不同行为，spawn policy / plan proposer / verify 都"看不见"PlanKind 区别 → 用户发出"commitment plan"和"exploration plan"在 spawn decision 阶段无差别 → 错配
2. **散落 if/switch**：未来加新 PlanKind（如"delegation plan"）需要改 5+ 处 if/switch → 重复 → 二义性
3. **分层违规风险**：workmodel 包是 L2 编排核心，mups/execute 是 L1 节点；workmodel 直接 import mups/execute 违反 openspec/specs/architecture/layering.md L1→L2 单向依赖
4. **M3 行为增量空间**：5 节点重构总图设计意图是"PlanKind 路由恢复"，但当前 spawn policy 不读 PlanKind；M3 必须把 spawn policy / Strategy 联动起来

### 2.3 目标行为（Strategy 抽象注入 + 行为增量）

1. **Strategy interface 抽象**（`workmodel/strategy.go` NEW）：3-4 个 method 定义 per-PlanKind 行为（RouteChannel / SpawnOverride / ShouldDecompose / IsReadOnly）
2. **4 PlanKind Strategy 实现**（`workmodel/strategy_*.go` 4 NEW）：commitment/protocol/scenario/exploration 各 1 个 struct
3. **DefaultStrategy registry**（`workmodel/strategy_default.go` NEW）：1 个 map[PlanKind]Strategy 包级 var + LookupStrategy helper
4. **WorkItemExecContext 扩展**（`sessionorchestrator/workitem_exec_context.go` MODIFIED）：新增 `Strategy workmodel.Strategy` 字段（interface，可选）；nil → `DefaultStrategy` 兜底
5. **SpawnPolicy 集成 Strategy**（`workmodel/spawn_decision_algebra.go` MODIFIED）：`checkVerdictDirection` 内 5 case + default 调 `strategy.SpawnOverride(planKind)` 覆盖默认 SpawnPolicy（如 CommitmentPlan + VerdictFail → SpawnNone terminal；ExplorationPlan + VerdictFail → SpawnDecompose 探索）
6. **新增功能单测**（`workmodel/strategy_test.go` NEW + `spawn_decision_algebra_test.go` MODIFIED）：4 PlanKind × 5 verdict = 20 组合覆盖 + 兜底 case
7. **0 行为变化承诺（M3 边界）**：现有 22+18 测试 0 修改 PASS（M3 在新增 4 PlanKind 决策点时，**default 行为 = 旧行为**）；新增 20+ 测试覆盖 PlanKind 路由

### 2.4 行为增量边界（关键）

| PlanKind | Verdict | 旧行为 (M3 前) | 新行为 (M3 后) | 行为变化 |
|----------|---------|---------------|---------------|----------|
| CommitmentPlan | VerdictPass | SpawnNone | SpawnNone | 0 变化 (兜底) |
| CommitmentPlan | VerdictFail | SpawnDecompose | **SpawnNone** (terminal, 1-step commitment 不重试) | 行为增量 |
| CommitmentPlan | VerdictPartial | SpawnDecompose | **SpawnNone** (terminal partial acceptance) | 行为增量 |
| ProtocolPlan | VerdictFail | SpawnDecompose | SpawnDecompose (default) | 0 变化 (兜底) |
| ScenarioPlan | VerdictPass | SpawnNone | SpawnNone | 0 变化 (兜底) |
| ScenarioPlan | VerdictFail | SpawnDecompose | **SpawnNone** (read-only, no retry) | 行为增量 |
| ExplorationPlan | VerdictPass | SpawnNone | **SpawnDecompose** (parallel explore) | 行为增量 |
| ExplorationPlan | VerdictFail | SpawnDecompose | SpawnDecompose | 0 变化 (兜底) |

**关键设计**：Strategy 注入的 SpawnOverride 行为是**显式声明**（在 `strategy_*.go` 4 文件中），与 `checkVerdictDirection` 5 case 决策解耦；旧默认行为通过"Strategy 兜底 = 旧 5 case" 兼容。

### 2.5 重构总图（5 节点 + cleanup + M3 落点）

| 阶段 | 范围 | 行为变化 | 本 change 落点 |
|------|------|----------|-----------------|
| M1 | Observe go-struct 化 | 0 行为变化 | ✅ S7_archived (DM-20260705-003) |
| M2 | Plan go-struct 化（kernel 复用） | 0 行为变化 | ✅ S7_archived (DM-20260705-004) |
| M4 | Verify 决策表化 | 0 行为变化 | ✅ S7_archived (DM-20260705-005) |
| M5 | SpawnDecision 3 子决策代数化（R0-R8） | 0 行为变化 | ✅ S7_archived (DM-20260705-006) |
| cleanup | 删除 2 个 _legacy_test.go 死代码 | 0 行为变化 (test 路径) | ✅ S7_archived (DM-20260705-007) |
| **M3** | **Strategy 抽象注入 WorkItemExecContext + 4 PlanKind 行为差异化** | **行为增量 (PlanKind 路由恢复)** | **本 change** |

**为什么 M3 最后做**：(1) M1+M2+M4+M5 0 行为变化重构已稳定；(2) M3 是行为增量（PlanKind 路由恢复），最大风险放最后；(3) cleanup 死代码已删除，5 节点重构"骨架"清空，只剩 M3 行为增量；(4) M3 完成后 5 节点重构总图闭环（mups-5node-refactor-roadmap.md 同步创建）。

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `workmodel/strategy.go` Strategy interface 3-4 method 定义 | P0 |
| AC2 | 4 PlanKind Strategy 实现 (commitment/protocol/scenario/exploration) | P0 |
| AC3 | DefaultStrategy registry + LookupStrategy helper | P0 |
| AC4 | WorkItemExecContext 增 `Strategy workmodel.Strategy` 字段 + nil 兜底 DefaultStrategy | P0 |
| AC5 | spawn_decision_algebra checkVerdictDirection 调 strategy.SpawnOverride(planKind) 覆盖默认 | P0 |
| AC6 | 4 PlanKind × 5 verdict = 20 组合新增单测 + Default 兜底 = 24 测试全 PASS | P0 |
| AC7 | 现有 22+18 spawn policy 测试 0 修改 PASS（M3 边界 = 兜底 0 行为变化） | P0 |
| AC8 | 全文 `go test ./... -race -count=1` 0 新增 fail（除 pre-existing 1 lint test） | P0 |
| AC9 | `go vet ./...` 0 warning | P0 |
| AC10 | mups-5node-refactor-roadmap.md 5 节点重构总图最终落地文档创建 | P1 |
| AC11 | PR CI `unit tests` 绿 + auto-merge 合入 master | P0 |
| AC12 | S7 归档后 `demand-archive-index.md` 入口 + `t-registry.md` 版本 bump + `a-registry.md` 版本 bump + `CHANGELOG.md` 行 | P1 |

## 4. 非目标

- 修改 `PlanKind` 4 类枚举（保持不变）
- 修改 `ChannelRegistry` 1:1 绑定（保持不变；M3 不改 Phase 3 PR-C2 既有设计）
- 修改 `CommitChannel` / `ProtocolChannel` / `ScenarioChannel` / `ExplorationChannel` 4 个 channel 实现（保持不变；M3 不动 execute 节点）
- 复活 ChannelRouter 4 文件 v1 死代码（明确不做；DM-20260626-009 已 decommissioned）
- 修改 `SpawnPolicy` 6 态枚举（保持不变）
- 修改 `SpawnPolicyEvaluator` 3 子决策主流程（保持 M5 不变；M3 只在 `checkVerdictDirection` 内 5 case 末尾调 `strategy.SpawnOverride` 覆盖）
- 任何 Execute / Observe / Plan / Verify 节点改造（M3 只在 spawn_decision_algebra 加 Strategy 注入点）
- 跨域 LLM 节点（D3 LLMGateway）改造
- 修改 ChannelRouter 4 PlanKind 路由的既有设计（保持 Phase 3 PR-C2 不变）
- 修改 `WorkItemExecContext` 已有 7 字段（M3 只新增 Strategy 字段）

## 5. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | DM-20260705-003 (M1) PR #403 MERGED ✅<br/>DM-20260705-004 (M2) PR #405 MERGED ✅<br/>DM-20260705-005 (M4) PR #407 MERGED ✅<br/>DM-20260705-006 (M5) PR #409 MERGED ✅<br/>DM-20260705-007 (cleanup) PR #411 MERGED ✅ |
| 约束 | "行为增量边界"：4 PlanKind × 5 verdict = 20 组合中，**默认行为 = 旧 5 case 行为**（兜底），仅显式声明的 4 行为变化生效；现有 22+18 测试 0 修改 |
| 约束 | Strategy 抽象层在 workmodel 包；不 import mups/execute；WorkItemExecContext 注入桥接 |
| 约束 | 全文 `go test -race -count=1` 0 fail（除 pre-existing 1 lint test `TestScan_FindsAllInvariantFiles`） |
| 约束 | S6-归档时同步 5 个域规范文档（spec / t-registry / a-registry / CHANGELOG / demand-archive-index）+ 创建 mups-5node-refactor-roadmap.md |

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 行为增量破坏现有 spawn policy 测试 | High | M3 兜底 = 旧 5 case 行为；现有 22 测试 0 修改 PASS；新增 20 测试只覆盖"显式声明"的行为变化 |
| 4 PlanKind Strategy 散落 if/switch | Med | 4 文件结构化（commitment/protocol/scenario/exploration 各 1 文件），DefaultStrategy registry 统一查找 |
| WorkItemExecContext 加 Strategy 字段破坏向后兼容 | Low | 字段是 interface (可空)；nil → DefaultStrategy 兜底；已有 7 调用方 0 修改 |
| 5 节点重构总图文档未创建 | Low | mups-5node-refactor-roadmap.md 在 M3 启动时一并补建 (作为最终落地文档) |
| spawn_decision_algebra 集成 Strategy 引入额外间接层 | Low | 仅在 `checkVerdictDirection` 5 case 末尾 1 行 `if p, ok := strategy.SpawnOverride(round.PlanKind); ok { return p }` 覆盖；性能 < 100 ns/次 |
| mups/execute 反向依赖 workmodel 风险 | Low | Strategy 抽象层在 workmodel 包；WorkItemExecContext 注入桥接；mups/execute 不感知 |

