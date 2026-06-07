# Context Engine V2 Design

**Change ID:** devrix-context-engine-v2
**Layer:** 2 - Context Engine
**Status:** S3 Ready (pending sign-off) — Grilled & Updated
**Version:** 2.0.1
**Based on:** `openspec/archive/2026-06-07-devrix-context-engine/design.md`
**Demand:** DM-20260607-003
**Grill Session:** 2026-06-07, 14 decisions resolved (see §十一 开放问题 + §十三 Grill 变更记录)

---

## 一、架构目标

### 1.1 V2 增量目标

| 业务目标 | V1 | V2 | 量化 |
|---------|----|----|------|
| Autocompact | skip | LLM 摘要 | 步骤 6 启用后 token 下降 ≥20%；P99 < 10s |
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

**Turn 定义：** 以 `user` role 为 turn 边界——从上一条 `user` 消息（含）到下一条 `user` 消息（不含）为一个 turn，中间的所有 assistant/tool_call/tool_result 消息属于该 turn。

保留：
- 首 `preserve_head_turns` 个 turn（默认 2）
- 尾 `preserve_tail_turns` 个 turn（默认 2）

当 `len(turns) <= preserve_head_turns + preserve_tail_turns` 时，不触发 Autocompact（所有 turn 均保留）。

中间 turn 的扁平消息列表送入 LLM 摘要。

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

Prompt 模板：

```
You are a conversation summarizer. Below is the middle segment of a developer-AI conversation.
Summarize ONLY what was explicitly discussed. Do NOT invent any details not present in the input.

Output strict JSON:
{
  "topics": ["list of technical topics discussed"],
  "decisions": ["list of decisions made or actions agreed"],
  "open_items": ["list of unresolved questions or pending tasks"]
}

Rules:
- If unsure about any detail, omit it rather than guess.
- Do NOT mention file paths, code, or tool outputs unless they appear in the input.
- Limit each array to at most 5 items.
- If a category has nothing to report, use an empty array.

Conversation segment:
{middle_messages_formatted}
```

摘要结果写入单条 `assistant` 消息，Metadata：`compressed_by=autocompact`, `original_count=N`。

### 2.4 失败降级

| 错误 | 行为 |
|------|------|
| LLM 超时（>10s）/错误 | 记录 `autocompact:degraded`，跳过步骤 6（等同 V1） |
| JSON 解析失败 | 重试 1 次；仍失败则 skip，记录 `autocompact:degraded` |
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

| 指标 | 类型 | 标签 |
|------|------|------|
| `devrix_ctx_autocompact_total` | Counter | — |
| `devrix_ctx_autocompact_degraded_total` | Counter | — |
| `devrix_ctx_verify_command_duration_ms` | Histogram | `command` |
| `devrix_ctx_verify_command_failures_total` | Counter | `command` |
| `devrix_ctx_compression_tokens_saved` | Histogram | — |

### 5.3 独立 Observer 接口（非破坏性）

新增两个独立可选接口，保持单一职责，避免破坏 V1 `IObserver` 实现方：

```go
// contracts.go

// ICompressionObserver 压缩管道可观测（步骤 1-7 + Autocompact）
type ICompressionObserver interface {
    EmitCompressionStep(sessionID, step string, before, after int)
    EmitAutocompact(sessionID string, meta AutocompactMeta)
}

// IPEVObserver PEV 阶段可观测（Verify Commands；V3 扩展 Plan）
type IPEVObserver interface {
    EmitVerifyCommand(sessionID, cmd string, result VerifyCommandResult)
}
```

```go
// EngineDeps
type EngineDeps struct {
    // ...
    CompressionObserver ICompressionObserver // optional, NoOp default
    PEVObserver         IPEVObserver         // optional, NoOp default
}
```

### 5.4 Metrics 标签基数约束

防止自定义配置导致 Prometheus 标签基数爆炸：

| 约束 | 规则 |
|------|------|
| Verify command name | 仅允许 `[a-z0-9_-]+`，配置校验时强制 |
| Verify command 数量 | 配置中 `verify_commands` ≤ 10，超出拒绝加载 |
| `exit_code` 标签 | **不加**（退出码变化多）；改用独立 counter `devrix_ctx_verify_command_failures_total{command="go-test"}` |
| Step name 标签 | 固定集合 7 个值，无基数风险 |

**新增 metrics：**

