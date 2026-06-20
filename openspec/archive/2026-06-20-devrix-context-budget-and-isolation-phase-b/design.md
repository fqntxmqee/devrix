# Design: Context Budget & Isolation — Phase B

**Change ID:** 2026-06-20-devrix-context-budget-and-isolation-phase-b
**Demand ID:** DM-20260620-001-B
**Status:** S3_Design
**Parent:** DM-20260620-001 Phase A (S7_Archived 2026-06-20)
**Base:** `openspec/changes/.../proposal.md` v1.0
**DSAFT Scenarios:** D2-S15, D4-S4, D7-S2

---

## 1. 现有架构基线 (Phase A 落地后)

### 1.1 SubTurnRunner 现状

**D7 边界 (D2→D7):**

```go
// internal/shared/contracts/subturn.go:20-33
type SubTurnRequest struct {
    SessionID      string
    AgentID        string
    AgentName      string
    SystemPrompt   string
    Messages       []types.Message  // 子 agent 完整 history (由 D2 拼装)
    Tools          []ToolSchema
    MaxTurns       int
    Scope          SubTurnScope
    ChildContext   *types.SessionContext
    FlowParams     SubQueryFlowParams
    FlowReporter   SubQueryFlowReporter
    Emit           EngineEmitFunc
}
```

**D7 实现 (turn 包):**

```go
// internal/layers/orchestration/turn/subturn.go:48-58
ch, err := r.Orch.RunTurn(runCtx, TurnRequest{
    SessionID:         req.SessionID,
    UserMessage:       lastUserMessage(req.Messages),
    SystemPrompt:      req.SystemPrompt,
    MaxTurns:          req.MaxTurns,
    Scope:             scope,
    PreloadedMessages: messagesWithoutLastUser(req.Messages),  // ← 总全量继承
    OverrideTools:     mapToolSchemas(req.Tools),
    SkipPersist:       true,
    Model:             modelFromChild(req.ChildContext),
})
```

### 1.2 D2 subquery 已有 fork 基建

**`internal/layers/contextengine/enforce/subquery.go:25-27`**:

```go
ForkMessages   []types.Message
ForkEnabled    bool
ForkDirective  string
```

**`subquery.go:168-174`** (D2 内 fork 决策):

```go
if params.ForkEnabled && params.ForkDirective != "" && len(params.ForkMessages) > 0 {
    forked := conversation.BuildForkedMessages(params.ForkDirective, params.ForkMessages)
    out := make([]types.Message, 0, len(forked)+len(params.PromptMessages))
    out = append(out, forked...)
    out = append(out, params.PromptMessages...)
    return out
}
```

**`internal/layers/contextengine/prepare/conversation/fork.go:11-29`**:

```go
const ForkPlaceholderResult = "Fork started — processing in background"  // 字节级固定

func BuildForkedMessages(directive string, parentMessages []types.Message) []types.Message {
    assistant, refs, ok := findLastAssistantWithToolCalls(parentMessages)
    if !ok || len(refs) == 0 {
        return []types.Message{buildForkDirectiveUser(directive, nil)}
    }
    cloned := cloneAssistantMessage(assistant)
    out := []types.Message{cloned, buildForkDirectiveUser(directive, refs)}
    return out
}
```

**D2-S10-A01-T41 (D2 域已 IMPLEMENTED, Phase A 注册)** 验证
`ForkPrefixFingerprint` 字节级稳定。

### 1.3 现状诊断

| 组件 | 现状 | 缺失 |
|------|------|------|
| D2 fork 逻辑 | ✅ 已有 (`subquery.go:168`) | 仅 `ForkEnabled` bool 触发,无 mode 维度 |
| D7 SubTurnRunner | ❌ 总 `PreloadedMessages=messagesWithoutLastUser` | 无 mode 字段,无 depth 检查 |
| contracts.SubTurnRequest | ❌ 无 Mode / Depth | — |
| tool schema (delegate/free_fork) | ❌ 无 mode 字段 | — |
| depth 限制 | ❌ `SessionContext.QueryDepth` 有但 SubTurnRunner 不查 | 无 max_depth 配置 |
| backward compat | ❌ 全量继承是默认 | 无 `legacy_mode=full` 切换 |

## 2. Phase B 设计

### 2.1 contracts 扩展 (B.1)

**`internal/shared/contracts/subturn.go` 新增**:

