# Design: devrix-observability-baggage

## 方案

1. 将 `BaggageManager` 迁入 `tracer` 包（避免 propagator ↔ observability 循环依赖）
2. 使用 W3C **`baggage`** 头（非 `tracestate`）
3. Gateway `startInboundSpan` 后写入 `session.id` / `user.id`
4. `CLIAgentTool.ensureSession` 通过 `PropagationEnvVars(ctx)` 注入子进程环境

## Baggage Keys

| Key | 来源 |
|-----|------|
| `session.id` | InboundMessage.SessionID |
| `user.id` | InboundMessage.UserID（非空时） |

## 子进程环境变量

| 变量 | 对应 W3C 头 |
|------|-------------|
| `TRACEPARENT` | traceparent |
| `TRACESTATE` | tracestate |
| `BAGGAGE` | baggage |
