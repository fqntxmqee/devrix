# Tasks: D2 QueryLoop Legacy Path Decommission (TD-QL-LOC)

**Change ID:** devrix-queryloop-legacy-decommission
**Demand ID:** DM-20260617-001
**Status:** S4_Implementation
**Last updated:** 2026-06-17

---

## W1 — D2.QueryLoop.Run 标 Deprecated

| Task | Description | File | Status |
|------|-------------|------|--------|
| W1.1 | Add `LegacyCounter metrics.Counter` + `warnLegacyOnce sync.Once` fields to `Loop` struct | `internal/layers/contextengine/query/loop.go` | done |
| W1.2 | Add Deprecated comment block above `Run()` (3-signal contract) | `internal/layers/contextengine/query/loop.go` | done |
| W1.3 | Increment `LegacyCounter` at the top of `Run()` (nil-safe) | `internal/layers/contextengine/query/loop.go` | done |
| W1.4 | Emit one-shot `slog.Warn` via `warnLegacyOnce.Do` (per Loop instance) | `internal/layers/contextengine/query/loop.go` | done |
| W1.5 | Wire `LegacyCounter` from observability bridge in `engine_builder.go` | `internal/layers/contextengine/engine_builder.go` | done |
| W1.6 | Add `resolveLegacyQueryLoopCounter` helper that nil-safe-gracefully degrades | `internal/layers/contextengine/engine_builder.go` | done |

## W2 — 注册 legacy metric

| Task | Description | File | Status |
|------|-------------|------|--------|
| W2.1 | Create `RegisterLegacyD2Metrics(registry)` factory | `internal/layers/observability/instrument/metrics/legacy.go` | done |
| W2.2 | Define `LegacyD2Metrics` struct with `QueryLoopInvocations` counter | `internal/layers/observability/instrument/metrics/legacy.go` | done |
| W2.3 | Document metric name `d2_query_loop_legacy_invocations_total` in package doc | `internal/layers/observability/instrument/metrics/legacy.go` | done |
| W2.4 | Cover with unit tests (T04) | `internal/layers/observability/instrument/metrics/legacy_test.go` | done |

## W3 — Routing & boot-time warning

| Task | Description | File | Status |
|------|-------------|------|--------|
| W3.1 | Add LEGACY warning to `IsLoopFirst()` docstring | `internal/layers/orchestration/coordinator/routing.go` | done |
| W3.2 | Emit boot-time `slog.Warn` when `routingMode == rule_orchestrate` | `internal/bootstrap/wire_coordinator.go` | done |

## W4 — Tests (P0 coverage on main path + legacy path)

| Task | Description | T-Layer ID | File | Status |
|------|-------------|------------|------|--------|
| W4.1 | Verify counter bumps on every direct Run() | T10 (D7-S2-A06) | `internal/layers/contextengine/query/loop_legacy_test.go` | done |
| W4.2 | Verify nil `LegacyCounter` does not panic | T10 (D7-S2-A06) | `internal/layers/contextengine/query/loop_legacy_test.go` | done |
| W4.3 | Verify orchestrator (loopFirst=true) does NOT bump counter | T09 (D7-S2-A06) | `internal/layers/orchestration/turn/loop_legacy_test.go` | done |
| W4.4 | Verify metric is registered + visible in registry | T04 (D5-S24-A02) | `internal/layers/observability/instrument/metrics/legacy_test.go` | done |
| W4.5 | Verify warning emitted exactly once per Loop instance | T05 (D5-S24-A02) | `internal/layers/contextengine/query/loop_warn_test.go` | done |
| W4.6 | Verify warning payload carries `change`, `dm`, `canonical_path` | T05 (D5-S24-A02) | `internal/layers/contextengine/query/loop_warn_test.go` | done |

## W5 — Spec & T-registry updates

| Task | Description | File | Status |
|------|-------------|------|--------|
| W5.1 | Add Gherkin scenarios to `specs/d7-orchestration/spec.md` | `openspec/specs/d7-orchestration/spec.md` | done (S3) |
| W5.2 | Add LEGACY marker to `specs/d2-context-engine/spec.md` D2-S10 | `openspec/specs/d2-context-engine/spec.md` | done (S3) |
| W5.3 | Register T04, T05 in D5 T-registry | `openspec/specs/d5-observability/t-registry.md` | pending |
| W5.4 | Register T09, T10 in D7 T-registry | `openspec/specs/d7-orchestration/t-registry.md` | pending |

## W6 — Documentation

| Task | Description | File | Status |
|------|-------------|------|--------|
| W6.1 | Write `tasks.md` (this file) | `openspec/changes/devrix-queryloop-legacy-decommission/tasks.md` | done |
| W6.2 | Write `acceptance-report.md` (S5) | `openspec/changes/devrix-queryloop-legacy-decommission/acceptance-report.md` | pending |
| W6.3 | Archive change directory (S6) | `openspec/archive/2026-06-17-devrix-queryloop-legacy-decommission/` | pending |

---

## File Manifest

### New files
- `internal/layers/observability/instrument/metrics/legacy.go`
- `internal/layers/observability/instrument/metrics/legacy_test.go`
- `internal/layers/contextengine/query/loop_legacy_test.go`
- `internal/layers/contextengine/query/loop_warn_test.go`
- `internal/layers/orchestration/turn/loop_legacy_test.go`

### Modified files
- `internal/layers/contextengine/query/loop.go` — Deprecated comment + LegacyCounter + warnLegacyOnce
- `internal/layers/contextengine/engine_builder.go` — wire LegacyCounter
- `internal/layers/orchestration/coordinator/routing.go` — IsLoopFirst() docstring
- `internal/bootstrap/wire_coordinator.go` — boot warning for rule_orchestrate
- `openspec/specs/d7-orchestration/spec.md` — 6 Gherkin scenarios (S3)
- `openspec/specs/d2-context-engine/spec.md` — LEGACY marker on D2-S10 (S3)

### Unchanged (rollback safety)
- All existing tests, configs, YAML files
- `internal/shared/contracts/llm_facade.go` (the "facade adapter" comment)
- `internal/layers/orchestration/turn/query_llm_caller.go` (the "facade adapter" comment)
- `query_loop.enabled` config option (still works, just deprecated)
- `loopFirst` config flag (still works, just emits warning on false)

---

## Verification Notes

- `go test ./...` not run locally (Go toolchain unavailable in this environment)
- All tests are pure Go, no external dependencies, ready for CI
- Linting deferred to CI; code follows existing repo style (gofmt, package-local docs)
- Revert path: `git revert <merge-commit>` (single commit per repo convention)
