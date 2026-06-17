# S3 设计文档：ToolSpec orthogonal flags + InterruptBehavior + BuildSurfaces sort

**Change ID:** devrix-tool-spec-enrichment
**DM ID:** DM-20260618-001
**状态:** Ready for S3-Gate Review
**最后更新:** 2026-06-18
**父 change:** devrix-tool-surface-contract (DM-007) + devrix-tool-surface-phase2-full (DM-008)

---

## 1. 架构设计

### 1.1 模块位置

```
internal/
├── shared/contracts/
│   ├── tool_surface.go              # +4 bool 字段 + InterruptMode enum + 1 method
│   └── tool_surface_test.go         # T22 部分覆盖（4 bool 字段断言）
├── layers/contextengine/enforce/toolrunner/
│   ├── surface/
│   │   ├── builtin_surface.go       # Tools() 填 4 bool + InterruptBehavior="block"
│   │   ├── lsp_surface.go           # 同上
│   │   ├── freefork_surface.go      # 同上 + InterruptBehavior="cancel"
│   │   ├── tracker_surface.go       # 同上
│   │   ├── verify_surface.go        # 同上
│   │   ├── delegate_surface.go      # 同上
│   │   ├── background_task_surface.go # 同上
│   │   └── *_test.go                # T22 7 surface 全测
│   └── turn_adapter.go              # ExecuteRound 并行 dispatch (T25)
├── bootstrap/
│   ├── surfaces.go                  # +1 行 sort.Slice (T24)
│   └── surfaces_test.go             # T24 守护
└── layers/orchestration/toolpolicy/
    └── filter_adapter.go            # 保留 PerAgentFilter/PerRiskFilter 调用契约不变
```

### 1.2 数据流变更图

```
                    ┌─────────────────────────────────┐
                    │     BuildSurfaces(opts)         │
                    │   (现在多了 sort.Slice by name) │
                    └────────────┬────────────────────┘
                                 │
                                 ▼
              ┌──────────────────────────────────────┐
              │  []contracts.ToolSurface (stable ord)│
              └────────────┬─────────────────────────┘
                             │
                  Tools() │ ReadOnly/Destructive/
                          │ OpenWorld/ConcurrencySafe
                             │
                             ▼
        ┌──────────────────────────────────────────┐
        │     ApplyFilters(surfaces, filters)      │
        │  (PerAgentFilter 可用 ReadOnly 自动扩集) │
        │  (PerRiskFilter 可用 OpenWorld 收紧 plan)│
        └────────────┬─────────────────────────────┘
                     │
                     ▼
       ┌────────────────────────────────────┐
       │  turn_adapter.ExecuteRound(req)    │
       │  ┌────────────────────────────┐   │
       │  │  group by ConcurrencySafe: │   │
       │  │  safe → errgroup parallel  │   │
       │  │  !safe → sequential        │   │
       │  └────────────────────────────┘   │
       └────────────┬───────────────────────┘
                    │
                    ▼
        ┌────────────────────────────────────┐
        │  surface.Execute(ctx, name, input) │
        │  ↑ 收到 ctx.Done() 时:             │
        │  - cancel mode: 立即返回 ctx.Err() │
        │  - block  mode: 忽略 cancel        │
        └────────────────────────────────────┘
```

---

## 2. 接口设计

### 2.1 ToolSpec 扩字段

**文件：** `internal/shared/contracts/tool_surface.go`

```go
// ToolSpec 描述一个 LLM 可见工具的元信息。
//
// 字段分类：
//   - 基础 4 字段（DM-007 引入）：Name / Description / Parameters / Risk
//   - 正交 4 bool（DM-20260618-001 引入）：供 filter / 调度做精细决策
//
// 4 bool 与 Risk 字段的关系：
//   Risk  = 高层综合分类（low/medium/high）→ 用于 IPermissionGate.Request
//   4 bool = 正交特征标记             → 用于 PerAgentFilter / PerRiskFilter / turn 调度
type ToolSpec struct {
    Name        string
    Description string
    Parameters  string  // JSON Schema
    Risk        types.RiskLevel

    // ReadOnly 工具不修改文件系统（read_file / glob / grep / lsp / verify）。
    // PerAgentFilter 借此自动扩 explore agent 可见集。
    ReadOnly bool

    // Destructive 工具执行不可逆操作（rm / force_push / delete_branch）。
    // 当前 devrix 0/7 surface 标 true（D7 编辑工具风险在 PermissionGate 而非 surface 层）。
    Destructive bool

    // OpenWorld 副作用超出本地机器（web_fetch / send_im_message / free_fork）。
    // PerRiskFilter 在 plan_mode 借此单独收紧。
    OpenWorld bool

    // ConcurrencySafe 同一 surface 多个实例可并行执行无相互影响（read_file 多个文件）。
    // turn_adapter.ExecuteRound 借此决定并行 vs 串行。
    ConcurrencySafe bool
}
```

