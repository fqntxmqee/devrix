# Devrix L5 测试点注册表

**Status:** Active
**Last Updated:** 2026-06-07

> L5 测试点是 OpenSpec S5 验收的确定性锚点。新增 L4 能力前 MUST 先在此登记或复用已有 L5。

---

## 登记规则

| 字段 | 说明 |
|------|------|
| ID | `L5-{LAYER}-{NN}`，LAYER = COMM / CTX / LLM / AGENT / OBS / EVO |
| Priority | P0（阻断交付）/ P1（需执行，失败记例外）/ P2（尽力） |
| L4 映射 | 关联的功能点 ID |
| Test 位置 | 测试文件路径 |
| Status | PLANNED / IMPLEMENTED / DEPRECATED |

---

## Layer 1: Communication (COMM)

### P0

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-COMM-01 | 运行 devrix 创建 CLI 会话 | L4-COMM-CLI | `tests/integration/gateway_session_test.go` | IMPLEMENTED |
| L5-COMM-02 | 空消息入站被拒绝 | L4-COMM-GW | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED |
| L5-COMM-03 | 会话空闲超时后拒绝消息 | L4-COMM-STORE | `tests/integration/gateway_session_test.go` | IMPLEMENTED |
| L5-COMM-04 | `/new` 命令解析正确 | L4-COMM-CMD | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED |
| L5-COMM-05 | `/help` 命令解析正确 | L4-COMM-CMD | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED |
| L5-COMM-06 | `/stop` 命令解析正确 | L4-COMM-CMD | `tests/acceptance/p0/comm_commands_test.go` | IMPLEMENTED |
| L5-COMM-07 | 权限请求超时自动拒绝 | L4-COMM-PERM | `tests/integration/permission_flow_test.go` | IMPLEMENTED |
| L5-COMM-08 | 入站消息经 Engine 产生出站响应 | L4-COMM-GW | `tests/acceptance/p0/comm_gateway_flow_test.go` | IMPLEMENTED |
| L5-COMM-17 | Milestone 循环依赖被拒绝 | L4-COMM-MS | `internal/layers/communication/milestone/service_test.go` | IMPLEMENTED |
| L5-COMM-18 | TaskFlow 按依赖顺序执行 | L4-COMM-TF | — | PLANNED |

### P1

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-COMM-09 | ShortId 唯一且排除歧义字符 | L4-COMM-ID | `internal/shared/types/shortid_test.go` | IMPLEMENTED |
| L5-COMM-10 | 无效 Token 返回 401 | L4-COMM-AUTH | — | PLANNED |
| L5-COMM-11 | 飞书消息解析正确 | L4-COMM-FEISHU | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED |
| L5-COMM-12 | 限流超限拒绝请求 | L4-COMM-RL | `internal/layers/communication/ratelimit/limiter_test.go` | IMPLEMENTED |
| L5-COMM-19 | 飞书消息去重 | L4-COMM-FEISHU | `internal/layers/communication/adapters/feishu_test.go` | IMPLEMENTED |
| L5-COMM-20 | UI 组件跨平台渲染 | L4-COMM-UI | — | PLANNED |

### P2

| L5 ID | 描述 | L4 映射 | Test 位置 | Status |
|-------|------|---------|-----------|--------|
| L5-COMM-21 | Prometheus `/metrics` 可访问 | L4-COMM-METRICS | — | PLANNED |
| L5-COMM-22 | 多实例注册与健康检查 | L4-COMM-INST | — | PLANNED |
| L5-COMM-23 | 钉钉 Adapter 消息收发（Mock） | L4-COMM-DING | — | PLANNED |

### Live Only（不阻断 PR）

| L5 ID | 描述 | 环境变量 | Status |
|-------|------|----------|--------|
| L5-COMM-L01 | 飞书 WebSocket 真连收发 | `DEVRIX_FEISHU_APP_ID`, `DEVRIX_FEISHU_APP_SECRET` | PLANNED |
| L5-COMM-L02 | 端到端 Stub Engine 回复 | — | PLANNED |

---

## Layer 2–6: 预留

| 前缀 | 层 | 状态 |
|------|-----|------|
| `L5-CTX-*` | Context Engine | 未登记 |
| `L5-LLM-*` | LLM Gateway | 未登记 |
| `L5-AGENT-*` | Multi-Agent | 未登记 |
| `L5-OBS-*` | Observability | 未登记 |
| `L5-EVO-*` | Evolution | 未登记 |

新增层时 MUST 在本表追加对应章节。
