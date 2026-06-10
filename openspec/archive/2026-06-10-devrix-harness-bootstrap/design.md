# Context Engine V5 Design: Harness Bootstrap

**Change ID:** devrix-harness-bootstrap
**Demand ID:** DM-20260609-004
**Layer:** 2 - Context Engine (D2-S9)
**Status:** Draft
**Version:** 5.0.0-draft（2026-06-10 修订：Review 澄清 + OpenSpec 对齐）
**Based on:** `openspec/specs/context-engine/spec.md` v4.0.0, claw-code harness patterns, AgentScope Java harness architecture

---

## 〇、Review 决议摘要（2026-06-10）

| 议题 | 决议 |
|------|------|
| L3 ID | **L3-BE-CTX-04**（CTX-03 已被 V3 Milestone 占用） |
| 压缩 vs Build | **messages-only 压缩 → Build → PEV**（§1.3、§10.10） |
| simple_mode 工具 | `bash`, `read_file`, `write_file` |
| PR1 Preflight | provisional context + tool filter；完整 XML 评分 PR2 |
| OpenSpec delta | `specs/context-engine/spec.md` + `specs/observability/spec.md` |
| Span 传播 | harness 埋点 MUST ctx 向下传递（对齐 observability-enhancement） |

---

## 一、架构目标

### 1.1 业务目标

| 目标 | V4 | V5a | 用户可感知结果 |
|------|----|----|----------------|
| Harness 分阶段启动 | 无 | Bootstrap 阶段图 | 会话启动日志可解释 |
| 工具面裁剪 | 全量 | ToolPool 过滤 | simple 模式更快、更少误调用 |
| Trust 延迟加载 | 无 | DeferredInit stub | 非 trusted 不加载扩展面（V5b 实装） |
| Pre-LLM 路由 | 无 | 可选 PromptRouter | 高频命令减少 LLM 选工具成本 |
| Transcript 分离 | 无 | TranscriptStore | 调试/审计可见原始 turn |

### 1.2 层间边界

```
Layer 2 Context Engine
├── harness/          ← V5 新增 (D2-S9)
│   ├── bootstrap.go      编排
│   ├── workspace.go      WorkspaceContext
│   ├── toolpool.go       ToolPoolFilter
│   ├── router.go         PromptRouter
│   ├── deferred.go       IDeferredInit
│   └── transcript.go     TranscriptStore
├── engine.go         ← Process 接入 harness.Run
├── pev/              ← 消费 ToolPool 裁剪后的 schema
├── compression/      ← 不变
└── memory/           ← 不变；Transcript 可选 persist V5b
```

**禁止：**
- harness 包 import `communication/*`
- Bootstrap 修改 Gateway 或 Adapter 代码

### 1.3 Process 集成序列（V5a — 修订）

```
ContextEngine.Process(session, message)
  │
  ├─ memory.LoadOrInit(session)          # 恢复 Messages；不写入最终 SystemPrompt
  │
  ├─ if harness.enabled && !session.HarnessInitialized:
  │     HarnessBootstrap.Run → HarnessSessionState（VisibleTools 缓存）
  │     emit info events (bootstrap stages)
  │
  ├─ agentsRaw := prompt.Load(WorkDir)   # 仅读 AGENTS.md
  ├─ memoryEntries := LongTerm.Recall()  # 返回 entries，不再 sc.SystemPrompt +=
  ├─ AppendUserMessage(message)
  │
  ├─ if shouldCompress:
  │     compressionPipeline.Run(msgs ONLY)  # ⚠️ 不传最终 system；token 估算仅 messages
  │
  ├─ if preflight.enabled:
  │     PreflightEvaluator(provisionalContext = agentsRaw + memoryEntries)
  ├─ if routing.enabled:
  │     RoutingHint := PromptRouter.Route(message, VisibleTools)
  │
  ├─ sc.SystemPrompt = SystemPromptAssembler.Build(...)  # priority 900，见 §十
  ├─ SetCompressedView(system=Build output + compressed messages)
  │
  ├─ pev.Run(ctx, sc, sc.CompressedView, message, VisibleTools, ...)
  │
  └─ transcript.Append(userMessage, assistantSummary)   # 成功路径 only
```

**时序约束（MUST）**：
1. 压缩 **不得** 依赖 Assembler 产出的 XML system
2. `CompressedView` 的 system 段 **必须** 等于 Build 输出
3. LongTerm recall **不得** 再使用 `sc.SystemPrompt += appendix`

