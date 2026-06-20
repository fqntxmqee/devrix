# Design: Context Budget & Isolation

**Change ID:** `2026-06-20-devrix-context-budget-and-isolation`  
**Demand ID:** DM-20260620-001  
**Status:** S3_Design

> **S3 目标**：把 proposal.md 的 AC 落到具体文件、函数签名、数据结构、错误处理路径。

---

## 0. 文件 / 路径总览

| 新增 / 修改 | 路径 | Phase |
|------------|------|-------|
| 新增 | `internal/layers/contextengine/prepare/persist/tool_result_store.go` | A |
| 新增 | `internal/layers/contextengine/prepare/persist/tool_result_store_test.go` | A |
| 新增 | `internal/layers/contextengine/prepare/audit/token_audit.go` | A |
| 新增 | `internal/layers/contextengine/prepare/audit/token_audit_test.go` | A |
| 修改 | `internal/layers/contextengine/prepare/token/counter.go` | A |
| 修改 | `internal/layers/contextengine/prepare/adapters/session_loader.go` | A |
| 修改 | `internal/layers/orchestration/turn/orchestrator.go` | A |
| 修改 | `internal/layers/orchestration/turn/llm.go` | A |
| 新增 | `internal/layers/communication/sender/card_precheck.go` | A |
| 新增 | `internal/layers/communication/sender/card_precheck_test.go` | A |
| 新增 | `internal/layers/communication/feishu/feishu_table_precheck.go` | A |
| 修改 | `internal/layers/communication/feishu/send.go` | A |
| 修改 | `internal/layers/orchestration/turn/subturn.go` | B |
| 新增 | `internal/layers/orchestration/turn/fork_messages.go` | B |
| 新增 | `internal/layers/orchestration/turn/fork_messages_test.go` | B |
| 修改 | `internal/layers/orchestration/turn/contracts.go`（如有 SubTurnRequest） | B |
| 修改 | `internal/layers/contextengine/prepare/prompt/assembler.go` | B |
| 修改 | `internal/layers/llmgateway/message.go` | B |
| 修改 | `internal/layers/multiagent/delegatetools/*.go`（schema 暴露 mode） | B |
| 修改 | `devrix.yaml` schema + 默认配置 | A+B |
| 修改 | `openspec/specs/d2-context-engine/t-registry.md` | A+B |
| 修改 | `openspec/specs/d7-orchestration/t-registry.md` | A+B |
| 修改 | `openspec/specs/d1-communication/t-registry.md` | A |

---

## 1. Phase A — 详细设计

### 1.1 AC1: 工具结果 size cap + 落盘

#### 数据结构

```go
// internal/layers/contextengine/prepare/persist/tool_result_store.go

package persist

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

// ToolResultRecord persists a single oversized tool result to disk.
type ToolResultRecord struct {
    ID         string    `json:"id"`
    SessionID  string    `json:"session_id"`
    ToolName   string    `json:"tool_name"`
    ToolUseID  string    `json:"tool_use_id"`
    FullPath   string    `json:"full_path"`
    Size       int       `json:"size"`
    PreviewLen int       `json:"preview_len"`
    CreatedAt  time.Time `json:"created_at"`
}

// ToolResultStore writes oversized tool results to disk and returns a preview marker.
type ToolResultStore struct {
    Root string // default: ~/.devrix/tool-results
}

func NewToolResultStore(root string) *ToolResultStore { ... }

// Persist writes content to disk; returns the preview marker that should replace
// the in-band tool result content.
//
// Format:
//   <persisted-output>
//   Output too large ({size:.1f}KB). Full output saved to: {path}
//   
//   Preview (first {previewLen} chars):
//   {preview}
//   ...
//   </persisted-output>
func (s *ToolResultStore) Persist(
    ctx context.Context,
    sessionID, toolName, toolUseID string,
    content string,
    maxChars int, // if content <= maxChars, returns content unchanged
) (string, error) { ... }

// List returns all records for a session (for debugging).
func (s *ToolResultStore) List(sessionID string) ([]ToolResultRecord, error) { ... }

// GC removes records older than retentionDays (default 7).
func (s *ToolResultStore) GC(retentionDays int) (int, error) { ... }
```

#### 调用点