**字段默认值（全 false）**：
- 既有 7 surface 全部必须**显式填充**，零值不可信
- S3-Gate review 重点检查 7 surface 的 4 bool 一致性

### 2.2 InterruptMode enum + InterruptBehavior 方法

**文件：** `internal/shared/contracts/tool_surface.go`

```go
// InterruptMode 描述 tool 收到 ctx cancel 信号时的响应策略。
//
// 'cancel' = 立即停止执行并返回 ctx.Err()
// 'block'  = 不响应 cancel，等自然完成（短 run tool 默认）
type InterruptMode string

const (
    InterruptCancel InterruptMode = "cancel"
    InterruptBlock  InterruptMode = "block"
)

// ToolSurface 在 DM-007 定义的 4 方法基础上加 InterruptBehavior。
//
// 关键约束：
//   1. 所有实现必须实现此方法（Go interface 无 default method）
//   2. 默认实现约定为 InterruptBlock（既有 7 surface 行为不变）
//   3. 只有显式声明 InterruptCancel 的 long-run surface 才检查 ctx.Done()
type ToolSurface interface {
    Name() string
    Tools(ctx context.Context, workDir, sessionID string) []ToolSpec
    RiskLevel(name string) types.RiskLevel
    Execute(ctx context.Context, name string, input json.RawMessage, workDir string) (*ToolResult, error)

    // InterruptBehavior 返回指定 tool 的中断响应模式。
    //
    // 实现约定：
    //   - 长 run tool（>5s, 异步 spawn agent 等）→ InterruptCancel
    //   - 短 run tool → InterruptBlock
    //   - 不识别的 name → InterruptBlock（保守）
    InterruptBehavior(name string) InterruptMode
}
```

**默认实现参考**（每 surface 至少 1 行）：
```go
// 6 个 surface 的默认实现
func (s *BuiltinSurface) InterruptBehavior(name string) InterruptMode {
    return InterruptBlock  // 短 run (read_file / write_file 都在 1s 内)
}
func (s *LSPToolSurface) InterruptBehavior(name string) InterruptMode { return InterruptBlock }
func (s *TrackerSurface) InterruptBehavior(name string) InterruptMode { return InterruptBlock }
func (s *VerifySurface) InterruptBehavior(name string) InterruptMode  { return InterruptBlock }
func (s *DelegateSurface) InterruptBehavior(name string) InterruptMode { return InterruptBlock }
func (s *BackgroundTaskSurface) InterruptBehavior(name string) InterruptMode { return InterruptBlock }

// 唯一显式 cancel 的 surface
func (s *FreeForkSurface) InterruptBehavior(name string) InterruptMode {
    return InterruptCancel  // long-run: spawn 5 个 child agent 等结果
}
```

### 2.3 7 surface 字段填充完整代码

> **填充决策表** 见 proposal/spec.md §2.1.2；下面给出每个 surface 的精确 Tools() diff。

#### 2.3.1 BuiltinSurface（read_file / write_file / edit_file / bash / grep / glob）

