# Delta: D1 Communication — D7-Only Dispatch

**Change ID:** `devrix-d1-d7-only-ingress`  
**Affects:** D1-S13-A03 DispatchToAgent

---

## MODIFIED

### Requirement: D1-S13-A03 DispatchToAgent

D1 MUST route non-agent inbound messages exclusively to D7 via `contracts.IOrchestrationEntry.ProcessMessage`. D1 MUST NOT invoke `contracts.IEngine.Process` directly.

#### Scenario: Inbound dispatches to D7

- GIVEN a wired `IOrchestrationEntry` and no `AgentFactory`
- WHEN `RouteInbound` receives a valid message
- THEN `orchestrationEntry.ProcessMessage` is invoked exactly once
- AND `IEngine.Process` is NOT called from D1 capture

#### Scenario: Missing orchestration entry fails fast

- GIVEN no `IOrchestrationEntry` on gateway
- WHEN `RouteInbound` receives a valid non-feedback message
- THEN an error is returned indicating orchestration is not configured

---

## REMOVED

### Requirement: D1→D2 Legacy Dispatch Path

- REMOVED `d7_enabled=false → contextEngine.Process` branch
- REMOVED migration coexistence matrix rows for `d7_enabled=false`
