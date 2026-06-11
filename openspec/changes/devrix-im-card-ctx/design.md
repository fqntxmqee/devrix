# Design: 飞书 IM 完成卡 — ctx 比例 + token 链路全埋点

**Change ID:** devrix-im-card-ctx  
**Demand ID:** DM-20260611-008  
**Status:** S3_Design

---

## 1. 架构概览

### 1.1 现状

```
D2 PEV  ─emit(complete, {usage, duration, model})→  D1 Gateway  ─buildCompletionSummary→  IM 摘要
                                                                          └─ "用时: X, 消耗: Y, 模型: M"
```

D5 观测：`pev.run` span 有 `pev.total_tokens`，但缺 prompt/completion 拆分和 ctx_pct。

### 1.2 目标

```
D2 PEV  ─emit(complete, {usage, duration, model, ctx_pct})→  D1 Gateway  ─buildCompletionSummary→  IM 摘要
                                                                              └─ "用时: X, 消耗: Y, ctx: Z%, 模型: M"
                                                                              └─ 当 Z=0 时省略 ctx
```

D5 观测：`pev.run` span 增 `pev.prompt_tokens` / `pev.completion_tokens` / `pev.ctx_pct`；milestone-only 路径增 `pev.llm_called=false`。

## 2. 关键设计决策

### 2.1 ctx_pct 计算公式

```
ctx_pct = prompt_tokens * 100 / MaxContextTokens
```

- `prompt_tokens`：`usage.PromptTokens`（PEV 主路径 = 最后一次 `runLLMCall` 的 `chunk.Usage.PromptTokens`；query loop 路径 = `res.Usage.PromptTokens`，含 iteration 累加）
- `MaxContextTokens`：`sc.TokenBudget.MaxContextTokens`，默认 128000
- 边界：MaxContextTokens=0 → 跳过（`ctx_pct=""`）
- 边界：prompt_tokens=0 → ctx_pct=0 → 摘要中省略 `ctx:` 段
- 边界：ctx_pct>100 → clamp 至 100

**为什么不用累计（跨 PEV run）**：

本期 P1 不修改 SessionContext 持久化 schema；ctx_pct 仅反映当前 PEV run 的最后 LLM 调用输入。跨 run 累计方案（DM-20260611-008-future）作为独立 future change 处理。

### 2.2 metadata 透传

emit `complete` 时新增 `ctx_pct` 字段（字符串）：

```go
ctxPct := computeCtxPct(usage.PromptTokens, sc.TokenBudget.MaxContextTokens)
emit(&gateway.EngineEvent{
    Type: "complete", SessionID: sc.SessionID,
    Metadata: map[string]string{
        "usage":    fmt.Sprintf("%d", usage.PromptTokens+usage.CompletionTokens),
        "duration": fmt.Sprintf("%d", duration),
        "model":    sc.Model,
        "ctx_pct":  fmt.Sprintf("%d", ctxPct),  // 新增
    },
})
```

D1 端读取 `event.Metadata["ctx_pct"]`，传给 `buildCompletionSummary(duration, usage, model, ctxPct)`。

### 2.3 摘要格式

**有 ctx**（prompt_tokens > 0）：

```
用时: 6s, 消耗: 1500 tokens, ctx: 12%, 模型: MiniMax-M2.7-highspeed
```

**无 ctx**（prompt_tokens=0 或 MaxContextTokens=0）：

```
用时: 6s, 消耗: 1500 tokens, 模型: MiniMax-M2.7-highspeed
```

**无 LLM 调用**（milestone-only 路径）：

```
用时: 0s, 消耗: 0 tokens, 模型: MiniMax-M2.7-highspeed
```

（D1 gateway 侧无法区分"无 LLM"和"LLM 0 token"，但两者都正确显示。）

### 2.4 Span 属性

`pev.run` span（pev_engine.go:520-525 区域）补：

| 属性 | 来源 | 说明 |
|------|------|------|
| `pev.prompt_tokens` | `usage.PromptTokens` | 最后 LLM 调用 prompt token 数 |
| `pev.completion_tokens` | `usage.CompletionTokens` | 最后 LLM 调用 completion token 数 |
| `pev.ctx_pct` | `ctxPct` | 0-100 |
| `pev.max_context_tokens` | `sc.TokenBudget.MaxContextTokens` | 上限 |
| `pev.llm_called` | bool | milestone-only 路径 = `false` |

`query.run` span（query_loop_run.go 区域）同步补同样属性。

D3 LLM gateway span 已有 `llm.tokens.prompt` / `llm.tokens.completion` / `llm.latency_ms`，**不修改**。

**DM-20260611-008 P2 补丁**（排查 token=0 根因）：D3 `llm.stream` span **新增** `llm.usage_received` 属性。

| 属性 | 来源 | 说明 |
|------|------|------|
| `llm.usage_received` | `usageReceived bool` | 整个 SSE 流期间是否出现过非零 usage 帧 |

排查路径（user 在 IM 端看到 `消耗: 0 tokens` 时）：

