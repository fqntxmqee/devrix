# D1 Communication Span 注册表

**Domain:** D1 Communication
**Version:** 2.0.0
**Status:** Active (2026-06-14)
**Canonical Source:** `internal/layers/observability/telemetry/names.go` · `internal/layers/observability/coverage/registry.go`

---

## Operations

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
| IM 入口启动延迟 | P99 < 500ms | `adapter.feishu.outbound` |
| 消息端到端延迟（入站 → 首 thinking） | P99 < 800ms | `gateway.message.receive` |

---

## 关联文档

- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