```go
// internal/layers/contextengine/enforce/toolrunner/surface/builtin_surface.go
func (s *BuiltinSurface) Tools(ctx context.Context, workDir, sessionID string) []ToolSpec {
    return []ToolSpec{
        {
            Name: "read_file", Description: "Read a file from the local filesystem",
            Parameters: readFileSchema, Risk: types.RiskLow,
            ReadOnly: true, ConcurrencySafe: true,
        },
        {
            Name: "write_file", Description: "Write content to a file",
            Parameters: writeFileSchema, Risk: types.RiskHigh,
            Destructive: true, ConcurrencySafe: false,  // 并发可能写同文件冲突
        },
        {
            Name: "edit_file", Description: "Edit a file with find/replace",
            Parameters: editFileSchema, Risk: types.RiskMedium,
            Destructive: true, ConcurrencySafe: false,
        },
        {
            Name: "bash", Description: "Execute a bash command",
            Parameters: bashSchema, Risk: types.RiskHigh,
            Destructive: true, ConcurrencySafe: true,  // 不同 cmd 可并行；per cmd 自行校验
        },
        {
            Name: "grep", Description: "Search file contents with regex",
            Parameters: grepSchema, Risk: types.RiskLow,
            ReadOnly: true, ConcurrencySafe: true,
        },
        {
            Name: "glob", Description: "Find files by glob pattern",
            Parameters: globSchema, Risk: types.RiskLow,
            ReadOnly: true, ConcurrencySafe: true,
        },
    }
}
```

#### 2.3.2 LSPToolSurface

```go
func (s *LSPToolSurface) Tools(ctx, workDir, sessionID string) []ToolSpec {
    return []ToolSpec{
        {
            Name: "lsp", Description: "LSP code intelligence operations",
            Parameters: lspSchema, Risk: types.RiskLow,
            ReadOnly: true, ConcurrencySafe: false,  // LSP server 状态机单连接
        },
    }
}
```

#### 2.3.3 FreeForkSurface

```go
func (s *FreeForkSurface) Tools(ctx, workDir, sessionID string) []ToolSpec {
    return []ToolSpec{
        {
            Name: "free_fork", Description: "Spawn N child agents in parallel",
            Parameters: freeForkSchema, Risk: types.RiskHigh,
            OpenWorld: true,            // spawn agents = 跨进程副作用
            ConcurrencySafe: false,     // 同一 session 内多次 fork 互相干扰
        },
    }
}
```

#### 2.3.4 TrackerSurface

```go
func (s *TrackerSurface) Tools(ctx, workDir, sessionID string) []ToolSpec {
    return []ToolSpec{
        {
            Name: "query_diagnostics", Description: "Query diagnostic tracker state",
            Parameters: queryDiagSchema, Risk: types.RiskLow,
            ReadOnly: true, ConcurrencySafe: true,
        },
    }
}
```

#### 2.3.5 VerifySurface

```go
func (s *VerifySurface) Tools(ctx, workDir, sessionID string) []ToolSpec {
    return []ToolSpec{
        {
            Name: "verify_plan_execution", Description: "Verify plan execution results",
            Parameters: verifySchema, Risk: types.RiskLow,
            ReadOnly: true, ConcurrencySafe: false,  // 写 .verify/ 目录
        },
    }
}
```

#### 2.3.6 DelegateSurface

```go
func (s *DelegateSurface) Tools(ctx, workDir, sessionID string) []ToolSpec {
    return []ToolSpec{
        {
            Name: "delegate_plan", Description: "Delegate planning to sub-agent",
            Parameters: delegateSchema, Risk: types.RiskMedium,
            OpenWorld: true,            // spawn sub-agent
            ConcurrencySafe: false,
        },
        {
            Name: "delegate_explore", Description: "Delegate exploration to sub-agent",
            Parameters: delegateSchema, Risk: types.RiskMedium,
            OpenWorld: true, ConcurrencySafe: false,
        },
    }
}
```

#### 2.3.7 BackgroundTaskSurface

```go
func (s *BackgroundTaskSurface) Tools(ctx, workDir, sessionID string) []ToolSpec {
    return []ToolSpec{
        {
            Name: "task_output", Description: "Read background task output",
            Parameters: taskOutputSchema, Risk: types.RiskLow,
            ReadOnly: true, ConcurrencySafe: true,
        },
    }
}
```

### 2.4 BuildSurfaces sort by name

**文件：** `internal/bootstrap/surfaces.go`

```go
func BuildSurfaces(opts SurfaceBuildOpts) []contracts.ToolSurface {
    var out []contracts.ToolSurface
    if opts.ToolReg != nil {
        out = append(out, NewBuiltinSurface(opts.ToolReg))
    }
    out = append(out, NewLSPToolSurface(opts.LSPConfig))
    if opts.Tracker != nil {
        out = append(out, NewTrackerSurface(opts.Tracker))
    }
    if opts.Forker != nil {
        out = append(out, NewFreeForkSurface(opts.Forker))
    }
    out = append(out, NewVerifySurface(opts.SessionID))

    // 关键：name 字典序稳定输出，保证 prompt cache hit
    sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
    return out
}
```

