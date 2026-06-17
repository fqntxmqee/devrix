# Design: D2 QueryLoop Legacy Path Decommission (TD-QL-LOC)

**Change ID:** devrix-queryloop-legacy-decommission
**Demand ID:** DM-20260617-001
**Status:** S3_Design
**Version:** v1.0
**Domain:** D2 (Context Engine, 核心域) + D7 (Orchestration, 核心域) + D5 (Observability, 公共域)
**Related Tech Debt:** `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC)

---

## 1. Root Cause Analysis

### 1.1 历史成因

`internal/layers/contextengine/query/loop.go` 的 `Loop.Run()` 持有"while 有 tool_use 就再来一次"的循环主逻辑，是 PEV 时代（D2-S1 Plan-Execute-Verify，DM-20260610-012 已退役）的历史产物。

DM-020（D7 Turn 编排上移）已把编排回调、per-turn 上下文注入、Hub-Spoke drain 上移 D7，但 **Loop 主循环本身没动** — 形成"半重构"遗留。

### 1.2 现状矛盾（自证）

`internal/shared/contracts/llm_facade.go:11` 与 `internal/layers/orchestration/turn/query_llm_caller.go:21` 两处注释自证：

```
// DSAFT: D7-S2-A07 (InvokeLLM) → D2 query loop 拆面出口
// DSAFT: D7-S2-A07 (InvokeLLM) — D2→D3 拆面 adapter.
```

**"拆面 adapter / 拆面出口" 这两个词本身就承认现状是绕道**。

### 1.3 现状正确的部分（不重构）

| 路径 | 现状 | 证据 |
|------|------|------|
| `loopFirst=true` 主路径 | D7 RunTurnLoop 直跑 `prepare→llm→tools→persist` | `orchestrator.go:49` |
| D7→D3 直调 | `GatewayInvoker.Stream` 直连 `llmgateway` | `llm.go:50` |
| `IsLoopFirst()` 配置开关 | 默认 `true` | `routing.go:24` |

**用户提的"D7 调用 D2 获取上下文组装结果 + D7 直调 D3"在 `loopFirst=true` 下已经是现状**。本 change 不重构主路径。

### 1.4 真正的债务

| 现象 | 根因 |
|------|------|
| D7 看不到 turn 内 tool_use | Loop 在 D2，循环内状态对 D7 不可见 |
| `loopFirst=false` 路径仍跑循环 | legacy executor 未标 Deprecated，仍可调用 |
| 配置项 `query_loop.enabled=false` 无警告 | 文档/CLI 未说明"仅作临时回滚" |
| 缺监控指标 | legacy 调用频次不可观测，演进时机无依据 |

## 2. Solution Design

### 2.1 设计原则

1. **不动主路径代码** — loopFirst=true 主路径已就绪，重构无收益
2. **不删除任何代码/配置/测试** — 保留回滚能力
3. **可观测性优先** — 加 metric 比删代码更重要
4. **演进时机量化** — legacy metric 阈值决定 Z1/Z2 触发

### 2.2 解决方案四件套（W1-W4）

#### W1: D2.QueryLoop.Run 标 Deprecated

**修改** `internal/layers/contextengine/query/loop.go`：
- 函数文档注释加 `// Deprecated:` + 指向 D7 主路径
- 进入函数时递增 legacy metric
- 首次进入时 slog.Warn（sync.Once，每个进程一次）

**关键代码片段**：

```go
// Run executes the loop until no tool calls, max turns, cancel, or hook stop.
//
// Deprecated: This is the LEGACY path (loopFirst=false). The canonical
// path is D7 RunTurnLoop (internal/layers/orchestration/turn/orchestrator.go)
// which calls D3 StreamChat directly. LoopFirst=true is the default.
//
// Decommission timeline (see openspec/tech-debt/queryloop-location.md):
//   - v1.0 (this change): log warning + emit metric
//   - Z1: thin wrapper calling D7 (when legacy metric < 1/day for 4 weeks)
//   - Z2: deleted (when legacy metric = 0 for 12 weeks)
//
// This function is preserved for emergency rollback only. Production
// deployments MUST keep loopFirst=true.
func (l *Loop) Run(ctx context.Context, sc *types.SessionContext, params Params, emit EmitFunc) (*Result, error) {
    legacyInvocationCounter.Inc()
    warnLegacyOnce.Do(func() {
        slog.Warn("D2.QueryLoop.Run is deprecated; use D7 RunTurnLoop (loopFirst=true)",
            "session_id", sc.SessionID,
            "see", "openspec/tech-debt/queryloop-location.md",
        )
    })
    // ... existing function body unchanged
}
```

**新增字段**（在 `Loop` 结构体加 sync.Once）：