| 指标 | 类型 | 标签 |
|------|------|------|
| `devrix_ctx_verify_command_failures_total` | Counter | `command` |

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
      timeout: 10s                  # P99 摘要延迟上限
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
    Name       string   `yaml:"name"`       // 仅允许 [a-z0-9_-]+
    Executable string   `yaml:"executable"`
    Args       []string `yaml:"args"`
    Timeout    time.Duration
}
```

### 6.3 配置校验（新增）

配置加载时强制执行以下校验：

| 校验项 | 规则 | 违反时行为 |
|--------|------|-----------|
| `verify_commands[].name` | 必须匹配 `^[a-z0-9_-]+$` | 拒绝加载，返回配置错误 |
| `verify_commands` 数量 | ≤ 10 | 拒绝加载，返回配置错误 |
| `executable` / `args[]` 元字符 | 禁止包含 `;` `\|` `&` `$` `` ` `` | 拒绝加载 |
| `autocompact.timeout` | 必须 ≤ 10s（与 P99 目标对齐） | 拒绝加载 |

WorkDir 校验（运行时）：`filepath.Clean(session.WorkDir)` 必须与 session 创建时的 WorkDir 精确匹配，不匹配则返回 `CTX_VERIFY_CMD_4012`。

---

## 七、包结构增量

```
internal/layers/contextengine/
├── compression/
│   ├── pipeline.go           # MODIFIED: 步骤 6 条件执行；counter 改为 contracts.ITokenCounter
│   ├── pipeline_test.go      # MODIFIED: 更新 constructor 调用
│   ├── autocompact.go        # NEW: AutocompactStep + prompt 模板
│   └── autocompact_test.go   # NEW
├── pev/
│   ├── pev_engine.go         # MODIFIED: 注入 IVerifyCommandRunner
│   ├── pev_engine_test.go    # MODIFIED
│   ├── verify_commands.go    # NEW: verifyPEV 方法扩展（commands 模式）
│   ├── verify_runner.go      # NEW: BuiltinVerifyRunner
│   └── verify_runner_test.go # NEW
├── token/
│   └── counter.go            # 实现 contracts.ITokenCounter（functional options 兼容）
├── contracts.go              # MODIFIED: +ICompressionObserver, +IPEVObserver
└── engine.go                 # MODIFIED: 注入 TokenCounter, CompressionObserver, PEVObserver
```

---

## 八、main.go 接线

```go
llmGW := llmgateway.New(cfg.LLMGateway)                // 实现 contextengine.ILLMGateway
tokenCtr := llmgateway.NewTokenCounter(cfg.LLMGateway)  // 实现 contracts.ITokenCounter

contextEngine := contextengine.NewContextEngine(contextengine.EngineDeps{
    LLM:                  llmGW,
    TokenCounter:         tokenCtr,
    Tools:                toolRunner,
    ToolsReg:             toolRegistry,
    Permission:           permissionGate,
    Observer:             obsBridge,
    CompressionObserver:  obsBridge,                     // 可选（共享 obsBridge 实例）
    PEVObserver:          obsBridge,                     // 可选（V3 扩展 Plan）
    VerifyRunner:         contextengine.NewBuiltinVerifyRunner(),
    Config:               cfg.ContextEngine,
})
```

`Pipeline` 构造函数采用 functional options：

