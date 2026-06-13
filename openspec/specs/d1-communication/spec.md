# D1 Communication Domain Specification

**Capability:** communication
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-13

---

## Overview

通信域负责消息网关、多协议适配器（飞书/CLI/WebSocket）、会话管理、权限、限流、事件总线和消息渲染。作为 Devrix 的入口层，所有用户交互经由此域进入系统。

## Scenarios

| ID | Scenario | Responsibility | Status |
|----|----------|----------------|--------|
| D1-S1 | Gateway | 消息网关、路由、会话管理 | IMPLEMENTED |
| D1-S2 | Adapters | 飞书、钉钉、CLI 适配器 | IMPLEMENTED |
| D1-S3 | Commands | CLI 命令解析 (/new, /stop, /help) | IMPLEMENTED |
| D1-S4 | Auth | 认证与授权 | PLANNED |
| D1-S5 | Milestone | 里程碑跟踪 | IMPLEMENTED |
| D1-S6 | RateLimit | 限流控制 | IMPLEMENTED |
| D1-S7 | Metrics | 通信层指标 | IMPLEMENTED |
| D1-S8 | Renderers | 消息渲染器 | IMPLEMENTED |
| D1-S9 | EventBus | 背压事件总线 | IMPLEMENTED |
| D1-S10 | Connection | 连接管理 | IMPLEMENTED |
| D1-S11 | Core | 核心配置解析 | IMPLEMENTED |
| D1-S12 | Instance | 实例注册 | IMPLEMENTED |

## Architecture

```
User/IM → Adapters (D1-S2) → Gateway (D1-S1) → Context Engine (D2)
                                    ↓
                              EventBus (D1-S9)
                                    ↓
                              Renderers (D1-S8) → User/IM
```

## Registries

- **A 层**: `a-registry.md` — 17 Activities
- **F 层**: `f-registry.md` — 21 Function Points
- **T 层**: `t-registry.md` — 22 Test Points (22 IMPLEMENTED)
