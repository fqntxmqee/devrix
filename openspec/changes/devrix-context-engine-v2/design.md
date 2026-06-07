# Context Engine V2 Design

**Change ID:** devrix-context-engine-v2
**Layer:** 2 - Context Engine
**Status:** S3 Ready (pending sign-off)
**Version:** 2.0.0
**Based on:** `openspec/archive/2026-06-07-devrix-context-engine/design.md`
**Demand:** DM-20260607-003

---

## 一、架构目标

### 1.1 V2 增量目标

| 业务目标 | V1 | V2 | 量化 |
|---------|----|----|------|
| Autocompact | skip | LLM 摘要 | 步骤 6 启用后 token 下降 ≥20%（基准样例） |
| Verify commands | basic only | 白名单命令 | 支持 `go test`/`go vet` |
| Token 计数 | char/4 | cl100k_base | 与 Gateway 误差 <5% |
| LLM 接线 | Mock | 真实 Gateway | 主路径无 Mock |
| 压缩可观测 | report only | span per step | 7 步 + autocompact + verify |

### 1.2 层间边界（不变 + 新增）

```
Layer 2 (Context Engine V2)              Layer 3 (LLM Gateway)
────────────────────────────            ───────────────────────
CompressionPipeline.Run()
  step 6 Autocompact ──────────────▶ ILLMGateway.ChatStream(model=autocompact.model)
PEV Execute ───────────────────────▶ ILLMGateway.ChatStream(session model)
TokenCounter ──────────────────────▶ shared/contracts.ITokenCounter（Gateway 实现）
PEV Verify (commands) ─────────────▶ IVerifyCommandRunner（exec.CommandContext，非 shell）
```

**禁止：** Context Engine 不得 import `llmgateway` 具体 adapter 包；仅依赖 `shared/contracts.ITokenCounter`、`contextengine.ILLMGateway` 接口。

### 1.3 压缩管道执行序（V2 决议）

```
messages (no system prompt yet)
  → [1] ToolResultBudget
  → [2] Snip
  → [3] Microcompact
  → [4] ContextCollapse
  → [6] Autocompact        // V2: 条件执行；在消息历史上摘要中间段
  → [5] SystemPromptAssembly
  → [7] TokenBlock
  → compressed_messages
```

> 步骤编号保持 1–7 不变；**物理执行顺序**为 1-4 → 6 → 5 → 7（与 V1 代码插入点一致）。

---

## 二、Autocompact（步骤 6）

### 2.1 触发条件

```go
func shouldAutocompact(msgs []types.Message, budget types.TokenBudget, counter ITokenCounter, cfg AutocompactConfig) bool {
    if !cfg.Enabled {
        return false
    }
    if len(msgs) < cfg.MinMessagesForSummary {
        return false
    }
    return counter.CountMessages(msgs) > budget.CompressionTarget
}
```

在步骤 1–4 执行后、步骤 5 Assembly **之前**评估（消息历史不含 system prompt）。

### 2.2 摘要段选取

保留：
- 首 `preserve_head_turns` 轮（默认 2）
- 尾 `preserve_tail_turns` 轮（默认 2）

中间段送入 LLM 摘要。

### 2.3 摘要请求

```go
// compression/autocompact.go

type AutocompactStep struct {
    llm      ILLMGateway
    counter  ITokenCounter
    observer IObserver
    cfg      AutocompactConfig
}

type SummaryOutput struct {
    Topics     []string `json:"topics"`
    Decisions  []string `json:"decisions"`
    OpenItems  []string `json:"open_items"`
}
```

Prompt 约束：
- 仅总结输入消息中已出现的事实
- 输出严格 JSON（`SummaryOutput`）
- 禁止编造文件路径或未执行的操作

摘要结果写入单条 `assistant` 消息，Metadata：`compressed_by=autocompact`, `original_count=N`。

### 2.4 失败降级

| 错误 | 行为 |
|------|------|
| LLM 超时/错误 | 记录 `autocompact:failed`，跳过步骤 6（等同 V1） |
| JSON 解析失败 | 重试 1 次；仍失败则 skip |
| 摘要后仍超限 | 进入步骤 7 TokenBlock |

### 2.5 Pipeline 变更

```go
// V1 (pipeline.go:74-75)
report.StepsApplied = append(report.StepsApplied, stepAutocompact+":skipped")

// V2
if shouldAutocompact(current, budget, p.counter, p.autocompactCfg) {
    next, err := p.autocompact.Run(ctx, current, budget)
    // ...
} else {
    report.StepsApplied = append(report.StepsApplied, stepAutocompact+":skipped")
}
```

---

## 三、PEV Verify Commands 模式

### 3.1 接口

