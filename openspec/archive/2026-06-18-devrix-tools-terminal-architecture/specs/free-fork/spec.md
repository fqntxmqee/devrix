# Free Fork Exploration Specification

**Surface ID:** D4-S11-A02 (Free Fork) + D4-S13-A02 (Worktree Isolation)
**Phase:** 1 (P0)
**Status:** S3_Designed

<!-- T: D4-S11-A02-T01, D4-S11-A02-T02, D4-S11-A02-T03, D4-S11-A02-T04, D4-S13-A02-T01 -->

## ADDED

### Requirement: ForkAgent Creation

#### Scenario: Successful Fork with N Sub-Agents
- GIVEN a session with available fork budget
- WHEN LLM calls `free_fork` with `{n: 3, directions: ["DB", "Goroutine", "Network"]}`
- THEN 3 sub-agents are created in parallel
- AND each sub-agent has an isolated worktree
- AND each sub-agent's WorkerContext is initialized with `Goal: explore_<direction>`
- AND the result includes the 3 sub-agent IDs for later SendMessage

<!-- T: D4-S11-A02-T01 -->

#### Scenario: Fork Concurrency Limit Hit
- GIVEN 7 forks are already active (max 8)
- WHEN LLM calls `free_fork` with `{n: 3}`
- THEN 1 fork is created immediately, 2 are queued
- AND a span `d4.fork.queued` is emitted with `queue_position`

#### Scenario: Fork Exceeds Total Budget
- GIVEN the session's total fork budget is 5
- WHEN LLM calls `free_fork` with `{n: 8}` (would exceed)
- THEN the fork is rejected with `ErrForkBudgetExhausted`
- AND a span `d4.fork.budget_exhausted` is emitted

### Requirement: SendMessage Inter-Agent Communication

#### Scenario: Sub-Agent Sends Message
- GIVEN sub-agent A and sub-agent B exist in the same fork group
- WHEN sub-agent A calls `SendMessage` with `{to: B_id, content: "found race in X"}`
- THEN sub-agent B receives the message in its next turn
- AND a span `d4.fork.message` is emitted with `{from, to, size}`

<!-- T: D4-S11-A02-T02 -->

#### Scenario: Cross-Group Message Denied
- GIVEN sub-agent A in fork group 1, sub-agent B in fork group 2
- WHEN sub-agent A tries to SendMessage to B
- THEN the message is denied
- AND a span `d4.fork.message_cross_group_denied` is emitted

### Requirement: Worktree Isolation

#### Scenario: Each Fork Gets Isolated Worktree
- GIVEN a workspace at `/workspace`
- WHEN a fork is created
- THEN the sub-agent works in `/workspace/.devrix-forks/<fork-id>/`
- AND the sub-agent cannot modify files outside its worktree (sandbox enforced)
- AND a span `d4.worktree.isolated` is emitted with `worktree_path`

<!-- T: D4-S13-A02-T01 -->

#### Scenario: Worktree Cleanup on Sub-Agent Exit
- GIVEN a sub-agent completes or times out
- WHEN the sub-agent exits
- THEN its worktree is marked for cleanup
- AND the cleanup happens lazily (next `git worktree prune` cycle)
- AND a span `d4.worktree.cleanup` is emitted

### Requirement: Resource Arbitration

#### Scenario: File Lock Contention
- GIVEN 2 sub-agents both need to modify `/shared/file.go`
- WHEN both attempt modification concurrently
- THEN the first sub-agent acquires the lock
- AND the second sub-agent receives a `WaitForLock` signal
- AND a span `d4.fork.lock_wait` is emitted

<!-- T: D4-S11-A02-T03 -->

#### Scenario: LSP Server Fair Scheduling
- GIVEN 4 sub-agents each want to use the same LSP server
- WHEN concurrent LSP requests are made
- THEN the LSP server is scheduled in round-robin across sub-agents
- AND no single sub-agent can monopolize the LSP server

#### Scenario: Sub-Agent Timeout
- GIVEN a sub-agent has been running for 60 seconds
- WHEN the timeout threshold is reached
- THEN the sub-agent is sent a cancellation signal
- AND a span `d4.fork.timeout` is emitted with `elapsed_ms`
- AND the partial result is returned to the parent

<!-- T: D4-S11-A02-T04 -->

### Requirement: LTL-Lite Invariants

The Free Fork surface MUST satisfy:

- `concurrent_forks <= 8` (hard limit)
- `worktree_isolation` (each fork has its own worktree)
- `send_message_within_group` (no cross-group messaging)
- `timeout_enforced` (sub-agent has max 60s lifetime)
- `lsp_fair_scheduling` (round-robin across forks)

## MODIFIED

### Requirement: Turn Adapter Recognizes free_fork Tool

The D7 turn_adapter is extended to route `free_fork` calls to the FreeForkSurface:

```go
// turn_adapter.go
case "free_fork":
    return freeForkSurface.Execute(ctx, name, input, wc)
```

### Requirement: Provision Module Registers FreeForkSurface

The multiagent provision module gains a new sub-component for FreeFork:

```go
// provision.go
func (p *Provision) FreeForkSurface() *FreeForkSurface {
    return p.freeFork
}
```

## REMOVED

(None)
