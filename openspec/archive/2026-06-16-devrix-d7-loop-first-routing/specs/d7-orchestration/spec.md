# Delta: D7 Orchestration — Loop-First Routing

**Change ID:** `devrix-d7-loop-first-routing`
**Demand ID:** `DM-20260616-002`
**Affects:** D7-S2 ProcessMessage, D7-S2 ClassifyIntent, D7-S2 RunTurn, D1 EngineEvent delivery

---

## ADDED Requirements

### Requirement: Loop-First Ingress Routing

When `coordinator.routing_mode` is `loop_first`, the system MUST route all non-empty, non-command inbound messages to the Turn loop without invoking ingress-level OrchestratePath.

**Priority:** P0  
**Rationale:** Aligns with Clawcode single-loop harness; eliminates rule-based misrouting (e.g. CJK greetings).

#### Scenario: Greeting routes to Turn only

- GIVEN `routing_mode=loop_first` and an active IM session
- WHEN the user sends 「你好」
- THEN ProcessMessage MUST invoke TurnOrchestrator.RunTurn
- AND MUST NOT invoke OrchestratePath at ingress
- AND logs MUST NOT contain `plan_formed` or `wave_started` for that turn

#### Scenario: Slash command bypasses Turn

- GIVEN `routing_mode=loop_first`
- WHEN the user sends `/task list`
- THEN CommandHandler.Handle MUST be invoked
- AND TurnOrchestrator.RunTurn MUST NOT be invoked

---

### Requirement: Tool-Gated Wave Orchestration

When `routing_mode=loop_first`, Wave orchestration MUST only start when the Turn loop LLM invokes the `delegate_wave` tool (or successor name registered in turn tool surface).

**Priority:** P0  
**Rationale:** Clawcode delegates complexity to tool choice inside the loop, not ingress rules.

#### Scenario: Complex goal triggers wave via tool

- GIVEN `routing_mode=loop_first` and Turn tools include `delegate_wave`
- WHEN the LLM emits a `delegate_wave` tool_call for a multi-step goal
- THEN OrchestratePath.Run MUST be invoked exactly once
- AND Wave artifacts MUST be streamed back through the Turn EngineEvent channel

---

### Requirement: Single-Path EngineEvent Delivery

For events emitted on the ProcessMessage return channel during an active turn, the system MUST deliver each event to the gateway handler exactly once.

**Priority:** P0  
**Rationale:** Prevents duplicate Feishu IM replies from channel + sink + agent sink overlap.

#### Scenario: No sink mirror on Turn stream

- GIVEN a Turn producing N EngineEvents on the ProcessMessage channel
- WHEN the gateway consumes the channel
- THEN EventPublisher.Publish MUST NOT be called for those N events
- AND handleEngineEvent MUST be invoked exactly N times (one per distinct event)

---

### Requirement: Legacy Rule-Orchestrate Rollback

When `coordinator.routing_mode` is `rule_orchestrate`, the system MUST preserve DM-20260615-004 ingress behavior including FastPathThreshold downgrade to OrchestratePath.

**Priority:** P1  
**Rationale:** Safe rollback without code revert.

#### Scenario: Legacy threshold downgrade

- GIVEN `routing_mode=rule_orchestrate` and `fast_path_threshold=90`
- WHEN ClassifyIntent returns IntentFast with confidence 70
- THEN ProcessMessage MUST route to OrchestratePath at ingress

---

## MODIFIED Requirements

### Requirement: D7-S2-A01 ProcessMessage Intent Dispatch

ProcessMessage MUST support two routing modes configured by `coordinator.routing_mode`:

- `loop_first`: Skip | Command | Turn (IntentFast)
- `rule_orchestrate`: Skip | Command | Fast | Orchestrate (legacy)

**Priority:** P0  
**Rationale:** Supersedes ingress-only 4-path matrix from DM-20260615-004 when loop_first is active.

#### Scenario: Default mode is loop_first

- GIVEN fresh install with default coordinator config
- WHEN routing_mode is unset
- THEN routing_mode MUST default to `loop_first`

---

### Requirement: D7-S5-A01 ClassifyIntent Rule Set

In `loop_first` mode, ClassifyIntent MUST NOT return IntentOrchestrate based solely on message length or absence of fast-pattern match.

**Priority:** P0  
**Rationale:** Removes the 70-confidence short-line → Orchestrate downgrade path.

#### Scenario: Short non-greeting message

- GIVEN `routing_mode=loop_first`
- WHEN the user sends a 10-character non-command message that does not match fast patterns
- THEN ClassifyIntent MUST return IntentFast (Turn) with confidence 100
- AND MUST NOT return IntentOrchestrate

---

## REMOVED Requirements

(None — `IntentOrchestrate` ingress auto-routing is deprecated in `loop_first` mode but retained under `rule_orchestrate`.)

---

## L5 Registry Draft (for t-registry.md sync at S5)

| L5 ID | Title | Priority | Phase |
|---|---|---|---|
| D7-S2-L5-01 | 问候语 Turn 不触发 Wave | P0 | 1 |
| D7-S2-L5-02 | delegate_wave tool 门控 Wave | P0 | 2 |
| D7-S2-L5-03 | Slash 命令零 LLM | P0 | 1 |
| D7-S2-L5-04 | EngineEvent 单投递 | P0 | 1 |
| D7-S2-L5-05 | enter_plan_mode tool | P1 | 2 |
| D7-S2-L5-06 | rule_orchestrate 回滚 | P1 | 1 |
