# Proposal: D2 QueryLoop Legacy Path Decommission (TD-QL-LOC)

**Change ID:** devrix-queryloop-legacy-decommission
**Demand ID:** DM-20260617-001
**Status:** S2_Design
**Version:** v1.0
**域:** D2 / D5 / D7
**架构债:** `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC)

---

## 1. Background

`loopFirst=true`（默认）路径下 D7 RunTurnLoop 已经直跑 `prepare → llm → tools → persist` 状态机（`internal/layers/orchestration/turn/orchestrator.go:49`），且通过 `GatewayInvoker.Stream` 直调 D3（`llm.go:50`）。**用户提的"D7 直调 D3 + D2 stateless 组装" 已是现状。**

唯一遗留矛盾：`loopFirst=false` legacy 路径下 `internal/layers/contextengine/query/loop.go` 的 `Loop.Run()` 仍持有循环主逻辑。这是 PEV 时代（D2-S1）历史产物，DM-020 半重构遗留。详细债务分析见 `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC)。

本 change **仅做"退役信号"**：标 Deprecated + 加监控 metric + 加主路径测试护栏。**不删除任何代码、配置或测试**。后续 Z1/Z2 演进由独立 DM 触发（见 AC8/AC9）。

## 2. Problem Statement

| # | 问题 | 影响 |
|---|------|------|
| P1 | `loopFirst=true` 下 D2→D3 拆面 adapter 缺测试护栏，未来重构可能回归 | 边界规则形同虚设 |
| P2 | `loopFirst=false` legacy 路径下 D2.QueryLoop.Run 仍跑循环 | 违反 D7 = Leader / D2 = Follower 边界 |
| P3 | `query_loop.enabled=false` 配置无警告文档 | 用户可能误用 |
| P4 | 缺监控指标，演进时机不可见 | Z1/Z2 何时开无依据 |

## 3. Proposed Solution

### 3.1 总策略

**v1.0（本 change）：单阶段交付，4 个独立工作项 + 4 个新 T 点。**

```
v1.0: Legacy Path Decommission 信号
   ├─ W1 D2.QueryLoop 标 Deprecated + metric 递增 + slog.Warn
   ├─ W2 legacy metric 注册到 observability
   ├─ W3 loopFirst=false 配置警告（CLI help + 文档注释）
   └─ W4 主路径测试护栏（T09/T10）+ spec.md LEGACY 标记
```

### 3.2 工作项详解

#### W1 — `internal/layers/contextengine/query/loop.go` 标 Deprecated

**改动范围**（不破坏函数体）：

```go
// Run executes the loop until no tool calls, max turns, cancel, or hook stop.
//
// Deprecated: This is the LEGACY path (loopFirst=false). The canonical
// path is D7 RunTurnLoop (internal/layers/orchestration/turn/orchestrator.go)
// which calls D3 StreamChat directly. LoopFirst=true is the default.
//
// Migration timeline:
//   - v1.0 (this change): log warning + emit metric
//   - Z1: thin wrapper calling D7 (when legacy metric < 1/day for 4 weeks)
//   - Z2: deleted (when legacy metric = 0 for 12 weeks)
//
// See openspec/tech-debt/queryloop-location.md (TD-QL-LOC).
func (l *Loop) Run(ctx context.Context, sc *types.SessionContext, params Params, emit EmitFunc) (*Result, error) {
    // Emit legacy metric
    metrics.D2QueryLoopLegacyInvocations.Inc()

    // Log warning once per process
    warnOnce.Do(func() {
        slog.Warn("D2.QueryLoop.Run is deprecated, use D7 RunTurnLoop (loopFirst=true)",
            "session_id", sc.SessionID,
            "see", "openspec/tech-debt/queryloop-location.md")
    })
    // ... existing function body unchanged
}
```

**不动**：`Loop` 结构体字段、其他方法、import 列表（除 `metrics` 与 `slog` 已存在）。

#### W2 — `internal/layers/observability/instrument/metrics/` 注册 metric

```go
// In existing metrics package (e.g. metrics.go or new file counters.go)
var D2QueryLoopLegacyInvocations = prometheus.NewCounter(
    prometheus.CounterOpts{
        Name: "d2_query_loop_legacy_invocations_total",
        Help: "D2.QueryLoop.Run invocation count (only increments when loopFirst=false). See TD-QL-LOC.",
    },
)
```

