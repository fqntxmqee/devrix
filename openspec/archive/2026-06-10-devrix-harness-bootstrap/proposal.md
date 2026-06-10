# Proposal: Harness Bootstrap Context Assembly (Context Engine V5)

**Change ID:** devrix-harness-bootstrap
**Demand ID:** DM-20260609-004
**Layer:** 2 - Context Engine (D2-S9)
**Type:** Enhancement
**Status:** Draft (Revised 2026-06-10 — Review + OpenSpec 合规)
**Based on:** context-engine V4 canonical spec, claw-code harness patterns, AgentScope Java harness architecture

---

## Revision Note

> **2026-06-10 Review 结论**：设计方向正确；开工前已澄清压缩/组装时序、L3 ID 冲突、ToolPool 工具名、OpenSpec delta 路径。详见 `demand.md` Q12–Q17。

---

## Problem Statement

Devrix Context Engine（V4）已具备完整的 **对话认知管线**（压缩、PEV、快照、LongTerm），但在 **Harness 启动装配** 上与 Claude Code 类 Agent 存在差距：

| 痛点 | 现状 (Devrix V4) | 期望 (claw-code 模式) |
|------|------------------|------------------------|
| 启动职责集中 | `bootstrap.NewContextEngine()` 一次性注入 | 分阶段 prefetch → setup → deferred init |
| 工具面裁剪 | 全量 registry + 执行时 PermissionGate | 启动期 ToolPool 过滤 + 执行期 Gate |
| Trust 模型 | 仅 tool_call 级审批 | Trust gate 控制 plugin/MCP 延迟加载 |
| 意图路由 | 完全依赖 LLM tool calling | 可选 pre-LLM command/tool 匹配 |
| Transcript | 与 Messages/CompressedView 混用 | 原始 turn log 与压缩视图分离 |
| Context 质量预检 | 无 | 规则 Preflight 四维评分 + warn-only |
| System 注入结构 | 平铺 AGENTS.md | Session Context + loaded_context XML 分层 |

**AgentScope 补充痛点：**

| 痛点 | AgentScope 做法 | Devrix 缺口 |
|------|-----------------|-------------|
| 能力正交扩展 | Hook priority + 共享对象 | Process 内步骤耦合 |
| 压缩前记忆沉淀 | flushBeforeCompact → daily log | Autocompact 前无 fact 提取 |
| 大工具结果 | ToolResultEviction 落盘占位 | 仅 step1 tool_result_budget 截断 |
| 上下文溢出 | forceCompactAndRetry | TokenBlock 直接 error |
| 工具面动态修复 | Preflight ToolFilter auto-repair | 无 pre-reasoning 工具裁剪 |

## Proposed Solution

在 `internal/layers/contextengine/harness/` 新增 **Harness Bootstrap** 子系统，在 `ContextEngine.Process()` 主路径 **之前**（或会话首次 Process 时）执行：

```
HarnessBootstrap.Run(ctx, session, opts)
  1. prefetch        — 工作区扫描（文件计数、根路径）
  2. workspace_ctx   — 构建 WorkspaceContext 值对象
  3. tool_pool       — 按配置裁剪 IToolRegistry 可见工具集
  4. deferred_init   — trusted 时加载 plugin/skill/MCP（V5a: stub）
  5. [optional] route — PromptRouter 生成 routing hints
  → 输出 HarnessSessionState 供 PEV / ContextAssembler 消费
```

**TranscriptStore** 独立管理原始 user/assistant turn 序列；`SessionContext.Messages` 仍作为压缩与 PEV 的 SoT，两者通过 `Process` 同步。

## Capabilities

| Capability | L4 映射 | PR | 说明 |
|------------|---------|----|------|
| harness-bootstrap | L4-CTX-HARNESS | PR1 | 阶段图编排、BootstrapReport、与 Process 集成 |
| tool-pool | L4-CTX-TOOLPOOL | PR1 | simple_mode / MCP / deny 裁剪 |
| prompt-router | L4-CTX-ROUTER | PR1 | 关键词匹配 command/tool（可选） |
| context-preflight | L4-CTX-PREFLIGHT | PR1 | 规则评分 + tool filter auto-repair |
| workspace-injector | L4-CTX-WORKSPACE | PR2 | Session Context + loaded_context 结构化注入 |
| transcript-store | L4-CTX-TRANSCRIPT | PR2 | append / compact / replay / dual-file log |

## Scope

### In Scope

**PR1「内核」:**
- harness-bootstrap + tool-pool + prompt-router + context-preflight 四项 Capability 的设计与规格
- `context_engine.harness` 配置块（tool_pool / routing / deferred_init 子节）
- L5-2-9-01 ~ 06 测试点草拟

