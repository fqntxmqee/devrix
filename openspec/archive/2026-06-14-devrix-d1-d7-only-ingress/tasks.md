# Tasks: D1 D7-Only Ingress

**Change ID:** `devrix-d1-d7-only-ingress`  
**Demand:** DM-20260614-007

---

## Phase 1: Contracts & Gateway

- [x] 1.1 Add `contracts.ISessionSnapshotExporter`
- [x] 1.2 Remove legacy RouteInbound branch; require orchestrationEntry
- [x] 1.3 Refactor `NewCommunicationGateway` / `SetOrchestrationEntry`
- [x] 1.4 Update `session_snapshot.go` to use exporter setter

---

## Phase 2: Bootstrap & Config

- [x] 2.1 `DefaultCoordinatorConfig().Enabled = true`
- [x] 2.2 `WireD7` returns error when disabled
- [x] 2.3 `main.go` / `obs-verify` fatal on WireD7 failure

---

## Phase 3: Tests

- [x] 3.1 Add `testutil.EngineOrchestrationEntry`
- [x] 3.2 Update capture + acceptance + integration tests
- [x] 3.3 Remove legacy matrix tests (`D7False_*`)

---

## Phase 4: OpenSpec Sync

- [x] 4.1 Delta specs in change package
- [x] 4.2 Update `d7-domain.md`, `t-registry.md`

---

## Quality Gate

- [x] `go build ./...`
- [x] `go test ./internal/layers/communication/capture/...`
- [x] `go test -tags='acceptance d1' ./tests/acceptance/p0/ -run D1_`
