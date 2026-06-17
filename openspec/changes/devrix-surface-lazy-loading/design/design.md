# S3 Design: devrix-surface-lazy-loading

**Change ID:** devrix-surface-lazy-loading
**Demand ID:** DM-20260618-003
**Status:** S3_Designed

---

## 1. 架构总览

```
                 BuildSurfaces (D7 bootstrap)
                          │
                          ▼
                  ToolSpec{... + DeferLoading bool}
                          │
              ┌───────────┴────────────┐
              │                        │
   surface.Tools(ctx, workDir, "")   orthogonal_flags
   填 DeferLoading=true/false         静态 truth table
              │
              ▼
         ToolSurface list (7 + ToolSearchSurface)
              │
              ▼
         turn_adapter.Prepare
              │
              ├─ 收集所有 ToolSpec
              ├─ filter out DeferLoading=true (除 tool_search 自身)
              └─ 下发到 LLM (5-10 tools instead of 80+)
              │
              ▼
         LLM: tool_search(query=...) ?
              │
              └─ 是 → ToolSearchSurface.Execute → 返回 matching ToolSpec list
                    │
                    └─ LLM 下一轮用完整 schema 调 defer 工具
```

## 2. 关键类型 / 接口变更

### 2.1 `contracts.ToolSpec` 加字段

```go
type ToolSpec struct {
    Name string
    Description string
    Parameters string
    Risk types.RiskLevel
    ReadOnly bool
    Destructive bool
    OpenWorld bool
    ConcurrencySafe bool
    // NEW: DeferLoading 标记是否从默认 prompt 中省略 schema,
    // 等待 LLM 通过 tool_search 主动检索.
    DeferLoading bool
}
```

### 2.2 `contracts.ToolFilter` 加方法

```go
type ToolFilter interface {
    Name() string
    Apply(ctx, spec, current) Decision  // 既有
    ShouldDefer(ctx, spec) bool         // NEW
}
```

默认实现 `AcceptAllFilter` 的 ShouldDefer 返回 false。
`PlanModeOpenWorldPolicy` 的 ShouldDefer 返回 `(mode == plan_mode && spec.OpenWorld && !in_allowlist)`。

### 2.3 `surface.OrthogonalFlagFor` 扩展

```go
// 在 orthogonal_flags.go 加 helper
func ShouldDeferByDefault(name string) bool {
    if strings.HasPrefix(name, "delegate_") { return true }
    if strings.HasSuffix(name, "_background") { return true }
    return false
}
```

## 3. Surface 实现

### 3.1 新增 `ToolSearchSurface`

```go
type ToolSearchSurface struct {
    allTools []contracts.ToolSpec  // 全量 (含 defer)
}

func (s *ToolSearchSurface) Name() string { return "tool_search" }
// Tools() 返回 [ToolSpec{Name:"tool_search", DeferLoading: false}]
// Risk: LOW, ConcurrencySafe: true, OpenWorld: false

func (s *ToolSearchSurface) Execute(ctx, name, input, _) (*ToolResult, error) {
    var req struct {
        Query string `json:"query"`
        Category string `json:"category,omitempty"`
    }
    json.Unmarshal([]byte(input), &req)
    matches := s.search(req.Query, req.Category)
    return &ToolResult{Output: marshal(matches)}, nil
}

func (s *ToolSearchSurface) search(query, category string) []ToolSpec {
    var out []ToolSpec
    for _, sp := range s.allTools {
        if sp.DeferLoading == false { continue }
        if category != "" && !strings.HasPrefix(sp.Name, category) { continue }
        if !strings.Contains(strings.ToLower(sp.Name), strings.ToLower(query)) { continue }
        out = append(out, sp)
        if len(out) >= 5 { break }
    }
    return out
}
```

### 3.2 `BuiltinSurface.Tools()` 填 DeferLoading