```go
compression.NewPipeline(
    compression.WithCounter(tokenCtr),
    compression.WithEnabled(cfg.CompressionEnabled),
    compression.WithAutocompactConfig(cfg.Autocompact),
)
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
| CTX_AUTOCOMPACT_4010 | `NewAutocompactFailedError` | LLM 摘要失败（超时/网络/JSON 解析）且降级；具体原因在 error message 中区分 |
| CTX_VERIFY_CMD_4011 | `NewVerifyCommandFailedError` | 验证命令非零退出 |
| CTX_VERIFY_CMD_4012 | `NewVerifyCommandRejectedError` | 配置/注入校验拒绝（元字符、WorkDir 越权、数量超限） |

登记于 `internal/shared/errors/context.go`。

---

## 十一、开放问题

| # | 问题 | V2 决议 | 状态 |
|---|------|---------|------|
| 1 | Autocompact 同步/异步 | 同步；步骤 1-4 <100ms，摘要 P99 < **10s** | **已决议** |
| 2 | Verify 命令执行 | executable+args，禁止 shell | **已决议** |
| 3 | 快照加密 | V2 不做 | **已决议** |
| 4 | Autocompact 模型 | `autocompact.model` 直接指定 | **已决议** |
| 5 | L5-CTX-08 语义 | `enabled=false` 时仍 skip | **已决议** |
| 6 | 管道步骤顺序 | 1-4 → 6 → 5 → 7 | **已决议** |
| 7 | ITokenCounter 归属 | `shared/contracts/tokencounter.go` | **已决议** |
| 8 | Autocompact 超时 | 10s（不可接受 30s 用户等待） | **已决议（Grill）** |
| 9 | Observer 接口拆分 | `ICompressionObserver` + `IPEVObserver`（单一职责） | **已决议（Grill）** |
| 10 | Turn 定义 | 以 `user` role 为边界切分 | **已决议（Grill）** |
| 11 | Autocompact Prompt 模板 | JSON + ≤5项/数组 + 空数组允许 | **已决议（Grill）** |
| 12 | Verify WorkDir 沙箱 | 固定 session.WorkDir + 精确匹配校验 | **已决议（Grill）** |
| 13 | Metrics 标签基数 | ≤10 命令 + name regex + 不加 exit_code 标签 | **已决议（Grill）** |
| 14 | Pipeline 构造函数 | Functional options 模式 | **已决议（Grill）** |
| 15 | verifyPEV 重构 | 改为 PEVEngine 方法 | **已决议（Grill）** |
| 16 | 错误码 4010 | 不拆分，error message 区分 | **已决议（Grill）** |
| 17 | 快照版本兼容 | 保持 ctx-v1，不升版本 | **已决议（Grill）** |
| 18 | 幻觉防护 | metadata 标记 + prompt 约束，V2 不检测 | **已决议（Grill）** |

---

## 十二、参考

| 文档 | 路径 |
|------|------|
| V1 归档设计 | `openspec/archive/2026-06-07-devrix-context-engine/design.md` |
| Canonical spec | `openspec/specs/context-engine/spec.md` |
| LLM Gateway | `openspec/archive/2026-06-07-devrix-llm-gateway/design.md` |
| 详细六段式 | `docs/context-engine-design.md`（V2 实施后同步附录 B） |

---

## 十三、Grill 变更记录（2026-06-07）

本次设计经 `grill-with-docs` 技能深度审查，14 项决策已决议。以下为与原 S3 初稿的差异：

| # | 变更点 | 原值 | 新值 | 影响范围 |
|---|--------|------|------|----------|
| 1 | `autocompact.timeout` | 30s | **10s** | config, design §2.4, spec scenario, proposal risks |
| 2 | Observer 接口 | 单一 `ICompressionObserver`（3 方法） | **拆为** `ICompressionObserver`（压缩）+ `IPEVObserver`（PEV） | contracts.go, engine.go, §5.3 |
| 3 | Turn 定义 | 未明确 | 以 `user` role 为边界 | autocompact.go, §2.2 |
| 4 | Autocompact Prompt | 无模板 | 结构化 JSON + ≤5项/数组 + 空数组允许 | autocompact.go, §2.3 |
| 5 | Verify WorkDir 安全 | `filepath.Clean` + trusted root 前缀 | 固定 session.WorkDir + 精确匹配 | verify_runner.go, §3.2, §6.3 |
| 6 | Metrics 标签基数 | 无约束 | ≤10 命令 + name regex + 不加 exit_code 标签 | §5.2, §5.4 |
| 7 | Pipeline 构造函数 | 普通参数 | Functional options 模式 | pipeline.go, engine.go, §8 |
| 8 | verifyPEV 函数 | 纯函数 | PEVEngine 方法 | pev_engine.go, §3.3 |
| 9 | 错误码 4010 | 未明确是否拆分 | 不拆分，error message 区分 | errors/context.go, §10 |
| 10 | 快照版本兼容 | 未讨论 | 保持 ctx-v1 | snapshot/store.go |
| 11 | 幻觉防护 | 仅 prompt 约束 | + metadata 标记 `compressed_by=autocompact` | autocompact.go, §2.3 |
| 12 | Autocompact Gateway 共享 | 未讨论 | 同实例，降级路径已覆盖 | §2.4, §4 |
| 13 | PEVEngine 新增依赖 | 无 | `IVerifyCommandRunner` 注入为字段 | pev_engine.go, §3.1, §7 |
| 14 | Metrics failure counter | 无 | 新增 `devrix_ctx_verify_command_failures_total` | §5.2 |

**跨 change 前置状态确认：**
- `contracts.ITokenCounter` 已存在于 `shared/contracts/tokencounter.go` — L2/L3 均已实现
- tasks.md T1/T2 实际已完成（接口定义 + HeuristicCounter 适配），仅需 T3 注入适配
