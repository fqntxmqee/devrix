---
acceptance-id: devrix-d1-d7-only-ingress
phase: S5_Acceptance
demand-id: DM-20260614-007
status: ACCEPTED
created: 2026-06-14
archived: 2026-06-14
---

# Acceptance Report — devrix-d1-d7-only-ingress

**Change:** D1 入站仅路由 D7 — 退役 D1→D2 legacy 路径  
**Demand:** DM-20260614-007  
**Commit:** `d4ab93b`

---

## Summary

D1 `RouteInbound` 已移除 legacy `D1→D2.Process` 分支；入站（非 Agent）仅经 `IOrchestrationEntry.ProcessMessage`。`d7.enabled=false` 时 `WireD7` 返回错误，进程启动失败。

## Verification

```bash
go build ./...
go test ./internal/layers/communication/... -count=1
go test -tags='acceptance d1' ./tests/acceptance/p0/ -count=1
```

| Result | Command |
|--------|---------|
| PASS | build |
| PASS | communication tests |
| PASS | D1 acceptance |

## T Points

| T ID | Status |
|------|--------|
| D7-D1-T01 | PASS — `TestGateway_D7Enabled_RoutesToEntry` |
| D1-S13-A03-T01 | PASS — matrix D7 path tests |
| D1-S13-A03-T02 | PASS — `TestGateway_MissingOrchestrationEntry` |
| D7-MIG-T01 | PASS — revised 2-combo matrix |

## Breaking Changes

- `NewCommunicationGateway` 移除 `contextEngine` 参数
- `SetOrchestrationEntry(entry)` 移除 `enabled` 参数
- `d7.enabled=false` 不再回退 D1→D2

## Verdict

**ACCEPTED** — P0 测试点全绿，BREAKING 变更已文档化。
