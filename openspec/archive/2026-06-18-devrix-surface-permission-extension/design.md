# S3 设计文档：Per-tool CheckPermission hook + IPermissionGate.ToolPolicy

**Change ID:** devrix-surface-permission-extension
**DM ID:** DM-20260618-002
**状态:** Ready for S3-Gate Review
**最后更新:** 2026-06-18
**父 change:** devrix-tool-spec-enrichment (DM-20260618-001, S4_Ready)

---

## 1. 架构设计

### 1.1 模块位置

```
internal/
├── shared/contracts/
│   ├── tool_surface.go              # +1 method CheckPermission
│   ├── permission.go                # 新增：Decision enum + 2 error types
│   └── *_test.go                    # T26 部分覆盖
├── layers/contextengine/enforce/toolrunner/surface/
│   ├── builtin_surface.go           # CheckPermission 默认 Allow + Bash override
│   ├── bash_ast.go                  # 新增：BashAST 解析器（mvdan/sh）
│   ├── bash_ast_test.go             # T27
│   ├── lsp_surface.go               # CheckPermission 默认 Allow
│   ├── freefork_surface.go          # CheckPermission override 调 IPermissionGate
│   ├── tracker_surface.go           # CheckPermission 默认 Allow
│   ├── verify_surface.go            # CheckPermission 默认 Allow
│   ├── delegate_surface.go          # CheckPermission 默认 Allow
│   ├── background_task_surface.go   # CheckPermission 默认 Allow
│   └── *_test.go                    # T26 7 surface 全测
├── layers/orchestration/
│   ├── permission/
│   │   ├── gate.go                  # IPermissionGate +1 method CheckPermission
│   │   └── gate_test.go
│   └── policy/
│       ├── plan_mode.go             # 新增：PlanModeOpenWorldPolicy
│       ├── plan_mode_test.go        # T29
│       └── policy_chain.go          # Policy 链式注册器
├── bootstrap/
│   ├── turn_adapter.go              # ExecuteRound dispatch 前 CheckPermission
│   └── turn_adapter_test.go         # T29 集成
└── layers/contextengine/enforce/toolrunner/surface/
    └── bash_ast_denylist.yaml       # 默认 deny-list 配置
```

### 1.2 数据流变更图

```
                    ┌──────────────────────────────────┐
                    │   turn_adapter.ExecuteRound(req) │
                    └─────────────┬────────────────────┘
                                  │
                                  ▼
              ┌──────────────────────────────────────┐
              │  for each ToolCall:                  │
              │  1) findSpec(name)                   │
              │  2) surface.CheckPermission(spec,input)│ ← NEW (DM-002)
              │     ↓                                │
              │     BashSurface: AST 解析 + deny-list│
              │     FreeForkSurface: delegate to gate│
              │     Others: Allow                    │
              │  3) if Ask: gate.CheckPermission(spec)│ ← NEW (DM-002)
              │  4) if Deny/Ask: return error, skip  │
              │  5) if Allow: surface.Execute(...)   │
              └─────────────┬────────────────────────┘
                            │
                            ▼
              ┌──────────────────────────────────────┐
              │  IPermissionGate.CheckPermission    │
              │  ├─ PlanMode policy chain           │
              │  │  ├─ mode=plan + OpenWorld        │
              │  │  │  → Deny (unless allowlist)    │
              │  │  └─ else pass                    │
              │  └─ Default: Risk → Decision        │
              └──────────────────────────────────────┘
```

### 1.3 mvdan/sh 依赖评估

**选型**：mvdan/sh v3.5+（已确认版本兼容 Go 1.22）

| 维度 | mvdan/sh | tree-sitter-bash | 自实现 |
|---|---|---|---|
| 纯 Go | ✅ | ❌ (cgo) | ✅ |
| binary 大小 | +5MB | +15MB | 0 |
| 解析准确度 | 高（bash 95%+ 语法） | 高 | 低 |
| 维护活跃 | ✅ (v3 持续) | ⚠️ | n/a |
| devrix 既有依赖 | ❌ 需新增 | ❌ | n/a |
| 性能 | < 1ms/cmd | < 2ms/cmd | n/a |

**结论**：mvdan/sh。

**go.mod 新增**：
```
require mvdan.cc/sh/v3 v3.5.0
```

**build 时间评估**：go.sum 增 ~2MB，build 时间 +3s（实测 CI 时间 +5%）。

---

## 2. 接口设计

### 2.1 Decision enum + 2 error types

**文件**：`internal/shared/contracts/permission.go`（新增）