```go
type Loop struct {
    // ... existing fields ...
    
    // warnLegacyOnce ensures the deprecation warning is emitted at most
    // once per process to avoid log spam.
    warnLegacyOnce sync.Once
}
```

> **注意**：`Loop` 结构体已经有 sync.Once 的潜在位置（compressor/init 状态），新增字段不破坏既有初始化。

#### W2: 注册 legacy metric 到 observability

**修改** `internal/layers/observability/instrument/metrics/`（具体文件需在该目录确认，预期为 `counters.go` 或 `legacy.go`）：

```go
// In internal/layers/observability/instrument/metrics/counters.go (or new file)
var (
    // LegacyD2QueryLoopInvocations counts invocations of the LEGACY D2 QueryLoop
    // path (only increments when loopFirst=false). Tracks the decommission
    // progress for TD-QL-LOC.
    LegacyD2QueryLoopInvocations = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "d2_query_loop_legacy_invocations_total",
        Help: "D2.QueryLoop.Run invocation count (LEGACY, loopFirst=false only). See openspec/tech-debt/queryloop-location.md.",
    })
)

func init() {
    MustRegister(LegacyD2QueryLoopInvocations)
}
```

> **复用现有 metrics 包** — 不引入新包，符合 D5 = metric 单一出口规则。

#### W3: CLI/文档警告

**修改 1**：`internal/layers/orchestration/coordinator/routing.go` — `IsLoopFirst()` 文档加警告

```go
// IsLoopFirst returns whether the canonical D7 RunTurnLoop path is enabled.
//
// IMPORTANT: Setting this to false enables the LEGACY D2.QueryLoop.Run path.
// It exists ONLY as a temporary rollback mechanism. Production deployments
// MUST keep this true (the default).
//
// See openspec/tech-debt/queryloop-location.md (TD-QL-LOC) for the
// decommission timeline.
func (c *Config) IsLoopFirst() bool { ... }
```

**修改 2**：CLI 帮助模块（`internal/cmd/devrix/cli.go` 或 `coordinator/config_help.go`）

```go
// In flag definitions:
flag.BoolVar(&loopFirst, "loop-first", true,
    "Use D7 RunTurnLoop canonical path (default true). "+
    "Setting to false enables LEGACY D2.QueryLoop.Run — "+
    "ONLY for temporary rollback. "+
    "See openspec/tech-debt/queryloop-location.md")
```

#### W4: 测试护栏 + spec.md LEGACY 标记

**测试位置**：`internal/layers/orchestration/turn/orchestrator_test.go`（新建）或 `legacy_guard_test.go`

```go
// D7-S2-A06-T09
func TestRunTurnLoop_LoopFirst_ZeroFacadeAdapterCalls(t *testing.T) {
    cfg := &Config{LoopFirst: true}
    defer observability.ResetLegacyCounter() // for test isolation
    
    for i := 0; i < 100; i++ {
        runOneSession(t, cfg)
    }
    
    assert.Equal(t, 0.0, getFacadeAdapterCallCount())
    assert.Equal(t, 0.0, testutil.ToFloat64(observability.LegacyD2QueryLoopInvocations))
}

// D7-S2-A06-T10
func TestRunTurnLoop_LoopFirst_ZeroD2QueryLoopCalls(t *testing.T) {
    cfg := &Config{LoopFirst: true}
    
    for i := 0; i < 100; i++ {
        runOneSession(t, cfg)
    }
    
    assert.Equal(t, 0.0, getD2QueryLoopCallCount())
}

// D5-S24-A02-T04
func TestLegacyMetric_Registered(t *testing.T) {
    err := http.ListenAndServe(":0", nil) // scrape /metrics
    // assert d2_query_loop_legacy_invocations_total appears in output
}

// D5-S24-A02-T05
func TestLegacyPath_LogsWarningOnce(t *testing.T) {
    cfg := &Config{LoopFirst: false}
    buf := captureSlog(t)
    
    for i := 0; i < 5; i++ {
        runOneSession(t, cfg)
    }
    
    assert.Equal(t, 1, strings.Count(buf.String(), "deprecated"))
}
```

**spec.md LEGACY 标记** — `openspec/specs/d2-context-engine/spec.md` D2-S10 章节开头加：

```markdown
### Requirement: D2 QueryLoop (LEGACY)

> **⚠️ LEGACY PATH** — This requirement documents the `loopFirst=false` fallback.
> Canonical path is D7-S2-A06 `RunTurnLoop` (`internal/layers/orchestration/turn/orchestrator.go`).
> See `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC) for decommission timeline.
> Preserved for rollback capability only. New capabilities MUST NOT depend on this path.

