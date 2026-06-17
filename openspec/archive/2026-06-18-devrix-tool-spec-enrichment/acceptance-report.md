# Acceptance Report: devrix-tool-spec-enrichment

**Change ID:** devrix-tool-spec-enrichment
**Demand ID:** DM-20260618-001
**Parent Demands:** DM-20260617-007 (devrix-tool-surface-contract, S7_archived); DM-20260617-008 (devrix-tool-surface-phase2-full, S7_archived)
**Status:** S5_Verified → S6_Archived
**Generated:** 2026-06-18
**Branch:** `feat/devrix-tool-spec-enrichment`
**Merged PR:** [#67](https://github.com/fqntxmqee/devrix/pull/67) (squash + delete-branch)

## Summary

本 change 完成 ToolSpec 4 个正交 bool 字段 + ToolSurface InterruptBehavior 方法的横切契约扩张, 并把 BuildSurfaces 输出顺序 sort.Slice by name (prompt cache 稳定), turn_adapter ExecuteRound 按 ConcurrencySafe 走并行 dispatch。

**4 个新 P0 T 点 (T22-T25) 全部 IMPLEMENTED, 既有 11 个 P0 T 点保持 PASS。**

| 类别 | 实现 | 测试 | 状态 |
|------|------|------|------|
| ToolSpec 4 bool (F02) | shared/contracts/tool_surface.go + 7 surface 填充 | T22 | PASS |
| InterruptBehavior (F05) | ToolSurface interface + 7 实现 + central truth table | T23 | PASS |
| BuildSurfaces sort.Slice (A05) | bootstrap/context_engine_builder.go | T24 | PASS |
| turn_adapter parallel dispatch | bootstrap/turn_adapter.go errgroup + indexed write-back | T25 | PASS |
| 既有 T01-T11 P0 (保持) | 既有路径 | T01-T11 | PASS |

## 验收门禁

- `go build ./...` — PASS
- `go test -race ./...` — PASS
- `go vet ./...` — PASS
- `gofmt -l` — clean
- `devrix-layer-lint --strict` — PASS
- CI (PR #67): unit tests / layer-lint (strict + warn) / integration / coverage — 全 SUCCESS
- 自动 squash merge + delete-branch — 完成

## Metrics

| 指标 | Baseline | Target | Actual |
|------|----------|--------|--------|
| tool_spec_orthogonal_flag_coverage | 0% | 100% | **100%** (7/7 surface) |
| build_surfaces_sort_stability | 0% | 100% | **100%** (sort.Slice by name) |
| turn_adapter_parallel_dispatch_pct | 0% | 100% | **100%** (ConcurrencySafe → errgroup) |

`tool_interrupt_cancel_response_ms` (target < 200ms) 进入 v1.1 (depend on Wave/runner 层的实际 cancel 接线), 本 change 仅交付契约。

## Follow-up

- **devrix-surface-permission-extension** (DM-20260618-002) — Per-tool CheckPermission + IPermissionGate.ToolPolicy + BashAST + PlanMode.
- **devrix-surface-lazy-loading** (DM-20260618-003) — DeferLoading + ShouldDefer + zodgen + ToolSearch + anthropic 平面.
