# Delta: D2 Context Engine — QueryLoop Dismantle

**Change ID:** `devrix-d2-queryloop-dismantle`  
**Demand ID:** DM-20260618-010

---

## REMOVED

### Requirement: D2-S16 RunQueryLoop (Thin Loop)

**Reason:** 循环调度权归 D7-S2-A06；D2 仅保留无状态 Prepare / ToolRound / Persist。  
**Removed:** 2026-06-18 (target)

#### Scenario: Legacy loop no longer exists
- GIVEN the codebase after Phase 3 merge
- WHEN searching for `query.Loop.Run`
- THEN zero production call sites exist

### Requirement: D2-S10 QueryLoop Module (Legacy Index)

**Reason:** Physical module `query/loop.go` deleted; capabilities split to D7 (loop) + D2-S18 (single tool round).

---

## MODIFIED

### Requirement: D2 Domain North Star

D2 SHALL provide **stateless** context assembly primitives only: PrepareExecutionContext, ExecuteToolRound, PersistSessionState. D2 MUST NOT own a multi-turn LLM↔Tool loop, MUST NOT invoke LLM, and MUST NOT import D7, D3, or D4 packages.

#### Scenario: Engine construction without LLM caller
- GIVEN bootstrap wiring for ContextEngine
- WHEN NewContextEngine is called
- THEN no QueryLLMCaller or LLMCaller field is required
- AND no panic occurs on nil LLM dependency

### Requirement: D2-S18 EnforceExecutionPolicy

ExecuteToolRound SHALL remain the sole D2 entry for tool execution during a turn. SubQuery and Background paths MUST NOT call an internal D2 loop; they delegate to D7 SubTurn.

#### Scenario: SubQuery uses D7 not D2 loop
- GIVEN a delegate_subquery invocation
- WHEN the sub-agent runs
- THEN D7 SubTurnExecutor handles the turn loop
- AND D2 ExecuteToolRound is invoked per tool round only

---

## ADDED

(None — additions are in D7 delta)

---

## Deprecated → Removed Migration Notes

| Legacy | Replacement |
|--------|-------------|
| `query.Loop.Run` | D7 `RunTurn` / `RunSubTurn` |
| `QueryLLMCaller` | D7 `GatewayInvoker` |
| `engine.Process` loop path | D7 `QueryLoopExecutor` only |
| `query_loop.enabled` config | removed |
| `routing_mode=rule_orchestrate` | removed or thin-wrap D7 |