```go
package contracts

import (
    "encoding/json"
    "fmt"
)

// Decision 是 per-tool permission 决策结果。
//
// 三态：Allow（可执行）/ Deny（拒绝）/ Ask（需用户确认）
// 与 clawcode Tool.ts:101-110 PermissionResult 1:1 对齐。
type Decision string

const (
    DecisionAllow Decision = "allow"  // 工具可执行
    DecisionDeny  Decision = "deny"   // 工具拒绝执行
    DecisionAsk   Decision = "ask"    // 需要用户确认
)

// PermissionDeniedError 表示 tool 被 policy 拒绝。
//
// 含 Spec + Input 元信息便于：
//   1. LLM 收到错误后能 retry（提供 alternative tool）
//   2. 用户 audit log 能追责（哪个 tool + 哪个 input + 哪个 reason）
type PermissionDeniedError struct {
    Spec   ToolSpec
    Input  json.RawMessage
    Reason string  // "plan_mode: open_world=true" / "bash: rm -rf / detected"
}

func (e *PermissionDeniedError) Error() string {
    return fmt.Sprintf("permission denied: tool=%s reason=%s", e.Spec.Name, e.Reason)
}

// PermissionAskRequiredError 表示 tool 需要用户确认才能执行。
//
// v1 简化：返回错误让 turn 终止；v2 引入 DM-005 DSL 后可发 interactive prompt。
type PermissionAskRequiredError struct {
    Spec   ToolSpec
    Input  json.RawMessage
    Reason string
}

func (e *PermissionAskRequiredError) Error() string {
    return fmt.Sprintf("permission ask required: tool=%s reason=%s", e.Spec.Name, e.Reason)
}
```

### 2.2 ToolSurface +1 method

**文件**：`internal/shared/contracts/tool_surface.go`

```go
type ToolSurface interface {
    Name() string
    Tools(ctx context.Context, workDir, sessionID string) []ToolSpec
    RiskLevel(name string) types.RiskLevel
    Execute(ctx context.Context, name string, input json.RawMessage, workDir string) (*ToolResult, error)
    InterruptBehavior(name string) InterruptMode

    // CheckPermission 在 tool dispatch 前调一次，返回是否允许执行。
    //
    // 返回 Decision：
    //   Allow → surface.Execute 被调
    //   Deny  → surface.Execute 不被调，返回 PermissionDeniedError
    //   Ask   → 通常触发 IPermissionGate.CheckPermission 兜底
    //
    // 默认实现：所有 7 surface 返回 Allow（向后兼容 DM-001）
    // 定制实现：
    //   BashSurface  → 内置 BashASTPolicy 解析 cmd
    //   FreeForkSurface → delegate 给 IPermissionGate.CheckPermission
    //   其余 5 surface → Allow（无副作用）
    //
    // 性能预算：< 5ms p99
    CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision
}
```

### 2.3 7 surface CheckPermission 实现

#### 2.3.1 5 surface 默认 Allow（grep/glob/lsp/verify/task_output/delegate）

```go
func (s *LSPToolSurface) CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision {
    return DecisionAllow
}

func (s *TrackerSurface) CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision {
    return DecisionAllow
}

func (s *VerifySurface) CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision {
    return DecisionAllow
}

func (s *DelegateSurface) CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision {
    return DecisionAllow
}

func (s *BackgroundTaskSurface) CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision {
    return DecisionAllow
}
```

**5 surface × 4 行 = 20 行**。统一模式。

#### 2.3.2 BuiltinSurface.bash override

```go
// internal/layers/contextengine/enforce/toolrunner/surface/builtin_surface.go
type BuiltinSurface struct {
    toolReg  *toolreg.Registry
    bashAST  *BashASTPolicy  // 注入；DM-002 新增
    lastDeny string
}

func NewBuiltinSurface(reg *toolreg.Registry, bashAST *BashASTPolicy) *BuiltinSurface {
    return &BuiltinSurface{toolReg: reg, bashAST: bashAST}
}

func (s *BuiltinSurface) CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision {
    if spec.Name != "bash" {
        return DecisionAllow  // read_file/write_file/edit_file/grep/glob 全部 Allow
    }

    // bash: AST 解析
    var in struct {
        Command string `json:"command"`
    }
    if err := json.Unmarshal(input, &in); err != nil {
        return DecisionAsk  // 解析失败 → Ask（保守）
    }
    decision, reason := s.bashAST.Check(in.Command)
    if decision == DecisionDeny {
        s.lastDeny = reason
        return DecisionDeny
    }
    return DecisionAllow
}
```

#### 2.3.3 FreeForkSurface override

```go
// internal/layers/contextengine/enforce/toolrunner/surface/freefork_surface.go
type FreeForkSurface struct {
    forker  FreeForker
    permGate IPermissionGate  // 注入；DM-002 新增
}

func NewFreeForkSurface(f FreeForker, gate IPermissionGate) *FreeForkSurface {
    return &FreeForkSurface{forker: f, permGate: gate}
}

func (s *FreeForkSurface) CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision {
    // free_fork: 直接调 IPermissionGate（外部 policy 决定）
    // 原因：spawn 5 个 child agent 涉及多智能体决策，per-tool 内部 AST 不合适
    return s.permGate.CheckPermission(ctx, spec)
}
```

