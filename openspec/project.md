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

## Domain (D) - Top-Level Architecture

| Domain ID | Domain Name | Responsibility |
|-----------|-------------|----------------|
| **D1** | Communication Domain (COMM) | IM Gateway, WebSocket, CLI adapter |
| **D2** | Context Engine Domain (CTX) | PEV Engine, 7-step compression, layered memory |
| **D3** | LLM Gateway Domain (LLM) | Model adapter, circuit breaker, token counter |
| **D4** | Multi-Agent Domain (AGENT) | Agent lifecycle, fork, collaboration modes |
| **D5** | Observability Domain (OBS) | Tracing, metrics, logging |
| **D6** | Evolution Domain (EVO) | Version management, A/B testing |

---

## Scenario (S) - Domain Scenarios

### D1 Communication Domain Scenarios

| Module ID | Module | Responsibility |
|-------|--------|----------------|
| D1-S1 | Gateway | 消息网关、路由、会话管理 |
| D1-S2 | Adapters | 飞书、WebSocket、CLI 适配器 |
| D1-S3 | Commands | CLI 命令解析 (/new, /stop, /help) |
| D1-S4 | Auth | 认证与授权 |
| D1-S5 | Milestone | 里程碑跟踪 |
| D1-S6 | RateLimit | 限流控制 |
| D1-S7 | Metrics | 通信层指标 |
| D1-S8 | Renderers | 消息渲染器 |

### D2 Context Engine Domain Scenarios

| Module ID | Module | Responsibility |
|-------|--------|----------------|
| D2-S1 | PEV | Plan-Execute-Verify 循环引擎 |
| D2-S2 | Compression | 七步压缩管道 |
| D2-S3 | Memory | 分层记忆管理 (Working/LongTerm) |
| D2-S4 | Token | Token 计数与预算管理 |
| D2-S5 | Registry | 操作注册表 |
| D2-S6 | Snapshot | 上下文快照 |
| D2-S7 | Prompt | Prompt 模板管理 |
| D2-S8 | Sandbox | 工具沙箱隔离 |
| D2-S9 | Harness | 会话 Bootstrap、ToolPool、System Prompt 装配（V5，灰度 `harness.enabled`） |

### D3 LLM Gateway Domain Scenarios

| Module ID | Module | Responsibility |
|-------|--------|----------------|
| D3-S1 | Adapter | 模型适配器 (DeepSeek, MiniMax) |
| D3-S2 | Gateway | LLM 路由与聚合 |
| D3-S3 | Breaker | 熔断器 |
| D3-S4 | Retry | 重试策略 |
| D3-S5 | Token | Token 计数 |
| D3-S6 | Config | 配置加载 |

### D4 Multi-Agent Domain Scenarios

| Module ID | Module | Responsibility |
|-------|--------|----------------|
| D4-S1 | Factory | Agent 工厂 |
| D4-S2 | Agent | Agent 生命周期管理 |
| D4-S3 | Permission | 权限管道 |
| D4-S4 | Fork | Agent Fork/Join |
| D4-S5 | Observer | 事件观察者 |

### D5 Observability Domain Scenarios

| Module ID | Module | Responsibility |
|-------|--------|----------------|
| D5-S1 | Tracer | 分布式追踪 |
| D5-S2 | Metrics | 指标收集 |
| D5-S3 | Logger | 日志记录 |
| D5-S4 | Exporter | 数据导出 |
| D5-S5 | Coverage | 操作覆盖率 |
| D5-S6 | Telemetry | 遥测数据 |
| D5-S7 | Settings | 配置管理 |

### D6 Evolution Domain Scenarios

| Module ID | Module | Responsibility |
|-------|--------|----------------|
| D6-S1 | Version | 版本检测与记录 |
| D6-S2 | Config | 配置热更新 |

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
│   │   ├── communication/        # D1
│   │   │   ├── gateway/        # D1-S1
│   │   │   ├── adapters/       # D1-S2
│   │   │   ├── commands/       # D1-S3
│   │   │   ├── auth/          # D1-S4
│   │   │   ├── milestone/     # D1-S5
│   │   │   ├── ratelimit/     # D1-S6
│   │   │   ├── metrics/       # D1-S7
│   │   │   └── renderers/     # D1-S8
│   │   ├── contextengine/     # D2
│   │   │   ├── pev/           # D2-S1
│   │   │   ├── compression/   # D2-S2
│   │   │   ├── memory/        # D2-S3
│   │   │   ├── token/         # D2-S4
│   │   │   ├── registry/      # D2-S5
│   │   │   ├── snapshot/      # D2-S6
│   │   │   ├── prompt/        # D2-S7
│   │   │   ├── sandbox/       # D2-S8
│   │   │   └── harness/       # D2-S9 (V5)
│   │   ├── llmgateway/        # D3
│   │   │   ├── adapter/       # D3-S1
│   │   │   ├── gateway/       # D3-S2
│   │   │   ├── breaker/       # D3-S3
│   │   │   ├── retry/         # D3-S4
│   │   │   ├── token/         # D3-S5
│   │   │   └── config/        # D3-S6
│   │   ├── observability/     # D5
│   │   │   ├── tracer/        # D5-S1
│   │   │   ├── metrics/       # D5-S2
│   │   │   ├── logger/        # D5-S3
│   │   │   ├── exporter/      # D5-S4
│   │   │   ├── coverage/      # D5-S5
│   │   │   ├── telemetry/     # D5-S6
│   │   │   └── settings/      # D5-S7
│   │   └── multiagent/        # D4 (待实现)
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

| Domain | Domain Name | Total | IMPLEMENTED | PLANNED |
|----|------------|-------|-------------|---------|
| D1 | Communication | 5 | 0 | 5 |
| D2 | Context Engine | 21 | 16 | 5 |
| D3 | LLM Gateway | 17 | 13 | 4 |
| D4 | Multi-Agent | 8 | 0 | 8 |
| D5 | Observability | 16 | 11 | 5 |
| D6 | Evolution | 2 | 0 | 2 |
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
| 1.1.0 | 2026-06-08 | Updated to D-S domain system |
