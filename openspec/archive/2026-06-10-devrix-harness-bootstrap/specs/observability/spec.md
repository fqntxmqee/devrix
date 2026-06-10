# Observability Specification (Delta)

**Capability:** observability
**Change ID:** devrix-harness-bootstrap
**Demand ID:** DM-20260609-004
**Parent Spec:** `openspec/specs/observability/spec.md` v1.3.0
**Merge target version:** 1.4.0

---

## ADDED Requirements

### Requirement: Harness Bootstrap Jaeger Operations

Harness Bootstrap 相关 Span MUST 使用 `{layer}.{module}.{action}` canonical 名称，常量定义于 `telemetry/names.go`，并登记于 `coverage/registry.go`（`Instrumented: true`，`SinceVersion: "2.1.0"`）。

**Priority**: P0
**Rationale**: Harness 多阶段编排需可追踪；Jaeger 过滤依赖 canonical Operation
**L4 映射**: L4-OBS-REGISTRY, L4-OBS-COVERAGE
**L5 映射**: L5-2-9-11, L5-5-5-02

#### Scenario: Registry includes harness operations

- GIVEN `coverage.AllOperations()` is loaded
- WHEN comparing to `telemetry/names.go` harness constants
- THEN all harness operation names exist with `Layer=context`, `Component=context_engine`
- AND `registry_test` expected list includes each harness operation

#### Scenario: Span hierarchy under context.process

- GIVEN harness enabled and OTLP/console tracing enabled
- WHEN `ContextEngine.Process` completes one turn
- THEN span `context.process` exists
- AND child span `context.harness.bootstrap.run` parent is `context.process` (first Process only)
- AND child span `context.system_prompt.build` parent is `context.process` and precedes `context.pev.run`
- AND child spans have `devrix.layer=context` and `devrix.component=context_engine`

#### Scenario: Bootstrap stage spans with ctx propagation

- GIVEN harness bootstrap runs with prefetch and tool_pool stages
- WHEN bootstrap completes
- THEN span `context.harness.bootstrap.stage` is emitted per stage
- AND each stage span parent MUST be `context.harness.bootstrap.run`
- AND each stage span has attribute `harness.stage` ∈ {prefetch, guards, setup, deferred_init, tool_pool}
- AND `context.harness.tool_pool` span includes `harness.tools.before` and `harness.tools.after`

#### Scenario: System prompt build span attributes

- GIVEN SystemPromptAssembler.Build completes
- WHEN span `context.system_prompt.build` ends
- THEN attributes include `system_prompt.total_tokens`, `system_prompt.memory_truncated`
- AND attributes include `system_prompt.layer3_tokens` and comma-separated `system_prompt.blocks`

#### Scenario: Preflight span without score cardinality explosion

- GIVEN preflight enabled with warnings
- WHEN `context.harness.preflight` span ends
- THEN attributes include `preflight.mode`, `preflight.warning_count`
- AND attributes MUST NOT include unbounded label values (no raw user message)

#### Scenario: harness.disabled skips harness spans

- GIVEN `context_engine.harness.enabled=false`
- WHEN Process runs
- THEN spans matching `context.harness.*` are NOT created
- AND span `context.system_prompt.build` is NOT created
- AND legacy `context.system_prompt.load` behavior unchanged

---

### Requirement: Harness Bootstrap Info Events

Bootstrap 各阶段 MUST 产生 info 事件（与 span 双写），供 Adapter 四流展示。

**Priority**: P1
**Rationale**: 用户需在 CLI/飞书看到 bootstrap 进度，不仅 Jaeger
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-HARNESS
**L5 映射**: L5-2-9-08

#### Scenario: Bootstrap stages observable via info events

- GIVEN harness enabled and observability bridge configured
- WHEN bootstrap runs
- THEN info events are emitted per stage with metadata `tools.before`, `tools.after`, `trusted`
- AND event metadata aligns with corresponding span attributes