The system MUST provide the LEGACY D2.QueryLoop.Run fallback when loopFirst=false.
... (existing requirement text) ...
```

### 2.3 域归属与 A/F 编排

| 工作 | 主域 | DSAFT Activity | 关联 F（功能点） |
|------|------|---------------|----------------|
| W1 D2.QueryLoop Deprecated | D2 | D2-S10-A01-RunLoop | F01 MetricIncrement, F02 WarnOnce |
| W2 legacy metric | D5 | D5-S24-A03-EmitLegacyMetric | F01 RegisterCounter, F02 AutoScrape |
| W3 CLI 警告 | D7 | D7-S2-A06-RunTurnLoop | F01 ConfigValidate, F02 HelpText |
| W4 测试 + spec 标记 | D7 + D5 + D2 | (跨域) | F01 PathGuard, F02 SlogCapture, F03 LegacyMarker |

## 3. Key Interfaces / Types

### 3.1 新增 metric 类型

```go
// internal/layers/observability/instrument/metrics/counters.go
LegacyD2QueryLoopInvocations prometheus.Counter
```

### 3.2 `Loop` 结构体新增字段

```go
type Loop struct {
    // ... existing fields ...
    warnLegacyOnce sync.Once
}
```

> **不可变性原则**：这是 Loop 实例状态变更（非值对象），符合 coding.md §9 实体 method 加锁变更状态规则。

### 3.3 公开 API 不变

- `Loop.Run()` 签名不变
- `IsLoopFirst()` 签名不变
- 配置项 `query_loop.enabled` 不变

## 4. Data Flow

### 4.1 loopFirst=true 主路径（不变）

```
D1.Gateway.RouteInbound
    └── D7.IOrchestrationEntry.ProcessMessage
            └── D7.RunTurnLoop (D7-S2-A06)
                    ├── D2.PrepareMessages (D2-S15)
                    ├── D7.InvokeLLM → D3.StreamChat (D7-S2-A07)
                    ├── D2.ExecuteToolRound (D2-S18)
                    └── D2.PersistTurn (D2-S17)
```

**无任何变更。**

### 4.2 loopFirst=false legacy 路径（加护栏）

```
D1.Gateway.RouteInbound
    └── D7.IOrchestrationEntry.ProcessMessage
            └── d2Executor.RunQueryLoop (bootstrap legacy)
                    └── D2.IEngine.Process
                            ├── D2.S15 PrepareExecutionContext
                            ├── D2.S16 RunQueryLoop ← W1: metric++, slog.Warn(once)
                            └── D2.S17 PersistSessionState
