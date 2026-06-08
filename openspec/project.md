# Devrix - OpenSpec Project Metadata

**Project Name:** Devrix - 开放大脑
**Project Type:** Go CLI Application
**Spec Version:** 1.0.0
**Layering Spec:** `openspec/specs/architecture/layering.md`
**L5 Registry:** `openspec/l5-registry.md`
**Created:** 2026-06-06
**Status:** Active

---

## Project Overview

Devrix is a multi-agent collaborative development assistant - "第二大脑" - that provides:
- Conversational loop with LLM
- Tool system with permission pipeline
- Sub-agent orchestration
- Context compression (7-step pipeline)
- Self-evolution capabilities

**对标:** Claude Code 全部能力
**架构参考:** cc-connect (Go)

## Technical Stack

| Category | Technology | Version |
|----------|------------|---------|
| Language | Go | 1.21+ |
| Testing | Go testing + 分层测试框架 | See `specs/testing-framework/spec.md` |
| Build | go build | - |
| CLI | cobra / urfave/cli | latest |
| Logging | slog | builtin |
| Storage | File System (JSON) | - |
| LLM | Anthropic, DeepSeek, OpenAI Compatible | - |

---

## Layer 1 (L1) - Top-Level Architecture

| L1 ID | Layer Name | Responsibility |
|-------|------------|----------------|
| **L1-1** | Communication Layer | IM Gateway, WebSocket, CLI adapter |
| **L1-2** | Context Engine Layer | PEV Engine, 7-step compression, layered memory |
| **L1-3** | LLM Gateway Layer | Model adapter, circuit breaker, token counter |
| **L1-4** | Multi-Agent Layer | Agent lifecycle, fork, collaboration modes |
| **L1-5** | Observability Layer | Tracing, metrics, logging |
| **L1-6** | Evolution Layer | Version management, A/B testing |

---

## Layer 2 (L2) - Sub-Modules

### L1-1 Communication Layer Modules

| L2 ID | Module | Responsibility |
|-------|--------|----------------|
| L1-1-L2-1 | Gateway | 消息网关、路由、会话管理 |
| L1-1-L2-2 | Adapters | 飞书、WebSocket、CLI 适配器 |
| L1-1-L2-3 | Commands | CLI 命令解析 (/new, /stop, /help) |
| L1-1-L2-4 | Auth | 认证与授权 |
| L1-1-L2-5 | Milestone | 里程碑跟踪 |
| L1-1-L2-6 | RateLimit | 限流控制 |
| L1-1-L2-7 | Metrics | 通信层指标 |
| L1-1-L2-8 | Renderers | 消息渲染器 |

### L1-2 Context Engine Layer Modules

| L2 ID | Module | Responsibility |
|-------|--------|----------------|
| L1-2-L2-1 | PEV | Plan-Execute-Verify 循环引擎 |
| L1-2-L2-2 | Compression | 七步压缩管道 |
| L1-2-L2-3 | Memory | 分层记忆管理 (Working/LongTerm) |
| L1-2-L2-4 | Token | Token 计数与预算管理 |
| L1-2-L2-5 | Registry | 操作注册表 |
| L1-2-L2-6 | Snapshot | 上下文快照 |
| L1-2-L2-7 | Prompt | Prompt 模板管理 |
| L1-2-L2-8 | Sandbox | 工具沙箱隔离 |

### L1-3 LLM Gateway Layer Modules

| L2 ID | Module | Responsibility |
|-------|--------|----------------|
| L1-3-L2-1 | Adapter | 模型适配器 (DeepSeek, MiniMax) |
| L1-3-L2-2 | Gateway | LLM 路由与聚合 |
| L1-3-L2-3 | Breaker | 熔断器 |
| L1-3-L2-4 | Retry | 重试策略 |
| L1-3-L2-5 | Token | Token 计数 |
| L1-3-L2-6 | Config | 配置加载 |

### L1-4 Multi-Agent Layer Modules