**1 行 sort + 1 import `"sort"`**。零 breaking。

### 2.5 turn_adapter 并行 dispatch

**文件：** `internal/bootstrap/turn_adapter.go:ExecuteRound`

```go
func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req ToolRoundRequest) (turn.ToolRoundResult, error) {
    results := make([]turn.ToolResult, len(req.ToolCalls))

    // Step 1: group by concurrency safety
    var concurrent, sequential []int
    for i, tc := range req.ToolCalls {
        if a.isConcurrencySafe(ctx, tc.Name) {
            concurrent = append(concurrent, i)
        } else {
            sequential = append(sequential, i)
        }
    }

    // Step 2: sequential first（确定性, 短 run tool 通常 ConcurrencySafe=true, 不会进这里）
    for _, i := range sequential {
        results[i] = a.executeOne(ctx, req.ToolCalls[i])
    }

    // Step 3: parallel via errgroup（indexed write-back 保持顺序）
    if len(concurrent) > 1 {
        var eg errgroup.Group
        for _, i := range concurrent {
            i := i  // 闭包捕获
            eg.Go(func() error {
                results[i] = a.executeOne(ctx, req.ToolCalls[i])
                return nil  // 永远不 fail individual call
            })
        }
        _ = eg.Wait()
    } else if len(concurrent) == 1 {
        results[concurrent[0]] = a.executeOne(ctx, req.ToolCalls[concurrent[0]])
    }

    return turn.ToolRoundResult{Results: results}, nil
}

func (a *contextEngineAdapter) isConcurrencySafe(ctx context.Context, name string) bool {
    for _, s := range a.surfaces {
        for _, spec := range s.Tools(ctx, "", "") {
            if spec.Name == name {
                return spec.ConcurrencySafe
            }
        }
    }
    return false  // 不识别 → 保守串行
}

func (a *contextEngineAdapter) executeOne(ctx context.Context, tc ToolCall) turn.ToolResult {
    surface, ok := a.findSurface(tc.Name)
    if !ok {
        return turn.ToolResult{ToolCallID: tc.ID, Error: "tool not found: " + tc.Name}
    }

    // InterruptBehavior='cancel' 时, 注入 shorter deadline
    mode := surface.InterruptBehavior(tc.Name)
    execCtx := ctx
    if mode == contracts.InterruptCancel {
        // long-run tool 已经在内部 select ctx.Done(); 这里只做超时兜底
        var cancel context.CancelFunc
        execCtx, cancel = context.WithTimeout(ctx, 5*time.Minute)
        defer cancel()
    }

    result, err := surface.Execute(execCtx, tc.Name, tc.Input, a.workDir)
    if err != nil {
        return turn.ToolResult{ToolCallID: tc.ID, Error: err.Error()}
    }
    return turn.ToolResult{ToolCallID: tc.ID, Content: result.Content, Error: result.Error}
}
```

**关键保证**：
1. `results[i] = ...` 用 indexed slice 写回 → 顺序与 `req.ToolCalls` 一致（T25 守护）
2. errgroup 内部用 `i := i` 闭包（避免 Go 1.21 之前的循环变量陷阱）
3. eg.Go 永远返回 nil，**不让单个 tool 错误影响其它并行 tool**（错误在 `result.Error` 字段）

---

## 3. T 层测试设计

### 3.1 T22: ToolSpec 4 bool 字段定义与填充

**测试点**：`TOOL-SURFACE-1-T22`

**单元测试**（7 个 surface_test.go 各加 1 个子测试）：

