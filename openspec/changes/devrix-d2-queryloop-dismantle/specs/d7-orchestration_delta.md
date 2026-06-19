# Delta: D7 Orchestration — SubTurn Extension

**Change ID:** `devrix-d2-queryloop-dismantle`  
**Demand ID:** DM-20260618-010

---

## ADDED

### Requirement: D7 SubTurn Execution (SubQuery / Background / Wave)

D7 SHALL execute nested LLM↔Tool loops for SubQuery, Background, and Wave SubAgent workers via the same RunTurn state machine as the main ingress path, using distinct TurnScope values.

D7 MUST call D2 only for Prepare, ExecuteToolRound, and Persist per iteration. D7 MUST call D3 directly for LLM streaming.

#### Scenario: SubQuery scope turn
- GIVEN a delegate_subquery tool invocation with parent session context
- WHEN D7 runs SubTurn with scope=sub
- THEN D7 invokes D2 Prepare with forked child context
- AND D7 invokes D3 for LLM streaming
- AND D7 invokes D2 ExecuteToolRound for each tool call batch
- AND SubQueryFlowReporter receives started/completed events

#### Scenario: Wave worker scope turn
- GIVEN a WaveScheduler SubAgent worker dispatch
- WHEN the worker runner starts execution
- THEN D7 SubTurn with scope=wave_worker runs to completion or cancellation
- AND scheduler ctx cancellation propagates to SubTurn ctx

#### Scenario: Background async scope
- GIVEN delegate_explore with async=true
- WHEN background execution starts
- THEN D7 SubTurn runs in a goroutine with scope=background
- AND terminal status is registered in BackgroundRegistry

### Requirement: D2 QueryLoop Legacy Path Removal

After migration, `routing_mode=rule_orchestrate` and direct D2 QueryLoop ingress MUST NOT exist. All ingress and nested execution MUST flow through D7 RunTurnLoop variants.

#### Scenario: No legacy D2 loop ingress
- GIVEN production default configuration
- WHEN ProcessMessage handles a FastPath message
- THEN only D7 RunTurn(scope=main) executes
- AND D2 QueryLoop.Run is never invoked

---

## MODIFIED

### Requirement: D7-S2-A06 RunTurnLoop

RunTurnLoop SHALL accept TurnScope to distinguish main, sub, background, and wave_worker execution. Scope-specific hooks (FlowReporter, sidechain, read-only tool filter) MUST be injected at D7, not D2.

---

## REMOVED

### Requirement: D2 QueryLoop Legacy Path Decommission (Z0-only)

Z0 deprecation signals (metric, slog.Warn) are superseded by full removal in this change. See DM-20260617-001 archive; TD-QL-LOC closed by DM-20260618-010.