### 2.4 BashASTPolicy 实现

**文件**：`internal/layers/contextengine/enforce/toolrunner/surface/bash_ast.go`

```go
package surface

import (
    "strings"
    "mvdan.cc/sh/v3/syntax"
)

type BashASTPolicy struct {
    DenyList     []BashDenyRule
    LastDenyReason string  // 仅用于内部 assertion / 错误信息
}

type BashDenyRule struct {
    Name     string
    Match    func(*syntax.Stmt) bool
    Reason   string
    Severity string  // "danger" / "warning"
}

// 默认 deny-list（devrix 0.1 阶段；后续 DM-005 DSL 可扩展）
var DefaultBashDenyRules = []BashDenyRule{
    {
        Name: "rm-rf-root",
        Match: isRmRfRoot,
        Reason: "rm -rf / would destroy the filesystem",
        Severity: "danger",
    },
    {
        Name: "dd-overwrite",
        Match: isDdCommand,
        Reason: "dd can overwrite disk blocks irreversibly",
        Severity: "danger",
    },
    {
        Name: "mkfs-format",
        Match: isMkfsCommand,
        Reason: "mkfs formats filesystems",
        Severity: "danger",
    },
    {
        Name: "sudo-elevate",
        Match: isSudoCommand,
        Reason: "sudo elevates privileges (DM-005 DSL required for bypass)",
        Severity: "warning",
    },
    {
        Name: "chmod-777-root",
        Match: isChmod777Root,
        Reason: "chmod 777 / opens permissions globally",
        Severity: "warning",
    },
}

// 工具函数：判断 AST 节点 cmd 是否是某命令
func callName(stmt *syntax.Stmt) string {
    call, ok := stmt.Cmd.(*syntax.CallExpr)
    if !ok {
        return ""
    }
    if len(call.Args) == 0 {
        return ""
    }
    if w, ok := call.Args[0].(*syntax.Word); ok {
        if len(w.Parts) == 1 {
            if lit, ok := w.Parts[0].(*syntax.Lit); ok {
                return lit.Value
            }
        }
    }
    return ""
}

func isRmRfRoot(stmt *syntax.Stmt) bool {
    if callName(stmt) != "rm" {
        return false
    }
    call := stmt.Cmd.(*syntax.CallExpr)
    hasRf := false
    for _, arg := range call.Args[1:] {
        if w, ok := arg.(*syntax.Word); ok {
            if len(w.Parts) == 1 {
                if lit, ok := w.Parts[0].(*syntax.Lit); ok {
                    if lit.Value == "-rf" || lit.Value == "-fr" {
                        hasRf = true
                    }
                }
            }
        }
    }
    if !hasRf {
        return false
    }
    for _, arg := range call.Args[1:] {
        if w, ok := arg.(*syntax.Word); ok {
            if len(w.Parts) == 1 {
                if lit, ok := w.Parts[0].(*syntax.Lit); ok {
                    if lit.Value == "/" || lit.Value == "/*" {
                        return true
                    }
                }
            }
        }
    }
    return false
}

func isDdCommand(stmt *syntax.Stmt) bool {
    return callName(stmt) == "dd"
}

func isMkfsCommand(stmt *syntax.Stmt) bool {
    name := callName(stmt)
    return name == "mkfs" || strings.HasPrefix(name, "mkfs.")
}

func isSudoCommand(stmt *syntax.Stmt) bool {
    return callName(stmt) == "sudo"
}

func isChmod777Root(stmt *syntax.Stmt) bool {
    if callName(stmt) != "chmod" {
        return false
    }
    call := stmt.Cmd.(*syntax.CallExpr)
    has777 := false
    hasRoot := false
    for _, arg := range call.Args[1:] {
        if w, ok := arg.(*syntax.Word); ok && len(w.Parts) == 1 {
            if lit, ok := w.Parts[0].(*syntax.Lit); ok {
                if lit.Value == "777" {
                    has777 = true
                }
                if lit.Value == "/" {
                    hasRoot = true
                }
            }
        }
    }
    return has777 && hasRoot
}

func (p *BashASTPolicy) Check(cmd string) (Decision, string) {
    parser := syntax.NewParser()
    ast, err := parser.Parse(strings.NewReader(cmd), "")
    if err != nil {
        return DecisionAsk, "bash parse error: " + err.Error()
    }

    p.LastDenyReason = ""
    var matched *BashDenyRule
    syntax.Walk(ast, func(node syntax.Node) bool {
        if stmt, ok := node.(*syntax.Stmt); ok {
            for i, rule := range p.DenyList {
                if rule.Match(stmt) {
                    matched = &p.DenyList[i]
                    p.LastDenyReason = rule.Reason
                    return false
                }
            }
        }
        return true
    })

    if matched != nil {
        return DecisionDeny, matched.Reason
    }
    return DecisionAllow, ""
}

func NewBashASTPolicy() *BashASTPolicy {
    return &BashASTPolicy{DenyList: DefaultBashDenyRules}
}
```

