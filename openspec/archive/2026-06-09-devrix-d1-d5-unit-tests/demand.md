# Demand: D1/D5 Unit Test Coverage

**Demand ID:** DM-20260609-003
**Parent:** DM-20260609-002
**Change ID:** devrix-d1-d5-unit-tests

## Goals

1. 补齐 D1 Communication 薄弱模块单测：connection manager、renderers、CLI adapter
2. 补齐 D5 Observability 薄弱模块单测：shutdown、health handlers、trace propagation
3. 提升域单元覆盖率：D1 ≥50%、D5 ≥45%

## Scope

| Package | Tests |
|---------|-------|
| `connection/` | Register、Heartbeat、超时断开、恢复、Stop |
| `renderers/` | CLIRenderer、PermissionRenderer 输出与辅助函数 |
| `adapters/` | CLI 命令、消息路由、权限 prompt、Stop |
| `observability/` | ShutdownManager、Health/Ready/Live HTTP |
| `tracer/` | W3C Inject/Extract、MapCarrier、HTTPHeaderCarrier |

## Success Criteria

- `./scripts/test-unit.sh` 全绿
- D1 单元行覆盖 ≥50%，D5 单元行覆盖 ≥45%
