# Acceptance Report: devrix-surface-lazy-loading

**Change ID:** devrix-surface-lazy-loading
**Demand ID:** DM-20260618-003
**Parent Demands:** DM-20260618-001 (devrix-tool-spec-enrichment, S7_archived); DM-20260618-002 (devrix-surface-permission-extension, S7_archived)
**Status:** S5_Verified → S6_Archived
**Generated:** 2026-06-18
**Branch:** `feat/devrix-surface-lazy-loading`
**Merged PR:** [#70](https://github.com/fqntxmqee/devrix/pull/70) (squash + delete-branch)

## Summary

本 change 把 per-tool lazy loading 升级为 D2 surface machinery 的 1st-class 契约, 让 LLM prompt 在 catalog 增长到 80+ 工具时仍能控制 token 预算。三层机制:
1. **静态标记**: `contracts.ToolSpec.DeferLoading bool` 在 surface.Tools() 一次性写入。
2. **runtime 决策**: `contracts.ToolFilter.DeferDecision` (ShouldDefer 钩子) + `DeferChain` / `AlwaysDefer` / `NeverDefer` helpers; `surface.ShouldDeferByDefault` 覆盖 6 个 hardcoded candidates。
3. **按需发现**: `surface.ToolSearchSurface` 暴露 `tool_search` tool, exact > glob > substring (top-5 cap) 检索 deferred tool schemas。

`turn_adapter.Prepare` 过滤 `DeferLoading=true` 的 specs (除 `tool_search` 本身) 并咨询 `deferDecider` 链。`PlanModeOpenWorldPolicy.ShouldDefer` 在 plan_mode + OpenWorld + not-allowlist 时 defer。`zodgen` 包把 Go struct 转 JSON Schema 子集供 tool_search 渲染参数 shape。`anthropic` 包是 v1.1 client 占位 stub。

**5 个新 P0 T 点 (T26-T30) 全部 IMPLEMENTED + PASS。**

| 类别 | 实现 | 测试 | 状态 |
|------|------|------|------|
| ToolSpec.DeferLoading (A01-F08) | shared/contracts/tool_surface.go + 6 surface 填充 (delegate_*, task_output_background) | T26 | PASS |
| DeferDecision / DeferChain / AlwaysDefer / NeverDefer | shared/contracts/tool_filter.go | T26 | PASS |
| ShouldDeferByDefault (orthogonal_flags) | surface/orthogonal_flags.go 6 hardcoded | T26 | PASS |
| PlanModeOpenWorldPolicy.ShouldDefer (A01) | orchestration/toolpolicy/plan_mode.go | T27/T29 | PASS |
| ToolSearchSurface (A02) | surface/tool_search_surface.go (exact>glob>substring, top-5) | T28 | PASS |
| turn_adapter.Prepare filter (A01) | bootstrap/turn_adapter.go (DeferLoading + deferDecider) | T29 | PASS |
| zodgen (Go struct → JSON Schema) | toolrunner/zodgen/zodgen.go | T30 | PASS |
| anthropic stub (v1.1 placeholder) | layers/anthropic/anthropic.go (ErrNotImplemented) | (no T) | PASS |
| 既有 T01-T11/T22-T29 P0 (保持) | 既有路径 | T01-T29 | PASS |

## 验收门禁

- `go build ./...` — PASS
- `go test -race ./...` — PASS
- `go vet ./...` — PASS
- `gofmt -l` — clean
- `devrix-layer-lint --strict` — PASS
- CI (PR #70): unit tests / layer-lint (strict + warn) / integration / coverage — 全 SUCCESS
- 自动 squash merge + delete-branch — 完成
- `scripts/verify-archive.sh` — PASS

## Metrics

| 指标 | Baseline | Target | Actual |
|------|----------|--------|--------|
| avg_tools_per_turn | ~80 | <10 | **TBD** (in production, expect <10 with 6 default-defer) |
| prompt_cache_hit_rate | <60% | >85% | **TBD** (Anthropic measure) |
| tool_search_coverage | 0% | >90% | **TBD** (real usage) |

prompt-cache hit rate + tool_search coverage 走 prod-side measure, v1.0 仅交付契约 + 路径。

## Follow-up

- v1.1 待办: anthropic real client (替代 stub); zodgen 完整 schema 转换 ($ref / oneOf / anyOf); ToolSearch embedding 检索 (对标 anthropic tool-search-tool)。
- 在 PlanMode 等场景下, TurnRequest.Mode 透传到 PrepareRequest.Mode → DeferDecision 链。