```go
for _, name := range builtinToolNames {
    readOnly, destructive, openWorld, concSafe := OrthogonalFlagFor(name)
    out = append(out, ToolSpec{
        Name: name, ...
        DeferLoading: ShouldDeferByDefault(name),  // NEW
    })
}
```

## 4. Bootstrap 改动

### 4.1 `BuildSurfaces` 加 ToolSearchSurface

```go
allSpecs := collectAllToolSpecs(surfaces)  // 全量 (含 DeferLoading=true)
tss := surface.NewToolSearchSurface(allSpecs)
out = append(out, tss)
sort.Slice(out, ...)  // 既有 sort by name
```

### 4.2 `TurnAdapter.Prepare` 过滤

```go
for _, sp := range allSpecs {
    if sp.DeferLoading && sp.Name != "tool_search" {
        // 过滤, 但 tool_search 必须下发 (它自身不被 defer)
        continue
    }
    // Apply ToolFilter.ShouldDefer
    if filter != nil && filter.ShouldDefer(ctx, sp) {
        continue
    }
    schemas = append(schemas, sp)
}
```

## 5. 测试 (T26-T30)

### T26: ToolSpec.DeferLoading 字段
```go
func TestToolSpec_DeferLoading_StaticCandidates(t *testing.T) {
    cases := []struct{ name string; want bool }{
        {"delegate_explore", true},
        {"delegate_status", true},
        {"delegate_research", true},
        {"task_output_background", true},
        {"bash", false},
        {"read_file", false},
        {"tool_search", false},  // 必须 non-defer
    }
    for _, c := range cases {
        if got := ShouldDeferByDefault(c.name); got != c.want {
            t.Errorf("%s defer = %v, want %v", c.name, got, c.want)
        }
    }
}
```

### T27: PlanModeOpenWorldPolicy.ShouldDefer
```go
func TestPlanModeOpenWorldPolicy_ShouldDefer(t *testing.T) {
    p := NewPlanModeOpenWorldPolicy([]string{"web_fetch"})
    // plan_mode + openworld + not in allowlist → defer
    ctx := context.WithValue(context.Background(), ModeKey{}, "plan_mode")
    spec := contracts.ToolSpec{Name: "web_search", OpenWorld: true}
    if !p.ShouldDefer(ctx, spec) { t.Error("want defer") }
    
    spec.AllowList match → not defer
    mode != plan_mode → not defer
    !OpenWorld → not defer
}
```

### T28: ToolSearchSurface.search
```go
func TestToolSearchSurface_Search(t *testing.T) {
    s := NewToolSearchSurface([]ToolSpec{
        {Name: "delegate_research", DeferLoading: true},
        {Name: "delegate_explore", DeferLoading: true},
        {Name: "bash", DeferLoading: false},  // 不该被搜出来
    })
    res := s.search("delegate", "")
    if len(res) != 2 { t.Fatalf("got %d", len(res)) }
}
```

### T29: turn_adapter.Prepare 过滤
```go
func TestPrepare_FilterDeferLoading(t *testing.T) {
    a := &contextEngineAdapter{surfaces: [...]}
    res, _ := a.Prepare(ctx, ...)
    for _, ts := range res.Tools {
        if ts.Name != "tool_search" && ts.DeferLoading {
            t.Errorf("defer tool %q leaked to LLM prompt", ts.Name)
        }
    }
}
```

### T30: zodgen
```go
type Z struct {
    Name string `json:"name" jsonschema:"required,description=user name"`
    Age  int    `json:"age,omitempty"`
}
schema := zodgen.Schema(Z{})
// expect: {"type":"object","properties":{"name":...,"age":...},"required":["name"]}
```

## 6. 风险 / 限制

- L1: tool_search 必须 non-defer (否则死锁)
- L2: tool_search 返回的 schema 必须是完整的 ToolSpec (LLM 调得动)
- L3: zodgen 只做 subset (type / properties / required / enum / description), 不做 $ref / oneOf / anyOf
- L4: anthropic package 仅 stub (v1.1)