# Design: D1 D7-Only Ingress

**Change ID:** `devrix-d1-d7-only-ingress`  
**Demand:** DM-20260614-007

---

## 1. 调用链（终态）

```text
Inbound → D1 capture.RouteInbound
            ├─ agentFactory? → D4 Agent.Run
            └─ orchestrationEntry.ProcessMessage → D7 coordinator
                    └─ FastPath/Orchestrate → D2 IEngine (bootstrap d2Executor)
```

**删除：** `RouteInbound → contextEngine.Process`

## 2. Gateway API

```go
// Before
NewCommunicationGateway(store, handler, contextEngine, permMgr, cfg)
SetOrchestrationEntry(entry, enabled bool)

// After
NewCommunicationGateway(store, handler, permMgr, cfg)
SetOrchestrationEntry(entry contracts.IOrchestrationEntry)  // required for dispatch
SetSessionSnapshotExporter(exp contracts.ISessionSnapshotExporter)  // optional
```

## 3. 启动约束

`bootstrap.WireD7` returns `error`:
- `coordinator.enabled=false` → `fmt.Errorf("d7: disabled; D1 requires orchestration entry")`
- `main` calls `log.Fatal` on error

## 4. 测试适配

`testutil.EngineOrchestrationEntry` implements `IOrchestrationEntry` by delegating to `IEngine.Process` — simulates D7 output without full coordinator stack.

## 5. Spec Delta

- `d7-domain.md` §Migration Coexistence: remove `d7_enabled=false` row; add RETIRED note
- `D1-S13-A03` dispatch scenarios: target = D7 only
- Remove D7-MIG legacy matrix scenarios (or mark ARCHIVED)