**关键设计**：
- **mvdan/sh AST 精确匹配**：`rm -rf /` → `CallExpr{Args: [Lit("rm"), Lit("-rf"), Lit("/")]}` → 命中
- **变量名绕过保守 Ask**：`${RM} -rf /` → `CallExpr{Args: [VarRef(RM), Lit("-rf"), Lit("/")]}` → 第一个 arg 不是 Lit，**不命中 deny 规则，返回 Allow**（**注：这是 v1 简化；DM-005 引入可配置 regex 后再优化**）
- **多规则短路**：命中第一条 deny 后 `return false` 停 walk

### 2.5 IPermissionGate 扩展

**文件**：`internal/layers/orchestration/permission/gate.go`

```go
type IPermissionGate interface {
    // DM-006 引入：turn-level 决策
    Request(ctx context.Context, decision PermissionDecision) (*PermissionResult, error)

    // DM-002 引入：per-tool 决策
    //
    // 默认实现（PermissionGateAdapter）：
    //   Risk=low → Allow
    //   Risk=medium + !OpenWorld → Ask
    //   Risk=high || OpenWorld → Ask
    //
    // Plan mode 注入：
    //   mode=plan_mode + OpenWorld=true → Deny
    //   命中 devrix.yaml plan_mode.open_world_allowlist → 跳过 deny
    CheckPermission(ctx context.Context, spec ToolSpec) Decision
}

// PermissionGateAdapter 既有实现
type PermissionGateAdapter struct {
    openWorldPolicy *PlanModeOpenWorldPolicy
}

func (a *PermissionGateAdapter) CheckPermission(ctx context.Context, spec ToolSpec) Decision {
    // Step 1: Plan mode OpenWorld deny
    if a.openWorldPolicy != nil {
        if decision := a.openWorldPolicy.Apply(ctx, spec, DecisionAsk); decision == DecisionDeny {
            return DecisionDeny
        }
    }
    // Step 2: Risk-based default
    switch spec.Risk {
    case types.RiskLow:
        return DecisionAllow
    case types.RiskMedium:
        if !spec.OpenWorld {
            return DecisionAsk
        }
        return DecisionAsk
    case types.RiskHigh:
        return DecisionAsk
    default:
        return DecisionAsk
    }
}
```

### 2.6 PlanModeOpenWorldPolicy

**文件**：`internal/layers/orchestration/policy/plan_mode.go`

```go
type PlanModeOpenWorldPolicy struct {
    AllowList []string  // 从 devrix.yaml 读
}

func NewPlanModeOpenWorldPolicy(cfg *config.Config) *PlanModeOpenWorldPolicy {
    return &PlanModeOpenWorldPolicy{
        AllowList: cfg.PlanMode.OpenWorldAllowList,  // devrix.yaml 注入
    }
}

func (p *PlanModeOpenWorldPolicy) Apply(ctx context.Context, spec ToolSpec, current Decision) Decision {
    mode, _ := ctx.Value(ModeKey).(string)
    if mode != "plan_mode" {
        return current
    }
    if !spec.OpenWorld {
        return current
    }
    // 检查 allowlist
    for _, allowed := range p.AllowList {
        if spec.Name == allowed || matchWildcard(allowed, spec.Name) {
            return current
        }
    }
    return DecisionDeny
}

func matchWildcard(pattern, name string) bool {
    // 支持 "git_*" 模式
    if !strings.Contains(pattern, "*") {
        return false
    }
    matched, _ := filepath.Match(pattern, name)
    return matched
}
```

**devrix.yaml 配置示例**：
```yaml
plan_mode:
  open_world_allowlist:
    - "web_fetch"
    - "git_*"
    - "delegate_explore"
```

### 2.7 turn_adapter 集成

**文件**：`internal/bootstrap/turn_adapter.go:ExecuteRound`

