# LLM Gateway Layer - Tasks

**Change ID:** devrix-llm-gateway
**Demand:** DM-20260607-004
**Layer:** 3
**Status:** S7 Archived (2026-06-07)
**Version:** 1.0.0

---

## 前置条件

- [x] V1 `devrix-context-engine` 已归档，master 基线绿
- [x] `devrix-observability` Bridge 可用（可选 NoOp）
- [x] 与 `devrix-context-engine-v2` 对齐 `shared/contracts/tokencounter.go`（T0.1 完成；T0.2 移交 CE V2）

---

## Provider 配置

| Provider | Adapter | 示例模型 |
|----------|---------|----------|
| `deepseek` | DeepSeekAdapter | deepseek-v4-pro, deepseek-v4-flash |
| `minimax` | MiniMaxAdapter | minimax-2.7-highspeed, minimax-3 |

---

## 任务分解

### Milestone 0: 跨 Change 握手

| Task | 描述 | L4 | L5 | 预估 | 依赖 |
|------|------|----|----|------|------|
| T0.1 | 新增 `shared/contracts/tokencounter.go`（与 CE V2 T1 同契约） | L4-LLM-TOKEN | L5-LLM-07 | 1h | — |
| T0.2 | CE V2 确认 `HeuristicCounter` 实现同一接口（联调 PR 或并行分支） | L4-CTX-STATE | L5-CTX-16 | 1h | T0.1 |

### Milestone 1: 基础框架 + Token Counter

| Task | 描述 | L4 | L5 | 预估 | 依赖 |
|------|------|----|----|------|------|
| T1.1 | 创建 `llmgateway/` 目录结构 | L4-LLM-GATEWAY | — | 0.5h | — |
| T1.2 | 实现 `contracts.go`（Request/Chunk/IGateway） | L4-LLM-GATEWAY | — | 1h | — |
| T1.3 | 实现 `token/counter.go`（`contracts.ITokenCounter`，cl100k_base） | L4-LLM-TOKEN | L5-LLM-07, L5-LLM-08 | 3h | T0.1, T1.2 |
| T1.4 | 单元测试 `token/counter_test.go` | L4-LLM-TOKEN | L5-LLM-07, L5-LLM-08 | 2h | T1.3 |
| T1.5 | 实现 `shared/errors/llm.go` | L4-LLM-GATEWAY | — | 1h | T1.2 |

### Milestone 2: 配置加载 + Router + Adapter Registry

| Task | 描述 | L4 | L5 | 预估 | 依赖 |
|------|------|----|----|------|------|
| T2.1 | 实现 `shared/config/llmgateway.go` + `config/loader.go` | L4-LLM-CONFIG | L5-LLM-09 | 2h | T1.2 |
| T2.2 | 实现 `gateway/router.go`（model_routing + default） | L4-LLM-GATEWAY | L5-LLM-16 | 2h | T2.1 |
| T2.3 | 实现 `adapter/registry.go` | L4-LLM-ADAPTER | — | 2h | T1.2 |
| T2.4 | 单元测试 `config/loader_test.go` | L4-LLM-CONFIG | L5-LLM-09 | 1h | T2.1 |
| T2.5 | 单元测试 `gateway/router_test.go` | L4-LLM-GATEWAY | L5-LLM-16 | 1h | T2.2 |
| T2.6 | 更新 `devrix.yaml` `llm_gateway` 段 | L4-LLM-CONFIG | L5-LLM-09 | 1h | T2.1 |

### Milestone 3: Circuit Breaker

| Task | 描述 | L4 | L5 | 预估 | 依赖 |
|------|------|----|----|------|------|
| T3.1 | 实现 `breaker/circuit_breaker.go` + `state.go` | L4-LLM-BREAKER | L5-LLM-03~06 | 3h | T1.2 |
| T3.2 | 单元测试 `breaker/circuit_breaker_test.go` | L4-LLM-BREAKER | L5-LLM-03~06 | 3h | T3.1 |
| T3.3 | 集成测试 `tests/integration/llm_circuit_breaker_test.go` | L4-LLM-BREAKER | L5-LLM-03~06 | 2h | T3.2 |

### Milestone 4: DeepSeek Adapter

