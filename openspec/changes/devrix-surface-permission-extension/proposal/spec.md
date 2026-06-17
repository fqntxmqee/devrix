# S2 提案：Per-tool CheckPermission hook + IPermissionGate.ToolPolicy

**Change ID:** devrix-surface-permission-extension
**DM ID:** DM-20260618-002
**状态:** S2_Clarified
**提案人:** AI Assistant
**日期:** 2026-06-18

---

## 1. 问题陈述

devrix 当前权限决策分两层且都有局限：

### 1.1 turn-level 决策太粗

`IPermissionGate.Request(ctx, decision)` 在 turn 开始时一次性决定整个 turn 的所有 tool 风险等级（DM-006 引入）。**实测问题**：

```go
// 既有：turn 启动时一次性决策
permDecision := ipg.Request(ctx, PermissionDecision{
    Mode: "plan_mode",
    Tools: ["read_file", "write_file", "bash", "web_fetch", "free_fork"],
})
// 一旦 "ask" 整个 turn 都 ask；ask 一次后整个 turn 不再 ask
```

**没有"在单个 tool dispatch 前"再决策一次**的能力。bash tool 在 plan_mode 下应被严格 policy 拒（如 `rm -rf /`），但当前无法针对 bash 内容做精细 AST 级别判断。

### 1.2 bash 没有 AST 级别 policy

`BuiltinSurface.bash.Execute` 当前仅校验命令是否在 deny-list 字符串里（粗糙 substring 匹配）。**无法识别**：

```bash
rm -rf /              # 直接命中
rm -rf /*             # 通配符绕过
${RM} -rf /           # 变量名绕过
/bin/rm -rf /         # 绝对路径绕过
```

clawcode 用 `mvdan/sh` Go 库做 bash AST 解析（bashParse tool.ts），可对 AST 节点做精确 policy：
- 把 cmd 解析成 AST
- 找 `rm` / `dd` / `mkfs` 等 command node
- 检查 flag 和 arg（`-rf`、target = `/`）

### 1.3 Plan mode 不会用 OpenWorld 字段收紧

DM-001 已经在 ToolSpec 加 `OpenWorld bool` 字段，但**当前没有任何 policy 消费这个字段**。plan_mode 仍按 Risk 阈值粗筛。

clawcode 的 `shouldAvoidPermissionPrompts`（hooks/tools.ts:43-58）显式用 `isOpenWorld()` 在 plan mode 拒绝网络/副作用 tool。

---

## 2. 解决方案

### 2.1 方案 A（推荐）：CheckPermission hook + BashAST + Plan mode policy

#### 2.1.1 ToolSurface interface +1 method

```go
// internal/shared/contracts/tool_surface.go
type Decision string

const (
    DecisionAllow Decision = "allow"  // 工具可执行
    DecisionDeny  Decision = "deny"   // 工具拒绝执行（PermissionDeniedError）
    DecisionAsk   Decision = "ask"    // 需要用户确认（PermissionAskRequiredError）
)

// ToolSurface 在 DM-001 v2 5 方法基础上加 CheckPermission。
//
// 调用时机：turn_adapter.ExecuteRound 在 surface.Execute 之前调
// 调用频率：每个 ToolCall 调 1 次
// 默认实现：所有 7 surface 默认返回 DecisionAllow
//
// 性能预算：< 5ms p99（in-process）
type ToolSurface interface {
    Name() string
    Tools(ctx context.Context, workDir, sessionID string) []ToolSpec
    RiskLevel(name string) types.RiskLevel
    Execute(ctx context.Context, name string, input json.RawMessage, workDir string) (*ToolResult, error)
    InterruptBehavior(name string) InterruptMode

    // CheckPermission 在 tool dispatch 前调一次，决定是否允许执行。
    //
    // 返回 Decision：
    //   Allow → surface.Execute 被调
    //   Deny  → surface.Execute 不被调，返回 PermissionDeniedError
    //   Ask   → surface.Execute 不被调，返回 PermissionAskRequiredError（含 spec + input 元信息）
    //
    // 默认实现：所有 7 surface 返回 Allow
    // 定制实现：BashSurface 内部 AST 解析 + deny-list
    CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision
}
```

**6 method interface**：Go 没有 default method，**7 surface 必须各自实现**。但每 surface 默认实现 = `return DecisionAllow`，**1 行代码**。