**PR2「装配」:**
- workspace-injector + transcript-store 两项 Capability 的设计与规格
- `context_engine.harness` 配置块（transcript / preflight 子节）
- L5-2-9-07 ~ 11、L5-5-5-02 测试点草拟
- `docs/context-engine-design.md` 附录增量（S3 后 S4 同步）

### Out of Scope
- V5b：真实 plugin/MCP/skill、multi-mode routing
- Parity audit、Python 移植

## Impact Analysis

| 组件 | 变更 | 详情 |
|------|------|------|
| `contextengine/engine.go` | 是 | Process 开头条件调用 HarnessBootstrap |
| `contextengine/contracts.go` | 是 | 新增 IHarnessBootstrap、IDeferredInit 等 |
| `bootstrap/context_engine.go` | 是 | 注入 Harness 依赖 |
| `shared/config/contextengine.go` | 是 | HarnessConfig |
| `shared/types/context.go` | 是 | HarnessSessionState、TranscriptEntry |
| `openspec/specs/context-engine/spec.md` | 否（S7） | 本变更 delta → 归档时 merge |
| Observability | 是 | 6 Jaeger Operation + registry + L5-2-9-11 |
| 数据库 | 否 | Transcript 可选文件持久化（V5a 内存） |

## Architecture Considerations

- **不破坏 V4 默认路径**：`harness.enabled=false` 时零行为变化
- **Accept Interfaces**：`IHarnessBootstrap`、`IToolPoolFilter`、`IPromptRouter`、`IDeferredInit`
- **Observability**：每 bootstrap stage 一个 internal span（对齐 claw-code HistoryLog 语义）
- **依赖方向**：harness 子包不得 import communication / adapters

## Success Criteria (S3 准出)

- [ ] demand.md 澄清完成，L1-L5 映射无悬空 ID（**L3-BE-CTX-04**，非 CTX-03）
- [ ] proposal / design / tasks / delta spec 四件套完整
- [ ] **`openspec validate devrix-harness-bootstrap` 通过**
- [ ] 每个 L4 能力 ≥1 L5 测试点（Gherkin Scenario）
- [ ] V5a / V5b 分期明确
- [ ] L5-2-9-11（Jaeger span 树）与 L5-5-5-02 测试计划写入 tasks.md
- [ ] 压缩 messages-only + Build 时序写入 design §1.3 与 delta spec

## Risks & Mitigations

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| 双轨 Messages + Transcript 不一致 | 中 | 中 | Process 末尾单点 sync；集成测试覆盖 |
| ToolPool 与 PermissionGate 双重过滤困惑 | 低 | 中 | 文档 + info 事件展示最终可见工具数 |
| 启动延迟增加 | 中 | 低 | prefetch 仅首次/session；可配置 skip |
| Routing 误匹配 | 中 | 低 | 默认 disabled；仅注入 hint 不强制调用 |
| Bootstrap 误重复执行 | 低 | 中 | HarnessInitialized 标志 + 幂等 L5 |
| 与 observability-enhancement 冲突 | 中 | 中 | PR2 前合并 ctx 传播规范；registry 一次改 |

## Version Roadmap

| 版本 | 增量 |
|------|------|
| V5a PR1「内核」（本变更） | Bootstrap + ToolPool + Router + Preflight | 
| V5a PR2「装配」（本变更） | Transcript + WorkspaceInjector + Jaeger + 集成测试 |
| V5b | DeferredInit 实装；SLM Preflight；`.devrix/workspace/` 目录 |
| V6 | flush-before-compact；ToolResultEviction；forceCompactAndRetry |

## AgentScope Design Patterns Adopted

| 模式 | 来源 | Devrix V5a 落地 |
|------|------|------------------|
| Thin Wrapper | HarnessAgent 不替换 ReAct | Bootstrap 不替换 PEV |
| Hook priority | Compaction(10)→Workspace(900) | Bootstrap stage priority 常量 |
| RuntimeContext | 每 call 注入 sessionId | `ProcessRuntimeContext` 值对象 |
| WorkspaceContextHook | PreCall 一次性 system 注入 | WorkspaceInjector 在压缩后、PEV 前 |
| Dual session files | compact `.jsonl` + full `.log.jsonl` | TranscriptStore + SessionLog |
| Context Preflight | RuleBasedPreflightChecker | 四维规则评分 warn-only |
| Shared objects | WorkspaceManager + RuntimeContext | HarnessSessionState + WorkspaceContext |

---

## Archive Information

**Archived:** 2026-06-10
**Duration:** 1 day (2026-06-09 demand → 2026-06-10 merge)
**Outcome:** Successfully implemented
**PR:** [#10](https://github.com/fqntxmqee/devrix/pull/10)

### Specs Updated
- `openspec/specs/context-engine/spec.md` — v5.0.0 Harness Bootstrap requirements
- `openspec/specs/observability/spec.md` — v1.4.0 harness Jaeger operations

### Archive Location
- `openspec/archive/2026-06-10-devrix-harness-bootstrap/`
