# Proposal: D1 DSAFT Refactor

**Change ID:** `devrix-d1-dsaft-refactor`  
**Demand ID:** DM-20260628-003  
**Status:** S7_Archived — 全 Phase 完成，见 `acceptance-report.md`

---

## Goal

Align D1 implementation with Canonical S13–S18: four outbound signal flows + command channel, **single cross-domain orchestration peer D7**.

## Design SoT

| 文档 | 内容 |
|------|------|
| **`design.md` v1.0** | S/A 职责、链路图、包结构、边界、Phase 2–4、Review 清单 |
| `acceptance-criteria.md` | LC / AC-S / AC-A / AC-R / T 映射 |
| `demand.md` | North Star + 三项决策 |

**实施冻结门：** design.md §11 Review 清单全部勾选后，方可合并 Phase 2+ 代码。

---

## 已完成

### Phase 1 — Boundary convergence ✅

- `bootstrap/sessionagents.Manager` 替代 `capture/agent_route.go`
- `SetBeforeDispatch` hook；capture 零 `multiagent` import
- delegate / guard / testutil 接线

### Phase 1.5 — AC & 规格 ✅

- `acceptance-criteria.md` + D1-RF-T01..T05
- `terminal-state-guide` / `f-registry` F03 迁 bootstrap

---

### Phase 2 — Gateway split ✅

- `session.go` / `outbound.go` / `dispatch.go` 从 gateway 拆出
- 移除 `capture.IContextEngine`；testutil/bootstrap 改用 `contracts.IEngine`
- D1-RF-T06（text delta）、D1-RF-T07（milestone presenter）

### Phase 3 — Channel DTO ✅

- `contracts/im_present.go` — WorkerKind, WorkerStreamEvent, WorkerCardOpts, TaskCLIHandler
- `feishu_worker_card.go` 零 `orchestration/*` import
- `cli.go` 依赖 `contracts.TaskCLIHandler` / `PlanCLIHandler`
- D1-RF-T09 channel adapters import 门禁

## 待实施（见 design.md §9）

| Phase | 摘要 |
|-------|------|
| **4** | a-registry legacy 归档；CI import lint |

## Retained (per decision)

- DingTalk adapter stack  
- `channel/instance/` + `connection/` multi-instance registry  

## Non-goals

- D7 内部编排重构  
- 删除 Feishu CardKit / Worker 双卡  
