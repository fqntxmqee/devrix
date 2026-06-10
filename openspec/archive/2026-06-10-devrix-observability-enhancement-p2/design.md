# Design: devrix-observability-enhancement-p2

## Delivered

| Component | Location |
|-----------|----------|
| SpanKind integration tests | `tests/integration/obs_pev_span_hierarchy_test.go` |
| Prompt version + template hash | `harness/system_prompt_assembler.go`, `engine.go` |
| gen_ai.client.token.usage | `observability/genai_tokens.go`, gateway + PEV |
| devrix debug export | `internal/cli/debug/export.go`, `cmd/devrix/main.go` |

## Deferred

- Baggage propagation
- cache_read / reasoning token types (no provider fields yet)
- docs/observability-design.md Canonical Trace Tree refresh
