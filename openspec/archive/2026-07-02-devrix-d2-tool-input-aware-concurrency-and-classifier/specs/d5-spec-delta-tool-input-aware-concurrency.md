# D5 Spec Delta: LTL-Lite L4-L6 终止不变量 Cross-Check 配套

**Change:** devrix-d2-tool-input-aware-concurrency-and-classifier (DM-20260702-009)
**Archived:** 2026-07-02
**Status:** S7_Archived

## Delta Summary

D5 可观测性层承接本 change 1 个 T 点:
- T25' GrowthBook override 1 flag (bash 30K→50K) — 已与 d2-spec-delta 中 Delta 4 重叠
- D5-S25-A04 新增: LTL-Lite L4-L6 终止不变量 cross-check 配套 (本 change 落地后)

## Delta 1: GrowthBook Override 1 Flag (T25')

### New Flag

| Flag Name | Default | Override | Description |
|-----------|---------|----------|-------------|
| `bash_readonly_threshold_bytes` | 30000 | 50000 | Bash readonly threshold for large command result |

### Production-Safety Constraints

- 默认全关: 启动时 `seedFeatureFlags` 走 secure default (空 map = 全关)
- flag 未开启时, override 返回 defaultVal, **0 行为变化**
- flag 运行时变更通过 GrowthBook SDK 推送, 不需要重启 devrix

### Files

- `internal/layers/observability/instrument/growthbook/registry.go` — flag 注册中心
- `internal/layers/observability/instrument/growthbook/concurrency_override.go` — T16-T17 IsConcurrencySafe 联动
- `internal/layers/observability/instrument/growthbook/growthbook_override_test.go` — Production-Safety 单测

## Delta 2: LTL-Lite L4-L6 Cross-Check 配套

### Background

本 change 在 PR-F 落地 Bash sibling abort + StreamingToolExecutor.Discard() 后, D7 编排层
的多实例并发协调增加, 需要 D5 终止不变量 cross-check 配套。

### New L4-L6 Cross-Check Rules

| Rule ID | Cross-Check | Implementation |
|---------|-------------|----------------|
| CC-STE-01 | Bash sibling abort 不得 override readonly guard | `bash/sibling_abort.go::Register` 仅在 watched call 取消, 不影响 readonly 判定 |
| CC-STE-02 | Discard on fallback 不得丢失 streaming tool_result | `streaming_executor.go::Discard` 保留所有 buffered call 的 ToolCallID |
| CC-STE-03 | Discard idempotent 不得 double-count | `discard_on_fallback.go::OnFallback` 内部 IsDiscarded check |

### Test Coverage

- `internal/layers/observability/instrument/ltl/invariants/termination/sibling_abort_test.go`
- `internal/layers/observability/instrument/ltl/invariants/termination/discard_test.go`

## Cross-Reference

- d2-spec-delta: ToolSurface v4 + 19 工具 default + partition + toCompactBlock + inputsEquivalent
- d7-spec-delta: D7 Execute 节点 partition + Bash sibling abort + Discard