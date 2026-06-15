# D7 Orchestration Domain - DSAFT Architecture Analysis

**Capability:** d7-orchestration
**Domain:** D7
**DSAFT Type:** Core Domain
**Version:** 1.0.0
**Status:** Active
**Last Updated:** 2026-06-15
**Parent:** spec.md, a-registry.md, f-registry.md

---

## Overview

D7 Orchestration Domain answers "what, in what order, how, hows it going". As a horizontal coordination layer, D7 orchestrates D2 (LLM-Tool execution) and D4 (multi-agent delegation), and publishes progress events to D1 (communication). D1 still owns ingress; D7 does not replace D1 Gateway.

## DSAFT Five-Layer Definition

| Layer | Definition | D7 Mapping |
|-------|-----------|-------------|
| **D** | Business capability boundary | D7 Orchestration Domain |
| **S** | Business value flow | S1-S5 (5 scenarios) |
| **A** | Business actions | 35 Canonical Activities |
| **F** | Function points | TBD |
| **T** | Test contracts | 56 test points |

## S Layer: Value Flow

| S ID | Scenario | North Star | Role | Status |
|------|----------|------------|------|--------|
| S1 | Work Model | Task/Plan lifecycle | State Authority | IMPLEMENTED |
| S2 | Session Orchestrator | Unified entry + Turn loop | Screening + Turn Leader | IMPLEMENTED |
| S3 | Wave Scheduler | Parallel execution | Mechanism Designer | IMPLEMENTED |
| S4 | Execution Flow | Progress transparent | Costly Signaler | IMPLEMENTED |
| S5 | Decision & Planning | Goal to task structure | Information Producer | IMPLEMENTED |

## A Layer: Activities

### D7-S2: Session Orchestrator

| A ID | Name | Type |
|------|------|------|
| D7-S2-A01 | ProcessMessage | USER |
| D7-S2-A02 | EvaluateIntent | USER |
| D7-S2-A03 | HandleInterrupt | USER |
| D7-S2-A04 | DispatchWorker | INTERNAL |
| D7-S2-A06 | RunTurnLoop | SYSTEM |
| D7-S2-A07 | InvokeLLM | SYSTEM |

### D7-S3: Wave Scheduler

| A ID | Name | Type |
|------|------|------|
| D7-S3-A01 | ScheduleWave | USER |
| D7-S3-A02 | ResolveWorkerContext | USER |
| D7-S3-A03 | GuardConflict | USER |

### D7-S4: Execution Flow

| A ID | Name | Type |
|------|------|------|
| D7-S4-A01 | PublishFlowEvent | SYSTEM |
| D7-S4-A02 | SnapshotWorkPlan | SYSTEM |
| D7-S4-A03 | NotifyGateway | SYSTEM |
| D7-S4-A04 | BridgeAgentSpoke | INTERNAL |
| D7-S4-A05 | BridgeSubQuerySpoke | INTERNAL |

### D7-S5: Decision & Planning

| A ID | Name | Type |
|------|------|------|
| D7-S5-A01 | ClassifyIntent | USER |
| D7-S5-A02 | SynthesizeTaskGraph | USER |
| D7-S5-A03 | SelectExecutor | USER |

## S Contains A Relationships

```
D7-S2 Session Orchestrator
  A01 ProcessMessage
    IntentSkip    -> close channel
    IntentCommand -> CommandHandler (/plan, /task, /help, /stop)
    IntentFast   -> FastPath.Run -> D2
    IntentOrchestrate -> OrchestratePath -> D7-S3/S5
  A02 EvaluateIntent
  A03 HandleInterrupt
  A04 DispatchWorker -> Hub-Spoke
  A06 RunTurnLoop -> A07 InvokeLLM -> D3
  A07 InvokeLLM

D7-S3 Wave Scheduler
  A01 ScheduleWave
  A02 ResolveWorkerContext
  A03 GuardConflict

D7-S4 Execution Flow
  A01 PublishFlowEvent -> GlobalHub.Publish
    -> workplan.Service.Apply
    -> imsink.GatewaySink -> D1
    -> SessionQueue
  A02 SnapshotWorkPlan
  A03 NotifyGateway -> D1
  A04 BridgeAgentSpoke
  A05 BridgeSubQuerySpoke

D7-S5 Decision & Planning
  A01 ClassifyIntent
  A02 SynthesizeTaskGraph
  A03 SelectExecutor
```

## Cross-Domain Relationships

### D7 Entry Points (Called By)

| Caller | Interface | Description |
|--------|-----------|-------------|
| D1 -> D7 | IOrchestrationEntry.ProcessMessage | D1 Gateway routes to D7 |
| D1 -> D7 | IOrchestrationEntry.Cancel | D1 StopProcess triggers Cancel |

### D7 Calls Other Domains

| Callee | Interface | Description |
|--------|-----------|-------------|
| D7 -> D2 | QueryLoopExecutor | FastPath + Tool execution |
| D7 -> D3 | ILLMGateway | DM-020: Direct LLM call |
| D7 -> D4 | DelegateExecutor | Worker delegation |
| D7 -> D1 | EventPublisher | Progress events |
| D6 -> D7 | AdvisoryValidator | Decision validation |
| D5 observes D7 | spans | orchestration.* spans |

## Key Design Decisions

1. Entry Replacement, Not Substitution
   - D7 does not replace D1 Gateway
   - D1 owns ingress; D7 takes over orchestration

2. Hub-Spoke Pattern
   - All FlowEvents flow through GlobalHub.Publish
   - SpokeBridge isolates D4 Agent / D2 SubQuery

3. DM-020: D2-D3 Separation
   - D2 owns "request LLM result"
   - D7 owns "execute LLM call" (direct D3)

4. Turn Leader Role
   - D7-S2 is Stackelberg leader
   - Owns LLM call decision boundary

5. v1.1 Orthogonal Routing (2026-06-15)
   - 4 independent execution chains
   - IntentSkip, IntentCommand, IntentFast, IntentOrchestrate

## Package Map

| Package | Scenario | Description |
|---------|----------|-------------|
| orchestration/coordinator/ | S2, S5 | SessionOrchestrator + Intent/Decision |
| orchestration/wave/ | S3 | Wave Scheduler |
| orchestration/flow/ | S4 | Execution Flow |
| orchestration/workplan/ | S4 | WorkPlan snapshot |
| orchestration/imsink/ | S4 | IMSink for D1 notification |
| orchestration/hubspoke/ | S4 | Hub-Spoke bridges |
| orchestration/turn/ | S2 | Turn Leader (DM-020) |

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-15 | Initial DSAFT architecture analysis |
