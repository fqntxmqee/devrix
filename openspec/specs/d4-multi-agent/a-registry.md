# D4 Multi-Agent Domain — A 层活动注册表

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.2.0
**Last Updated:** 2026-06-30
**Change ID:** devrix-d4-dsaft-restructuring
**Demand ID:** DM-20260629-004
**Parent:** `openspec/specs/architecture/layering.md`
**Domain SoT:** `d4-domain.md`

> **终态流程 / D7 派发：** 见 `terminal-state-guide.md`  
> **Span↔T Runbook：** 见 `observability-guide.md`

---

## Canonical A 层（SoT）— D4-S11–S16

| A ID | Name | Canonical S | Legacy 映射 | Code（v2.0） |
|------|------|-------------|-------------|-------------|
| D4-S11-A01 | CreateAgent | S11 | S1-A01 | `provision/factory.go` |
| D4-S11-A02 | EnhancePrompt | S11 | S4-A01 | `collaboration/prompt.go` |
| D4-S11-A03 | RegisterBuiltin | S11 | S7-A01 | `orchestration/delegatetools/builtin_agents.go`（D7 owns; D4 仅薄壳转发） |
| D4-S12-A01 | RunAgentLoop | S12 | S2-A01 | `run/lifecycle.go` |
| D4-S12-A02 | ResolvePermission | S12 | S2-A02 | `run/perm_gate.go` |
| D4-S13-A01 | ForkAndJoin | S13 | S3-A01/A02 | `run/forkjoin.go` |
| D4-S13-A02 | ManageSessionView | S13 | S9-A01 | `isolate/sessionview.go` |
| D4-S13-A03 | WrapWorkerEngine | S13 | S2-A03 | `provision/factory.go`（PR-1 #0 已 inline） |
| D4-S14-A01 | ExecuteWorker | S14 | S10-A01（执行） | `execute/worker.go` |
| D4-S15-A01 | RegisterExternalTool | S15 | S6-A01 | `external/registry.go` |
| D4-S15-A02 | ExecuteExternalTool | S15 | S6-A02 | `external/cli_session.go` + `cli_execute.go`（CLI）, `external/cursor_session.go` + `cursor_execute.go`（Cursor） |
| D4-S15-A03 | ParseStreamOutput | S15 | S6-A03 | `external/stream_json.go` |
| D4-S16-A01 | LoadAgentConfig | S16 | config | `shared/config/multiagent.go` |

**迁 D7（Out of Scope D4）：**

| Legacy A | 目标 |
|----------|------|
| S10-A01 DelegateOrFallback | D7-S2-A04 DispatchWorker |
| S10-A02 BridgeFlowEvents | D7-S4-A04/A05 SpokeBridge |

---

## Legacy A 层（冻结追溯）

> DM-20260629-004 PR-4 #2 registry-sync：D4-S1~S10 详细 A 表已下沉到 `openspec/archive/2026-06-15-devrix-d4-sa-refine/legacy-s1-s10.md`。本域概览见 `d4-domain.md §Legacy Module Index`。

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 3.2.0 | 2026-06-30 | DM-20260629-004 PR-4 #2 registry-sync：12 个 F 路径全替换到 v2.0d；D4-S1~S10 详细表下沉 archive |
| 3.1.0 | 2026-06-16 | Canonical S11–S16 表 + Legacy 冻结（DM-20260614-018） |

---

## Statistics

| Track | Activities |
|-------|------------|
| Canonical S11–S16 | 13 A |
| Migrated to D7 (Out of Scope) | 2 A |
| Legacy S1–S10 (frozen) | 15 A (archived) |
