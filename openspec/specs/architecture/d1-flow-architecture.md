# D1 Communication Flow Architecture

**Capability:** d1-flow-architecture
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-30
**Domain SoT:** `../specs/d1-communication/d1-domain.md`

---

## Architecture（价值流 — v2.0 实现）

```
[User IM]
  → S17 Parse* → S13 Accept → Persist → Dispatch (D7|Agent)
  → Agent events → S18 EventBus → present/ (S14|S15|S16)
  → S17 Encode* → [User IM]
  S18 overlay: Critical Conclusion 永不 Drain
```

## Package Map

> **目标布局** 见 `code-layout.md` §5–§6。下表为当前路径速查。

| scenario-slug | 当前路径 | Canonical S |
|---------------|----------|-------------|
| `capture` | `capture/`（原 `gateway/`）, `signal/` | S13 |
| `thinking` / `taskprogress` / `conclusion` | `present/`（已拆分） | S14–S16 |
| `channel` | `adapters/`, `connection/`, `instance/`, `ratelimit/`, `renderers/` | S17 |
| `delivery` | `eventbus/` | S18 |
| `kernel` | `core/` | Domain Kernel |

## Architecture（Legacy 包结构 — RETIRED v2.0）

```
User/IM ──→ Adapters (D1-S2) ──→ Gateway (D1-S1) ──→ Context Engine (D2)
  │            │  │                    │                    │
  │  Feishu    │  ├─ CardKit 流式      ├─ AgentRoute ──→ D4 MultiAgent
  │  DingTalk  │  ├─ Worker 卡片       ├─ EventDispatcher
  │  CLI       │  └─ Session Resolve   ├─ PermissionManager
  │            │                       ├─ SessionStore (File)
  └─ Renderers (D1-S8) ←── EventBus (D1-S9) ←─┘
       │                         │
       ├─ CLIRenderer            ├─ Drain/Compact/Reconnect
       ├─ DingTalkCardRenderer   └─ Priority (Critical/Normal/Low)
       └─ Components
```

## 跨域接线

| From | To | 接线 | 守卫 |
|------|----|------|------|
| D1-S13-A03-F02 routeD7 | D7 IOrchestrationEntry.ProcessMessage | composition root (`bootstrap/sessionagents`) | D1 capture import boundary + `lint-d1-imports.sh` |
| D1-S13-A04 ResolvePermissionGate | D4 `sessionagents/manager.ResolveAgentPermission` | bootstrap wire | D1 capture lint |
| D1-S14/S15/S16 EmitOutboundSignal | D7 orchestrator (signal consumer) | EventBus channel | 双向命名空间 |
| D1 capture ↔ D5 observability | `Observability`, `Bridge`, tracer, telemetry | import (allowed) | — |
| D1 channel/adapters | `core.Card` + `CardBuilder` 平台无关模型 | renderers / adapters | D7 import forbidden (`lint-d1-imports.sh`) |

## Legacy 路径索引（RETIRED — 仅追溯）

> 已废弃的 D1-S1–S12 包映射见 `../specs/d1-communication/t-registry.md` §Legacy Archive。完整 frozen index 见 `openspec/archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md`。新代码禁止使用下表路径。

| 旧子包 | 原 Legacy S | 目标 scenario-slug |
|--------|-------------|-------------------|
| `gateway/` | S1 | `capture/` |
| `adapters/` | S2 | `channel/` |
| `eventbus/` | S9 | `delivery/` |
| `core/` | S11 | `kernel/` |

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-30 | 初版：从 `spec.md` 拆出（DM-20260629-005 PR-2 #1 god-doc-split pt1），保留 Architecture（价值流）+ Package Map + Legacy 包结构 + 跨域接线 + Legacy 路径索引 |