`harness.enabled=false` 时：Build 退化为 V4（`prompt.Load` + `FormatLongTermAppendix`），行为 bit-identical。

---

## 二、Bootstrap 阶段图

对齐 claw-code `build_bootstrap_graph()`，V5a 实现前 5 阶段：

| Stage | ID | 职责 | V5a 实现 |
|-------|-----|------|----------|
| 1 | `prefetch` | 工作区 side effects | 扫描 WorkDir 文件计数 |
| 2 | `guards` | 环境检查 | Go version、WorkDir 可读写 |
| 3 | `setup` | 并行加载 command/tool 元数据 | 从 IToolRegistry.ListTools |
| 4 | `deferred_init` | trust-gated 扩展加载 | stub：记录 enabled 标志 |
| 5 | `tool_pool` | 裁剪可见工具集 | ToolPoolFilter.Apply |
| 6 | `mode_routing` | local/remote/ssh | **V5b** |
| 7 | `query_loop` | 交给 Process/PEV | 已有 |

### 2.1 Stage Priority（对齐 AgentScope Hook priority）

Bootstrap 阶段与后续注入按 **priority 升序** 执行，确保 system prompt **最后** 组装：

| Priority | Stage / Hook | 职责 |
|----------|--------------|------|
| 10 | prefetch / guards | 环境扫描 |
| 20 | setup / deferred_init / tool_pool | 工具面装配 |
| 30 | preflight | 上下文质量评分 + tool filter |
| 40 | routing | advisory hints |
| 50 | compression (V4) | 七步压缩 |
| 900 | workspace_inject | Session Context + loaded_context（末位） |
| — | pev | Execute→Verify |

```go
type BootstrapStage string

const (
    StagePrefetch     BootstrapStage = "prefetch"
    StageGuards       BootstrapStage = "guards"
    StageSetup        BootstrapStage = "setup"
    StageDeferredInit BootstrapStage = "deferred_init"
    StageToolPool     BootstrapStage = "tool_pool"
)

type BootstrapReport struct {
    StagesApplied []BootstrapStage
    Workspace     WorkspaceContext
    ToolCount     int // before filter
    VisibleTools  int // after filter
    Trusted       bool
    Duration      time.Duration
}
```

---

## 三、核心类型

### 3.1 WorkspaceContext

```go
type WorkspaceContext struct {
    WorkDir         string
    SourceRoots     []string
    PythonFileCount int // 0 if N/A
    TestFileCount   int
    AgentsMDPresent bool
    ScannedAt       time.Time
}
```

构建逻辑：Walk WorkDir（深度限制 configurable），统计 `*.go` / `*_test.go`，检测 `AGENTS.md`。

### 3.2 ToolPoolFilter

```go
type ToolPoolConfig struct {
    SimpleMode      bool     `yaml:"simple_mode"`
    IncludeMCP      bool     `yaml:"include_mcp"`
    DenyNames       []string `yaml:"deny_names"`
    DenyPrefixes    []string `yaml:"deny_prefixes"`
}

func (f *ToolPoolFilter) Filter(all []ToolSchema, cfg ToolPoolConfig) []ToolSchema
```

**simple_mode 默认保留（Devrix registry 真实名称）：** `bash`, `read_file`, `write_file`（claw-code `read`/`edit` 映射见 demand Q14）。

**MCP 过滤：** tool name 或 description 含 `mcp`（case insensitive）时排除。

### 3.3 Trust-gated DeferredInit

```go
type IDeferredInit interface {
    Run(ctx context.Context, trusted bool, session *types.Session) (DeferredInitResult, error)
}

type DeferredInitResult struct {
    PluginInit   bool
    SkillInit    bool
    MCPPrefetch  bool
    SessionHooks bool
}
```

V5a：`NoOpDeferredInit` — trusted=true 时四项均为 true（标志位），无实际 IO。

### 3.4 PromptRouter

```go
type RoutingHint struct {
    Commands []string
    Tools    []string
    Scores   map[string]int
}

func (r *PromptRouter) Route(prompt string, tools []ToolSchema, limit int) RoutingHint
```

算法：token 化 prompt，对 tool/command name + description 做 substring 计分（claw-code `_score` 同等逻辑）。