```go
// SubAgentMode classifies how a sub-agent inherits parent history (Phase B).
type SubAgentMode string

const (
    SubAgentModeBrief SubAgentMode = "brief"  // 子 agent 全新 history, 节省 token
    SubAgentModeFork  SubAgentMode = "fork"   // 父 tool_use 保留, tool_result 占位 (cache-friendly)
    SubAgentModeFull  SubAgentMode = "full"   // 全量继承父 history (旧行为, 向后兼容)
)

type SubTurnRequest struct {
    SessionID      string
    AgentID        string
    AgentName      string
    SystemPrompt   string
    Messages       []types.Message
    Tools          []ToolSchema
    MaxTurns       int
    Scope          SubTurnScope
    ChildContext   *types.SessionContext
    FlowParams     SubQueryFlowParams
    FlowReporter   SubQueryFlowReporter
    Emit           EngineEmitFunc
    // Phase B 新增:
    Mode           SubAgentMode  // 缺省 "brief"; D2 显式设 fork/full
    Depth          int           // root=0, 1=first-level sub-agent, ...
}
```

### 2.2 错误类型 (B.1)

**`internal/shared/errors/subturn.go` (新文件)**:

```go
package errors

import "errors"

// ErrSubagentDepthExceeded is returned when a sub-agent spawn request would
// exceed the configured MaxSubagentDepth (Phase B AC9).
var ErrSubagentDepthExceeded = errors.New("subagent: recursion depth exceeded; use mode=brief to reduce context size")
```

**SentinelError 模式**: 与现有 `internal/shared/errors/` 保持一致
(参见 `project/coding.md` §9)。

### 2.3 SubTurnRunner 重构 (B.1)

**构造函数** (新增 cfg 注入):

```go
// internal/layers/orchestration/turn/subturn.go
type SubTurnConfig struct {
    DefaultMode      contracts.SubAgentMode  // 缺省 brief
    MaxSubagentDepth int                    // 缺省 3
}

type SubTurnRunner struct {
    Orch TurnOrchestrator
    Cfg  SubTurnConfig  // 新增
}

func NewSubTurnRunner(orch TurnOrchestrator, cfg SubTurnConfig) *SubTurnRunner {
    if cfg.DefaultMode == "" {
        cfg.DefaultMode = contracts.SubAgentModeBrief  // 显式默认
    }
    if cfg.MaxSubagentDepth == 0 {
        cfg.MaxSubagentDepth = 3
    }
    return &SubTurnRunner{Orch: orch, Cfg: cfg}
}
```

**RunSubTurn 3-mode dispatch**:

```go
func (r *SubTurnRunner) RunSubTurn(ctx context.Context, req contracts.SubTurnRequest) (*contracts.SubTurnResult, error) {
    // ... (参数校验不变) ...

    // B.1 AC9: depth 检查
    if req.Depth >= r.Cfg.MaxSubagentDepth {
        return nil, fmt.Errorf("subturn: depth %d >= max %d: %w",
            req.Depth, r.Cfg.MaxSubagentDepth, errors.ErrSubagentDepthExceeded)
    }

    // B.1 AC6: mode 决定 PreloadedMessages
    mode := req.Mode
    if mode == "" {
        mode = r.Cfg.DefaultMode
    }
    var preloaded []types.Message
    switch mode {
    case contracts.SubAgentModeBrief:
        preloaded = nil  // 子 agent 全新 history, 不继承父
    case contracts.SubAgentModeFork:
        // D2 已经在 Messages 中构造 fork; SubTurnRunner 仅剥离 last user
        preloaded = messagesWithoutLastUser(req.Messages)
    case contracts.SubAgentModeFull:
        // 旧行为: 全量继承父 history
        preloaded = messagesWithoutLastUser(req.Messages)
    default:
        return nil, fmt.Errorf("subturn: unknown mode %q", mode)
    }

    ch, err := r.Orch.RunTurn(runCtx, TurnRequest{
        SessionID:         req.SessionID,
        UserMessage:       lastUserMessage(req.Messages),
        SystemPrompt:      req.SystemPrompt,
        MaxTurns:          req.MaxTurns,
        Scope:             scope,
        PreloadedMessages: preloaded,  // ← 关键变化
        OverrideTools:     mapToolSchemas(req.Tools),
        SkipPersist:       true,
        Model:             modelFromChild(req.ChildContext),
    })
    if err != nil {
        return nil, err
    }
    return collectSubTurnResult(ch, req.SessionID, emit)
}
```