| Task | 描述 | L4 | L5 | 预估 | 依赖 |
|------|------|----|----|------|------|
| T4.1 | 实现 `adapter/adapter.go` + `sse_parser.go` | L4-LLM-ADAPTER | — | 3h | T1.2 |
| T4.2 | 实现 `adapter/deepseek.go`（httptest mock server） | L4-LLM-ADAPTER | L5-LLM-01 | 4h | T4.1 |
| T4.3 | 单元测试 `adapter/deepseek_test.go` | L4-LLM-ADAPTER | L5-LLM-01 | 3h | T4.2 |

### Milestone 5: MiniMax Adapter

| Task | 描述 | L4 | L5 | 预估 | 依赖 |
|------|------|----|----|------|------|
| T5.1 | 实现 `adapter/minimax.go` | L4-LLM-ADAPTER | L5-LLM-02 | 4h | T4.1 |
| T5.2 | 单元测试 `adapter/minimax_test.go` | L4-LLM-ADAPTER | L5-LLM-02 | 3h | T5.1 |

### Milestone 6: Gateway + Retry + Bridge

| Task | 描述 | L4 | L5 | 预估 | 依赖 |
|------|------|----|----|------|------|
| T6.1 | 实现 `retry/retry.go`（含 fallback_model） | L4-LLM-RETRY | L5-LLM-12 | 2h | T1.2 |
| T6.2 | 单元测试 `retry/retry_test.go` | L4-LLM-RETRY | L5-LLM-12 | 2h | T6.1 |
| T6.3 | 实现 `gateway/gateway.go`（编排 + Bridge 埋点） | L4-LLM-GATEWAY | — | 4h | T2.3, T3.1, T4.1, T6.1 |
| T6.4 | 实现 `bridges/llm/bridge.go` | L4-LLM-GATEWAY | — | 2h | T6.3 |
| T6.5 | 单元测试 `gateway/gateway_test.go` | L4-LLM-GATEWAY | L5-LLM-14 | 3h | T6.3 |
| T6.6 | 集成测试 `tests/integration/llm_fallback_test.go` | L4-LLM-ADAPTER | L5-LLM-10, L5-LLM-11 | 3h | T6.3 |
| T6.7 | 集成测试 `tests/integration/llm_observer_test.go` | L4-LLM-OBS | L5-LLM-13 | 2h | T6.3 |
| T6.8 | `cmd/devrix/main.go` Mock → Bridge + Gateway | L4-LLM-GATEWAY | — | 2h | T6.4 |

### Milestone 7: 验收与归档准备

| Task | 描述 | L4 | L5 | 预估 | 依赖 |
|------|------|----|----|------|------|
| T7.1 | 确认 `l5-registry.md` 全部 LLM L5 路径与 Status | — | L5-LLM-* | 1h | All |
| T7.2 | 生成 `acceptance-report.md` | — | P0 全绿 | 2h | All |
| T7.3 | S7 合并 `openspec/specs/llm_gateway_layer_delta.md` → canonical | — | — | 1h | T7.2 |

---

## 工时汇总

| Milestone | 任务数 | 工时 |
|-----------|--------|------|
| M0: 跨 Change | 2 | 2h |
| M1: Token + 框架 | 5 | 7.5h |
| M2: 配置 + Router | 6 | 9h |
| M3: Circuit Breaker | 3 | 8h |
| M4: DeepSeek | 3 | 10h |
| M5: MiniMax | 2 | 7h |
| M6: Gateway + Bridge | 8 | 18h |
| M7: 验收 | 3 | 4h |
| **总计** | **32** | **65.5h** |

---

## 风险识别

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| API 兼容性问题 | 高 | httptest mock server + fixture |
| Token 计数误差 | 中 | 基准样例集；L5-LLM-07 |
| 熔断器状态竞争 | 中 | mutex 保护 |
| MiniMax API 文档缺失 | 高 | 复用 sse_parser；DeepSeek 先行 |
| CE V2 契约漂移 | 高 | T0 联调 `ITokenCounter` |

---

## 完成记录

- [x] **M0–M6** 全部实现并通过单元/集成测试
- [x] **T7.1** `l5-registry.md` L5-LLM-01~14、16 → IMPLEMENTED
- [x] **T7.2** `acceptance-report.md` verdict ACCEPTED
- [x] **T7.3** delta 合并至 `openspec/specs/llm-gateway/spec.md`
- [ ] **L5-LLM-15** 熔断器状态持久化 — 移交 V2 backlog