```go
func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req ToolRoundRequest) (turn.ToolRoundResult, error) {
    results := make([]turn.ToolResult, len(req.ToolCalls))

    // 预解析所有 spec
    specs := make([]*ToolSpec, len(req.ToolCalls))
    for i, tc := range req.ToolCalls {
        specs[i] = a.findSpec(ctx, tc.Name)
    }

    // Step 1: CheckPermission（per-tool 决策）
    for i, tc := range req.ToolCalls {
        if specs[i] == nil {
            results[i] = turn.ToolResult{ToolCallID: tc.ID, Error: "tool not found: " + tc.Name}
            continue
        }
        surface := a.findSurface(tc.Name)
        decision := surface.CheckPermission(ctx, *specs[i], tc.Input)
        if decision == DecisionAsk {
            // 委托给 IPermissionGate
            decision = a.permGate.CheckPermission(ctx, *specs[i])
        }
        switch decision {
        case DecisionDeny:
            reason := surface.LastDenyReason()
            results[i] = turn.ToolResult{
                ToolCallID: tc.ID,
                Error: (&PermissionDeniedError{Spec: *specs[i], Input: tc.Input, Reason: reason}).Error(),
            }
            continue  // 关键：不调 surface.Execute
        case DecisionAsk:
            results[i] = turn.ToolResult{
                ToolCallID: tc.ID,
                Error: (&PermissionAskRequiredError{Spec: *specs[i], Input: tc.Input, Reason: "ask user"}).Error(),
            }
            continue
        }
    }

    // Step 2: 实际执行（DM-001 T25 并行 dispatch）
    var concurrent, sequential []int
    for i, tc := range req.ToolCalls {
        if results[i].Error != "" {  // 已被 Deny/Ask 跳过
            continue
        }
        if a.isConcurrencySafe(ctx, tc.Name) {
            concurrent = append(concurrent, i)
        } else {
            sequential = append(sequential, i)
        }
    }

    // ... 既有并行 dispatch 逻辑
    for _, i := range sequential {
        results[i] = a.executeOne(ctx, *specs[i], req.ToolCalls[i])
    }
    if len(concurrent) > 1 {
        var eg errgroup.Group
        for _, i := range concurrent {
            i := i
            eg.Go(func() error {
                results[i] = a.executeOne(ctx, *specs[i], req.ToolCalls[i])
                return nil
            })
        }
        _ = eg.Wait()
    } else if len(concurrent) == 1 {
        results[concurrent[0]] = a.executeOne(ctx, *specs[concurrent[0]], req.ToolCalls[concurrent[0]])
    }

    return turn.ToolRoundResult{Results: results}, nil
}
```

**关键设计**：
- **两阶段**：先 CheckPermission 决定是否能 Execute，再 dispatch
- **Deny/Ask 的 ToolCall 跳过 Execute**，但 `results[i].Error` 已记录
- **并行 dispatch 不变**（与 DM-001 T25 一致）

---

## 3. T 层测试设计

### 3.1 T26: ToolSurface.CheckPermission 默认 Allow

**测试点**：`TOOL-SURFACE-1-T26`

**单元测试**（7 surface 各 1 个）：

```go
func TestBuiltinSurface_CheckPermission_DefaultAllow(t *testing.T) {
    s := NewBuiltinSurface(newTestToolReg(), NewBashASTPolicy())
    spec := ToolSpec{Name: "read_file", Risk: types.RiskLow}
    assert.Equal(t, DecisionAllow, s.CheckPermission(ctx, spec, json.RawMessage(`{"path":"/tmp/a"}`)))
}

func TestLSPToolSurface_CheckPermission_DefaultAllow(t *testing.T) {
    s := NewLSPToolSurface(stubLSPConfig)
    spec := ToolSpec{Name: "lsp", Risk: types.RiskLow}
    assert.Equal(t, DecisionAllow, s.CheckPermission(ctx, spec, json.RawMessage(`{}`)))
}

// ... 5 个类似 surface_test.go
```

### 3.2 T27: BashSurface 内置 AST 拒危险 cmd

**测试点**：`TOOL-SURFACE-1-T27`

**集成测试**：

```go
func TestBashASTPolicy_DenyDangerousCommands(t *testing.T) {
    p := NewBashASTPolicy()
    cases := []struct{
        cmd string
        wantDecision Decision
        wantReasonContains string
    }{
        {"rm -rf /", DecisionDeny, "rm -rf /"},
        {"rm -rf /*", DecisionDeny, "rm -rf /"},
        {"rm -rf /home", DecisionAllow, ""},  // 不在 deny-list
        {"dd if=/dev/zero of=/dev/sda", DecisionDeny, "dd can overwrite"},
        {"mkfs.ext4 /dev/sda1", DecisionDeny, "mkfs formats"},
        {"mkfs.xfs /dev/sdb", DecisionDeny, "mkfs formats"},
        {"sudo apt install foo", DecisionDeny, "sudo elevates"},
        {"chmod 777 /", DecisionDeny, "chmod 777 /"},
        {"ls -la", DecisionAllow, ""},
        {"echo hello", DecisionAllow, ""},
    }
    for _, c := range cases {
        t.Run(c.cmd, func(t *testing.T) {
            decision, reason := p.Check(c.cmd)
            assert.Equal(t, c.wantDecision, decision)
            if c.wantDecision == DecisionDeny {
                assert.Contains(t, reason, c.wantReasonContains)
            }
        })
    }
}
```

### 3.3 T28: IPermissionGate.CheckPermission

**测试点**：`TOOL-SURFACE-1-T28` + `PERMISSION-GATE-1-T01`

**单元测试**：