**注入方式：** 写入 `<routing_hints advisory="true">`（§10.7.4），不强制 PEV 调用。

### 3.5 TranscriptStore

```go
type TranscriptStore struct {
    Entries []TranscriptEntry
    Flushed bool
}

type TranscriptEntry struct {
    Role      types.MessageRole
    Content   string
    Timestamp time.Time
}

func (t *TranscriptStore) Append(role, content)
func (t *TranscriptStore) Compact(keepLast int)
func (t *TranscriptStore) Replay() []TranscriptEntry
```

挂载点：`SessionContext.Transcript *TranscriptStore`（optional nil = V4 兼容）。

**双文件模型（AgentScope Session 对齐）：**

| 文件 | 用途 | Devrix V5a |
|------|------|------------|
| `<sessionId>.jsonl` | LLM 可见 compact 上下文 | `CompressedView` 序列化（已有 snapshot） |
| `<sessionId>.log.jsonl` | append-only 完整对话 | `SessionLog` 新类型，Transcript 落盘 |

---

## 3.6 ProcessRuntimeContext（AgentScope RuntimeContext）

```go
type ProcessRuntimeContext struct {
    SessionID string
    UserID    string
    RequestID string
    Extra     map[string]string // 不持久化
}
```

每 `Process()` 构造，供 Preflight / WorkspaceInjector / Observer 使用；**不写入 ContextSnapshot**。

---

## 3.7 Context Preflight（agentscope-agent 对齐）

```go
type PreflightScores struct {
    Relevance    int // 0-100
    Completeness int
    Safety       int
    TokenBudget  int
}

type PreflightResult struct {
    Scores       PreflightScores
    Warnings     []string
    ToolFilter   ToolFilterDecision
    Mode         string // warn-only | block
}

func (e *PreflightEvaluator) Evaluate(
    sc *types.SessionContext,
    userMessage string,
    visibleTools []ToolSchema,
    assembledContext string,
) PreflightResult
```

**规则（V5a，无 SLM）：**
- completeness：空消息 → 0 分
- safety：prompt 注入正则 + 敏感词关键词
- tokenBudget：`len(assembled)/4` vs `preflight.token_budget` × warn_ratio
- tool filter：`ToolRelevanceFilter` 排除与 userMessage 无关工具（auto-repair）

默认 `mode=warn-only`：仅 `info` 事件 + observer，不阻断 Process。

---

## 3.8 WorkspaceInjector

`WorkspaceInjector` 是 **§十 System Prompt Assembly Spec** 的实现入口；在压缩后、PEV 前调用 `SystemPromptAssembler.Build()`，产出最终 `sc.SystemPrompt`。

V5a 不重复定义模板与预算公式——以 §十 为 SoT。

---

## 十、System Prompt Assembly Spec

> **SoT**：V5a system prompt 组装的唯一规范。实现：`internal/layers/contextengine/harness/system_prompt_assembler.go`  
> **借鉴**：AgentScope `WorkspaceContextHook`（分层 + XML + token budget）；claw-code `build_system_init_message` / `render_context`（运行时 harness 块）

### 10.1 设计原则

| 原则 | 说明 |
|------|------|
| **人格与运行时分离** | `AGENTS.md` = 项目规约；Harness/Session 块 = 每轮动态事实 |
| **最后组装** | priority 900：LongTerm / Bootstrap / Preflight / Routing 就绪后再 `Build()` |
| **每 Process 一次** | 不在 PEV 每轮 iteration 重复 append system |
| **分区预算** | 固定块优先；`memory_context` 吃剩余 token；截断带恢复指引 |
| **harness 灰度** | `enabled=false` → V4 退化路径，不输出 XML |

### 10.2 四层结构

最终 `sc.SystemPrompt` 由以下四层 **顺序拼接**（空层省略）：

```
┌─ Layer 0: devrix_core          （框架固定模板，中文）
├─ Layer 1: session_context      （每 Process 动态）
├─ Layer 2: workspace_guidance   （框架固定模板，中文）
└─ Layer 3: loaded_context       （XML 容器，内含各 context 子块）
```

**完整形态示例（缩略）：**