通过现有 `metrics.MustRegister()` 注册到 `/metrics` 端点。**不引入新 metric 包**。

#### W3 — `IsLoopFirst()` 文档 + CLI 帮助警告

**文件 1：** `internal/layers/orchestration/coordinator/routing.go`

```go
// IsLoopFirst returns whether the canonical D7 RunTurnLoop path is enabled.
//
// IMPORTANT: Setting this to false enables the LEGACY D2.QueryLoop.Run path.
// It exists ONLY as a temporary rollback mechanism. Production deployments
// MUST keep this true (the default). See openspec/tech-debt/queryloop-location.md
// for the decommission timeline.
func (c *Config) IsLoopFirst() bool { ... }
```

**文件 2：** CLI 帮助模块（`internal/cmd/devrix/cli.go` 或类似）

```go
// In flag definitions:
flag.BoolVar(&loopFirst, "loop-first", true,
    "Use D7 RunTurnLoop canonical path (default true). "+
    "Setting to false enables LEGACY D2.QueryLoop.Run — "+
    "ONLY for temporary rollback. "+
    "See openspec/tech-debt/queryloop-location.md")
```

#### W4 — 测试护栏 + spec.md LEGACY 标记

**新增 T 层测试**（位置：`internal/layers/orchestration/turn/orchestrator_test.go` 或独立 `legacy_guard_test.go`）：

```go
// D7-S2-A06-T09
func TestRunTurnLoop_LoopFirst_ZeroFacadeAdapterCalls(t *testing.T) {
    // Given loopFirst=true (default)
    // When 100 sessions run full turn
    // Then facade adapter counter == 0
}

// D7-S2-A06-T10
func TestRunTurnLoop_LoopFirst_ZeroD2QueryLoopCalls(t *testing.T) {
    // Given loopFirst=true (default)
    // When 100 sessions run full turn
    // Then D2.QueryLoop.Run invocation counter == 0
}

// D5-S24-A02-T04
func TestLegacyMetric_Registered(t *testing.T) {
    // When devrix starts
    // Then /metrics endpoint exposes d2_query_loop_legacy_invocations_total
}

// D5-S24-A02-T05
func TestLegacyPath_LogsWarningOnce(t *testing.T) {
    // Given loopFirst=false
    // When QueryLoop.Run called 5 times
    // Then slog.Warn emitted exactly once
}
```

**`openspec/specs/d2-context-engine/spec.md` 改动**（D2-S10 章节开头加 LEGACY 标记）：

```markdown
### Requirement: D2 QueryLoop (LEGACY)

> **⚠️ LEGACY PATH** — This requirement documents the `loopFirst=false` fallback.
> Canonical path is D7-S2-A06 `RunTurnLoop` (`internal/layers/orchestration/turn/orchestrator.go`).
> See `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC) for decommission timeline.
> This requirement is preserved for rollback capability only. New capabilities
> MUST NOT depend on this path.

