# Proposal: Context Engine Layer (Layer 2)

**Change ID:** devrix-context-engine
**Layer:** 2 - Context Engine
**Type:** New Capability
**Status:** Draft
**Based on:** devrix-foundation delta, communication layer integration

---

## Motivation

通信层（Layer 1）已具备 Gateway、Session、四流事件与 Milestone/TaskFlow（V3），但消息处理仍使用 `StubContextEngine`（Echo 回显），无法：

1. 维护对话历史与 Token 预算
2. 在超长会话中执行七步压缩
3. 驱动 PEV（Plan-Execute-Verify）任务循环
4. 将 `Session.ContextSnapshot` 持久化恢复

上下文引擎是 Devrix「对标 Claude Code」的核心层，必须在 LLM Gateway 与 Multi-Agent 之前落地设计。

## Goals

| Goal | V1 | V2 | V3 |
|------|----|----|-----|
| 替换 StubContextEngine | ✅ | ✅ | ✅ |
| PEV Execute→Verify | ✅ | ✅ | ✅ |
| PEV Plan + Milestone | ❌ | ❌ | ✅ |
| 压缩步骤 1-5, 7 | ✅ | ✅ | ✅ |
| Autocompact（步骤 6） | ❌ | ✅ | ✅ |
| Working + ShortTerm Memory | ✅ | ✅ | ✅ |
| LongTerm Memory | ❌ | ❌ | ✅ |

## Capabilities

| Capability | L4 映射 | 说明 |
|------------|---------|------|
| context-engine-core | L4-CTX-STATE | Process 入口、快照、与 Gateway 事件契约 |
| pev-engine | L4-CTX-PEV | 简化 PEV 循环 |
| compression-pipeline | L4-CTX-COMPRESS | 七步压缩（V1 跳过 step 6） |
| layered-memory | L4-CTX-MEMORY | Working / ShortTerm / LongTerm(V3) |

## Technical Approach

- 包路径：`internal/layers/contextengine/`
- 实现 `gateway.IContextEngine`（或将接口上移到 `internal/shared/contracts`）
- 依赖注入：`ILLMGateway`、`IToolRunner`、`IObserver`（接口，V1 可 mock）
- 持久化：ShortTerm 序列化到 `Session.ContextSnapshot` + `~/.devrix/context/{sessionId}.json`

## Impact

| 组件 | 影响 |
|------|------|
| `cmd/devrix/main.go` | 注入真实 ContextEngine |
| `gateway.IContextEngine` | 可能扩展 EngineEvent 元数据 |
| `types.Session` | ContextSnapshot 格式版本化 |
| `openspec/l5-registry.md` | 新增 L5-CTX-* |
| LLM Gateway | 需提供 chat/stream 接口（可后置 stub） |
| Multi-Agent | 需提供 tool 执行接口（可后置 stub） |

## Scope

**In Scope:** 本变更的设计与规格（无代码）

**Out of Scope:** 实现、LLM 多模型适配、Evolution 层

## Open Questions

1. Token 计数使用本地 tiktoken 还是 LLM Gateway 统一提供？
2. System Prompt 来源：项目 `AGENTS.md` / `.devrix/` 配置 / 内置默认？
3. PEV Verify 阶段默认验证命令集（test / lint / build）是否可配置？

## Documentation Deliverables

| 产物 | 路径 | 框架 |
|------|------|------|
| 详细架构设计 | `docs/context-engine-design.md` | `docs/detail design framework.md` ①~⑥ |
| OpenSpec 实施设计 | `openspec/changes/devrix-context-engine/design.md` | 包结构/代码/分期 |
| Delta 规格 | `specs/context-engine/spec.md` | OpenSpec Gherkin |
| 任务清单 | `tasks.md` | L4/L5 映射 |

## Success Criteria (S3 准出)

- [x] `docs/context-engine-design.md` 按六段式框架完整
- [ ] proposal / design / specs / tasks 四件套完整
- [ ] 每个 L4 能力至少 1 个 L5 测试点
- [ ] 与通信层 `IContextEngine` 契约对齐
- [ ] V1/V2/V3 分期明确