```markdown
<!-- Layer 0 -->
你是 Devrix，多智能体开发助手。遵循 PEV 循环…

<!-- Layer 1 -->
## Session Context
Agent: Devrix
Today's date: Monday Jun 9, 2026
Operating system: darwin 25.3.0
Workspace directory: /path/to/project
Session ID: sess_abc
Request ID: req_xyz
Model: claude-sonnet

<!-- Layer 2 -->
## Workspace Guidance
- 项目规约见下方 `<agents_context>`…
- 回答历史决策前先依赖 `<memory_context>`…
…

<!-- Layer 3 -->
## Workspace Files (Injected)
The following <loaded_context> was loaded from workspace and harness runtime.

<loaded_context>
  <agents_context>...</agents_context>
  <memory_context>...</memory_context>
  <harness_init>...</harness_init>
  <workspace_snapshot>...</workspace_snapshot>
  <routing_hints advisory="true">...</routing_hints>
  <preflight_warnings>...</preflight_warnings>
</loaded_context>
```

### 10.3 Block 目录（Layer 3 子块）

| XML Tag | 来源 | 必填 | V5a | 说明 |
|---------|------|------|-----|------|
| `agents_context` | `prompt.Loader(WorkDir)` | 否 | ✅ | AGENTS.md / .devrix/AGENTS.md / fallback |
| `memory_context` | LongTerm Recall 或 MEMORY.md | 否 | ✅ | Recall 结果格式化；V5b 可读 workspace MEMORY.md |
| `harness_init` | `BootstrapReport` | 否 | ✅ | claw-code System Init 风格 |
| `workspace_snapshot` | `WorkspaceContext` | 否 | ✅ | claw-code PortContext 风格（Go 项目字段） |
| `routing_hints` | `RoutingHint` | 否 | 可选 | `advisory="true"`，不强制工具调用 |
| `preflight_warnings` | `PreflightResult.Warnings` | 否 | 可选 | 仅 WARN 时注入；不含 numeric scores |
| `domain_knowledge_context` | `knowledge/` | 否 | **V5b** | AgentScope 知识目录 |

**空块规则：** content 为 blank 时输出自闭合 tag：`<agents_context></agents_context>`（与 AgentScope 一致）。

### 10.4 Layer 0 — Devrix Core（框架模板）

**路径：** `internal/layers/contextengine/harness/templates/devrix_core.zh.md`（嵌入或 embed）

**Must 覆盖（不得依赖 AGENTS.md）：**

- Devrix 角色边界（开发助手，非通用聊天）
- PEV：Execute → Verify；工具失败可重试
- 权限：高风险工具需用户批准（Gateway 展示 + L2 Gate）
- 压缩：历史可能被截断，以最近消息与 loaded_context 为准
- LongTerm：跨会话记忆在 `memory_context`，非完整历史

**Must NOT：** 项目特定栈、目录约定、团队规范（留给 `agents_context`）。

### 10.5 Layer 1 — Session Context（动态模板）

```go
const sessionContextTemplate = `## Session Context
Agent: {{.AgentName}}
Today's date: {{.DateHuman}}
Operating system: {{.OS}}
Workspace directory: {{.WorkDir}}
Session ID: {{.SessionID}}
Request ID: {{.RequestID}}
Model: {{.Model}}
`
```

| 字段 | 来源 |
|------|------|
| `AgentName` | 固定 `Devrix` 或 config |
| `DateHuman` | `time.Now()`，`Monday Jun 9, 2026` 格式（AgentScope 同款） |
| `OS` | `runtime.GOOS` + 可选版本 |
| `WorkDir` | `session.WorkDir` 绝对路径 |
| `SessionID` / `RequestID` | `ProcessRuntimeContext` |
| `Model` | `session.Model` |

### 10.6 Layer 2 — Workspace Guidance（框架模板）

**路径：** `templates/workspace_guidance.zh.md`

**Must 覆盖：**

| 主题 | 指引要点 |
|------|----------|
| AGENTS | `<agents_context>` 为项目规约 SoT |
| Memory | 优先读 `memory_context`；不足时依赖 LongTerm recall API（V3） |
| Knowledge | V5b：`knowledge/` 按需 read，勿整库灌入 prompt |
| Tools | 可见工具集已由 Harness 裁剪；advisory routing 不强制 |
| Files | 文件操作限定 WorkDir（Sandbox 规则） |

### 10.7 Layer 3 子块模板

#### 10.7.1 `harness_init`（claw-code）

