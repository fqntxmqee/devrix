# D1 Communication Span 注册表

**Domain:** D1 Communication
**Version:** 3.0.0
**Status:** Active (2026-06-14)
**Change:** DM-20260614-006 — 切法 A
**Canonical Source:** `internal/layers/observability/telemetry/names.go` · `internal/layers/observability/coverage/registry.go`

---

## Canonical Operations（v1.1 落地）

| Operation | Kind | S | Bind T | Key Attributes |
|-----------|------|---|--------|----------------|
| `d1.capture.persist` | INTERNAL | S13 | D1-S13-A02-T01 | session_id, adapter |
| `d1.dispatch.route` | INTERNAL | S13 | D1-S13-A03-T01/T02 | route_target (d7/legacy/agent) |
| `d1.signal.thinking` | INTERNAL | S14 | D1-S14-A01-F01-T01 | session_id, sequence |
| `d1.signal.task` | INTERNAL | S15 | D1-S15-A02-F01-T01 | session_id, tool_name, worker_id |
| `d1.signal.conclusion` | INTERNAL | S16 | D1-S16-A02-T01 | session_id, is_terminal |
| `d1.signal.chain_integrity` | INTERNAL | S14–S16 | D1-S16-A02-T01 | session_id, break_at_kind |
| `d1.signal.task.work_proof` | INTERNAL | S15 | D1-S15-A02-F01-T01 | tool_name, worker_id, linked_conclusion |
| `eventbus.publish_critical` | INTERNAL | S18 | D1-S18-A01-F02-T01 | event_type (complete/error) |
| `eventbus.drain` | INTERNAL | S18 | D1-S18-A01-F03-T01 | drained_count |
| `adapter.feishu.encode` | CLIENT | S17 | D1-S17-A01-T01 | signal_kind |

---

## Legacy Operations（保留至 v2.0）

### Gateway（11 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `gateway.message.receive` | SERVER | gateway | 1.2.0 | session_id, adapter |
| `gateway.session.lifecycle` | INTERNAL | gateway | 1.3.0 | session_id |
| `gateway.session.create` | INTERNAL | gateway | 2.0.0 | session_id |
| `gateway.session.get` | INTERNAL | gateway | 2.0.0 | session_id |
| `gateway.session.expire` | INTERNAL | gateway | 2.0.0 | session_id |
| `gateway.store.create` | INTERNAL | gateway | 2.0.0 | session_id, store_type |
| `gateway.store.get` | INTERNAL | gateway | 2.0.0 | session_id |
| `gateway.store.update` | INTERNAL | gateway | 2.0.0 | session_id |
| `gateway.store.delete` | INTERNAL | gateway | 2.0.0 | session_id |
| `gateway.permission.check` | INTERNAL | gateway | 2.0.0 | tool_name, risk_level, result |
| `gateway.agent.create` | INTERNAL | gateway | 2.0.0 | session_id, mode |
| `gateway.engine_event.handle` | INTERNAL | gateway | 2.0.0 | event_type |

### Adapter（3 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `adapter.message.receive` | SERVER | adapter | 1.3.0 | adapter_type |
| `adapter.cli.send` | CLIENT | adapter | 2.0.0 | — |
| `adapter.feishu.outbound` | CLIENT | adapter | 2.0.0 | — |

---

## 端到端延迟目标

| 指标 | 目标 | 关联 Span |
|------|------|-----------|
| IM 入口启动延迟 | P99 < 500ms | `adapter.feishu.encode` |
| 入站 → 首 thinking | P99 < 800ms | `d1.signal.thinking` |
| complete costly 送达 | P99 < 800ms from inbound | `d1.signal.conclusion` |

---

## 关联文档

- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