The system MUST provide the LEGACY D2.QueryLoop.Run fallback when loopFirst=false.
...
```

**不动**：现有 Scenario 文本、T 层测试代码（保留 IMPLEMENTED 状态）。

### 3.3 域归属

| 工作 | 主域 | 横切域 |
|------|------|--------|
| W1 `Loop.Run` 标 Deprecated | D2 | D5（metric 出口） |
| W2 metric 注册 | D5 | — |
| W3 CLI/文档警告 | D7 | — |
| W4 测试 + spec 标记 | D7 | D5（metric 测试）、D2（spec 标记） |

## 4. Success Metrics

| # | 指标 | 基线 | 目标 |
|---|------|------|------|
| M1 | `d2_query_loop_legacy_invocations_total` 在 prod 环境每 24h 增量 | 当前不可观测 | < 1 invocations/day（Z1 触发条件）|
| M2 | D7-S2-A06-T09/T10 测试覆盖率 | 0 | 100% PASS（AC1/AC2 阻断）|
| M3 | loopFirst=false CLI 警告可见率 | 0 | 100%（devrix --help 必含警告文本）|
| M4 | D2-S10 spec.md LEGACY 标记存在 | 否 | 是（AC6 阻断）|

## 5. Implementation Plan

| 阶段 | 产出物 | 状态 |
|------|--------|------|
| S1 需求 | `demand.md` | ✅ 本次完成 |
| S2 提案 | `proposal.md` + `.openspec.yaml` | ✅ 本次完成 |
| Tech Debt | `openspec/tech-debt/queryloop-location.md` | ✅ 已存在（与本 change 同源）|
| S3 设计 | `design.md` + `specs/d7-orchestration/spec.md` ADDED Section | ❌ 待 sub-change |
| S4 实现 | 代码 + 4 个 T 测试 + metric + 文档警告 | ❌ 待 sub-change |
| S4-Gate | code review | ❌ 待 sub-change |
| S5 验收 | AC1-AC7 全 PASS + AC11-AC13 全 PASS | ❌ 待 sub-change |
| S6 归档 | `openspec/archive/2026-06-17-devrix-queryloop-legacy-decommission/` | ❌ 待 sub-change |

**S3 设计阶段建议：** 单独建一个 `openspec/changes/devrix-queryloop-legacy-decommission/design.md`，并新增 `openspec/changes/devrix-queryloop-legacy-decommission/specs/d7-orchestration/spec.md`（ADDED Requirements 部分）。

## 6. Risks & Mitigations

详见 `demand.md` §6。核心摘要：

| 风险 | 缓解 |
|------|------|
| 现有用户依赖 loopFirst=false | AC11：不删任何代码 |
| Loop.Run 被外部测试直接引用 | AC11：仅注释 + metric |
| legacy metric 注册路径不一致 | W2 走现有 metrics 包 |
| D2-S10 T 层测试在 spec.md 标 LEGACY 后被 CI 当作失败 | spec.md LEGACY 是文档标记；T 层测试保持 IMPLEMENTED |
| 集成测试 mock 全链路不稳 | 复用 `tests/integration/d7_turn_loop_*` 基础设施 |
| slog.Warn 高频开销 | sync.Once 一次性输出 |

## 7. Out of Scope

完整列表见 `demand.md` §7。摘要：

- 不实现 Z1/Z2（仅锁定触发条件）
- 不删除任何 Go 源文件
- 不删除任何配置项
- 不删除任何 T 层测试
- 不重构主路径代码
- 不迁移 D2-S10 测试到 D7-S2-A06
- 不处理其他 D2/D7 边界问题
- 不覆盖 D4/D5/D6/D1/D3 域（仅 D2/D5/D7）

## 8. Decision

### Decision: 标 Deprecated + 加 metric，不删除任何代码

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. **标 Deprecated + 加 metric + 加护栏**（推荐） | 不破坏回滚；演进时机可观测；T 层护栏防回归 | 债务仍在 |
| B. 立即删除 D2.QueryLoop.Run | 一次性解决 | 破坏现有用户；无回滚路径 |
| C. 重构主路径（用户提的"D7 跑循环"） | 架构上更纯 | 主路径已是 D7（loopFirst=true），重构无收益 |

**选择:** A
**理由:** C 已被 DM-020 + DM-20260616-002 实现（loopFirst=true 是默认）；B 风险过大；A 是当前最小代价且演进可见的方案。

### Decision: metric 走现有 D5 metrics 包，不新建包

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. **复用 `internal/layers/observability/instrument/metrics/`**（推荐） | 符合 D5 现有架构；自动注册到 `/metrics` 端点 | 与现有 metric 共享命名空间 |
| B. 新建 `internal/layers/contextengine/metrics/` | 隔离 D2 域 metric | 破坏 D5 = metric 单一出口规则 |

**选择:** A
**理由:** devrix layering 规则规定 metric 出口统一在 D5（公共层）。D2 不持有 metric 定义。

### Decision: slog.Warn 一次性输出（sync.Once）

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. **sync.Once 每个进程一次**（推荐） | 无性能开销；不刷屏 | 用户可能错过警告 |
| B. 每次调用输出 | 警告可见性高 | 日志刷屏；性能开销 |
| C. 静默（只 metric） | 零开销 | 用户看不到警告 |

**选择:** A
**理由:** 与 `Loop` 结构体绑定 `sync.Once` 字段；首次调用输出 + metric 递增；后续仅 metric 递增。文档警告通过 CLI 帮助（独立路径）确保用户能看到。

---

## 附录 A: Gherkin 需求草案

### A.1 主路径护栏（→ S3 specs/d7-orchestration/spec.md ADDED）

```gherkin
Feature: D2 QueryLoop Legacy Path Decommission (TD-QL-LOC)

  Scenario: loopFirst=true (default) — D2.QueryLoop.Run 零调用
    Given devrix.yaml has loopFirst=true (default)
    When 100 个独立 session 走完完整 turn
    Then d2_query_loop_legacy_invocations_total counter == 0
    And D2→D3 拆面 adapter 调用计数 == 0

  Scenario: loopFirst=true — 拆面 adapter 文件不被运行时引用
    Given loopFirst=true 配置
    When process 启动并跑一条 message
    Then internal/shared/contracts/llm_facade.go 加载但不被调用
    And internal/layers/orchestration/turn/query_llm_caller.go 加载但不被调用

  Scenario: loopFirst=false 触发 Deprecated 警告（一次性）
    Given devrix.yaml 显式设 loopFirst=false
    When 任意一条 message 触发 QueryLoop.Run
    Then slog 输出包含 "Deprecated: D2.QueryLoop.Run is deprecated, use D7 RunTurnLoop (loopFirst=true)"
    And 警告每个进程仅输出一次（第 2 次调用不重复）

  Scenario: legacy metric 注册到 observability
    Given devrix 启动
    When 查询 /metrics 端点
    Then d2_query_loop_legacy_invocations_total 出现在指标列表
    And metric 类型为 counter，无 label

  Scenario: loopFirst=false 配置警告（CLI help）
    Given devrix --help 输出
    When grep "loop-first"
    Then 输出包含 "WARNING" + "LEGACY" + "ONLY for temporary rollback"

  Scenario: 主路径代码零改动
    Given loopFirst=true 配置
    When 检查 git diff vs 上一个 release tag
    Then internal/layers/orchestration/turn/orchestrator.go 0 行变更
    And internal/layers/orchestration/turn/llm.go 0 行变更
    And internal/bootstrap/wire_coordinator.go 0 行变更

  Scenario: D2-S10 spec 章节标 LEGACY
    Given openspec/specs/d2-context-engine/spec.md
    When 检查 D2-S10 Requirement 章节
    Then 章节开头包含 "⚠️ LEGACY PATH" 标记
    And 标记包含 TD-QL-LOC 链接