**关键设计决策**:
- `fork` 模式仍走 `messagesWithoutLastUser` 因为 D2 已经在
  `req.Messages` 中构造了 `BuildForkedMessages` 输出
- `brief` 模式**不剥离 last user** — 子 agent 唯一看到的 history
  是 `UserMessage` (从 `lastUserMessage(req.Messages)` 取)
- `full` 模式保持旧行为, 默认 brief 意味着旧调用方无显式声明
  全部走 brief, 加 `legacy_mode: full` 切回旧行为

### 2.4 配置注入 (B.1)

**`internal/shared/config/orchestration.go` 新增**:

```go
type ContextSubagentConfig struct {
    DefaultMode string `yaml:"default_mode"`  // "brief" (default) | "fork" | "full"
    LegacyMode  string `yaml:"legacy_mode"`   // "full" 切回旧行为; 后续 minor 移除
    MaxDepth    int    `yaml:"max_depth"`     // 缺省 3
}

type ContextConfig struct {
    Budget   ContextBudgetConfig      `yaml:"budget"`
    Subagent ContextSubagentConfig    `yaml:"subagent"`  // 新增
}
```

**`internal/bootstrap/wire_coordinator.go:169` 改造**:

```go
subCfg := turn.SubTurnConfig{
    DefaultMode: contracts.SubAgentMode(cfg.Context.Subagent.DefaultMode),
    MaxSubagentDepth: cfg.Context.Subagent.MaxDepth,
}
subTurn := turn.NewSubTurnRunner(turnOrch, subCfg)
```

**`devrix.yaml` schema**:

```yaml
context:
  budget:
    max_tool_result_chars: 12000      # Phase A 已有
    max_assistant_chars: 24000        # Phase A 已有
    proactive_fold_percent: 0.6       # Phase A 已有
  subagent:
    default_mode: brief               # Phase B 新默认
    legacy_mode: full                 # 旧调用方显式切换
    max_depth: 3                      # 递归深度限制
```

**`legacy_mode=full` 行为**:
- `SubTurnConfig.DefaultMode = full`
- SubTurnRunner 对所有未显式 `req.Mode` 的 caller 用 full
- 旧调用方加 `legacy_mode: full` → 立即切回旧行为
- 后续 minor release 移除 `legacy_mode` 配置项 (发出 deprecation warning)

### 2.5 tool schema 暴露 (B.2)

**`internal/layers/orchestration/delegatetools/freefork.go` 改造**:

现有 schema 缺省形态 (B.2 改造前):

```go
var delegateInputSchema = `{
  "type": "object",
  "properties": {
    "agent": {"type": "string", "description": "..."},
    "task": {"type": "string", "description": "..."}
  },
  "required": ["agent", "task"]
}`
```

B.2 改造后:

```go
var delegateInputSchema = `{
  "type": "object",
  "properties": {
    "agent": {"type": "string", "description": "..."},
    "task": {"type": "string", "description": "..."},
    "mode": {
      "type": "string",
      "enum": ["brief", "fork", "full"],
      "default": "brief",
      "description": "Sub-agent context inheritance mode (Phase B). brief=全新 history (default); fork=父 tool_use 保留 (cache-friendly); full=全量继承 (旧行为)"
    }
  },
  "required": ["agent", "task"]
}`
```

`free_fork` 同理。

**集成点**:
- D4 multi-agent surface 执行 delegate/free_fork 时, 解析 `mode` 字段
- 显式 `mode` 时透传到 `SubQueryParams.Mode` → `SubTurnRequest.Mode`
- 缺省时由 `subquery.Run()` 决定 (默认 brief, 除非 SubQueryParams 显式)

### 2.6 D2 subquery 透传 (B.1)

**`internal/layers/contextengine/enforce/subquery.go:97-109` 改造**:

```go
// B.1: 透传 Mode + Depth 到 SubTurnRequest
res, err := deps.SubTurn.RunSubTurn(ctx, contracts.SubTurnRequest{
    SessionID:    child.SessionID,
    AgentID:      params.AgentID,
    AgentName:    params.AgentName,
    SystemPrompt: params.SystemPrompt,
    Messages:     initial,
    Tools:        tools,
    MaxTurns:     params.MaxTurns,
    Scope:        contracts.SubTurnScopeSubQuery,
    ChildContext: child,
    FlowParams:   flowParams,
    FlowReporter: reporter,
    Mode:         params.Mode,                  // 新增
    Depth:        child.QueryDepth,            // 新增 (来自 SessionContext)
})
```

