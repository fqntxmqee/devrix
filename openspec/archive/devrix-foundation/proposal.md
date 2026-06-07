# Proposal: Devrix Foundation - Multi-Agent Development Assistant

**Change ID:** `devrix-foundation`
**Created:** 2026-06-06
**Status:** Archived

---

## Problem Statement

开发团队需要一个**多智能协同开发助手**，对标 Claude Code 的全部能力：
- 对话循环 (Conversational Loop)
- 工具系统 (Tool System)
- 子智能体调度 (Sub-Agent Orchestration)
- 权限管线 (Permission Pipeline)
- 上下文压缩 (Context Compression)

当前没有满足此需求的开源 Node.js/TypeScript 实现。

## Proposed Solution

构建 **Devrix** - "开发大脑"，一个基于六层架构的 Go CLI 应用：

| Layer | Name | V1 Implementation |
|-------|------|-------------------|
| ① | Communication | CLI Adapter + Gateway |
| ② | Context Engine | PEV Engine + 7-step Compression |
| ③ | LLM Gateway | Model Adapters + Circuit Breaker |
| ④ | Multi-Agent | Agent Lifecycle + Permission Pipeline |
| ⑤ | Observability | Trace Events + Metrics + Logging |
| ⑥ | Evolution | Placeholder (V2 implementation) |

### 核心设计决策

1. **单进程 + 模块化** - 适合 PC 端部署，简化调试
2. **Go 1.21+** - 性能/稳定性/可移植性最佳
3. **标准库 + cobra** - 减少依赖，简化构建
4. **文件持久化** - JSON 文件，符合 V1 简单需求
5. **V1 CLI 优先** - Electron/Tauri GUI 留待 V2

## Scope

### In Scope (V1 MVP)
- CLI 命令行界面
- 基本上下文压缩 (步骤 1-5, 7)
- 单一 Agent 执行 (简化 Fork)
- 内置工具注册表 (read, glob, ls, grep, fetch, write, edit, bash, git)
- 权限管线 (同步确认模式)
- 流式响应
- 基础可观察性 (traces, metrics, logs)
- Anthropic Claude + DeepSeek + OpenAI Compatible 支持

### Out of Scope (V2/V3)
- IM 适配器 (飞书/钉钉/Telegram)
- WebSocket 支持
- Autocompact (LLM 摘要生成)
- Rate Limiter
- 完整 5 种协作模式
- Long-term Memory
- Milestone DAG
- A/B Testing
- Version Management

## Impact Analysis

| Component | Change Required | Details |
|-----------|-----------------|---------|
| Communication Layer | Yes | New implementation |
| Context Engine Layer | Yes | New implementation |
| LLM Gateway Layer | Yes | New implementation |
| Multi-Agent Layer | Yes | New implementation |
| Observability Layer | Yes | New implementation |
| Evolution Layer | Yes | Placeholder only |
| Shared Types/Config/Errors | Yes | New implementation |
| CLI Entry Point | Yes | New implementation |

## Architecture Considerations

### 模块依赖规则
```
Communication Layer
  ├──→ Context Engine Layer
  │         ├──→ LLM Gateway Layer → 外部 LLM API
  │         ├──→ Multi-Agent Layer → 系统资源
  │         └──→ Observability Layer
  └──→ Observability Layer
            └──→ Evolution Layer (passive)
```

### 关键接口

1. **IMessageAdapter** - 通信层与 IM 平台的接口
2. **IContextEngine** - 上下文管理的核心接口
3. **ILLMGateway** - LLM 调用的统一接口
4. **IAgent** - Agent 的生命周期接口
5. **IPermissionPipeline** - 权限确认的接口
6. **IObserver** - 可观察性的接口

### 设计原则
- **Immutable Data** - 创建新对象，从不修改现有对象
- **Small Files** - 每个文件 200-400 行，800 行上限
- **Error Handling** - 每层都要有错误处理
- **Input Validation** - 在系统边界验证输入

## Success Criteria

- [ ] CLI 启动成功，显示欢迎信息
- [ ] 用户可以发送消息并收到 LLM 响应
- [ ] 内置工具可以正常执行 (read, glob, grep 等)
- [ ] 权限确认流程正常工作
- [ ] 流式响应正常显示
- [ ] 上下文压缩在超长对话中生效
- [ ] 错误信息清晰且有上下文
- [ ] 单元测试覆盖率 ≥ 80%
- [ ] 所有 delta specs 中的 Scenario 都有对应实现

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| LLM API 不稳定导致体验差 | Medium | Medium | Circuit Breaker + 重试 |
| Token 预算控制不准确 | Medium | High | 充分测试边界情况 |
| 权限管线超时导致阻塞 | Low | Medium | 60s 超时自动拒绝 |
| 单进程内存泄漏 | Low | High | Observability 监控 |

---

## Archive Information

**Archived:** 2026-06-07
**Duration:** 1 day (2026-06-06 to 2026-06-07)
**Outcome:** Partially implemented - Communication Layer complete

### Completed (Phase 1 - Communication Layer)
- [x] CLI Adapter with ANSI rendering
- [x] FileSessionStore with idle timeout
- [x] Communication Gateway (RouteInbound/RouteOutbound)
- [x] PermissionManager with timeout
- [x] Command handler (/new, /stop, /help)
- [x] RateLimiter (Token Bucket)
- [x] Milestone Service with DAG
- [x] Feishu Adapter (WebSocket)
- [x] Interface abstractions for testability (FeishuAPI, GatewayAPI)
- [x] Unit tests with mock implementations

### Files Modified
- `internal/layers/communication/` - Complete communication layer implementation
- `internal/shared/types/` - Shared types (Session, Message, PermissionRequest, Milestone)
- `openspec/changes/devrix-foundation/specs/communication/design.md` - Updated architecture doc

### Specs Updated
- `openspec/specs/communication_layer_delta.md` - Milestone DAG moved from V3 to V1

### Test Coverage
| Module | Coverage |
|--------|----------|
| ratelimit | 95.2% |
| gateway | 55.7% |
| milestone | 42.4% |
| adapters/feishu | 23.2% |

### Critical Fixes Applied
1. escapeJSON - Added `/` escaping
2. RateLimiter - Fixed integer division bug
3. Gateway - Added sessionStore.Delete() on expire
4. PermissionManager - Fixed deadlock in resolveRequest

### Remaining Work
- Phases 3-9 (Context Engine, LLM Gateway, Multi-Agent, Observability, Evolution)
- Full test coverage (target 80%)
- CLI entry point integration
