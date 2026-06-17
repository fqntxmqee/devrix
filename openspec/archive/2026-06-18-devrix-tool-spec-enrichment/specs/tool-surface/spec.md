# Delta Spec: Tool Spec Enrichment (TOOL-SURFACE-1 v2)

**Demand ID:** DM-20260618-001
**Capability:** tool-surface
**Domains:** D2 (Context Engine), D7 (Orchestration), 横切 TOOL-SURFACE-1
**变更类型:** MODIFIED (delta vs DM-20260617-007 v1)
**基础 spec:** `openspec/archive/2026-06-17-devrix-tool-surface-contract/specs/tool-surface/spec.md`

---

## MODIFIED Requirements

### REQ-TS-07: ToolSpec orthogonal flags (4 bool)

The `contracts.ToolSpec` struct (defined in
`internal/shared/contracts/tool_surface.go`) SHALL add four orthogonal
boolean fields: `ReadOnly`, `Destructive`, `OpenWorld`, `ConcurrencySafe`.

Each field SHALL default to `false` (zero value). Every ToolSurface
implementation SHALL explicitly set these fields for every tool in its
`Tools(ctx, workDir, sessionID)` return value. Implementations SHALL NOT
rely on default-zero behavior.

Field semantics:
- `ReadOnly`: tool does not modify the filesystem (e.g. read_file, glob, grep, lsp, verify)
- `Destructive`: tool performs irreversible operations (e.g. write_file, edit_file, bash)
- `OpenWorld`: tool's side effects extend beyond the local machine (e.g. free_fork spawning agents, web_fetch)
- `ConcurrencySafe`: multiple invocations of the same tool may run in parallel without mutual interference (e.g. read_file on different paths)

The `Risk` field SHALL remain unchanged for backward compatibility. The
4 bool fields are consumed by `PerAgentFilter` (auto-extend explore
agent's visible set via `ReadOnly`), `PerRiskFilter` (tighten plan_mode
via `OpenWorld`), and `turn_adapter.ExecuteRound` (parallel dispatch
via `ConcurrencySafe`).

#### Scenario: 7 surface Tools() populates all 4 bool fields

- **GIVEN** any of the 7 surface implementations (BuiltinSurface, LSPToolSurface, FreeForkSurface, TrackerSurface, VerifySurface, DelegateSurface, BackgroundTaskSurface)
- **WHEN** `Tools(ctx, "", "")` is called
- **THEN** for every returned ToolSpec, the 4 bool fields SHALL be explicitly set (assertion: `spec.ReadOnly || spec.Destructive || spec.OpenWorld || spec.ConcurrencySafe` is true; at least one flag MUST be true)

#### Scenario: bash tool is Destructive but ConcurrencySafe