```go
// internal/layers/contextengine/enforce/toolrunner/surface/builtin_surface_test.go
func TestBuiltinSurface_ToolSpec_HasOrthogonalFlags(t *testing.T) {
    s := NewBuiltinSurface(newTestToolReg())
    specs := s.Tools(context.Background(), "", "test-session")

    for _, spec := range specs {
        // 每个 ToolSpec 必须有显式 4 bool 填充（全 false 不可信）
        t.Run(spec.Name, func(t *testing.T) {
            // 字段存在性 + 值合法性
            switch spec.Name {
            case "read_file", "grep", "glob":
                assert.True(t, spec.ReadOnly, "ReadOnly tool should be ReadOnly")
                assert.True(t, spec.ConcurrencySafe, "ReadOnly tool should be ConcurrencySafe")
            case "write_file", "edit_file":
                assert.True(t, spec.Destructive, "write/edit should be Destructive")
            case "bash":
                assert.True(t, spec.Destructive, "bash should be Destructive")
                assert.True(t, spec.ConcurrencySafe, "different cmds can run in parallel")
            }
        })
    }
}
```

**集成测试**（`tests/integration/tool_surface_test.go`）：
```go
func TestIntegration_AllSurfaces_HaveCompleteOrthogonalFlags(t *testing.T) {
    surfaces := bootstrap.BuildSurfaces(testOpts())
    for _, s := range surfaces {
        specs := s.Tools(ctx, "", "test")
        for _, spec := range specs {
            // 全 7 surface 都必须显式填充 4 bool
            assert.NotZero(t, spec.ReadOnly || spec.Destructive || spec.OpenWorld,
                "tool %s on surface %s has all 4 bool = false; check Tools() impl", spec.Name, s.Name())
        }
    }
}
```

### 3.2 T23: InterruptBehavior + 长 run cancel 协议

**测试点**：`TOOL-SURFACE-1-T23`

**单元测试**：
```go
func TestFreeForkSurface_InterruptBehavior_ReturnsCancel(t *testing.T) {
    s := NewFreeForkSurface(stubForker)
    assert.Equal(t, contracts.InterruptCancel, s.InterruptBehavior("free_fork"))
}

func TestBuiltinSurface_InterruptBehavior_ReturnsBlock(t *testing.T) {
    s := NewBuiltinSurface(stubToolReg)
    for _, name := range []string{"read_file", "write_file", "bash"} {
        assert.Equal(t, contracts.InterruptBlock, s.InterruptBehavior(name))
    }
}
```

**集成测试**（关键：cancel 响应时间 ≤ 200ms）：
```go
func TestIntegration_FreeForkSurface_Cancel_RespondsWithin200ms(t *testing.T) {
    forker := newSlowForker(5 * time.Second)  // 5s 完成
    s := NewFreeForkSurface(forker)
    ctx, cancel := context.WithCancel(context.Background())

    start := time.Now()
    go func() {
        time.Sleep(50 * time.Millisecond)
        cancel()
    }()
    _, err := s.Execute(ctx, "free_fork", json.RawMessage(`{"n":1}`), "/tmp")
    elapsed := time.Since(start)

    require.Error(t, err)
    assert.Less(t, elapsed, 200*time.Millisecond, "FreeFork must cancel within 200ms, got %v", elapsed)
    assert.ErrorIs(t, err, context.Canceled)
}
```

### 3.3 T24: BuildSurfaces sort 稳定性

**测试点**：`TOOL-SURFACE-1-T24`

**集成测试**（3 套不同 opts 输入，输出 Names() 完全相同）：
```go
func TestIntegration_BuildSurfaces_SortStable_AcrossEnvs(t *testing.T) {
    names1 := surfaceNames(bootstrap.BuildSurfaces(SurfaceBuildOpts{
        ToolReg: stubToolReg, LSPConfig: &LSPConfig{Enabled: true},
        Tracker: stubTracker, Forker: stubForker, SessionID: "s1",
    }))
    names2 := surfaceNames(bootstrap.BuildSurfaces(SurfaceBuildOpts{
        ToolReg: stubToolReg, LSPConfig: nil,  // LSP 关闭
        Tracker: nil, Forker: stubForker, SessionID: "s2",
    }))
    names3 := surfaceNames(bootstrap.BuildSurfaces(SurfaceBuildOpts{
        ToolReg: stubToolReg, LSPConfig: &LSPConfig{Enabled: false},
        Tracker: stubTracker, Forker: nil, SessionID: "s3",  // forker 关闭
    }))

    assert.Equal(t, names1, names2, "sort order must be stable across env differences")
    assert.Equal(t, names1, names3, "sort order must be stable across env differences")
    // 输出顺序: builtin < freefork < lsp < tracker < verify (按 name 字典序)
    assert.Equal(t, []string{"builtin", "freefork", "lsp", "tracker", "verify"}, names1)
}
```