在 `internal/layers/orchestration/turn/llm.go` 的工具结果构造点之后，**tool_result 内容塞进 messages 之前**调用：

```go
// 旧代码 (orchestrator.go:510-525):
content := r.Output
if r.Error != "" {
    content = r.Error
}
messages = append(messages, buildToolResultMsg(req.SessionID, r))

// 新代码:
const maxChars = 12000 // 来自 devrix.yaml context.tool_result.max_chars
const previewChars = 2000
content := r.Output
if r.Error != "" {
    content = r.Error
}
if utf8.RuneCountInString(content) > maxChars {
    previewed, err := store.Persist(ctx, req.SessionID, r.ToolName, r.ToolUseID, content, maxChars)
    if err != nil {
        // fall back to truncation without persist
        content = truncateHead(content, maxChars) + "\n...[truncated, persist failed]"
    } else {
        content = previewed
    }
}
messages = append(messages, buildToolResultMsg(req.SessionID, r, content))
```

#### 工具白名单（仅这几类触发 size cap）

```go
// internal/layers/contextengine/prepare/persist/tool_result_store.go
var sizeCappedTools = map[string]bool{
    "read_file":     true,
    "bash:grep":     true,
    "bash:rg":       true,
    "bash:find":     true,
    "bash:ls":       true,
    "bash:cat":      true,
    "bash:head":     true,
    "bash:tail":     true,
}

func ShouldCap(toolName string) bool {
    return sizeCappedTools[toolName]
}
```

非白名单工具（如 `task_create` JSON 响应）不 cap，避免误伤。

### 1.2 AC2: assistant 输出折叠

#### 数据结构

```go
// internal/layers/contextengine/prepare/persist/turn_output_store.go
// (复用 ToolResultStore 的 root，仅换子目录 turn-outputs)

type TurnOutputRecord struct {
    SessionID string `json:"session_id"`
    TurnNum   int    `json:"turn_num"`
    Role      string `json:"role"`
    FullPath  string `json:"full_path"`
    Size      int    `json:"size"`
    HeadLen   int    `json:"head_len"`
    TailLen   int    `json:"tail_len"`
    CreatedAt time.Time `json:"created_at"`
}

// FoldAssistantOutput: 当 assistant 单条 content > maxChars 时,
//   返回 (foldedContent, error)，格式：
//   <prior-output-summary>
//   ... (前 800 字)
//   ... [middle {N} chars truncated; see {path}]
//   ... (后 200 字)
//   </prior-output-summary>
func FoldAssistantOutput(
    store *ToolResultStore, // 复用同一 store，目录换为 turn-outputs
    sessionID string,
    turnNum int,
    content string,
    maxChars int,
) (string, error) { ... }
```

#### 调用点

turn loop 每个 iteration 结束（在 `messages = append(...)` 之后），**scan 刚 append 的 assistant message 是否超长**：

```go
// orchestrator.go: ~line 562（assistant + tool result 追加之后）
for _, m := range assistantMsgsAdded {
    if m.Role == types.MessageRoleAssistant &&
       utf8.RuneCountInString(m.Content) > maxAssistantChars {
        folded, err := FoldAssistantOutput(store, sessionID, turnNum, m.Content, maxAssistantChars)
        if err == nil {
            m.Content = folded
        }
    }
}
```

### 1.3 AC3: per-iteration Prepare

#### 改动

把 `internal/layers/orchestration/turn/orchestrator.go:245` 的 Prepare 调用从「循环开头一次」移入 `for { ... }` 内：

```go
// 旧：for 外
prepared, err = o.context.Prepare(ctx, PrepareRequest{...})
for { ... }

// 新：for 内
for {
    // D2 Prepare 移到此处
    prepared, err = o.context.Prepare(ctx, PrepareRequest{
        SessionID:    req.SessionID,
        SystemPrompt: systemPrompt,
        Messages:     messages,
        Tools:        tools,
        Budget:       budgetFromConfig(o.budget),
    })
    if err != nil {
        return ...
    }
    // ... 原有 CompressHint 处理 ...
    // ... 原有 LLM invoke ...
}
```

**性能影响**：Prepare 内部是 O(n) snapshot lookup + token estimate，22 步任务实测 P95 增加 < 50ms（proposal 风险已评估）。

### 1.4 AC4: per-iteration token audit + proactive fold

