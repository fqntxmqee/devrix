# LTL-Lite Invariant Specification Framework

**Surface ID:** Cross-cutting (ltllite package)
**Phase:** 1.5 (P0)
**Status:** S3_Designed

<!-- T: PERMISSION-GATE-1-T01, PERMISSION-GATE-1-T02 (cross-cutting) -->

## ADDED

### Requirement: Invariant DSL (Go Struct Tag)

#### Scenario: Parse Invariant from Struct Tag
- GIVEN a Go struct with `invariant:"<pre> => <post>"` tags
- WHEN the LTL-Lite parser processes the struct
- THEN each tagged field becomes an `Invariant{Name, Pre, Post}` entry
- AND the parser returns an `InvariantSet` for the struct

#### Scenario: Valid Invariant Format
- GIVEN an invariant tag `invariant:"read_only => !modifies_files"`
- WHEN the parser validates the format
- THEN the invariant is accepted
- AND the operator is `=>`
- AND the precondition is `read_only`
- AND the postcondition is `!modifies_files`

#### Scenario: Invalid Invariant Format
- GIVEN an invariant tag with no operator (e.g., `invariant:"foo bar"`)
- WHEN the parser validates the format
- THEN the parse fails with `ErrInvalidInvariant`
- AND the error includes the field name

### Requirement: Runtime Invariant Checking

#### Scenario: Invariant Holds - No Violation
- GIVEN a system state where `read_only=true` and `modifies_files=false`
- WHEN the InvariantSet.Check is called with this state
- THEN no violations are returned
- AND the check completes within 5ms

#### Scenario: Invariant Violated
- GIVEN a system state where `read_only=true` but `modifies_files=true`
- WHEN the InvariantSet.Check is called with this state
- THEN a `Violation{Invariant: "read_only_implies_no_mutation", State: ...}` is returned
- AND a span `ltllite.violation` is emitted

<!-- T: PERMISSION-GATE-1-T01 -->

#### Scenario: Multiple Invariants Checked in One Pass
- GIVEN an InvariantSet with 5 invariants
- WHEN Check is called
- THEN all 5 are evaluated
- AND all violations are returned in a single slice

### Requirement: CI Lint Integration

#### Scenario: _invariant.go File Existence Check
- GIVEN a Surface directory (e.g., `internal/layers/contextengine/lsp/`)
- WHEN the CI lint runs
- THEN it verifies that `_invariant.go` exists in the directory
- AND the file is not empty

#### Scenario: Invariant Tag Syntax Validation
- GIVEN a `_invariant.go` file with struct tags
- WHEN the CI lint runs
- THEN it parses all invariant tags
- AND reports any malformed tags as build failures

<!-- T: PERMISSION-GATE-1-T02 -->

#### Scenario: Cross-Surface Invariant Conflict Detection
- GIVEN two surfaces declare conflicting invariants (e.g., "all tools are read_only" vs "X is destructive")
- WHEN the CI lint runs
- THEN the conflict is reported
- AND S3-Gate must resolve the conflict

### Requirement: Turn-Time Validation Hook

#### Scenario: Invariant Check at Turn Start
- GIVEN a D7 turn is about to start
- WHEN turn_adapter.Prepare is called
- THEN all loaded ToolSurfaces' invariants are checked
- AND if any violation, the turn is aborted with `ErrInvariantViolation`

#### Scenario: Invariant Check at Tool Execution
- GIVEN a tool is about to be executed
- WHEN D7 turn_adapter.Dispatch is called
- THEN the relevant invariants are re-checked
- AND any violation blocks the tool execution

### Requirement: LTL-Lite Self-Invariants

The LTL-Lite framework itself MUST satisfy:

- `parse_latency <= 10ms_per_struct` (parsing performance bound)
- `check_latency <= 5ms_per_turn` (turn-time check bound)
- `violation_does_not_panic` (runtime check is panic-safe)
- `ci_lint_deterministic` (same input always produces same lint result)

## MODIFIED

### Requirement: Each Surface MUST Declare Invariants

Every ToolSurface implementation in the codebase MUST have a corresponding `_invariant.go` file declaring its invariants. This is enforced by CI lint.

Existing surfaces will be migrated to LTL-Lite as part of Phase 1.5 migration plan:
- D2 Surface (`enforce/toolrunner/surface/`)
- LSP Surface (Phase 1)
- Bash Surface (Phase 1)
- FreeFork Surface (Phase 1)
- Tracker (Phase 1)
- Verify (Phase 1)
- MCP Surface (Phase 2)

### Requirement: CI Workflow Adds Invariant Lint Step

The `.github/workflows/ci.yml` gains a new step:

```yaml
- name: LTL-Lite Invariant Lint
  run: |
    for surface_dir in $(find internal/layers -type d -name "surface" -o -name "*surface*"); do
      test -f "$surface_dir/_invariant.go" || (echo "Missing _invariant.go in $surface_dir" && exit 1)
    done
    go run tools/ci-lint-invariant/main.go
```

## REMOVED

(None - LTL-Lite is purely additive)
