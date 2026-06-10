# Context Engine Specification (Delta)

**Capability:** context-engine
**Change ID:** devrix-harness-bootstrap
**Demand ID:** DM-20260609-004
**Parent Spec:** `openspec/specs/context-engine/spec.md` v4.0.0
**Merge target version:** 5.0.0

---

## ADDED Requirements

### Requirement: Harness Bootstrap Orchestration

系统 MUST 在 Context Engine 域提供分阶段 Harness Bootstrap 编排，对标 Claude Code Harness 启动模式。

**Priority**: P1
**Rationale**: Devrix V4 缺少 trust-gated 启动与工具面预装配，扩展 plugin/MCP 时无明确接入点
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-HARNESS
**L5 映射**: L5-2-9-01, L5-2-9-03, L5-2-9-08

#### Scenario: Bootstrap runs on first Process when enabled

- GIVEN `context_engine.harness.enabled=true`
- AND session has not completed harness initialization
- WHEN `ContextEngine.Process` is called
- THEN HarnessBootstrap runs stages prefetch → guards → setup → deferred_init → tool_pool
- AND bootstrap `info` events are emitted with stage metadata
- AND session is marked harness-initialized

#### Scenario: Bootstrap skipped when disabled

- GIVEN `context_engine.harness.enabled=false`
- WHEN `ContextEngine.Process` is called
- THEN no harness stages run
- AND behavior matches Context Engine V4

#### Scenario: Bootstrap idempotent per session

- GIVEN harness already initialized for session
- WHEN subsequent `Process` calls occur
- THEN full bootstrap is not re-run
- AND ToolPool visible tool set is reused from `HarnessSessionState`

---

### Requirement: Workspace Context Scan

系统 MUST 在 bootstrap prefetch 阶段扫描 session WorkDir 并构建 WorkspaceContext。

**Priority**: P1
**Rationale**: PEV 与 Assembler 需要工作区元信息用于 system 注入与调试
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-HARNESS
**L5 映射**: L5-2-9-02

#### Scenario: Scan work directory metadata

- GIVEN a valid session WorkDir
- WHEN prefetch stage runs
- THEN WorkspaceContext includes work dir path
- AND source/test file counts are populated
- AND AGENTS.md presence is detected

#### Scenario: Invalid work directory

- GIVEN WorkDir is missing or not readable
- WHEN prefetch stage runs
- THEN bootstrap continues with degraded WorkspaceContext
- AND a recoverable `info` event describes the degradation

---

### Requirement: Trust-Gated Deferred Initialization

系统 MUST 支持 trust-gated deferred init；非 trusted 模式下 MUST NOT 标记 plugin/skill/MCP 为已初始化。

**Priority**: P1
**Rationale**: IM/非受信入口不应预加载扩展面
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-HARNESS
**L5 映射**: L5-2-9-04

#### Scenario: Trusted deferred init

- GIVEN `context_engine.harness.trusted=true` and `harness.deferred_init.enabled=true`
- WHEN deferred_init stage runs
- THEN DeferredInitResult marks plugin_init, skill_init, mcp_prefetch, session_hooks as enabled

#### Scenario: Untrusted deferred init skipped

- GIVEN `context_engine.harness.trusted=false`
- WHEN deferred_init stage runs
- THEN all DeferredInitResult flags are false
- AND no extension surfaces are loaded (V5a: stub only)

---

### Requirement: Tool Pool Filtering

系统 MUST 在 bootstrap 阶段按配置裁剪可见工具集，先于 PEV 工具 schema 暴露。

**Priority**: P0
**Rationale**: 启动期缩小工具面，减少误调用与 schema token 开销
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-TOOLPOOL
**L5 映射**: L5-2-9-05, L5-2-9-14

#### Scenario: Simple mode restricts tool surface

- GIVEN `context_engine.harness.tool_pool.simple_mode=true`
- WHEN tool_pool stage runs
- THEN only Devrix builtin tools `bash`, `read_file`, `write_file` remain visible
- AND PEV receives filtered ToolSchema list (not full registry ListTools)

#### Scenario: MCP tools excluded

- GIVEN `context_engine.harness.tool_pool.include_mcp=false`
- WHEN tool_pool stage runs
- THEN tools whose name or description contains "mcp" are excluded

#### Scenario: Deny list filters tools

- GIVEN `deny_names` contains "bash"
- WHEN tool_pool stage runs
- THEN bash tool is not in visible tool set
- AND IPermissionGate is never invoked for denied tools

---

### Requirement: Advisory Prompt Routing

The system MUST support optional pre-LLM command/tool matching when routing is enabled, and MUST keep V4 behavior when routing is disabled. Routing hints MUST be advisory only and MUST NOT force PEV to invoke specific tools.

系统 MUST 在 routing 启用时支持 pre-LLM 匹配；禁用时 MUST 保持 V4 行为。

**Priority**: P2
**Rationale**: 高频命令可选缩短 LLM 选工具成本，不替代 tool calling
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-ROUTER
**L5 映射**: L5-2-9-06

#### Scenario: Routing enabled produces hints

- GIVEN `context_engine.harness.routing.enabled=true`
- AND user message contains tool-related keywords
- WHEN Process prepares LLM context
- THEN RoutingHint lists matched tools with scores
- AND system prompt includes advisory routing block

#### Scenario: Routing disabled

- GIVEN `context_engine.harness.routing.enabled=false`
- WHEN Process runs
- THEN no routing hint is appended
- AND LLM tool selection is unchanged from V4

---

### Requirement: Transcript Lifecycle Separation

系统 MUST 维护独立于 CompressedView 的 TranscriptStore，用于 turn 级审计与 replay。

**Priority**: P1
**Rationale**: 压缩视图丢失原始 turn 细节，审计需要独立序列
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-TRANSCRIPT
**L5 映射**: L5-2-9-07

