# Acceptance Report: devrix-d1-dsaft-refactor (DM-20260628-003)

**Change ID:** devrix-d1-dsaft-refactor  
**Demand ID:** DM-20260628-003  
**Status:** S7_Archived  
**Acceptance Date:** 2026-06-28

---

## 1. Phase 交付摘要

| Phase | 范围 | 状态 |
|-------|------|------|
| 1 | sessionagents 迁出 capture | ✅ |
| 1.5 | AC + t-registry + terminal-state-guide | ✅ |
| 2 | Gateway 拆分 + IContextEngine 移除 | ✅ |
| 3 | contracts DTO + channel 解耦 | ✅ |
| 4 | Legacy 标注 + CI import lint + spec 回写 | ✅ |

---

## 2. Refactor T 矩阵（D1-RF-T01..T09）

| T ID | 描述 | 状态 |
|------|------|------|
| D1-RF-T01 | capture 零 multiagent/orchestration import | ✅ |
| D1-RF-T02 | beforeDispatch + D7 ProcessMessage | ✅ |
| D1-RF-T03 | permission_required → RoutePermission | ✅ |
| D1-RF-T04 | orphan EngineEvent sink | ✅ |
| D1-RF-T05 | nil orchestrationEntry 失败 | ✅ |
| D1-RF-T06 | text delta Conclusion journey | ✅ |
| D1-RF-T07 | milestone_progress presenter | ✅ |
| D1-RF-T08 | Gateway 拆分测试锚点 | ✅ |
| D1-RF-T09 | channel/adapters import 门禁 | ✅ |

---

## 3. LC 承诺

| ID | 承诺 | 状态 |
|----|------|------|
| LC-1..LC-4 | 四条流 + 必达 | ✅ 既有 T + signal journey |
| LC-5 | capture 边界 | ✅ D1-RF-T01 |
| LC-6 | session leader 在 bootstrap | ✅ D1-RF-T02..T04 |

---

## 4. 验证命令

```bash
go test ./internal/layers/communication/capture/... ./internal/bootstrap/sessionagents/...
go test ./internal/layers/communication/channel/adapters/...
go test -tags="acceptance d1" ./tests/acceptance/p0/... -run D1
./scripts/lint-d1-imports.sh
go build ./cmd/devrix/...
```

---

## 5. Follow-up（非本 change）

| 项 | 说明 |
|----|------|
| Phase 3b | `/task` 完全迁 D7 CommandHandler |
| gateway.go LOC | span helpers 留 facade（324 LOC）；可选抽 `spans.go` |
| Real-device 飞书 E2E | observability Runbook 手工清单 |

---

## 6. Canonical spec 回写

- `a-registry.md` v3.1.0 — Legacy ARCHIVED + S1-A04 SUPERSEDED  
- `t-registry.md` v3.2.0 — D1-RF-T06..T09  
- `layer-delta.md` — DSAFT refactor + REMOVED 表  
- `spec.md` — Cross-Domain Dependencies 更新  