```go
// pev/verify_runner.go

type IVerifyCommandRunner interface {
    Run(ctx context.Context, cmd VerifyCommand) (VerifyCommandResult, error)
}

type VerifyCommand struct {
    Name       string
    Executable string        // 如 "go"，禁止路径含 shell 元字符
    Args       []string       // 如 ["test", "./..."]
    Timeout    time.Duration
    WorkDir    string
}

type VerifyCommandResult struct {
    ExitCode int
    Stdout   string
    Stderr   string
    Duration time.Duration
}
```

### 3.2 安全规则

1. 命令必须在 `verify_commands` 白名单内（按 `name` 匹配）
2. 使用 `exec.CommandContext(ctx, executable, args...)`，**禁止** `sh -c`
3. `executable` 与每个 `arg` 禁止包含 `; | & $ \` ` 等 shell 元字符
4. `WorkDir` 默认为 `session.WorkDir`；`filepath.Clean` 后必须在 trusted root（如 session 创建时记录的根路径）之下
5. 超时后 kill 进程，返回非零 exit

### 3.3 Verify 流程扩展

```go
func verifyPEV(mode string, results []ToolResult, runner IVerifyCommandRunner, cmds []VerifyCommand, policy string) types.VerifyResult {
    switch mode {
    case "none":
        return types.VerifyResult{Passed: true}
    case "basic":
        return verifyBasic(results)
    case "commands":
        if !verifyBasic(results).Passed {
            return verifyBasic(results)
        }
        return verifyCommands(runner, cmds, policy)
    default:
        return verifyBasic(results)
    }
}
```

`verify_policy`:
- `all_pass`（默认）：所有命令 exit 0 才通过
- `any_pass`：任一命令 exit 0 即通过

### 3.4 与 PEV 重试

Verify 失败 → `Deviation` 按失败命令数加权 → 未达 `max_iterations` 则重新 Execute。

---

## 四、Token 计数统一

### 4.1 共享契约（L2/L3 共同依赖）

```go
// internal/shared/contracts/tokencounter.go

package contracts

// ITokenCounter Token 计数（L2 Context Engine + L3 LLM Gateway 共享）
type ITokenCounter interface {
    CountText(text string) int
    CountMessages(messages []types.Message) int
    CountWithSystemPrompt(systemPrompt string, messages []types.Message) int
    TruncateToTokens(text string, maxTokens int) string
    EncodingForModel(model string) string // e.g. "cl100k_base"; Gateway 按 model 映射
}
```

> **跨 change 对齐：** `devrix-llm-gateway` M1 须实现此接口（非独立签名）。L5-CTX-16 基准样例使用 `cl100k_base` 兼容模型；其他模型允许 ±5% 误差或单独基准集。

### 4.2 Context Engine 注入

```go
// engine deps — 直接注入 contracts.ITokenCounter，无需 import llmgateway
type EngineDeps struct {
    // ...
    TokenCounter contracts.ITokenCounter
}
```

V1 `token/counter.go`（char/4）保留为 `HeuristicCounter`，实现同一接口，仅用于 `token_counter.source=heuristic` 测试模式。

### 4.3 注入点

- `compression.Pipeline` — 预算判断、ToolResultBudget、TokenBlock
- `engine.Process` — 压缩触发阈值
- `AutocompactStep` — 摘要输出预算

---

## 五、Observability 集成

### 5.1 Span

| Span 名称 | 属性 |
|-----------|------|
| `ctx.compress.step` | `step`, `tokens_before`, `tokens_after`, `session_id` |
| `ctx.compress.autocompact` | `model`, `summary_tokens`, `latency_ms`, `degraded` |
| `ctx.pev.verify.commands` | `command`, `exit_code`, `duration_ms` |

### 5.2 Metrics

| 指标 | 类型 |
|------|------|
| `devrix_ctx_autocompact_total` | Counter |
| `devrix_ctx_autocompact_degraded_total` | Counter |
| `devrix_ctx_verify_command_duration_ms` | Histogram |
| `devrix_ctx_compression_tokens_saved` | Histogram |

### 5.3 IObserver 扩展（非破坏性）

新增独立可选接口，避免破坏 V1 `IObserver` 实现方：

```go
// contracts.go
type ICompressionObserver interface {
    EmitCompressionStep(sessionID, step string, before, after int)
    EmitAutocompact(sessionID string, meta AutocompactMeta)
    EmitVerifyCommand(sessionID, cmd string, result VerifyCommandResult)
}

// EngineDeps.CompressionObserver ICompressionObserver // optional, NoOp default
```

---

## 六、配置

### 6.1 devrix.yaml 增量