#### 数据结构

```go
// internal/layers/contextengine/prepare/audit/token_audit.go

package audit

type AuditResult struct {
    TotalTokens      int
    SystemTokens     int
    MessagesTokens   int
    OverBudget       bool
    BudgetPercent    float64
    LargestMsgTokens int
    LargestMsgIdx    int
    FoldTriggered    bool
}

func AuditMessages(counter *token.Counter, systemPrompt string, msgs []types.Message, budget int) AuditResult { ... }

// ShouldFoldProactively: 当 audit.TotalTokens > budget * 0.6,
// 主动 fold 最大 assistant message（不等 CompressHint）。
func ShouldFoldProactively(a AuditResult, maxAssistantChars int) bool {
    return a.OverBudget || a.TotalTokens > a.BudgetPercent // 0.6 default
}
```

#### 调用点

```go
// orchestrator.go: turn loop 每个 iteration 开头
for {
    // 1. Prepare (AC3)
    prepared, err = o.context.Prepare(...)
    // 2. Token audit (AC4)
    auditResult := audit.AuditMessages(counter, prepared.SystemPrompt, messages, o.budget.TotalTokens)
    if audit.ShouldFoldProactively(auditResult, o.budget.MaxAssistantChars) {
        foldLargestAssistantMessage(messages, store, sessionID, turnNum, o.budget.MaxAssistantChars)
    }
    // 3. ... 原有 LLM invoke ...
}
```

### 1.5 AC5: feishu card table 数预检

#### 接口

```go
// internal/layers/communication/sender/card_precheck.go

package sender

type CardContentPrecheck interface {
    // Name returns human-readable name for logs.
    Name() string
    // Check inspects the card content and returns an error if it violates limits.
    // Common errors: ErrTooManyTables, ErrTooLong, etc.
    Check(content string) error
}

type CardPrecheckConfig struct {
    MaxTablesPerCard int // default 5
    MaxCharsPerCard  int // default 50000 (feishu hard limit)
}

func DefaultCardPrecheckConfig() CardPrecheckConfig {
    return CardPrecheckConfig{MaxTablesPerCard: 5, MaxCharsPerCard: 50000}
}

// ErrTooManyTables signals the response has too many tables; caller should
// fall back to plain text path.
var ErrTooManyTables = errors.New("card content has too many tables")
```

#### feishu 实现

```go
// internal/layers/communication/feishu/feishu_table_precheck.go

type FeishuTableCountPrecheck struct {
    cfg CardPrecheckConfig
}

func (p *FeishuTableCountPrecheck) Name() string { return "feishu-table-count" }

func (p *FeishuTableCountPrecheck) Check(content string) error {
    tableCount := strings.Count(content, "<table")
    if tableCount > p.cfg.MaxTablesPerCard {
        return fmt.Errorf("%w: count=%d, limit=%d", ErrTooManyTables, tableCount, p.cfg.MaxTablesPerCard)
    }
    return nil
}
```

#### 调用点（feishu send.go）

```go
// feishu/send.go: sendCard() 入口
func (s *Sender) sendCard(ctx context.Context, content string) error {
    if err := s.precheck.Check(content); err != nil {
        if errors.Is(err, sender.ErrTooManyTables) {
            // 降级到纯文本路径
            slog.Warn("feishu: card precheck failed, falling back to plain text",
                "precheck", s.precheck.Name(), "error", err)
            return s.sendPlainText(ctx, content)
        }
        return err
    }
    // 原有发送逻辑
    ...
}
```

### 1.6 配置项

```yaml
# devrix.yaml 新增
context:
  tool_result:
    max_chars: 12000      # AC1 单条上限
    preview_chars: 2000   # 落盘后 preview 长度
    persist_root: ~/.devrix/tool-results
    retention_days: 7
  assistant_output:
    max_chars: 24000      # AC2 折叠阈值
    head_chars: 800       # 折叠后保留头部
    tail_chars: 200       # 折叠后保留尾部
    persist_root: ~/.devrix/turn-outputs
    retention_days: 7
  budget:
    total_tokens: 80000   # AC4 总体预算
    proactive_fold_ratio: 0.6  # 超 60% 触发主动 fold
  subagent:
    default_mode: brief   # Phase B 默认
    legacy_mode: ""       # 旧调用方一次性切换；空字符串 = 跟随 default_mode
    max_depth: 3

# feishu 配置扩展
communication:
  feishu:
    card_precheck:
      enabled: true
      max_tables_per_card: 5
      max_chars_per_card: 50000
```