| L2 ID | Module | Responsibility |
|-------|--------|----------------|
| L1-4-L2-1 | Factory | Agent 工厂 |
| L1-4-L2-2 | Agent | Agent 生命周期管理 |
| L1-4-L2-3 | Permission | 权限管道 |
| L1-4-L2-4 | Fork | Agent Fork/Join |
| L1-4-L2-5 | Observer | 事件观察者 |

### L1-5 Observability Layer Modules

| L2 ID | Module | Responsibility |
|-------|--------|----------------|
| L1-5-L2-1 | Tracer | 分布式追踪 |
| L1-5-L2-2 | Metrics | 指标收集 |
| L1-5-L2-3 | Logger | 日志记录 |
| L1-5-L2-4 | Exporter | 数据导出 |
| L1-5-L2-5 | Coverage | 操作覆盖率 |
| L1-5-L2-6 | Telemetry | 遥测数据 |
| L1-5-L2-7 | Settings | 配置管理 |

### L1-6 Evolution Layer Modules

| L2 ID | Module | Responsibility |
|-------|--------|----------------|
| L1-6-L2-1 | Version | 版本检测与记录 |
| L1-6-L2-2 | Config | 配置热更新 |

---

## Data Flow

```
User → Communication → Context Engine → LLM Gateway → Multi-Agent → Observability → Evolution → User
           ↓              ↓              ↓            ↓            ↓
         Session       PEV           Model        Tool        Trace
         Store       Compress       Breaker     Permission  Metrics
```

---

## Design Principles

1. **Accept Interfaces, Return Structs** - Go idiom for flexibility
2. **Small Packages** - Keep packages focused, < 500 lines typical
3. **Error Wrapping** - Always wrap errors with context: `fmt.Errorf("context: %w", err)`
4. **Explicit Over Implicit** - No magic, clear dependencies
5. **Concurrency Safety** - Use channels or mutexes, document ownership
6. **Minimum Coverage** - 80% test coverage
7. **L5-Driven Testing** - All L4/L3 changes MUST map to L5 test points (see `l5-registry.md`)

---

## Directory Structure

```
devrix/
├── cmd/
│   └── devrix/
│       └── main.go              # Entry point
├── internal/
│   ├── layers/
│   │   ├── communication/        # L1-1
│   │   │   ├── gateway/        # L1-1-L2-1
│   │   │   ├── adapters/       # L1-1-L2-2
│   │   │   ├── commands/       # L1-1-L2-3
│   │   │   ├── auth/          # L1-1-L2-4
│   │   │   ├── milestone/     # L1-1-L2-5
│   │   │   ├── ratelimit/     # L1-1-L2-6
│   │   │   ├── metrics/       # L1-1-L2-7
│   │   │   └── renderers/     # L1-1-L2-8
│   │   ├── contextengine/     # L1-2
│   │   │   ├── pev/           # L1-2-L2-1
│   │   │   ├── compression/   # L1-2-L2-2
│   │   │   ├── memory/        # L1-2-L2-3
│   │   │   ├── token/         # L1-2-L2-4
│   │   │   ├── registry/      # L1-2-L2-5
│   │   │   ├── snapshot/      # L1-2-L2-6
│   │   │   ├── prompt/        # L1-2-L2-7
│   │   │   └── sandbox/       # L1-2-L2-8
│   │   ├── llmgateway/        # L1-3
│   │   │   ├── adapter/       # L1-3-L2-1
│   │   │   ├── gateway/       # L1-3-L2-2
│   │   │   ├── breaker/       # L1-3-L2-3
│   │   │   ├── retry/         # L1-3-L2-4
│   │   │   ├── token/         # L1-3-L2-5
│   │   │   └── config/        # L1-3-L2-6
│   │   ├── observability/     # L1-5
│   │   │   ├── tracer/        # L1-5-L2-1
│   │   │   ├── metrics/       # L1-5-L2-2
│   │   │   ├── logger/        # L1-5-L2-3
│   │   │   ├── exporter/      # L1-5-L2-4
│   │   │   ├── coverage/      # L1-5-L2-5
│   │   │   ├── telemetry/     # L1-5-L2-6
│   │   │   └── settings/      # L1-5-L2-7
│   │   └── multiagent/        # L1-4 (待实现)
│   └── shared/
│       ├── types/
│       ├── config/
│       ├── errors/
│       └── utils/
├── pkg/
│   └── i18n/
├── openspec/
│   ├── project.md             # This file
│   ├── l5-registry.md         # L5 测试点注册表
│   ├── specs/
│   │   ├── architecture/
│   │   │   └── layering.md    # 分层架构规范
│   │   ├── testing-framework/
│   │   └── testing-quality/
│   └── changes/
├── tests/
│   ├── testutil/
│   ├── integration/           # //go:build integration
│   ├── e2e/                  # //go:build smoke
│   ├── acceptance/            # //go:build acceptance
│   │   └── p0/
│   ├── performance/           # //go:build performance
│   └── security/              # //go:build security
└── scripts/
    ├── test-unit.sh
    ├── test-integration.sh
    ├── test-e2e.sh
    ├── test-acceptance.sh
    └── test-all.sh
```

