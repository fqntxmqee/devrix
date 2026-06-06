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
| Testing | Go testing (builtin) | - |
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
│   ├── specs/                   # Delta specs per layer
│   └── changes/                # Change proposals
└── tests/                       # Integration tests
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

## Related Documents

- Obsidian Vault: `01知识探索/项目/devrix/`
- Architecture Reference: cc-connect (Go daemon)
