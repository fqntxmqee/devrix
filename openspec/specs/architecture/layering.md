# Devrix Layer Architecture Specification

**Capability:** architecture-layering
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-08

---

## Overview

本文档定义了 Devrix 的正式分层架构，使用 **L1-L2 二级编号系统**。

---

## Layer 1 (L1) - Top-Level Layers

| L1 ID | Layer Name | Responsibility |
|-------|------------|---------------|
| L1-1 | Communication Layer | IM Gateway, WebSocket, CLI adapter |
| L1-2 | Context Engine Layer | PEV Engine, 7-step compression, layered memory |
| L1-3 | LLM Gateway Layer | Model adapter, circuit breaker, token counter |
| L1-4 | Multi-Agent Layer | Agent lifecycle, fork, collaboration modes |
| L1-5 | Observability Layer | Tracing, metrics, logging |
| L1-6 | Evolution Layer | Version management, A/B testing |

---

## Layer 2 (L2) - Sub-Modules

每个 L1 层内部包含多个 L2 模块，采用 **L1-N-L2-M** 格式编号。

### L1-1 Communication Layer Modules

| L2 ID | Module Name | Responsibility |
|-------|------------|---------------|
| L1-1-L2-1 | Gateway Module | 消息网关、路由、会话管理 |
| L1-1-L2-2 | Adapters Module | 飞书、WebSocket、CLI 适配器 |
| L1-1-L2-3 | Commands Module | CLI 命令解析 (/new, /stop, /help) |
| L1-1-L2-4 | Auth Module | 认证与授权 |
| L1-1-L2-5 | Milestone Module | 里程碑跟踪 |
| L1-1-L2-6 | RateLimit Module | 限流控制 |
| L1-1-L2-7 | Metrics Module | 通信层指标 |
| L1-1-L2-8 | Renderers Module | 消息渲染器 |

### L1-2 Context Engine Layer Modules

| L2 ID | Module Name | Responsibility |
|-------|------------|---------------|
| L1-2-L2-1 | PEV Module | Plan-Execute-Verify 循环引擎 |
| L1-2-L2-2 | Compression Module | 七步压缩管道 |
| L1-2-L2-3 | Memory Module | 分层记忆管理 (Working/LongTerm) |
| L1-2-L2-4 | Token Module | Token 计数与预算管理 |
| L1-2-L2-5 | Registry Module | 操作注册表 |
| L1-2-L2-6 | Snapshot Module | 上下文快照 |
| L1-2-L2-7 | Prompt Module | Prompt 模板管理 |
| L1-2-L2-8 | Sandbox Module | 工具沙箱隔离 |

### L1-3 LLM Gateway Layer Modules

| L2 ID | Module Name | Responsibility |
|-------|------------|---------------|
| L1-3-L2-1 | Adapter Module | 模型适配器 (DeepSeek, MiniMax) |
| L1-3-L2-2 | Gateway Module | LLM 路由与聚合 |
| L1-3-L2-3 | Breaker Module | 熔断器 |
| L1-3-L2-4 | Retry Module | 重试策略 |
| L1-3-L2-5 | Token Module | Token 计数 |
| L1-3-L2-6 | Config Module | 配置加载 |

### L1-4 Multi-Agent Layer Modules

| L2 ID | Module Name | Responsibility |
|-------|------------|---------------|
| L1-4-L2-1 | Factory Module | Agent 工厂 |
| L1-4-L2-2 | Agent Module | Agent 生命周期管理 |
| L1-4-L2-3 | Permission Module | 权限管道 |
| L1-4-L2-4 | Fork Module | Agent Fork/Join |
| L1-4-L2-5 | Observer Module | 事件观察者 |

### L1-5 Observability Layer Modules

| L2 ID | Module Name | Responsibility |
|-------|------------|---------------|
| L1-5-L2-1 | Tracer Module | 分布式追踪 |
| L1-5-L2-2 | Metrics Module | 指标收集 |
| L1-5-L2-3 | Logger Module | 日志记录 |
| L1-5-L2-4 | Exporter Module | 数据导出 |
| L1-5-L2-5 | Coverage Module | 操作覆盖率 |
| L1-5-L2-6 | Telemetry Module | 遥测数据 |
| L1-5-L2-7 | Settings Module | 配置管理 |

### L1-6 Evolution Layer Modules