- **GIVEN** BuiltinSurface.Tools(ctx, "", "")
- **WHEN** the returned ToolSpec with Name="bash" is inspected
- **THEN** `Destructive` SHALL be `true`
- **AND** `ConcurrencySafe` SHALL be `true` (different bash commands can run in parallel; per-command validation is the command's responsibility)
- **AND** `ReadOnly` SHALL be `false`
- **AND** `OpenWorld` SHALL be `false`

#### Scenario: free_fork tool is OpenWorld and not ConcurrencySafe

- **GIVEN** FreeForkSurface.Tools(ctx, "", "")
- **WHEN** the returned ToolSpec with Name="free_fork" is inspected
- **THEN** `OpenWorld` SHALL be `true` (spawning child agents is a cross-process side effect)
- **AND** `ConcurrencySafe` SHALL be `false` (multiple forks within a session interfere with each other)
- **AND** `ReadOnly` SHALL be `false`
- **AND** `Destructive` SHALL be `false` (forking itself is reversible by killing children)

### REQ-TS-08: InterruptBehavior method on ToolSurface

The `contracts.ToolSurface` interface SHALL add a fifth method
`InterruptBehavior(name string) InterruptMode` defined in
`internal/shared/contracts/tool_surface.go`.

`InterruptMode` SHALL be a string enum with two values:
- `InterruptCancel` ("cancel"): surface SHALL immediately stop execution and return `ctx.Err()` when the context is cancelled
- `InterruptBlock` ("block"): surface SHALL ignore context cancellation and complete naturally

The default behavior for all surfaces SHALL be `InterruptBlock` (backward
compatible with the 7 surfaces from DM-007). Only `FreeForkSurface`
SHALL explicitly return `InterruptCancel` for `free_fork`.

The surface's `Execute(ctx, ...)` method SHALL, when
`InterruptBehavior(name) == InterruptCancel`, select on `ctx.Done()`
internally and return `ctx.Err()` within 200ms of cancellation.

#### Scenario: FreeForkSurface.InterruptBehavior returns cancel

- **GIVEN** a FreeForkSurface constructed with a stub forker
- **WHEN** `InterruptBehavior("free_fork")` is called
- **THEN** the result SHALL be `contracts.InterruptCancel`

#### Scenario: 6 short-run surfaces return block

- **GIVEN** any of BuiltinSurface, LSPToolSurface, TrackerSurface, VerifySurface, DelegateSurface, BackgroundTaskSurface
- **WHEN** `InterruptBehavior(name)` is called for any tool name
- **THEN** the result SHALL be `contracts.InterruptBlock`

#### Scenario: FreeForkSurface cancel responds within 200ms

- **GIVEN** a FreeForkSurface configured with a forker that takes 5s to complete
- **AND** a context with cancel function
- **WHEN** `Execute(ctx, "free_fork", ...)` is invoked
- **AND** `cancel()` is called 50ms after Execute starts
- **THEN** Execute SHALL return within 200ms of the cancel call
- **AND** the returned error SHALL wrap `context.Canceled`

### REQ-TS-09: BuildSurfaces deterministic ordering

The `bootstrap.BuildSurfaces(SurfaceBuildOpts) []contracts.ToolSurface`
function (defined in `internal/bootstrap/surfaces.go`) SHALL return
surfaces sorted by `Name()` in lexicographic ascending order.

The sort SHALL be applied after all conditional appends (ToolReg,
Tracker, Forker optional paths). The output order SHALL be stable
across different `SurfaceBuildOpts` values where the same set of
surfaces is included (e.g. lsp_enabled / lsp_disabled should not change
the relative order of the other surfaces).

#### Scenario: sort order is lexicographic by surface name

- **GIVEN** a SurfaceBuildOpts that produces surfaces named: lsp, freefork, tracker, builtin, verify
- **WHEN** `BuildSurfaces(opts)` is called
- **THEN** the returned surface order SHALL be: builtin, freefork, lsp, tracker, verify (sorted by Name())

#### Scenario: sort order is stable across env differences

- **GIVEN** three different SurfaceBuildOpts values:
  - `opts1`: ToolReg, LSPConfig{enabled=true}, Tracker, Forker
  - `opts2`: ToolReg, LSPConfig{nil}, Forker (no Tracker)
  - `opts3`: ToolReg, LSPConfig{enabled=false}, Tracker, no Forker
- **WHEN** `BuildSurfaces` is called for each
- **THEN** the Names() of the returned surfaces SHALL be identical (modulo absent surfaces)
- **AND** the relative order of present surfaces SHALL be the same (builtin < freefork < lsp < tracker < verify)

### REQ-TS-10: turn_adapter parallel dispatch by ConcurrencySafe

The `bootstrap.contextEngineAdapter.ExecuteRound(ctx, req)` method
(defined in `internal/bootstrap/turn_adapter.go`) SHALL dispatch tool
calls as follows:

1. Group tool calls by `spec.ConcurrencySafe` from the corresponding surface.
2. Run `ConcurrencySafe=false` tool calls **sequentially** in the original order.
3. Run `ConcurrencySafe=true` tool calls **in parallel** using `errgroup.Group` (Go's `golang.org/x/sync/errgroup` package).
4. The returned `ToolRoundResult.Results` SHALL be in the same order as `req.ToolCalls` (achieved via indexed slice write-back: `results[i] = ...`).

`errgroup.Go` SHALL always return `nil`; per-tool errors are recorded in `result.Error` and SHALL NOT fail other parallel tool calls.

The `isConcurrencySafe(ctx, name)` helper SHALL iterate `a.surfaces` and
return the `ConcurrencySafe` value from the matching `ToolSpec`. Unknown
tool names SHALL default to `false` (conservative: sequential).

#### Scenario: 2 parallel read_file results in original order

- **GIVEN** a turn_adapter with surfaces that include BuiltinSurface
- **AND** a `ToolRoundRequest` with 2 ToolCalls, both name="read_file" on different paths
- **WHEN** `ExecuteRound(ctx, req)` is called
- **THEN** `result.Results` SHALL have 2 elements
- **AND** `result.Results[0].ToolCallID` SHALL equal `req.ToolCalls[0].ID`
- **AND** `result.Results[1].ToolCallID` SHALL equal `req.ToolCalls[1].ID`
- **AND** the wall-clock elapsed time SHALL be less than 2x a single read_file (parallel speedup)

#### Scenario: mixed safe and unsafe tools

- **GIVEN** a `ToolRoundRequest` with 3 ToolCalls: 2 read_file (ConcurrencySafe=true) + 1 write_file (ConcurrencySafe=false)
- **WHEN** `ExecuteRound(ctx, req)` is called
- **THEN** the 2 read_file calls SHALL run in parallel
- **AND** the 1 write_file call SHALL run sequentially (in the order it appears in `req.ToolCalls`)
- **AND** `result.Results` SHALL preserve the original order `[read_file_0, write_file, read_file_1]`

#### Scenario: parallel dispatch does not race

- **GIVEN** a turn_adapter with 5 concurrent read_file ToolCalls
- **WHEN** `ExecuteRound(ctx, req)` is invoked
- **THEN** `go test -race` SHALL report zero race conditions
- **AND** all 5 results SHALL be present in `result.Results` in the original order

### REQ-TS-11: Backward compatibility with DM-007 11 T points

The changes from this delta (REQ-TS-07 through REQ-TS-10) SHALL NOT
break the 11 P0 T points established in DM-007 and DM-008:

- T01-T07: 7 surface implementations satisfy the ToolSurface interface
- T08: per-agent ⊇ main engine tool set
- T09: turn_adapter.findSurface dispatch path
- T10: IPermissionGate integration
- T11: devrix tool list CLI output

In particular:
- The 4 existing ToolSpec fields (Name / Description / Parameters / Risk) SHALL NOT change.
- The 4 existing ToolSurface methods (Name / Tools / RiskLevel / Execute) SHALL NOT change.
- Library packages (freefork, tracker, verify, multiagent, orchestration) SHALL NOT be modified.
- All 7 surfaces SHALL continue to satisfy `var _ contracts.ToolSurface = ...` (compile-time assertion).

#### Scenario: 7 surfaces still satisfy ToolSurface interface

- **GIVEN** contracts.ToolSurface with 5 methods (4 original + InterruptBehavior)
- **WHEN** all 7 surface implementations are compiled
- **THEN** the 7 compile-time assertions `var _ contracts.ToolSurface = (*XxxSurface)(nil)` SHALL all pass
- **AND** `go build ./...` SHALL succeed with zero errors

#### Scenario: library packages unchanged

- **GIVEN** the diff of this change
- **WHEN** `git diff --stat` is filtered to library paths: `internal/layers/contextengine/freefork/`, `internal/layers/contextengine/tracker/`, `internal/layers/contextengine/verify/`, `internal/layers/multiagent/`, `internal/layers/orchestration/`
- **THEN** the diff SHALL be empty (0 lines changed)

#### Scenario: 11 existing T points still pass

- **GIVEN** the test suite `go test -race ./...` runs the 11 existing P0 T point tests
- **WHEN** this change is applied
- **THEN** all 11 existing tests SHALL continue to pass (T01-T11)
- **AND** the 4 new tests (T22-T25) SHALL also pass

---

## REMOVED Requirements

_None._ This change is purely additive (4 new fields, 1 new method, 1 new enum, sort.Slice, parallel dispatch). All existing fields, methods, and behavior are preserved.

---

## Cross-Reference

- **Spec v1 baseline**: `openspec/archive/2026-06-17-devrix-tool-surface-contract/specs/tool-surface/spec.md` (REQ-TS-01 through REQ-TS-06)
- **Parent changes**:
  - DM-20260617-007 (S7_archived) — ToolSurface 4-method interface baseline
  - DM-20260617-008 (S7_archived) — 0 global loop closure
- **Downstream changes** (will consume this delta):
  - DM-20260618-002 (devrix-surface-permission-extension) — Per-tool `CheckPermission(ctx, spec)` hook will consume `OpenWorld` / `Destructive` flags
  - DM-20260618-003 (devrix-surface-lazy-loading) — `ShouldDefer()` method will be added as 6th method on ToolSurface, and lazy-loaded tools will use `ReadOnly` / `OpenWorld` for filter pre-classification
- **T registration**: TOOL-SURFACE-1-T22, T23, T24, T25 to be added to `openspec/specs/tool-surface/t-registry.md`
- **clawcode reference**: `clawcode/src/Tool.ts:402-407, 410-416`; `clawcode/src/tools.ts:362-366`