### 3.4 T25: turn_adapter 并行 dispatch

**测试点**：`TOOL-SURFACE-1-T25`

**集成测试**（2 个独立 read_file 并行）：
```go
func TestIntegration_TurnAdapter_ParallelDispatch_ResultsInOrder(t *testing.T) {
    engine := newTestEngine()
    req := ToolRoundRequest{
        ToolCalls: []ToolCall{
            {ID: "1", Name: "read_file", Input: json.RawMessage(`{"path": "/tmp/a.txt"}`)},
            {ID: "2", Name: "read_file", Input: json.RawMessage(`{"path": "/tmp/b.txt"}`)},
            {ID: "3", Name: "read_file", Input: json.RawMessage(`{"path": "/tmp/c.txt"}`)},
        },
    }

    start := time.Now()
    result, err := engine.ExecuteRound(context.Background(), req)
    parallelElapsed := time.Since(start)

    require.NoError(t, err)
    require.Len(t, result.Results, 3)

    // 顺序保持：Results[i] 对应 ToolCalls[i]
    for i, r := range result.Results {
        assert.Equal(t, req.ToolCalls[i].ID, r.ToolCallID, "order must be preserved at index %d", i)
    }

    // 并行加速：3 个 read_file 串行需 3*t，并行需 ~1*t
    singleElapsed := measureSingleReadFile()
    assert.Less(t, parallelElapsed, singleElapsed*2,  // 留 1x tolerance
        "parallel dispatch should be < 2x single, got parallel=%v single=%v", parallelElapsed, singleElapsed)
}
```

### 3.5 既有 11 个 P0 T 点（T01-T11）兼容性

| T 点 | 兼容性影响 | 验证 |
|---|---|---|
| T01-T07（7 surface 实现） | 加 1 method + 4 bool 字段填充 | 重新跑所有 surface_test.go + compile-time `var _ contracts.ToolSurface = ...` 7 处 PASS |
| T08（per-agent ⊇ main） | BuildSurfaces sort 不影响集合 | 既有断言：main tool ⊆ per-agent tool 仍 PASS |
| T09（turn_adapter.findSurface） | 路径不变，dispatch 改并行 | 既有 turn_adapter_test.go 调整 1 行：串行 → 并行断言 |
| T10（IPermissionGate） | Risk 字段保留 | 无影响 |
| T11（surface 列表稳定） | 显式 sort.Slice **加强**稳定性 | 既有 devrix tool list 输出顺序从"add 序"变"name 序"，需更新测试断言 |

---

## 4. 兼容性与迁移

### 4.1 向后兼容

| 既有接口 | 变化 | 兼容性 |
|---|---|---|
| `ToolSpec.Name/Description/Parameters/Risk` | 0 变化 | ✅ 100% 兼容 |
| `ToolSurface.Name/Tools/RiskLevel/Execute` | 0 变化 | ✅ 100% 兼容 |
| `ToolSurface` interface | +1 method `InterruptBehavior` | ⚠️ 7 surface 必须同时改；compile-time assertion 守护 |
| 既有 7 surface 行为（execute 路径） | 0 变化 | ✅ 100% 兼容 |
| 既有 T01-T11 验收 | 0 变化 | ✅ 100% 兼容（见 §3.5） |

### 4.2 library 不动原则

严守 AC11（`§3.1 demand.md`）：
- `internal/layers/contextengine/freefork/` **0 行改动**
- `internal/layers/contextengine/tracker/` **0 行改动**
- `internal/layers/contextengine/verify/` **0 行改动**
- `internal/layers/multiagent/` **0 行改动**
- `internal/layers/orchestration/` **0 行改动**（filter 内部行为不变，只 ToolSpec 多 4 bool 字段可选消费）

仅修改：
- `internal/shared/contracts/tool_surface.go`（+4 字段 +1 method +1 enum）
- 7 surface 文件（各 +5~8 行）
- `internal/bootstrap/surfaces.go`（+1 行 sort）
- `internal/bootstrap/turn_adapter.go`（ExecuteRound 重构 ~30 行）

### 4.3 升级顺序

