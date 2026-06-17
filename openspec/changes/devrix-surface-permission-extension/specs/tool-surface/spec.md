# Delta Spec: Surface Permission Extension (TOOL-SURFACE-1 v3)

**Demand ID:** DM-20260618-002
**Capability:** tool-surface + permission-gate
**Domains:** D2 (Context Engine), D7 (Orchestration), 横切 TOOL-SURFACE-1 + PERMISSION-GATE-1
**变更类型:** MODIFIED (delta vs DM-20260618-001 v2)
**基础 spec:** `openspec/changes/devrix-tool-spec-enrichment/specs/tool-surface/spec.md` (v2)

---

## MODIFIED Requirements

### REQ-TS-12: ToolSurface.CheckPermission hook

The `contracts.ToolSurface` interface (defined in
`internal/shared/contracts/tool_surface.go`) SHALL add a sixth method
`CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision`.

`Decision` SHALL be a string enum with three values defined in
`internal/shared/contracts/permission.go`:
- `DecisionAllow` ("allow"): the tool may proceed to Execute
- `DecisionDeny` ("deny"): the tool is rejected; Execute SHALL NOT be called
- `DecisionAsk` ("ask"): the tool requires user confirmation; turn_adapter SHALL consult `IPermissionGate.CheckPermission`

#### Default implementations

- `BuiltinSurface.CheckPermission` SHALL return `DecisionAllow` for non-bash tools. For `bash`, it SHALL delegate to an injected `BashASTPolicy` which uses `mvdan.cc/sh/v3/syntax` to parse the command and apply deny-list rules.
- `LSPToolSurface`, `TrackerSurface`, `VerifySurface`, `DelegateSurface`, `BackgroundTaskSurface` SHALL all return `DecisionAllow` (no side effects).
- `FreeForkSurface.CheckPermission` SHALL delegate to `IPermissionGate.CheckPermission(ctx, spec)` (free-fork is a multi-agent decision, not a per-tool AST decision).

#### Calling sequence

`turn_adapter.ExecuteRound` SHALL, for each ToolCall, invoke `surface.CheckPermission` first. If the result is `DecisionAsk`, it SHALL then invoke `permGate.CheckPermission(ctx, spec)`. If the final decision is `Deny` or `Ask`, `surface.Execute` SHALL NOT be called and the corresponding `ToolResult.Error` SHALL be set to a `PermissionDeniedError` or `PermissionAskRequiredError` respectively.

#### Scenario: 5 short-run surfaces default to Allow

- **GIVEN** any of LSPToolSurface, TrackerSurface, VerifySurface, DelegateSurface, BackgroundTaskSurface
- **WHEN** `CheckPermission(ctx, spec, input)` is called for any tool name
- **THEN** the result SHALL be `DecisionAllow`

#### Scenario: 7 surfaces satisfy the 6-method interface

- **GIVEN** contracts.ToolSurface with 6 methods (5 from v2 + CheckPermission)
- **WHEN** all 7 surface implementations are compiled
- **THEN** the 7 compile-time assertions `var _ contracts.ToolSurface = (*XxxSurface)(nil)` SHALL all pass
- **AND** `go build ./...` SHALL succeed with zero errors

### REQ-TS-13: BashASTPolicy deny-list

The `BashASTPolicy` type (defined in
`internal/layers/contextengine/enforce/toolrunner/surface/bash_ast.go`)
SHALL use `mvdan.cc/sh/v3/syntax` to parse bash commands and apply a
default deny-list with at minimum these rules:

| Rule name | Pattern | Reason |
|-----------|---------|--------|
| `rm-rf-root` | `rm -rf /` or `rm -rf /*` | filesystem destruction |
| `dd-overwrite` | `dd ...` | disk block overwrite |
| `mkfs-format` | `mkfs` or `mkfs.*` | filesystem format |
| `sudo-elevate` | `sudo ...` | privilege escalation |
| `chmod-777-root` | `chmod 777 /` | permission opening |

A rule matches when its `Match(*syntax.Stmt) bool` function returns true
for any `*syntax.Stmt` node in the parsed AST. The first matching rule
SHALL cause `BashASTPolicy.Check(cmd)` to return `DecisionDeny` and the
matching rule's `Reason` string.

Commands that fail to parse SHALL result in `DecisionAsk` (conservative).

#### Scenario: rm -rf / is denied

