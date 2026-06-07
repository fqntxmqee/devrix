---
demand-id: DM-20260607-004
title: LLM Gateway Layer V1（DeepSeek + MiniMax + 熔断 + Token 统一）
source: 架构/产品
priority: P0
status: S7_ARCHIVED
l1-domain: devrix
created: 2026-06-07
---

# LLM Gateway Layer V1

## 1. 原始描述

> Devrix 当前主路径仍注入 Mock LLM（`contextengine/mock/llm.go`），Context Engine V2 硬依赖真实 Layer 3：
> 统一 Token 计数（cl100k_base）、流式 `ChatStream`、多 Provider 适配与熔断保护。
>
> 按 OpenSpec 规范，为 LLM Gateway Layer（Layer 3）补齐 demand → proposal → design → specs → tasks，
> 解除 Context Engine V2 与生产接线阻塞。

## 2. 澄清记录

### Q1: V1 支持哪些 Provider？

**A**: DeepSeek、MiniMax（OpenAI-compatible SSE）；Anthropic/OpenAI 归 V2。 — 2026-06-07

### Q2: `ILLMGateway` 接口定义在哪？

**A**: 保留在 `contextengine/contracts.go`（L2 消费方定义）。L3 **不 import** `contextengine/`；通过 `internal/bridges/llm` 薄适配实现接口。 — 2026-06-07

### Q3: `ToolCall.RiskLevel` 由谁填充？

**A**: **L2 Context Engine**（`pev_engine.go` 已有 fallback：`toolsReg.RiskLevel`）。L3 只返回 `{ID, Name, Input}`，不注入 `IToolRegistry`。 — 2026-06-07

### Q4: Provider 如何路由（`LLMRequest` 无 Provider 字段）？

**A**: 路由优先级：`req.Model` 前缀匹配 `model_routing` → `providers[provider].default_model`（model 为空时）→ `llm_gateway.default_provider`。Bridge 在调用前解析 provider。 — 2026-06-07

### Q5: Fallback 与 Circuit Breaker 职责？

**A**: **Retry 策略**负责指数退避 + `fallback_model` 切换；**Circuit Breaker** 仅负责失败率保护与快速拒绝（`LLM_CIRCUIT_1002`）。二者独立，禁止「熔断触发 fallback」表述。 — 2026-06-07

### Q6: `ITokenCounter` 接口归属？

**A**: 定义于 `internal/shared/contracts/tokencounter.go`，L2/L3 共同依赖；Gateway `token/counter.go` 为权威实现。 — 2026-06-07

### Q7: `ChatComplete` 是否 V1 范围？

**A**: V1 **Out of Scope**；L2 仅使用 `ChatStream`（含 Autocompact 摘要）。 — 2026-06-07

### Q8: 可观测性如何集成？

**A**: 注入 `*observability.Bridge`（span + `devrix_llm_*` metrics），不新建独立 Observer 包；与 Observability Layer 命名一致。 — 2026-06-07

### Q9: Health Check 是否 V1？

**A**: V1 **Out of Scope**（P2 / V1.1）；启动时仅校验配置与 API Key 环境变量存在性。 — 2026-06-07

### Q10: 与 Context Engine V2 的阻塞关系？

**A**: CE V2 M1 依赖 Gateway M1（`ITokenCounter`）；CE V2 M2+ 依赖 Gateway M3–M6（`ChatStream` + Adapter）。Gateway 应优先开发。 — 2026-06-07

## 3. 澄清范围

### 3.1 L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | devrix | 开发大脑 | 已有 |
| L2 | L2-DEVRIX-02 | 对话式开发助手 | 已有 |
| L3-BE | L3-BE-LLM-01 | 流式 LLM 对话调用 | **新增** |
| L3-BE | L3-BE-LLM-02 | Token 预算与计数 | **新增** |
| L4-BE | L4-LLM-GATEWAY | LLM 网关编排 | **新增** |
| L4-BE | L4-LLM-ADAPTER | Provider 适配器 | **新增** |
| L4-BE | L4-LLM-BREAKER | 熔断器 | **新增** |
| L4-BE | L4-LLM-TOKEN | Token 计数器 | **新增** |
| L4-BE | L4-LLM-CONFIG | 配置加载 | **新增** |
| L4-BE | L4-LLM-RETRY | 重试与 Fallback | **新增** |
| L4-BE | L4-LLM-OBS | LLM 可观测埋点 | **新增** |
| L5 | L5-LLM-01 ~ L5-LLM-16 | 见 `l5-registry.md` | 草拟 |

### 3.2 范围

**In Scope（本变更）**:
- OpenSpec 四件套 + L5 登记
- `shared/contracts/tokencounter.go` + Gateway TokenCounter（cl100k_base）
- DeepSeek / MiniMax Adapter（OpenAI-compatible SSE）
- Circuit Breaker（默认 scope: `provider`）
- Retry + `fallback_model`
- `model_routing` + Provider 配置加载
- `internal/bridges/llm` 实现 `contextengine.ILLMGateway`
- `devrix.yaml` `llm_gateway` 配置段
- Observability Bridge 埋点

**Out of Scope（本变更）**:
- `ChatComplete` 非流式接口
- `IHealthCheck` HTTP 探活
- Anthropic / OpenAI 适配器（V2）
- Rate Limiter（V2）
- 熔断器状态持久化（L5-LLM-15，P2）
- Circuit breaker scope `provider:model`（V1.1 配置项预留）

### 3.3 下游依赖方

| 变更 | 依赖 Gateway 能力 | 说明 |
|------|-------------------|------|
| `devrix-context-engine-v2` | M1 TokenCounter + M3–M6 ChatStream | 硬依赖 |
| `devrix-observability` | Bridge 注入点 | 已就绪，并行 |