```
Trusted: {{.Trusted}}
Visible tools: {{.VisibleTools}} (filtered from {{.TotalTools}})
Deferred init: plugin={{.PluginInit}} skill={{.SkillInit}} mcp={{.MCPPrefetch}} hooks={{.SessionHooks}}
Bootstrap stages: {{.StagesApplied}}
```

#### 10.7.2 `workspace_snapshot`（claw-code → Go）

```
WorkDir: {{.WorkDir}}
AGENTS.md present: {{.AgentsMDPresent}}
Go source files: {{.GoFileCount}}
Go test files: {{.GoTestFileCount}}
Module: {{.GoModulePath}}          # 从 go.mod 解析，optional
Scanned at: {{.ScannedAt RFC3339}}
```

#### 10.7.3 `memory_context`

V5a 由 `memory.FormatLongTermAppendix` 重构为 **无外层 `## 项目记忆` 标题** 的 bullet 列表，放入 XML：

```
- [topic] content preview...
```

截断 notice（AgentScope 同款，中文）：

```
... (记忆已截断 — 更多内容请依赖 LongTerm recall 或项目文档) ...
```

#### 10.7.4 `routing_hints`

```xml
<routing_hints advisory="true">
Matched tools: read, bash
Matched commands: none
Note: These hints are advisory only; you may still choose other tools.
</routing_hints>
```

`routing.enabled=false` 或零匹配：**省略整个 tag**（非空 tag）。

#### 10.7.5 `preflight_warnings`

```xml
<preflight_warnings>
- 估算 token 接近预算上限
- 消息可能包含敏感信息关键词
</preflight_warnings>
```

**禁止**注入 `PreflightScores` 数值（仅 trace/info 事件）。无 warning 时省略 tag。

### 10.8 Token 预算算法

**配置：**

- `context_engine.harness.workspace.max_context_tokens`（默认 `8000`）— 仅约束 **Layer 3 XML 内** 可变长块
- Layer 0/1/2 不计入该 budget（通常 <2k tokens，单独 observability 记录）

**估算：** `estimateTokens(s) = max(1, len(s)/4)`（与 AgentScope / Devrix counter 对齐）

**分配顺序（Layer 3 内部）：**

```
budget := max_context_tokens
fixed := estimateTokens(harness_init) + estimateTokens(workspace_snapshot)
         + estimateTokens(routing_hints) + estimateTokens(preflight_warnings)
agents := min(estimateTokens(agentsRaw), budget - fixed)
budget -= fixed + agents
memory := min(estimateTokens(memoryRaw), max(0, budget))
```

**截断实现：**

```go
func truncateToTokenBudget(text string, maxTokens int) string {
    maxChars := maxTokens * 4
    if len(text) <= maxChars {
        return text
    }
    return text[:maxChars] + memoryTruncationNoticeZH
}
```

**优先级：** `agents_context` 全量优先（在 budget 允许内）→ `memory_context` 使用剩余 → 动态块（harness/snapshot/routing/warnings）先扣 fixed。

**Observability（BuildReport）：**

| 字段 | 说明 |
|------|------|
| `system_prompt.total_tokens` | 四层合计 |
| `system_prompt.layer0_tokens` … `layer3_tokens` | 分层 |
| `system_prompt.agents_tokens` / `memory_tokens` | XML 子块 |
| `system_prompt.memory_truncated` | bool |

### 10.9 组装 API

```go
// harness/system_prompt_assembler.go

type SystemPromptBuildInput struct {
    WorkDir          string
    Session          *types.Session
    Runtime          ProcessRuntimeContext
    AgentsRaw        string              // prompt.Loader output
    MemoryEntries    []memory.MemoryEntry
    Bootstrap        *BootstrapReport    // nil if harness disabled
    Workspace        *WorkspaceContext
    Routing          *RoutingHint        // nil if disabled
    Preflight        *PreflightResult    // nil if disabled
    HarnessEnabled   bool
}

type SystemPromptBuildReport struct {
    TotalTokens      int
    LayerTokens      [4]int
    MemoryTruncated  bool
    BlocksIncluded   []string            // XML tag names
}

type SystemPromptAssembler struct {
    coreTemplate      string
    guidanceTemplate  string
    cfg               *config.WorkspacePromptConfig
    counter           contracts.ITokenCounter // 可选，默认 chars/4
}

func (a *SystemPromptAssembler) Build(in SystemPromptBuildInput) (prompt string, report SystemPromptBuildReport)

// V4 退化
func (a *SystemPromptAssembler) BuildLegacy(agentsRaw string, memoryAppendix string) string
```

