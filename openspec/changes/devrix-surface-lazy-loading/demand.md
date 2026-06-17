# Demand: devrix-surface-lazy-loading

**Demand ID:** DM-20260618-003
**Created:** 2026-06-18
**Priority:** P0
**Status:** S1_Proposed
**Parents:**
- devrix-tool-spec-enrichment (DM-20260618-001, S7_archived)
- devrix-surface-permission-extension (DM-20260618-002, in-flight)

## Problem Statement

ToolSpec currently exposes 7 surfaces × ~12 tools ≈ 80+ tool schemas to the LLM on every turn (via `Prepare.Tools`). Anthropic's "Tool Search Tool" and Claude Code's `defer_loading` flag prove that **at scale** this is wasteful: most turns only need 2-4 tools, the rest are "discoverable but not present in context".

实测影响：
- **prompt cache miss**: 大 schema 列表每次 hash 不同，导致 cache 命中率低
- **context budget**: 80+ JSON schemas 占 ~10-15K tokens 的纯开销
- **tool selection drift**: 模型看到太多同名 / 相似功能 tool，倾向于选错

## Proposed Solution (Summary)

引入 ToolSpec 上的 `DeferLoading bool` 字段 + ToolFilter 上 `ShouldDefer(ctx, spec) bool` 钩子 + 一个新 surface `ToolSearchSurface` (Claude 风格的 tool search 工具)。

三层 lazy 机制：
1. **Static defer**: BuildSurfaces 阶段基于工具名 hardcoded 列表标 defer (e.g. `delegate_*` / `*_background`)
2. **Filter-based defer**: ToolFilter.ShouldDefer(ctx, spec) 动态判断 (e.g. plan_mode 把 open-world tools 标 defer)
3. **ToolSearch runtime discovery**: LLM 调用 `tool_search` → 返回匹配的 tool definitions → 下一轮再调用

附加：
- `zodgen` Go-side 生成 zod-like schema (JSON Schema from Go struct tags)
- `anthropic` 平面：从 Anthropic API spec 拉 tool list (后续 v1.1，本 change 只留 stub)

## Acceptance Criteria

| AC | 描述 |
|----|------|
| AC1 | ToolSpec 加 `DeferLoading bool` 字段（v1.1 字段，defer 标记） |
| AC2 | ToolFilter 加 `ShouldDefer(ctx, spec) bool` 钩子 |
| AC3 | 新建 `ToolSearchSurface` (7th surface 类型变体)，提供 `tool_search` 工具 |
| AC4 | BuildSurfaces 阶段把 6 个 defer-候选工具 (`delegate_*` × 5 + `*_background` × 1) 标 defer |
| AC5 | plan_mode 下 ToolFilter 把 open-world tools 标 defer（除 allowlist） |
| AC6 | turn_adapter.Prepare 在 LLM 调用前**过滤** DeferLoading=true 的 tools (保留 schema 但不下发到 LLM) |
| AC7 | turn_adapter 支持 `tool_search` 调用：返回匹配的 ToolSpec 列表 (schema) |
| AC8 | zodgen 工具：接受 Go struct tag (`json:"..."`) 输出 JSON Schema (subset) |
| AC9 | `anthropic` package stub：v1.1 入口，列出待实现 items (无实现) |
| AC10 | T 注册表：TOOL-SURFACE-1-T26-T30 (5 个新 P0 T 点) |
| AC11 | go test -race ./... PASS |
| AC12 | go vet + gofmt + devrix-layer-lint --strict PASS |

## Out of Scope (v1.1+)

- 真实 Anthropic API 调用 (留 stub)
- zodgen 完整 schema 转换 (只做 subset: type / properties / required / enum)
- ToolSearch 工具的全文检索 (只做 exact + glob match)
- ToolSearch 的 embedding 检索

## Risks

- **R1**: Anthropic 实际 tool_search tool 在 Sonnet 4.5 已 GA，本 change 只在架构层闭环，不接 API。后续 v1.1 通过 `anthropic` package 接入。
- **R2**: defer 工具的 schema 必须保留可被 ToolSearch 返回 — 否则 LLM 看不到 schema 无法调用。v1 实现: schema 全量保留, 仅 Prepare 阶段不下发。
- **R3**: zodgen 简化版的 JSON Schema 可能与 LLM provider 不兼容 — 仅用于 internal ToolSearch 输出。

## Related

- [Anthropic Tool Search Tool](https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/tool-search-tool)
- [Claude Code defer_loading](https://docs.claude.com/en/docs/claude-code/settings#available-settings)