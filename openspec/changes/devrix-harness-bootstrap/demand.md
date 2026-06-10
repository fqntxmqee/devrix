---
demand-id: DM-20260609-004
title: Harness Bootstrap 上下文装配层（Claw-code + AgentScope）
source: 架构评审 / 产品
priority: P1
status: CLARIFYING
l1-domain: devrix
created: 2026-06-09
---

# Harness Bootstrap 上下文装配层（Claw-code + AgentScope）

## 1. 原始描述

> Devrix 现有 Layer 2 上下文引擎（V4）擅长 **会话认知态管理**：消息历史、七步压缩、PEV Execute→Verify、快照与 LongTerm 记忆。
> 对标项目 **claw-code**（Claude Code Harness 洁净室移植）在 **Harness 启动装配** 上更完整：分阶段 bootstrap、trust-gated deferred init、ToolPool 裁剪、prompt 前置路由、Transcript 生命周期分离。
>
> 工作区内 **AgentScope Java**（`agentscope-harness` + `agentscope-agent`）在 **Hook 正交编排、Workspace 注入、Compaction 与 Preflight** 上也有成熟实践，应一并借鉴。
>
> 需要在 Devrix 中引入 **Harness Bootstrap 子能力**，吸收 claw-code + AgentScope 优势，与现有 ContextEngine 互补，而非替换 PEV/压缩/记忆管线。
>
> 参考：claw-code `src/{bootstrap_graph,runtime,setup,...}.py`；AgentScope `docs/en/harness/architecture.md`、`agentscope-agent` Context Preflight。

## 2. 澄清记录

### Q1: 新能力是独立 Layer 还是 Context Engine 子模块？
**A**: 作为 **D2 Context Engine 域内新场景 D2-S9 Harness Bootstrap**，包路径 `internal/layers/contextengine/harness/`。不新增 L1 层，避免与通信层 Gateway 职责重叠。 — 2026-06-09

### Q2: V5 首批实现范围？
**A**: **V5a（本变更）** 实现：Bootstrap 阶段图、WorkspaceContext 扫描、ToolPool 裁剪（simple_mode / MCP / deny）、Trust-gated deferred init 框架（plugin/skill/MCP 为 stub 接口）、PromptRouter 可选前置路由、TranscriptStore 与 Messages 分离、Bootstrap 可观测事件。**V5b（后续）** 再对接真实 plugin/MCP/skill 加载与 multi-mode routing（local/remote/ssh）。 — 2026-06-09

### Q3: 与现有 IPermissionGate 关系？
**A**: **互补**：Bootstrap 在 **会话/轮次开始前** 裁剪工具面（deny 列表、simple_mode）；`IPermissionGate` 仍在 **单次 tool_call 执行前** 做风险审批。两者均生效，Bootstrap 先过滤 registry 可见集。 — 2026-06-09

### Q4: Pre-LLM Routing 是否替代 LLM tool calling？
**A**: **不替代**。Routing 为可选优化：高置信度匹配时缩短工具 schema 或注入 routing hint；默认 `routing.enabled=false` 保持 V4 行为。 — 2026-06-09

### Q5: 配置与灰度策略？
**A**: 新增 `context_engine.harness` YAML 块；默认 `enabled=false`，与 `plan.enabled` 相同灰度模式。 — 2026-06-09

### Q6: claw-code parity audit 是否纳入？
**A**: **Out of Scope**。Devrix 不自建 TS snapshot parity；仅借鉴 **架构模式**，不复制 reference_data JSON。 — 2026-06-09

### Q7: AgentScope 借鉴范围？
**A**: **V5a** 借鉴：Hook 优先级阶段编排、RuntimeContext 每轮注入、Workspace 结构化 system 注入（Session Context + loaded_context XML）、双轨 session 文件（compact vs log）、规则 Preflight 评分。**V5b/V6** 再考虑：Compaction flush-before-compact、ToolResultEviction、SLM Preflight、FTS memory_search。 — 2026-06-09

### Q8: 是否引入 Hook 框架替换 PEV？
**A**: **否**。遵循 AgentScope「Thin Wrapper」哲学：PEV 仍是推理核；Bootstrap/Preflight 作为 **Process 前/压缩前的正交阶段**，通过 priority 排序，不替换 Execute→Verify。 — 2026-06-09

### Q9: Context Preflight 默认行为？
**A**: 默认 `preflight.mode=warn-only`（对齐 agentscope-agent）；仅写 trace/info，不阻断 Process；`block` 模式 V5b 再开。Tool filter 默认 `auto-repair`（裁剪无关工具 schema）。 — 2026-06-09

### Q10: Workspace 目录是否强制？
**A**: V5a **不强制**新目录结构；继续读项目根 `AGENTS.md`。V5b 可选支持 `.devrix/workspace/` 子树（MEMORY.md、knowledge/），与 AgentScope workspace 布局对齐。 — 2026-06-09

### Q11: V5a 是否应拆为多个 PR？
**A**: **是**。V5a 整体工作量较大，分两个 PR 交付以降低单分支变更风险与 review 负担：
- **PR1「内核」**：M1 配置类型 + M2 Bootstrap 核心 + M3 ToolPool/Router/Preflight + 轻量集成测试。产出独立可测试的 `harness/` 包骨架。
- **PR2「装配」**：M4 Process 集成 + M5 集成/验收测试 + M6 Jaeger 埋点 + M7 文档。依赖 PR1 接口，对接现有 ContextEngine 流水线。
- 两个 PR **共享同一 Change ID**，PR1 合入后 PR2 在其基础上增量。
- **归档策略**：PR1 合入仅 S4 Gate，不归档；待 PR2 合入后统一 S5 验收 + S6 一次归档。 — 2026-06-10

