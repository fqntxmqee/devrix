# S2 提案: devrix-surface-lazy-loading

**Change ID:** devrix-surface-lazy-loading
**Demand ID:** DM-20260618-003
**Status:** S7_Archived
**Priority:** P0
**Date:** 2026-06-18
**Merged PR:** [#70](https://github.com/fqntxmqee/devrix/pull/70)

---

## 1. 问题陈述

devrix 的 `turn_adapter.Prepare` 每次 LLM 调用下发 80+ tool schemas (7 surface × ~12 tools), 即使当前 turn 只需要 2-3 个。Anthropic Claude Sonnet 4.5 + Claude Code v2.x 都已引入 tool-search-tool / defer_loading 来应对该问题。

具体 3 个可观测影响：

1. **prompt cache miss**: 大 schema 列表每次序列化 hash 不同, 导致 Anthropic prompt cache 命中率 < 60%
2. **context budget**: 80+ JSON schemas 占 ~12K tokens 纯开销 (实测)
3. **tool selection drift**: 模型面对过多相似工具倾向选错 (delegate_status vs delegate_explore vs ...)

## 2. 解决方案

三层 lazy 机制, 由 orchestrator (turn_adapter) 控制时机:

### 2.1 Static defer (BuildSurfaces 阶段)

`BuildSurfaces` 给每个 ToolSpec 加 `DeferLoading bool`。hardcoded 候选:
- `delegate_*` (5 个: delegate_explore / delegate_status / delegate_status_all / delegate_plan / delegate_research)
- `*_background` (1 个: task_output_background)

### 2.2 Filter-based defer (runtime)

ToolFilter 加 `ShouldDefer(ctx, spec) bool` 方法。PlanModeOpenWorldPolicy 已经在 runtime 标记 deny; 增加一个 shouldDefer 通道, 让 plan_mode 把所有 open-world tools 标 defer (而非直接 deny), 等 LLM 用 tool_search 检索。

### 2.3 ToolSearch runtime discovery (LLM 调用)

新建 surface: `ToolSearchSurface` 提供 `tool_search` 工具, 接受 `query` + `category` 参数, 返回匹配的 ToolSpec 列表 (JSON Schema subset)。

调用流程:
```
LLM turn 1:
  Prepare 下发 6 个非 defer 的 tools (bash / read / write / glob / grep / tool_search)
  LLM: tool_search(query="delegate to research agent")
    → 返回 [delegate_research] 的完整 schema
  LLM: delegate_research(...)  ← 现在调得动了
```

## 3. 涉及文件

```
新增:
  internal/layers/contextengine/enforce/toolrunner/surface/tool_search_surface.go
  internal/layers/contextengine/enforce/toolrunner/surface/tool_search_surface_test.go
  internal/layers/contextengine/enforce/toolrunner/zodgen/zodgen.go
  internal/layers/contextengine/enforce/toolrunner/zodgen/zodgen_test.go
  internal/layers/anthropic/anthropic.go (stub, v1.1)
  internal/layers/anthropic/anthropic_test.go (stub, v1.1)
  internal/layers/contextengine/enforce/toolrunner/surface/builtin_surface.go (改: Tools() 加 DeferLoading)

修改:
  internal/shared/contracts/tool_surface.go
    ToolSpec: + DeferLoading bool
    ToolSurface: 不变 (defer 是 ToolSpec 字段, 不是 method)
  internal/shared/contracts/tool_filter.go
    ToolFilter: + ShouldDefer(ctx, spec) bool
  internal/bootstrap/context_engine_builder.go
    BuildSurfaces: + DeferLoadingCandidates 注册 + sort.Slice 不变
  internal/bootstrap/turn_adapter.go
    Prepare: 按 DeferLoading 过滤
    ExecuteRound: 加 tool_search special-case (route 到 ToolSearchSurface)
  internal/layers/contextengine/enforce/toolrunner/surface/builtin_surface.go
    Tools() 输出的 ToolSpec 加 DeferLoading 字段填充 (从 orthogonal_flags 取)
```

## 4. 测试点 (P0 T26-T30)

- **T26**: ToolSpec.DeferLoading 字段定义 + BuildSurfaces 6 个候选工具标 defer
- **T27**: ToolFilter.ShouldDefer 钩子 + PlanModeOpenWorldPolicy 集成 (plan_mode → defer all open-world)
- **T28**: ToolSearchSurface 提供 tool_search tool (返回匹配的 ToolSpec list)
- **T29**: turn_adapter.Prepare 过滤 DeferLoading=true 的 tools (不下发到 LLM)
- **T30**: zodgen: Go struct → JSON Schema subset

## 5. 度量

| 指标 | Baseline | Target |
|------|----------|--------|
| avg_tools_per_turn | 80 | **<10** |
| prompt_cache_hit_rate | <60% | **>85%** |
| tool_selection_accuracy | ~70% | **>85%** (tool_search 降低选择错误) |

## 6. 风险

- **R1**: LLM 在看不到 schema 的情况下若直接调 defer 工具, 行为不可预测. 缓解: ToolSearch 必须返回 schema 才能调用.
- **R2**: zodgen 简化版 JSON Schema 可能与 LLM provider 不兼容. 缓解: 仅用于 internal ToolSearch 输出, 不下发到 LLM.
- **R3**: tool_search 本身被 defer 了 → 死锁. 缓解: tool_search 必须 in-pack (DeferLoading=false).

## 7. Out of Scope

- 真实 Anthropic API 调用 (anthropic package 只留 stub)
- zodgen 完整 schema 转换 (只做 subset)
- ToolSearch 全文/embedding 检索 (只做 exact + glob)