- **GIVEN** a BashASTPolicy with the default deny-list
- **WHEN** `Check("rm -rf /")` is called
- **THEN** the result SHALL be `DecisionDeny`
- **AND** the reason SHALL contain "rm -rf /"

#### Scenario: rm -rf /home is allowed

- **GIVEN** a BashASTPolicy with the default deny-list
- **WHEN** `Check("rm -rf /home")` is called
- **THEN** the result SHALL be `DecisionAllow`

#### Scenario: dd overwriting disk is denied

- **GIVEN** a BashASTPolicy with the default deny-list
- **WHEN** `Check("dd if=/dev/zero of=/dev/sda")` is called
- **THEN** the result SHALL be `DecisionDeny`
- **AND** the reason SHALL contain "dd"

#### Scenario: parse error returns Ask

- **GIVEN** a BashASTPolicy with the default deny-list
- **WHEN** `Check("if then else end")` (incomplete bash) is called
- **THEN** the result SHALL be `DecisionAsk`

### REQ-TS-14: IPermissionGate.CheckPermission + PlanMode policy

The `IPermissionGate` interface (defined in
`internal/layers/orchestration/permission/gate.go`) SHALL add a second
method `CheckPermission(ctx context.Context, spec ToolSpec) Decision`.

The default implementation `PermissionGateAdapter.CheckPermission` SHALL:
1. Apply `PlanModeOpenWorldPolicy`: when `ctx.Value(ModeKey) == "plan_mode"` AND `spec.OpenWorld == true`, return `DecisionDeny` UNLESS `spec.Name` is in `cfg.PlanMode.OpenWorldAllowList`.
2. If step 1 returns the input decision (not Deny), apply risk-based default:
   - `RiskLow` → `DecisionAllow`
   - `RiskMedium` (with or without OpenWorld) → `DecisionAsk`
   - `RiskHigh` → `DecisionAsk`

The allowlist supports wildcard patterns (e.g. `"git_*"` matches `"git_fetch"`, `"git_log"`).

#### Scenario: plan mode denies free_fork

- **GIVEN** a context with `ModeKey = "plan_mode"`
- **AND** a ToolSpec with `Name="free_fork"`, `OpenWorld=true`, `Risk=RiskHigh`
- **AND** a PlanModeOpenWorldPolicy with empty AllowList
- **WHEN** `PermissionGateAdapter.CheckPermission(ctx, spec)` is called
- **THEN** the result SHALL be `DecisionDeny`

#### Scenario: plan mode allowlist bypasses deny

- **GIVEN** a context with `ModeKey = "plan_mode"`
- **AND** a ToolSpec with `Name="git_fetch"`, `OpenWorld=true`, `Risk=RiskMedium`
- **AND** a PlanModeOpenWorldPolicy with AllowList = `["git_*"]`
- **WHEN** `PermissionGateAdapter.CheckPermission(ctx, spec)` is called
- **THEN** the result SHALL be `DecisionAsk` (the Risk-based default after the allowlist bypasses the deny)

#### Scenario: risk low allows

- **GIVEN** a context with `ModeKey = "default"`
- **AND** a ToolSpec with `Risk=RiskLow`
- **WHEN** `PermissionGateAdapter.CheckPermission(ctx, spec)` is called
- **THEN** the result SHALL be `DecisionAllow`

#### Scenario: risk high asks

- **GIVEN** a context with `ModeKey = "default"`
- **AND** a ToolSpec with `Risk=RiskHigh`
- **WHEN** `PermissionGateAdapter.CheckPermission(ctx, spec)` is called
- **THEN** the result SHALL be `DecisionAsk`

### REQ-TS-15: turn_adapter permission check before dispatch

`bootstrap.contextEngineAdapter.ExecuteRound(ctx, req)` SHALL execute
in two phases:

**Phase 1 (sequential, decision-only)**: for each ToolCall, call
`surface.CheckPermission(ctx, spec, input)`. If the result is
`DecisionAsk`, call `permGate.CheckPermission(ctx, spec)`. If the final
decision is `Deny` or `Ask`, populate `results[i].Error` with the
appropriate error type and skip Phase 2 for this index.

**Phase 2 (parallel dispatch, DM-001 T25)**: for ToolCalls that survived
Phase 1, group by `spec.ConcurrencySafe` and dispatch as in DM-001.

The order of `result.Results` SHALL match `req.ToolCalls` (preserved by
indexed slice write-back).

#### Scenario: Deny skips Execute