**`SubQueryParams` 新增** (B.1):

```go
type SubQueryParams struct {
    // ... 现有字段 ...
    Mode contracts.SubAgentMode  // 新增; 缺省 brief
}
```

`child.QueryDepth` 来自 `SessionContext.QueryDepth + 1`
(已在 `subquery.go:163` 计算), SubTurnRunner 只需检查 `req.Depth >= MaxSubagentDepth`。

### 2.7 fork 模式 + AC11a (B.3)

**核心**: D2 subquery 已有 fork 基建 (B.1 引入 Mode 后):
- D4 surface 设 `params.Mode = "fork"` + `params.ForkEnabled = true`
- D2 subquery 走 `BuildForkedMessages` 路径
- D7 SubTurnRunner `mode=fork` → `PreloadedMessages = messagesWithoutLastUser(forkedMessages)`
- 子 agent 看到: parent tool_use + placeholder tool_result + directive + child user message

**Prefix 稳定性** (B.3 验证):
- `ForkPrefixFingerprint` (D2 已有) 字节级稳定
- B.3 新增 D7 集成测试:
  ```go
  func TestSubTurnRunner_ForkMode_PrefixStability(t *testing.T) {
      parent := buildParentWithToolCalls(...)
      a := buildForkedMessages(parent, "scope auth")
      b := buildForkedMessages(parent, "scope billing")
      aPre := messagesWithoutLastUser(a)
      bPre := messagesWithoutLastUser(b)
      // 子 agent 看到的 prefix 字节级一致
      if !bytes.Equal([]byte(fingerprint(aPre)), []byte(fingerprint(bPre))) {
          t.Fatal("fork sibling prefix should match for cache sharing")
      }
  }
  ```

### 2.8 整体调用链 (B.1 + B.2 + B.3 综合)

```
LLM tool call (delegate / free_fork)
  ↓ D4 surface 解析 input.mode
  ↓ SubQueryParams.Mode = "fork" | "brief" | "full"
  ↓
D2 subquery.Run(SubQueryParams{Mode, ParentSC})
  ↓ child.QueryDepth = parent.QueryDepth + 1
  ↓
  if Mode == "fork" && ForkEnabled:
    forked = conversation.BuildForkedMessages(directive, ForkMessages)
    initial = forked + PromptMessages
  else:
    initial = filterIncomplete + PromptMessages  (brief 模式: PromptMessages only)
  ↓
contracts.SubTurnRequest{Mode, Depth=child.QueryDepth, Messages=initial}
  ↓
D7 SubTurnRunner.RunSubTurn(req)
  ↓ depth check: req.Depth < MaxSubagentDepth
  ↓ mode dispatch:
    - brief: PreloadedMessages=nil
    - fork:  PreloadedMessages=messagesWithoutLastUser(req.Messages)  // D2 已构造 fork
    - full:  PreloadedMessages=messagesWithoutLastUser(req.Messages)  // 旧行为
  ↓
D7 Orchestrator.RunTurn(TurnRequest{PreloadedMessages, ...})
  ↓
LLM streaming
  ↓
collectSubTurnResult → SubTurnResult
```

### 2.9 文件布局 (B.1-B.4 综合)

| 路径 | 变化 | 阶段 |
|------|------|------|
| `internal/shared/contracts/subturn.go` | `SubAgentMode` + `Mode`/`Depth` 字段 | B.1 |
| `internal/shared/errors/subturn.go` | `ErrSubagentDepthExceeded` sentinel (新) | B.1 |
| `internal/shared/config/orchestration.go` | `ContextSubagentConfig` | B.1 |
| `internal/layers/orchestration/turn/subturn.go` | `SubTurnConfig` + 3-mode dispatch + depth check | B.1 |
| `internal/layers/orchestration/turn/subturn_test.go` | 3-mode × depth 边界测试 | B.1 |
| `internal/layers/orchestration/turn/subturn_fork_test.go` | fork prefix 稳定 + sibling 测试 (新) | B.3 |
| `internal/layers/orchestration/delegatetools/freefork.go` | tool schema mode 字段 | B.2 |
| `internal/layers/orchestration/delegatetools/delegate_tools.go` | tool schema mode 字段 | B.2 |
| `internal/layers/contextengine/enforce/subquery.go` | `SubQueryParams.Mode` + 透传 | B.1 |
| `internal/bootstrap/wire_coordinator.go` | `NewSubTurnRunner(orch, cfg)` | B.1 |
| `devrix.yaml` | `context.subagent.*` schema | B.1 |
| `openspec/specs/d7-orchestration/t-registry.md` | +6 P0 T 点 (B.1-B.3) | B.4 |
| `openspec/specs/d4-multi-agent/t-registry.md` | +2 P0 T 点 (B.2 schema) | B.4 |
| `openspec/specs/d2-context-engine/t-registry.md` | +3 P0 T 点 (B.3 fork prefix) | B.4 |
| `openspec/specs/d4-multi-agent/spec.md` | mode 字段 + Gherkin scenarios | B.4 |
| `openspec/specs/d7-orchestration/spec.md` | 3-mode + depth Gherkin scenarios | B.4 |
| `docs/...` (or README/context-budget.md) | mode 选型指南 + legacy_mode 切换说明 | B.4 |

