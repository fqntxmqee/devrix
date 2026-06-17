# S2 提案：ToolSpec orthogonal flags + InterruptBehavior + BuildSurfaces sort

**Change ID:** devrix-tool-spec-enrichment
**DM ID:** DM-20260618-001
**状态:** S2_Clarified
**提案人:** AI Assistant
**日期:** 2026-06-18

---

## 1. 问题陈述

devrix 的 `contracts.ToolSpec` 当前只有 4 个字段：`Name / Description / Parameters / Risk`。其中 `Risk types.RiskLevel` 是单 enum 字符串，把"读/写、是否可逆、是否 open-world、是否可并发" 4 个正交关注点压在一个 1 维排序上。

实测导致的 3 个具体问题：

### 1.1 PerAgentFilter 无法基于"读/写"扩 worker 集合

explore agent 当前可见集只能从 `PerAgentFilter.allowlist` 的 4 个 tool（read_file / glob / grep / list_dir）里选。**如果新加一个 ReadOnly tool（譬如 `git_log_read`），必须手动加到 allowlist**。

如果 ToolSpec 有 `ReadOnly bool` 字段，PerAgentFilter 可以写：
```go
// 伪代码
func (f *PerAgentFilter) Apply(specs, ctx) []ToolSpec {
    if ctx.AgentType == "explore" {
        return specs.Where(s => s.ReadOnly)  // 自动扩
    }
    ...
}
```

### 1.2 plan_mode 无法收紧 open-world tools

PerRiskFilter 当前只看 `Risk` 阈值。**所有 risk="medium" 的 tool 都被视为"有外部副作用"**，但实际有些"会写文件"的 tool 是确定性的（write_file），而"会发网络请求"的 tool 才是真 open-world（web_fetch）。

OpenWorld bool 字段让 PerRiskFilter 能在 plan_mode 单独收紧：
```go
if ctx.Mode == "plan_mode" && spec.OpenWorld {
    return nil  // 全 drop
}
```

### 1.3 长 run tool 缺 interrupt 协议

`FreeForkSurface.Execute` 可能跑 30s+（批量 fork 5 个 child agent 等结果）。当前 D7 turn 收到用户新消息时只能等 FreeFork 跑完。

clawcode 的 `interruptBehavior: 'cancel' | 'block'`（Tool.ts:410-416）让 surface 显式声明：
- `cancel` — 收到 interrupt 信号时立即返回 ctx.Err()
- `block` — 不响应 interrupt，等自然完成

devrix 缺这个协议 = 60s timeout 内用户无法 cancel 任何 long-run tool。

### 1.4 BuildSurfaces 输出顺序抖动

当前 BuildSurfaces 按"添加顺序"返回：
```go
if opts.ToolReg != nil { out = append(out, NewBuiltinSurface(...)) }
out = append(out, NewLSPToolSurface(...))      // 总是有
if opts.Tracker != nil { out = append(out, NewTrackerSurface(...)) }
if opts.Forker != nil { out = append(out, NewFreeForkSurface(...)) }
out = append(out, NewVerifySurface())           // 总是有
```

`opts.Tracker` / `opts.Forker` nil 与否会让 surface 列表在不同 env 下数量不同，**但即使数量相同，添加顺序也由调用方决定**。devrix 已有 3 个 bootstrap 入口（main / per-agent / delegate），每个可能传不同 opts。

LLM 拿到的 tool schema 列表顺序变化 → 破坏 prompt cache → LLM API 重新计算整段 system prompt 的 hash → cache miss。

clawcode 显式做 `byName sort + uniqBy`（tools.ts:362-366）保稳定。

---

## 2. 解决方案

### 2.1 方案 A（推荐）：ToolSpec 4 bool + ToolSurface +1 method + BuildSurfaces sort

#### 2.1.1 ToolSpec 加 4 orthogonal bool 字段

```go
// internal/shared/contracts/tool_surface.go
type ToolSpec struct {
    Name           string
    Description    string
    Parameters     string  // JSON Schema
    Risk           types.RiskLevel
    // 4 orthogonal flags (DM-20260618-001 v2) — 让 filter / 调度能基于精细维度决策
    ReadOnly       bool    // 读不改文件系统（如 read_file / glob / grep）
    Destructive    bool    // 不可逆操作（如 rm / force_push / delete_branch）
    OpenWorld      bool    // 副作用超出本地（如 web_fetch / send_im_message）
    ConcurrencySafe bool   // 可并发执行（如 read_file 多个文件可并行）
}
```

**与 Risk 字段的关系**：
- Risk 是**高层综合分类**（low / medium / high），用于 IPermissionGate.Request
- 4 bool 是**正交特征标记**，用于 PerAgentFilter / PerRiskFilter / turn 调度

