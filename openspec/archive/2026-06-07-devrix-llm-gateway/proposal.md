# Proposal: LLM Gateway Layer V1

**Change ID:** devrix-llm-gateway
**Layer:** 3 - LLM Gateway
**Type:** New Capability
**Status:** S7 Archived
**Based on:** `openspec/specs/llm_gateway_layer_delta.md`, `devrix-context-engine` (V1 archived)
**Demand:** DM-20260607-004

---

## Problem Statement

Devrix 六层架构中 Layer 3 尚未实现，导致：

1. **主路径仍用 Mock LLM** — `cmd/devrix/main.go` 无法接通真实模型
2. **Token 计数分裂** — L2 使用 char/4 启发式，压缩触发时机不可靠
3. **无 Provider 抽象** — 无法切换 DeepSeek / MiniMax，无熔断与重试保护
4. **阻塞 Context Engine V2** — Autocompact 与 Token 统一硬依赖本层

## Proposed Solution

| 能力 | V1 方案 |
|------|---------|
| 模型适配 | DeepSeek + MiniMax，OpenAI-compatible SSE 解析 |
| 网关编排 | Token 预算检查 → Circuit Allow → Retry（含 fallback）→ Stream |
| Token 统一 | `shared/contracts.ITokenCounter`，cl100k_base 实现 |
| L2 接线 | `internal/bridges/llm` 适配 `contextengine.ILLMGateway` |
| 可观测 | 注入 `observability.Bridge`，`devrix_llm_*` metrics + span |

## Goals

| Goal | 现状 | V1 目标 |
|------|------|---------|
| 真实 LLM 调用 | Mock | DeepSeek + MiniMax 流式 |
| Token 计数 | L2 启发式 | Gateway cl100k_base ±5% |
| 故障保护 | 无 | Circuit Breaker + Retry |
| Provider 路由 | 无 | model_routing + default_provider |
| CE V2 解锁 | 阻塞 | M1+M3 交付后可开工 |

## Capabilities

| Capability | L4 映射 | 说明 |
|------------|---------|------|
| llm-gateway-core | L4-LLM-GATEWAY | ChatStream 编排、Provider 路由 |
| provider-adapter | L4-LLM-ADAPTER | DeepSeek / MiniMax SSE 适配 |
| circuit-breaker | L4-LLM-BREAKER | 按 provider 熔断 |
| token-counter | L4-LLM-TOKEN | 实现 `contracts.ITokenCounter` |
| config-loader | L4-LLM-CONFIG | YAML + 环境变量 API Key |
| retry-fallback | L4-LLM-RETRY | 指数退避 + fallback_model |
| llm-observability | L4-LLM-OBS | Bridge span/metrics |
| llm-bridge | L4-LLM-GATEWAY | L3→L2 类型适配（bridges 包） |

## Alternatives Considered

| 方案 | 结论 |
|------|------|
| L3 直接 import `contextengine` 实现接口 | 拒绝 — 违反层边界 |
| `ILLMGateway` 下沉 `shared/contracts` | 拒绝 V1 — 改动 L2 面过大；用 Bridge |
| Gateway 填充 RiskLevel | 拒绝 — L2 已实现 fallback |
| 独立 ILLMObserver 包 | 拒绝 — 复用 Observability Bridge |
| V1 含 Anthropic 适配器 | 拒绝 — 归 V2 |

## Impact

| 组件 | 变更 |
|------|------|
| `internal/layers/llmgateway/` | **新增** 完整 Layer 3 |
| `internal/shared/contracts/tokencounter.go` | **新增** 共享契约 |
| `internal/shared/config/llmgateway.go` | **新增** 配置结构 |
| `internal/bridges/llm/bridge.go` | **新增** `contextengine.ILLMGateway` 适配 |
| `internal/shared/errors/llm.go` | **新增** LLM 错误码 |
| `devrix.yaml` | +`llm_gateway` 段 |
| `cmd/devrix/main.go` | Mock → Bridge + Gateway（S4 末） |
| `openspec/l5-registry.md` | L5-LLM-01~16 路径校正 |
| `openspec/specs/llm_gateway_layer_delta.md` | Fallback/Circuit 语义修正 |

## Scope

**In Scope:** 见 `demand.md` §3.2

**Out of Scope:** ChatComplete、HealthCheck、Anthropic/OpenAI、Rate Limiter、熔断持久化

## Dependencies

```
devrix-context-engine (V1 archived)
        │
        ▼
devrix-llm-gateway (本变更) ──shared/contracts.ITokenCounter──┐
        │                                                      │
        ▼                                                      ▼
devrix-context-engine-v2                              devrix-observability
```

**阻塞关系:** Context Engine V2 等待本变更 M1（Token）+ M3–M6（ChatStream）。

## Success Criteria (S3 准出)

- [x] demand / proposal / design / specs / tasks 四件套完整
- [x] L5-LLM-01 ~ L5-LLM-16 已登记 `l5-registry.md`
- [x] 跨层决议 Q1~Q10 已闭合（见 demand.md）
- [x] Bridge 模式、ITokenCounter、model_routing 已写入 design
- [x] Retry 与 Circuit Breaker 职责分离
- [x] 测试路径符合 `testing-framework/spec.md`（`internal/**/*_test.go`）

## Risks

| 风险 | 缓解 |
|------|------|
| MiniMax API 文档不全 | httptest mock + DeepSeek 先行 |
| Token 计数误差 | 基准样例集 ±5%；L5-LLM-07 |
| SSE tool_calls 增量解析 | 单独 parser 模块 + fixture 测试 |
| API Key 缺失 | 启动 fail-fast + `LLM_AUTH_1004` |

## Timeline (估算)

| 阶段 | 工期 |
|------|------|
| S3 规划（本 PR） | 1d |
| S4 实现（M1–M7） | 8–9d |
| S5 验收 | 2d |

---

## Archive Information

**Archived:** 2026-06-07
**Duration:** 1 day (2026-06-07)
**Outcome:** Successfully implemented (V1)

### Files Modified
- `internal/layers/llmgateway/` — Gateway, adapters, breaker, retry, token counter
- `internal/bridges/llm/` — L3→L2 Bridge + WireFromConfig
- `internal/shared/contracts/tokencounter.go`, `shared/config/llmgateway.go`, `shared/errors/llm.go`
- `cmd/devrix/main.go`, `cmd/devrix-feishu/main.go` — Mock → real Gateway wiring
- `tests/integration/llm_*_test.go` — fallback, circuit breaker, observer

### Specs Updated
- `openspec/specs/llm-gateway/spec.md` — canonical Layer 3 specification
- `openspec/specs/llm_gateway_layer_delta.md` — merged reference
- `openspec/l5-registry.md` — L5-LLM-01~14, L5-LLM-16 IMPLEMENTED

### Acceptance
- Verdict: **ACCEPTED** (see `acceptance-report.md`)
- Demand: DM-20260607-004
