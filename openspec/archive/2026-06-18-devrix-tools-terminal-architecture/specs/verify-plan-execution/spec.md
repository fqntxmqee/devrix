# Verify Plan Execution Specification

**Surface ID:** D6-S11-A02
**Phase:** 1 (P0)
**Status:** S3_Designed

<!-- T: D6-S11-A02-T01, D6-S11-A02-T02, D6-S11-A02-T03 -->

## ADDED

### Requirement: tasks.md Parsing

#### Scenario: Parse Standard tasks.md
- GIVEN a tasks.md file with the standard format (## Tasks + ### Task + acceptance criteria)
- WHEN VerifyAggregate parses the file
- THEN each task is extracted with its acceptance criteria
- AND the parse result is a list of `Task{name, criteria[]}` structs

<!-- T: D6-S11-A02-T01 -->

#### Scenario: Empty tasks.md
- GIVEN an empty tasks.md file
- WHEN VerifyAggregate parses the file
- THEN the parse result is an empty list
- AND a warning span `d6.verify.empty_tasks` is emitted

#### Scenario: Malformed tasks.md
- GIVEN a tasks.md with syntax errors
- WHEN VerifyAggregate parses the file
- THEN the parse fails with `ErrVerifyParseFailed`
- AND a span `d6.verify.parse_failed` is emitted with the error

### Requirement: Verification Item Execution

#### Scenario: Type A - Go Tests
- GIVEN a task with acceptance criterion "go test -race ./... passes"
- WHEN VerifyAggregate executes the verification
- THEN the command is run
- AND the result is parsed (exit code, stdout, stderr)
- AND a span `d6.verify.test_run` is emitted

<!-- T: D6-S11-A02-T02 -->

#### Scenario: Type B - go vet + gofmt
- GIVEN a task with acceptance criterion "go vet && gofmt -l"
- WHEN VerifyAggregate executes the verification
- THEN both commands are run
- AND the result indicates pass/fail based on combined output

#### Scenario: Type C - P0 T Point Coverage
- GIVEN a task with acceptance criterion "P0 T 点 100% PASS"
- WHEN VerifyAggregate executes the verification
- THEN the P0 T points for the relevant domain are queried
- AND the result is pass only if all P0 are green
- AND a span `d6.verify.p0_check` is emitted with pass/total count

#### Scenario: Type D - CI Lint
- GIVEN a task with acceptance criterion "import lint passes"
- WHEN VerifyAggregate executes the verification
- THEN the CI lint command is run
- AND the result indicates pass/fail

### Requirement: Result Aggregation

#### Scenario: All Tasks Pass
- GIVEN all tasks' verifications pass
- WHEN VerifyAggregate aggregates the results
- THEN the overall verdict is `Pass`
- AND the S4-Gate is auto-approved
- AND a span `d6.verify.pass` is emitted

<!-- T: D6-S11-A02-T03 -->

#### Scenario: One Task Fails
- GIVEN task 3/5 fails verification
- WHEN VerifyAggregate aggregates the results
- THEN the overall verdict is `Fail`
- AND the failed task's name and reason are returned
- AND the S4-Gate is held (manual review required)
- AND a span `d6.verify.fail` is emitted with the failed task

#### Scenario: Verification Timeout
- GIVEN a task's verification takes > 5 minutes
- WHEN the timeout is reached
- THEN the verification is marked as `Timeout`
- AND the overall verdict is `Fail` (treat timeout as fail)
- AND a span `d6.verify.timeout` is emitted

### Requirement: Integration with D6 Evolution

#### Scenario: Verify Result Triggers Reputation Update
- GIVEN a verification cycle completes (pass or fail)
- WHEN the result is aggregated
- THEN the D6 reputation store is updated
- AND a failing verification decreases the LLM/tool reputation
- AND a passing verification restores reputation

#### Scenario: Verify Result Emitted to D5 Span
- GIVEN a verification cycle completes
- WHEN the result is aggregated
- THEN a span `d6.verify.cycle_complete` is emitted with `{verdict, task_count, passed, failed}`

### Requirement: LTL-Lite Invariants

The Verify surface MUST satisfy:

- `verification_idempotent` (running twice gives same result)
- `failure_does_not_modify_code` (failed verification doesn't roll back code)
- `timeout_enforced` (max 5min per verification)
- `p0_check_completeness` (P0 check is exhaustive, no skipped tests)

## MODIFIED

### Requirement: D6 Evolution Module Gains Verify Sub-Component

The `evolution/guard/` package gains a new sub-component for plan execution verification:

```go
// internal/layers/evolution/guard/verify_plan_execution/
```

This is a sibling of the existing `verify/` (D6-S11-A01 — single-shot verification).

## REMOVED

(None)