1. Jaeger 查 `llm.stream` span 的 `llm.usage_received`：
   - **`false`**：provider SSE 流里**完全没出现** usage 字段 → 问题在 provider 协议（MiniMax 该模型不返回 usage）。属 D3 之外，需 provider 侧或切协议。
   - **`true` 但 `llm.tokens.prompt=0`**：矛盾状态，不应出现。
2. Jaeger 查 `pev.run` span 的 `pev.prompt_tokens`：
   - **`llm.usage_received=true` 但 `pev.prompt_tokens=0`**：D2 PEV 链路未正确累加 → Devrix 端问题。
   - **两者一致 >0**：链路通畅，IM 仍 0 的话 → 检查 D1 gateway 是否部署了新代码。

### 2.5 milestone-only 路径修复

`pev_engine.go:138` 现状：

```go
emit(&gateway.EngineEvent{
    Type: "complete", SessionID: sc.SessionID,
    Metadata: map[string]string{"usage": "0", "duration": "0", "model": sc.Model},
})
```

**判断**：该路径在 `runMilestones` 完成后立即 complete，**没有再调用 LLM**（plan 已先调用 LLM），所以 `usage: "0"` 是"当前 run 没有 LLM 调用"的语义，不是 bug。

**处理**：
- 显式打 `llm_called: "false"` 让观测层能区分
- 摘要端**不**做特殊处理（与"LLM 调用但 usage=0"显示一致）

## 3. 函数签名变更

### 3.1 `buildCompletionSummary`

```go
// 旧
func buildCompletionSummary(durationStr, usageStr, model string) string

// 新
func buildCompletionSummary(durationStr, usageStr, model, ctxPctStr string) string
```

向后兼容：当 `ctxPctStr` 为空或为 "0" 时，省略 `ctx:` 段。

### 3.2 `computeCtxPct` 新增 helper（summary.go 内）

```go
// computeCtxPct returns the percentage of the context window used by the last
// LLM call's prompt tokens. Returns 0 if MaxContextTokens is 0.
func computeCtxPct(promptTokens, maxContextTokens int) int {
    if maxContextTokens <= 0 || promptTokens <= 0 {
        return 0
    }
    pct := promptTokens * 100 / maxContextTokens
    if pct > 100 {
        pct = 100
    }
    return pct
}
```

**不**放 telemetry/names.go（与格式化耦合，放 summary.go 更近；可被单测覆盖）。

## 4. 失败模式与降级

| 场景 | 表现 | 处理 |
|------|------|------|
| LLM SSE 帧无 `usage` | `chunk.Usage = {0,0,0}`，链路传到 IM = "0 tokens" | 不在 D2 修复（属 D3 provider 协议层）；用 Jaeger `llm.tokens.prompt=0` 定位 |
| `sc.TokenBudget.MaxContextTokens = 0` | ctx_pct 永远 = 0 | 摘要中省略 `ctx:` 段（与 prompt=0 行为一致） |
| emit complete 时 usage 未到 | `usage = {0,0,0}` | 当前实现已如此；本变更不引入新失败 |
| provider 协议中途变更 | IM 端误报 0 | D5 已有 `llm.tokens.prompt=0` 可在 Jaeger 看到 |

## 5. 测试矩阵

| L5 | 场景 | 断言 |
|----|------|------|
| L5-1-1-03 | `buildCompletionSummary("7655", "1500", "claude-sonnet-4-6", "12")` | `"用时: 8s, 消耗: 1500 tokens, ctx: 12%, 模型: claude-sonnet-4-6"` |
| L5-1-1-03 | `buildCompletionSummary` ctx=0/空/garbage 全部省略 ctx 段 | 4 个子 case |
| L5-1-1-04 | `ComputeCtxPct` 边界：0 prompt / 0 max / 负数 / 超限 clamp | 9 个子 case |
| L5-2-1-13 | PEV 主路径 + milestone-only 路径 emit complete 含 `ctx_pct` / `llm_called` | metadata 断言 |
| L5-2-1-14 | query loop runSpan 含 `pev.prompt_tokens` / `pev.completion_tokens` / `pev.ctx_pct` | span attribute 断言 |

## 6. 风险与回滚

| 风险 | 缓解 |
|------|------|
| 改 `buildCompletionSummary` 签名破坏现有调用方 | 仓库内 grep：`buildCompletionSummary` 仅有 1 个调用方（gateway.go:423） + 1 个测试方（summary_test.go:116） |
| ctx_pct 计算口径与用户预期不符 | 与用户已对齐（口径 A：prompt_tokens / MaxContextTokens） |
| 摘要长度变长导致 IM 卡片溢出 | ctx 段最多 8 字符（"ctx: 100%, "），飞书卡片可承载 |
| milestone-only 路径显示 "0 tokens" 困惑 | 摘要端不处理；Jaeger 通过 `pev.llm_called=false` 区分 |

**回滚**：Revert PR。`buildCompletionSummary` 签名变更点集中，单点回滚。

## 7. 关联

- 依赖 D3 LLM gateway 已有 span（`llm.tokens.*`），不修改
- 不影响 OpenSpec 其他 change
- future: DM-20260611-008-future（跨 PEV run 累计 token）
