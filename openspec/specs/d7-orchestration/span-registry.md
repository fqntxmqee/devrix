# D7 Orchestration Span 注册表

**Domain:** D7 Orchestration
**Version:** 2.0.0
**Status:** Active (2026-06-14)
**Canonical Source:** `internal/layers/observability/instrument/telemetry/names.go` · `internal/layers/observability/diagnose/coverage/registry.go`

---

## Operations

### Orchestrator（5 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `orchestration.session.process` | INTERNAL | orchestrator | 2.1.0 | session_id, message.len, orchestration.route |
| `orchestration.intent.classify` | INTERNAL | orchestrator | 2.1.0 | orchestration.intent.kind, orchestration.intent.confidence, orchestration.classify.source, orchestration.command |
| `orchestration.wave.schedule` | INTERNAL | orchestrator | 2.1.0 | session_id, wave_id |
| `orchestration.wave.task.execute` | INTERNAL | orchestrator | 2.1.0 | task_id, worker_type |
| `orchestration.flow.event.publish` | INTERNAL | orchestrator | 2.1.0 | event_kind, worker_id, source |

---

## Trace Tree

```
gateway.message.receive
└── orchestration.session.process
    ├── orchestration.intent.classify
    └── context.process
        └── query.loop.run
            └── query.loop.turn
                └── query.loop.llm.call

orchestration.wave.schedule
└── orchestration.wave.task.execute
    └── orchestration.flow.event.publish
```

---

## 命名规范

| 场景 | 格式 | 示例 |
|------|------|------|
| Span Operation | `orchestration.{module}.{action}` | `orchestration.wave.schedule` |

---

## 关联文档

- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