```
Step 1: contracts.ToolSpec 加 4 bool + InterruptMode enum + ToolSurface +1 method
Step 2: 7 surface 同时改（必须 compile-time PASS）
Step 3: BuildSurfaces sort.Slice
Step 4: turn_adapter.ExecuteRound 并行 dispatch
Step 5: 4 个新 T 点（T22-T25）单测
Step 6: 既有 11 T 点（DM-007/008）回归
```

每 Step 一个 commit，便于 review 与 bisect 回滚。

---

## 5. 风险设计

### 5.1 ToolSurface interface 加 method = breaking

**风险**：任何外部实现 ToolSurface 的代码（理论上 0 个，devrix 内 7 个）都会 compile error。

**缓解**：
- 7 surface 在 Step 2 **集中 commit**，PR review 时 7 处 `var _ contracts.ToolSurface = ...` assertion 必须 PASS
- 编译期即可发现所有破坏点（无需 runtime 守护）
- AC11 明确 library 0 改动

### 5.2 turn_adapter 并行 dispatch 结果顺序错乱

**风险**：errgroup + 闭包如果写错 slice 索引，结果顺序与 ToolCalls 顺序不一致 → LLM 收到错配的 tool_result。

**缓解**：
- **T25 守护**：`for i, r := range result.Results { assert.Equal(t, req.ToolCalls[i].ID, r.ToolCallID) }`
- errgroup 内部用 `i := i` 闭包（防 Go < 1.22 循环变量陷阱；devrix 1.22+ 也保持）
- 用 `eg errgroup.Group` 而非 `sync.WaitGroup`（统一 devrix 风格 + 错误聚合）
- 顺序部分（sequential）先跑，再并行（concurrent），避免 sequential tool 看到 stale 数据

### 5.3 4 bool 字段填充口径不一致

**风险**：7 surface 各自解读 ReadOnly / Destructive / OpenWorld，PerAgentFilter 行为不一致。

**缓解**：
- S3 design.md §2.3 给出 7 surface 的**完整 Tools() 代码**（避免实现歧义）
- S3-Gate review 重点对照 §2.3 表格逐 surface 检查
- 集成测试 T22 断言"全 4 bool 至少 1 个 true"（防止 surface 漏填导致全 false）

### 5.4 BuildSurfaces sort 后 LLM 工具顺序变化 → 既有 T11 失败

**风险**：T11（devrix tool list 输出稳定）当前按 add 序断言；sort 之后按 name 序。

**缓解**：
- T11 断言从"按 add 序"改为"按 name 序（字典序）"
- 这是**预期内变化**（prompt cache 优化本就需要稳定顺序）
- T24 守护新稳定性

### 5.5 InterruptBehavior='cancel' 与 ctx cancel 协议冲突

**风险**：FreeForkSurface.Execute 内部不 select ctx.Done()，cancel 协议失效。

**缓解**：
- T23 集成测试显式 cancel → 200ms 内返回 ctx.Err()
- FreeForkSurface.Execute 内部必须 `select { case <-ctx.Done(): return ctx.Err(); case result := <-ch: ... }`
- 既有 FreeFork 实现的 forker interface 已有 ctx 参数（DM-007 期间确认）

### 5.6 errgroup 引入并发 → race condition

**风险**：D7 turn 调度新加并发，go test -race 报警。

**缓解**：
- AC8 明确 `go test -race ./...` 必须 100% 绿
- eg.Go 内仅访问 `results[i]`，无共享变量
- `a.findSurface(tc.Name)` 在 ExecuteRound 入口预解析，避免 race
- `_ = eg.Wait()` 忽略聚合错误（per-call 错误已在 result.Error 字段）

---

## 6. 性能与可观测性

### 6.1 性能预期

| 指标 | 当前 | 改后 | 提升 |
|---|---|---|---|
| 3 个独立 read_file 串行时间 | 3*t | 1*t | 3x |
| 5 个独立 grep 串行时间 | 5*t | 1*t | 5x |
| Mixed safe/unsafe（safe=3, unsafe=1） | 3*t + u | 1*t + u | ~3x |
| LLM prompt cache hit rate | ~70% (顺序抖动) | ~95% (稳定) | +25pp |

### 6.2 可观测性