**XML 构建：**

```go
func buildLoadedContext(blocks map[string]string) string {
    var b strings.Builder
    b.WriteString("<loaded_context>\n")
    for _, tag := range []string{
        "agents_context", "memory_context", "harness_init",
        "workspace_snapshot", "routing_hints", "preflight_warnings",
    } {
        content, ok := blocks[tag]
        if !ok && tag optional { continue }
        b.WriteString(buildXMLContext(tag, content))
    }
    b.WriteString("</loaded_context>\n")
    return b.String()
}

func buildXMLContext(tag, content string) string {
    content = strings.TrimSpace(content)
    if content == "" {
        return "  <" + tag + "></" + tag + ">\n"
    }
    return "  <" + tag + ">\n" + indentLines(content, "    ") + "\n  </" + tag + ">\n"
}
```

### 10.10 与压缩管道 / PEV 的边界

| 对象 | 进入压缩管道？ | 说明 |
|------|----------------|------|
| `sc.Messages` | ✅ | 七步压缩仅处理对话历史 |
| `sc.SystemPrompt` | ❌ | 由 Assembler 整段替换；压缩 step5 `assembly` 仍将 system 置首 |
| `CompressedView` | system + messages | PEV 使用压缩后 view；system 为 Build 结果 |

**快照持久化：** `ContextSnapshot.systemPrompt` 存 **Assembler 输出**（含 XML）。恢复会话时无需重跑 Bootstrap，但 **新 Process 仍重新 Build** 以刷新 Session Context 日期与 recall。

### 10.11 V4 兼容（harness.enabled=false）

```go
func (a *SystemPromptAssembler) Build(in SystemPromptBuildInput) (string, SystemPromptBuildReport) {
    if !in.HarnessEnabled {
        appendix := memory.FormatLongTermAppendix(in.MemoryEntries, defaultMax)
        return a.BuildLegacy(in.AgentsRaw, appendix), SystemPromptBuildReport{}
    }
    // ... full 4-layer build
}
```

`BuildLegacy` 等价于 V4：`AgentsRaw + appendix`，无 XML、无 Session Context。

### 10.12 测试锚点（L5）

| L5 ID | 断言 |
|-------|------|
| L5-2-9-10 | Build 输出含 `<agents_context>` 且 AGENTS 正文在内 |
| L5-2-9-10 | Session Context 含 SessionID 与 WorkDir |
| L5-2-9-10 | memory 超 budget 时出现截断 notice |
| L5-2-9-10 | `harness.enabled=false` 与 BuildLegacy 字节级一致 |
| L5-2-9-01 | harness_init 含 VisibleTools 与 Trusted |

---

## 四、配置契约

```yaml
context_engine:
  harness:
    enabled: false          # 灰度开关
    trusted: true           # CLI 默认 trusted；IM 可 false
    prefetch:
      enabled: true
      max_walk_depth: 4
    tool_pool:
      simple_mode: false
      include_mcp: true
      deny_names: []
      deny_prefixes: []
    routing:
      enabled: false
      max_matches: 5
    deferred_init:
      enabled: true         # false 时跳过 stage 4
    transcript:
      enabled: true
      compact_after_turns: 20
      session_log_enabled: true   # append-only .log.jsonl
  preflight:
    enabled: false
    mode: warn-only               # warn-only | block (V5b)
    token_budget: 8000
    warn_ratio: 0.85
    tool_filter:
      enabled: true
      mode: auto-repair           # none | auto-repair
  workspace:
    max_context_tokens: 8000      # Layer 3 XML 内可变块 budget
    agent_name: Devrix
    additional_context_files: []    # V5b：相对 WorkDir，如 .devrix/SOUL.md
    embed_core_template: true       # false 时 Layer 0 仅一行 fallback
```

---

## 五、可观测性（摘要）

> **完整 Jaeger 规范见 §十一；测试见 §十二。**  
> Delta：`specs/observability_delta.md`