**Risk 保留向后兼容**：既有 11 个 P0 T 点不依赖 Risk 字段，零回归。

#### 2.1.2 7 bool 字段填充决策表

S3 design.md 会出完整表，这里给摘要：

| Surface (tool) | ReadOnly | Destructive | OpenWorld | ConcurrencySafe | Risk |
|---|---|---|---|---|---|
| `BuiltinSurface.read_file` | ✅ | ❌ | ❌ | ✅ | low |
| `BuiltinSurface.write_file` | ❌ | ✅ | ❌ | ❌ | high |
| `BuiltinSurface.edit_file` | ❌ | ✅ | ❌ | ❌ | medium |
| `BuiltinSurface.bash` | ❌ | ✅ | ❌ | ✅ (per cmd) | high |
| `BuiltinSurface.grep` | ✅ | ❌ | ❌ | ✅ | low |
| `BuiltinSurface.glob` | ✅ | ❌ | ❌ | ✅ | low |
| `LSPToolSurface.lsp` | ✅ | ❌ | ❌ | ❌ | low |
| `FreeForkSurface.free_fork` | ❌ | ❌ | ✅ (spawn agents) | ❌ | high |
| `TrackerSurface.query_diagnostics` | ✅ | ❌ | ❌ | ✅ | low |
| `VerifySurface.verify_plan_execution` | ✅ | ❌ | ❌ | ❌ | low |
| `DelegateSurface.delegate_*` | ❌ | ❌ | ✅ | ❌ | medium |
| `BackgroundTaskSurface.task_output` | ✅ | ❌ | ❌ | ✅ | low |

**填充策略**：每 surface 在 `Tools()` 返回的 `[]ToolSpec` 里直接写字段，不通过 risk 计算反推。

#### 2.1.3 ToolSurface 加 InterruptBehavior 方法

```go
// internal/shared/contracts/tool_surface.go
type InterruptMode string

const (
    InterruptCancel InterruptMode = "cancel"  // 收到 cancel 立即返回
    InterruptBlock  InterruptMode = "block"   // 不响应 cancel，等自然完成
)

// InterruptBehavior returns the interrupt mode for toolName. Default is
// InterruptBlock for backward compatibility (existing 7 surfaces).
type ToolSurface interface {
    // ... 现有 4 方法
    InterruptBehavior(name string) InterruptMode
}
```

**默认实现**（在 contracts 包）：
```go
// type defaultInterruptBehavior struct{}
// func (defaultInterruptBehavior) InterruptBehavior(string) InterruptMode { return InterruptBlock }
```

但 Go interface 没有 default method，**所以 7 surface 必须各自实现**。每个 surface 加 1 行：
```go
func (s *FreeForkSurface) InterruptBehavior(name string) contracts.InterruptMode {
    return contracts.InterruptCancel  // long-run tool
}

func (s *BuiltinSurface) InterruptBehavior(name string) contracts.InterruptMode {
    return contracts.InterruptBlock   // 默认 block
}
// ... 其余 5 surface 同样 InterruptBlock
```

#### 2.1.4 BuildSurfaces sort by name

```go
// internal/bootstrap/surfaces.go
func BuildSurfaces(opts SurfaceBuildOpts) []contracts.ToolSurface {
    var out []contracts.ToolSurface
    // ... 现有填充逻辑
    sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
    return out
}
```

**1 行代码 + 1 import**。零 breaking change。

#### 2.1.5 turn_adapter 并行 dispatch

```go
// internal/bootstrap/turn_adapter.go:ExecuteRound
func (a *contextEngineAdapter) ExecuteRound(ctx, req) (ToolRoundResult, error) {
    results := make([]turn.ToolResult, len(req.ToolCalls))

    // Group tool calls by concurrency safety
    var concurrent, sequential []int
    for i, tc := range req.ToolCalls {
        if a.isConcurrencySafe(ctx, tc.Name) {
            concurrent = append(concurrent, i)
        } else {
            sequential = append(sequential, i)
        }
    }

    // Sequential first (deterministic for non-safe tools)
    for _, i := range sequential {
        results[i] = a.executeOne(ctx, req.ToolCalls[i])
    }

    // Concurrent (errgroup, indexed write back)
    if len(concurrent) > 1 {
        var eg errgroup.Group
        for _, i := range concurrent {
            i := i
            eg.Go(func() error {
                results[i] = a.executeOne(ctx, req.ToolCalls[i])
                return nil  // never fail individual call
            })
        }
        _ = eg.Wait()
    } else if len(concurrent) == 1 {
        results[concurrent[0]] = a.executeOne(ctx, req.ToolCalls[concurrent[0]])
    }

    return turn.ToolRoundResult{Results: results}, nil
}

func (a *contextEngineAdapter) isConcurrencySafe(ctx, name string) bool {
    for _, s := range a.surfaces {
        for _, spec := range s.Tools(ctx, "", "") {
            if spec.Name == name {
                return spec.ConcurrencySafe
            }
        }
    }
    return false
}
```