| L2 ID | Module Name | Responsibility |
|-------|------------|---------------|
| L1-6-L2-1 | Version Module | 版本检测与记录 |
| L1-6-L2-2 | Config Module | 配置热更新 |

---

## Directory Structure Mapping

```
devrix/
internal/
layers/
├── communication/                  # L1-1
│   ├── gateway/                   # L1-1-L2-1
│   ├── adapters/                  # L1-1-L2-2
│   ├── commands/                  # L1-1-L2-3
│   ├── auth/                     # L1-1-L2-4
│   ├── milestone/                # L1-1-L2-5
│   ├── ratelimit/                # L1-1-L2-6
│   ├── metrics/                  # L1-1-L2-7
│   └── renderers/               # L1-1-L2-8
│
├── contextengine/                # L1-2
│   ├── pev/                      # L1-2-L2-1
│   ├── compression/              # L1-2-L2-2
│   ├── memory/                   # L1-2-L2-3
│   ├── token/                   # L1-2-L2-4
│   ├── registry/                # L1-2-L2-5
│   ├── snapshot/                 # L1-2-L2-6
│   ├── prompt/                   # L1-2-L2-7
│   └── sandbox/                 # L1-2-L2-8
│
├── llmgateway/                   # L1-3
│   ├── adapter/                 # L1-3-L2-1
│   ├── gateway/                  # L1-3-L2-2
│   ├── breaker/                  # L1-3-L2-3
│   ├── retry/                   # L1-3-L2-4
│   ├── token/                   # L1-3-L2-5
│   └── config/                  # L1-3-L2-6
│
├── multiagent/                   # L1-4 (待实现)
│   ├── factory/                 # L1-4-L2-1
│   ├── agent/                   # L1-4-L2-2
│   ├── permission/              # L1-4-L2-3
│   ├── fork/                    # L1-4-L2-4
│   └── observer/                # L1-4-L2-5
│
├── observability/                # L1-5
│   ├── tracer/                  # L1-5-L2-1
│   ├── metrics/                 # L1-5-L2-2
│   ├── logger/                  # L1-5-L2-3
│   ├── exporter/                # L1-5-L2-4
│   ├── coverage/                # L1-5-L2-5
│   ├── telemetry/               # L1-5-L2-6
│   └── settings/                # L1-5-L2-7
│
└── evolution/                    # L1-6 (待实现)
    ├── version/                 # L1-6-L2-1
    └── config/                  # L1-6-L2-2
```

---

## L4 Capability Mapping (Legacy)

> **DEPRECATED**: 旧系统使用 `L4-{LAYER}-{NAME}` 格式，已废弃。
> 请使用新的 L1-L2 二级编号系统。

| Legacy ID | New L1-L2 ID | 说明 |
|-----------|--------------|------|
| L4-COMM-STORE | L1-1-L2-1 (Gateway) | 会话存储 |
| L4-COMM-CMD | L1-1-L2-3 (Commands) | 命令处理 |
| L4-CTX-STATE | L1-2 (Context Engine) | 状态管理 |
| L4-CTX-PEV | L1-2-L2-1 (PEV) | PEV 引擎 |
| L4-CTX-COMPRESS | L1-2-L2-2 (Compression) | 压缩管道 |
| L4-CTX-MEMORY | L1-2-L2-3 (Memory) | 记忆管理 |
| L4-LLM-ADAPTER | L1-3-L2-1 (Adapter) | 模型适配器 |
| L4-LLM-BREAKER | L1-3-L2-3 (Breaker) | 熔断器 |
| L4-LLM-TOKEN | L1-3-L2-5 (Token) | Token 计数 |
| L4-OBS-TRACING | L1-5-L2-1 (Tracer) | 追踪 |
| L4-OBS-METRICS | L1-5-L2-2 (Metrics) | 指标 |
| L4-OBS-LOGGING | L1-5-L2-3 (Logger) | 日志 |

---

## L5 Test Points Registry

L5 测试点编号格式: `L5-{L1}-{L2}-{NN}`

例如:
- `L5-1-1-01` = L1-1 (Communication) L2-1 (Gateway) 测试点 01
- `L5-2-1-01` = L1-2 (Context Engine) L2-1 (PEV) 测试点 01
- `L5-3-3-01` = L1-3 (LLM Gateway) L2-3 (Breaker) 测试点 01

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-08 | Initial specification |
