# D1 Communication Domain - DSAFT Architecture Analysis

**Capability:** d1-communication
**Domain:** D1
**DSAFT Type:** Core Domain
**Version:** 1.0.0
**Status:** Active
**Last Updated:** 2026-06-15
**Parent:** spec.md, a-registry.md, f-registry.md

---

## Overview

D1 Communication Domain handles IM platform integration, message presentation, and delivery guarantee. As the entry layer of Devrix, all user interactions flow through this domain.

## DSAFT Five-Layer Definition

| Layer | Definition | D1 Mapping |
|-------|-----------|------------|
| **D** | Business capability boundary | D1 Communication Domain |
| **S** | Business value flow | S13-S18 (6 scenarios) |
| **A** | Business actions | 16 Canonical Activities |
| **F** | Function points | 18 Canonical Functions |
| **T** | Test contracts | 56 test points |

## S Layer: Value Flow

| S ID | Scenario | User Goal | Status |
|------|----------|-----------|--------|
| S13 | CaptureUserIntent | Command parsing, permission | IMPLEMENTED |
| S14 | PresentThinking | Real-time thinking output | IMPLEMENTED |
| S15 | PresentTaskProgress | Task/tool/Worker progress | IMPLEMENTED |
| S16 | DeliverConclusion | Final conclusion | IMPLEMENTED |
| S17 | ConnectChannel | Multi-IM platform | IMPLEMENTED |
| S18 | GuaranteeDelivery | Guaranteed delivery | IMPLEMENTED |

## A Layer: Activities

| A ID | Name | S归属 | Kind |
|------|------|-------|------|
| D1-S13-A01 | AcceptInboundMessage | S13 | USER |
| D1-S13-A02 | PersistUserTurn | S13 | SYSTEM |
| D1-S13-A03 | DispatchToAgent | S13 | USER |
| D1-S13-A04 | ResolvePermissionGate | S13 | USER |
| D1-S13-A05 | ParseCommand | S13 | USER |
| D1-S14-A01 | EmitThinkingDelta | S14 | SYSTEM |
| D1-S15-A01 | EmitToolProgress | S15 | SYSTEM |
| D1-S15-A02 | EmitWorkerProgress | S15 | SYSTEM |
| D1-S16-A01 | EmitSummaryChunk | S16 | SYSTEM |
| D1-S16-A02 | FinalizeReply | S16 | SYSTEM |
| D1-S17-A01 | ParseFeishuInbound | S17 | USER |
| D1-S17-A02 | ParseDingTalkInbound | S17 | USER |
| D1-S17-A03 | ParseCLIInbound | S17 | USER |
| D1-S17-A04 | ManageConnection | S17 | INTERNAL |
| D1-S17-A05 | RegisterInstance | S17 | INTERNAL |
| D1-S17-A06 | CheckRateLimit | S17 | INTERNAL |
| D1-S18-A01 | DeliverOutboundSignal | S18 | SYSTEM |

## Cross-Domain Relationships

| Direction | Domain | Interface |
|-----------|--------|-----------|
| D1 -> D2 | Context Engine | IEngine.Process |
| D1 -> D4 | MultiAgent | IAgentFactory |
| D1 -> D7 | Orchestration | IOrchestrationEntry.ProcessMessage |
| D1 <-> D5 | Observability | Observability, Bridge |

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-15 | Initial DSAFT architecture analysis |