#### 2.1.2 Decision enum + 错误类型

```go
// internal/shared/contracts/permission.go
type PermissionDeniedError struct {
    Spec   ToolSpec
    Input  json.RawMessage
    Reason string  // "plan_mode: open_world=true" / "bash: rm -rf / detected"
}

func (e *PermissionDeniedError) Error() string {
    return fmt.Sprintf("permission denied: tool=%s, reason=%s", e.Spec.Name, e.Reason)
}

type PermissionAskRequiredError struct {
    Spec   ToolSpec
    Input  json.RawMessage
    Reason string
}

func (e *PermissionAskRequiredError) Error() string {
    return fmt.Sprintf("permission ask required: tool=%s, reason=%s", e.Spec.Name, e.Reason)
}
```

#### 2.1.3 7 surface 默认实现

```go
// 6 个 surface 的默认实现
func (s *BuiltinSurface) CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision {
    return DecisionAllow
}
// 实际 BashSurface 会 override，调用 BashASTPolicy
// 实际 FreeForkSurface 会 override，调用 IPermissionGate.CheckPermission
// 其余 5 surface 默认 Allow
```

**关键设计**：
- 默认 Allow：既有行为零变化（向后兼容 DM-001/007/008）
- Override 时机：BashSurface 调 AST；FreeForkSurface 调 IPermissionGate
- 其余 5 surface 永远 Allow（grep / glob / lsp / verify / task_output 都无副作用）

#### 2.1.4 BashSurface 内置 AST 解析

**库选型**：mvdan/sh（纯 Go，无 cgo，5MB binary）

```go
// internal/layers/contextengine/enforce/toolrunner/surface/bash_ast.go
import "mvdan.cc/sh/v3/syntax"

type BashASTPolicy struct {
    DenyList []BashDenyRule
}

type BashDenyRule struct {
    Match   func(*syntax.Stmt) bool  // AST 节点匹配
    Reason  string                   // 错误信息
    Severity string                  // "danger" / "warning"
}

// 默认 deny-list（devrix 0.1 阶段）
var DefaultBashDenyRules = []BashDenyRule{
    {
        Match: isRmRfRoot,
        Reason: "rm -rf / would destroy the filesystem",
        Severity: "danger",
    },
    {
        Match: isDdCommand,
        Reason: "dd can overwrite disk blocks",
        Severity: "danger",
    },
    {
        Match: isMkfsCommand,
        Reason: "mkfs formats filesystems",
        Severity: "danger",
    },
    {
        Match: isSudoCommand,
        Reason: "sudo elevates privileges",
        Severity: "warning",
    },
    {
        Match: isChmod777Root,
        Reason: "chmod 777 / opens permissions",
        Severity: "warning",
    },
}

func isRmRfRoot(stmt *syntax.Stmt) bool {
    cmd := stmt.Cmd
    if call, ok := cmd.(*syntax.CallExpr); ok {
        if isWordEqual(call.Args[0], "rm") {
            hasRf := false
            for _, arg := range call.Args[1:] {
                if isWordEqual(arg, "-rf") || isWordEqual(arg, "-fr") {
                    hasRf = true
                }
            }
            if hasRf {
                for _, arg := range call.Args[1:] {
                    if isWordEqual(arg, "/") || isWordEqual(arg, "/*") {
                        return true
                    }
                }
            }
        }
    }
    return false
}

func (p *BashASTPolicy) Check(cmd string) Decision {
    parser := syntax.NewParser()
    ast, err := parser.Parse(strings.NewReader(cmd), "")
    if err != nil {
        return DecisionAsk  // 解析失败 → Ask（保守）
    }
    syntax.Walk(ast, func(node syntax.Node) bool {
        if stmt, ok := node.(*syntax.Stmt); ok {
            for _, rule := range p.DenyList {
                if rule.Match(stmt) {
                    p.lastReason = rule.Reason
                    return false  // 停 walk
                }
            }
        }
        return true
    })
    if p.lastReason != "" {
        return DecisionDeny
    }
    return DecisionAllow
}

func (s *BuiltinSurface) CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision {
    if spec.Name != "bash" {
        return DecisionAllow
    }
    var input_ struct {
        Command string `json:"command"`
    }
    if err := json.Unmarshal(input, &input_); err != nil {
        return DecisionAsk
    }
    decision := s.bashAST.Check(input_.Command)
    if decision == DecisionDeny {
        s.lastDenyReason = s.bashAST.lastReason
    }
    return decision
}
```