```

### A.2 Z1/Z2 触发条件（→ 后续 DM 的需求草案）

```gherkin
Feature: Z1 演进触发（thin wrapper）

  Scenario: Z1 触发条件
    Given legacy metric 连续 28 天记录
    When 每日平均 invocations < 1
    Then 允许开 Z1 sub-change
    And Z1 将 Loop.Run 改为 thin wrapper 直接调 D7.RunTurnLoop

Feature: Z2 演进触发（删除）

  Scenario: Z2 触发条件
    Given Z1 已合并并发布
    When legacy metric 连续 84 天 = 0
    Then 允许开 Z2 sub-change
    And Z2 删除 D2.QueryLoop + 拆面 adapter + query_loop.enabled 配置项
```

---

## 9. 检查清单（S2 完成确认）

- [x] `.openspec.yaml` 所有字段已填写（domains: D2/D5/D7）
- [x] `dsaft_scenarios` / `dsaft_activities` 已标注（含 D7-S2-A06/D2-S10/D5-S24）
- [x] `proposal.md` 包含方案对比（§8 Decision × 3）+ 风险评估（§6）
- [x] T 层测试点预登记（4 个新 T 点：T09/T10 + T04/T05）
- [x] Out of Scope 已明确声明（§7，7 项不动）
- [x] Gherkin 草案附录（§A）覆盖 7 个主路径护栏场景 + Z1/Z2 触发场景
- [x] 架构债引用（`openspec/tech-debt/queryloop-location.md`）
- [x] 不删除任何代码/配置/测试（AC11）