## 3. T 点规划 (B.1-B.5)

### 3.1 B.1 — D7-S2-A06 (T14-T17)

| T ID | 描述 | 测试位置 |
|------|------|---------|
| **D7-S2-A06-T14** | **AC6 brief mode: `req.Mode=brief` → `PreloadedMessages=nil`, 子 agent 全新 history** | `internal/layers/orchestration/turn/subturn_test.go::TestSubTurnRunner_BriefMode_PreloadedMessagesNil` |
| **D7-S2-A06-T15** | **AC6 full mode: `req.Mode=full` → `PreloadedMessages=messagesWithoutLastUser(req.Messages)` (旧行为)** | `subturn_test.go::TestSubTurnRunner_FullMode_BackwardCompat` |
| **D7-S2-A06-T16** | **AC9 depth limit: `req.Depth >= MaxSubagentDepth` → 返回 `ErrSubagentDepthExceeded`, error message 引导改 brief** | `subturn_test.go::TestSubTurnRunner_DepthLimit_{Equals,Exceeds,BoundaryAtMaxMinus1}` |
| **D7-S2-A06-T17** | **AC6 default mode from config: `req.Mode==""` 时走 `SubTurnConfig.DefaultMode`** | `subturn_test.go::TestSubTurnRunner_DefaultModeFromConfig` |

### 3.2 B.2 — D4-S4-A07 (T01-T02)

| T ID | 描述 | 测试位置 |
|------|------|---------|
| **D4-S4-A07-T01** | **AC10 delegate schema: `delegate` tool input schema 增加 `mode?: "brief"\|"fork"\|"full"`, 缺省 brief** | `internal/layers/orchestration/delegatetools/delegate_schema_test.go::TestDelegateInputSchema_HasModeEnum` |
| **D4-S4-A07-T02** | **AC10 free_fork schema: `free_fork` tool input schema 同上** | `delegatetools/freefork_schema_test.go::TestFreeForkInputSchema_HasModeEnum` |

### 3.3 B.3 — D2-S15 (T01-T03)

| T ID | 描述 | 测试位置 |
|------|------|---------|
| **D2-S15-A08-T06** | **AC11a fork sibling prefix 稳定: `buildForkedMessages(parent, directiveA)` 与 `(parent, directiveB)` 的 PreloadedMessages 字节级一致** | `internal/layers/orchestration/turn/subturn_fork_test.go::TestSubTurnRunner_ForkSiblingPrefixStable` |
| **D2-S15-A08-T07** | **AC8 full mode backward compat: `mode=full` 调用 SubTurnRunner 行为与 Phase A 等价** | `subturn_test.go::TestSubTurnRunner_FullMode_BehaviorEquivPhaseA` |
| **D2-S15-A08-T08** | **AC11a fork prefix fingerprint 包含 `ForkPlaceholderResult` 字面量** | `subturn_fork_test.go::TestSubTurnRunner_ForkPrefix_ContainsPlaceholder` |

### 3.4 B.5 — AC12 (T01, regression)

| T ID | 描述 | 测试位置 |
|------|------|---------|
| **(D5-DIAG-T06)** | **AC12 D5 spans 22 步复跑: `prompt_tokens` P95 ≤ 40K** | `tests/acceptance/p0/d5_spans_replay_test.go::TestD5SpansReplay_22StepsPromptTokensP95Leq40K` |

## 4. 配置兼容性矩阵