| Stage | Jaeger Operation | info 事件 metadata |
|-------|------------------|-------------------|
| bootstrap | `context.harness.bootstrap.run` | `harness.trusted`, `harness.stages` |
| stage | `context.harness.bootstrap.stage` | `harness.stage` |
| tool_pool | `context.harness.tool_pool` | `tools.before`, `tools.after` |
| preflight | `context.harness.preflight` | `preflight.warning_count` |
| routing | `context.harness.route` | `matched_tools`, `matched_commands` |
| system prompt | `context.system_prompt.build` | `system_prompt.blocks`, `memory_truncated` |

**V5a Must：** 6 个 Operation 常量 + registry 登记 + L5-2-9-11 集成测试。

---

## 六、包结构与文件清单（S4 指引）

```
internal/layers/contextengine/harness/
├── bootstrap.go
├── tracing.go                   # startHarnessSpan 辅助
├── system_prompt_assembler.go
...
internal/layers/observability/telemetry/
└── names.go                     # +6 harness operations
internal/layers/observability/coverage/
└── registry.go                  # +6 OperationMeta
tests/integration/
└── context_harness_obs_test.go  # L5-2-9-11
```

---

## 七、决策记录

| # | 问题 | 决议 |
|---|------|------|
| 1 | 独立 Layer vs 子包 | D2-S9 子包 `harness/` |
| 2 | 默认 enabled | false（灰度） |
| 3 | Routing 是否强制 | 否，仅 advisory hint |
| 4 | Transcript 持久化 | V5a 内存；V5b 可选文件 |
| 5 | simple_mode 工具集 | bash, read, edit（Devrix 命名） |
| 6 | DeferredInit V5a | NoOp 标志位，无 IO |
| 7 | AgentScope 不替换 PEV | Bootstrap/Preflight 正交注入 |
| 8 | Preflight 默认 | warn-only |
| 9 | V6 路线图 | flush-before-compact, ToolResultEviction, forceCompactAndRetry |
| 10 | System Prompt SoT | §十 Assembly Spec；Loader 只读 AGENTS |
| 11 | Layer 0/2 语言 | V5a 中文框架模板 |
| 12 | Preflight scores | 不进 system，仅 warnings tag |
| 13 | **V5a 交付分期** | **PR1（M1+M2+M3）→ PR2（M4+M5+M6+M7），共享 Change ID；PR1 合入后不归档，PR2 合入后一次归档** |
| 14 | **L3 资产 ID** | Harness 使用 **L3-BE-CTX-04**（非 CTX-03） |
| 15 | **压缩/Build 时序** | messages-only 压缩 → Build → CompressedView assembly |
| 16 | **OpenSpec delta 路径** | `specs/{capability}/spec.md`，非 `*_delta.md` 扁平文件 |

---

## 八、与 claw-code 对照

| claw-code 模块 | Devrix V5a 映射 |
|----------------|-----------------|
| `bootstrap_graph.py` | `BootstrapStage` 常量 + Report |
| `context.py` / `build_port_context` | `WorkspaceContext` → `<workspace_snapshot>` |
| `system_init.py` | `<harness_init>` |
| `setup.py` / `run_setup` | `HarnessBootstrap.Run` stage 1-3 |
| `deferred_init.py` | `IDeferredInit` |
| `tool_pool.py` | `ToolPoolFilter` |
| `runtime.route_prompt` | `<routing_hints advisory="true">` |
| `transcript.py` | `TranscriptStore` |
| `query_engine.compact_messages` | transcript.compact + 现有 compression 管道 |

---

## 九、AgentScope 对照与分期

| AgentScope 能力 | 文件/文档 | Devrix 分期 |
|-----------------|-----------|-------------|
| Hook priority 编排 | `architecture.md` §4 | **V5a** stage priority |
| WorkspaceContextHook | `WorkspaceContextHook.java` | **V5a** §十 SystemPromptAssembler |
| RuntimeContext | `RuntimeContext` | **V5a** ProcessRuntimeContext → Layer 1 |
| `<loaded_context>` XML | `WorkspaceContextHook.java` | **V5a** Layer 3 |
| Token budget / truncate | `WorkspaceContextHook.java` | **V5a** §10.8 |
| Dual session files | `workspace.md` | **V5a** Transcript + SessionLog |
| Context Preflight | `ContextPreflightHook.java` | **V5a** `<preflight_warnings>` only |
| CompactionHook flush+offload | `CompactionHook.java` | **V6** |
| ToolResultEviction | `memory.md` | **V6** |
| forceCompactAndRetry | `architecture.md` §5 | **V6** |
| MemoryConsolidator + FTS | `memory.md` | 已有 LongTerm V3，V5b 增强 |
| AbstractFilesystem 双层读写 | `workspace.md` | **V5b** 多租户 |