- **GIVEN** a turn_adapter with a mock BashSurface whose `CheckPermission` returns `DecisionDeny`
- **AND** a ToolRoundRequest with 1 ToolCall (name="bash", input=`{"command":"rm -rf /"}`)
- **WHEN** `ExecuteRound(ctx, req)` is called
- **THEN** `result.Results[0].Error` SHALL contain "permission denied"
- **AND** `mockSurface.executeCount` SHALL be `0` (Execute was NOT called)

#### Scenario: Ask escalates to IPermissionGate

- **GIVEN** a turn_adapter with a mock surface whose `CheckPermission` returns `DecisionAsk`
- **AND** a permGate that returns `DecisionAllow` for this spec
- **WHEN** `ExecuteRound(ctx, req)` is called
- **THEN** `surface.Execute` SHALL be called
- **AND** `result.Results[0].Error` SHALL be empty

#### Scenario: Allow proceeds to parallel dispatch

- **GIVEN** a turn_adapter with 3 read_file ToolCalls (all Allow)
- **WHEN** `ExecuteRound(ctx, req)` is called
- **THEN** Phase 1 SHALL mark all 3 as Allow
- **AND** Phase 2 SHALL dispatch all 3 in parallel (per DM-001 T25)
- **AND** `result.Results` SHALL preserve the original order

### REQ-TS-16: Backward compatibility with DM-001 + DM-007 15 T points

The changes from this delta (REQ-TS-12 through REQ-TS-15) SHALL NOT
break the 15 P0 T points established in DM-007, DM-008, and DM-001:

- T01-T11: 7 surface implementations + per-agent + tool list CLI
- T22-T25: 4 bool flags + InterruptBehavior + BuildSurfaces sort + parallel dispatch

Specifically:
- `BuiltinSurface` (read_file / write_file / edit_file / grep / glob) default `DecisionAllow` preserves prior behavior.
- `IPermissionGate.Request` (DM-006) is unchanged.
- The new `mvdan.cc/sh/v3` dependency does not affect any existing library.
- Library packages (freefork, tracker, verify, multiagent, orchestration business logic) SHALL NOT be modified.

#### Scenario: 15 existing T points still pass

- **GIVEN** the test suite `go test -race ./...` runs the 15 existing P0 T point tests
- **WHEN** this change is applied
- **THEN** all 15 existing tests SHALL continue to pass (T01-T11, T22-T25)
- **AND** the 6 new tests (T26-T29, PERMISSION-GATE-1-T01, T02) SHALL also pass

#### Scenario: library packages unchanged

- **GIVEN** the diff of this change
- **WHEN** `git diff --stat` is filtered to library paths: `internal/layers/contextengine/freefork/`, `internal/layers/contextengine/tracker/`, `internal/layers/contextengine/verify/`, `internal/layers/multiagent/`
- **THEN** the diff SHALL be empty (0 lines changed)

---

## REMOVED Requirements

_None._ This change is purely additive (1 new method on ToolSurface, 1 new method on IPermissionGate, 1 new enum, 2 new error types, new BashASTPolicy package, new PlanMode policy).

---

## Cross-Reference

- **Spec v2 baseline**: `openspec/changes/devrix-tool-spec-enrichment/specs/tool-surface/spec.md` (REQ-TS-07 through REQ-TS-11)
- **Spec v1 baseline**: `openspec/archive/2026-06-17-devrix-tool-surface-contract/specs/tool-surface/spec.md` (REQ-TS-01 through REQ-TS-06)
- **Parent changes**:
  - DM-20260617-007 (S7_archived) — ToolSurface 4-method interface
  - DM-20260617-008 (S7_archived) — 0 global closure
  - DM-20260618-001 (S4_Ready) — ToolSpec 4 bool fields + InterruptBehavior (5th method)
- **Downstream changes** (will consume this delta):
  - DM-005 (devrix-policy-dsl-yaml) — Per-tool policy DSL will replace hard-coded `DefaultBashDenyRules` and `PlanMode.OpenWorldAllowList` with YAML config
- **T registration**: TOOL-SURFACE-1-T26, T27, T28, T29; PERMISSION-GATE-1-T01, T02 to be added to their respective `t-registry.md` files
- **clawcode reference**: `clawcode/src/Tool.ts:101-110, 404-410`; `clawcode/src/hooks/tools.ts:43-58`; `clawcode/src/tools/BashTool/bashParse.ts`
- **External dependency**: `mvdan.cc/sh/v3 v3.5.0+` (pure Go bash parser)