| devrix.yaml | req.Mode | SubTurnRunner 行为 | 旧调用方影响 |
|------------|----------|-------------------|-------------|
| (无配置) | "" | brief (default brief) | **新行为 (节省 token)** |
| `default_mode: full` | "" | full | **旧行为 (兼容)** |
| `legacy_mode: full` | "" | full | **旧行为 (兼容, deprecation warning)** |
| (任意) | "brief" | brief | 显式 brief |
| (任意) | "fork" | fork | 显式 fork |
| (任意) | "full" | full | 显式 full |

**默认 brief 破坏性变更**: Phase B.1 合入后, 旧调用方**无感知**
(仍然能跑), 但**消息体变小** (D5 spans 51K → 目标 40K)。
如有真实现网 caller 强依赖全量 history, 加 `legacy_mode: full` 切回。

## 5. 失败模式

### 5.1 brief mode 丢失父上下文

**症状**: 子 agent 不知道用户上次问了什么, 给错答案。
**检测**: 现有 `subturn_test.go::TestSubTurnRunner_RunSubTurn` 需扩
mode=brief 测试。
**恢复**: error message 明确引导调用方改 `mode=fork` 或 `mode=full`。
**度量**: integration test 覆盖 fork/full/brief 三路径。

### 5.2 depth 误拒合法场景

**症状**: 4 层递归被拒, 任务失败。
**检测**: 现有调用链 D5 spans 实测 ≤ 2 层, 4 层 (≥ 3 depth) 极少。
**恢复**: error message 引导改 `mode=brief` (节省 token 后可能允许更深)。
**度量**: `TestSubTurnRunner_DepthLimit_Exceeds` 验证 error message 格式。

### 5.3 fork prefix 字节级不稳定

**症状**: sibling sub-agent 看到不同 prefix, 后续切 Anthropic 时 cache miss。
**检测**: `TestSubTurnRunner_ForkSiblingPrefixStable` 字节级断言。
**恢复**: 严格覆盖 `ForkPlaceholderResult` 字面量; `BuildForkedMessages`
已有 prefix 稳定性测试 (`D2-S10-A01-T41`).
**度量**: integration test 跑 10 个 sibling fork, prefix fingerprint 100% 一致。

### 5.4 schema 不匹配

**症状**: LLM 调用 delegate 工具时 schema 校验失败。
**检测**: tool schema json dump 验证。
**恢复**: 缺省 brief, 向后兼容; integration test 覆盖 default path。
**度量**: `TestDelegateInputSchema_HasModeEnum` json schema 解析。

## 6. AC12 回归设计 (B.5)

### 6.1 D5 spans 任务复跑脚本

**`tests/fixtures/d5-spans-replay.jsonl`**: 保存 D5 spans 设计任务的
原 prompt + 22 步用户输入序列。

**`tests/acceptance/p0/d5_spans_replay_test.go`** (新):

```go
//go:build acceptance

func TestD5SpansReplay_22StepsPromptTokensP95Leq40K(t *testing.T) {
    // 加载 fixture
    fixture := loadD5SpansFixture(t)
    // 模拟 22 步 turn (用真实 LLM gateway 或 mock)
    var promptTokens []int
    for i, step := range fixture.Steps {
        // ... 模拟 LLM call ...
        promptTokens = append(promptTokens, llmReq.PromptTokens)
    }
    // 断言 P95 ≤ 40K
    p95 := percentile(promptTokens, 95)
    if p95 > 40000 {
        t.Fatalf("D5 spans P95 prompt_tokens = %d > 40K", p95)
    }
}
```

### 6.2 度量

- **P95 prompt_tokens ≤ 40K** (vs Phase A 51K)
- **feishu 0 ERROR** (沿用 Phase A 验证)
- **22 步 token 增长曲线** 作为 benchmark artifact (保存到
  `tests/fixtures/d5-spans-replay-bench.json`)

### 6.3 失败处理

若 B.5 不达 40K:
1. 检查 fork mode 是否被触发 (D5 spans 是 sibling 调研场景)
2. 检查 brief mode 是否所有 sub-agent 都用
3. 考虑将 `default_mode: fork` 调整为 D5 spans 场景

## 7. Reference

- 现有 fork 基建: `internal/layers/contextengine/prepare/conversation/fork.go`
- Phase A 归档: `openspec/archive/2026-06-20-devrix-context-budget-and-isolation/`
- D5 spans 数据: `/Users/fukai/.devrix/logs/llm/unknown.jsonl` 行 3336-3338
- clawcode 3 mode: `src/tools/AgentTool/AgentTool.tsx:495-602`
- SentinelError 模式: `internal/shared/errors/`, `project/coding.md` §9