---

## 十一、可观测性（Jaeger / OTLP）

> **SoT**：`specs/observability_delta.md`  
> **约束**：`openspec/specs/observability/spec.md` V1.2+ `{layer}.{module}.{action}`

### 11.1 缺口与 V5a 动作

| 现状 | 缺口 | V5a 动作 |
|------|------|----------|
| 原 §五 用非 canonical 名 | Jaeger 无法按 Operation 过滤 | 6 个 `context.harness.*` + `context.system_prompt.build` |
| L5-2-9-08 仅 info | 无 span 树断言 | 新增 **L5-2-9-11** |
| registry 无 harness | Coverage zero_hit 误报 | 登记 `SinceVersion: 2.1.0` |
| 仅 `system_prompt.load` | 与 Assembler 混淆 | load=读文件，build=四层组装 |

### 11.2 Span 树（`context.process` 子 span）

```
context.process
├── context.harness.bootstrap.run        [首次 Process]
│   ├── context.harness.bootstrap.stage    [harness.stage=...]
│   └── context.harness.tool_pool
├── context.harness.preflight            [optional]
├── context.harness.route                [optional]
├── context.system_prompt.load           [读 AGENTS.md]
├── context.system_prompt.build          [Assembler]
└── context.pev.run
```

（`snapshot.load` / `longterm.recall` / `compression.run` 等与现网一致，略）

### 11.3 Operation 常量

```go
OpContextHarnessBootstrapRun   = "context.harness.bootstrap.run"
OpContextHarnessBootstrapStage = "context.harness.bootstrap.stage"
OpContextHarnessToolPool       = "context.harness.tool_pool"
OpContextHarnessPreflight      = "context.harness.preflight"
OpContextHarnessRoute          = "context.harness.route"
OpContextSystemPromptBuild     = "context.system_prompt.build"
```

`LayerAndComponent`：`context.harness.*` 与 `context.system_prompt.build` → `(context, context_engine)`。

### 11.4 关键 Span 属性

| Operation | 必填属性 |
|-----------|----------|
| `bootstrap.run` | `harness.trusted`, `harness.stages_count`, `harness.duration_ms` |
| `bootstrap.stage` | `harness.stage`, `harness.duration_ms` |
| `tool_pool` | `harness.tools.before`, `harness.tools.after` |
| `preflight` | `preflight.mode`, `preflight.warning_count`（**禁止**用户原文） |
| `route` | `harness.route.tool_count`, `harness.route.command_count` |
| `system_prompt.build` | `system_prompt.total_tokens`, `layer0_tokens`…`layer3_tokens`, `memory_truncated`, `blocks` |

### 11.5 实现与双写

- `harness/tracing.go`：`startHarnessSpan(parentCtx, op, attrs...)`
- info 事件保留（Adapter 四流）；Span 供 Jaeger — **同一语义双写**

---

## 十二、测试规格

### 12.1 测试矩阵

| 文件 | Tag | L5 |
|------|-----|-----|
| `harness/*_test.go` | unit | L5-2-9-01~10 |
| `tests/integration/context_harness_bootstrap_test.go` | `integration && d2` | L5-2-9-08 |
| `tests/integration/context_harness_obs_test.go` | `integration && d2` | **L5-2-9-11** |
| `tests/acceptance/p0/ctx_harness_bootstrap_test.go` | `acceptance && p0` | P0 子集 |
| `telemetry/names_test.go` + `registry_test.go` | unit | **L5-5-5-02** |

### 12.2 L5-2-9-11（P0）— Jaeger span 树

- **enabled**：Process 后 span 名含 `context.harness.bootstrap.run`、`context.system_prompt.build`、`context.pev.run`；stage span 含 `harness.stage`
- **disabled**：无 `context.harness.*` / `context.system_prompt.build`

### 12.3 L5-5-5-02（P1）— Registry 对账

- `names.go` 6 个 harness 常量 ∈ `coverage.AllOperations()`
- `LayerAndComponent(OpContextHarnessToolPool)` → `context`, `context_engine`

### 12.4 S5 验收命令

```bash
./scripts/test-domain.sh d2
go test -tags='integration && d2' ./tests/integration/ -run Harness -count=1
```