### Q12: L3-BE-CTX-03 是否可用于 Harness？
**A**: **否**。`L3-BE-CTX-03` 已在 V3 登记为「复杂任务分解与里程碑推进」。Harness 使用 **`L3-BE-CTX-04`（会话 Harness 启动与工具面装配）**。 — 2026-06-10

### Q13: 压缩与 System Prompt 组装时序？
**A**: **方案 A（采纳）**：压缩 **仅处理 Messages**；`SystemPromptAssembler.Build` 在压缩之后、PEV 之前；CompressedView 的 system 段使用 Build 输出。禁止在 Build 前依赖完整 XML system 做压缩 token 估算。 — 2026-06-10

### Q14: simple_mode 工具名映射？
**A**: Devrix builtin 为 `bash`, `read_file`, `write_file`（非 claw-code 的 read/edit）。ToolPool allowlist 必须使用 Devrix registry 真实名称。 — 2026-06-10

### Q15: PR1 Preflight 边界？
**A**: PR1 Preflight 使用 **provisional context**（agentsRaw + memory entries），完整 XML 评分在 PR2 Assembler 就绪后扩展；PR1 必须实现 tool filter 子能力。 — 2026-06-10

### Q16: OpenSpec 结构是否合规？
**A**: 原 `specs/*_delta.md` 扁平文件导致 `openspec validate` 失败；已迁移为 `specs/context-engine/spec.md` + `specs/observability/spec.md`，使用 `## ADDED Requirements` 标准头。 — 2026-06-10

### Q17: 与 observability-enhancement 协同？
**A**: Harness PR2 埋点 MUST 采用 span ctx 传播规范（`ctx, span := startSpan` 并向下传递）；L5-2-9-11 与 L5-OBS-TRACE-04 层级断言可对齐。 — 2026-06-10

## 3. 澄清范围

### 3.1 L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | devrix | 开发大脑 | 已有 |
| L2 | L2-DEVRIX-02 | 对话式开发助手 | 已有 |
| L3-BE | L3-BE-CTX-04 | 会话 Harness 启动与工具面装配 | **新增** |
| L3-BE | L3-BE-CTX-01 | 处理用户消息并维护上下文 | 已有（扩展接入点） |
| L4-BE | L4-CTX-HARNESS | Harness Bootstrap 编排 | **新增** |
| L4-BE | L4-CTX-TOOLPOOL | 工具池裁剪与权限上下文 | **新增** |
| L4-BE | L4-CTX-ROUTER | Prompt 前置路由 | **新增** |
| L4-BE | L4-CTX-TRANSCRIPT | Transcript 生命周期 | **新增** |
| L4-BE | L4-CTX-PREFLIGHT | Context Preflight 规则评分 | **新增** |
| L4-BE | L4-CTX-WORKSPACE | Workspace 结构化注入 | **新增** |
| L4-BE | L4-CTX-PEV / COMPRESS / MEMORY | 现有能力 | 已有（V6 扩展 flush/eviction） |
| L5 | L5-2-9-01 ~ L5-2-9-15, L5-5-5-02 | Harness Bootstrap + Jaeger 测试点 | **草拟** |

### 3.2 范围

**In Scope（V5a — PR1「内核」）**:
- OpenSpec 四件套 + delta spec + L5 草拟登记
- Harness Bootstrap 阶段图与 `Run()` 编排
- WorkspaceContext（工作区元信息扫描）
- ToolPool：simple_mode、include_mcp、deny_names、deny_prefixes
- Trust-gated DeferredInit 接口 + NoOp/stub 实现
- PromptRouter：关键词匹配 command/tool hint（可选）
- ContextPreflight：规则四维评分（relevance/completeness/safety/tokenBudget）+ warn-only（provisional context）
- 对应单元测试 + 轻量集成测试

**In Scope（V5a — PR2「装配」）**:
- TranscriptStore：append / compact / replay；**双文件** compact view + append-only log（AgentScope 模式）
- WorkspaceInjector：Session Context + `<loaded_context>` 结构化 system 块（**SoT：`design.md` §十**）
- RuntimeContext：每 Process 注入 sessionId/userId/extra（不持久化）
- `ContextEngine.Process` 接入点（harness.enabled 时）
- Jaeger span 树 + registry 对账（§十一、§十二；`observability_delta.md`）
- 集成/验收测试，标注 `Covers: L5-2-9-*`
- 文档更新 + L5 registry 登记

**Out of Scope（V5a）**:
- 真实 plugin / skill / MCP 动态加载（V5b）
- multi-mode routing（local/remote/ssh/teleport/deep-link）
- claw-code parity audit 框架
- SLM Preflight（Ollama 小模型二审，V5b）
- Compaction flush-before-compact / ToolResultEviction（V6，对齐 AgentScope CompactionHook）
- forceCompactAndRetry on LLM ContextOverflow（V6）
- 强制 `.devrix/workspace/` 目录树（V5b 可选）

**Out of Scope（永久）**:
- 复制 claw-code 或 Claude Code 专有源码
- 引入 Python 运行时依赖
