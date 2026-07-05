# Proposal: d7-mups-strategy-injection — Strategy 抽象注入 WorkItemExecContext (M3)

**Change ID:** d7-mups-strategy-injection
**Demand ID:** DM-20260705-008
**Status:** S3_Design

## 1. Background

MUPS 5 节点重构总图（M1+M2+M4+M5 + cleanup）已 S7_archived。M3 是总图最后一步，是**唯一行为增量** — 把 `PlanKind` (Commitment/Protocol/Scenario/Exploration) 路由策略从 mups/execute/channel.go 内的隐式 `ChannelRegistry` 抽提为 **Strategy 抽象**，注入到 `WorkItemExecContext`，让 spawn policy 等下游节点能根据 PlanKind 显式选择 strategy（per-PlanKind 行为：commitment=1-step synchronous / protocol=multi-step async / scenario=read-only probe / exploration=parallel experiments）。

之前 4 节点 (M1+M2+M4+M5) 都是 0 行为变化 refactor（refactor before increment）；M3 是行为增量（PlanKind 路由恢复），按 DSAFT 5 节点重构总图设计意图，让 PlanKind 不再"被忽略"（之前 spawn_policy 只把 PlanKind 当数字日志字段，未真正用作行为分支）。

## 2. Problem Statement

**核心问题**：PlanKind 4 类路由在 Phase 3 PR-C2 ChannelRouter 已实现（mups/execute/channel.go），但 spawn policy 决策未与之联动：
- `round.PlanKind` (int) 仅在 `spawnRationale` 文案中日志，未用作 SpawnPolicy 决策分支
- 下游节点（spawn policy / plan proposer / verify）需要"按 PlanKind 选行为"时，只能硬编码 if/switch 散落 5+ 处
- workmodel 包（spawn policy 所在）不能直接 import mups/execute ChannelRouter（违反分层 L1→L2 单向依赖）

**次要问题**：
- 5 节点重构总图设计意图未完全落地（M1-M5 kernel 完成但 PlanKind 路由"只到 Execute 节点，不到决策节点"）
- mups-5node-refactor-roadmap.md 总图文档缺失（demand.md §2.5 提及但未创建）

**不解决**：
- 不修改 PlanKind 4 类枚举
- 不修改 ChannelRegistry 1:1 绑定（Phase 3 PR-C2 既有设计）
- 不修改 4 个 channel 实现（CommitChannel / ProtocolChannel / ScenarioChannel / ExplorationChannel）
- 不复活 ChannelRouter 4 文件 v1 死代码（明确不做；DM-20260626-009 已 decommissioned）
- 不修改 SpawnPolicy 6 态枚举
- 不修改 SpawnPolicyEvaluator 3 子决策主流程（M3 仅在 checkVerdictDirection 5 case 末尾 1 行调 strategy.SpawnOverride 覆盖）

## 3. Proposed Solution

**核心方案**：3 层抽象 + 1 个桥接：

**Layer 1: Strategy interface (workmodel 包)**
- `Strategy` interface: 3-4 method (RouteChannel / SpawnOverride / ShouldDecompose / IsReadOnly)
- 不依赖 mups/execute，避免分层违规

**Layer 2: 4 PlanKind Strategy 实现 (workmodel 包)**
- `commitmentStrategy`: 1-step synchronous, terminal fail (no decompose)
- `protocolStrategy`: multi-step async, decompose on failure
- `scenarioStrategy`: read-only probe, no retry
- `explorationStrategy`: parallel experiments, decompose heavily

**Layer 3: DefaultStrategy registry (workmodel 包)**
- `defaultStrategies`: `map[plan.PlanKind]Strategy` 包级 var
- `LookupStrategy(planKind) Strategy`: 兜底 `protocolStrategy` (默认行为)
- `RegisterStrategy(planKind, Strategy)`: 注册扩展点

**桥接: WorkItemExecContext 字段**
- 新增 `Strategy workmodel.Strategy` 字段（interface，可空）
- nil → `DefaultStrategy` 兜底
- 通过 `WithWorkItemExecContext` 注入

**集成: spawn_decision_algebra checkVerdictDirection 末尾**
- 5 case 末尾 1 行 `if p, ok := strategy.SpawnOverride(round.PlanKind); ok { return p }` 覆盖默认
- Default 行为 = 旧 5 case（兜底）；Strategy 显式声明 4 行为变化

**步骤**：
1. S1 需求：demand.md (DM-20260705-008) ✅
2. S2 提案：proposal.md + .openspec.yaml (status: s3_design) ✅
3. S3 设计：design.md (六段式 detailed)
4. S3-Gate：自检 design 完整性
5. S4 实现：6 NEW 文件 + 2 MODIFIED 文件
6. S4-Gate：自检代码
7. S5 验收：跑测试 + acceptance-report.md (verdict: ACCEPTED)
8. S6-交付：开 PR + auto-merge 合入 master
9. S6-归档：move to archive/ + 同步 5 个域规范文档 + 创建 mups-5node-refactor-roadmap.md 总图文档

## 4. Success Metrics

| Metric | Before (M3 前) | After (M3 后) | 备注 |
|--------|----------------|---------------|------|
| Strategy 抽象层 | 无 (散落 if/switch) | 1 interface + 4 实现 + 1 registry | 0 散落 |
| `WorkItemExecContext.Strategy` 字段 | 无 | interface, nil 兜底 DefaultStrategy | 0 破坏 |
| spawn policy 读 PlanKind 决策 | 0% (仅日志) | 100% (Strategy 兜底 0 行为变化 + 4 显式行为变化) | 4 行为变化 |
| 行为增量边界 | n/a | Default 兜底 = 旧 5 case 行为 | 22+18 测试 0 修改 |
| 新增单测 (4×5+default) | 0 | 24 (20 显式 + 4 兜底) | 全 PASS |
| mups-5node-refactor-roadmap.md 总图 | 无 | 5 节点总图最终落地 | P1 |
| `go test -race -count=1 ./...` 失败数 | 1 (pre-existing) | 1 (pre-existing) | 0 新增 |
| `go vet ./...` 警告数 | 0 | 0 | 0 新增 |