**关键保证**：`results[i] = ...` 用 indexed slice 写回，**结果顺序与 req.ToolCalls 顺序一致**。

### 2.2 方案 B（不推荐）：只加 bool 字段，不加 InterruptBehavior

只解决 1.1 / 1.2 / 1.4，**不解决 1.3**。

- ✅ 改动量更小（~150 行）
- ❌ 长 run tool 仍无法 cancel，60s timeout 问题继续
- ❌ 跟 clawcode 完整借鉴对比，少 1 块拼图

### 2.3 方案 C（备选）：用 enum 替代 4 bool

```go
type ToolCategory int
const (
    CategoryReadOnly ToolCategory = 1 << iota
    CategoryDestructive
    CategoryOpenWorld
    CategoryConcurrencySafe
)
```

- ✅ 单字段多 flag
- ❌ 与既有 4 维 PerAgentFilter / PerRiskFilter 决策路径不匹配
- ❌ 4 bool 在 JSON Marshal / t-registry 文档里更易读
- ❌ Go enum 不可扩展（加新 flag 要改 type）

### 2.4 决策

**选择方案 A**。理由：
1. 4 bool 与 clawcode 的 4 个独立方法（`isReadOnly() / isDestructive() / isOpenWorld() / isConcurrencySafe()`）一一对应，**借鉴路径最短**
2. InterruptBehavior 是 1 行 method addition，**与 4 bool 是独立关注点**，合并到 1 个 change 减少 PR 数
3. 1 行 sort.Slice **几乎零成本**，但 prompt cache 提升明显
4. 并行 dispatch 是 4 bool 字段的**直接受益者**（没有 ConcurrencySafe 字段就没法做）

---

## 3. 实施计划

| 阶段 | 任务 | 估时 | 交付物 |
|---|---|---|---|
| 1 | 创建 `feat/devrix-tool-spec-enrichment` 分支 | 1 min | git branch |
| 2 | S3 design.md 完整设计（含 7 bool 字段填充决策表完整版） | 1 h | design.md |
| 3 | ToolSpec + 4 bool 字段 + InterruptMode enum + ToolSurface +1 method | 1 h | contracts/tool_surface.go |
| 4 | 7 surface 各加 ToolSpec 字段填充 + InterruptBehavior 实现 | 3 h | 7 surface.go + 7 surface_test.go |
| 5 | BuildSurfaces sort.Slice | 5 min | bootstrap/surfaces.go |
| 6 | turn_adapter.ExecuteRound 并行 dispatch | 2 h | bootstrap/turn_adapter.go |
| 7 | 全量回归 + 4 个新 P0 T 点（T22-T25） | 2 h | tests/integration/tool_surface_test.go + tests/turn_adapter_test.go |
| 8 | S5 验收 + S6 归档（PR + auto-merge） | 1 h | PR + verify-archive.sh |
| **总计** | | **~10 h (1.5 day)** | |

### 3.1 执行顺序

1. Step 2-3（先有 design，后有 interface）
2. Step 4-5（7 surface 改 + 1 行 sort，可并行）
3. Step 6（依赖 Step 4 完成）
4. Step 7（依赖 Step 4-6）
5. Step 8（依赖 Step 7）

每个 Step 完成后立即 `git add` + `git commit`（独立 commit），便于回滚与 review。

---

## 4. 成功指标

| Metric | Baseline | Target | 测量 |
|---|---|---|---|
| ToolSpec bool 字段覆盖 surface 数 | 0/7 | 7/7 | `git grep "ReadOnly:.*true\|ReadOnly:.*false" internal/layers/contextengine/enforce/toolrunner/surface/` 命中 7 文件 |
| ToolSurface 实现 InterruptBehavior surface 数 | 0/7 | 7/7 | `git grep "InterruptBehavior" internal/layers/contextengine/enforce/toolrunner/surface/` 命中 7 文件 |
| BuildSurfaces 输出顺序跨 env 稳定性 | 不稳定 | 稳定 | T24: 3 套不同 opts 输入，输出 Names() 字符串完全相同 |
| turn_adapter 并行 dispatch 加速比 | 1.0x | 1.5-2.5x | T25: 2 个独立 read_file 并行时间 < 1.5x 单个 |
| Long run tool interrupt 响应时间 | 60s (timeout) | < 200ms | T23: FreeForkSurface.Execute 在 ctx cancel 后 200ms 内返回 |
| 既有 P0 T 点（11 个） | 11/11 PASS | 11/11 PASS | `go test -race ./...` |
| `go vet ./...` + `staticcheck` warning | 0 | 0 | CI |
| 单测覆盖率（新增代码） | n/a | ≥ 80% | `go test -cover` |