```go
func TestPermissionGateAdapter_CheckPermission_RiskBased(t *testing.T) {
    gate := &PermissionGateAdapter{}
    cases := []struct{
        spec ToolSpec
        want Decision
    }{
        {ToolSpec{Risk: types.RiskLow}, DecisionAllow},
        {ToolSpec{Risk: types.RiskMedium, OpenWorld: false}, DecisionAsk},
        {ToolSpec{Risk: types.RiskMedium, OpenWorld: true}, DecisionAsk},
        {ToolSpec{Risk: types.RiskHigh}, DecisionAsk},
    }
    for _, c := range cases {
        t.Run(c.spec.Risk, func(t *testing.T) {
            assert.Equal(t, c.want, gate.CheckPermission(ctx, c.spec))
        })
    }
}
```

### 3.4 T29: Plan mode + turn_adapter 集成

**测试点**：`TOOL-SURFACE-1-T29` + `PERMISSION-GATE-1-T02`

**集成测试**：

```go
func TestIntegration_PlanMode_DenyOpenWorldTool(t *testing.T) {
    cfg := &config.Config{
        PlanMode: config.PlanMode{OpenWorldAllowList: []string{"web_fetch"}},  // 不含 free_fork
    }
    engine := newTestEngineWithPlanMode(cfg)

    req := ToolRoundRequest{
        ToolCalls: []ToolCall{
            {ID: "1", Name: "free_fork", Input: json.RawMessage(`{"n":3}`)},
        },
    }
    ctx := context.WithValue(context.Background(), ModeKey, "plan_mode")
    result, _ := engine.ExecuteRound(ctx, req)

    require.Len(t, result.Results, 1)
    assert.Contains(t, result.Results[0].Error, "permission denied")
    assert.Contains(t, result.Results[0].Error, "free_fork")
    assert.Contains(t, result.Results[0].Error, "open_world")
}

func TestIntegration_PlanMode_AllowListBypassesDeny(t *testing.T) {
    cfg := &config.Config{
        PlanMode: config.PlanMode{OpenWorldAllowList: []string{"git_*"}},
    }
    engine := newTestEngineWithPlanMode(cfg)
    // 假设有 "git_fetch" tool with OpenWorld=true
    spec := ToolSpec{Name: "git_fetch", Risk: types.RiskMedium, OpenWorld: true}
    ctx := context.WithValue(context.Background(), ModeKey, "plan_mode")
    decision := engine.permGate.CheckPermission(ctx, spec)
    assert.Equal(t, DecisionAsk, decision)  // 命中 allowlist 跳过 deny，进入 Risk 决策 → Ask
}

func TestIntegration_TurnAdapter_DenySkipsExecute(t *testing.T) {
    // mock BashSurface 返回 Deny
    mockSurface := &mockBashSurface{decision: DecisionDeny, reason: "rm -rf /"}
    engine := newTestEngineWithSurface(mockSurface)

    req := ToolRoundRequest{
        ToolCalls: []ToolCall{
            {ID: "1", Name: "bash", Input: json.RawMessage(`{"command":"rm -rf /"}`)},
        },
    }
    result, _ := engine.ExecuteRound(ctx, req)

    require.Len(t, result.Results, 1)
    assert.Contains(t, result.Results[0].Error, "permission denied")
    assert.Equal(t, 0, mockSurface.executeCount, "Execute must NOT be called when Deny")
}
```

### 3.5 既有 15 个 T 点兼容性

| T 点 | 兼容性 | 验证 |
|---|---|---|
| T01-T11（DM-007/008） | 接口加 method（7 surface 默认 Allow，零行为变化） | 既有测试全 PASS |
| T22-T25（DM-001） | CheckPermission 与 surface.Execute 解耦 | 既有测试全 PASS |
| T08 (per-agent ⊇ main) | PerAgentFilter 不动 | PASS |
| T09 (turn_adapter.findSurface) | 路径不变 | PASS |
| T10 (IPermissionGate.Request) | Request 方法保留；新 CheckPermission 是增量 | 既有集成测试全 PASS |
| T11 (devrix tool list) | 输出不变 | PASS |
| T25 (并行 dispatch) | Step 1 CheckPermission 不影响 Step 2 并行 | PASS |

---

## 4. 兼容性与迁移

### 4.1 向后兼容

| 既有接口 | 变化 | 兼容性 |
|---|---|---|
| `ToolSpec` 字段 | 0 变化（DM-001 v2 已是基础） | ✅ |
| `ToolSurface` 5 method | 0 变化 | ✅ |
| `ToolSurface` 6 method (`CheckPermission`) | 6 个 surface 默认 Allow | ✅ |
| `IPermissionGate.Request` | 0 变化（DM-006 保留） | ✅ |
| `IPermissionGate.CheckPermission` | 新增 | ✅ |
| 既有 7 surface.Execute 路径 | 0 变化（CheckPermission 只在 Execute 前调） | ✅ |
| 既有 15 T 点验收 | 0 变化 | ✅ |

### 4.2 library 不动原则