**关键保证**：
- mvdan/sh 是纯 Go，**无 cgo 依赖**，go build 5s 内
- 解析 `rm -rf /` → AST 节点 `CallExpr{Args: ["rm", "-rf", "/"]}` → 精确匹配
- 解析 `rm -rf /*` → AST 节点 `CallExpr{Args: ["rm", "-rf", "/*"]}` → 精确匹配
- 解析 `${RM} -rf /` → AST 节点 `CallExpr{Args: [VarRef{RM}, "-rf", "/"]}` → 第一个 arg 不是字面量"rm"，**保守 Ask**

#### 2.1.5 IPermissionGate 加 CheckPermission method

```go
// internal/layers/orchestration/permission/gate.go
type IPermissionGate interface {
    // DM-006 引入：turn-level 决策
    Request(ctx context.Context, decision PermissionDecision) (*PermissionResult, error)

    // DM-002 引入：per-tool 决策
    // 默认实现：读 spec.Risk + spec.OpenWorld + spec.Destructive
    //   Risk=low → Allow
    //   Risk=medium + !OpenWorld → Ask
    //   Risk=high || OpenWorld → Ask
    // Plan mode 注入：OpenWorld=true → Deny（可被 yaml allowlist 覆盖）
    CheckPermission(ctx context.Context, spec ToolSpec) Decision
}
```

**Plan mode policy 注册**（在 `internal/layers/orchestration/policy/plan_mode.go`）：

```go
// PlanModeOpenWorldPolicy 在 plan mode 时拒绝 OpenWorld=true 的 tool
type PlanModeOpenWorldPolicy struct {
    AllowList []string  // 从 devrix.yaml 读
}

func (p *PlanModeOpenWorldPolicy) Apply(ctx context.Context, spec ToolSpec, current Decision) Decision {
    mode := ctx.Value("mode").(string)  // 由 turn_adapter 注入
    if mode != "plan_mode" {
        return current
    }
    if !spec.OpenWorld {
        return current
    }
    // 检查 allowlist
    for _, allowed := range p.AllowList {
        if spec.Name == allowed {
            return current  // 命中 allowlist，不 deny
        }
    }
    return DecisionDeny
}
```

**与 clawcode 1:1 对齐**：`shouldAvoidPermissionPrompts`（hooks/tools.ts:43-58）。

#### 2.1.6 turn_adapter ExecuteRound 集成

```go
// internal/bootstrap/turn_adapter.go:ExecuteRound
func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req ToolRoundRequest) (turn.ToolRoundResult, error) {
    // ... 既有并行 dispatch 逻辑（DM-001）

    for _, i := range allIndices {
        tc := req.ToolCalls[i]
        spec := a.findSpec(ctx, tc.Name)
        if spec == nil {
            results[i] = turn.ToolResult{ToolCallID: tc.ID, Error: "tool not found: " + tc.Name}
            continue
        }

        // 1) ToolSurface.CheckPermission（DM-002 新增）
        surface := a.findSurface(tc.Name)
        decision := surface.CheckPermission(ctx, *spec, tc.Input)
        if decision == DecisionAsk {
            // 2) IPermissionGate.CheckPermission（外部决策）
            decision = a.permGate.CheckPermission(ctx, *spec)
        }
        switch decision {
        case DecisionDeny:
            results[i] = turn.ToolResult{ToolCallID: tc.ID, Error: &PermissionDeniedError{Spec: *spec, Reason: "..."}.Error()}
            continue  // 不调 surface.Execute
        case DecisionAsk:
            results[i] = turn.ToolResult{ToolCallID: tc.ID, Error: &PermissionAskRequiredError{Spec: *spec}.Error()}
            continue
        }
        // decision == Allow → 调 surface.Execute
        results[i] = a.executeOne(ctx, *spec, tc, surface)
    }
    return turn.ToolRoundResult{Results: results}, nil
}
```

**调用顺序**：
1. **ToolSurface.CheckPermission** —— 工具自己的精细判断（Bash AST）
2. **IPermissionGate.CheckPermission** —— 外部 policy（Plan mode OpenWorld deny）
3. **surface.Execute** —— 实际执行

