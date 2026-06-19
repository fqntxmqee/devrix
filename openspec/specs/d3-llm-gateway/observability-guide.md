# D3 LLM Gateway — 可观测性与验收指南

**Capability:** llm-gateway
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-16
**Parent:** `d3-domain.md` · `span-registry.md` · `t-registry.md`
**Complements:** `terminal-state-guide.md` · `../d7-orchestration/observability-guide.md`

---

## 0. 文档定位

| 本文 | SoT |
|------|-----|
| 5 个稳定 runtime span | `span-registry.md`（**禁止改名**） |
| D7 Invoke Trace 树 | `telemetry/names.go` |
| T 摘要 + P0 Runbook | `t-registry.md`（35 T，19 P0） |

---

## 1. Canonical Span ↔ S 绑定

| Operation | S | Kind |
|-----------|---|------|
| `llm.provider.route` | S1 | INTERNAL |
| `llm.stream` | S2 | CLIENT |
| `llm.adapter.stream` | S2 | CLIENT |
| `llm.circuit_breaker` | S3 | INTERNAL |
| `llm.retry` | S3 | INTERNAL |

**v1.1 增量（非 span）：**

| 资产 | S | 类型 |
|------|---|------|
| `llm_breaker_state` | S3 | metric |
| `llm_breaker_transitions_total` | S3 | metric |
| `llm_tier_resolve_total` | S1 | metric |
| `safety.check.duration_ms` | S5 | span event |
| `flow.breaker.*` | S3 | EngineEvent ×3 |

---

## 2. Trace 树

### 2.1 D7 Invoke（Canonical）

```text
D7_Orchestration_LLM_Invoke
├── llm.provider.route          (S1)
└── llm.stream                  (S2)
    ├── safety.check.duration_ms (S5 event)
    ├── llm.circuit_breaker     (S3)
    │   └── llm.retry
    │       └── llm.adapter.stream
    └── (budget — S4 无独立 span)
```

### 2.2 Bridge 路径（D3-X CROSS）

```text
llm.stream (bridge root)
└── llm.adapter.stream
```

---

## 3. T 层验收矩阵（按 S 摘要）

| S | P0 重点 | 代表 T |
|---|---------|--------|
| S1 | Tier/Provider 解析 | D3-S1-A01-T01/T02 |
| S2 | 双 Adapter 流式 + Protocol BREAKING | D3-S2-A01-T01/T02/T06 |
| S3 | Breaker 四态 + Retry 耗尽 | D3-S3-A01-T01~T04, T09 |
| S4 | 预算超限 | D3-S4-A01-T01~T03 |
| S5 | critical reject / warning | D3-S5-A01-T01/T02 |
| S6 | config fail-fast | D3-S6-A01-T01 |

### P0 必跑清单

```bash
# Route
go test ./internal/layers/llmgateway/route/ -v

# Stream + Adapter BREAKING
go test ./internal/layers/llmgateway/stream/adapter/ -v

# Protect (Breaker + Retry)
go test ./internal/layers/llmgateway/protect/ -v

# Budget + Guard
go test ./internal/layers/llmgateway/budget/ -v
go test ./internal/layers/llmgateway/guard/ -v

# Configure fail-fast
go test ./internal/layers/llmgateway/configure/ -v
```

---

## 4. 生产检查清单

| 检查项 | 期望 |
|--------|------|
| `llm.stream` 仅在 D7 Invoke 子树 | 无 D2 独立 `query.loop.llm.call` 新路径 |
| Breaker metric 有 provider label | v1.1 emit ON |
| obs nil 拒绝启动 | ConfigureGateway T 绿 |
| span 字面量未改名 | 对照 span-registry §0 |

---

## 5. 相关文档

| 文档 | 关系 |
|------|------|
| `span-registry.md` | operation 全表 |
| `t-registry.md` | T 全表 + Legacy alias |
| `terminal-state-guide.md` | Invoke 时序 |