```

**改动仅在 RunQueryLoop 函数顶部加 metric + warn**，函数体完全不变。

## 5. File Manifest（新增/修改/删除文件清单）

### 5.1 新增

| 文件 | 状态 |
|------|------|
| `openspec/changes/devrix-queryloop-legacy-decommission/{demand.md,proposal.md,design.md,specs/d7-orchestration/spec.md,tasks.md,acceptance-report.md,.openspec.yaml}` | 已写/待写 |
| `openspec/tech-debt/queryloop-location.md` | ✅ 已写 |
| `internal/layers/orchestration/turn/orchestrator_test.go` 或 `legacy_guard_test.go` | 待写（T09/T10/T04/T05） |

### 5.2 修改

| 文件 | 改动 |
|------|------|
| `internal/layers/contextengine/query/loop.go` | `Loop.Run()` 注释 + `sync.Once` 字段 + 顶部 metric 递增 + slog.Warn |
| `internal/layers/observability/instrument/metrics/counters.go` | 注册 `LegacyD2QueryLoopInvocations` |
| `internal/layers/orchestration/coordinator/routing.go` | `IsLoopFirst()` 文档警告 |
| `internal/cmd/devrix/cli.go` 或 `coordinator/config_help.go` | `--loop-first` flag help 文本警告 |
| `openspec/specs/d2-context-engine/spec.md` | D2-S10 章节开头加 LEGACY 标记 |
| `openspec/specs/d7-orchestration/t-registry.md` | 新增 T09/T10 条目 |
| `openspec/specs/d5-observability/t-registry.md` | 新增 T04/T05 条目 |

### 5.3 不变更（明确锁定）

- `internal/layers/contextengine/query/loop.go` 的 `Loop.Run()` 函数体（仅顶部加 2 行 + sync.Once 字段）
- `internal/shared/contracts/llm_facade.go` 全文
- `internal/layers/orchestration/turn/query_llm_caller.go` 全文
- `internal/layers/orchestration/turn/orchestrator.go` 全文（主路径）
- `internal/layers/orchestration/turn/llm.go` 全文（主路径）
- `internal/bootstrap/wire_coordinator.go` 全文
- `devrix.yaml` 的 `query_loop.enabled` 配置项定义
- 现有 D2-S10 T 层测试代码（保留 IMPLEMENTED 状态，仅在 spec.md 加 LEGACY 文档标记）

## 6. Regression Risk Assessment

### 6.1 风险表

| 风险 | 影响 | 缓解 |
|------|------|------|
| **W1 顶部加 metric/warn 改动 Loop 结构体** | 既有 Loop 初始化代码可能 panic（sync.Once 零值可用，prometheus.Counter 是全局变量） | sync.Once 零值即可用；prometheus.Counter 全局变量 MustRegister；T09/T10 在 CI 跑全量回归 |
| **W2 metric 命名冲突** | /metrics 端点报重复注册 panic | MustRegister 已有的 Name uniqueness 检查；T04 测试断言注册成功 |
| **W3 CLI 帮助文本改动** | 用户意外设 loopFirst=false 但看到警告反而误用 | 警告明确"ONLY for temporary rollback"，与现状一致 |
| **W4 spec.md LEGACY 标记** | 现有 Scenario 被解读为"待删除" | 标记是文档说明，不影响 IMPLEMENTED 状态；T 层测试代码不动 |
| **W4 测试文件 mock 不到完整 turn** | T09/T10 集成测试不稳定 | 复用 `tests/integration/d7_turn_loop_*` 已有基础设施（参考 `path_regression_integration_test.go`） |
| **D2/D7 边界规则回归** | 后续修改可能无意中调用 D2→D3 | T09 专门护栏"loopFirst=true 时拆面 adapter 零调用" |

### 6.2 不变更承诺（明确）

- `Loop.Run()` 函数体逻辑 0 改动
- 主路径代码（orchestrator.go / llm.go / bootstrap.go）0 改动
- 公开 API 签名 0 改动
- 现有测试 0 删除

## 7. Rollback Plan

### 7.1 回滚触发条件

- CI 失败（unit/integration/layer-lint）
- S4-Gate 自检发现 Critical issue
- 生产环境 metric 异常

### 7.2 回滚步骤

1. `git revert <merge-commit>` — 一次 commit 撤销所有 W1-W4 改动
2. `git push origin feat/devrix-queryloop-legacy-decommission` 或 master 直接 force-with-lease（仅限热修）
3. 验证 devrix 启动正常，loopFirst=true 主路径不受影响
4. 在 PR 描述加回滚原因 + 后续 follow-up

### 7.3 回滚可行性

- 所有改动是**纯增量**（加注释/字段/metric/CLI 文本/测试/spec 标记），无现有逻辑删除
- 回滚后 `Loop.Run()` 函数体完全不变
- 回滚后 metric 消失但不影响任何现有指标
- 回滚后 spec.md LEGACY 标记消失

**结论：低风险回滚，单 git revert 即可。**

---

## 附录: S3-Gate 自检清单

按 `openspec/specs/project/review-design.md` §2 逐项：

| 检查项 | 状态 | 备注 |
|--------|------|------|
| 层归属正确 | ✅ | D2/D5/D7 各自本职，metric 走 D5 单一出口 |
| 接口方向正确 | ✅ | D2 不调 D3（保留现状），D7 编排 D2 + D3 |
| 不重复造轮子 | ✅ | 复用现有 metrics 包、sync.Once 零值、testutil |
| 跨层依赖最小 | ✅ | 仅 Loop 结构体加 1 个字段 + 顶层 2 行 |
| 设计决策有记录 | ✅ | proposal.md §8 Decision × 3 |
| 需求可追溯 | ✅ | demand AC1-AC7 → proposal 工作项 W1-W4 → design 章节 |
| 验收标准覆盖 | ✅ | AC1/AC2→T09/T10；AC4→T04；AC5→T05 |
| Out of Scope 明确 | ✅ | proposal.md §7 + design.md §5.3 |
| DM ID 无冲突 | ✅ | DM-20260617-001，今日首个 |
| Gherkin 格式正确 | ✅ | proposal.md §A 全部 GIVEN/WHEN/THEN 完整 |
| Happy path + sad path | ✅ | loopFirst=true + loopFirst=false 各有 Scenario |
| 并发场景覆盖 | ⚠️ | sync.Once 本身并发安全；建议 T05 增加并发调用测试 |
| 错误路径覆盖 | ✅ | T05 验证 slog.Warn 不重复（错误路径） |
| T 层映射完整 | ✅ | 每个 Requirement 标注 T09/T10/T04/T05 |
| 回归风险已评估 | ✅ | §6 风险表 |
| 回滚方案可行 | ✅ | §7 单 git revert |
| 性能影响已评估 | ✅ | sync.Once 一次性开销，可忽略 |