---

## 2. Phase B — 详细设计

### 2.1 AC6: SubTurnRunner.Mode 字段

#### 改动

`internal/layers/orchestration/turn/contracts.go`：

```go
// 新增 type
type SubTurnMode string

const (
    SubTurnModeBrief SubTurnMode = "brief"
    SubTurnModeFork  SubTurnMode = "fork"
    SubTurnModeFull  SubTurnMode = "full"
)

// SubTurnRequest 新增字段（如已有则扩展）
type SubTurnRequest struct {
    // ... 现有字段 ...
    Mode  SubTurnMode // 新增；缺省 = SubTurnModeBrief
    Depth int         // 新增；缺省 = 0
}
```

`internal/layers/orchestration/turn/subturn.go`：

```go
func (r *SubTurnRunner) RunSubTurn(ctx context.Context, req contracts.SubTurnRequest) (*contracts.SubTurnResult, error) {
    // ... 现有校验 ...

    mode := req.Mode
    if mode == "" {
        // 从 devrix.yaml 读取 default_mode / legacy_mode
        mode = r.resolveDefaultMode()
    }

    // depth 检查
    if req.Depth >= r.maxDepth {
        return nil, fmt.Errorf("%w: depth=%d, max=%d",
            contracts.ErrSubagentDepthExceeded, req.Depth, r.maxDepth)
    }

    var preloaded []types.Message
    var userMsg types.Message

    switch mode {
    case contracts.SubTurnModeBrief:
        // 只传 user brief
        userMsg = lastUserMessage(req.Messages)
        preloaded = nil
    case contracts.SubTurnModeFork:
        // fork 模式：用 buildForkedMessages 构造（见 2.2）
        forked := buildForkedMessages(req.Messages, lastUserMessage(req.Messages))
        userMsg = forked.userMsg
        preloaded = forked.preloaded
    case contracts.SubTurnModeFull:
        // 全量继承（向后兼容）
        userMsg = lastUserMessage(req.Messages)
        preloaded = messagesWithoutLastUser(req.Messages)
    default:
        return nil, fmt.Errorf("subturn: unknown mode %q", mode)
    }

    // ... 现有 RunTurn 调用，PreloadedMessages 用上面计算的结果 ...
}
```

### 2.2 AC7: buildForkedMessages（占位 + cache 稳定）

#### 函数签名

```go
// internal/layers/orchestration/turn/fork_messages.go

package turn

type ForkedMessages struct {
    preloaded []types.Message
    userMsg   types.Message
}

// buildForkedMessages takes parent's full messages and a directive,
// returns the messages the fork sub-agent should see.
//
// Strategy:
//   1. 保留 parent 最后一条 assistant message（含所有 tool_use 块）
//   2. 紧随其后追加一条 user message，内容为：
//      - 所有 tool_result 替换为占位 "Fork started — processing in background"
//      - 末尾追加 directive（子 agent 的任务 brief）
//   3. 这样保证：所有 fork 子 agent 的 messages prefix 字节级一致（除了 directive 本身）
func buildForkedMessages(parentMsgs []types.Message, directive types.Message) ForkedMessages { ... }
```

#### 实现要点

```go
func buildForkedMessages(parentMsgs []types.Message, directive types.Message) ForkedMessages {
    if len(parentMsgs) == 0 {
        return ForkedMessages{
            preloaded: nil,
            userMsg:   directive,
        }
    }

    // 1. 找到最后一条 assistant message
    var lastAssistant *types.Message
    for i := len(parentMsgs) - 1; i >= 0; i-- {
        if parentMsgs[i].Role == types.MessageRoleAssistant {
            lastAssistant = &parentMsgs[i]
            break
        }
    }
    if lastAssistant == nil {
        // 没有 assistant message，fallback 到 brief 模式
        return ForkedMessages{preloaded: nil, userMsg: directive}
    }

    // 2. 构造占位 tool_result
    //    假设 lastAssistant.Content 包含 JSON tool_use blocks；
    //    解析出所有 tool_use_id，构造对应的占位 tool_result。
    placeholderUserMsg := types.Message{
        SessionID: directive.SessionID,
        Role:      types.MessageRoleUser,
        Content: buildPlaceholderToolResults(lastAssistant.Content) + "\n\n" + directive.Content,
    }

    return ForkedMessages{
        preloaded: []types.Message{*lastAssistant},
        userMsg:   placeholderUserMsg,
    }
}
```

