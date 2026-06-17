# Delta Spec: Tool Surface Contract (TOOL-SURFACE-1)

**Demand ID:** DM-20260617-007
**Capability:** tool-surface-contract
**Domains:** D2 (Context Engine), D7 (Orchestration), 横切 TOOL-SURFACE-1

## ADDED Requirements

### REQ-TS-01: ToolSurface 拆面契约

The system SHALL expose a `contracts.ToolSurface` interface with four
methods — `Name() / Tools(ctx, workDir, sessionID) []ToolSpec /
RiskLevel(name) RiskLevel / Execute(ctx, name, input, workDir)
(*ToolResult, error)` — defined in `internal/shared/contracts/tool_surface.go`.

Library packages (freefork, tracker, verify, etc.) SHALL NOT depend on
this contract; the dependency direction is
`contracts ← surface (in toolrunner/surface) ← library`.

#### Scenario: 7 surface implementations satisfy the interface

- **GIVEN** the contracts package defines ToolSurface with 4 methods
- **WHEN** any of BuiltinSurface, LSPToolSurface, FreeForkSurface, TrackerSurface, VerifySurface, DelegateSurface, BackgroundTaskSurface is constructed
- **THEN** the surface SHALL satisfy the interface (compile-time `var _ contracts.ToolSurface = ...` assertion passes)

#### Scenario: LSPToolSurface returns the lsp schema unconditionally

- **GIVEN** an LSPToolSurface constructed with `LSPConfig{Enabled: false}` or `nil`
- **WHEN** `Tools(ctx, "", "")` is called
- **THEN** the surface SHALL return a non-empty slice with one ToolSpec named "lsp"
- **AND** calling `Execute(ctx, "lsp", ...)` SHALL return `ToolResult{Error: "lsp: tool is disabled (LSPConfig.Enabled=false)..."}`

### REQ-TS-02: ToolFilter 拆面契约

The system SHALL expose a `contracts.ToolFilter` interface with one
method — `Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec` — defined
in `internal/shared/contracts/tool_filter.go`. `FilterCtx` SHALL carry
`SessionID / AgentType / Mode / RiskThreshold` fields.

The contracts package SHALL provide `Composite(filters...)`, `Allow(names...)`,
`Deny(names...)` constructors and an `ApplyFilters(surfaces, filters, ctx)`
helper that wraps each surface with a pre-computed visible spec list.

#### Scenario: Composite chain applies filters in FIFO order

- **GIVEN** two filters `F1` (drops name "x") and `F2` (drops name "y")
- **WHEN** `Composite(F1, F2).Apply([{Name:"x"},{Name:"y"},{Name:"z"}], ctx)` runs
- **THEN** the output SHALL be `[{Name:"z"}]` (F1 removes "x", then F2 sees {y,z} and removes "y")

#### Scenario: ApplyFilters wraps each surface with visible spec list

- **GIVEN** two surfaces (s1, s2) and a filter that drops "tool_a"
- **WHEN** `ApplyFilters([s1, s2], [filter], ctx)` is called
- **THEN** the returned surfaces SHALL be filteredSurface wrappers
- **AND** calling `Execute(ctx, "tool_a", ...)` on the wrapped surface SHALL return `ToolResult{Error: "...not visible..."}` without calling the parent

### REQ-TS-03: Bootstrap 3 入口收编为 1 入口

`bootstrap.NewContextEngine` (main engine) and `bootstrap.buildWithGate`
(per-agent engine) SHALL both construct their tool set via a single
`BuildSurfaces(SurfaceBuildOpts{...})` call. The output of
`BuildSurfaces` is passed to `EngineDeps.Surfaces`. The third entry
`WireDelegate` SHALL be reduced to a per-agent post-init hook that
attaches delegate_* surfaces, not a separate tool-registration path.

#### Scenario: per-agent ⊇ main engine tool set

- **GIVEN** the main engine and a per-agent engine constructed via ContextEngineBuilder
- **WHEN** both engines' `Surfaces()` accessors are iterated and each `Tools()` is collected
- **THEN** every tool the main engine exposes SHALL also appear in the per-agent engine's visible set
- **AND** per-agent MAY add additional tools (delegate_*) but SHALL NOT remove main-visible tools

### REQ-TS-04: turn_adapter.ExecuteRound 走 surface.Execute

`turn_adapter.ExecuteRound` SHALL dispatch tool calls through
`findSurface(name) → filteredSurface.Execute(ctx, name, input, workDir)`.
The legacy path `a.tools.Execute` (IToolRunner) SHALL NOT be used as the
primary dispatch; it MAY be retained for non-tool commands (e.g. LLM
control) but is NOT the tool dispatch path.

#### Scenario: tool call dispatches through surface

- **GIVEN** a tool call `ToolCall{Name: "read_file", Input: '{"path": "foo.txt"}'}` arriving in ExecuteRound
- **WHEN** dispatch is invoked
- **THEN** `findSurface("read_file")` SHALL return the BuiltinSurface
- **AND** `BuiltinSurface.Execute(ctx, "read_file", ...)` SHALL be called
- **AND** the result SHALL be returned to the LLM as a tool message

### REQ-TS-05: 6+ global singleton 两阶段删除

The system SHALL delete (or in stage 1, render zero-reference) the
following global singletons: `toolrunner.globalFreeForker`,
`tracker.GlobalTracker`, `transcript.GlobalWriter`, `flow.GlobalHub`,
`workmodel.GlobalTaskManager`, `sessionqueue.GlobalSessionQueue`,
`freefork.SetGlobalForker`, plus their `SetGlobal*` setters.

Stage 1 (PR #63) SHALL delete the toolrunner layer globals (freefork / lsp / verify
adapters, tracker wire.go). Stage 2 (PR #64 followup) SHALL delete the
remaining 5 globals (transcript / flow / workmodel / sessionqueue / freefork-in-package)
by extending EngineDeps with explicit dependency fields and updating all
callers to inject at construction time.

#### Scenario: git grep returns zero references after stage 1

- **GIVEN** PR #63 is merged
- **WHEN** `git grep -n "SetGlobal\|globalFreeForker\|GlobalTracker" internal/layers/contextengine/enforce/toolrunner/ internal/layers/observability/diagnose/tracker/` is run
- **THEN** the output SHALL be empty (no production-code references)

### REQ-TS-06: devrix tool list CLI

The `devrix tool list` subcommand SHALL dump the current
`BuildSurfaces` output as text or JSON. With `--agent=main` (default) no
filter is applied. With `--agent=explore` (or other non-main) the
`toolpolicy.AsToolFilter` is applied, dropping `delegate_*` and
read-only worker tools.

CLI invocation: `devrix tool list [--agent TYPE] [--format text|json]`

#### Scenario: text output groups tools by surface

- **GIVEN** the standard surface list (builtin + lsp + verify + ...)
- **WHEN** `devrix tool list` is run with default flags
- **THEN** stdout SHALL contain `=== main engine tool list (N surfaces, M tools) ===`
- **AND** for each surface, a `[<surface_name>] K tools` block listing every ToolSpec as `name RISK description`

#### Scenario: explore agent filter drops delegate_*

- **GIVEN** a surface list that includes a synthetic "delegate_explore" tool
- **WHEN** `devrix tool list --agent=explore` is run
- **THEN** the rendered tool list SHALL NOT contain `delegate_explore`
- **AND** the header SHALL read `=== explore engine tool list (...) ===`
