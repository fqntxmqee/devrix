# Devrix - OpenSpec Project Metadata

**Project Name:** Devrix - 开发大脑
**Project Type:** Go CLI Application
**Spec Version:** 1.0.0
**Created:** 2026-06-06
**Status:** Draft

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

## Six-Layer Architecture

| Layer | Name | Responsibility |
|-------|------|----------------|
| ① | Communication Layer | IM Gateway, WebSocket, CLI adapter |
| ② | Context Engine Layer | PEV Engine, 7-step compression, layered memory |
| ③ | LLM Gateway Layer | Model adapter, circuit breaker, token counter |
| ④ | Multi-Agent Layer | Agent lifecycle, fork, collaboration modes |
| ⑤ | Observability Layer | Tracing, metrics, logging |
| ⑥ | Evolution Layer | Version management, A/B testing |

## Version Scope

| Version | Milestone | Features |
|---------|-----------|----------|
| V1 | MVP | CLI mode, basic context compression (steps 1-5,7), single/simple agent |
| V2 | Enhanced | IM adapters, autocompact (step 6), rate limiter |
| V3 | Full | All features, milestone DAG, peer-review, full collaboration modes |

## Data Flow

```
User → Communication → Context Engine → LLM Gateway → Multi-Agent → Observability → Evolution → User
                ↓              ↓              ↓            ↓            ↓
              Session        PEV           Model        Tool        Trace
              Store          Compress      Breaker      Permission  Metrics
```

## Design Principles

1. **Accept Interfaces, Return Structs** - Go idiom for flexibility
2. **Small Packages** - Keep packages focused, < 500 lines typical
3. **Error Wrapping** - Always wrap errors with context: `fmt.Errorf("context: %w", err)`
4. **Explicit Over Implicit** - No magic, clear dependencies
5. **Concurrency Safety** - Use channels or mutexes, document ownership
6. **Minimum Coverage** - 80% test coverage
7. **L5-Driven Testing** - All L4 changes MUST map to L5 test points (see `l5-registry.md`)

## Directory Structure

```
devrix/
├── cmd/
│   └── devrix/
│       └── main.go              # Entry point
├── internal/
│   ├── layers/
│   │   ├── communication/        # Layer 1
│   │   ├── contextengine/       # Layer 2
│   │   ├── llmgateway/          # Layer 3
│   │   ├── multiagent/          # Layer 4
│   │   ├── observability/        # Layer 5
│   │   └── evolution/           # Layer 6
│   └── shared/
│       ├── types/               # Shared type definitions
│       ├── config/              # Configuration management
│       ├── errors/              # Error definitions
│       └── utils/               # Utility functions
├── pkg/
│   └── i18n/                   # Internationalization
├── openspec/
│   ├── project.md               # This file
│   ├── l5-registry.md           # L5 测试点注册表（验收锚点）
│   ├── specs/                   # Source of Truth specs
│   │   ├── testing-framework/   # 测试框架规范（强制）
│   │   └── *_layer_delta.md     # 各层 Delta 规格
│   └── changes/                 # Change proposals
├── tests/
│   ├── testutil/                # 跨包 Mock / Fixture
│   ├── integration/             # //go:build integration
│   ├── e2e/                     # //go:build smoke
│   └── acceptance/              # //go:build acceptance (L5 P0/P1/P2)
└── scripts/
    └── test-*.sh                # 测试执行门禁
```

**Note:** Go project structure follows standard layout:
- `cmd/` - Application entry points
- `internal/` - Private application code
- `pkg/` - Public libraries

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

## Testing Framework

**强制规范**: `openspec/specs/testing-framework/spec.md`

| 阶段 | 命令 | 用途 |
|------|------|------|
| 日常开发 | `./scripts/test-unit.sh` | PR 门禁 |
| 合入前 | `./scripts/test-integration.sh` + `./scripts/test-e2e.sh` | 集成验证 |
| S5 验收 | `./scripts/test-acceptance.sh` | L5 P0 验收 |
| 全量 | `./scripts/test-all.sh` | 发布前 |
| S5 验收报告 | `./scripts/gen-acceptance-report.sh --change {slug}` | 生成 acceptance-report.md |

新增 L4 能力流程：登记 L5 → 编写测试（标注 `// Covers: L5-*`）→ 更新 `l5-registry.md` → 跑关联脚本 → S5 生成验收报告。

CI：`.github/workflows/ci.yml`（unit → integration/e2e/acceptance → coverage 报告）。

## L3/L4 Asset Registry (Context Engine)

> V1 已归档：`openspec/archive/2026-06-07-devrix-context-engine/`
> V2 已归档：`openspec/archive/2026-06-07-devrix-context-engine-v2/` — Autocompact + Verify commands + Token 统一
> V3 规划中：`openspec/changes/devrix-context-engine-v3/` — PEV Plan + Milestone DAG + LongTerm Memory

### L3 业务活动

| ID | 名称 | L4 映射 |
|----|------|---------|
| L3-BE-CTX-01 | 处理用户消息并维护上下文 | L4-CTX-STATE, L4-CTX-PEV, L4-CTX-MEMORY |
| L3-BE-CTX-02 | 超长对话压缩 | L4-CTX-COMPRESS |
| L3-BE-CTX-03 | 复杂任务分解与里程碑推进 | L4-CTX-PLAN, L4-CTX-PEV |

### L4 功能点

| ID | 名称 | 包路径 |
|----|------|--------|
| L4-CTX-STATE | 上下文状态管理 | `contextengine/engine.go`, `prompt/` |
| L4-CTX-PEV | PEV 执行循环 | `contextengine/pev/` |
| L4-CTX-COMPRESS | 七步压缩管道 | `contextengine/compression/` |
| L4-CTX-MEMORY | 分层记忆 | `contextengine/memory/`, `snapshot/` |
| L4-CTX-OBS | 压缩/验证可观测 | `contextengine/` + Observability bridge（V2） |
| L4-CTX-PLAN | 任务规划与 DAG 生成 | `contextengine/pev/plan.go`（V3） |

## Related Documents

- **Architecture Docs:** `docs/README.md`（含 `detail design framework.md` 六段式模板）
- Context Engine Design: `docs/context-engine-design.md`
- Obsidian Vault: `01知识探索/项目/devrix/`
- Architecture Reference: cc-connect (Go daemon)
- Testing Framework Spec: `openspec/specs/testing-framework/spec.md`
- L5 Registry: `openspec/l5-registry.md`