## 5. Implementation Plan

**S4 实施步骤**（8 步）：

1. **NEW** `workmodel/strategy.go` (~40 行): `Strategy` interface 定义
2. **NEW** `workmodel/strategy_commitment.go` (~30 行): `commitmentStrategy` struct + 3 method
3. **NEW** `workmodel/strategy_protocol.go` (~30 行): `protocolStrategy` struct + 3 method
4. **NEW** `workmodel/strategy_scenario.go` (~30 行): `scenarioStrategy` struct + 3 method
5. **NEW** `workmodel/strategy_exploration.go` (~30 行): `explorationStrategy` struct + 3 method
6. **NEW** `workmodel/strategy_default.go` (~40 行): `defaultStrategies` map + `LookupStrategy` + `RegisterStrategy`
7. **MODIFIED** `sessionorchestrator/workitem_exec_context.go` (~10 行): 新增 `Strategy` 字段 + `WithWorkItemExecContext` 注入
8. **MODIFIED** `workmodel/spawn_decision_algebra.go` (~10 行): `checkVerdictDirection` 末尾 1 行 `strategy.SpawnOverride` 覆盖

**S4 测试新增**（3 步）：

1. **NEW** `workmodel/strategy_test.go` (~120 行): 4 PlanKind × 5 verdict = 20 组合 + 4 兜底 = 24 测试
2. **NEW** `workmodel/strategy_default_test.go` (~50 行): DefaultStrategy registry + LookupStrategy 兜底测试
3. **MODIFIED** `workmodel/spawn_decision_algebra_test.go` (~30 行): 增加 4 PlanKind × 5 verdict 集成测试

**S4 文档新增**（1 步）：

1. **NEW** `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md` (~200 行): 5 节点重构总图最终落地文档（M1-M5 + cleanup 完整 timeline + 设计意图 + 5 节点闭环）

**S4 验证步骤**（5 步）：

1. `go test ./internal/layers/orchestration/workmodel/ -race -count=1` → 22+18+24 = 64/64 PASS
2. `go test ./internal/layers/orchestration/sessionorchestrator/ -race -count=1` → 30/30 PASS (WorkItemExecContext 注入 0 破坏)
3. `go vet ./...` → 0 warning
4. `go test ./... -race -count=1` → 1 fail (pre-existing) 不变
5. `grep "switch.*PlanKind" --include="*.go" -r internal/layers/orchestration/workmodel/` → 0 hits (Strategy 替代散落 switch)

## 6. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| 行为增量破坏现有 spawn policy 测试 | High | M3 兜底 = 旧 5 case 行为；现有 22 测试 0 修改 PASS；新增 24 测试只覆盖"显式声明"的行为变化 |
| 4 PlanKind Strategy 散落 if/switch | Med | 4 文件结构化（commitment/protocol/scenario/exploration 各 1 文件），DefaultStrategy registry 统一查找 |
| WorkItemExecContext 加 Strategy 字段破坏向后兼容 | Low | 字段是 interface (可空)；nil → DefaultStrategy 兜底；已有 7 调用方 0 修改 |
| 5 节点重构总图文档未创建 | Low | mups-5node-refactor-roadmap.md 在 M3 启动时一并补建 (作为最终落地文档) |
| spawn_decision_algebra 集成 Strategy 引入额外间接层 | Low | 仅在 `checkVerdictDirection` 5 case 末尾 1 行 `if p, ok := strategy.SpawnOverride(round.PlanKind); ok { return p }` 覆盖；性能 < 100 ns/次 |
| mups/execute 反向依赖 workmodel 风险 | Low | Strategy 抽象层在 workmodel 包；WorkItemExecContext 注入桥接；mups/execute 不感知 |

## 7. Out of Scope

- 修改 PlanKind 4 类枚举（保持不变）
- 修改 ChannelRegistry 1:1 绑定（保持不变；M3 不改 Phase 3 PR-C2 既有设计）
- 修改 CommitChannel / ProtocolChannel / ScenarioChannel / ExplorationChannel 4 个 channel 实现（保持不变）
- 复活 ChannelRouter 4 文件 v1 死代码（明确不做；DM-20260626-009 已 decommissioned）
- 修改 SpawnPolicy 6 态枚举
- 修改 SpawnPolicyEvaluator 3 子决策主流程（保持 M5 不变；M3 只在 `checkVerdictDirection` 内 5 case 末尾调 `strategy.SpawnOverride` 覆盖）
- 任何 Execute / Observe / Plan / Verify 节点改造（M3 只在 spawn_decision_algebra 加 Strategy 注入点）
- 跨域 LLM 节点（D3 LLMGateway）改造
- 修改 ChannelRouter 4 PlanKind 路由的既有设计（保持 Phase 3 PR-C2 不变）
- 修改 WorkItemExecContext 已有 7 字段（M3 只新增 Strategy 字段）

## 8. 关联

- **Parent Demands**: DM-20260705-003 (M1) / DM-20260705-004 (M2) / DM-20260705-005 (M4) / DM-20260705-006 (M5) / DM-20260705-007 (cleanup)
- **Predecessor**: 5 节点重构总图（M1-M5 + cleanup 全部 S7_archived）
- **Successor**: 5 节点重构总图闭环 (mups-5node-refactor-roadmap.md 文档创建)