---

## 5. 风险与缓解

| 风险 | 等级 | 缓解 |
|---|---|---|
| **ToolSurface interface 加方法 = breaking** | H | 7 surface 必须全部改；S3-Gate 走严格 review 5 段式；compile-time `var _ contracts.ToolSurface = ...` 7 处 assertion 必须 PASS |
| **turn_adapter 并行 dispatch 结果顺序错乱** | H | 集成测试断言 `results[i] == req.ToolCalls[i]` 对应；用 errgroup + indexed slice 写回 |
| **4 bool 字段填充口径不一致** | M | S3 design.md 给出完整决策表（已在 §2.1.2）；7 surface 改完后 1 个 PR commit 集中 review |
| **BuildSurfaces sort 后 LLM 工具顺序变了导致回归** | L | T24 显式守护；既有 T08（per-agent ⊇ main）保持 PASS |
| **InterruptBehavior 与 ctx cancel 协议冲突** | L | T23 覆盖：D7 turn cancel → D2 surface.Execute select ctx.Done() → 200ms 内返回 ctx.Err() |
| **D7 turn 调度引入并发 = 新 race condition** | M | `go test -race ./...` 必须 100% 绿；errgroup 内部访问 results[i] 用局部 i 闭包 |

---

## 6. Open Questions

| Q | 状态 | 决策 |
|---|---|---|
| 4 bool 字段默认值是 true 还是 false？ | S3 决 | **全 false**（保守；既有 surface 必须显式标注） |
| InterruptBehavior 默认值？ | S3 决 | **`block`**（既有 7 surface 行为不变；只 FreeForkSurface 显式 cancel） |
| BuildSurfaces sort 放在哪一层？ | S3 决 | **`bootstrap/surfaces.go:BuildSurfaces` 内部**（单一收敛点；不在 surface 内） |
| turn_adapter 并行是 go 关键字还是 errgroup？ | S3 决 | **errgroup**（已 errgroup.Group 在 devrix 其它地方用；统一风格） |
| ConcurrencySafe=false 的 tool 串行还是也并行？ | S3 决 | **串行**（可能与 ConcurrencySafe=true 的有依赖；保守） |
| 既有 11 个 P0 T 点是否要加 bool 字段断言？ | S3 决 | **否**（保持向后兼容；T22 是新加的，专门测 bool 字段） |
| 是否要在 S3 design.md 给出 7 surface bool 字段填充完整代码？ | S3 决 | **是**（避免 S4 实施时 7 surface 各自解读不一致） |

---

## 7. Out of Scope

- **不修改** 13 个 diagnostic-tools-parity library 对外 API
- **不重写** `IPermissionGate` 接口（per-tool checkPermission 是 DM-002 范围）
- **不实现** Zod-equivalent schema 验证（DM-003 范围）
- **不引入** SurfaceSearch / Lazy loading（DM-003 范围）
- **不实现** MCP 集成（不在本轮 roadmap）
- **不修改** 既有的 11 个 P0 T 点期望值（保持 PASS）
- **不重构** 任何 global / singleton（DM-008 已闭环 0 global）
- **不修改** `ToolSpec.Risk` 字段（保留向后兼容）

---

## 8. 关联参考

- 上游 change：`openspec/archive/2026-06-17-devrix-tool-surface-contract/` (DM-007)
- 上游 change：`openspec/archive/2026-06-17-devrix-tool-surface-phase2-full/` (DM-008)
- 借鉴源：`docs/reference/clawcode-tool-design-comparison.md` §8.1 P0-(1) + P0-(3)
- clawcode 参考实现：
  - `clawcode/src/Tool.ts:402-407` (`isReadOnly / isDestructive / isConcurrencySafe`)
  - `clawcode/src/Tool.ts:410-416` (`interruptBehavior: 'cancel' | 'block'`)
  - `clawcode/src/tools.ts:362-366` (byName sort for prompt cache stability)
  - `clawcode/src/Tool.ts:404` (`isOpenWorld`)
- DSAFT 方法论：`docs/methodology/dsaft-methodology.md` §12 (Facet Decomposition)
- 域归档：`openspec/specs/d2-context-engine/`, `openspec/specs/d7-orchestration/`
- T 注册表：`openspec/specs/tool-surface/t-registry.md`