**关键保证**：
- Deny/Ask → **不调 surface.Execute**（T29 集成测试守护）
- results 顺序保持（与 DM-001 T25 一致）
- errgroup 并行不变

### 2.2 方案 B（不推荐）：只加 CheckPermission method，无 AST

只解决 1.1 / 1.3，**不解决 1.2**。

- ✅ 改动量小（~200 行）
- ❌ bash 仍是 substring 匹配，危险命令可绕过
- ❌ 跟 clawcode 完整借鉴对比，少 1 块拼图

### 2.3 方案 C（备选）：用 enum 替代 Decision string

```go
type Decision int
const (
    DecisionAllow Decision = iota
    DecisionDeny
    DecisionAsk
)
```

- ✅ 性能稍好（int 比较）
- ❌ 序列化不友好（log/JSON 时要转字符串）
- ❌ 与 clawcode 1:1 不对齐（clawcode 是 string）

### 2.4 决策

**选择方案 A**。理由：
1. Decision string 与 clawcode 的 `PermissionResult` enum 1:1 对齐
2. Bash AST 是 P0 安全需求，不能用 substring 糊弄
3. Plan mode OpenWorld deny 与 DM-001 的 4 bool 字段联动最强

---

## 3. 实施计划

| 阶段 | 任务 | 估时 | 交付物 |
|---|---|---|---|
| 1 | S3 design.md 完整设计 | 1 h | design.md |
| 2 | Decision enum + 2 error type 定义在 contracts | 30 min | contracts/permission.go |
| 3 | ToolSurface +1 method CheckPermission | 15 min | contracts/tool_surface.go |
| 4 | 7 surface 各加 CheckPermission 默认实现（6 个 Allow + BashSurface 调 AST） | 2 h | 7 surface.go |
| 5 | BashAST 解析器（mvdan/sh）+ 默认 deny-list | 3 h | surface/bash_ast.go |
| 6 | IPermissionGate +1 method CheckPermission + PlanMode policy | 2 h | orchestration/permission/gate.go + policy/plan_mode.go |
| 7 | turn_adapter dispatch 前 CheckPermission 调用 | 1 h | bootstrap/turn_adapter.go |
| 8 | 全量回归 + 4 个新 P0 T 点（T26-T29 + 2 个 PERMISSION-GATE-1） | 2 h | tests/integration/permission_test.go |
| 9 | S5 验收 + S6 归档（PR + auto-merge） | 1 h | PR + verify-archive.sh |
| **总计** | | **~12.75 h (2 人日)** | |

### 3.1 执行顺序

1. Step 1-2（design + Decision enum）
2. Step 3-4（interface + 7 surface 默认实现）
3. Step 5（Bash AST，依赖 Step 3）
4. Step 6（IPermissionGate，依赖 Step 3）
5. Step 7（turn_adapter 集成，依赖 Step 4+6）
6. Step 8（测试）
7. Step 9（PR）

每个 Step 独立 commit。

---

## 4. 成功指标

| Metric | Baseline | Target | 测量 |
|---|---|---|---|
| 7 surface 实现 CheckPermission surface 数 | 0/7 | 7/7 | `git grep "func.*CheckPermission" internal/layers/contextengine/enforce/toolrunner/surface/` 命中 7 文件 |
| Bash AST 危险命令 deny 率 | 0% | 100% | T27: 10 个危险命令（rm -rf /, dd, mkfs, sudo, chmod 777 /, ...）100% Deny |
| Plan mode OpenWorld deny 触发率 | 0% | 100% | T29: 5 个 OpenWorld tool 在 plan mode 100% Deny |
| CheckPermission overhead p99 | n/a | < 5ms | benchmark `BenchmarkBuiltinSurface_CheckPermission` |
| turn_adapter Deny 时不调 Execute | 0/0 | 100% | T29: mock Deny → surface.Execute 调用计数 = 0 |
| 既有 15 个 P0 T 点（DM-007/008/001） | 15/15 PASS | 15/15 PASS | `go test -race ./...` |
| `go vet ./...` + `staticcheck` warning | 0 | 0 | CI |
| 单测覆盖率（新代码） | n/a | ≥ 80% | `go test -cover` |

---

## 5. 风险与缓解

