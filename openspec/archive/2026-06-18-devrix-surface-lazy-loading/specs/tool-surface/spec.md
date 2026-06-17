# Spec: devrix-surface-lazy-loading (TOOL-SURFACE-1 Lazy Loading)

**Change ID:** devrix-surface-lazy-loading
**Demand ID:** DM-20260618-003
**Domain:** tool-surface (横切契约域)
**Status:** S3_Designed

---

## 1. Capability

### TOOL-SURFACE-1-A01-F08: ToolSpec.DeferLoading + ToolFilter.ShouldDefer

#### Contract

`contracts.ToolSpec.DeferLoading bool` —— BuildSurfaces / orthogonal_flags 静态标记
+ `contracts.ToolFilter.ShouldDefer(ctx, spec) bool` —— runtime 动态标记
+ 两者任一为 true 则 turn_adapter.Prepare 过滤掉 (tool_search 除外)

#### Surface 实现

| Surface | Tool name | DeferLoading | Notes |
|---------|-----------|--------------|-------|
| builtin | `delegate_explore` / `delegate_status` / `delegate_status_all` / `delegate_plan` / `delegate_research` | true | static prefix match `delegate_` |
| builtin | `task_output_background` | true | static suffix match `_background` |
| builtin | `bash` / `read` / `write` / `edit` / `glob` / `grep` / ... | false | always in-pack |
| new | `tool_search` | **false** (强制) | 否则死锁 |

#### Filter 实现

| Filter | ShouldDefer 条件 |
|--------|------------------|
| AcceptAllFilter | false |
| PlanModeOpenWorldPolicy | `(mode == plan_mode && spec.OpenWorld && !in_allowlist)` |

### TOOL-SURFACE-1-A02: ToolSearchSurface (新 surface)

#### Contract

```
ToolSearchSurface:
  Name() == "tool_search"
  Tools() == [ToolSpec{Name:"tool_search", DeferLoading: false, ...}]
  Execute(query, category) → 匹配的 ToolSpec list (top-5)
```

#### 搜索算法

exact name match > glob match > substring match > empty

#### 调用流程

```
LLM turn 1:
  Prepare 下发 6 个非 defer 的 tools (bash / read / write / glob / grep / tool_search)
  LLM: tool_search(query="delegate to research agent")
    → 返回 [{Name:"delegate_research", Description:"...", Parameters:..., DeferLoading:true}]
  LLM turn 2:
    delegate_research(...)  ← 现在调得动
```

### zodgen (新建 toolrunner/zodgen)

```
Go struct (with json + jsonschema tags) → JSON Schema subset
支持: type, properties, required, enum, description
不支持: $ref, oneOf, anyOf, allOf (v1.1)
```

### anthropic package (stub)

```
internal/layers/anthropic/anthropic.go
  列出 v1.1 待实现 items (Client / ListTools / ToolUse)
  不实现 (compile-only stub)
```

## 2. 测试点 (T26-T30)

- **T26** (TOOL-SURFACE-1-A01-F08): ToolSpec.DeferLoading + ShouldDeferByDefault (6 个 hardcoded candidates)
- **T27** (TOOL-SURFACE-1-A01-F08): PlanModeOpenWorldPolicy.ShouldDefer runtime defer
- **T28** (TOOL-SURFACE-1-A02): ToolSearchSurface.search (exact / glob / substring)
- **T29** (TOOL-SURFACE-1-A02): turn_adapter.Prepare 过滤 DeferLoading=true 的 tools (tool_search 必须下发)
- **T30** (TOOL-SURFACE-1-A02): zodgen Schema() (Go struct → JSON Schema subset)

**T registration**: TOOL-SURFACE-1-T26, T27, T28, T29, T30 to be added to `openspec/specs/tool-surface/t-registry.md`