```yaml
context_engine:
  # ... V1 字段保留 ...
  compression:
    autocompact:
      enabled: true
      model: deepseek-v4-flash     # 直接指定模型名，映射至 LLMRequest.Model
      summary_max_tokens: 512
      min_messages_for_summary: 8
      preserve_head_turns: 2
      preserve_tail_turns: 2
      timeout: 30s                  # P99 摘要延迟上限
  pev:
    verify_mode: basic             # basic | none | commands
    verify_policy: all_pass        # all_pass | any_pass
    verify_commands:
      - name: go-test
        executable: go
        args: ["test", "./..."]
        timeout: 120s
      - name: go-vet
        executable: go
        args: ["vet", "./..."]
        timeout: 30s
  token_counter:
    source: gateway                # gateway | heuristic（仅测试）
```

### 6.2 Config 结构

```go
type AutocompactConfig struct {
    Enabled               bool
    Model                 string
    SummaryMaxTokens      int
    MinMessagesForSummary int
    PreserveHeadTurns     int
    PreserveTailTurns     int
    Timeout               time.Duration
}

type VerifyCommandConfig struct {
    Name       string
    Executable string
    Args       []string
    Timeout    time.Duration
}
```

---

## 七、包结构增量

```
internal/layers/contextengine/
├── compression/
│   ├── pipeline.go           # MODIFIED: 步骤 6 条件执行
│   ├── autocompact.go        # NEW
│   └── autocompact_test.go   # NEW
├── verify_commands.go        # NEW（与 pev_engine.go 同包，避免 V1 包结构迁移）
├── verify_runner.go          # NEW
├── token/
│   └── counter.go            # HeuristicCounter 实现 shared/contracts.ITokenCounter
├── contracts.go              # MODIFIED
└── engine.go                 # MODIFIED: 注入点
```

---

## 八、main.go 接线

```go
llmGW := llmgateway.New(cfg.LLMGateway)           // 实现 contextengine.ILLMGateway
tokenCtr := llmgateway.NewTokenCounter(cfg.LLMGateway) // 实现 contracts.ITokenCounter

contextEngine := contextengine.NewContextEngine(contextengine.EngineDeps{
    LLM:                  llmGW,
    TokenCounter:         tokenCtr,
    Tools:                toolRunner,
    ToolsReg:             toolRegistry,
    Permission:           permissionGate,
    Observer:             obsBridge,
    CompressionObserver:  obsBridge,                // 可选
    VerifyRunner:         contextengine.NewBuiltinVerifyRunner(),
    Config:               cfg.ContextEngine,
})
```

`DEVRIX_ENGINE=context` 与 `devrix-feishu` 同步更新。

---

## 九、测试策略与 L5 映射

| L5 ID | 层级 | 文件 |
|-------|------|------|
| L5-CTX-12 | unit + acceptance | `compression/autocompact_test.go` |
| L5-CTX-13 | unit | `compression/autocompact_test.go` |
| L5-CTX-14 | integration | `tests/integration/context_verify_commands_test.go` |
| L5-CTX-15 | integration | 同上 |
| L5-CTX-16 | unit | `token/gateway_adapter_test.go` |
| L5-CTX-17 | integration | `tests/integration/context_compression_obs_test.go` |
| L5-CTX-18 | integration | `tests/integration/context_llm_gateway_test.go` |

---

## 十、错误码

| Code | 常量 | 场景 |
|------|------|------|
| CTX_AUTOCOMPACT_4010 | `NewAutocompactFailedError` | LLM 摘要失败且降级 |
| CTX_VERIFY_CMD_4011 | `NewVerifyCommandFailedError` | 验证命令非零退出 |
| CTX_VERIFY_CMD_4012 | `NewVerifyCommandRejectedError` | 配置/注入校验拒绝 |

登记于 `internal/shared/errors/context.go`。

---

## 十一、开放问题

| # | 问题 | V2 决议 | 状态 |
|---|------|---------|------|
| 1 | Autocompact 同步/异步 | 同步；步骤 1-4 <100ms，摘要 P99 <30s | **已决议** |
| 2 | Verify 命令执行 | executable+args，禁止 shell | **已决议** |
| 3 | 快照加密 | V2 不做 | **已决议** |
| 4 | Autocompact 模型 | `autocompact.model` 直接指定 | **已决议** |
| 5 | L5-CTX-08 语义 | `enabled=false` 时仍 skip | **已决议** |
| 6 | 管道步骤顺序 | 1-4 → 6 → 5 → 7 | **已决议** |
| 7 | ITokenCounter 归属 | `shared/contracts/tokencounter.go` | **已决议** |

---

## 十二、参考

| 文档 | 路径 |
|------|------|
| V1 归档设计 | `openspec/archive/2026-06-07-devrix-context-engine/design.md` |
| Canonical spec | `openspec/specs/context-engine/spec.md` |
| LLM Gateway | `openspec/changes/devrix-llm-gateway/design.md` |
| 详细六段式 | `docs/context-engine-design.md`（V2 实施后同步附录 B） |