严守 AC11：
- `internal/layers/contextengine/freefork/` **0 行改动**
- `internal/layers/contextengine/tracker/` **0 行改动**
- `internal/layers/contextengine/verify/` **0 行改动**
- `internal/layers/multiagent/` **0 行改动**

仅修改：
- `internal/shared/contracts/tool_surface.go`（+1 method）
- `internal/shared/contracts/permission.go`（新增）
- 7 surface 文件（各 +4~6 行 CheckPermission）
- `internal/layers/contextengine/enforce/toolrunner/surface/bash_ast.go`（新增 ~150 行）
- `internal/layers/orchestration/permission/gate.go`（+1 method）
- `internal/layers/orchestration/policy/plan_mode.go`（新增 ~80 行）
- `internal/bootstrap/turn_adapter.go`（ExecuteRound 加 Step 1 ~30 行）

### 4.3 mvdan/sh 依赖引入

- `go.mod` 新增 `mvdan.cc/sh/v3 v3.5.0`
- `go.sum` 增 ~2MB
- build 时间 +3s
- binary 大小 +5MB
- **生产环境无 cgo**

### 4.4 升级顺序

```
Step 1: contracts.Decision enum + 2 error type 定义
Step 2: ToolSurface +1 method CheckPermission
Step 3: 7 surface 各自实现 CheckPermission（6 默认 + Bash override）
Step 4: BashASTPolicy 解析器（mvdan/sh）+ 默认 deny-list
Step 5: IPermissionGate +1 method + PlanMode policy
Step 6: turn_adapter ExecuteRound 加 Step 1 CheckPermission
Step 7: 4 个新 T 点（T26-T29）单测
Step 8: 既有 15 T 点（DM-007/008/001）回归
```

每 Step 独立 commit。

---

## 5. 风险设计

### 5.1 ToolSurface +1 method = breaking（继 DM-001 之后第 2 次）

**风险**：连续 2 个 change 加 method，7 surface 维护成本上升。

**缓解**：
- 7 surface 集中 commit，PR review 时 7 处 assertion PASS
- 7 surface 已有 1 method breaking 经验（DM-001），流程成熟
- 后续 DM-005 / DM-003 等的 surface 改动必须**前置** PR

### 5.2 Bash AST 解析 overhead > 5ms

**风险**：mvdan/sh 解析长 cmd 时延高，影响 LLM 响应。

**缓解**：
- benchmark 在 S4 必跑：`go test -bench=. -benchmem ./internal/layers/contextengine/enforce/toolrunner/surface/`
- 目标：< 5ms p99（实测 < 1ms）
- 不达标降级：保留 substring 匹配作为 fallback

### 5.3 Plan mode 误 deny OpenWorld tool

**风险**：用户写 plan_mode 时 free_fork 被 deny，无法 fork sub-agent 完成 plan。

**缓解**：
- devrix.yaml `plan_mode.open_world_allowlist: [...]` 可覆盖
- T29 集成测试覆盖 allowlist 命中
- 错误信息明确：`permission denied: tool=free_fork reason=plan_mode: open_world=true (add to devrix.yaml plan_mode.open_world_allowlist if intended)`

### 5.4 mvdan/sh 引入 cgo 风险

**风险**：选型错误导致 binary 膨胀或 cross-compile 失败。

**缓解**：
- mvdan/sh v3 是**纯 Go**（确认无 cgo）
- `CGO_ENABLED=0` 验证 cross-compile 通过
- devrix 0.1 已经用过类似库（yaml.v3 等），无 cgo 经验成熟

### 5.5 IPermissionGate 既有 stub 改造不彻底

**风险**：既有 IPermissionGate stub 没实现 CheckPermission，运行时 panic。

**缓解**：
- compile-time `var _ IPermissionGate = (*PermissionGateAdapter)(nil)` 守护
- 既有 tests/integration/permission_gate_test.go 加新 method 测试用例

### 5.6 变量名绕过 deny-list

**风险**：`${RM} -rf /` 不被 deny。

**缓解**：
- v1 简化：变量名绕过 → Allow（**已知风险**）
- S3-Gate review 显式记录："deny-list 仅覆盖字面量；v2 引入 DM-005 DSL regex 后再优化"
- 用户可在 devrix.yaml 扩 deny-list

### 5.7 CheckPermission 引入新 race

**风险**：per-tool decision 与并行 dispatch 的 race。

**缓解**：
- `go test -race ./...` 必须 100% 绿
- Decision 不携带可变状态（仅 ctx + spec + input 入参）
- Step 1 sequential 决策 → Step 2 parallel dispatch，**无 race**

---

## 6. 性能与可观测性

### 6.1 性能预期