---

## Version Scope

| Version | Milestone | Features |
|---------|-----------|----------|
| V1 | MVP | CLI mode, basic context compression (steps 1-5,7), single/simple agent |
| V2 | Enhanced | IM adapters, autocompact (step 6), rate limiter |
| V3 | Full | All features, milestone DAG, peer-review, full collaboration modes |

---

## CLI Commands

| Command | Description |
|---------|-------------|
| `devrix` | Start interactive CLI |
| `devrix daemon install` | Install as system service |
| `devrix daemon start` | Start daemon |
| `devrix daemon stop` | Stop daemon |
| `devrix project list` | List projects |
| `devrix project add <name>` | Add project |
| `devrix session list` | List sessions |
| `devrix session new` | New session |
| `/new`, `/stop`, `/help` | In-session commands |

---

## Testing Framework

**强制规范**: `openspec/specs/testing-framework/spec.md`

| 阶段 | 命令 | 用途 |
|------|------|------|
| 日常开发 | `./scripts/test-unit.sh` | PR 门槛 |
| 集成测试 | `./scripts/test-integration.sh` + `./scripts/test-e2e.sh` | 集成验证 |
| S5 验收 | `./scripts/test-acceptance.sh` | L5 P0 验收 |
| 全部 | `./scripts/test-all.sh` | 颁布前 |
| S5 验收报告 | `./scripts/gen-acceptance-report.sh --change {slug}` | 生成报告 |

**CI**: `.github/workflows/ci.yml`（unit → integration/e2e/acceptance → coverage 报告）

---

## L5 Test Points Summary

See `openspec/l5-registry.md` for full details.

| L1 | Layer Name | Total | IMPLEMENTED | PLANNED |
|----|------------|-------|-------------|---------|
| L1-1 | Communication | 5 | 0 | 5 |
| L1-2 | Context Engine | 21 | 16 | 5 |
| L1-3 | LLM Gateway | 17 | 13 | 4 |
| L1-4 | Multi-Agent | 8 | 0 | 8 |
| L1-5 | Observability | 16 | 11 | 5 |
| L1-6 | Evolution | 2 | 0 | 2 |
| **Total** | | **69** | **40** | **29** |

---

## Related Specifications

| Spec | Path |
|------|------|
| Layer Architecture | `openspec/specs/architecture/layering.md` |
| L5 Test Registry | `openspec/l5-registry.md` |
| Testing Framework | `openspec/specs/testing-framework/spec.md` |
| Testing Quality | `openspec/specs/testing-quality/spec.md` |

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-06 | Initial project metadata |
| 1.1.0 | 2026-06-08 | Updated to L1-L2 layering system |