#### Scenario: Append transcript on Process completion

- GIVEN `context_engine.harness.transcript.enabled=true`
- WHEN Process completes successfully
- THEN user message and assistant summary are appended to TranscriptStore
- AND Messages history remains compression pipeline input

#### Scenario: Transcript compact after threshold

- GIVEN transcript entries exceed `compact_after_turns`
- WHEN compact is triggered
- THEN only the most recent N entries are retained
- AND replay returns compacted sequence

#### Scenario: Messages and transcript on error path

- GIVEN Process fails before PEV completes
- WHEN error event is emitted
- THEN TranscriptStore MUST NOT append partial assistant turn
- AND Messages state matches V4 error semantics

---

### Requirement: Context Preflight Scoring

系统 MUST 在 PEV 前对上下文做规则 Preflight 评分（对齐 AgentScope agentscope-agent），默认 warn-only。

**Priority**: P1
**Rationale**: 在 reasoning 前发现 token/安全/工具面问题
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-PREFLIGHT
**L5 映射**: L5-2-9-09

#### Scenario: Preflight warn-only emits warnings

- GIVEN `context_engine.preflight.enabled=true` and `preflight.mode=warn-only`
- WHEN provisional context (agentsRaw + memory entries) exceeds warn_ratio of token_budget
- THEN PreflightScores include degraded tokenBudget score
- AND warnings are emitted via info event
- AND Process continues to PEV

#### Scenario: Tool filter auto-repair

- GIVEN `preflight.tool_filter.mode=auto-repair`
- AND user message is unrelated to tool "bash"
- WHEN preflight evaluates visible tools
- THEN bash is excluded from PEV ToolSchema list
- AND info event records excluded tool names
- AND visible tool count MUST NOT drop below configured `preflight.tool_filter.min_tools` (default 1)

#### Scenario: Preflight block mode rejected in V5a

- GIVEN `context_engine.preflight.mode=block`
- WHEN application config is loaded
- THEN config validation MUST fail with clear error (V5b 再支持 block)

---

### Requirement: Workspace Structured Injection

系统 MUST 按 `design.md` §十 System Prompt Assembly Spec 组装最终 system prompt。

**Priority**: P0
**Rationale**: 分层 system prompt 分离人格、会话事实与 loaded context
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-WORKSPACE
**L5 映射**: L5-2-9-10, L5-2-9-12, L5-2-9-13

#### Scenario: Four-layer prompt with loaded_context XML

- GIVEN harness enabled and AGENTS.md present
- WHEN SystemPromptAssembler.Build runs after message compression
- THEN output includes Layer 0 devrix_core, Layer 1 session_context, Layer 2 workspace_guidance
- AND Layer 3 contains `<loaded_context>` with non-empty `<agents_context>`
- AND sc.SystemPrompt is set before PEVEngine.Run

#### Scenario: Session context dynamic fields

- GIVEN a session with WorkDir and SessionID
- WHEN Build runs
- THEN Session Context section includes today's date, WorkDir absolute path, SessionID, and Model
- AND date reflects current Process time (not snapshot stale date)

#### Scenario: Memory token budget truncation

- GIVEN memory_context raw size exceeds remaining Layer 3 budget
- WHEN Build allocates tokens per §10.8 algorithm
- THEN memory_context is truncated with Chinese truncation notice
- AND BuildReport.memory_truncated is true

#### Scenario: V4 legacy build when harness disabled

- GIVEN harness.enabled=false
- WHEN Build runs
- THEN output equals AgentsRaw plus LongTerm appendix format from V4
- AND no XML or Session Context sections appear

#### Scenario: CompressedView system matches Assembler output

- GIVEN harness enabled and compression ran on messages only
- WHEN Build completes and CompressedView is assembled
- THEN CompressedView system message content equals Assembler output
- AND compression token budget excluded final XML system size (messages-only compression)

---

### Requirement: Dual Session Log Files

Transcript 分离 MUST 支持 compact view 与 append-only full log 双轨（对齐 AgentScope sessions 布局）。

**Priority**: P2
**Rationale**: 审计需要 append-only 完整 log，LLM 仅需 compact view
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-TRANSCRIPT
**L5 映射**: L5-2-9-07

#### Scenario: Append-only session log

- GIVEN `context_engine.harness.transcript.session_log_enabled=true`
- WHEN Process completes
- THEN user and assistant turns are appended to session log
- AND log is not replaced by compression pipeline

---

## MODIFIED Requirements

### Requirement: Context Engine Core

The system MUST provide a real context engine implementation. When harness is enabled, Process MUST follow: LoadOrInit → Bootstrap (first Process) → LongTerm recall → compress messages only → SystemPromptAssembler.Build → PEV.Run. When `harness.enabled=false`, behavior MUST remain bit-identical to V4.

系统 MUST 提供可替换 Stub 的真实上下文引擎；harness 启用时 MUST 按 design §1.3 时序执行。

**Priority**: P0
**Rationale**: Harness 正交注入不改变 PEV 推理核，但需明确与压缩/组装的时序
**L3 映射**: L3-BE-CTX-01

#### Scenario: V4 compatibility with harness disabled

- GIVEN `context_engine.harness.enabled=false`
- WHEN any V4 L5 scenario for Process runs
- THEN all V4 expectations still pass

#### Scenario: Process pipeline order when harness enabled

- GIVEN `context_engine.harness.enabled=true`
- WHEN Process runs one turn
- THEN execution order is LoadOrInit → Bootstrap (first time) → LongTerm Recall → append user message → compress messages → SystemPromptAssembler.Build → PEV.Run → transcript append
- AND LongTerm recall MUST NOT mutate sc.SystemPrompt via string append (entries passed to Assembler)
