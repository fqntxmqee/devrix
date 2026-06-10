# Proposal: Devrix 可观察层增强 P1

**Change ID:** devrix-observability-enhancement-p1
**Demand ID:** DM-20260610-002
**Parent:** devrix-observability-enhancement (P0 archived)
**Status:** In Progress

## Capabilities

| Capability | L4 | Priority |
|------------|-----|----------|
| tool_latency histogram | observability.metrics.tool_latency | P0 |
| compression_ratio histogram | observability.metrics.compression_ratio | P0 |
| compression decision attrs | observability.trace.compression_attrs | P1 |
| session incident export | observability.debug.export | P1 |

## Success Criteria

- [ ] `devrix_tool_latency{tool,risk_level,status}` 可 scrape
- [ ] `devrix_compression_ratio` 可 scrape
- [ ] `context.compression.run` span 含 trigger_reason + ratio
- [ ] `cmd/debug-export` 输出 schema v1 JSON bundle