**Cache 稳定性保证**：所有 fork 子 agent 拿到的 `preloaded` 是 parent 最后一条 assistant message（**同一个对象**），`userMsg` 中 placeholder 部分字节级一致，**只有 directive 不同**。Anthropic SDK 会把 system prompt + 相同 prefix 视为可缓存段；不同 fork 的 directive 不进入缓存段。

### 2.3 AC9: 递归深度限制

```go
// internal/layers/orchestration/turn/subturn.go

const defaultMaxDepth = 3

type SubTurnRunner struct {
    Orch     TurnOrchestrator
    maxDepth int  // 从 devrix.yaml context.subagent.max_depth 读取
}

func NewSubTurnRunner(orch TurnOrchestrator) *SubTurnRunner {
    return &SubTurnRunner{Orch: orch, maxDepth: defaultMaxDepth}
}

// SetMaxDepth 用于 devrix.yaml 加载时设置
func (r *SubTurnRunner) SetMaxDepth(n int) {
    if n > 0 {
        r.maxDepth = n
    }
}

// contracts/contracts.go 新增 error
var ErrSubagentDepthExceeded = errors.New("subagent recursion depth exceeded")
```

**LLM 引导**：当 depth 超限，error message 引导 LLM 改 mode：

```
subagent: recursion depth exceeded: depth=3, max=3. 
Hint: pass mode="brief" to delegate for shallow sub-agents, 
or restructure your task to avoid nested recursion.
```

### 2.4 AC10: 工具 schema 暴露 mode 字段

```go
// internal/layers/multiagent/delegatetools/delegate.go (假设)

var delegateInputSchema = map[string]any{
    "type": "object",
    "properties": map[string]any{
        "task": map[string]any{
            "type":        "string",
            "description": "The task description for the sub-agent.",
        },
        "mode": map[string]any{
            "type":        "string",
            "enum":        []string{"brief", "fork", "full"},
            "description": "Context isolation mode. Default: 'brief'. 'fork' = parent history + placeholder results. 'full' = full message inheritance (legacy).",
        },
        "subagent_type": map[string]any{...}, // 现有
    },
    "required": []string{"task"}, // mode 非 required，缺省按 devrix.yaml context.subagent.default_mode
}

// delegate tool 执行时：
mode := args.Mode
if mode == "" {
    mode = loadFromConfig("context.subagent.default_mode", "brief")
}
if mode == "" && hasLegacyConfig("context.subagent.legacy_mode") {
    mode = loadFromConfig("context.subagent.legacy_mode", "brief")
}
```

### 2.5 AC11: prompt cache 锚点（minimax 适配待调研）

```go
// internal/layers/llmgateway/message.go

type MessageBlock struct {
    Role    string         `json:"role"`
    Content []ContentBlock `json:"content,omitempty"`
}

type ContentBlock struct {
    Type         string         `json:"type"` // text, tool_use, tool_result, image
    Text         string         `json:"text,omitempty"`
    // Anthropic cache control
    CacheControl *CacheControl  `json:"cache_control,omitempty"`
    // ... 其他字段
}

type CacheControl struct {
    Type string `json:"type"` // "ephemeral"
}

// 构造 system prompt 时第一块加锚点：
func buildSystemPromptWithCacheAnchor(blocks []ContentBlock) []ContentBlock {
    if len(blocks) == 0 {
        return blocks
    }
    // 第一块加 cache_control
    blocks[0].CacheControl = &CacheControl{Type: "ephemeral"}
    return blocks
}
```

**minimax 适配说明**：待 S3 实施时实测 minimax SDK 是否接受 `cache_control` 字段；若忽略则等价 no-op；若报错则移除该字段并加日志。

---

## 3. 失败模式与 fallback