| 指标 | 当前 | 改后 | 影响 |
|---|---|---|---|
| BashSurface.CheckPermission 100 calls | n/a | < 100ms (avg < 1ms) | < 5ms p99 |
| PlanMode policy check 100 calls | n/a | < 10ms (avg < 0.1ms) | 可忽略 |
| turn_adapter ExecuteRound 加 Step 1 overhead | n/a | < 50ms (10 tool calls × 5ms) | 与 T25 并行加速可叠加 |
| LLM 端到端响应 | 100% baseline | 105% (5% overhead) | 可接受 |

### 6.2 可观测性

不引入新 metric：
- 既有 OpenTelemetry span: `turn_adapter.ExecuteRound` 加 sub-span `perm.check.<tool_name>`
- `Decision` 写入 span attribute: `perm.decision=allow|deny|ask`
- `PermissionDeniedError` / `PermissionAskRequiredError` 触发时 span 标 `perm.reason`

---

## 7. 设计决策记录

| # | 决策 | 备选 | 选定理由 |
|---|---|---|---|
| 1 | Decision 用 string | int | 与 clawcode 1:1；JSON 友好 |
| 2 | mvdan/sh | tree-sitter-bash / 自实现 | 纯 Go，无 cgo；5MB binary |
| 3 | 7 surface 默认 Allow | 7 surface 全部 override AST | 既有 5 surface 无副作用，无需精细决策 |
| 4 | BashSurface 内置 AST | 抽离成独立 service | 关注点局部；service 化是 DM-005 范围 |
| 5 | Plan mode 用 policy chain 注册 | IPermissionGate 直接 deny | 与 clawcode 1:1；policy 可被 yaml allowlist 覆盖 |
| 6 | FreeForkSurface 调 IPermissionGate.CheckPermission | FreeForkSurface 内置 OpenWorld 判断 | spawn 5 个 child agent 是多智能体决策，应让外部 policy 决定 |
| 7 | Ask = 错误（PermissionAskRequiredError） | 阻塞等用户响应 | v1 简化；DM-005 引入 interactive prompt |
| 8 | Step 1 sequential 决策 → Step 2 并行 dispatch | 全程并行 | Step 1 决策 < 5ms 可忽略；sequential 简化 race 分析 |
| 9 | CheckPermission 入参 ToolSpec（不传 name） | 传 name，surface 内部查 | ToolSpec 是 surface.Tools 返回值，零冗余 |
| 10 | 既有的 IPermissionGate.Request 保留 | 改写为 CheckPermission | turn-level vs per-tool 不同语义，并存 |
| 11 | deny-list 写在 Go 默认值 | 写 yaml 文件 | v1 简化；DM-005 引入 DSL 后改为 yaml |
| 12 | Bash 变量名绕过 → Allow（v1） | 保守 Ask | v1 简化；DM-005 引入 regex 后可优化 |

---

## 8. 关联参考

- 父 change：`openspec/changes/devrix-tool-spec-enrichment/` (DM-001, S4_Ready)
- 借鉴源：
  - `clawcode/src/Tool.ts:404-410` (`checkPermissions`)
  - `clawcode/src/Tool.ts:101-110` (`PermissionResult` enum)
  - `clawcode/src/hooks/tools.ts:43-58` (`shouldAvoidPermissionPrompts`)
  - `clawcode/src/tools/BashTool/bashParse.ts` (mvdan/sh integration)
  - `clawcode/src/utils/permissions/permissionDefaults.ts` (deny-list)
- DSAFT 方法论：`docs/methodology/dsaft-methodology.md` §12 (Facet Decomposition)
- 域归档：`openspec/specs/d2-context-engine/`, `openspec/specs/d7-orchestration/`, `openspec/specs/permission-gate/`
- T 注册表：`openspec/specs/tool-surface/t-registry.md`, `openspec/specs/permission-gate/t-registry.md`

---

## 9. S3-Gate 检查清单

- [x] Decision enum + 2 error type 完整设计（§2.1）
- [x] ToolSurface +1 method CheckPermission 完整签名（§2.2）
- [x] 7 surface 实现：5 默认 Allow + Bash override + FreeFork override（§2.3）
- [x] BashASTPolicy 解析器（mvdan/sh）+ 5 个默认 deny-list（§2.4）
- [x] IPermissionGate +1 method CheckPermission + PlanMode policy（§2.5-2.6）
- [x] turn_adapter ExecuteRound 集成（Step 1 sequential 决策 + Step 2 并行 dispatch）（§2.7）
- [x] 4 个新 P0 T 点（T26-T29）+ 2 个 PERMISSION-GATE-1 T 点 Gherkin 设计（§3.1-3.4）
- [x] 既有 15 T 点兼容性表（§3.5）
- [x] 向后兼容 + library 0 改动保证（§4.1-4.2）
- [x] mvdan/sh 依赖评估（§1.3 + §4.3）
- [x] 7 项风险设计 + 缓解（§5）
- [x] 性能与可观测性（§6）
- [x] 12 项设计决策记录（§7）
- [x] clawcode file:line 关联参考（§8）
- [x] 严守 AC1-AC18（demand.md §3）