| 风险 | 等级 | 缓解 |
|---|---|---|
| **ToolSurface +1 method = breaking（DM-001 之后第 2 次）** | H | 7 surface 集中 commit；compile-time `var _ contracts.ToolSurface = ...` 7 处 assertion 必须 PASS |
| **Bash AST 解析 overhead > 5ms** | M | mvdan/sh 是纯 Go；benchmark 在 S4 必跑；不达标则降级 substring |
| **Plan mode 误 deny OpenWorld tool** | M | devrix.yaml `plan_mode.open_world_allowlist: [...]` 可覆盖；T29 集成测试覆盖 allowlist 命中 |
| **mvdan/sh 引入新依赖** | L | 评估：纯 Go 5MB binary，5s build；与 clawcode 选型一致 |
| **IPermissionGate 既有 stub 改造不彻底** | M | compile-time `var _ IPermissionGate = ...` 守护；既有 tests 通过 |
| **deny-list 漏判** | M | 0 阶段仅"教科书"危险命令；DM-005 引入 DSL 后可加正则 |

---

## 6. Open Questions

| Q | 状态 | 决策 |
|---|---|---|
| Decision 用 string 还是 int？ | S3 决 | **string**（与 clawcode 1:1，JSON 友好） |
| mvdan/sh 还是 tree-sitter-bash？ | S3 决 | **mvdan/sh**（纯 Go，无 cgo） |
| 既有 IPermissionGate.Request 保留吗？ | S3 决 | **保留**（turn-level 决策；新 CheckPermission 是 per-tool 增量） |
| Plan mode OpenWorld deny 可被覆盖吗？ | S3 决 | **可**（devrix.yaml `plan_mode.open_world_allowlist`） |
| CheckPermission 在 surface 还是 IPermissionGate？ | S3 决 | **先 surface 后 IPermissionGate**（surface 内 AST 决策；外部 policy 兜底） |
| Ask 决策 = 错误还是阻塞？ | S3 决 | **错误**（PermissionAskRequiredError，简化 v1 实现） |
| 既有 11 个 P0 T 点是否要加 CheckPermission 断言？ | S3 决 | **否**（T26-T29 是新加的，专门测 CheckPermission） |
| 是否要在 S3 design.md 给出 Bash AST deny-list 完整配置？ | S3 决 | **是**（避免 S4 实施时分散） |
| CheckPermission 入参是 ToolSpec 还是 (name, input)？ | S3 决 | **ToolSpec**（与 surface.Tools 返回值一致，避免 surface 内部再查） |

---

## 7. Out of Scope

- **不修改** 13 个 diagnostic-tools-parity library 对外 API
- **不重写** `IPermissionGate.Request` 接口（DM-006 范围，保留向后兼容）
- **不实现** Per-tool policy DSL（DM-005 范围）
- **不实现** Permission audit log（DM-005 范围）
- **不实现** Zod-equivalent schema 验证（DM-003 范围）
- **不引入** ToolSearch / SurfaceSearch / Lazy loading（DM-003 范围）
- **不实现** MCP 集成
- **不重构** 任何 global / singleton
- **不修改** ToolSpec 4 bool 字段（DM-001 已是 v2）

---

## 8. 关联参考

- 上游 change：`openspec/archive/2026-06-17-devrix-tool-surface-contract/` (DM-007)
- 上游 change：`openspec/archive/2026-06-17-devrix-tool-surface-phase2-full/` (DM-008)
- 上游 change：`openspec/changes/devrix-tool-spec-enrichment/` (DM-001, S4_Ready)
- 借鉴源：`docs/reference/clawcode-tool-design-comparison.md` §8.2 P0-(2)
- clawcode 参考实现：
  - `clawcode/src/Tool.ts:404-410` (`checkPermissions`)
  - `clawcode/src/Tool.ts:101-110` (`PermissionResult` enum)
  - `clawcode/src/hooks/tools.ts:43-58` (`shouldAvoidPermissionPrompts` + plan mode OpenWorld)
  - `clawcode/src/tools/BashTool/bashParse.ts` (mvdan/sh integration)
- DSAFT 方法论：`docs/methodology/dsaft-methodology.md` §12 (Facet Decomposition)
- 域归档：`openspec/specs/d2-context-engine/`, `openspec/specs/d7-orchestration/`
- T 注册表：`openspec/specs/tool-surface/t-registry.md`, `openspec/specs/permission-gate/t-registry.md`