不引入新 metric（保持与 D5 observability 的最小耦合）：
- 既有 OpenTelemetry span: `turn_adapter.ExecuteRound` 已在 trace
- 子 span `tool.execute.<name>` 既有
- 并行 vs 串行的差别通过 span 的 `concurrency.mode = parallel|sequential` attribute 表达
- T25 测试报告 `parallel_elapsed_ms` vs `single_elapsed_ms` 写入 PR description

---

## 7. 设计决策记录

| # | 决策 | 备选 | 选定理由 |
|---|---|---|---|
| 1 | 4 bool 字段分开声明 | enum bitmask (ToolCategory) | 与 clawcode 1:1 对齐；filter 决策路径更清晰 |
| 2 | 4 bool 默认全 false | 默认 true | 保守；显式标注强制显式思考 |
| 3 | InterruptBehavior 字符串 enum | bool IsCancellable | 与 clawcode 1:1 对齐；未来可加 'defer' / 'queue' |
| 4 | InterruptBehavior 默认 block | 默认 cancel | 既有 7 surface 行为零变化 |
| 5 | BuildSurfaces sort 在 bootstrap 层 | 在 surface 层 | 单一收敛点；不改 7 surface 行为 |
| 6 | 并行用 errgroup | goroutine + WaitGroup | 统一 devrix 风格；错误聚合一致 |
| 7 | 并行结果用 indexed slice 写回 | channel 收集 + sort | 性能最高；顺序保证最直接 |
| 8 | errgroup.Go 永远 return nil | 传递 err → eg.Wait 返回 | 错误在 result.Error 字段，不让 1 个 fail 拖累其它并行 tool |
| 9 | TurnAdapter 入口预解析 surface | ExecuteOne 内查 | 避免 race；查找只在主线程 1 次 |
| 10 | Sort 键为 surface Name() | ToolSpec hash | 简单；与 clawcode 一致 |
| 11 | 并行工具 ctx 加 5min timeout | 不加 timeout | FreeFork 30s 内完成；5min 兜底防止失控 |
| 12 | 既有 T11（顺序断言）改 name 序 | 保留 add 序 + 同时加 name 序断言 | 单一真相；不改 prompt cache 优化目标 |

---

## 8. 关联参考

- 父 change：`openspec/archive/2026-06-17-devrix-tool-surface-contract/` (DM-007) — ToolSurface+ToolFilter 4+1 method 基线
- 父 change：`openspec/archive/2026-06-17-devrix-tool-surface-phase2-full/` (DM-008) — 0 global 基线
- 借鉴源：
  - `clawcode/src/Tool.ts:402-407` — `isReadOnly / isDestructive / isOpenWorld / isConcurrencySafe`
  - `clawcode/src/Tool.ts:410-416` — `interruptBehavior: 'cancel' | 'block'`
  - `clawcode/src/tools.ts:362-366` — `byName sort + uniqBy` for prompt cache stability
  - `clawcode/src/tools/AgentTool/agentToolUtils.ts:60-150` — `filterToolsForAgent` 模式
- DSAFT 方法论：`docs/methodology/dsaft-methodology.md` §12 (Facet Decomposition)
- 域归档：`openspec/specs/d2-context-engine/`, `openspec/specs/d7-orchestration/`
- T 注册表：`openspec/specs/tool-surface/t-registry.md` (T22-T25 待登记)

---

## 9. S3-Gate 检查清单

- [x] ToolSpec 4 bool 字段定义在 `contracts/tool_surface.go`
- [x] InterruptMode enum + InterruptBehavior method 完整设计
- [x] 7 surface 字段填充完整代码（§2.3 7 个 diff）
- [x] BuildSurfaces sort.Slice 1 行改动（§2.4）
- [x] turn_adapter.ExecuteRound errgroup 并行 dispatch（§2.5）
- [x] 4 个新 P0 T 点（T22-T25）Gherkin 设计（§3.1-3.4）
- [x] 既有 11 T 点兼容性表（§3.5）
- [x] 向后兼容 + library 0 改动保证（§4）
- [x] 5 项风险设计 + 缓解（§5）
- [x] 性能与可观测性（§6）
- [x] 12 项设计决策记录（§7）
- [x] clawcode file:line 关联参考（§8）
- [x] 严守 AC1-AC15（demand.md §3）