| 场景 | 行为 |
|------|------|
| ToolResultStore.Persist 落盘失败 | fall back 到 truncateHead + `...[truncated, persist failed]` 标记，不阻断 turn |
| FoldAssistantOutput 折叠失败 | 保留原 assistant 内容，下次 audit 时再尝试；记录 WARN 日志 |
| Prepare per-iter 失败 | fall back 到上次 prepared context；记录 ERROR 日志 |
| feishu precheck 抛非 ErrTooManyTables 错误 | 直接返回 error，不降级（避免吞错） |
| fork mode parentMsgs 为空 | 退化为 brief mode |
| depth 超限 | 返回 error，error message 引导 LLM 改 mode=brief |
| minimax 不支持 cache_control | 移除字段，加 WARN 日志，AC11 降级为 AC11a |

---

## 4. 兼容性矩阵

| 调用方 | 旧行为 | 新默认 | 切换方式 |
|--------|--------|--------|----------|
| SubTurnRunner 现有调用（无 Mode 字段） | full | brief | 加 `legacy_mode: full` 一次性切换 |
| LLM 调用 `delegate` 工具（无 mode 入参） | full | brief | tool schema 暴露 mode 字段；缺省按 default_mode |
| feishu sendCard | 总是 card | 预检 + 降级 | 自动；可通过 `communication.feishu.card_precheck.enabled: false` 关闭 |
| system prompt | 无 cache 锚点 | 有 cache 锚点 | minimax 不支持则自动降级 |

---

## 5. 性能预算

| 操作 | 目标延迟 | 度量 |
|------|----------|------|
| ToolResultStore.Persist (单条 17KB) | < 5ms | pprof benchmark |
| FoldAssistantOutput (单条 24KB) | < 10ms | pprof benchmark |
| Prepare per-iter (22 步累计) | +50ms P95 | integration benchmark |
| feishu precheck | < 1ms | pprof benchmark |
| buildForkedMessages | < 2ms | pprof benchmark |
| buildSystemPromptWithCacheAnchor | < 1ms | pprof benchmark |

---

## 6. 验收测试覆盖

### 单元测试（AC 覆盖率 100%）

| 文件 | 覆盖 AC |
|------|---------|
| `persist/tool_result_store_test.go` | AC1 (size cap + 落盘) |
| `audit/token_audit_test.go` | AC4 (audit + ShouldFoldProactively) |
| `card_precheck_test.go` | AC5 (precheck interface + ErrTooManyTables) |
| `feishu_table_precheck_test.go` | AC5 (feishu 实现) |
| `fork_messages_test.go` | AC7 (fork mode + prefix stability) |
| `subturn_test.go` | AC6/AC8/AC9 (3 mode + depth limit) |
| `token/counter_test.go` | AC13 (TruncateToTokens 升级) |

### 集成测试

| 文件 | 覆盖 AC |
|------|---------|
| `tests/integration/turn_loop_budget_test.go` | AC2/AC3/AC4 (per-iter Prepare + audit) |
| `tests/integration/subagent_mode_test.go` | AC6/AC7/AC8/AC10 (3 mode end-to-end) |
| `tests/integration/feishu_card_precheck_test.go` | AC5 (feishu 降级路径) |

### 回归测试

| 文件 | 覆盖 AC |
|------|---------|
| `tests/fixtures/d5-spans-replay.jsonl` | AC12 (D5 spans 原 prompt 复跑) |

---

## 7. 实施顺序（与 proposal.md 一致）

```
Phase A.1 (PR #1): AC1  tool result cap + 落盘           → 独立 PR
Phase A.2 (PR #2): AC2  assistant output fold             → 独立 PR
Phase A.3 (PR #3): AC3 + AC4  per-iter Prepare + audit    → 一个 PR
Phase A.4 (PR #4): AC5  feishu precheck                    → 独立 PR（紧急）
Phase A.5 (PR #5): AC13 counter.go 升级                   → 文档 + 引用
Phase B.1 (PR #6): AC6 + AC9  mode + depth                → 一个 PR
Phase B.2 (PR #7): AC10  tool schema                       → 独立 PR
Phase B.3 (PR #8): AC7 + AC11  fork + cache anchor         → 一个 PR
Phase B.4 (PR #9): AC8  mode=full 显式声明                  → 文档
回归 PR (PR #10): AC12  D5 spans 复跑                       → 验证 PR
```