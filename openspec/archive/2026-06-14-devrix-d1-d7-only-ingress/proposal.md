# Proposal: D1 D7-Only Ingress

**Change ID:** `devrix-d1-d7-only-ingress`  
**Demand:** DM-20260614-007  
**Created:** 2026-06-14  
**Status:** Approved

---

## Problem Statement

D1 `capture.RouteInbound` 仍保留 `d7_enabled=false` 时的 **D1→D2.Process** legacy 分支。设计终态要求 D1 仅 dispatch 到 D7；D2 由 D7 经 `QueryLoopExecutor` 调用。双轨增加认知负担且与 `d7-domain.md` 终态不一致。

## Proposed Solution

1. **Remove** legacy branch and `orchestrationEnabled` flag.
2. **Require** `IOrchestrationEntry` on gateway before inbound dispatch (bootstrap `WireD7` mandatory).
3. **Extract** session snapshot to `contracts.ISessionSnapshotExporter` setter (decouple from ingress engine).
4. **Default** `coordinator.enabled=true`; startup fails if D7 wiring disabled.
5. **BREAKING:** `NewCommunicationGateway` drops `contextEngine` parameter; `SetOrchestrationEntry(entry, enabled)` → `SetOrchestrationEntry(entry)`.

## Impact Analysis

| Component | Change | Details |
|-----------|--------|---------|
| D1 capture | Yes | RouteInbound, constructor, Stop, tests |
| bootstrap | Yes | WireD7 returns error; main fatal on failure |
| config | Yes | Default d7.enabled=true |
| D7 coordinator | No | Entry contract unchanged |
| D2 contextengine | No | Still used by D7 d2Executor |

## Success Criteria

- [ ] No `contextEngine.Process` call from D1 capture
- [ ] All RouteInbound dispatch tests use IOrchestrationEntry
- [ ] Specs updated; legacy migration matrix retired

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Tests relied on direct engine injection | `testutil.EngineOrchestrationEntry` adapter |
| External configs with d7.enabled=false | Startup error with clear log message |
