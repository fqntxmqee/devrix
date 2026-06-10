# Design: devrix-observability-enhancement-p1

**Parent:** `openspec/archive/2026-06-10-devrix-observability-enhancement/design.md` §6–§7

## Delivered (P1)

| Component | Location |
|-----------|----------|
| `tool_latency` Histogram | `observability/bridge.go` → `pev_engine.go` |
| `compression_ratio` Histogram | `contextengine/engine.go` |
| Compression decision attrs | `context.compression.run` span |
| `RecordSpanError` | `telemetry/span_error.go` |
| Incident export v1 | `observability/incident/export.go`, `cmd/debug-export` |

## Deferred

- SpanKind audit (T3b)
- Prompt version hash (T3c)
- `gen_ai.client.token.usage` Counter
- `devrix debug export` subcommand in main binary
